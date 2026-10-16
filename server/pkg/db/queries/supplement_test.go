package queries_test

import (
	"context"
	"encoding/json"
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
	fx                                             *testutil.Fixture
	q                                              *db.Queries
	workspace, author, agent, issue, task, runtime pgtype.UUID
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
		agent: util.MustParseUUID(agent), issue: util.MustParseUUID(issue), task: util.MustParseUUID(task), runtime: util.MustParseUUID(runtime),
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

func (f *runInputFixture) waitingWake(t *testing.T, question pgtype.UUID) db.QuestionWakePending {
	t.Helper()
	anchor, err := f.q.GetQuestionWakeAnswerAnchor(t.Context(), db.GetQuestionWakeAnswerAnchorParams{
		WorkspaceID: f.workspace, IssueID: f.issue, AgentID: f.agent,
	})
	if err != nil {
		t.Fatal(err)
	}
	row, err := f.q.UpsertQuestionWakePending(t.Context(), db.UpsertQuestionWakePendingParams{
		WorkspaceID: f.workspace, IssueID: f.issue, AgentID: f.agent,
		ActorUserID: f.author, SourceQuestionID: question, AnswerAnchor: anchor,
	})
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, "DELETE FROM question_wake_pending WHERE workspace_id = $1", f.workspace)
	return row
}

func (f *runInputFixture) questionInput(t *testing.T, at time.Time) (pgtype.UUID, pgtype.UUID) {
	t.Helper()
	question := dbid.NewV7()
	f.fx.Insert(t, "question", testutil.Cols{
		"id": question, "workspace_id": f.workspace, "asker_agent_id": f.agent,
		"asked_in_task_id": f.task, "ask_id": question, "ask_position": 0,
		"prompt": "Which toolbar?", "answer_type": "text", "status": "answered",
	})
	answer := util.MustParseUUID(f.fx.Insert(t, "question_answer", testutil.Cols{
		"id": dbid.NewV7(), "workspace_id": f.workspace, "question_id": question,
		"answerer_user_id": f.author, "value": testutil.Raw(`'{"text":"header"}'::jsonb`),
		"created_at": at, "updated_at": at,
	}))
	f.fx.Insert(t, "question_event", testutil.Cols{
		"id": dbid.NewV7(), "workspace_id": f.workspace, "question_id": question,
		"kind": "answered", "actor_type": "member", "actor_id": f.author, "created_at": at,
	})
	return question, answer
}

func (f *runInputFixture) wantQuestionInput(t *testing.T, question, exclude pgtype.UUID, anchor pgtype.Timestamptz, want int64, wantBacks int) {
	t.Helper()
	count, err := f.q.CountUnseenQuestionAnswers(t.Context(), db.CountUnseenQuestionAnswersParams{
		WorkspaceID: f.workspace, IssueID: f.issue, AgentID: f.agent, QuestionIds: []pgtype.UUID{question},
		ExcludeTaskID: exclude, AnswerAnchor: anchor,
	})
	if err != nil || count != want {
		t.Fatalf("unseen count = %d, %v; want %d", count, err, want)
	}
	answers, err := f.q.ListUnseenQuestionAnswers(t.Context(), db.ListUnseenQuestionAnswersParams{
		WorkspaceID: f.workspace, IssueID: f.issue, AgentID: f.agent, QuestionIds: []pgtype.UUID{question},
		ExcludeTaskID: exclude, AnswerAnchor: anchor,
	})
	if err != nil || int64(len(answers)) != want {
		t.Fatalf("claim answers = %d, %v; want %d", len(answers), err, want)
	}
	backs, err := f.q.ListUnseenQuestionAskBacks(t.Context(), db.ListUnseenQuestionAskBacksParams{
		WorkspaceID: f.workspace, IssueID: f.issue, AgentID: f.agent, QuestionIds: []pgtype.UUID{question},
		ExcludeTaskID: exclude, AnswerAnchor: anchor,
	})
	if err != nil || len(backs) != wantBacks {
		t.Fatalf("claim ask-backs = %d, %v; want %d", len(backs), err, wantBacks)
	}
}

