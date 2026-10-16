package handler

import (
	"errors"
	"net/http"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type MembershipOrganisationResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type MyMembershipResponse struct {
	WorkspaceID   string                           `json:"workspace_id"`
	WorkspaceName string                           `json:"workspace_name"`
	WorkspaceSlug string                           `json:"workspace_slug"`
	Role          string                           `json:"role"`
	Organisations []MembershipOrganisationResponse `json:"organisations"`
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
			Organisations: make([]MembershipOrganisationResponse, len(row.OrganisationIds)),
		}
		for j, id := range row.OrganisationIds {
			memberships[i].Organisations[j] = MembershipOrganisationResponse{
				ID: uuidToString(id), Name: row.OrganisationNames[j],
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"memberships": memberships})
}

// membershipOrganisations validates the complete set before any membership write.
func membershipOrganisations(w http.ResponseWriter, r *http.Request, queries *db.Queries, workspaceID pgtype.UUID, role string, organisations []string, missingMessage string) ([]pgtype.UUID, bool) {
	if role != "client" {
		if len(organisations) != 0 {
			writeError(w, http.StatusBadRequest, "staff membership must not have organisations")
			return nil, false
		}
		return []pgtype.UUID{}, true
	}
	if len(organisations) == 0 {
		writeError(w, http.StatusBadRequest, missingMessage)
		return nil, false
	}
	ids := make([]pgtype.UUID, 0, len(organisations))
	seen := make(map[[16]byte]struct{}, len(organisations))
	for _, organisation := range organisations {
		id, ok := parseUUIDOrBadRequest(w, organisation, "organisation id")
		if !ok {
			return nil, false
		}
		if _, exists := seen[id.Bytes]; exists {
			continue
		}
		_, err := queries.GetOrganisationInWorkspace(r.Context(), db.GetOrganisationInWorkspaceParams{
			ID: id, WorkspaceID: workspaceID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusBadRequest, missingMessage)
			} else {
				writeError(w, http.StatusInternalServerError, "failed to load organisation")
			}
			return nil, false
		}
		seen[id.Bytes] = struct{}{}
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b pgtype.UUID) int {
		return slices.Compare(a.Bytes[:], b.Bytes[:])
	})
	return ids, true
}

func organisationIDsResponse(ids []pgtype.UUID) []string {
	result := make([]string, len(ids))
	for i, id := range ids {
		result[i] = uuidToString(id)
	}
	return result
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
