package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/integrations/ghsnapshot"
	"github.com/multica-ai/multica/server/internal/testutil"
)

type snapshotRoundTripper func(*http.Request) (*http.Response, error)

func (fn snapshotRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func TestPRRefreshMissedMergeCompletesIssueOnce(t *testing.T) {
	if testHandler == nil {
		t.Skip("handler test fixture not initialized")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	issueID := dbfx.Issue(t, "Missed merge", testutil.Cols{"status": "in_progress"})
	prID := dbfx.Insert(t, "github_pull_request", testutil.Cols{
		"workspace_id": testWorkspaceID, "installation_id": int64(1867001),
		"repo_owner": "acme", "repo_name": "refresh", "pr_number": 1,
		"title": "Manual delivery link", "state": "open", "html_url": "https://github.com/acme/refresh/pull/1",
		"head_sha": "head", "pr_created_at": "2026-10-18T00:00:00Z", "pr_updated_at": "2026-10-18T00:00:00Z",
	})
	dbfx.InsertNoID(t, "issue_pull_request", testutil.Cols{
		"issue_id": issueID, "pull_request_id": prID, "linked_by_type": "member",
	}, "issue_id=$1 AND pull_request_id=$2", issueID, prID)
	dbfx.Cleanup(t, `DELETE FROM activity_log WHERE issue_id=$1`, issueID)

	key, _ := generateTestRSAKeyPEM(t)
	t.Setenv("GITHUB_APP_ID", "1")
	t.Setenv("GITHUB_APP_PRIVATE_KEY", string(key))
	transport := http.DefaultTransport
	http.DefaultTransport = snapshotRoundTripper(func(r *http.Request) (*http.Response, error) {
		body := `{"data":{"repository":{"pullRequest":{"headRefOid":"head","state":"MERGED","isDraft":false,"mergedAt":"2026-10-19T11:11:00Z","closedAt":"2026-10-19T11:11:00Z","updatedAt":"2026-10-19T11:11:00Z","baseRefName":"main","headRefName":"work-1","mergeable":"UNKNOWN","mergeStateStatus":"UNKNOWN","commits":{"nodes":[]}}}}}`
		status := http.StatusOK
		if strings.HasSuffix(r.URL.Path, "/access_tokens") {
			body = fmt.Sprintf(`{"token":"test-token","expires_at":%q}`, time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
			status = http.StatusCreated
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = transport })
	client, err := ghsnapshot.NewClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	applied := make(chan struct{}, 1)
	manager := ghsnapshot.NewManager(client, testHandler.Queries, testPool, func(ctx context.Context, id pgtype.UUID, previousState string, snap *ghsnapshot.PRSnapshot) {
		testHandler.broadcastPRSnapshotApplied(ctx, id, previousState, snap)
		applied <- struct{}{}
	})
	manager.Start(ctx)
	refresh := func() {
		t.Helper()
		manager.Enqueue(1867001, "acme", "refresh", 1)
		select {
		case <-applied:
		case <-time.After(5 * time.Second):
			t.Fatal("snapshot refresh did not complete")
		}
	}
	refresh()
	if got := issueStatusForTest(t, issueID); got != "done" {
		t.Fatalf("issue after missed merge refresh = %q, want done", got)
	}
	stale := &ghPullRequestPayload{}
	stale.Repository.Owner.Login, stale.Repository.Name = "acme", "refresh"
	stale.PullRequest.Number, stale.PullRequest.Head.SHA = 1, "head"
	stale.PullRequest.State = "open"
	stale.PullRequest.UpdatedAt = "2026-10-18T00:00:00Z"
	testHandler.mirrorPullRequestForWorkspace(ctx, parseUUID(testWorkspaceID), 1867001, stale, prLinkPolicy{unrestricted: true})
	var state string
	dbfx.QueryRow(t, `SELECT state FROM github_pull_request WHERE id=$1`, prID).Scan(&state)
	if state != "merged" {
		t.Fatalf("late pre-merge webhook regressed the refreshed PR to %q", state)
	}
	dbfx.Exec(t, `UPDATE issue SET status='in_progress' WHERE id=$1`, issueID)
	refresh()
	if got := issueStatusForTest(t, issueID); got != "in_progress" {
		t.Fatalf("repeat snapshot re-completed a reopened issue: %q", got)
	}
}