func TestQuestionWakeRetainsInputAcrossOrdinaryRetryAndExcludesClaimingLineage(t *testing.T) {
	f := newRunInputFixture(t)
	at := time.Now().UTC().Add(-30 * time.Minute).Truncate(time.Microsecond)
	question, answer := f.questionInput(t, at)
	f.fx.Insert(t, "question_event", testutil.Cols{
		"id": dbid.NewV7(), "workspace_id": f.workspace, "question_id": question,
		"kind": "ask_back", "actor_type": "member", "actor_id": f.author, "created_at": at,
	})
	pending := f.waitingWake(t, question)
	retry := util.MustParseUUID(f.fx.Task(t, util.UUIDToString(f.agent), testutil.Cols{
		"runtime_id": f.runtime, "issue_id": f.issue, "status": "completed", "retry_of_task_id": f.task,
		"created_at": at.Add(time.Minute), "dispatched_at": at.Add(time.Minute), "started_at": at.Add(time.Minute),
	}))
	// Ordinary retries do not receive question input and cannot move the wait.
	f.wantQuestionInput(t, question, pgtype.UUID{}, pgtype.Timestamptz{}, 1, 1)
	if err := f.q.DeleteQuestionWakePending(t.Context(), db.DeleteQuestionWakePendingParams{
		WorkspaceID: f.workspace, IssueID: f.issue, AgentID: f.agent,
	}); err != nil {
		t.Fatal(err)
	}
	// Without a retained wait, the ordinary latest-started cutoff still applies.
	f.wantQuestionInput(t, question, pgtype.UUID{}, pgtype.Timestamptz{}, 0, 0)
	f.wantQuestionInput(t, question, pgtype.UUID{}, pending.AnswerAnchor, 1, 1)
	wake := util.MustParseUUID(f.fx.Task(t, util.UUIDToString(f.agent), testutil.Cols{
		"runtime_id": f.runtime, "issue_id": f.issue, "status": "completed", "trigger_evidence_kind": "question_wake",
		"created_at": at.Add(2 * time.Minute), "dispatched_at": at.Add(2 * time.Minute), "started_at": at.Add(2 * time.Minute),
	}))
	// A previously queued wake did receive the answer and ask-back at claim.
	f.wantQuestionInput(t, question, pgtype.UUID{}, pending.AnswerAnchor, 0, 0)
	// Hydrating that wake must not exclude its own input.
	f.wantQuestionInput(t, question, wake, pending.AnswerAnchor, 1, 1)
	f.fx.Exec(t, "UPDATE question_answer SET updated_at = $2 WHERE id = $1", answer, at.Add(3*time.Minute))
	f.fx.Insert(t, "question_event", testutil.Cols{
		"id": dbid.NewV7(), "workspace_id": f.workspace, "question_id": question,
		"kind": "answer_edited", "actor_type": "member", "actor_id": f.author, "created_at": at.Add(3 * time.Minute),
	})
	// Input after the other wake's claim remains unseen.
	f.wantQuestionInput(t, question, pgtype.UUID{}, pending.AnswerAnchor, 1, 0)
	f.fx.Exec(t, `UPDATE agent_task_queue SET retry_of_task_id = $2,
		trigger_evidence_kind = 'question_wake', dispatched_at = $3, started_at = $3 WHERE id = $1`,
		retry, wake, at.Add(4*time.Minute))
	// Retry hydration excludes its own question-wake ancestors as well.
	f.wantQuestionInput(t, question, retry, pending.AnswerAnchor, 1, 1)
}

func TestQuestionWakeAskBackRequiresHandledReceiptForThisAgent(t *testing.T) {
	f := newRunInputFixture(t)
	question, _ := f.questionInput(t, time.Now().UTC())
	back := util.MustParseUUID(f.fx.Insert(t, "question_event", testutil.Cols{
		"id": dbid.NewV7(), "workspace_id": f.workspace, "question_id": question,
		"kind": "ask_back", "actor_type": "member", "actor_id": f.author,
	}))
	f.waitingWake(t, question)
	f.receipt(t, "answer", back, pgtype.UUID{})
	f.deliver(t, back)
	f.wantQuestionInput(t, question, pgtype.UUID{}, pgtype.Timestamptz{}, 1, 1)
	if _, err := f.q.MarkRunInputSeen(t.Context(), db.MarkRunInputSeenParams{TaskID: f.task, ItemID: back}); err != nil {
		t.Fatal(err)
	}
	f.wantQuestionInput(t, question, pgtype.UUID{}, pgtype.Timestamptz{}, 1, 1)
	if _, err := f.q.MarkRunInputHandled(t.Context(), db.MarkRunInputHandledParams{TaskID: f.task, ItemID: back}); err != nil {
		t.Fatal(err)
	}
	f.wantQuestionInput(t, question, pgtype.UUID{}, pgtype.Timestamptz{}, 1, 0)
	f.fx.Insert(t, "question_event", testutil.Cols{
		"id": dbid.NewV7(), "workspace_id": f.workspace, "question_id": question,
		"kind": "ask_back", "actor_type": "member", "actor_id": f.author,
	})
	f.wantQuestionInput(t, question, pgtype.UUID{}, pgtype.Timestamptz{}, 1, 1)
	other := f.fx.Agent(t, "Other agent", "")
	f.fx.Exec(t, "UPDATE agent_task_queue SET agent_id = $2 WHERE id = $1", f.task, other)
	f.wantQuestionInput(t, question, pgtype.UUID{}, pgtype.Timestamptz{}, 1, 2)
}

