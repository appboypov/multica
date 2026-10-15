package handler

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func milestoneReleaseFixture(t *testing.T, ws string) (project, repository, release string) {
	t.Helper()
	project = dbfx.Project(t, "Milestone project", testutil.Cols{"workspace_id": ws})
	repository = dbfx.Insert(t, "repository", testutil.Cols{
		"workspace_id": ws, "project_id": project, "github_repo_id": 1,
		"full_name": "test/" + uuid.NewString(), "private": false,
		"default_branch": "main", "checkout_daemon_id": "test", "checkout_path": "/tmp/test",
		"version_policy": "tags", "last_tag": "",
	})
	release = dbfx.Insert(t, "release", testutil.Cols{
		"workspace_id": ws, "repository_id": repository, "position": 0,
		"version": "1.0.0", "title": "Milestone release",
	})
	return
}
func milestoneRequest(method, id string, body any) *http.Request {
	return withURLParam(newRequest(method, "/api/releases/"+id, body), "id", id)
}
func milestoneFromLine(t *testing.T, line []ReleaseResponse, release string) []ReleaseMilestoneResponse {
	t.Helper()
	for _, row := range line {
		if row.ID == release {
			return row.Milestones
		}
	}
	t.Fatalf("release %s absent from response", release)
	return nil
}

func TestReleaseMilestonesCRUDAndOrdering(t *testing.T) {
	_, _, release := milestoneReleaseFixture(t, testWorkspaceID)
	var fetched ReleaseResponse
	testutil.Call(t, testHandler.GetRelease, milestoneRequest("GET", release, nil)).Want(200).JSON(&fetched)
	if fetched.Milestones == nil || len(fetched.Milestones) != 0 {
		t.Fatalf("empty milestones = %#v", fetched.Milestones)
	}
	var line struct {
		Releases []ReleaseResponse `json:"releases"`
	}
	for _, body := range []map[string]string{
		{"name": "UI prototype", "date": "2026-11-02"},
		{"name": "Acceptance test on beta", "date": "2026-11-09"},
		{"name": "Early review", "date": "2026-11-02"},
	} {
		testutil.Call(t, testHandler.AddReleaseMilestone, milestoneRequest("POST", release, body)).Want(201).JSON(&line)
	}
	milestones := milestoneFromLine(t, line.Releases, release)
	if len(milestones) != 3 || milestones[0].Name != "Early review" || milestones[1].Name != "UI prototype" || milestones[2].Date != "2026-11-09" {
		t.Fatalf("milestones not ordered by date then name: %+v", milestones)
	}
	for _, row := range milestones {
		dbfx.Cleanup(t, `DELETE FROM release_milestone WHERE id = $1`, row.ID)
	}
	id := milestones[2].ID
	testutil.Call(t, testHandler.EditReleaseMilestone, milestoneRequest("PATCH", id, map[string]string{"date": "2026-10-31"})).Want(200).JSON(&line)
	milestones = milestoneFromLine(t, line.Releases, release)
	if milestones[0].ID != id || milestones[0].Name != "Acceptance test on beta" || milestones[0].Date != "2026-10-31" {
		t.Fatalf("partial date edit = %+v", milestones)
	}
	testutil.Call(t, testHandler.EditReleaseMilestone, milestoneRequest("PATCH", id, map[string]string{"name": "Beta acceptance"})).Want(200)
	testutil.Call(t, testHandler.GetRelease, milestoneRequest("GET", release, nil)).Want(200).JSON(&fetched)
	if len(fetched.Milestones) != 3 || fetched.Milestones[0].Name != "Beta acceptance" {
		t.Fatalf("GET milestones = %+v", fetched.Milestones)
	}
	testutil.Call(t, testHandler.ListReleases, newRequest("GET", "/api/releases", nil)).Want(200).JSON(&line)
	if got := milestoneFromLine(t, line.Releases, release); len(got) != 3 || got[0].ID != id {
		t.Fatalf("list milestones = %+v", got)
	}
	dbfx.Issue(t, "Milestone progress", testutil.Cols{"release_id": release})
	var progress struct {
		Releases []struct {
			ReleaseID  string                     `json:"release_id"`
			Milestones []ReleaseMilestoneResponse `json:"milestones"`
		} `json:"releases"`
	}
	testutil.Call(t, testHandler.ListReleaseProgress, newRequest("GET", "/api/release-progress", nil)).Want(200).JSON(&progress)
	found := false
	for _, total := range progress.Releases {
		if total.ReleaseID == release {
			found = true
			if len(total.Milestones) != 3 || total.Milestones[0].ID != id {
				t.Fatalf("progress milestones = %+v", total.Milestones)
			}
		}
	}
	if !found {
		t.Fatal("release missing from progress")
	}
	testutil.Call(t, testHandler.RemoveReleaseMilestone, milestoneRequest("DELETE", id, nil)).Want(200).JSON(&line)
	if got := milestoneFromLine(t, line.Releases, release); len(got) != 2 {
		t.Fatalf("after removal = %+v", got)
	}
	testutil.Call(t, testHandler.RemoveRelease, milestoneRequest("DELETE", release, nil)).Want(200)
	if count := dbfx.Count(t, `SELECT count(*) FROM release_milestone WHERE release_id = $1`, release); count != 0 {
		t.Fatalf("release deletion left %d milestones", count)
	}
}

