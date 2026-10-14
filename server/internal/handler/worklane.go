package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/worklanehead"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Worklane activity actions (ADR-0008). Entries are activity_log rows with
// worklane_id set and issue_id NULL; details carry {name, runtime_name} on
// worklane_created and {from, to} on worklane_renamed. Issue actions stay
// string literals in their listeners.
const (
	worklaneActionCreated = "worklane_created"
	worklaneActionRenamed = "worklane_renamed"
)

// WorklaneIssueRef names an issue of a worklane.
type WorklaneIssueRef struct {
	ID         string `json:"id"`
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
	Status     string `json:"status"`
}

type WorklaneResponse struct {
	ID           string            `json:"id"`
	WorkspaceID  string            `json:"workspace_id"`
	RepositoryID string            `json:"repository_id"`
	Name         string            `json:"name"`
	Folder       *string           `json:"folder"`
	DaemonID     *string           `json:"daemon_id"`
	Path         *string           `json:"path"`
	IsMain       bool              `json:"is_main"`
	Position     int32             `json:"position"`
	State        string            `json:"state"`
	Branch       *string           `json:"branch"`
	ActiveIssue  *WorklaneIssueRef `json:"active_issue"`
	NextIssue    *WorklaneIssueRef `json:"next_issue"`
	CreatedAt    string            `json:"created_at"`
	UpdatedAt    string            `json:"updated_at"`
}

func worklaneToResponse(w db.Worklane) WorklaneResponse {
	return WorklaneResponse{
		ID:           uuidToString(w.ID),
		WorkspaceID:  uuidToString(w.WorkspaceID),
		RepositoryID: uuidToString(w.RepositoryID),
		Name:         w.Name,
		Folder:       textToPtr(w.Folder),
		DaemonID:     textToPtr(w.DaemonID),
		Path:         textToPtr(w.Path),
		IsMain:       w.IsMain,
		Position:     w.Position,
		State:        worklanehead.StateFree,
		Branch:       textToPtr(w.Branch),
		CreatedAt:    timestampToString(w.CreatedAt),
		UpdatedAt:    timestampToString(w.UpdatedAt),
	}
}

// worklaneResponses is the single loader for every worklane response.
func (h *Handler) worklaneResponses(ctx context.Context, workspaceID pgtype.UUID, lanes []db.Worklane) ([]WorklaneResponse, error) {
	ids := make([]pgtype.UUID, len(lanes))
	for i := range lanes {
		ids[i] = lanes[i].ID
	}
	rows, err := h.Queries.ListOpenWorklaneIssues(ctx, db.ListOpenWorklaneIssuesParams{WorkspaceID: workspaceID, WorklaneIds: ids})
	if err != nil {
		return nil, err
	}
	issues := worklanehead.FromRows(rows)
	out := make([]WorklaneResponse, len(lanes))
	ref := func(issue *worklanehead.Issue) *WorklaneIssueRef {
		if issue == nil {
			return nil
		}
		return &WorklaneIssueRef{ID: issue.ID, Identifier: issue.Identifier, Title: issue.Title, Status: issue.Status}
	}
	for i, lane := range lanes {
		out[i] = worklaneToResponse(lane)
		head := worklanehead.Head(lane.Branch.String, issues[uuidToString(lane.ID)])
		out[i].State, out[i].ActiveIssue, out[i].NextIssue = head.State, ref(head.Active), ref(head.Next)
	}
	return out, nil
}

func (h *Handler) writeWorklaneResponse(w http.ResponseWriter, ctx context.Context, lane db.Worklane) {
	rows, err := h.worklaneResponses(ctx, lane.WorkspaceID, []db.Worklane{lane})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load worklane issues")
		return
	}
	writeJSON(w, http.StatusOK, rows[0])
}

func (h *Handler) publishWorklaneUpdated(ctx context.Context, lane db.Worklane, actorType, actorID string) {
	rows, err := h.worklaneResponses(ctx, lane.WorkspaceID, []db.Worklane{lane})
	if err != nil {
		slog.Error("load worklane update failed", "worklane_id", uuidToString(lane.ID), "error", err)
		return
	}
	h.publish(protocol.EventWorklaneUpdated, uuidToString(lane.WorkspaceID), actorType, actorID,
		worklaneUpdatedPayload{WorklaneEventPayload: protocol.WorklaneEventPayload{WorklaneID: uuidToString(lane.ID), RepositoryID: uuidToString(lane.RepositoryID)}, Worklane: rows[0]})
}

// worklaneUpdatedPayload is worklane:updated: the ids and the worklane.
type worklaneUpdatedPayload struct {
	protocol.WorklaneEventPayload
	Worklane WorklaneResponse `json:"worklane"`
}

