package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ReactToRunInput marks only a delivered receipt belonging to the token's run.
func (h *Handler) ReactToRunInput(w http.ResponseWriter, r *http.Request) {
	if !isTaskTokenActor(r) || r.Header.Get("X-Task-ID") == "" || r.Header.Get("X-Agent-ID") == "" || r.Header.Get("X-Workspace-ID") == "" {
		writeError(w, http.StatusUnauthorized, "react works only inside an agent's run")
		return
	}
	taskID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "taskId"), "task id")
	if !ok {
		return
	}
	itemID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "itemId"), "item id")
	if !ok {
		return
	}
	if uuidToString(taskID) != r.Header.Get("X-Task-ID") {
		writeError(w, http.StatusNotFound, "this run did not receive that item")
		return
	}
	task, err := h.Queries.GetAgentTaskInWorkspace(r.Context(), db.GetAgentTaskInWorkspaceParams{
		ID: taskID, WorkspaceID: parseUUID(r.Header.Get("X-Workspace-ID")),
	})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && uuidToString(task.AgentID) != r.Header.Get("X-Agent-ID")) {
		writeError(w, http.StatusNotFound, "this run did not receive that item")
		return
	}
	if err != nil {
		slog.Error("run input reaction task lookup failed", "task_id", uuidToString(taskID), "error", err)
		writeError(w, http.StatusInternalServerError, "failed to react to run input")
		return
	}
	var req struct {
		Emoji string `json:"emoji"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var receipt db.TaskSupplement
	switch req.Emoji {
	case "👀":
		receipt, err = h.Queries.MarkRunInputSeen(r.Context(), db.MarkRunInputSeenParams{TaskID: task.ID, ItemID: itemID})
	case "✅":
		receipt, err = h.Queries.MarkRunInputHandled(r.Context(), db.MarkRunInputHandledParams{TaskID: task.ID, ItemID: itemID})
	default:
		writeError(w, http.StatusBadRequest, "react takes 👀 or ✅")
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "this run did not receive that item")
		return
	}
	if err != nil {
		slog.Error("run input reaction failed", "task_id", uuidToString(task.ID), "item_id", uuidToString(itemID), "error", err)
		writeError(w, http.StatusInternalServerError, "failed to react to run input")
		return
	}
	payload := map[string]any{
		"issue_id": uuidToString(receipt.IssueID), "item_kind": receipt.ItemKind, "item_id": uuidToString(receipt.ItemID),
		"task_id": uuidToString(task.ID), "agent_id": uuidToString(task.AgentID),
		"seen_at": timestampToPtr(receipt.SeenAt), "handled_at": timestampToPtr(receipt.HandledAt),
	}
	h.publish("run_input:reacted", uuidToString(receipt.WorkspaceID), "agent", uuidToString(task.AgentID), payload)
	slog.Debug("run input reaction recorded", "task_id", uuidToString(task.ID), "item_id", uuidToString(itemID), "emoji", req.Emoji)
	writeJSON(w, http.StatusOK, payload)
}
