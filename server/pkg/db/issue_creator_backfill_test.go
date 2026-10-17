package db_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func TestIssueCreatorBackfill(t *testing.T) {
	fx, _ := todoFixture(t)
	ctx := context.Background()
	runtime := fx.Runtime(t, "Creator backfill runtime")
	agent := fx.Agent(t, "Creator backfill agent", runtime)
	origin := backfillTask(t, fx, agent, runtime, testutil.Cols{"status": "completed"})
	unattributed := backfillTask(t, fx, agent, runtime, testutil.Cols{"originator_user_id": nil})
	// Parents the member left: one with its whole tree, one on its own.
	treeOptOut := fx.Issue(t, "Creator backfill tree opt-out", testutil.Cols{"creator_type": "agent", "creator_id": agent})
	issueOptOut := fx.Issue(t, "Creator backfill issue opt-out", testutil.Cols{"creator_type": "agent", "creator_id": agent})
	parents := map[int]string{3: treeOptOut, 4: issueOptOut}
	credited := make([]string, 91)
	for n := range credited {
		kind := "agent_create"
		if n%2 == 0 {
			kind = "quick_create"
		}
		cols := testutil.Cols{
			"creator_type": "agent", "creator_id": agent,
			"origin_type": kind, "origin_id": origin, "status": "in_review",
		}
		if parent, ok := parents[n]; ok {
			cols["parent_issue_id"] = parent
		}
		credited[n] = fx.Issue(t, fmt.Sprintf("Creator backfill %d", n), cols)
	}
	other := fx.User(t, "Existing creator", "creator-other-"+dbid.NewV7().String()+"@example.test")
	fx.Member(t, fx.WorkspaceID, other, "member")
	unchanged := []struct {
		name        string
		cols        testutil.Cols
		creatorType string
		creatorID   string
	}{
		{"member creator", testutil.Cols{"creator_type": "member", "creator_id": other, "origin_type": "quick_create", "origin_id": origin}, "member", other},
		{"no originator", testutil.Cols{"origin_type": "agent_create", "origin_id": unattributed}, "agent", agent},
		{"quick create without originator", testutil.Cols{"origin_type": "quick_create", "origin_id": unattributed}, "agent", agent},
		{"missing task", testutil.Cols{"origin_type": "agent_create", "origin_id": dbid.NewV7()}, "agent", agent},
		{"other origin type", testutil.Cols{"origin_type": "autopilot", "origin_id": origin}, "agent", agent},
		{"no origin", nil, "agent", agent},
	}
	ids := make([]string, len(unchanged))
	for n, tc := range unchanged {
		cols := testutil.Cols{"creator_type": "agent", "creator_id": agent}
		for key, value := range tc.cols {
			cols[key] = value
		}
		ids[n] = fx.Issue(t, tc.name, cols)
	}
	migration, err := os.ReadFile("../../migrations/753_issue_creator_backfill.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := fx.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Both backfills are table-wide, so roll them back before fixture cleanup.
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	// The member's delegated subscription on one credited issue, their opt-out on
	// another, and no row on the rest, as on issues from before the delegated rule.
	if _, err := tx.Exec(ctx, `
		INSERT INTO issue_subscriber (issue_id, user_type, user_id, reason, unsubscribed_at, opt_out_scope)
		VALUES ($1, 'member', $5, 'delegated', NULL, NULL), ($2, 'member', $5, 'delegated', now(), 'issue'),
		       ($3, 'member', $5, 'manual', now(), 'subtree'), ($4, 'member', $5, 'manual', now(), 'issue')
	`, credited[0], credited[1], treeOptOut, issueOptOut, fx.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("apply creator backfill: %v", err)
	}
	if got := backfillCount(t, tx, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND creator_type='member' AND creator_id=$2`, fx.WorkspaceID, fx.UserID); got != 91 {
		t.Fatalf("corrected creators = %d, want 91", got)
	}
	// An empty reason means no row.
	subscriptions := []struct {
		name, issueID, reason string
		unsubscribed          bool
	}{
		{"active delegated subscription", credited[0], "creator", false},
		{"opted-out subscription", credited[1], "delegated", true},
		{"no subscription", credited[2], "creator", false},
		{"under a tree opt-out", credited[3], "", false},
		{"under an issue opt-out", credited[4], "creator", false},
	}
	for _, tc := range subscriptions {
		var reason string
		var unsubscribed bool
		if err := tx.QueryRow(ctx, `SELECT coalesce(max(reason), ''), coalesce(bool_or(unsubscribed_at IS NOT NULL), false) FROM issue_subscriber WHERE issue_id=$1 AND user_type='member' AND user_id=$2`, tc.issueID, fx.UserID).Scan(&reason, &unsubscribed); err != nil {
			t.Fatal(err)
		}
		if reason != tc.reason || unsubscribed != tc.unsubscribed {
			t.Errorf("%s = %q, unsubscribed %v; want %q, unsubscribed %v", tc.name, reason, unsubscribed, tc.reason, tc.unsubscribed)
		}
	}
	if got := backfillCount(t, tx, `SELECT count(*) FROM issue_subscriber WHERE issue_id = ANY($1) AND user_type='member' AND user_id=$2 AND reason='creator' AND unsubscribed_at IS NULL`, credited, fx.UserID); got != 89 {
		t.Fatalf("active creator subscriptions = %d, want 89", got)
	}
	for n, tc := range unchanged {
		var creatorType, creatorID string
		if err := tx.QueryRow(ctx, `SELECT creator_type, creator_id FROM issue WHERE id=$1`, ids[n]).Scan(&creatorType, &creatorID); err != nil {
			t.Fatal(err)
		}
		if creatorType != tc.creatorType || creatorID != tc.creatorID {
			t.Errorf("%s creator = %s/%s, want %s/%s", tc.name, creatorType, creatorID, tc.creatorType, tc.creatorID)
		}
	}
	if tag, err := tx.Exec(ctx, string(migration)); err != nil || tag.RowsAffected() != 0 {
		t.Fatalf("repeat backfill = %v, %v; want zero changes", tag, err)
	}
	if got := backfillCount(t, tx, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND creator_id=$2 AND creator_type='member' AND origin_id=$3 AND origin_type IN ('agent_create', 'quick_create') AND updated_at=created_at`, fx.WorkspaceID, fx.UserID, origin); got != 91 {
		t.Fatalf("preserved origin and timestamps = %d, want 91", got)
	}
	down, err := os.ReadFile("../../migrations/753_issue_creator_backfill.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(down)); err != nil {
		t.Fatalf("creator down migration: %v", err)
	}
	if got := backfillCount(t, tx, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND creator_type='member' AND creator_id=$2`, fx.WorkspaceID, fx.UserID); got != 91 {
		t.Fatalf("down migration changed member creators: got %d", got)
	}
	todoMigration, err := os.ReadFile("../../migrations/754_todo_backfill.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(todoMigration)); err != nil {
		t.Fatalf("apply subsequent todo backfill: %v", err)
	}
	if got := backfillCount(t, tx, `SELECT count(*) FROM todo WHERE workspace_id=$1 AND member_id=$2 AND kind='review'`, fx.WorkspaceID, fx.UserID); got != 91 {
		t.Fatalf("reviews for corrected member creator = %d, want 91", got)
	}
}