func TestQuestionWakePendingCoalescesAttributionAndEarliestAnchor(t *testing.T) {
	f := newRunInputFixture(t)
	first := f.waitingWake(t, dbid.NewV7())
	earlier := pgtype.Timestamptz{InfinityModifier: pgtype.NegativeInfinity, Valid: true}
	row, err := f.q.UpsertQuestionWakePending(t.Context(), db.UpsertQuestionWakePendingParams{
		WorkspaceID: f.workspace, IssueID: f.issue, AgentID: f.agent,
		ActorUserID: dbid.NewV7(), SourceQuestionID: dbid.NewV7(), AnswerAnchor: earlier,
	})
	if err != nil || row.ActorUserID != first.ActorUserID || row.SourceQuestionID != first.SourceQuestionID ||
		row.CreatedAt != first.CreatedAt || row.AnswerAnchor != earlier {
		t.Fatalf("coalesced wait = %+v, %v; first = %+v", row, err, first)
	}
	reloaded, err := f.q.GetQuestionWakePending(t.Context(), db.GetQuestionWakePendingParams{
		WorkspaceID: f.workspace, IssueID: f.issue, AgentID: f.agent,
	})
	if err != nil || reloaded != row {
		t.Fatalf("persisted wait = %+v, %v; want %+v", reloaded, err, row)
	}
	f.fx.Exec(t, `UPDATE agent_task_queue SET context = '{"head_sha":"abc","other":true}' WHERE id = $1`, f.task)
	task, err := f.q.MergeQuestionWakeTaskAnchor(t.Context(), db.MergeQuestionWakeTaskAnchorParams{ID: f.task, AnswerAnchor: earlier})
	if err != nil {
		t.Fatal(err)
	}
	var context map[string]any
	if err := json.Unmarshal(task.Context, &context); err != nil {
		t.Fatal(err)
	}
	if context["head_sha"] != "abc" || context["other"] != true || context["question_wake_anchor"] != "-infinity" {
		t.Fatalf("merged context = %s", task.Context)
	}
	f.fx.Exec(t, "UPDATE agent_task_queue SET status = 'failed' WHERE id = $1", f.task)
	retry, err := f.q.CreateRetryTask(t.Context(), db.CreateRetryTaskParams{ID: f.task})
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Cleanup(t, "DELETE FROM agent_task_queue WHERE id = $1", retry.ID)
	if string(retry.Context) != string(task.Context) {
		t.Fatalf("retry context = %s; want %s", retry.Context, task.Context)
	}
}

