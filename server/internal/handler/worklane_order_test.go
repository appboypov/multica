package handler

import (
	"math"
	"net/http"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func laneOrderFixture(t *testing.T) (string, string) {
	t.Helper()
	project := dbfx.Project(t, "Lane order")
	repo := dbfx.Insert(t, "repository", testutil.Cols{
		"workspace_id": testWorkspaceID, "project_id": project, "github_repo_id": 1,
		"full_name": "test/" + uuid.NewString(), "private": true, "default_branch": "main",
		"checkout_daemon_id": "test", "checkout_path": "/test", "version_policy": "none", "last_tag": "",
	})
	lane := dbfx.Insert(t, "worklane", testutil.Cols{"workspace_id": testWorkspaceID, "repository_id": repo, "name": "main", "is_main": true})
	return repo, lane
}

func laneOrderRequest(t *testing.T, handler http.HandlerFunc, method, lane, issue string, body any, status int, out any) {
	t.Helper()
	r := testutil.WithURLParams(newRequest(method, "/api/worklanes/"+lane+"/issues", body), "id", lane, "issueId", issue)
	call := func(w http.ResponseWriter, r *http.Request) {
		middleware.RequireWorkspaceMember(testHandler.Queries)(handler).ServeHTTP(w, r)
	}
	response := testutil.Call(t, call, r).Want(status)
	if out != nil {
		response.JSON(out)
	}
}

func TestWorklaneMovePersistsOrderAndHead(t *testing.T) {
	_, lane := laneOrderFixture(t)
	first := dbfx.Issue(t, "first", testutil.Cols{"worklane_id": lane, "lane_position": 1})
	second := dbfx.Issue(t, "second", testutil.Cols{"worklane_id": lane, "lane_position": 2})
	third := dbfx.Issue(t, "third", testutil.Cols{"worklane_id": lane, "lane_position": 3})
	var result struct {
		Issues []IssueResponse `json:"issues"`
	}
	laneOrderRequest(t, testHandler.MoveWorklaneIssue, "POST", lane, third, map[string]string{"before": first}, 200, &result)
	want := []string{third, first, second}
	ids := func(issues []IssueResponse) []string {
		out := []string{}
		for _, issue := range issues {
			out = append(out, issue.ID)
		}
		return out
	}
	if got := ids(result.Issues); !reflect.DeepEqual(got, want) {
		t.Fatalf("move order = %v, want %v", got, want)
	}
	laneOrderRequest(t, testHandler.ListWorklaneIssues, "GET", lane, "", nil, 200, &result)
	if got := ids(result.Issues); !reflect.DeepEqual(got, want) {
		t.Fatalf("reloaded order = %v, want %v", got, want)
	}
	var head WorklaneResponse
	laneOrderRequest(t, testHandler.GetWorklane, "GET", lane, "", nil, 200, &head)
	if head.NextIssue == nil || head.NextIssue.ID != third {
		t.Fatalf("head = %+v, want %s", head.NextIssue, third)
	}
	laneOrderRequest(t, testHandler.MoveWorklaneIssue, "POST", lane, third, map[string]string{"after": second}, 200, &result)
	if got := ids(result.Issues); !reflect.DeepEqual(got, []string{first, second, third}) {
		t.Fatalf("after order = %v", got)
	}
}

func TestWorklaneMoveRejectsInvalidTargets(t *testing.T) {
	repo, lane := laneOrderFixture(t)
	_, otherLane := laneOrderFixture(t)
	first := dbfx.Issue(t, "first", testutil.Cols{"worklane_id": lane, "lane_position": 1})
	outside := dbfx.Issue(t, "outside", testutil.Cols{"worklane_id": otherLane})
	release := dbfx.Insert(t, "release", testutil.Cols{"workspace_id": testWorkspaceID, "repository_id": repo, "position": 1, "version": "1.0.0", "title": "Release", "description": ""})
	released := dbfx.Issue(t, "released", testutil.Cols{"worklane_id": lane, "release_id": release, "lane_position": 2})
	for _, tc := range []struct {
		name string
		body any
		want int
	}{
		{"missing", map[string]string{}, 400},
		{"both", map[string]string{"before": outside, "after": released}, 400},
		{"self", map[string]string{"before": first}, 400},
		{"malformed", map[string]string{"before": "not-an-id"}, 400},
		{"another lane", map[string]string{"before": outside}, 404},
		{"another release", map[string]string{"before": released}, 409},
	} {
		t.Run(tc.name, func(t *testing.T) {
			laneOrderRequest(t, testHandler.MoveWorklaneIssue, "POST", lane, first, tc.body, tc.want, nil)
		})
	}
	laneOrderRequest(t, testHandler.MoveWorklaneIssue, "POST", otherLane, first, map[string]string{"before": outside}, 404, nil)
	foreign := dbfx.Workspace(t, "Foreign lane", "foreign-lane-"+uuid.NewString())
	foreignLane := dbfx.Insert(t, "worklane", testutil.Cols{"workspace_id": foreign, "repository_id": repo, "name": "foreign"})
	laneOrderRequest(t, testHandler.MoveWorklaneIssue, "POST", foreignLane, first, map[string]string{"before": released}, 404, nil)
	var position float64
	dbfx.QueryRow(t, "SELECT lane_position FROM issue WHERE id=$1", first).Scan(&position)
	if position != 1 {
		t.Fatalf("rejected move changed position to %v", position)
	}
}

func TestIssueLanePlacementAppendsAndClears(t *testing.T) {
	_, lane := laneOrderFixture(t)
	dbfx.Issue(t, "first", testutil.Cols{"worklane_id": lane, "lane_position": 9, "number": nextWorkspaceIssueNumber(t)})
	var created IssueResponse
	testutil.Call(t, testHandler.CreateIssue, newRequest("POST", "/api/issues", map[string]any{"title": "append", "worklane_id": lane})).Want(201).JSON(&created)
	dbfx.Cleanup(t, "DELETE FROM issue WHERE id=$1", created.ID)
	var position *float64
	dbfx.QueryRow(t, "SELECT lane_position FROM issue WHERE id=$1", created.ID).Scan(&position)
	if position == nil || *position != 10 {
		t.Fatalf("created lane position = %v, want 10", position)
	}
	r := withURLParam(newRequest("PUT", "/api/issues/"+created.ID+"/placement", map[string]any{"worklane_id": nil, "release_id": nil}), "id", created.ID)
	testutil.Call(t, testHandler.SetIssuePlacement, r).Want(200)
	dbfx.QueryRow(t, "SELECT lane_position FROM issue WHERE id=$1", created.ID).Scan(&position)
	if position != nil {
		t.Fatalf("removed worklane retained position %v", *position)
	}
	r = withURLParam(newRequest("PUT", "/api/issues/"+created.ID+"/placement", map[string]any{"worklane_id": lane, "release_id": nil}), "id", created.ID)
	testutil.Call(t, testHandler.SetIssuePlacement, r).Want(200)
	dbfx.QueryRow(t, "SELECT lane_position FROM issue WHERE id=$1", created.ID).Scan(&position)
	if position == nil || *position != 10 {
		t.Fatalf("reassigned lane position = %v, want 10", position)
	}
}

func TestLaneMovePositionFractionAndExhaustion(t *testing.T) {
	id := parseUUID(uuid.NewString())
	left, right := parseUUID(uuid.NewString()), parseUUID(uuid.NewString())
	rows := []db.ListWorklaneOrderRow{{ID: left, LanePosition: pgtype.Float8{Float64: 1, Valid: true}}, {ID: id}, {ID: right, LanePosition: pgtype.Float8{Float64: 2, Valid: true}}}
	if position, ok := laneMovePosition(rows, id); !ok || position != 1.5 {
		t.Fatalf("midpoint = %v, %v", position, ok)
	}
	rows[2].LanePosition.Float64 = math.Nextafter(1, 2)
	if _, ok := laneMovePosition(rows, id); ok {
		t.Fatal("exhausted float gap must normalize")
	}
	rows[2].LanePosition.Valid = false
	if _, ok := laneMovePosition(rows, id); ok {
		t.Fatal("null neighbor must normalize")
	}
}

func TestWorklaneTableOrderPaginatesReleaseThenLane(t *testing.T) {
	repo, lane := laneOrderFixture(t)
	release := dbfx.Insert(t, "release", testutil.Cols{"workspace_id": testWorkspaceID, "repository_id": repo, "position": 1, "version": "1.0.0", "title": "Release", "description": ""})
	last := dbfx.Issue(t, "no release", testutil.Cols{"worklane_id": lane, "lane_position": -1, "position": -100})
	second := dbfx.Issue(t, "second", testutil.Cols{"worklane_id": lane, "release_id": release, "lane_position": 2, "position": 1})
	first := dbfx.Issue(t, "first", testutil.Cols{"worklane_id": lane, "release_id": release, "lane_position": 1, "position": 100})
	var cursor *string
	var order []string
	for range 3 {
		var response issueTableRowsResponse
		testutil.Call(t, testHandler.ListIssueTableRows, newRequest("POST", "/api/issues/table/rows", issueTableRowsRequest{
			Query: issueTableQuerySpec{
				Scope: issueTableScope{Kind: "worklane", WorklaneID: lane},
				Sort:  issueTableSortRequest{Field: "position", Direction: "asc"},
			},
			Group: issueTableGroupSpec{Kind: "none"},
			Page:  issueTablePageRequest{Limit: 1, Cursor: cursor},
		})).Want(200).JSON(&response)
		for _, row := range response.Rows {
			order = append(order, row.Issue.ID)
		}
		cursor = response.NextCursor
		if cursor == nil {
			break
		}
	}
	if !reflect.DeepEqual(order, []string{first, second, last}) || cursor != nil {
		t.Fatalf("paginated lane order = %v, cursor = %v", order, cursor)
	}
}
