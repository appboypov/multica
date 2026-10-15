package queries_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

type runInputFixture struct {
	fx                                    *testutil.Fixture
	q                                     *db.Queries
	workspace, author, agent, issue, task pgtype.UUID
}

func newRunInputFixture(t *testing.T) *runInputFixture {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is required for receipt query tests")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("configured test database is unreachable: %v", err)
	}
	bootstrap := testutil.New(pool, "", "")
	suffix := fmt.Sprint(time.Now().UnixNano())
	author := bootstrap.User(t, "Brian", "run-input-"+suffix+"@example.test")
	workspace := bootstrap.Workspace(t, "Run inputs", "run-input-"+suffix)
	fx := testutil.New(pool, workspace, author)
	runtime := fx.Runtime(t, "omp", testutil.Cols{"provider": "omp"})
	agent := fx.Agent(t, "Thomas", runtime)
	issue := fx.Issue(t, "Receipt query coverage")
	task := fx.Task(t, agent, testutil.Cols{
		"runtime_id": runtime, "issue_id": issue, "status": "running",
		"created_at":    testutil.Raw("clock_timestamp() - interval '1 hour'"),
		"dispatched_at": testutil.Raw("clock_timestamp() - interval '1 hour'"),
		"started_at":    testutil.Raw("clock_timestamp() - interval '1 hour'"),
	})
	fx.InsertNoID(t, "task_supplement_capability", testutil.Cols{
		"task_id": task, "workspace_id": workspace, "issue_id": issue, "capability": "task-supplement-v1",
	}, "task_id = $1", task)
	fx.Cleanup(t, "DELETE FROM task_supplement WHERE task_id = $1", task)
	return &runInputFixture{
		fx: fx, q: db.New(pool), workspace: util.MustParseUUID(workspace), author: util.MustParseUUID(author),
		agent: util.MustParseUUID(agent), issue: util.MustParseUUID(issue), task: util.MustParseUUID(task),
	}
}

func (f *runInputFixture) receipt(t *testing.T, kind string, item, comment pgtype.UUID) db.CreateRunInputSupplementRow {
	t.Helper()
	row, err := f.q.CreateRunInputSupplement(t.Context(), db.CreateRunInputSupplementParams{
		WorkspaceID: f.workspace, IssueID: f.issue, TaskID: f.task, AuthorID: f.author,
		ItemKind: kind, ItemID: item, CommentID: comment, Content: pgtype.Text{String: "received text", Valid: true},
	})
	if err != nil {
		t.Fatalf("create %s receipt: %v", kind, err)
	}
	return row
}

func (f *runInputFixture) deliver(t *testing.T, item pgtype.UUID) {
	t.Helper()
	claim, err := f.q.ClaimNextTaskSupplement(t.Context(), f.task)
	if err != nil || claim.ItemID != item {
		t.Fatalf("claim = %+v, %v; want item %v", claim, err, item)
	}
	if _, err := f.q.AckTaskSupplementDelivered(t.Context(), db.AckTaskSupplementDeliveredParams{TaskID: f.task, ItemID: item}); err != nil {
		t.Fatal(err)
	}
}

