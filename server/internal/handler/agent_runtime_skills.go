package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/logger"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const maxRuntimeSkillKeyLength = 512

// RuntimeSkillException identifies one runtime-local skill whose state differs
// from the agent's section switch: off while the agent inherits its runtime's
// skills, on while it does not. RuntimeID scopes the choice to a single
// machine/runtime so a same-named skill on another runtime is unaffected.
type RuntimeSkillException struct {
	RuntimeID string `json:"runtime_id"`
	Provider  string `json:"provider"`
	Root      string `json:"root"`
	Key       string `json:"key"`
	Name      string `json:"name,omitempty"`
	Plugin    string `json:"plugin,omitempty"`
}

func decodeRuntimeSkillExceptions(raw []byte) []RuntimeSkillException {
	if len(raw) == 0 {
		return []RuntimeSkillException{}
	}
	var skills []RuntimeSkillException
	if err := json.Unmarshal(raw, &skills); err != nil || skills == nil {
		return []RuntimeSkillException{}
	}
	return skills
}

func runtimeSkillExceptionsFor(raw []byte, runtimeID, provider string) []RuntimeSkillException {
	all := decodeRuntimeSkillExceptions(raw)
	result := make([]RuntimeSkillException, 0, len(all))
	for _, skill := range all {
		if skill.RuntimeID == runtimeID && skill.Provider == provider {
			result = append(result, skill)
		}
	}
	return result
}

func normalizeRuntimeSkillIdentity(root, key, plugin string) (string, string, string, bool) {
	root = strings.TrimSpace(root)
	key = strings.TrimSpace(key)
	plugin = strings.TrimSpace(plugin)
	if len(key) == 0 || len(key) > maxRuntimeSkillKeyLength {
		return "", "", "", false
	}
	cleaned := filepath.ToSlash(filepath.Clean(filepath.FromSlash(key)))
	if cleaned == "." || filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", "", "", false
	}
	switch root {
	case "provider", "universal":
		plugin = ""
	case "plugin":
		if plugin == "" {
			return "", "", "", false
		}
	default:
		return "", "", "", false
	}
	return root, cleaned, plugin, true
}

func sameRuntimeSkill(a, b RuntimeSkillException) bool {
	return a.RuntimeID == b.RuntimeID && a.Provider == b.Provider &&
		a.Root == b.Root && a.Key == b.Key && a.Plugin == b.Plugin
}

func runtimeSkillControlsSupported(provider string) bool {
	return provider == "claude" || provider == "codex" || provider == "omp"
}

// runtimeSkillIdentity names one runtime-local skill in a request.
type runtimeSkillIdentity struct {
	RuntimeID string `json:"runtime_id"`
	Root      string `json:"root"`
	Key       string `json:"key"`
	Name      string `json:"name"`
	Plugin    string `json:"plugin"`
}

type runtimeSkillEnabledRequest struct {
	runtimeSkillIdentity
	Enabled *bool `json:"enabled"`
}

// runtimeSkillTarget resolves a requested runtime skill on a local runtime of
// the workspace whose runs enforce skill filters. It writes the refusal and
// returns false when the runtime or the identity is not accepted.
func (h *Handler) runtimeSkillTarget(w http.ResponseWriter, r *http.Request, workspaceID, runtimeID pgtype.UUID, req runtimeSkillIdentity) (RuntimeSkillException, bool) {
	rt, err := h.getAgentRuntime(r.Context(), obsmetrics.RuntimeLookupSourceRuntimeAPI, runtimeID)
	if err != nil || rt.WorkspaceID != workspaceID {
		writeError(w, http.StatusNotFound, "runtime not found")
		return RuntimeSkillException{}, false
	}
	if rt.RuntimeMode != "local" || !runtimeSkillControlsSupported(rt.Provider) {
		writeError(w, http.StatusBadRequest, "runtime skill controls are only supported for claude, codex and omp")
		return RuntimeSkillException{}, false
	}
	root, key, plugin, valid := normalizeRuntimeSkillIdentity(req.Root, req.Key, req.Plugin)
	if !valid || (root == "plugin" && rt.Provider != "claude") {
		writeError(w, http.StatusBadRequest, "invalid runtime skill identity")
		return RuntimeSkillException{}, false
	}
	name := strings.TrimSpace(req.Name)
	if len(name) > maxRuntimeSkillKeyLength {
		writeError(w, http.StatusBadRequest, "invalid runtime skill name")
		return RuntimeSkillException{}, false
	}
	return RuntimeSkillException{
		RuntimeID: uuidToString(runtimeID),
		Provider:  rt.Provider,
		Root:      root,
		Key:       key,
		Name:      name,
		Plugin:    plugin,
	}, true
}

// withRuntimeSkillState returns the stored exceptions with target set to
// enabled: the skill is an exception exactly when its state differs from the
// section switch.
func withRuntimeSkillState(raw []byte, target RuntimeSkillException, enabled, inherit bool) ([]byte, error) {
	current := decodeRuntimeSkillExceptions(raw)
	next := make([]RuntimeSkillException, 0, len(current)+1)
	for _, skill := range current {
		if sameRuntimeSkill(skill, target) {
			continue
		}
		next = append(next, skill)
	}
	if enabled != inherit {
		next = append(next, target)
	}
	return json.Marshal(next)
}

