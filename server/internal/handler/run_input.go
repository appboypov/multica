package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
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
	payload := protocol.RunInputReactedPayload{
		IssueID: uuidToString(receipt.IssueID), ItemKind: receipt.ItemKind, ItemID: uuidToString(receipt.ItemID),
		TaskID: uuidToString(task.ID), AgentID: uuidToString(task.AgentID),
		SeenAt: timestampToPtr(receipt.SeenAt), HandledAt: timestampToPtr(receipt.HandledAt),
	}
	h.publish(protocol.EventRunInputReacted, uuidToString(receipt.WorkspaceID), "agent", uuidToString(task.AgentID), payload)
	slog.Debug("run input reaction recorded", "task_id", uuidToString(task.ID), "item_id", uuidToString(itemID), "emoji", req.Emoji)
	writeJSON(w, http.StatusOK, payload)
}

// RunInputMarkResponse is one run's visible acknowledgement of an item.
type RunInputMarkResponse struct {
	TaskID    string  `json:"task_id"`
	AgentID   string  `json:"agent_id"`
	SeenAt    *string `json:"seen_at,omitempty"`
	HandledAt *string `json:"handled_at,omitempty"`
}

func (h *Handler) listRunInputMarks(ctx context.Context, workspaceID pgtype.UUID, itemIDs []pgtype.UUID) map[string][]RunInputMarkResponse {
	marks := make(map[string][]RunInputMarkResponse)
	if len(itemIDs) == 0 {
		return marks
	}
	rows, err := h.Queries.ListRunInputMarksByItems(ctx, db.ListRunInputMarksByItemsParams{WorkspaceID: workspaceID, ItemIds: itemIDs})
	if err != nil {
		slog.Error("run input marks lookup failed", "workspace_id", uuidToString(workspaceID), "error", err)
		return marks
	}
	for _, row := range rows {
		id := uuidToString(row.ItemID)
		marks[id] = append(marks[id], RunInputMarkResponse{
			TaskID: uuidToString(row.TaskID), AgentID: uuidToString(row.AgentID),
			SeenAt: timestampToPtr(row.SeenAt), HandledAt: timestampToPtr(row.HandledAt),
		})
	}
	return marks
}