func TestRunInputCommentReconciliationUsesNewestHandledReceipt(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		seen, handled, edit, older bool
		want                       int
	}{
		{name: "delivered without handled mark", want: 1},
		{name: "seen only", seen: true, want: 1},
		{name: "handled", handled: true},
		{name: "newer unhandled edit", handled: true, edit: true, want: 1},
		{name: "edit of comment older than run", edit: true, older: true, want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRunInputFixture(t)
			cols := testutil.Cols{}
			if tc.older {
				cols["created_at"] = testutil.Raw("clock_timestamp() - interval '2 hours'")
			}
			comment := util.MustParseUUID(f.fx.Comment(t, util.UUIDToString(f.issue), "initial text", cols))
			if !tc.older {
				f.receipt(t, "comment", comment, comment)
				f.deliver(t, comment)
				if tc.seen {
					if _, err := f.q.MarkRunInputSeen(t.Context(), db.MarkRunInputSeenParams{TaskID: f.task, ItemID: comment}); err != nil {
						t.Fatal(err)
					}
				}
				if tc.handled {
					if _, err := f.q.MarkRunInputHandled(t.Context(), db.MarkRunInputHandledParams{TaskID: f.task, ItemID: comment}); err != nil {
						t.Fatal(err)
					}
				}
			}
			if tc.edit {
				f.fx.Exec(t, "UPDATE comment SET content = 'edited text' WHERE id = $1", comment)
				item := dbid.NewV7()
				f.receipt(t, "comment_edit", item, comment)
				f.deliver(t, item)
			}
			task, err := f.q.GetAgentTask(t.Context(), f.task)
			if err != nil {
				t.Fatal(err)
			}
			comments, err := f.q.ListReconcilableCommentsForIssueSince(t.Context(), db.ListReconcilableCommentsForIssueSinceParams{
				TaskID: f.task, IssueID: f.issue, AgentID: f.agent, Since: task.CreatedAt,
			})
			if err != nil || len(comments) != tc.want {
				t.Fatalf("reconcilable comments = %d, %v; want %d", len(comments), err, tc.want)
			}
			if tc.want == 1 && (comments[0].ID != comment || (tc.edit && comments[0].Content != "edited text")) {
				t.Fatalf("replayed wrong comment: %+v", comments)
			}
		})
	}
}

func TestRunInputMarksKeepFirstTimestampAndRequireDelivery(t *testing.T) {
	f := newRunInputFixture(t)
	item := dbid.NewV7()
	f.receipt(t, "issue_change", item, pgtype.UUID{})
	seen := db.MarkRunInputSeenParams{TaskID: f.task, ItemID: item}
	handled := db.MarkRunInputHandledParams{TaskID: f.task, ItemID: item}
	if _, err := f.q.MarkRunInputSeen(t.Context(), seen); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("pending seen mark = %v; want no rows", err)
	}
	if _, err := f.q.MarkRunInputHandled(t.Context(), handled); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("pending handled mark = %v; want no rows", err)
	}
	pending, err := f.q.GetTaskSupplementForRun(t.Context(), db.GetTaskSupplementForRunParams{WorkspaceID: f.workspace, TaskID: f.task, ItemID: item})
	if err != nil || pending.SeenAt.Valid || pending.HandledAt.Valid || pending.Status != "pending" {
		t.Fatalf("pending receipt changed: %+v, %v", pending, err)
	}
	f.deliver(t, item)
	firstSeen, err := f.q.MarkRunInputSeen(t.Context(), seen)
	if err != nil || !firstSeen.SeenAt.Valid {
		t.Fatalf("seen: %+v, %v", firstSeen, err)
	}
	secondSeen, err := f.q.MarkRunInputSeen(t.Context(), seen)
	if err != nil || secondSeen.SeenAt != firstSeen.SeenAt {
		t.Fatalf("repeated seen changed timestamp: %+v, %v", secondSeen, err)
	}
	firstHandled, err := f.q.MarkRunInputHandled(t.Context(), handled)
	if err != nil || !firstHandled.HandledAt.Valid {
		t.Fatalf("handled: %+v, %v", firstHandled, err)
	}
	secondHandled, err := f.q.MarkRunInputHandled(t.Context(), handled)
	if err != nil || secondHandled.HandledAt != firstHandled.HandledAt || secondHandled.SeenAt != firstSeen.SeenAt {
		t.Fatalf("repeated handled changed timestamps: %+v, %v", secondHandled, err)
	}
}