// publishWorklaneEvent publishes worklane:created, worklane:updated or
// worklane:deleted; updated also carries the worklane response.
func (h *Handler) publishWorklaneEvent(eventType, workspaceID, actorType, actorID string, w db.Worklane) {
	ids := protocol.WorklaneEventPayload{WorklaneID: uuidToString(w.ID), RepositoryID: uuidToString(w.RepositoryID)}
	if eventType == protocol.EventWorklaneUpdated {
		h.publishWorklaneUpdated(context.Background(), w, actorType, actorID)
		return
	}
	h.publish(eventType, workspaceID, actorType, actorID, ids)
}

// repositoryWorklanes is a repository's worklanes, main first.
func (h *Handler) repositoryWorklanes(ctx context.Context, workspaceID, repositoryID pgtype.UUID) ([]db.Worklane, error) {
	all, err := h.Queries.ListWorklanes(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]db.Worklane, 0, len(all))
	for _, wl := range all {
		if wl.RepositoryID == repositoryID {
			out = append(out, wl)
		}
	}
	return out, nil
}

// ListWorklanes is every worklane of the workspace, each repository's main
// first.
func (h *Handler) ListWorklanes(w http.ResponseWriter, r *http.Request) {
	wsUUID, ok := parseUUIDOrBadRequest(w, ctxWorkspaceID(r.Context()), "workspace id")
	if !ok {
		return
	}
	rows, err := h.Queries.ListWorklanes(r.Context(), wsUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list worklanes")
		return
	}
	out, err := h.worklaneResponses(r.Context(), wsUUID, rows)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load worklane issues")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"worklanes": out, "total": len(out)})
}

// loadWorkspaceWorklane reads the worklane of the URL in the caller's
// workspace, answering 404 when it is not there.
func (h *Handler) loadWorkspaceWorklane(w http.ResponseWriter, r *http.Request) (db.Worklane, pgtype.UUID, bool) {
	wsUUID, ok := parseUUIDOrBadRequest(w, ctxWorkspaceID(r.Context()), "workspace id")
	if !ok {
		return db.Worklane{}, wsUUID, false
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "worklane id")
	if !ok {
		return db.Worklane{}, wsUUID, false
	}
	wl, err := h.Queries.GetWorklaneInWorkspace(r.Context(), db.GetWorklaneInWorkspaceParams{ID: id, WorkspaceID: wsUUID})
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "worklane not found")
		} else {
			writeError(w, http.StatusInternalServerError, "failed to load worklane")
		}
		return db.Worklane{}, wsUUID, false
	}
	return wl, wsUUID, true
}

func (h *Handler) GetWorklane(w http.ResponseWriter, r *http.Request) {
	wl, _, ok := h.loadWorkspaceWorklane(w, r)
	if !ok {
		return
	}
	h.writeWorklaneResponse(w, r.Context(), wl)
}

