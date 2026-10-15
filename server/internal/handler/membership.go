package handler

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type MembershipOrganisationResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type MyMembershipResponse struct {
	WorkspaceID   string                          `json:"workspace_id"`
	WorkspaceName string                          `json:"workspace_name"`
	WorkspaceSlug string                          `json:"workspace_slug"`
	Role          string                          `json:"role"`
	Organisation  *MembershipOrganisationResponse `json:"organisation"`
}

func (h *Handler) ListMyMemberships(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	rows, err := h.Queries.ListMyMemberships(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list memberships")
		return
	}
	memberships := make([]MyMembershipResponse, len(rows))
	for i, row := range rows {
		memberships[i] = MyMembershipResponse{
			WorkspaceID: uuidToString(row.WorkspaceID), WorkspaceName: row.WorkspaceName,
			WorkspaceSlug: row.WorkspaceSlug, Role: row.Role,
		}
		if row.Role == "client" && row.OrganisationID.Valid {
			memberships[i].Organisation = &MembershipOrganisationResponse{
				ID: uuidToString(row.OrganisationID), Name: row.OrganisationName.String,
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"memberships": memberships})
}

// membershipOrganisation validates the binding before any membership write.
func membershipOrganisation(w http.ResponseWriter, r *http.Request, queries *db.Queries, workspaceID pgtype.UUID, role string, organisation *string, missingMessage string) (pgtype.UUID, bool) {
	if role != "client" {
		if organisation != nil {
			writeError(w, http.StatusBadRequest, "staff membership must not have an organisation")
			return pgtype.UUID{}, false
		}
		return pgtype.UUID{}, true
	}
	if organisation == nil {
		writeError(w, http.StatusBadRequest, missingMessage)
		return pgtype.UUID{}, false
	}
	organisationID, ok := parseUUIDOrBadRequest(w, *organisation, "organisation id")
	if !ok {
		return pgtype.UUID{}, false
	}
	_, err := queries.GetOrganisationInWorkspace(r.Context(), db.GetOrganisationInWorkspaceParams{
		ID: organisationID, WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusBadRequest, missingMessage)
		} else {
			writeError(w, http.StatusInternalServerError, "failed to load organisation")
		}
		return pgtype.UUID{}, false
	}
	return organisationID, true
}

// checkMembershipAdmission must run under LockUserForMembership when a row is
// created, so concurrent staff joins and client acceptance cannot both succeed.
func checkMembershipAdmission(w http.ResponseWriter, r *http.Request, queries *db.Queries, userID pgtype.UUID, role string) bool {
	if role == "client" {
		count, err := queries.CountMembershipsByUser(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check memberships")
			return false
		}
		if count > 0 {
			writeError(w, http.StatusConflict, "a client needs an account without other workspace memberships")
			return false
		}
	} else {
		isClient, err := queries.IsClientAccount(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check memberships")
			return false
		}
		if isClient {
			writeError(w, http.StatusConflict, "this account is a client account")
			return false
		}
	}
	return true
}
