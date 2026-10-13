package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/logger"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type OrganisationResponse struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	Key         string `json:"key"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type CreateOrganisationRequest struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

type UpdateOrganisationRequest struct {
	Key string `json:"key"`
}

var organisationKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func validateOrganisationKey(w http.ResponseWriter, key string) bool {
	if !organisationKeyPattern.MatchString(key) {
		writeError(w, http.StatusBadRequest, "key must be lowercase letters, digits and dashes, starting with a letter or digit")
		return false
	}
	return true
}

func organisationToResponse(org db.Organisation) OrganisationResponse {
	return OrganisationResponse{
		ID:          uuidToString(org.ID),
		WorkspaceID: uuidToString(org.WorkspaceID),
		Name:        org.Name,
		Key:         org.Key,
		CreatedAt:   timestampToString(org.CreatedAt),
		UpdatedAt:   timestampToString(org.UpdatedAt),
	}
}

func (h *Handler) writeOrganisationWriteError(w http.ResponseWriter, r *http.Request, err error, action string) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "organisation_workspace_name_unique":
			writeError(w, http.StatusConflict, "organisation name is taken")
			return
		case "organisation_workspace_key_unique":
			writeError(w, http.StatusConflict, "organisation key is taken")
			return
		}
	}
	slog.Error("organisation "+action+" failed", append(logger.RequestAttrs(r), "error", err)...)
	writeError(w, http.StatusInternalServerError, "failed to "+action+" organisation")
}

func (h *Handler) ListOrganisations(w http.ResponseWriter, r *http.Request) {
	wsUUID, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace_id")
	if !ok {
		return
	}
	organisations, err := h.Queries.ListOrganisations(r.Context(), wsUUID)
	if err != nil {
		slog.Error("list organisations failed", append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to list organisations")
		return
	}
	resp := make([]OrganisationResponse, len(organisations))
	for i, org := range organisations {
		resp[i] = organisationToResponse(org)
	}
	writeJSON(w, http.StatusOK, map[string]any{"organisations": resp, "total": len(resp)})
}

func (h *Handler) GetOrganisation(w http.ResponseWriter, r *http.Request) {
	idUUID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "organisation id")
	if !ok {
		return
	}
	wsUUID, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace_id")
	if !ok {
		return
	}
	org, err := h.Queries.GetOrganisationInWorkspace(r.Context(), db.GetOrganisationInWorkspaceParams{
		ID: idUUID, WorkspaceID: wsUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "organisation not found")
			return
		}
		slog.Error("get organisation failed", append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to get organisation")
		return
	}
	writeJSON(w, http.StatusOK, organisationToResponse(org))
}

func (h *Handler) CreateOrganisation(w http.ResponseWriter, r *http.Request) {
	var req CreateOrganisationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if !validateOrganisationKey(w, req.Key) {
		return
	}
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	org, err := h.Queries.CreateOrganisation(r.Context(), db.CreateOrganisationParams{
		WorkspaceID: wsUUID, Name: req.Name, Key: req.Key,
	})
	if err != nil {
		h.writeOrganisationWriteError(w, r, err, "create")
		return
	}
	resp := organisationToResponse(org)
	h.publish(protocol.EventOrganisationCreated, workspaceID, "member", userID, protocol.OrganisationEventPayload{OrganisationID: resp.ID})
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) UpdateOrganisation(w http.ResponseWriter, r *http.Request) {
	idUUID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "organisation id")
	if !ok {
		return
	}
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	var req UpdateOrganisationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !validateOrganisationKey(w, req.Key) {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	org, err := h.Queries.UpdateOrganisationKey(r.Context(), db.UpdateOrganisationKeyParams{
		ID: idUUID, WorkspaceID: wsUUID, Key: req.Key,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "organisation not found")
			return
		}
		h.writeOrganisationWriteError(w, r, err, "update")
		return
	}
	resp := organisationToResponse(org)
	h.publish(protocol.EventOrganisationUpdated, workspaceID, "member", userID, protocol.OrganisationEventPayload{OrganisationID: resp.ID})
	writeJSON(w, http.StatusOK, resp)
}
