package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// AgentQuestionsSettingsResponse is the workspace Questions default.
type AgentQuestionsSettingsResponse struct {
	Enabled bool `json:"enabled"`
	// OwnSettingCount counts open issues with their own Agent questions; they
	// ignore this default.
	OwnSettingCount int64 `json:"own_setting_count"`
	// BriefSection is what agents get while questions are on.
	BriefSection string `json:"brief_section"`
}

type updateAgentQuestionsRequest struct {
	Enabled *bool `json:"enabled"`
}

func (h *Handler) agentQuestionsSettings(r *http.Request, workspaceID pgtype.UUID) (AgentQuestionsSettingsResponse, error) {
	ws, err := h.Queries.GetWorkspace(r.Context(), workspaceID)
	if err != nil {
		return AgentQuestionsSettingsResponse{}, err
	}
	terminal, err := h.terminalIssueStatusKeys(r.Context(), workspaceID)
	if err != nil {
		return AgentQuestionsSettingsResponse{}, err
	}
	count, err := h.Queries.CountOwnAgentQuestionIssues(r.Context(), db.CountOwnAgentQuestionIssuesParams{WorkspaceID: workspaceID, TerminalStatusKeys: terminal})
	if err != nil {
		return AgentQuestionsSettingsResponse{}, err
	}
	return AgentQuestionsSettingsResponse{
		Enabled:         service.AgentQuestionsDefault(ws.Settings),
		OwnSettingCount: count,
		BriefSection:    execenv.QuestionsSection,
	}, nil
}

// GetAgentQuestions returns the workspace Questions default. Any member.
func (h *Handler) GetAgentQuestions(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	if _, ok := h.workspaceMember(w, r, workspaceID); !ok {
		return
	}
	out, err := h.agentQuestionsSettings(r, parseUUID(workspaceID))
	if err != nil {
		slog.Warn("load agent questions settings failed", "error", err, "workspace_id", workspaceID)
		writeError(w, http.StatusInternalServerError, "could not load agent questions settings")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// UpdateAgentQuestions sets the workspace Questions default. Issues on
// Workspace default follow it from their next run. Owners and admins only.
func (h *Handler) UpdateAgentQuestions(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	var in updateAgentQuestionsRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil || in.Enabled == nil {
		writeError(w, http.StatusBadRequest, "invalid agent questions body")
		return
	}
	member, ok := h.systemWakeupHuman(w, r, workspaceID)
	if !ok {
		return
	}
	if !roleAllowed(member.Role, "owner", "admin") {
		writeError(w, http.StatusForbidden, "only owners and admins can change workspace defaults")
		return
	}
	wsID := parseUUID(workspaceID)
	if err := service.SetAgentQuestionsDefault(r.Context(), h.Queries, wsID, *in.Enabled); err != nil {
		slog.Warn("update agent questions default failed", "error", err, "workspace_id", workspaceID)
		writeError(w, http.StatusInternalServerError, "could not save agent questions settings")
		return
	}
	ws, err := h.Queries.GetWorkspace(r.Context(), wsID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load agent questions settings")
		return
	}
	h.publish(protocol.EventWorkspaceUpdated, workspaceID, "member", requestUserID(r), map[string]any{"workspace": h.workspaceToResponse(ws)})
	out, err := h.agentQuestionsSettings(r, wsID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load agent questions settings")
		return
	}
	writeJSON(w, http.StatusOK, out)
}