func TestRunInputHandledAnswerIsUnseenAgainAfterEdit(t *testing.T) {
	f := newRunInputFixture(t)
	questionID := dbid.NewV7()
	f.fx.Insert(t, "question", testutil.Cols{
		"id": questionID, "workspace_id": f.workspace, "asker_agent_id": f.agent,
		"asked_in_task_id": f.task, "ask_id": questionID, "ask_position": 0,
		"prompt": "Which toolbar?", "answer_type": "text", "status": "answered",
	})
	answerID := dbid.NewV7()
	f.fx.Insert(t, "question_answer", testutil.Cols{
		"id": answerID, "workspace_id": f.workspace, "question_id": questionID,
		"answerer_user_id": f.author, "value": testutil.Raw(`'{"text":"header"}'::jsonb`),
	})
	answered := util.MustParseUUID(f.fx.Insert(t, "question_event", testutil.Cols{
		"id": dbid.NewV7(), "workspace_id": f.workspace, "question_id": questionID,
		"kind": "answered", "actor_type": "member", "actor_id": f.author,
	}))
	wantUnseen := func(want int64) {
		t.Helper()
		count, err := f.q.CountUnseenQuestionAnswers(t.Context(), db.CountUnseenQuestionAnswersParams{
			WorkspaceID: f.workspace, IssueID: f.issue, AgentID: f.agent, QuestionIds: []pgtype.UUID{questionID},
		})
		if err != nil || count != want {
			t.Fatalf("unseen count = %d, %v; want %d", count, err, want)
		}
		answers, err := f.q.ListUnseenQuestionAnswers(t.Context(), db.ListUnseenQuestionAnswersParams{
			WorkspaceID: f.workspace, IssueID: f.issue, AgentID: f.agent, QuestionIds: []pgtype.UUID{questionID},
		})
		if err != nil || int64(len(answers)) != want {
			t.Fatalf("claim answers = %d, %v; want %d", len(answers), err, want)
		}
	}
	wantUnseen(1)
	f.receipt(t, "answer", answered, pgtype.UUID{})
	f.deliver(t, answered)
	wantUnseen(1)
	if _, err := f.q.MarkRunInputHandled(t.Context(), db.MarkRunInputHandledParams{TaskID: f.task, ItemID: answered}); err != nil {
		t.Fatal(err)
	}
	wantUnseen(0)
	// A second member's event must not replace the first member's latest event.
	other := f.fx.User(t, "Anouk", "run-input-other-"+util.UUIDToString(questionID)+"@example.test")
	f.fx.Insert(t, "question_answer", testutil.Cols{
		"id": dbid.NewV7(), "workspace_id": f.workspace, "question_id": questionID,
		"answerer_user_id": other, "value": testutil.Raw(`'{"text":"footer"}'::jsonb`),
	})
	f.fx.Insert(t, "question_event", testutil.Cols{
		"id": dbid.NewV7(), "workspace_id": f.workspace, "question_id": questionID,
		"kind": "answered", "actor_type": "member", "actor_id": other,
	})
	wantUnseen(1)
	f.fx.Exec(t, `UPDATE question_answer SET value = '{"text":"sidebar"}', updated_at = clock_timestamp() WHERE id = $1`, answerID)
	f.fx.Insert(t, "question_event", testutil.Cols{
		"id": dbid.NewV7(), "workspace_id": f.workspace, "question_id": questionID,
		"kind": "answer_edited", "actor_type": "member", "actor_id": f.author,
	})
	wantUnseen(2)
}

func TestRunInputClaimAndIdempotencyForEveryKind(t *testing.T) {
	for _, kind := range []string{"comment", "comment_edit", "answer", "issue_change"} {
		t.Run(kind, func(t *testing.T) {
			f := newRunInputFixture(t)
			item := dbid.NewV7()
			var comment pgtype.UUID
			if kind == "comment" || kind == "comment_edit" {
				comment = util.MustParseUUID(f.fx.Comment(t, util.UUIDToString(f.issue), "current comment text"))
				if kind == "comment" {
					item = comment
				}
			}
			row := f.receipt(t, kind, item, comment)
			if row.ClientRequestID != item || row.CommentID.Valid != comment.Valid {
				t.Fatalf("incorrect identity: %+v", row)
			}
			_, err := f.q.CreateRunInputSupplement(t.Context(), db.CreateRunInputSupplementParams{
				WorkspaceID: f.workspace, IssueID: f.issue, TaskID: f.task, AuthorID: f.author,
				ItemKind: kind, ItemID: item, CommentID: comment,
			})
			if !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("duplicate receipt = %v; want no insert", err)
			}
			claim, err := f.q.ClaimNextTaskSupplement(t.Context(), f.task)
			if err != nil || claim.ItemID != item || claim.ItemKind != kind || claim.Content != "received text" || claim.AuthorName != "Brian" {
				t.Fatalf("claim lost item content or identity: %+v, %v", claim, err)
			}
			if _, err := f.q.ClaimNextTaskSupplement(t.Context(), f.task); !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("duplicate claim = %v", err)
			}
		})
	}
}
