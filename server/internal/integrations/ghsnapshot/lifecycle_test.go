package ghsnapshot

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestRefreshReconcilesPullRequestLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name, initial, state, want string
		draft                      bool
		mergedAt, closedAt         string
	}{
		{name: "merged", initial: "open", state: "MERGED", want: "merged", mergedAt: "2026-10-19T11:11:00Z", closedAt: "2026-10-19T11:11:00Z"},
		{name: "closed", initial: "open", state: "CLOSED", want: "closed", closedAt: "2026-10-19T11:11:00Z"},
		{name: "draft", initial: "open", state: "OPEN", draft: true, want: "draft"},
		{name: "reopened", initial: "closed", state: "OPEN", want: "open"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, q, pr := lifecyclePR(t)
			if _, err := pool.Exec(context.Background(), `UPDATE github_pull_request SET state=$2, closed_at=$3 WHERE id=$1`, pr.ID, tc.initial, time.Unix(1_700_000_000, 0)); err != nil {
				t.Fatal(err)
			}
			srv := graphqlServer(t, func(map[string]any) string {
				return fmt.Sprintf(`{"repository":{"pullRequest":{"headRefOid":"head","state":%q,"isDraft":%t,"mergedAt":%s,"closedAt":%s,"updatedAt":"2026-10-19T11:12:00Z","baseRefName":"main","headRefName":"work-1","mergeable":"UNKNOWN","mergeStateStatus":"UNKNOWN","commits":{"nodes":[]}}}}`, tc.state, tc.draft, nullableJSONTime(tc.mergedAt), nullableJSONTime(tc.closedAt))
			})
			defer srv.Close()
			m := NewManager(newTestClient(t, srv.URL), q, pool, nil)
			m.jitter = func() time.Duration { return 0 }
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			m.ctx = ctx
			m.process(ctx, address{pr.InstallationID, pr.RepoOwner, pr.RepoName, pr.PrNumber})
			got, err := q.GetGitHubPullRequestByID(ctx, pr.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.State != tc.want {
				t.Errorf("state = %q, want %q", got.State, tc.want)
			}
			for name, value := range map[string]struct {
				valid bool
				value time.Time
				want  string
			}{
				"merged_at": {got.MergedAt.Valid, got.MergedAt.Time, tc.mergedAt},
				"closed_at": {got.ClosedAt.Valid, got.ClosedAt.Time, tc.closedAt},
			} {
				if value.valid != (value.want != "") || (value.valid && value.value.UTC().Format(time.RFC3339) != value.want) {
					t.Errorf("%s = %v (valid=%v), want %q", name, value.value, value.valid, value.want)
				}
			}
			if !got.PrUpdatedAt.Time.Equal(time.Date(2026, 10, 19, 11, 12, 0, 0, time.UTC)) {
				t.Errorf("PR update timestamp = %v, want GitHub updatedAt", got.PrUpdatedAt)
			}
			if tc.want == "merged" || tc.want == "closed" {
				if m.attempts[address{pr.InstallationID, pr.RepoOwner, pr.RepoName, pr.PrNumber}] != 0 {
					t.Error("terminal PR started an undecided snapshot chase")
				}
			}
		})
	}
}

func nullableJSONTime(value string) string {
	if value == "" {
		return "null"
	}
	return fmt.Sprintf("%q", value)
}

func lifecyclePR(t *testing.T) (*pgxpool.Pool, *db.Queries, db.GithubPullRequest) {
	t.Helper()
	pool := testDBPool(t)
	q := db.New(pool)
	fx := testutil.New(pool, "", "")
	wsID := fx.Workspace(t, "Snapshot lifecycle", "snapshot-"+randHex(t))
	id := fx.Insert(t, "github_pull_request", testutil.Cols{
		"workspace_id": wsID, "installation_id": int64(1867),
		"repo_owner": "o", "repo_name": "lifecycle", "pr_number": 1,
		"title": "Lifecycle", "state": "open", "html_url": "https://github.com/o/lifecycle/pull/1",
		"head_sha": "head", "pr_created_at": time.Unix(1_700_000_000, 0), "pr_updated_at": time.Unix(1_700_000_000, 0),
	})
	var prID pgtype.UUID
	if err := prID.Scan(id); err != nil {
		t.Fatal(err)
	}
	pr, err := q.GetGitHubPullRequestByID(context.Background(), prID)
	if err != nil {
		t.Fatal(err)
	}
	return pool, q, pr
}

func TestApplySnapshotRejectsOlderLifecycle(t *testing.T) {
	pool, q, pr := lifecyclePR(t)
	ctx := context.Background()
	newer := time.Unix(1_700_000_200, 0)
	fx := testutil.New(pool, "", "")
	fx.Exec(t, `UPDATE github_pull_request SET state='merged', merged_at=$2, closed_at=$2, pr_updated_at=$2 WHERE id=$1`, pr.ID, newer)
	m := &Manager{queries: q, pool: pool, now: time.Now}
	applied, err := m.applySnapshot(ctx, pr.ID, &PRSnapshot{
		HeadSHA: "head", State: "open", UpdatedAt: newer.Add(-time.Minute),
		Mergeable: "MERGEABLE", MergeStateStatus: "CLEAN",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := q.GetGitHubPullRequestByID(ctx, pr.ID)
	if err != nil {
		t.Fatal(err)
	}
	if applied || got.State != "merged" || !got.MergedAt.Time.Equal(newer) || !got.ClosedAt.Time.Equal(newer) || got.SnapshotFetchedAt.Valid {
		t.Fatalf("older same-head response overwrote a merge: applied=%v PR=%+v", applied, got)
	}
}

func TestApplySnapshotRollsBackLifecycleOnCheckFailure(t *testing.T) {
	pool, q, pr := lifecyclePR(t)
	ctx := context.Background()
	mergedAt := time.Unix(1_700_000_200, 0)
	fx := testutil.New(pool, "", "")
	fx.InsertNoID(t, "github_pull_request_check_run", testutil.Cols{
		"pr_id": pr.ID, "head_sha": "head", "ordinal": 0, "name": "original", "status": "in_progress",
	}, "pr_id=$1", pr.ID)
	m := &Manager{queries: q, pool: pool, now: time.Now}
	_, err := m.applySnapshot(ctx, pr.ID, &PRSnapshot{
		HeadSHA: "head", State: "merged", UpdatedAt: mergedAt, MergedAt: &mergedAt, ClosedAt: &mergedAt,
		Contexts: []CheckContext{{Name: "invalid\x00check", Status: "completed"}},
	})
	if err == nil {
		t.Fatal("a check name with a NUL byte must reject the batch")
	}
	got, err := q.GetGitHubPullRequestByID(ctx, pr.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "open" || got.MergedAt.Valid || got.ClosedAt.Valid || got.SnapshotFetchedAt.Valid || !got.PrUpdatedAt.Time.Equal(pr.PrUpdatedAt.Time) {
		t.Fatalf("failed check insert committed lifecycle data: %+v", got)
	}
	var checkName string
	fx.QueryRow(t, `SELECT name FROM github_pull_request_check_run WHERE pr_id=$1`, pr.ID).Scan(&checkName)
	if checkName != "original" {
		t.Fatalf("failed batch replaced the stored check: %q", checkName)
	}
}
