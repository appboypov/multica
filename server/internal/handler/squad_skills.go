package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/logger"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// A squad carries the same skill model as an agent: workspace skills it is
// assigned, each with an enabled flag, plus a runtime skill section switch
// and the runtime skills that differ from it. A squad has no runtime of its
// own, so a runtime skill change names its runtime explicitly.

type squadSkillIDsRequest struct {
	SkillIDs []string `json:"skill_ids"`
}

// loadManageableSquad loads the URL's squad for a member who may change it.
func (h *Handler) loadManageableSquad(w http.ResponseWriter, r *http.Request) (db.Squad, string, bool) {
	workspaceID := workspaceIDFromURL(r, "workspaceId")
	member, ok := h.requireWorkspaceMember(w, r, workspaceID, "workspace not found")
	if !ok {
		return db.Squad{}, "", false
	}
	squad, _, ok := h.loadSquadInWorkspace(w, r)
	if !ok {
		return db.Squad{}, "", false
	}
	if !canManageSquad(member, squad) {
		writeError(w, http.StatusForbidden, "insufficient permissions")
		return db.Squad{}, "", false
	}
	return squad, workspaceID, true
}

func (h *Handler) ListSquadSkills(w http.ResponseWriter, r *http.Request) {
	squad, _, ok := h.loadSquadInWorkspace(w, r)
	if !ok {
		return
	}
	resp, err := h.squadSkillSummaries(r, squad)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list squad skills")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// SetSquadSkills replaces the squad's assigned workspace skills.
func (h *Handler) SetSquadSkills(w http.ResponseWriter, r *http.Request) {
	h.writeSquadSkills(w, r, true)
}

// AddSquadSkills assigns workspace skills to the squad next to the ones it has.
func (h *Handler) AddSquadSkills(w http.ResponseWriter, r *http.Request) {
	h.writeSquadSkills(w, r, false)
}

func (h *Handler) writeSquadSkills(w http.ResponseWriter, r *http.Request, replace bool) {
	squad, workspaceID, ok := h.loadManageableSquad(w, r)
	if !ok {
		return
	}
	var req squadSkillIDsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	skillUUIDs, ok := parseUUIDSliceOrBadRequest(w, req.SkillIDs, "skill_ids")
	if !ok {
		return
	}
	if !h.validateSkillIDsInWorkspace(w, r, squad.WorkspaceID, skillUUIDs) {
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)
	if replace {
		if err := qtx.RemoveAllSquadSkills(r.Context(), squad.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to clear squad skills")
			return
		}
	}
	for _, skillID := range skillUUIDs {
		if err := qtx.AddSquadSkill(r.Context(), db.AddSquadSkillParams{SquadID: squad.ID, SkillID: skillID}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to add squad skill")
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit")
		return
	}
	h.writeUpdatedSquadSkills(w, r, squad, workspaceID)
}

func (h *Handler) SetSquadSkillEnabled(w http.ResponseWriter, r *http.Request) {
	squad, workspaceID, ok := h.loadManageableSquad(w, r)
	if !ok {
		return
	}
	skillID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "skillId"), "skill_id")
	if !ok {
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Enabled == nil {
		writeError(w, http.StatusBadRequest, "enabled is required")
		return
	}
	rows, err := h.Queries.SetSquadSkillEnabled(r.Context(), db.SetSquadSkillEnabledParams{
		SquadID: squad.ID,
		SkillID: skillID,
		Enabled: *req.Enabled,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update squad skill")
		return
	}
	if rows == 0 {
		writeError(w, http.StatusNotFound, "squad skill not found")
		return
	}
	h.writeUpdatedSquadSkills(w, r, squad, workspaceID)
}

func (h *Handler) RemoveSquadSkill(w http.ResponseWriter, r *http.Request) {
	squad, workspaceID, ok := h.loadManageableSquad(w, r)
	if !ok {
		return
	}
	skillID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "skillId"), "skill_id")
	if !ok {
		return
	}
	if err := h.Queries.RemoveSquadSkill(r.Context(), db.RemoveSquadSkillParams{SquadID: squad.ID, SkillID: skillID}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove squad skill")
		return
	}
	h.writeUpdatedSquadSkills(w, r, squad, workspaceID)
}

