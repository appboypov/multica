package handler

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
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

	req := withURLParam(newRequest("PUT", "/api/projects/"+projectID, map[string]any{
		"title": "after rename",
	}), "id", projectID)
	var response ProjectResponse
	testutil.Call(t, testHandler.UpdateProject, req).Want(http.StatusOK).JSON(&response)
	if response.OrganisationID == nil || *response.OrganisationID != orgID {
		t.Fatalf("response organisation_id = %v, want %s", response.OrganisationID, orgID)
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

func TestProjectCreateAndMoveOrganisation(t *testing.T) {
	orgID := dbfx.Insert(t, "organisation", testutil.Cols{"workspace_id": testWorkspaceID, "name": "Project Rebl Records", "key": "project-rebl"})
	otherOrgID := dbfx.Insert(t, "organisation", testutil.Cols{"workspace_id": testWorkspaceID, "name": "Project De App Specialist", "key": "project-das"})
	var project ProjectResponse
	testutil.Call(t, testHandler.CreateProject, newRequest("POST", "/api/projects", map[string]any{
		"title": "Fan app", "organisation_id": orgID,
	})).Want(http.StatusCreated).JSON(&project)
	dbfx.Cleanup(t, `DELETE FROM project WHERE id = $1`, project.ID)
	if project.OrganisationID == nil || *project.OrganisationID != orgID {
		t.Fatalf("created organisation_id = %v, want %s", project.OrganisationID, orgID)
	}

	for _, tc := range []struct {
		name string
		body map[string]any
		want *string
	}{
		{"move", map[string]any{"organisation_id": otherOrgID}, &otherOrgID},
		{"keep", map[string]any{"title": "Fan app renamed"}, &otherOrgID},
		{"clear", map[string]any{"organisation_id": nil}, nil},
		{"set first", map[string]any{"organisation_id": orgID}, &orgID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var updated ProjectResponse
			req := withURLParam(newRequest("PUT", "/api/projects/"+project.ID, tc.body), "id", project.ID)
			testutil.Call(t, testHandler.UpdateProject, req).Want(http.StatusOK).JSON(&updated)
			var stored *string
			dbfx.QueryRow(t, `SELECT organisation_id::text FROM project WHERE id = $1`, project.ID).Scan(&stored)
			for source, got := range map[string]*string{"response": updated.OrganisationID, "database": stored} {
				if (got == nil) != (tc.want == nil) || (got != nil && tc.want != nil && *got != *tc.want) {
					t.Fatalf("%s organisation_id = %v, want %v", source, got, tc.want)
				}
			}
			var fetched ProjectResponse
			testutil.Call(t, testHandler.GetProject, withURLParam(newRequest("GET", "/api/projects/"+project.ID, nil), "id", project.ID)).Want(http.StatusOK).JSON(&fetched)
			if (fetched.OrganisationID == nil) != (tc.want == nil) || (fetched.OrganisationID != nil && tc.want != nil && *fetched.OrganisationID != *tc.want) {
				t.Fatalf("GET organisation_id = %v, want %v", fetched.OrganisationID, tc.want)
			}
		})
	}
	var list struct {
		Projects []ProjectResponse `json:"projects"`
	}
	testutil.Call(t, testHandler.ListProjects, newRequest("GET", "/api/projects", nil)).Want(http.StatusOK).JSON(&list)
	found := false
	for _, row := range list.Projects {
		if row.ID == project.ID {
			found = true
			if row.OrganisationID == nil || *row.OrganisationID != orgID {
				t.Fatalf("list organisation_id = %v, want %s", row.OrganisationID, orgID)
			}
		}
	}
	if !found {
		t.Fatal("created project is absent from list")
	}
}

func TestProjectCreateWithoutOrganisation(t *testing.T) {
	var project ProjectResponse
	testutil.Call(t, testHandler.CreateProject, newRequest("POST", "/api/projects", map[string]any{"title": "CLI project"})).Want(http.StatusCreated).JSON(&project)
	dbfx.Cleanup(t, `DELETE FROM project WHERE id = $1`, project.ID)
	if project.OrganisationID != nil {
		t.Fatalf("project without organisation got %v", project.OrganisationID)
	}
}

