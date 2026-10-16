package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/middleware"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// RegisterAgentSessionRequest describes a process followed by its local daemon.
type RegisterAgentSessionRequest struct {
	WorkspaceID      string    `json:"workspace_id"`
	DaemonID         string    `json:"daemon_id"`
	AgentID          string    `json:"agent_id"`
	PID              int32     `json:"pid"`
	ProcessStartedAt time.Time `json:"process_started_at"`
	HerdrPaneID      *string   `json:"herdr_pane_id"`
	WorkDir          string    `json:"work_dir"`
	Repository       *string   `json:"repository"`
	Branch           *string   `json:"branch"`
	Status           string    `json:"status"`
}

type UpdateAgentSessionRequest struct {
	WorkspaceID string  `json:"workspace_id"`
	DaemonID    string  `json:"daemon_id"`
	Status      string  `json:"status"`
	HerdrPaneID *string `json:"herdr_pane_id,omitempty"`
	Repository  *string `json:"repository"`
	Branch      *string `json:"branch"`
}

// requireAgentSessionDaemon preserves the DaemonAuth PAT/JWT path: those
// callers supply their machine id; an mdt_ token's machine id is authoritative.
func (h *Handler) requireAgentSessionDaemon(w http.ResponseWriter, r *http.Request, workspaceID, daemonID string) (pgtype.UUID, string, bool) {
	wsID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return pgtype.UUID{}, "", false
	}
	if !h.requireDaemonWorkspaceAccess(w, r, uuidToString(wsID)) {
		return pgtype.UUID{}, "", false
	}
	daemonID = strings.TrimSpace(daemonID)
	if tokenID := middleware.DaemonIDFromContext(r.Context()); tokenID != "" {
		if daemonID != "" && daemonID != tokenID {
			writeError(w, http.StatusForbidden, "daemon_id does not match token")
			return pgtype.UUID{}, "", false
		}
		daemonID = tokenID
	}
	if daemonID == "" {
		writeError(w, http.StatusBadRequest, "daemon_id is required")
		return pgtype.UUID{}, "", false
	}
	return wsID, daemonID, true
}

func validAgentSessionStatus(status string) bool {
	switch status {
	case "idle", "working", "blocked", "done", "unknown":
		return true
	default:
		return false
	}
}

func (h *Handler) RegisterAgentSession(w http.ResponseWriter, r *http.Request) {
	var req RegisterAgentSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	wsID, daemonID, ok := h.requireAgentSessionDaemon(w, r, req.WorkspaceID, req.DaemonID)
	if !ok {
		return
	}
	agentID, ok := parseUUIDOrBadRequest(w, req.AgentID, "agent_id")
	if !ok {
		return
	}
	if req.Status == "" {
		req.Status = "unknown"
	}
	if req.PID <= 0 || req.ProcessStartedAt.IsZero() || strings.TrimSpace(req.WorkDir) == "" || !validAgentSessionStatus(req.Status) {
		writeError(w, http.StatusBadRequest, "pid, process_started_at, work_dir and a valid status are required")
		return
	}
	if _, err := h.Queries.GetAgentInWorkspace(r.Context(), db.GetAgentInWorkspaceParams{ID: agentID, WorkspaceID: wsID}); err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "agent not found")
		} else {
			slog.Warn("load session agent failed", "agent_id", req.AgentID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to load agent")
		}
		return
	}
	session, err := h.Queries.UpsertAgentSession(r.Context(), db.UpsertAgentSessionParams{
		WorkspaceID: wsID, AgentID: agentID, DaemonID: daemonID,
		Pid: req.PID, ProcessStartedAt: pgtype.Timestamptz{Time: req.ProcessStartedAt, Valid: true},
		HerdrPaneID: ptrToText(req.HerdrPaneID), WorkDir: req.WorkDir,
		Repository: ptrToText(req.Repository), Branch: ptrToText(req.Branch), Status: req.Status,
	})
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "session not found")
		} else {
			slog.Warn("register agent session failed", "agent_id", req.AgentID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to register session")
		}
		return
	}
	h.publish(protocol.EventAgentSessions, uuidToString(wsID), "system", daemonID, protocol.AgentSessionsEventPayload{AgentID: uuidToString(session.AgentID)})
	writeJSON(w, http.StatusCreated, map[string]string{"id": uuidToString(session.ID)})
}