// SetAgentRuntimeSkillEnabled turns one skill that is discovered from the
// agent's currently assigned local runtime on or off. The skill is an
// exception exactly when its state differs from the section switch.
func (h *Handler) SetAgentRuntimeSkillEnabled(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	agent, ok := h.loadAgentForUser(w, r, agentID)
	if !ok {
		return
	}
	if !h.canManageAgent(w, r, agent) {
		return
	}

	var req runtimeSkillEnabledRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Enabled == nil {
		writeError(w, http.StatusBadRequest, "runtime_id, root, key, and enabled are required")
		return
	}
	runtimeID, ok := parseUUIDOrBadRequest(w, req.RuntimeID, "runtime_id")
	if !ok {
		return
	}
	if !agent.RuntimeID.Valid || agent.RuntimeID != runtimeID {
		writeError(w, http.StatusConflict, "agent is no longer assigned to this runtime")
		return
	}
	target, ok := h.runtimeSkillTarget(w, r, agent.WorkspaceID, runtimeID, req.runtimeSkillIdentity)
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
	locked, err := qtx.GetAgentForUpdate(r.Context(), agent.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load agent")
		return
	}
	if !locked.RuntimeID.Valid || locked.RuntimeID != runtimeID {
		writeError(w, http.StatusConflict, "agent is no longer assigned to this runtime")
		return
	}

	payload, err := withRuntimeSkillState(locked.RuntimeSkillExceptions, target, *req.Enabled, locked.InheritRuntimeSkills)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode runtime skill settings")
		return
	}
	updated, err := qtx.UpdateAgentRuntimeSkillExceptions(r.Context(), db.UpdateAgentRuntimeSkillExceptionsParams{
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

	h.broadcastAgentRuntimeSkills(r, updated)
	w.WriteHeader(http.StatusNoContent)
}

// SetAgentRuntimeSkillsInherited sets every runtime skill of the agent on or
// off, which also clears its per-skill exceptions.
func (h *Handler) SetAgentRuntimeSkillsInherited(w http.ResponseWriter, r *http.Request) {
	agent, ok := h.loadAgentForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	if !h.canManageAgent(w, r, agent) {
		return
	}

	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Enabled == nil {
		writeError(w, http.StatusBadRequest, "enabled is required")
		return
	}
	if !agent.RuntimeID.Valid {
		writeError(w, http.StatusBadRequest, "agent has no runtime")
		return
	}
	rt, err := h.getAgentRuntime(r.Context(), obsmetrics.RuntimeLookupSourceRuntimeAPI, agent.RuntimeID)
	if err != nil || rt.WorkspaceID != agent.WorkspaceID {
		writeError(w, http.StatusNotFound, "runtime not found")
		return
	}
	if rt.RuntimeMode != "local" || !runtimeSkillControlsSupported(rt.Provider) {
		writeError(w, http.StatusBadRequest, "runtime skill controls are only supported for claude, codex and omp")
		return
	}

	updated, err := h.Queries.UpdateAgentInheritRuntimeSkills(r.Context(), db.UpdateAgentInheritRuntimeSkillsParams{
		ID:                   agent.ID,
		InheritRuntimeSkills: *req.Enabled,
	})
	if err != nil {
		slog.Error("runtime skill inheritance: update failed",
			append(logger.RequestAttrs(r), "error", err, "agent_id", uuidToString(agent.ID))...)
		writeError(w, http.StatusInternalServerError, "failed to update runtime skill inheritance")
		return
	}
	slog.Info("runtime skill inheritance updated",
		append(logger.RequestAttrs(r), "agent_id", uuidToString(agent.ID), "enabled", *req.Enabled)...)
	h.broadcastAgentRuntimeSkills(r, updated)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) broadcastAgentRuntimeSkills(r *http.Request, agent db.Agent) {
	agentID := uuidToString(agent.ID)
	resp := h.agentToResponse(agent)
	if err := h.enrichAgentResponseWithTargets(r.Context(), &resp, agent.ID); err != nil {
		slog.Warn("runtime skill update: load invocation targets for broadcast failed",
			append(logger.RequestAttrs(r), "error", err, "agent_id", agentID)...)
	}
	// Reload assigned skills so a runtime-skill update cannot signal that the
	// agent's workspace skill bindings were cleared.
	if err := h.attachAgentSkills(r.Context(), &resp, agent.ID); err != nil {
		slog.Warn("runtime skill update: load agent skills for broadcast failed",
			append(logger.RequestAttrs(r), "error", err, "agent_id", agentID)...)
	}
	actorType, actorID := h.resolveActor(r, requestUserID(r), uuidToString(agent.WorkspaceID))
	h.publish(protocol.EventAgentStatus, uuidToString(agent.WorkspaceID), actorType, actorID,
		map[string]any{"agent": broadcastAgentResponse(resp)})
}