func TestQuestionWakeBlockersAndCurrentWaitFailedTails(t *testing.T) {
	f := newRunInputFixture(t)
	pending := f.waitingWake(t, dbid.NewV7())
	for _, tc := range []struct {
		status string
		retry  bool
		want   bool
	}{
		{"queued", false, true}, {"dispatched", false, true}, {"running", false, true},
		{"waiting_local_directory", false, true}, {"deferred", false, false},
		{"deferred", true, true}, {"completed", false, false}, {"failed", false, false},
	} {
		f.fx.Exec(t, "UPDATE agent_task_queue SET status = $2, retry_of_task_id = $3 WHERE id = $1",
			f.task, tc.status, pgtype.UUID{Bytes: dbid.NewV7().Bytes, Valid: tc.retry})
		got, err := f.q.HasActiveQuestionWakeTask(t.Context(), db.HasActiveQuestionWakeTaskParams{
			WorkspaceID: f.workspace, IssueID: f.issue, AgentID: f.agent,
		})
		if err != nil || got != tc.want {
			t.Fatalf("blocker %s retry=%t = %t, %v; want %t", tc.status, tc.retry, got, err, tc.want)
		}
	}
	wantCandidates := func(want int) {
		t.Helper()
		rows, err := f.q.ListQuestionWakeRetryCandidates(t.Context(), db.ListQuestionWakeRetryCandidatesParams{
			WorkspaceID: f.workspace, IssueID: f.issue, AgentID: f.agent,
		})
		if err != nil || len(rows) != want || (want == 1 && rows[0].ID != f.task) {
			t.Fatalf("retry candidates = %+v, %v; want %d", rows, err, want)
		}
	}
	f.fx.Exec(t, "UPDATE agent_task_queue SET completed_at = $2 WHERE id = $1", f.task, pending.CreatedAt.Time.Add(-time.Minute))
	wantCandidates(0)
	f.fx.Exec(t, "UPDATE agent_task_queue SET completed_at = $2 WHERE id = $1", f.task, pending.CreatedAt.Time.Add(time.Minute))
	wantCandidates(1)
	successor := util.MustParseUUID(f.fx.Task(t, util.UUIDToString(f.agent), testutil.Cols{
		"runtime_id": f.runtime, "issue_id": f.issue, "status": "cancelled", "retry_of_task_id": f.task,
	}))
	wantCandidates(0)
	child, err := f.q.GetRetryTaskForParent(t.Context(), f.task)
	if err != nil || child.ID != successor {
		t.Fatalf("materialized cancelled retry = %+v, %v", child, err)
	}
	// A later successor in any state also supersedes the tail without retry lineage.
	f.fx.Exec(t, "UPDATE agent_task_queue SET retry_of_task_id = NULL WHERE id = $1", successor)
	wantCandidates(0)
}

func TestQuestionWakePendingRecoveryPagesPastUnavailableTargets(t *testing.T) {
	f := newRunInputFixture(t)
	f.waitingWake(t, dbid.NewV7())
	// Missing target rows remain visible to recovery for retirement. Equal times
	// require pagination to use the whole identity tuple.
	issue1 := util.MustParseUUID("00000000-0000-0000-0000-000000000001")
	issue2 := util.MustParseUUID("00000000-0000-0000-0000-000000000002")
	at := pgtype.Timestamptz{Time: time.Now().UTC().Add(24 * time.Hour).Truncate(time.Microsecond), Valid: true}
	for _, issue := range []pgtype.UUID{issue1, issue2} {
		if _, err := f.q.UpsertQuestionWakePending(t.Context(), db.UpsertQuestionWakePendingParams{
			WorkspaceID: f.workspace, IssueID: issue, AgentID: f.agent, ActorUserID: f.author,
			SourceQuestionID: dbid.NewV7(), AnswerAnchor: pgtype.Timestamptz{InfinityModifier: pgtype.NegativeInfinity, Valid: true},
		}); err != nil {
			t.Fatal(err)
		}
		f.fx.Exec(t, "UPDATE question_wake_pending SET created_at = $3 WHERE workspace_id = $1 AND issue_id = $2",
			f.workspace, issue, at)
	}
	page, err := f.q.ListQuestionWakePending(t.Context(), db.ListQuestionWakePendingParams{
		AfterCreatedAt: at, AfterWorkspaceID: f.workspace, AfterIssueID: pgtype.UUID{Valid: true},
		AfterAgentID: pgtype.UUID{Valid: true}, MaxRows: 1,
	})
	if err != nil || len(page) != 1 || page[0].IssueID != issue1 || page[0].WorkspaceID != f.workspace {
		t.Fatalf("first recovery page = %+v, %v", page, err)
	}
	cursor := page[0]
	page, err = f.q.ListQuestionWakePending(t.Context(), db.ListQuestionWakePendingParams{
		AfterCreatedAt: cursor.CreatedAt, AfterWorkspaceID: cursor.WorkspaceID,
		AfterIssueID: cursor.IssueID, AfterAgentID: cursor.AgentID, MaxRows: 1,
	})
	if err != nil || len(page) != 1 || page[0].IssueID != issue2 || page[0].WorkspaceID != f.workspace {
		t.Fatalf("second recovery page = %+v, %v", page, err)
	}
}
