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
	for n := range 91 {
		kind := "agent_create"
		if n%2 == 0 {
			kind = "quick_create"
		}
		fx.Issue(t, fmt.Sprintf("Creator backfill %d", n), testutil.Cols{
			"creator_type": "agent", "creator_id": agent,
			"origin_type": kind, "origin_id": origin, "status": "in_review",
		})
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
	migration, err := os.ReadFile("../../migrations/743_issue_creator_backfill.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := fx.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Both backfills are table-wide, so roll them back before fixture cleanup.
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	if _, err := tx.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("apply creator backfill: %v", err)
	}
	if got := backfillCount(t, tx, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND creator_type='member' AND creator_id=$2`, fx.WorkspaceID, fx.UserID); got != 91 {
		t.Fatalf("corrected creators = %d, want 91", got)
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
	down, err := os.ReadFile("../../migrations/743_issue_creator_backfill.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(down)); err != nil {
		t.Fatalf("creator down migration: %v", err)
	}
	if got := backfillCount(t, tx, `SELECT count(*) FROM issue WHERE workspace_id=$1 AND creator_type='member' AND creator_id=$2`, fx.WorkspaceID, fx.UserID); got != 91 {
		t.Fatalf("down migration changed member creators: got %d", got)
	}
	todoMigration, err := os.ReadFile("../../migrations/744_todo_backfill.up.sql")
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
