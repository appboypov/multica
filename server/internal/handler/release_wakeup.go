package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) ListReleaseWakeups(w http.ResponseWriter, r *http.Request) {
	rel, ok := h.loadRelease(w, r)
	if !ok {
		return
	}
	if _, ok := h.workspaceMember(w, r, uuidToString(rel.WorkspaceID)); !ok {
		return
	}
	rows, err := h.Queries.ListReleaseWakeups(r.Context(), db.ListReleaseWakeupsParams{WorkspaceID: rel.WorkspaceID, ReleaseID: rel.ID})
	if err != nil {
		wakeupError(w, err)
		return
	}
	agent, _, ownerErr := service.ResolveReleaseTaskOwner(r.Context(), h.Queries, rel)
	if ownerErr != nil && !errors.Is(ownerErr, service.ErrReleaseOwnerUnavailable) {
		wakeupError(w, ownerErr)
		return
	}
	type releaseWakeupResponse struct {
		db.IssueWakeup
		AgentID   string          `json:"agent_id"`
		AgentName string          `json:"agent_name"`
		Condition json.RawMessage `json:"condition"`
	}
	out := make([]releaseWakeupResponse, len(rows))
	for i, row := range rows {
		out[i] = releaseWakeupResponse{IssueWakeup: row.IssueWakeup, AgentID: uuidToString(agent.ID), AgentName: agent.Name, Condition: json.RawMessage(row.IssueWakeup.Condition)}
	}
	writeJSON(w, 200, out)
}

func (h *Handler) CreateReleaseWakeup(w http.ResponseWriter, r *http.Request) {
	rel, ok := h.loadRelease(w, r)
	if !ok {
		return
	}
	var in service.WakeupInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&in) != nil {
		writeError(w, 400, "invalid wakeup body")
		return
	}
	actorType, actorID := h.resolveActor(r, requestUserID(r), uuidToString(rel.WorkspaceID))
	originator := h.invokeOriginatorFromRequest(r, actorType, actorID)
	if originator == "" {
		writeError(w, 403, "a human originator is required")
		return
	}
	rule, err := (&service.IssueWakeupService{Tasks: h.TaskService}).CreateRelease(r.Context(), rel, parseUUID(originator), h.wakeupSourceTaskID(r), in)
	if err != nil {
		wakeupError(w, err)
		return
	}
	writeJSON(w, 201, rule)
}

func (h *Handler) DeleteReleaseWakeup(w http.ResponseWriter, r *http.Request) {
	rel, ok := h.loadRelease(w, r)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "wakeupId"), "wakeup id")
	if !ok {
		return
	}
	actorType, actorID := h.resolveActor(r, requestUserID(r), uuidToString(rel.WorkspaceID))
	originator := h.invokeOriginatorFromRequest(r, actorType, actorID)
	if originator == "" {
		writeError(w, 403, "a human originator is required")
		return
	}
	err := (&service.IssueWakeupService{Tasks: h.TaskService}).DeleteRelease(r.Context(), rel, id, parseUUID(originator))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, 404, "wakeup not found")
		return
	}
	if err != nil {
		wakeupError(w, err)
		return
	}
	w.WriteHeader(204)
}
