package db_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func todoFixture(t *testing.T) (*testutil.Fixture, *db.Queries) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is required for database query tests")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	fx := testutil.New(pool, "", "")
	suffix := dbid.NewV7().String()
	fx.UserID = fx.User(t, "Todo member", "todo-"+suffix+"@example.test")
	fx.WorkspaceID = fx.Workspace(t, "Todo workspace", "todo-"+suffix, testutil.Cols{"issue_prefix": "TODO"})
	fx.Member(t, fx.WorkspaceID, fx.UserID, "owner")
	fx.Cleanup(t, "DELETE FROM todo WHERE workspace_id = $1", fx.WorkspaceID)
	return fx, db.New(pool)
}

func todoUUID(t *testing.T, value string) pgtype.UUID {
	t.Helper()
	id, err := util.ParseUUID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func openTodo(t *testing.T, fx *testutil.Fixture, q *db.Queries, kind, source string, issueID pgtype.UUID) db.Todo {
	t.Helper()
	row, err := q.OpenTodo(context.Background(), db.OpenTodoParams{
		ID: dbid.NewV7(), WorkspaceID: todoUUID(t, fx.WorkspaceID), MemberID: todoUUID(t, fx.UserID),
		Kind: kind, SourceKey: source, IssueID: issueID, Title: "Snapshot title", Details: []byte(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func TestTodoOpenAndMarkDone(t *testing.T) {
	fx, q := todoFixture(t)
	ctx := context.Background()
	row := openTodo(t, fx, q, "mention", "comment", pgtype.UUID{})
	_, err := q.OpenTodo(ctx, db.OpenTodoParams{
		ID: dbid.NewV7(), WorkspaceID: row.WorkspaceID, MemberID: row.MemberID,
		Kind: row.Kind, SourceKey: row.SourceKey, Title: "Repeated event", Details: []byte(`{}`),
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("repeated open returned %v, want no inserted row", err)
	}
	if n := fx.Count(t, "SELECT count(*) FROM todo WHERE workspace_id = $1", fx.WorkspaceID); n != 1 {
		t.Fatalf("repeated event left %d rows, want 1", n)
	}
	_, err = q.MarkTodoDone(ctx, db.MarkTodoDoneParams{ID: row.ID, WorkspaceID: row.WorkspaceID, MemberID: dbid.NewV7()})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("another member marked the row done: %v", err)
	}
	_, err = q.GetTodoForMember(ctx, db.GetTodoForMemberParams{ID: row.ID, WorkspaceID: dbid.NewV7(), MemberID: row.MemberID})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("another workspace fetched the row: %v", err)
	}
	done, err := q.MarkTodoDone(ctx, db.MarkTodoDoneParams{ID: row.ID, WorkspaceID: row.WorkspaceID, MemberID: row.MemberID})
	if err != nil || !done.DoneAt.Valid || done.DoneBy.String != "member" {
		t.Fatalf("mark done: row=%+v error=%v", done, err)
	}
	_, err = q.MarkTodoDone(ctx, db.MarkTodoDoneParams{ID: row.ID, WorkspaceID: row.WorkspaceID, MemberID: row.MemberID})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("repeated completion changed the row: %v", err)
	}
	stored, err := q.GetTodoForMember(ctx, db.GetTodoForMemberParams{ID: row.ID, WorkspaceID: row.WorkspaceID, MemberID: row.MemberID})
	if err != nil || stored.DoneAt != done.DoneAt || stored.DoneBy != done.DoneBy {
		t.Fatalf("completed row was not preserved: row=%+v error=%v", stored, err)
	}
	reopened := openTodo(t, fx, q, row.Kind, row.SourceKey, pgtype.UUID{})
	if reopened.ID == row.ID || reopened.DoneAt.Valid {
		t.Fatalf("later event did not create a fresh open row: %+v", reopened)
	}
}

func TestTodoCloseScopes(t *testing.T) {
	fx, q := todoFixture(t)
	ctx := context.Background()
	issueID := todoUUID(t, fx.Issue(t, "Close scope"))
	first := openTodo(t, fx, q, "mention", "shared", issueID)
	other := openTodo(t, fx, q, "assigned", "shared", issueID)
	secondMember := dbid.NewV7()
	fx.Exec(t, `INSERT INTO todo (workspace_id, member_id, kind, source_key, issue_id, title)
		VALUES ($1, $2, 'mention', 'shared', $3, 'Other recipient')`, fx.WorkspaceID, secondMember, issueID)
	otherWorkspace := dbid.NewV7()
	fx.Exec(t, `INSERT INTO todo (workspace_id, member_id, kind, source_key, title)
		VALUES ($1, $2, 'mention', 'shared', 'Other workspace')`, otherWorkspace, first.MemberID)
	fx.Cleanup(t, "DELETE FROM todo WHERE workspace_id = $1", otherWorkspace)
	params := db.CloseTodosParams{WorkspaceID: first.WorkspaceID, MemberID: first.MemberID, Kind: "mention", SourceKey: "shared"}
	closed, err := q.CloseTodos(ctx, params)
	if err != nil || len(closed) != 1 || closed[0].ID != first.ID || closed[0].DoneBy.String != "handled" {
		t.Fatalf("member close: rows=%+v error=%v", closed, err)
	}
	closed, err = q.CloseTodos(ctx, params)
	if err != nil || len(closed) != 0 {
		t.Fatalf("repeated close: rows=%+v error=%v", closed, err)
	}
	params.MemberID = pgtype.UUID{}
	closed, err = q.CloseTodos(ctx, params)
	if err != nil || len(closed) != 1 || closed[0].MemberID != secondMember {
		t.Fatalf("all-member source close: rows=%+v error=%v", closed, err)
	}
	closed, err = q.CloseTodosOnIssue(ctx, db.CloseTodosOnIssueParams{
		WorkspaceID: first.WorkspaceID, IssueID: issueID, Kinds: []string{"assigned", "run_failed"}, MemberID: first.MemberID,
	})
	if err != nil || len(closed) != 1 || closed[0].ID != other.ID {
		t.Fatalf("issue close: rows=%+v error=%v", closed, err)
	}
	if n := fx.Count(t, "SELECT count(*) FROM todo WHERE workspace_id = $1 AND done_at IS NULL", otherWorkspace); n != 1 {
		t.Fatalf("close touched another workspace: %d open rows", n)
	}
}

func TestTodoListAndSummary(t *testing.T) {
	fx, q := todoFixture(t)
	ctx := context.Background()
	workspaceID, memberID := todoUUID(t, fx.WorkspaceID), todoUUID(t, fx.UserID)
	issueID := todoUUID(t, fx.Issue(t, "Live issue title", testutil.Cols{"status": "in_review", "due_date": testutil.Raw("CURRENT_DATE + 1")}))
	review := openTodo(t, fx, q, "review", "review", issueID)
	questionUUID := dbid.NewV7()
	questionID := fx.Insert(t, "question", testutil.Cols{
		"id": questionUUID, "ask_id": questionUUID, "ask_position": 0,
		"workspace_id": fx.WorkspaceID, "asker_agent_id": dbid.NewV7(),
		"asked_in_task_id": dbid.NewV7(), "prompt": "Approve?", "answer_type": "approval", "due_date": testutil.Raw("CURRENT_DATE - 1"),
	})
	question := openTodo(t, fx, q, "question", questionID, issueID)
	fx.Exec(t, "UPDATE todo SET question_id = $1, created_at = now() - interval '7 days' WHERE id = $2", questionID, question.ID)
	mention := openTodo(t, fx, q, "mention", "new-comment", issueID)
	assigned := openTodo(t, fx, q, "assigned", "assignment", issueID)
	fx.Exec(t, "UPDATE todo SET created_at = now() - interval '1 day' WHERE id = $1", assigned.ID)
	openTodo(t, fx, q, "mention", "deleted-issue", dbid.NewV7())

	otherWorkspace := fx.Workspace(t, "Other workspace", "other-"+dbid.NewV7().String())
	fx.Member(t, otherWorkspace, fx.UserID, "member")
	fx.Exec(t, `INSERT INTO todo (workspace_id, member_id, kind, source_key, title)
		VALUES ($1, $2, 'quota_reached', 'period', 'Quota')`, otherWorkspace, fx.UserID)
	fx.Cleanup(t, "DELETE FROM todo WHERE workspace_id = $1", otherWorkspace)
	leftWorkspace := fx.Workspace(t, "Left workspace", "left-"+dbid.NewV7().String())
	fx.Exec(t, `INSERT INTO todo (workspace_id, member_id, kind, source_key, title)
		VALUES ($1, $2, 'quota_reached', 'period', 'Stale membership')`, leftWorkspace, fx.UserID)
	fx.Cleanup(t, "DELETE FROM todo WHERE workspace_id = $1", leftWorkspace)

	rows, err := q.ListOpenTodos(ctx, db.ListOpenTodosParams{WorkspaceID: workspaceID, MemberID: memberID})
	if err != nil || len(rows) != 3 {
		t.Fatalf("list: rows=%+v error=%v", rows, err)
	}
	if rows[0].ID != question.ID || !rows[0].Overdue || rows[1].ID != mention.ID || rows[2].ID != assigned.ID {
		t.Fatalf("want overdue question then newest rows, got %+v", rows)
	}
	if rows[0].Title != "Live issue title" || rows[0].IssueTitle.String != "Live issue title" || rows[0].IssueStatus.String != "in_review" || rows[0].QuestionAnswerType.String != "approval" || rows[0].IssueIdentifier.String != "TODO-1" {
		t.Fatalf("source projection: %+v", rows[0])
	}
	if rows[1].DueDate.Valid || rows[1].Overdue || !rows[2].DueDate.Valid || rows[2].Overdue {
		t.Fatalf("due date belongs only to question/assigned/review: %+v", rows)
	}
	systemRows, err := q.ListOpenTodos(ctx, db.ListOpenTodosParams{WorkspaceID: todoUUID(t, otherWorkspace), MemberID: memberID})
	if err != nil || len(systemRows) != 1 || systemRows[0].IssueIdentifier.Valid || systemRows[0].IssueTitle.Valid || systemRows[0].DueDate.Valid || systemRows[0].Overdue || systemRows[0].Title != "Quota" {
		t.Fatalf("system row must preserve null source fields: rows=%+v error=%v", systemRows, err)
	}
	counts, err := q.CountOpenTodosByWorkspace(ctx, memberID)
	if err != nil || len(counts) != 2 {
		t.Fatalf("summary: rows=%+v error=%v", counts, err)
	}
	for _, count := range counts {
		want := int64(1)
		if count.WorkspaceID == workspaceID {
			want = 3
		}
		if count.Count != want {
			t.Fatalf("summary count=%d, want %d for %s", count.Count, want, count.WorkspaceID)
		}
	}
	_, err = q.MarkTodoDone(ctx, db.MarkTodoDoneParams{ID: question.ID, WorkspaceID: workspaceID, MemberID: memberID})
	if err != nil {
		t.Fatal(err)
	}
	rows, err = q.ListOpenTodos(ctx, db.ListOpenTodosParams{WorkspaceID: workspaceID, MemberID: memberID})
	if err != nil || len(rows) != 3 {
		t.Fatalf("review should return when question closes: rows=%+v error=%v", rows, err)
	}
	foundReview := false
	for _, row := range rows {
		foundReview = foundReview || row.ID == review.ID
	}
	if !foundReview {
		t.Fatal("review stayed hidden after the question closed")
	}
	fx.Exec(t, "UPDATE issue SET title = 'Renamed', status = 'blocked', due_date = CURRENT_DATE - 1 WHERE id = $1", issueID)
	live, err := q.GetTodoForMember(ctx, db.GetTodoForMemberParams{ID: assigned.ID, WorkspaceID: workspaceID, MemberID: memberID})
	if err != nil || live.Title != "Renamed" || live.IssueStatus.String != "blocked" || !live.Overdue {
		t.Fatalf("source edits must be read live: row=%+v error=%v", live, err)
	}
}

func TestTodoVocabularyConstraints(t *testing.T) {
	fx, _ := todoFixture(t)
	for _, test := range []struct{ kind, actor, done string }{
		{"unknown", "member", "handled"},
		{"mention", "unknown", "handled"},
		{"mention", "member", "unknown"},
	} {
		_, err := fx.Pool.Exec(context.Background(), `INSERT INTO todo
			(workspace_id, member_id, kind, source_key, title, actor_type, done_by)
			VALUES ($1, $2, $3, 'invalid', 'Invalid vocabulary', $4, $5)`,
			fx.WorkspaceID, fx.UserID, test.kind, test.actor, test.done)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
			t.Fatalf("invalid vocabulary %+v: want CHECK violation, got %v", test, err)
		}
	}
}
