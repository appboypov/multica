package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

// An update that does not name organisation_id keeps the project's
// organisation: the handler seeds UpdateProjectParams from the stored row.
func TestUpdateProjectKeepsOrganisationID(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	orgID := dbfx.Insert(t, "organisation", testutil.Cols{
		"workspace_id": testWorkspaceID,
		"name":         "Keep Org",
		"key":          "keep-org",
	})
	projectID := dbfx.Project(t, "before rename", testutil.Cols{"organisation_id": orgID})

	w := httptest.NewRecorder()
	req := withURLParam(newRequest("PUT", "/api/projects/"+projectID, map[string]any{
		"title": "after rename",
	}), "id", projectID)
	testHandler.UpdateProject(w, req)
	if w.Code != 200 {
		t.Fatalf("UpdateProject: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var title string
	var gotOrgID *string
	dbfx.QueryRow(t, `SELECT title, organisation_id::text FROM project WHERE id = $1`, projectID).Scan(&title, &gotOrgID)
	if title != "after rename" {
		t.Fatalf("title = %q, want %q", title, "after rename")
	}
	if gotOrgID == nil || *gotOrgID != orgID {
		t.Fatalf("organisation_id = %v, want %s", gotOrgID, orgID)
	}
}
