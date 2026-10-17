package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type ReleaseMilestoneResponse struct {
	ID        string `json:"id"`
	ReleaseID string `json:"release_id"`
	Name      string `json:"name"`
	Date      string `json:"date"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func milestoneToResponse(row db.ReleaseMilestone) ReleaseMilestoneResponse {
	return ReleaseMilestoneResponse{
		ID: uuidToString(row.ID), ReleaseID: uuidToString(row.ReleaseID),
		Name: row.Name, Date: row.Date.Time.Format(time.DateOnly),
		CreatedAt: timestampToString(row.CreatedAt), UpdatedAt: timestampToString(row.UpdatedAt),
	}
}

func (h *Handler) GetRelease(w http.ResponseWriter, r *http.Request) {
	rel, ok := h.loadRelease(w, r)
	if !ok {
		return
	}
	rows, err := h.releasesToResponse(r.Context(), h.Queries, rel.WorkspaceID, []db.Release{rel})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read release milestones")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		ReleaseResponse
		Release ReleaseResponse `json:"release"`
	}{ReleaseResponse: rows[0], Release: rows[0]})
}

func (h *Handler) AddReleaseMilestone(w http.ResponseWriter, r *http.Request) {
	h.writeReleaseMilestone(w, r, "add")
}
func (h *Handler) EditReleaseMilestone(w http.ResponseWriter, r *http.Request) {
	h.writeReleaseMilestone(w, r, "edit")
}
func (h *Handler) RemoveReleaseMilestone(w http.ResponseWriter, r *http.Request) {
	h.writeReleaseMilestone(w, r, "remove")
}

// Milestone writes share the release line's repository lock. Re-read the parent
// after locking so a concurrent release removal cannot leave an orphan.
func (h *Handler) writeReleaseMilestone(w http.ResponseWriter, r *http.Request, action string) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	ws, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace id")
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "id")
	if !ok {
		return
	}
	var req struct {
		Name *string `json:"name"`
		Date *string `json:"date"`
	}
	if action != "remove" && json.NewDecoder(r.Body).Decode(&req) != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ctx := r.Context()
	releaseID := id
	if action != "add" {
		milestone, err := h.Queries.GetReleaseMilestoneInWorkspace(ctx, db.GetReleaseMilestoneInWorkspaceParams{ID: id, WorkspaceID: ws})
		if err != nil {
			writeMilestoneReadError(w, err, "milestone")
			return
		}
		releaseID = milestone.ReleaseID
	}
	rel, err := h.Queries.GetReleaseInWorkspace(ctx, db.GetReleaseInWorkspaceParams{ID: releaseID, WorkspaceID: ws})
	if err != nil {
		writeMilestoneReadError(w, err, "release")
		return
	}
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		writeError(w, 500, "failed to start transaction")
		return
	}
	defer tx.Rollback(ctx)
	q := h.Queries.WithTx(tx)
	if _, err = q.LockReleaseMilestoneOwners(ctx, db.LockReleaseMilestoneOwnersParams{ID: rel.ID, WorkspaceID: ws}); err != nil {
		writeMilestoneReadError(w, err, "release")
		return
	}
	if _, err = q.LockRepositoryForLine(ctx, db.LockRepositoryForLineParams{ID: rel.RepositoryID, WorkspaceID: ws}); err != nil {
		writeMilestoneReadError(w, err, "repository")
		return
	}
	if _, err = q.GetReleaseInWorkspace(ctx, db.GetReleaseInWorkspaceParams{ID: rel.ID, WorkspaceID: ws}); err != nil {
		writeMilestoneReadError(w, err, "release")
		return
	}
	var name, date string
	if action != "add" {
		row, err := q.GetReleaseMilestoneInWorkspace(ctx, db.GetReleaseMilestoneInWorkspaceParams{ID: id, WorkspaceID: ws})
		if err != nil {
			writeMilestoneReadError(w, err, "milestone")
			return
		}
		name, date = row.Name, row.Date.Time.Format(time.DateOnly)
	}
	if action != "remove" {
		if req.Name != nil {
			name = strings.TrimSpace(*req.Name)
		}
		if req.Date != nil {
			date = *req.Date
		}
		if name == "" {
			writeError(w, 400, "name is required")
			return
		}
		day, err := time.Parse(time.DateOnly, date)
		if err != nil || day.Format(time.DateOnly) != date {
			writeError(w, 400, "date must be a calendar day in YYYY-MM-DD format")
			return
		}
		if action == "add" {
			_, err = q.CreateReleaseMilestone(ctx, db.CreateReleaseMilestoneParams{WorkspaceID: ws, ReleaseID: rel.ID, Name: name, Date: pgtype.Date{Time: day, Valid: true}})
		} else {
			_, err = q.UpdateReleaseMilestone(ctx, db.UpdateReleaseMilestoneParams{ID: id, WorkspaceID: ws, Name: name, Date: pgtype.Date{Time: day, Valid: true}})
		}
		if err != nil {
			writeError(w, 500, "failed to write milestone")
			return
		}
	} else if err = q.DeleteReleaseMilestone(ctx, db.DeleteReleaseMilestoneParams{ID: id, WorkspaceID: ws}); err != nil {
		writeError(w, 500, "failed to delete milestone")
		return
	}
	line, err := q.ListReleaseLine(ctx, db.ListReleaseLineParams{WorkspaceID: ws, RepositoryID: rel.RepositoryID})
	if err != nil {
		writeError(w, 500, "failed to read release line")
		return
	}
	responses, err := h.releasesToResponse(ctx, q, ws, line)
	if err != nil {
		writeError(w, 500, "failed to read release milestones")
		return
	}
	if tx.Commit(ctx) != nil {
		writeError(w, 500, "failed to commit milestone")
		return
	}
	actorType, actorID := h.resolveActor(r, userID, uuidToString(ws))
	h.publish(protocol.EventReleaseLineUpdated, uuidToString(ws), actorType, actorID, protocol.ReleaseLineEventPayload{RepositoryID: uuidToString(rel.RepositoryID)})
	status := http.StatusOK
	if action == "add" {
		status = http.StatusCreated
	}
	writeJSON(w, status, map[string]any{"releases": responses})
}

func writeMilestoneReadError(w http.ResponseWriter, err error, resource string) {
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, resource+" not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "failed to load "+resource)
}
