package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/todo"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) ListTodos(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID, ok := parseUUIDOrBadRequest(w, ctxWorkspaceID(r.Context()), "workspace id")
	if !ok {
		return
	}
	rows, err := h.Queries.ListOpenTodos(r.Context(), db.ListOpenTodosParams{
		WorkspaceID: workspaceID,
		MemberID:    parseUUID(userID),
	})
	if err != nil {
		slog.Error("failed to list todos", "workspace_id", uuidToString(workspaceID), "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list todos")
		return
	}
	resp := make([]map[string]any, len(rows))
	for i, row := range rows {
		resp[i] = todo.Response(db.GetTodoForMemberRow(row))
	}
	writeJSON(w, http.StatusOK, resp)
}

// TodoSummary is account-level and does not require an active workspace.
func (h *Handler) TodoSummary(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	rows, err := h.Queries.CountOpenTodosByWorkspace(r.Context(), parseUUID(userID))
	if err != nil {
		slog.Error("failed to summarize todos", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to summarize todos")
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (h *Handler) MarkTodoDone(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID, ok := parseUUIDOrBadRequest(w, ctxWorkspaceID(r.Context()), "workspace id")
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "todo id")
	if !ok {
		return
	}
	params := db.GetTodoForMemberParams{ID: id, WorkspaceID: workspaceID, MemberID: parseUUID(userID)}
	row, err := h.Queries.GetTodoForMember(r.Context(), params)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "todo not found")
		return
	}
	if err != nil {
		slog.Error("failed to get todo", "todo_id", uuidToString(id), "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get todo")
		return
	}
	if row.DoneAt.Valid {
		writeJSON(w, http.StatusOK, todo.Response(row))
		return
	}
	closed, err := (todo.Service{Queries: h.Queries, TxStarter: h.TxStarter, Bus: h.Bus}).MarkDone(r.Context(), db.MarkTodoDoneParams(params))
	if errors.Is(err, pgx.ErrNoRows) {
		// A concurrent member action or source event may already have closed it.
		row, err = h.Queries.GetTodoForMember(r.Context(), params)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "todo not found")
			return
		}
	} else if err == nil {
		row.DoneAt, row.DoneBy = closed.DoneAt, closed.DoneBy
	}
	if err != nil {
		slog.Error("failed to mark todo done", "todo_id", uuidToString(id), "error", err)
		writeError(w, http.StatusInternalServerError, "failed to mark todo done")
		return
	}
	writeJSON(w, http.StatusOK, todo.Response(row))
}
