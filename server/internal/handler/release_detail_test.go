package handler

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestReleaseIssuesCanonicalLaneOrderRetainsTerminalLaneless(t *testing.T) {
	_, repo, release := milestoneReleaseFixture(t, testWorkspaceID)
	main := dbfx.Insert(t, "worklane", testutil.Cols{"workspace_id": testWorkspaceID, "repository_id": repo, "name": "main", "is_main": true, "position": 100})
	later := dbfx.Insert(t, "worklane", testutil.Cols{"workspace_id": testWorkspaceID, "repository_id": repo, "name": "Later", "position": 2})
	earlier := dbfx.Insert(t, "worklane", testutil.Cols{"workspace_id": testWorkspaceID, "repository_id": repo, "name": "Earlier", "position": 1})
	last := dbfx.Issue(t, "Terminal retained after lane deletion", testutil.Cols{"release_id": release, "status": "cancelled"})
	third := dbfx.Issue(t, "Later lane", testutil.Cols{"release_id": release, "worklane_id": later, "lane_position": 0})
	second := dbfx.Issue(t, "Earlier lane", testutil.Cols{"release_id": release, "worklane_id": earlier, "lane_position": 0})
	first := dbfx.Issue(t, "Main lane", testutil.Cols{"release_id": release, "worklane_id": main, "lane_position": 10})
	var out struct {
		Issues []ReleaseIssueResponse `json:"issues"`
	}
	testutil.Call(t, testHandler.ListReleaseIssues, milestoneRequest("GET", release, nil)).Want(200).JSON(&out)
	var ids []string
	for _, issue := range out.Issues {
		ids = append(ids, issue.ID)
		if issue.Identifier == "" {
			t.Fatalf("issue lacks identifier: %+v", issue)
		}
	}
	if want := []string{first, second, third, last}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("release issue order = %v; want %v", ids, want)
	}
	if out.Issues[3].StatusCategory != "closed" || out.Issues[3].WorklaneID != nil {
		t.Fatalf("terminal laneless projection = %+v", out.Issues[3])
	}
}

func TestReleaseDetailScopeAndEmptyCollections(t *testing.T) {
	_, _, release := milestoneReleaseFixture(t, testWorkspaceID)
	var out struct {
		Issues []ReleaseIssueResponse `json:"issues"`
	}
	testutil.Call(t, testHandler.ListReleaseIssues, milestoneRequest("GET", release, nil)).Want(200).JSON(&out)
	if out.Issues == nil || len(out.Issues) != 0 {
		t.Fatalf("empty issues = %#v", out.Issues)
	}
	otherWorkspace := dbfx.Workspace(t, "Other release scope", "release-scope-"+uuid.NewString())
	_, _, other := milestoneReleaseFixture(t, otherWorkspace)
	testutil.Call(t, testHandler.GetRelease, milestoneRequest("GET", other, nil)).Want(404)
	testutil.Call(t, testHandler.ListReleaseIssues, milestoneRequest("GET", other, nil)).Want(404)
	testutil.Call(t, testHandler.ListReleaseActivity, milestoneRequest("GET", other, nil)).Want(404)
}
