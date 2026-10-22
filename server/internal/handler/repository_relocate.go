package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/workpath"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// A relocate moves a repository to a new checkout and rewrites the recorded
// paths of its worklanes and issue worktrees (ADR-0037). It runs in two
// steps on the existing checkout check: the client starts a check of the new
// checkout with the repository and its worktree prefixes, which makes the
// runtime also look for every landing path, and then posts the passed check
// here with the same prefixes. History keeps the paths it recorded.

// RelocateRepositoryRequest is the body of POST /api/repositories/{id}/relocate.
type RelocateRepositoryRequest struct {
	CheckoutCheckID  string                    `json:"checkout_check_id"`
	WorktreePrefixes []protocol.WorktreePrefix `json:"worktree_prefixes"`
}

// RelocatedIssueWorktree is an issue worktree after a relocate.
type RelocatedIssueWorktree struct {
	ID       string `json:"id"`
	IssueID  string `json:"issue_id"`
	DaemonID string `json:"daemon_id"`
	Path     string `json:"path"`
	State    string `json:"state"`
}

// RelocateRepositoryResponse lists the rows a relocate moved and the landing
// paths the runtime could not find.
type RelocateRepositoryResponse struct {
	Repository     RepositoryResponse       `json:"repository"`
	Worklanes      []WorklaneResponse       `json:"worklanes"`
	IssueWorktrees []RelocatedIssueWorktree `json:"issue_worktrees"`
	MissingPaths   []string                 `json:"missing_paths"`
}

// normalizeWorktreePrefixes trims and cleans each prefix and refuses one whose
// sides are not absolute or "~/" paths.
func normalizeWorktreePrefixes(raw []protocol.WorktreePrefix) ([]protocol.WorktreePrefix, error) {
	out := make([]protocol.WorktreePrefix, 0, len(raw))
	for _, prefix := range raw {
		prefix.From = strings.TrimSpace(prefix.From)
		prefix.To = strings.TrimSpace(prefix.To)
		if !workpath.ValidPrefix(prefix) {
			return nil, fmt.Errorf("worktree prefix %q=%q: both sides are an absolute path or start with ~/", prefix.From, prefix.To)
		}
		prefix.From = path.Clean(prefix.From)
		prefix.To = path.Clean(prefix.To)
		out = append(out, prefix)
	}
	return out, nil
}

// recordedWorktreePaths are the recorded paths of the repository's worklanes
// and issue worktrees, in every state.
func recordedWorktreePaths(ctx context.Context, q *db.Queries, workspaceID, repositoryID pgtype.UUID) ([]db.Worklane, []db.IssueWorktree, error) {
	lanes, err := q.ListWorklanesWithPathForRepository(ctx, db.ListWorklanesWithPathForRepositoryParams{RepositoryID: repositoryID, WorkspaceID: workspaceID})
	if err != nil {
		return nil, nil, fmt.Errorf("list worklanes: %w", err)
	}
	trees, err := q.ListIssueWorktreesForRepository(ctx, db.ListIssueWorktreesForRepositoryParams{RepositoryID: repositoryID, WorkspaceID: workspaceID})
	if err != nil {
		return nil, nil, fmt.Errorf("list issue worktrees: %w", err)
	}
	return lanes, trees, nil
}

// relocationProbe is what a relocate check asks the runtime to look for.
func (h *Handler) relocationProbe(ctx context.Context, repository db.Repository, prefixes []protocol.WorktreePrefix) (*protocol.RepositoryRelocationProbe, error) {
	lanes, trees, err := recordedWorktreePaths(ctx, h.Queries, repository.WorkspaceID, repository.ID)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(lanes)+len(trees))
	for _, lane := range lanes {
		paths = append(paths, lane.Path.String)
	}
	for _, tree := range trees {
		paths = append(paths, tree.Path)
	}
	return &protocol.RepositoryRelocationProbe{OldCheckout: repository.CheckoutPath, Prefixes: prefixes, Paths: paths}, nil
}

// relocationMissing reports whether a landing path cannot be vouched for: the
// runtime did not find it, or the check never saw the row's recorded path.
func relocationMissing(check *RuntimeRepositoryCheckoutRequest, recorded, landing string) bool {
	return slices.Contains(check.Result.MissingPaths, landing) || !slices.Contains(check.Relocation.Paths, recorded)
}

