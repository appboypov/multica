package handler

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestShipReleaseRecordsPublicationAndFreezesVersion(t *testing.T) {
	_, repo, release := milestoneReleaseFixture(t, testWorkspaceID)
	next := dbfx.Insert(t, "release", testutil.Cols{"workspace_id": testWorkspaceID, "repository_id": repo, "position": 1, "version": "1.0.1", "bump": "patch", "title": "Next"})
	var out struct {
		Releases []ReleaseResponse `json:"releases"`
	}
	testutil.Call(t, testHandler.ShipRelease, milestoneRequest("POST", next, nil)).Want(409)
	before := time.Now().Add(-time.Second)
	testutil.Call(t, testHandler.ShipRelease, milestoneRequest("POST", release, nil)).Want(200).JSON(&out)
	if len(out.Releases) != 2 || out.Releases[0].ShippedAt == nil || out.Releases[1].ShippedAt != nil || out.Releases[1].Version != "1.0.1" {
		t.Fatalf("shipped line = %+v", out.Releases)
	}
	stamp, err := time.Parse(time.RFC3339, *out.Releases[0].ShippedAt)
	if err != nil || stamp.Before(before) || stamp.After(time.Now()) {
		t.Fatalf("ship timestamp = %q, %v", *out.Releases[0].ShippedAt, err)
	}
	if count := dbfx.Count(t, `SELECT count(*) FROM activity_log WHERE repository_id = $1 AND action = 'release_shipped' AND details->>'version' = '1.0.0'`, repo); count != 1 {
		t.Fatalf("shipping activities = %d", count)
	}
	testutil.Call(t, testHandler.ShipRelease, milestoneRequest("POST", release, nil)).Want(409)
	testutil.Call(t, testHandler.EditRelease, milestoneRequest("PATCH", release, map[string]string{"title": "Published", "version": "2.0.0"})).Want(409)
	testutil.Call(t, testHandler.EditRelease, milestoneRequest("PATCH", release, map[string]string{"title": "Published", "description": "Published note", "version": "1.0.0"})).Want(200)
	testutil.Call(t, testHandler.MoveRelease, milestoneRequest("POST", release, map[string]string{"direction": "later"})).Want(409)
	testutil.Call(t, testHandler.MoveRelease, milestoneRequest("POST", next, map[string]string{"direction": "earlier"})).Want(409)
	testutil.Call(t, testHandler.RemoveRelease, milestoneRequest("DELETE", release, nil)).Want(409)
	testutil.Call(t, testHandler.ShipRelease, milestoneRequest("POST", next, nil)).Want(200)
	var got ReleaseResponse
	testutil.Call(t, testHandler.GetRelease, milestoneRequest("GET", release, nil)).Want(200).JSON(&got)
	if got.Version != "1.0.0" || got.Title != "Published" || got.ShippedAt == nil || *got.ShippedAt != *out.Releases[0].ShippedAt {
		t.Fatalf("persisted shipped release = %+v", got)
	}
}

func TestShipReleaseRefusesUnfinishedIssuesAndUnmergedPRs(t *testing.T) {
	_, _, release := milestoneReleaseFixture(t, testWorkspaceID)
	issue := dbfx.Issue(t, "Not done", testutil.Cols{"release_id": release, "status": "todo"})
	var refusal struct {
		Code   string              `json:"code"`
		Issues []IssueRefusalIssue `json:"issues"`
	}
	testutil.Call(t, testHandler.ShipRelease, milestoneRequest("POST", release, nil)).Want(409).JSON(&refusal)
	if refusal.Code != "release_not_ready" || len(refusal.Issues) != 1 || refusal.Issues[0].ID != issue {
		t.Fatalf("readiness refusal = %+v", refusal)
	}
	dbfx.Exec(t, `UPDATE issue SET status = 'done' WHERE id = $1`, issue)
	pr := dbfx.Insert(t, "github_pull_request", testutil.Cols{
		"workspace_id": testWorkspaceID, "installation_id": 1, "repo_owner": "test", "repo_name": "ship",
		"pr_number": 1, "title": "Not merged", "state": "open", "html_url": "https://github.com/test/ship/pull/1",
		"pr_created_at": time.Now(), "pr_updated_at": time.Now(),
	})
	dbfx.InsertNoID(t, "issue_pull_request", testutil.Cols{"issue_id": issue, "pull_request_id": pr}, "issue_id = $1 AND pull_request_id = $2", issue, pr)
	testutil.Call(t, testHandler.ShipRelease, milestoneRequest("POST", release, nil)).Want(409)
	dbfx.Exec(t, `UPDATE github_pull_request SET state = 'merged' WHERE id = $1`, pr)
	testutil.Call(t, testHandler.ShipRelease, milestoneRequest("POST", release, nil)).Want(200)
}

func TestShipReleaseWorkspaceScopeAndRepositoryGate(t *testing.T) {
	ws := dbfx.Workspace(t, "Other shipping workspace", "ship-"+uuid.NewString())
	_, _, foreign := milestoneReleaseFixture(t, ws)
	for _, id := range []string{foreign, uuid.NewString()} {
		testutil.Call(t, testHandler.ShipRelease, milestoneRequest("POST", id, nil)).Want(404)
	}
	testutil.Call(t, testHandler.ShipRelease, milestoneRequest("POST", "invalid", nil)).Want(400)
	_, repo, release := milestoneReleaseFixture(t, testWorkspaceID)
	dbfx.Exec(t, `UPDATE repository SET version_policy = 'none' WHERE id = $1`, repo)
	testutil.Call(t, testHandler.ShipRelease, milestoneRequest("POST", release, nil)).Want(409)
}

func TestShipReleaseSerializesOnRepositoryLock(t *testing.T) {
	_, repo, release := milestoneReleaseFixture(t, testWorkspaceID)
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT id FROM repository WHERE id = $1 FOR UPDATE`, repo); err != nil {
		t.Fatal(err)
	}
	results := make(chan int, 2)
	for range 2 {
		go func() {
			recorder := httptest.NewRecorder()
			testHandler.ShipRelease(recorder, milestoneRequest("POST", release, nil))
			results <- recorder.Code
		}()
	}
	select {
	case status := <-results:
		t.Fatalf("ship bypassed repository lock: %d", status)
	case <-time.After(100 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	counts := map[int]int{}
	for range 2 {
		select {
		case status := <-results:
			counts[status]++
		case <-time.After(5 * time.Second):
			t.Fatal("shipping did not finish after unlock")
		}
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatalf("concurrent ship statuses = %v", counts)
	}
}