func TestProjectRejectsForeignAndUnknownOrganisation(t *testing.T) {
	workspaceID := dbfx.Workspace(t, "Foreign project organisation", "project-foreign-"+uuid.NewString())
	foreignID := dbfx.Insert(t, "organisation", testutil.Cols{"workspace_id": workspaceID, "name": "Foreign", "key": "foreign"})
	localID := dbfx.Insert(t, "organisation", testutil.Cols{"workspace_id": testWorkspaceID, "name": "Local project organisation", "key": "local-project"})
	projectID := dbfx.Project(t, "Keep local organisation", testutil.Cols{"organisation_id": localID})
	for _, id := range []string{foreignID, uuid.NewString(), "not-a-uuid", ""} {
		t.Run("organisation "+id, func(t *testing.T) {
			for _, method := range []string{"POST", "PUT"} {
				req := newRequest(method, "/api/projects", map[string]any{"title": "Rejected", "organisation_id": id})
				handler := testHandler.CreateProject
				if method == "PUT" {
					handler = testHandler.UpdateProject
					req = withURLParam(req, "id", projectID)
				}
				response := testutil.Call(t, handler, req).Want(http.StatusBadRequest).Map()
				if (id == foreignID || id != "" && id != "not-a-uuid") && response["error"] != "organisation not found" {
					t.Fatalf("%s missing organisation error = %v", method, response["error"])
				}
			}
		})
	}
	var stored string
	dbfx.QueryRow(t, `SELECT organisation_id::text FROM project WHERE id = $1`, projectID).Scan(&stored)
	if stored != localID {
		t.Fatalf("rejected update replaced organisation with %s", stored)
	}
	if count := dbfx.Count(t, `SELECT count(*) FROM project WHERE workspace_id = $1 AND title = 'Rejected'`, testWorkspaceID); count != 0 {
		t.Fatalf("rejected creates wrote %d projects", count)
	}
}

func TestProjectSearchIncludesCompletedAndDoesNotDemoteCancelled(t *testing.T) {
	workspaceID := dbfx.Workspace(t, "Project search organisations", "project-search-org-"+uuid.NewString())
	orgID := dbfx.Insert(t, "organisation", testutil.Cols{"workspace_id": workspaceID, "name": "Search organisation", "key": "search"})
	cancelledID := dbfx.Project(t, "Organisation Search", testutil.Cols{
		"workspace_id": workspaceID, "status": "cancelled", "organisation_id": orgID,
		"updated_at": testutil.Raw("now() - interval '1 hour'"),
	})
	completedID := dbfx.Project(t, "Organisation Search Completed", testutil.Cols{
		"workspace_id": workspaceID, "status": "completed", "organisation_id": orgID,
	})
	activeID := dbfx.Project(t, "Organisation Search Active", testutil.Cols{
		"workspace_id": workspaceID, "status": "in_progress",
		"updated_at": testutil.Raw("now() - interval '2 hours'"),
	})
	for _, suffix := range []string{"", "&include_closed=true"} {
		var response struct {
			Projects []SearchProjectResponse `json:"projects"`
		}
		req := testutil.WithHeaders(newRequest("GET", "/api/projects/search?q=Organisation"+suffix, nil), "X-Workspace-ID", workspaceID)
		testutil.Call(t, testHandler.SearchProjects, req).Want(http.StatusOK).JSON(&response)
		if len(response.Projects) != 3 {
			t.Fatalf("search%s returned %d projects, want 3: %+v", suffix, len(response.Projects), response.Projects)
		}
		for i, want := range []string{completedID, cancelledID, activeID} {
			if response.Projects[i].ID != want {
				t.Fatalf("search%s row %d = %s, want %s (recency, not status)", suffix, i, response.Projects[i].ID, want)
			}
		}
		for _, row := range response.Projects[:2] {
			if row.OrganisationID == nil || *row.OrganisationID != orgID {
				t.Fatalf("search organisation_id = %v, want %s", row.OrganisationID, orgID)
			}
		}
		if response.Projects[2].OrganisationID != nil {
			t.Fatal("unassigned search project must have null organisation_id")
		}
	}
}
