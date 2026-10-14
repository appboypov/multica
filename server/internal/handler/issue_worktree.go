package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// isAbsoluteDaemonPath accepts a POSIX absolute path or a Windows drive path.
// The server stores only the absolute paths a daemon resolved (ADR-0001).
func isAbsoluteDaemonPath(path string) bool {
	if filepath.IsAbs(path) || strings.HasPrefix(path, "/") {
		return true
	}
	return len(path) >= 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/') &&
		((path[0] >= 'a' && path[0] <= 'z') || (path[0] >= 'A' && path[0] <= 'Z'))
}

// RecordIssueWorktree is the daemon's record of an issue worktree it made or
// reused for a task: an upsert on (issue, repository) as active (ADR-0007).
func (h *Handler) RecordIssueWorktree(w http.ResponseWriter, r *http.Request) {
	task, workspaceID, ok := h.requireDaemonTaskAccessWithWorkspace(w, r, chi.URLParam(r, "taskId"))
	if !ok {
		return
	}
	if !task.IssueID.Valid {
		writeError(w, http.StatusBadRequest, "task has no issue")
		return
	}
	var body struct {
		RepositoryID string `json:"repository_id"`
		Path         string `json:"path"`
		Branch       string `json:"branch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	repositoryUUID, ok := parseUUIDOrBadRequest(w, body.RepositoryID, "repository_id")
	if !ok {
		return
	}
	body.Path = strings.TrimSpace(body.Path)
	body.Branch = strings.TrimSpace(body.Branch)
	if !isAbsoluteDaemonPath(body.Path) {
		writeError(w, http.StatusBadRequest, "path must be absolute")
		return
	}
	if body.Branch == "" {
		writeError(w, http.StatusBadRequest, "branch is required")
		return
	}
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	if _, err := h.Queries.GetRepositoryInWorkspace(r.Context(), db.GetRepositoryInWorkspaceParams{ID: repositoryUUID, WorkspaceID: wsUUID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "repository not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load repository")
		return
	}
	runtime, err := h.Queries.GetAgentRuntime(r.Context(), task.RuntimeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load runtime")
		return
	}
	row, err := h.Queries.UpsertIssueWorktree(r.Context(), db.UpsertIssueWorktreeParams{
		WorkspaceID:  wsUUID,
		IssueID:      task.IssueID,
		RepositoryID: repositoryUUID,
		DaemonID:     runtime.DaemonID.String,
		Path:         body.Path,
		Branch:       body.Branch,
	})
	if err != nil {
		slog.Error("record issue worktree failed", "error", err, "task_id", uuidToString(task.ID))
		writeError(w, http.StatusInternalServerError, "failed to record issue worktree")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": uuidToString(row.ID), "state": row.State})
}

// ReportIssueWorktreeRemoval is the daemon's answer to one heartbeat removal:
// removed deletes the row, kept records why and says so on the issue.
func (h *Handler) ReportIssueWorktreeRemoval(w http.ResponseWriter, r *http.Request) {
	idUUID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "issue worktree id")
	if !ok {
		return
	}
	var body struct {
		RuntimeID string `json:"runtime_id"`
		Outcome   string `json:"outcome"`
		Reason    string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	rt, ok := h.requireDaemonRuntimeAccess(w, r, body.RuntimeID)
	if !ok {
		return
	}
	switch body.Outcome {
	case "removed":
		_, err := h.Queries.DeleteIssueWorktree(r.Context(), db.DeleteIssueWorktreeParams{
			ID:          idUUID,
			WorkspaceID: rt.WorkspaceID,
			DaemonID:    rt.DaemonID.String,
		})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusInternalServerError, "failed to delete issue worktree")
			return
		}
	case "kept":
		reason := strings.TrimSpace(body.Reason)
		if reason == "" {
			writeError(w, http.StatusBadRequest, "reason is required")
			return
		}
		row, err := h.Queries.KeepIssueWorktree(r.Context(), db.KeepIssueWorktreeParams{
			KeptReason:  strToText(reason),
			ID:          idUUID,
			WorkspaceID: rt.WorkspaceID,
			DaemonID:    rt.DaemonID.String,
		})
		if err != nil {
			// The issue was reopened meanwhile, or another answer won: the
			// row is no longer removing, so there is nothing to keep.
			if errors.Is(err, pgx.ErrNoRows) {
				writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to keep issue worktree")
			return
		}
		h.noteIssueWorktreeKept(r.Context(), row)
	default:
		writeError(w, http.StatusBadRequest, "outcome must be removed or kept")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// noteIssueWorktreeKept adds the one system comment that says why a closed
// issue's worktree stayed. Best-effort: the row already records the reason.
func (h *Handler) noteIssueWorktreeKept(ctx context.Context, row db.IssueWorktree) {
	issue, err := h.Queries.GetIssue(ctx, row.IssueID)
	if err != nil {
		slog.Warn("issue worktree kept: load issue failed", "error", err, "issue_id", uuidToString(row.IssueID))
		return
	}
	created, err := h.Queries.CreateComment(ctx, db.CreateCommentParams{
		ID:          dbid.NewV7(),
		IssueID:     issue.ID,
		WorkspaceID: issue.WorkspaceID,
		AuthorType:  "system",
		AuthorID:    pgtype.UUID{Valid: true},
		Content:     fmt.Sprintf("`%s` stayed because it %s.", row.Path, row.KeptReason.String),
		Type:        "system",
		ParentID:    pgtype.UUID{Valid: false},
	})
	if err != nil {
		slog.Warn("issue worktree kept: create system comment failed", "error", err, "issue_id", uuidToString(issue.ID))
		return
	}
	comment := created.Comment()
	h.publish(protocol.EventCommentCreated, uuidToString(issue.WorkspaceID), "system", "", map[string]any{
		"comment":             commentToResponse(comment, nil, nil),
		"issue_title":         issue.Title,
		"issue_assignee_type": textToPtr(issue.AssigneeType),
		"issue_assignee_id":   uuidToPtr(issue.AssigneeID),
		"issue_status":        issue.Status,
		"issue_revision":      created.IssueRevision,
	})
}

// SyncIssueWorktreeStatus follows the issue's terminal-status transition and
// wakes each online runtime of the daemons that hold its recorded worktrees.
func (h *Handler) SyncIssueWorktreeStatus(ctx context.Context, issue IssueResponse) {
	workspaceID := parseUUID(issue.WorkspaceID)
	status := issuestatus.Effective(ctx, h.Queries, workspaceID, issue.Status)
	state := "active"
	if status == "done" || status == "cancelled" {
		state = "removing"
	}
	rows, err := h.Queries.SetIssueWorktreesStateForIssue(ctx, db.SetIssueWorktreesStateForIssueParams{
		State: state, IssueID: parseUUID(issue.ID), WorkspaceID: workspaceID,
	})
	if err != nil {
		slog.Error("update issue worktree state failed", "issue_id", issue.ID, "error", err)
		return
	}
	if state != "removing" || len(rows) == 0 {
		return
	}
	daemons := make(map[string]bool, len(rows))
	for _, row := range rows {
		daemons[row.DaemonID] = true
	}
	runtimes, err := h.Queries.ListAgentRuntimes(ctx, workspaceID)
	if err != nil {
		slog.Error("load issue worktree runtimes failed", "issue_id", issue.ID, "error", err)
		return
	}
	for _, runtime := range runtimes {
		if runtime.Status == "online" && daemons[runtime.DaemonID.String] {
			h.requestDaemonPendingWork(uuidToString(runtime.ID), protocol.PendingWorkKindIssueWorktrees)
		}
	}
}