func TestReleaseMilestonesValidationAndWorkspaceScope(t *testing.T) {
	_, _, release := milestoneReleaseFixture(t, testWorkspaceID)
	for _, body := range []map[string]any{
		{}, {"name": " ", "date": "2026-11-02"}, {"name": "UI"},
		{"name": "UI", "date": "2026-02-30"}, {"name": "UI", "date": "2026-11-02T00:00:00Z"},
		{"name": "UI", "date": "2026-1-02"}, {"name": 7, "date": "2026-11-02"},
	} {
		testutil.Call(t, testHandler.AddReleaseMilestone, milestoneRequest("POST", release, body)).Want(400)
	}
	ws := dbfx.Workspace(t, "Other milestone workspace", "milestones-"+uuid.NewString())
	_, _, foreign := milestoneReleaseFixture(t, ws)
	id := dbfx.Insert(t, "release_milestone", testutil.Cols{"workspace_id": ws, "release_id": foreign, "name": "Foreign", "date": "2026-11-02"})
	testutil.Call(t, testHandler.GetRelease, milestoneRequest("GET", foreign, nil)).Want(404)
	testutil.Call(t, testHandler.AddReleaseMilestone, milestoneRequest("POST", foreign, map[string]string{"name": "No", "date": "2026-11-02"})).Want(404)
	testutil.Call(t, testHandler.EditReleaseMilestone, milestoneRequest("PATCH", id, map[string]string{"name": "No"})).Want(404)
	testutil.Call(t, testHandler.RemoveReleaseMilestone, milestoneRequest("DELETE", id, nil)).Want(404)
	testutil.Call(t, testHandler.AddReleaseMilestone, milestoneRequest("POST", uuid.NewString(), map[string]string{"name": "No", "date": "2026-11-02"})).Want(404)
	if count := dbfx.Count(t, `SELECT count(*) FROM release_milestone WHERE release_id = $1`, release); count != 0 {
		t.Fatalf("invalid requests wrote %d milestones", count)
	}
}

func TestReleaseMilestonesDeletedWithParents(t *testing.T) {
	for _, parent := range []string{"repository", "project", "workspace"} {
		t.Run(parent, func(t *testing.T) {
			ws := testWorkspaceID
			if parent == "workspace" {
				ws = dbfx.Workspace(t, "Milestone delete workspace", "milestone-delete-"+uuid.NewString())
				dbfx.Insert(t, "member", testutil.Cols{"workspace_id": ws, "user_id": testUserID, "role": "owner"})
			}
			project, repo, release := milestoneReleaseFixture(t, ws)
			dbfx.Insert(t, "release_milestone", testutil.Cols{"workspace_id": ws, "release_id": release, "name": "Remove with parent", "date": "2026-11-02"})
			switch parent {
			case "repository":
				testutil.Call(t, testHandler.DeleteRepository, milestoneRequest("DELETE", repo, nil)).Want(204)
			case "project":
				testutil.Call(t, testHandler.DeleteProject, milestoneRequest("DELETE", project, nil)).Want(204)
			case "workspace":
				testutil.Call(t, testHandler.DeleteWorkspace, milestoneRequest("DELETE", ws, nil)).Want(204)
			}
			if count := dbfx.Count(t, `SELECT count(*) FROM release_milestone WHERE release_id = $1`, release); count != 0 {
				t.Fatalf("%s deletion left %d milestones", parent, count)
			}
		})
	}
}
