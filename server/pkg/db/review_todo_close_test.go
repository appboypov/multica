package db_test

import (
	"context"
	"os"
	"testing"
)

// Open creator review to-dos close; the Decider's review to-dos and other
// kinds on the same issue stay open.
func TestCloseCreatorReviewTodos(t *testing.T) {
	fx, q := todoFixture(t)
	issue := fx.Issue(t, "Built")
	issueID := todoUUID(t, issue)
	creator := openTodo(t, fx, q, "review", issue, issueID)
	decider := openTodo(t, fx, q, "review", "decider:bug:"+issue, issueID)
	assigned := openTodo(t, fx, q, "assigned", issue, issueID)
	migration, err := os.ReadFile("../../migrations/808_close_creator_review_todos.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := fx.Pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// The migration is table-wide. Roll it back so concurrent suites' rows stay as they are.
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	if _, err := tx.Exec(context.Background(), string(migration)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	if got := backfillCount(t, tx, `SELECT count(*) FROM todo WHERE id=$1 AND done_at IS NOT NULL AND done_by='handled'`, creator.ID); got != 1 {
		t.Error("creator review to-do stayed open")
	}
	for name, id := range map[string]any{"decider review": decider.ID, "assigned": assigned.ID} {
		if got := backfillCount(t, tx, `SELECT count(*) FROM todo WHERE id=$1 AND done_at IS NULL`, id); got != 1 {
			t.Errorf("%s to-do was closed", name)
		}
	}
}