func (h *Handler) UpdateAgentSession(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "session_id")
	if !ok {
		return
	}
	var req UpdateAgentSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	wsID, daemonID, ok := h.requireAgentSessionDaemon(w, r, req.WorkspaceID, req.DaemonID)
	if !ok {
		return
	}
	if !validAgentSessionStatus(req.Status) {
		writeError(w, http.StatusBadRequest, "invalid session status")
		return
	}
	session, err := h.Queries.UpdateAgentSession(r.Context(), db.UpdateAgentSessionParams{
		ID: id, WorkspaceID: wsID, DaemonID: daemonID,
		Status: req.Status, Repository: ptrToText(req.Repository), Branch: ptrToText(req.Branch),
		HerdrPaneID: ptrToText(req.HerdrPaneID),
	})
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "session not found")
		} else {
			slog.Warn("update agent session failed", "session_id", uuidToString(id), "error", err)
			writeError(w, http.StatusInternalServerError, "failed to update session")
		}
		return
	}
	h.publish(protocol.EventAgentSessions, uuidToString(wsID), "system", daemonID, protocol.AgentSessionsEventPayload{AgentID: uuidToString(session.AgentID)})
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) EndAgentSession(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "session_id")
	if !ok {
		return
	}
	wsID, daemonID, ok := h.requireAgentSessionDaemon(w, r, r.URL.Query().Get("workspace_id"), r.URL.Query().Get("daemon_id"))
	if !ok {
		return
	}
	// Distinguish a harmless retry from an attempt to end another daemon's
	// session. Keep the tenant guard here as well as in the actual DELETE.
	var owner string
	err := h.DB.QueryRow(r.Context(), `SELECT daemon_id FROM agent_session WHERE id = $1 AND workspace_id = $2`, id, wsID).Scan(&owner)
	if err != nil {
		if isNotFound(err) {
			w.WriteHeader(http.StatusNoContent)
		} else {
			slog.Warn("load agent session failed", "session_id", uuidToString(id), "error", err)
			writeError(w, http.StatusInternalServerError, "failed to load session")
		}
		return
	}
	if owner != daemonID {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	agentID, err := h.Queries.DeleteAgentSession(r.Context(), db.DeleteAgentSessionParams{ID: id, WorkspaceID: wsID, DaemonID: daemonID})
	if err != nil && !isNotFound(err) {
		slog.Warn("end agent session failed", "session_id", uuidToString(id), "error", err)
		writeError(w, http.StatusInternalServerError, "failed to end session")
		return
	}
	if err == nil {
		h.publish(protocol.EventAgentSessions, uuidToString(wsID), "system", daemonID, protocol.AgentSessionsEventPayload{AgentID: uuidToString(agentID)})
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListDaemonAgentSessions(w http.ResponseWriter, r *http.Request) {
	wsID, daemonID, ok := h.requireAgentSessionDaemon(w, r, r.URL.Query().Get("workspace_id"), r.URL.Query().Get("daemon_id"))
	if !ok {
		return
	}
	sessions, err := h.Queries.ListAgentSessionsForDaemon(r.Context(), db.ListAgentSessionsForDaemonParams{WorkspaceID: wsID, DaemonID: daemonID})
	if err != nil {
		slog.Warn("list daemon agent sessions failed", "daemon_id", daemonID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list sessions")
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Sessions []db.AgentSession `json:"sessions"`
	}{Sessions: sessions})
}

type AgentSessionRuntimeResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type AgentSessionIssueResponse struct {
	ID         string `json:"id"`
	Identifier string `json:"identifier"`
}

// AgentSessionResponse matches the session DTO consumed by CLI and page readers.
type AgentSessionResponse struct {
	ID          string                       `json:"id"`
	Kind        string                       `json:"kind"`
	PID         *int32                       `json:"pid"`
	HerdrPaneID *string                      `json:"herdr_pane_id"`
	Runtime     *AgentSessionRuntimeResponse `json:"runtime"`
	TaskID      *string                      `json:"task_id"`
	Issue       *AgentSessionIssueResponse   `json:"issue"`
	WorkDir     string                       `json:"work_dir"`
	Repository  *string                      `json:"repository"`
	Branch      *string                      `json:"branch"`
	Status      string                       `json:"status"`
	StartedAt   string                       `json:"started_at"`
}

type AgentSessionsResponse struct {
	Sessions []AgentSessionResponse `json:"sessions"`
}

func (h *Handler) ListAgentSessions(w http.ResponseWriter, r *http.Request) {
	agent, ok := h.loadAgentForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	workspaceID := uuidToString(agent.WorkspaceID)
	actorType, actorID := h.resolveActor(r, requestUserID(r), workspaceID)
	if !h.canAccessPrivateAgent(r.Context(), agent, actorType, actorID, workspaceID) {
		writeError(w, http.StatusForbidden, "you do not have access to this agent")
		return
	}
	rows, err := h.Queries.ListAgentSessions(r.Context(), db.ListAgentSessionsParams{WorkspaceID: agent.WorkspaceID, AgentID: agent.ID})
	if err != nil {
		slog.Warn("list agent sessions failed", "agent_id", uuidToString(agent.ID), "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list sessions")
		return
	}
	resp := AgentSessionsResponse{Sessions: make([]AgentSessionResponse, 0, len(rows))}
	for _, row := range rows {
		session := AgentSessionResponse{
			ID: uuidToString(row.ID), Kind: row.Kind, HerdrPaneID: textToPtr(row.HerdrPaneID),
			TaskID: uuidToPtr(row.TaskID), WorkDir: row.WorkDir, Repository: textToPtr(row.Repository),
			Branch: textToPtr(row.Branch), Status: row.Status, StartedAt: timestampToString(row.StartedAt),
		}
		if row.Pid.Valid {
			session.PID = &row.Pid.Int32
		}
		if row.RuntimeID.Valid {
			session.Runtime = &AgentSessionRuntimeResponse{ID: uuidToString(row.RuntimeID), Name: row.RuntimeName.String}
		}
		if row.IssueID.Valid {
			session.Issue = &AgentSessionIssueResponse{ID: uuidToString(row.IssueID), Identifier: row.IssueIdentifier}
		}
		resp.Sessions = append(resp.Sessions, session)
	}
	writeJSON(w, http.StatusOK, resp)
}
