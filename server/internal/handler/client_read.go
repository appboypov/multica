package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type ClientWaitResponse struct {
	ID               string  `json:"id"`
	Title            string  `json:"title"`
	Description      *string `json:"description"`
	DueDate          *string `json:"due_date"`
	OrganisationName string  `json:"organisation_name"`
}

type ClientReleaseResponse struct {
	Version string `json:"version"`
	Shipped bool   `json:"shipped"`
}

type ClientPlannedWorkResponse struct {
	ID               string                `json:"id"`
	Title            string                `json:"title"`
	Status           string                `json:"status"`
	OrganisationName string                `json:"organisation_name"`
	ProjectName      string                `json:"project_name"`
	Release          ClientReleaseResponse `json:"release"`
}

type ClientWaitListResponse struct {
	Waits []ClientWaitResponse `json:"waits"`
	Total int64                `json:"total"`
}

type ClientPlannedWorkListResponse struct {
	Issues []ClientPlannedWorkResponse `json:"issues"`
	Total  int64                       `json:"total"`
}

func (h *Handler) requireClientReadWorkspace(w http.ResponseWriter, r *http.Request) (pgtype.UUID, pgtype.UUID, bool) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return pgtype.UUID{}, pgtype.UUID{}, false
	}
	user := parseUUID(userID)
	workspace, err := h.Queries.GetClientReadWorkspace(r.Context(), user)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusForbidden, "client membership required")
		} else {
			slog.Error("client reads: load membership", "user_id", userID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to load client membership")
		}
		return pgtype.UUID{}, pgtype.UUID{}, false
	}
	return user, workspace, true
}

func clientReadPage(r *http.Request) (int32, int32) {
	limit := int64(100)
	if value, err := strconv.ParseInt(r.URL.Query().Get("limit"), 10, 64); err == nil && value > 0 && value < limit {
		limit = value
	}
	offset, err := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 32)
	if err != nil || offset < 0 {
		offset = 0
	}
	return int32(limit), int32(offset)
}

func (h *Handler) ListClientWaits(w http.ResponseWriter, r *http.Request) {
	user, workspace, ok := h.requireClientReadWorkspace(w, r)
	if !ok {
		return
	}
	terminalKeys, err := h.terminalIssueStatusKeys(r.Context(), workspace)
	if err != nil {
		slog.Error("client reads: load terminal statuses", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list waits")
		return
	}
	total, err := h.Queries.CountClientWaits(r.Context(), db.CountClientWaitsParams{
		UserID: user, WorkspaceID: workspace, TerminalStatusKeys: terminalKeys,
	})
	if err != nil {
		slog.Error("client reads: count waits", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list waits")
		return
	}
	limit, offset := clientReadPage(r)
	rows, err := h.Queries.ListClientWaits(r.Context(), db.ListClientWaitsParams{
		UserID: user, WorkspaceID: workspace, TerminalStatusKeys: terminalKeys,
		PageLimit: limit, PageOffset: offset,
	})
	if err != nil {
		slog.Error("client reads: list waits", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list waits")
		return
	}
	waits := make([]ClientWaitResponse, len(rows))
	for i, row := range rows {
		waits[i] = ClientWaitResponse{
			ID: uuidToString(row.ID), Title: row.Title, Description: textToPtr(row.Description),
			DueDate: dateToPtr(row.DueDate), OrganisationName: row.OrganisationName,
		}
	}
	writeJSON(w, http.StatusOK, ClientWaitListResponse{Waits: waits, Total: total})
}

func (h *Handler) GetClientWait(w http.ResponseWriter, r *http.Request) {
	user, workspace, ok := h.requireClientReadWorkspace(w, r)
	if !ok {
		return
	}
	id, err := util.ParseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "wait not found")
		return
	}
	terminalKeys, err := h.terminalIssueStatusKeys(r.Context(), workspace)
	if err != nil {
		slog.Error("client reads: load terminal statuses", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to read wait")
		return
	}
	row, err := h.Queries.GetClientWait(r.Context(), db.GetClientWaitParams{
		UserID: user, WorkspaceID: workspace, TerminalStatusKeys: terminalKeys, ID: id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "wait not found")
		} else {
			slog.Error("client reads: get wait", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to read wait")
		}
		return
	}
	writeJSON(w, http.StatusOK, ClientWaitResponse{
		ID: uuidToString(row.ID), Title: row.Title, Description: textToPtr(row.Description),
		DueDate: dateToPtr(row.DueDate), OrganisationName: row.OrganisationName,
	})
}

func (h *Handler) ListClientPlannedWork(w http.ResponseWriter, r *http.Request) {
	user, workspace, ok := h.requireClientReadWorkspace(w, r)
	if !ok {
		return
	}
	total, err := h.Queries.CountClientPlannedWork(r.Context(), db.CountClientPlannedWorkParams{
		UserID: user, WorkspaceID: workspace,
	})
	if err != nil {
		slog.Error("client reads: count planned work", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list planned work")
		return
	}
	limit, offset := clientReadPage(r)
	rows, err := h.Queries.ListClientPlannedWork(r.Context(), db.ListClientPlannedWorkParams{
		UserID: user, WorkspaceID: workspace, PageLimit: limit, PageOffset: offset,
	})
	if err != nil {
		slog.Error("client reads: list planned work", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list planned work")
		return
	}
	issues := make([]ClientPlannedWorkResponse, len(rows))
	for i, row := range rows {
		issues[i] = ClientPlannedWorkResponse{
			ID: uuidToString(row.ID), Title: row.Title, Status: row.Status,
			OrganisationName: row.OrganisationName, ProjectName: row.ProjectName,
			Release: ClientReleaseResponse{Version: row.Version, Shipped: row.Shipped},
		}
	}
	writeJSON(w, http.StatusOK, ClientPlannedWorkListResponse{Issues: issues, Total: total})
}