// RenameWorklane saves a new name with its worklane_renamed entry. main keeps
// its name.
func (h *Handler) RenameWorklane(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	wl, wsUUID, ok := h.loadWorkspaceWorklane(w, r)
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	name := strings.TrimSpace(body.Name)
	if wl.IsMain {
		writeWorklaneRefusal(w, http.StatusBadRequest, "main_fixed", "main cannot be renamed", "")
		return
	}
	if name == "" {
		writeWorklaneRefusal(w, http.StatusBadRequest, "name_required", "a worklane needs a name", "")
		return
	}
	if name == wl.Name {
		h.writeWorklaneResponse(w, ctx, wl)
		return
	}
	siblings, err := h.repositoryWorklanes(ctx, wsUUID, wl.RepositoryID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load worklanes")
		return
	}
	for _, other := range siblings {
		if other.ID != wl.ID && strings.EqualFold(other.Name, name) {
			writeWorklaneRefusal(w, http.StatusConflict, "name_taken", fmt.Sprintf("%s is taken in this repository", other.Name), other.Name)
			return
		}
	}
	userID := requestUserID(r)
	var actorID pgtype.UUID
	if u, err := util.ParseUUID(userID); err == nil {
		actorID = u
	}
	details, err := json.Marshal(map[string]string{"from": wl.Name, "to": name})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to rename worklane")
		return
	}

	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(ctx)
	qtx := h.Queries.WithTx(tx)
	renamed, err := qtx.RenameWorklane(ctx, db.RenameWorklaneParams{Name: name, ID: wl.ID, WorkspaceID: wsUUID})
	if err != nil {
		if isUniqueViolation(err) {
			writeWorklaneRefusal(w, http.StatusConflict, "name_taken", fmt.Sprintf("%s is taken in this repository", name), name)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to rename worklane")
		return
	}
	if _, err := qtx.CreateWorklaneActivity(ctx, db.CreateWorklaneActivityParams{
		WorkspaceID:  wsUUID,
		WorklaneID:   wl.ID,
		RepositoryID: wl.RepositoryID,
		ActorType:    strToText("member"),
		ActorID:      actorID,
		Action:       worklaneActionRenamed,
		Details:      details,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to rename worklane")
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to rename worklane")
		return
	}
	h.publishWorklaneEvent(protocol.EventWorklaneUpdated, uuidToString(wsUUID), "member", userID, renamed)
	h.writeWorklaneResponse(w, ctx, renamed)
}

// RemoveWorklane asks the worklane's machine to remove its worktree. The
// worklane goes once the daemon reports it removed (design decision 4).
func (h *Handler) RemoveWorklane(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	wl, wsUUID, ok := h.loadWorkspaceWorklane(w, r)
	if !ok {
		return
	}
	if wl.IsMain {
		writeWorklaneRefusal(w, http.StatusBadRequest, "main_fixed", "main cannot be removed", "")
		return
	}
	daemonID := wl.DaemonID.String
	machine, err := h.loadWorklaneMachine(ctx, wsUUID, daemonID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load runtimes")
		return
	}
	if !machine.online {
		writeWorklaneRefusal(w, http.StatusConflict, "runtime_offline", fmt.Sprintf("%s is offline", machine.name), machine.name)
		return
	}
	if !runtimeHasCapability(machine.runtime.Metadata, protocol.DaemonCapabilityWorklaneWorktreeV1) {
		writeWorklaneRefusal(w, http.StatusConflict, "runtime_cannot_make_worktrees", fmt.Sprintf("%s cannot remove worktrees yet: update its Multica", machine.name), machine.name)
		return
	}
	repo, err := h.Queries.GetRepositoryInWorkspace(ctx, db.GetRepositoryInWorkspaceParams{ID: wl.RepositoryID, WorkspaceID: wsUUID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load repository")
		return
	}
	worklaneID := uuidToString(wl.ID)
	createdBy := requestUserID(r)
	if h.refuseWorklaneWithOpenIssues(w,ctx,h.Queries,wsUUID,wl.ID) { return }
	req := &WorklaneRequest{
		ID:            randomID(),
		Kind:          protocol.WorklaneRequestKindRemove,
		WorkspaceID:   uuidToString(wsUUID),
		RepositoryID:  uuidToString(wl.RepositoryID),
		WorklaneID:    &worklaneID,
		Name:          wl.Name,
		Folder:        wl.Folder.String,
		Path:          wl.Path.String,
		Base:          "origin/" + repo.DefaultBranch,
		DaemonID:      daemonID,
		RuntimeName:   machine.name,
		Status:        worklaneRequestRunning,
		Steps:         []WorklaneRequestStep{},
		CreatedBy:     &createdBy,
		FullName:      repo.FullName,
		DefaultBranch: repo.DefaultBranch,
		CheckoutPath:  repo.CheckoutPath,
		CreatedAt:     time.Now(),
	}
	if err := h.enqueueWorklaneRequest(ctx, req, uuidToString(machine.runtime.ID)); err != nil {
		slog.Error("worklane remove enqueue failed", "error", err, "worklane_id", worklaneID)
		writeError(w, http.StatusInternalServerError, "failed to remove worklane")
		return
	}
	writeJSON(w, http.StatusAccepted, req)
}

type WorklaneActivityEntryResponse struct {
	ID        string          `json:"id"`
	ActorType string          `json:"actor_type"`
	ActorID   *string         `json:"actor_id"`
	Action    string          `json:"action"`
	Details   json.RawMessage `json:"details"`
	CreatedAt string          `json:"created_at"`
}

// ListWorklaneActivity is the newest 200 entries of the worklane and the
// repository-wide entries since it was made, oldest first (ADR-0008).
func (h *Handler) ListWorklaneActivity(w http.ResponseWriter, r *http.Request) {
	wl, wsUUID, ok := h.loadWorkspaceWorklane(w, r)
	if !ok {
		return
	}
	rows, err := h.Queries.ListActivitiesForWorklane(r.Context(), db.ListActivitiesForWorklaneParams{
		WorkspaceID:       wsUUID,
		WorklaneID:        wl.ID,
		RepositoryID:      wl.RepositoryID,
		WorklaneCreatedAt: wl.CreatedAt,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list worklane activity")
		return
	}
	entries := make([]WorklaneActivityEntryResponse, 0, len(rows))
	for _, row := range rows {
		details := json.RawMessage(row.Details)
		if len(details) == 0 || !json.Valid(details) {
			details = json.RawMessage("{}")
		}
		entries = append(entries, WorklaneActivityEntryResponse{
			ID:        uuidToString(row.ID),
			ActorType: row.ActorType.String,
			ActorID:   uuidToPtr(row.ActorID),
			Action:    row.Action,
			Details:   details,
			CreatedAt: timestampToString(row.CreatedAt),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}