// RelocateRepository moves the repository to the checkout a passed relocate
// check found and rewrites its worklane and issue worktree paths with the
// check's prefixes (ADR-0037). It refuses while a task may work in the
// repository's folders.
func (h *Handler) RelocateRepository(w http.ResponseWriter, r *http.Request) {
	var req RelocateRepositoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	repository, ok := h.loadRepositoryInWorkspace(w, r)
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := uuidToString(repository.WorkspaceID)
	prefixes, err := normalizeWorktreePrefixes(req.WorktreePrefixes)
	if err != nil {
		writeErrorCode(w, http.StatusBadRequest, "worktree_prefix_invalid", err.Error())
		return
	}
	check, ok := h.passedCheckout(w, r, workspaceID, req.CheckoutCheckID, repository.FullName, repository.DefaultBranch)
	if !ok {
		return
	}
	if check.RepositoryID != uuidToString(repository.ID) || check.Relocation == nil || !slices.Equal(check.Relocation.Prefixes, prefixes) {
		writeErrorCode(w, http.StatusBadRequest, "relocate_check_mismatch", "the checkout check was not a relocate check of this repository with these worktree prefixes")
		return
	}
	if check.Result.Home == "" {
		writeErrorCode(w, http.StatusBadRequest, "relocate_check_mismatch", "the runtime that ran the check did not report its home folder; update its daemon")
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)
	if _, err := tx.Exec(r.Context(), `SELECT id FROM repository WHERE id = $1 AND workspace_id = $2 FOR UPDATE`, repository.ID, repository.WorkspaceID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to lock repository")
		return
	}
	active, err := qtx.CountActiveTasksForRepository(r.Context(), db.CountActiveTasksForRepositoryParams{ProjectID: repository.ProjectID, RepositoryID: repository.ID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load tasks")
		return
	}
	if active > 0 {
		writeErrorCode(w, http.StatusConflict, "repository_busy", fmt.Sprintf("%d task(s) are dispatched or running in this repository's project; relocate once they finish", active))
		return
	}

	relocation := workpath.Relocation{OldCheckout: repository.CheckoutPath, NewCheckout: check.Result.ResolvedPath, Prefixes: prefixes}
	home := check.Result.Home
	lanes, trees, err := recordedWorktreePaths(r.Context(), qtx, repository.WorkspaceID, repository.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load worktrees")
		return
	}
	resp := RelocateRepositoryResponse{Worklanes: []WorklaneResponse{}, IssueWorktrees: []RelocatedIssueWorktree{}, MissingPaths: []string{}}
	var movedLanes []db.Worklane
	for _, lane := range lanes {
		landing, moved := relocation.Apply(lane.Path.String, home)
		if !moved {
			continue
		}
		updated, err := qtx.SetWorklanePath(r.Context(), db.SetWorklanePathParams{Path: pgtype.Text{String: landing, Valid: true}, ID: lane.ID, WorkspaceID: lane.WorkspaceID})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update worklane path")
			return
		}
		movedLanes = append(movedLanes, updated)
		resp.Worklanes = append(resp.Worklanes, worklaneToResponse(updated))
		if relocationMissing(check, lane.Path.String, landing) {
			resp.MissingPaths = append(resp.MissingPaths, landing)
		}
	}
	for _, tree := range trees {
		landing, moved := relocation.Apply(tree.Path, home)
		if !moved {
			continue
		}
		updated, err := qtx.SetIssueWorktreePath(r.Context(), db.SetIssueWorktreePathParams{Path: landing, ID: tree.ID, WorkspaceID: tree.WorkspaceID})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update issue worktree path")
			return
		}
		resp.IssueWorktrees = append(resp.IssueWorktrees, RelocatedIssueWorktree{
			ID:       uuidToString(updated.ID),
			IssueID:  uuidToString(updated.IssueID),
			DaemonID: updated.DaemonID,
			Path:     updated.Path,
			State:    updated.State,
		})
		if relocationMissing(check, tree.Path, landing) {
			resp.MissingPaths = append(resp.MissingPaths, landing)
		}
	}
	updated, err := qtx.UpdateRepositoryCheckout(r.Context(), db.UpdateRepositoryCheckoutParams{
		CheckoutDaemonID: check.DaemonID,
		CheckoutPath:     check.Result.ResolvedPath,
		ID:               repository.ID,
		WorkspaceID:      repository.WorkspaceID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update repository checkout")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to relocate repository")
		return
	}

	resp.Repository = repositoryToResponse(updated)
	h.publish(protocol.EventRepositoryUpdated, workspaceID, "member", userID, protocol.RepositoryEventPayload{
		RepositoryID: resp.Repository.ID,
		ProjectID:    resp.Repository.ProjectID,
	})
	for _, lane := range movedLanes {
		h.publishWorklaneEvent(protocol.EventWorklaneUpdated, workspaceID, "member", userID, lane)
	}
	writeJSON(w, http.StatusOK, resp)
}
