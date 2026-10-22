package handler

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

// slugTestOrganisation inserts an organisation with a fresh key, so reruns
// and other tests never share its projects' slugs.
func slugTestOrganisation(t *testing.T, name string) (id, key string) {
	t.Helper()
	key = "o" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	return dbfx.Insert(t, "organisation", testutil.Cols{"workspace_id": testWorkspaceID, "name": name + " " + key, "key": key}), key
}

// wantErrorCode fails the test unless the error body carries code.
func wantCodeOf(t *testing.T, response *testutil.Response, code string) {
	t.Helper()
	if got := response.Map()["code"]; got != code {
		t.Fatalf("error code = %v, want %s (body %s)", got, code, response.Text())
	}
}

// A slug names the project's folder under its organisation's folder
// (ADR-0037): it follows the folder-name rule and is unique per organisation,
// not per workspace.
func TestProjectSlugValidationAndUniqueness(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	orgID, _ := slugTestOrganisation(t, "Slug org")
	otherOrgID, _ := slugTestOrganisation(t, "Other slug org")
	create := func(body map[string]any, want int) *testutil.Response {
		response := testutil.Call(t, testHandler.CreateProject, newRequest(http.MethodPost, "/api/projects", body)).Want(want)
		if want == http.StatusCreated {
			var created ProjectResponse
			response.JSON(&created)
			dbfx.Cleanup(t, `DELETE FROM project WHERE id = $1`, created.ID)
		}
		return response
	}
	update := func(id string, body map[string]any, want int) *testutil.Response {
		return testutil.Call(t, testHandler.UpdateProject, withURLParam(newRequest(http.MethodPut, "/api/projects/"+id, body), "id", id)).Want(want)
	}

	for _, bad := range []string{"Fan App", "-fan-app", "fan_app", "fan/app", "Fan-app"} {
		wantCodeOf(t, create(map[string]any{"title": "Bad slug", "organisation_id": orgID, "slug": bad}, http.StatusBadRequest), "project_slug_invalid")
	}

	var fan ProjectResponse
	create(map[string]any{"title": "Fan app", "organisation_id": orgID, "slug": " fan-app "}, http.StatusCreated).JSON(&fan)
	if fan.Slug == nil || *fan.Slug != "fan-app" {
		t.Fatalf("created slug = %v, want fan-app", fan.Slug)
	}
	wantCodeOf(t, create(map[string]any{"title": "Fan app again", "organisation_id": orgID, "slug": "fan-app"}, http.StatusConflict), "project_slug_taken")

	// Another organisation has its own folder, so the same slug is free there.
	var elsewhere ProjectResponse
	create(map[string]any{"title": "Fan app elsewhere", "organisation_id": otherOrgID, "slug": "fan-app"}, http.StatusCreated).JSON(&elsewhere)
	if elsewhere.Slug == nil || *elsewhere.Slug != "fan-app" {
		t.Fatalf("slug in the other organisation = %v, want fan-app", elsewhere.Slug)
	}

	var other ProjectResponse
	create(map[string]any{"title": "Back office", "organisation_id": orgID, "slug": "back-office"}, http.StatusCreated).JSON(&other)
	wantCodeOf(t, update(other.ID, map[string]any{"slug": "Back Office"}, http.StatusBadRequest), "project_slug_invalid")
	wantCodeOf(t, update(other.ID, map[string]any{"slug": "fan-app"}, http.StatusConflict), "project_slug_taken")
	// Moving a project into an organisation whose project holds its slug
	// collides as well.
	wantCodeOf(t, update(elsewhere.ID, map[string]any{"organisation_id": orgID}, http.StatusConflict), "project_slug_taken")

	var stored string
	dbfx.QueryRow(t, `SELECT slug FROM project WHERE id = $1`, other.ID).Scan(&stored)
	if stored != "back-office" {
		t.Fatalf("refused updates changed the slug to %q", stored)
	}
	var renamed ProjectResponse
	update(other.ID, map[string]any{"slug": "admin"}, http.StatusOK).JSON(&renamed)
	if renamed.Slug == nil || *renamed.Slug != "admin" {
		t.Fatalf("updated slug = %v, want admin", renamed.Slug)
	}
}

// A project's repositories live under its organisation and slug (ADR-0037),
// so neither can go while the project holds a repository, and a repository
// cannot join a project that lacks one.
func TestProjectFolderRequiredForRepositories(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	orgID, _ := slugTestOrganisation(t, "Folder org")
	update := func(id string, body map[string]any, want int) *testutil.Response {
		return testutil.Call(t, testHandler.UpdateProject, withURLParam(newRequest(http.MethodPut, "/api/projects/"+id, body), "id", id)).Want(want)
	}

	project := dbfx.Project(t, "Folder project", testutil.Cols{"organisation_id": orgID, "slug": "folder-project"})
	dbfx.Insert(t, "repository", testutil.Cols{
		"workspace_id": testWorkspaceID, "project_id": project, "github_repo_id": 1,
		"full_name": "test/r" + uuid.NewString(), "private": true, "default_branch": "main",
		"checkout_daemon_id": uuid.NewString(), "checkout_path": "/test/checkout", "version_policy": "none",
	})
	wantCodeOf(t, update(project, map[string]any{"organisation_id": nil}, http.StatusBadRequest), "project_organisation_required")
	wantCodeOf(t, update(project, map[string]any{"slug": nil}, http.StatusBadRequest), "project_slug_required")
	wantCodeOf(t, update(project, map[string]any{"slug": ""}, http.StatusBadRequest), "project_slug_required")
	var organisation, slug *string
	dbfx.QueryRow(t, `SELECT organisation_id::text, slug FROM project WHERE id = $1`, project).Scan(&organisation, &slug)
	if organisation == nil || *organisation != orgID || slug == nil || *slug != "folder-project" {
		t.Fatalf("refused updates left organisation %v, slug %v", organisation, slug)
	}
	// Edits that keep both still work.
	update(project, map[string]any{"title": "Folder project renamed"}, http.StatusOK)

	// Without repositories nothing needs the folder, so both may go.
	empty := dbfx.Project(t, "Empty folder project", testutil.Cols{"organisation_id": orgID, "slug": "empty-folder"})
	var cleared ProjectResponse
	update(empty, map[string]any{"organisation_id": nil, "slug": nil}, http.StatusOK).JSON(&cleared)
	if cleared.OrganisationID != nil || cleared.Slug != nil {
		t.Fatalf("cleared project organisation %v, slug %v", cleared.OrganisationID, cleared.Slug)
	}

	for _, tc := range []struct {
		name    string
		project string
		code    string
	}{
		{"no organisation", dbfx.Project(t, "No organisation", testutil.Cols{"slug": "no-organisation"}), "project_organisation_required"},
		{"no slug", dbfx.Project(t, "No slug", testutil.Cols{"organisation_id": orgID}), "project_slug_required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := testutil.Call(t, testHandler.CreateRepository, newRequest(http.MethodPost, "/api/repositories", map[string]any{
				"project_id": tc.project, "full_name": "test/r" + uuid.NewString(), "version_policy": "none", "checkout_check_id": "unused",
			})).Want(http.StatusBadRequest)
			wantCodeOf(t, response, tc.code)
			if n := dbfx.Count(t, `SELECT count(*) FROM repository WHERE project_id = $1`, tc.project); n != 0 {
				t.Fatalf("refused create left %d repositories", n)
			}
		})
	}
}