func (h *Handler) squadSkillSummaries(r *http.Request, squad db.Squad) ([]SkillSummaryResponse, error) {
	skills, err := h.Queries.ListSquadSkillSummaries(r.Context(), squad.ID)
	if err != nil {
		return nil, err
	}
	resp := make([]SkillSummaryResponse, len(skills))
	for i, s := range skills {
		resp[i] = skillSummaryToResponse(
			s.ID, s.WorkspaceID, s.Name, s.Description, s.Config,
			s.CreatedBy, s.CreatedAt, s.UpdatedAt,
		)
		resp[i].Enabled = &s.Enabled
	}
	return resp, nil
}

func (h *Handler) writeUpdatedSquadSkills(w http.ResponseWriter, r *http.Request, squad db.Squad, workspaceID string) {
	resp, err := h.squadSkillSummaries(r, squad)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list squad skills")
		return
	}
	h.publish(protocol.EventSquadUpdated, workspaceID, "member", requestUserID(r), map[string]any{
		"squad_id": uuidToString(squad.ID),
		"skills":   resp,
	})
	writeJSON(w, http.StatusOK, resp)
}

// SetSquadRuntimeSkillEnabled turns one skill of a named local runtime on or
// off for the squad. The skill is an exception exactly when its state differs
// from the squad's section switch. The runtime must be one the caller may
// read: another member's private machine answers not-found, as everywhere.
func (h *Handler) SetSquadRuntimeSkillEnabled(w http.ResponseWriter, r *http.Request) {
	squad, workspaceID, ok := h.loadManageableSquad(w, r)
	if !ok {
		return
	}
	var req runtimeSkillEnabledRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Enabled == nil {
		writeError(w, http.StatusBadRequest, "runtime_id, root, key, and enabled are required")
		return
	}
	rt, _, ok := h.requireRuntimeReadAccess(w, r, obsmetrics.RuntimeLookupSourceRuntimeAPI, req.RuntimeID)
	if !ok {
		return
	}
	if rt.WorkspaceID != squad.WorkspaceID {
		writeError(w, http.StatusNotFound, "runtime not found")
		return
	}
	target, ok := runtimeSkillTarget(w, rt, req.runtimeSkillIdentity)
	if !ok {
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to begin transaction")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)
	locked, err := qtx.LockSquadForUpdate(r.Context(), db.LockSquadForUpdateParams{ID: squad.ID, WorkspaceID: squad.WorkspaceID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load squad")
		return
	}
	payload, err := withRuntimeSkillState(locked.RuntimeSkillExceptions, target, *req.Enabled, locked.InheritRuntimeSkills)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode runtime skill settings")
		return
	}
	updated, err := qtx.UpdateSquadRuntimeSkillExceptions(r.Context(), db.UpdateSquadRuntimeSkillExceptionsParams{
		ID:                     locked.ID,
		RuntimeSkillExceptions: payload,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update runtime skill")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit")
		return
	}
	h.broadcastSquadRuntimeSkills(r, updated, workspaceID)
	w.WriteHeader(http.StatusNoContent)
}

// SetSquadRuntimeSkillsInherited sets every runtime skill of the squad on or
// off, which also clears its per-skill exceptions.
func (h *Handler) SetSquadRuntimeSkillsInherited(w http.ResponseWriter, r *http.Request) {
	squad, workspaceID, ok := h.loadManageableSquad(w, r)
	if !ok {
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Enabled == nil {
		writeError(w, http.StatusBadRequest, "enabled is required")
		return
	}
	updated, err := h.Queries.UpdateSquadInheritRuntimeSkills(r.Context(), db.UpdateSquadInheritRuntimeSkillsParams{
		ID:                   squad.ID,
		InheritRuntimeSkills: *req.Enabled,
	})
	if err != nil {
		slog.Error("squad runtime skill section: update failed",
			append(logger.RequestAttrs(r), "error", err, "squad_id", uuidToString(squad.ID))...)
		writeError(w, http.StatusInternalServerError, "failed to update runtime skill section")
		return
	}
	h.broadcastSquadRuntimeSkills(r, updated, workspaceID)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) broadcastSquadRuntimeSkills(r *http.Request, squad db.Squad, workspaceID string) {
	resp, err := h.squadToResponseWithPreview(r.Context(), squad)
	if err != nil {
		slog.Warn("squad runtime skill update: load member preview for broadcast failed",
			append(logger.RequestAttrs(r), "error", err, "squad_id", uuidToString(squad.ID))...)
	}
	h.publish(protocol.EventSquadUpdated, workspaceID, "member", requestUserID(r), map[string]any{"squad": resp})
}
