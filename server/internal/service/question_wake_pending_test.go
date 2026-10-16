package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func acceptQuestionWait(t *testing.T, f *questionFixture, row QuestionDTO, actor QuestionActor) db.QuestionWakePending {
	t.Helper()
	wake, err := f.svc.Wake(context.Background(), f.workspace, util.MustParseUUID(row.ID), actor, pgtype.UUID{})
	if err != nil || wake.TaskID != "" {
		t.Fatalf("waiting wake = %+v, %v", wake, err)
	}
	pending, err := f.svc.Queries.GetQuestionWakePending(context.Background(), db.GetQuestionWakePendingParams{WorkspaceID: f.workspace, IssueID: f.issue, AgentID: f.agent})
	if err != nil {
		t.Fatal(err)
	}
	return pending
}

func drainQuestionWait(t *testing.T, f *questionFixture) {
	t.Helper()
	if err := f.svc.DrainQuestionWake(context.Background(), f.workspace, f.agent, f.issue); err != nil {
		t.Fatal(err)
	}
}

func TestQuestionWaitingWakeCoalescesAndRetainsFirstAttribution(t *testing.T) {
	f := newQuestionFixture(t)
	first, second := f.ask(t), f.ask(t)
	f.answer(t, first, f.answerer, "per line")
	f.answer(t, second, f.answerer, "per total")
	pending := acceptQuestionWait(t, f, first, f.other)
	start := make(chan struct{})
	errorsCh := make(chan error, 8)
	for range 8 {
		go func() {
			<-start
			result, err := f.svc.Wake(context.Background(), f.workspace, util.MustParseUUID(second.ID), f.answerer, pgtype.UUID{})
			if err == nil && (result.TaskID != "" || result.AnswerCount != 2) {
				err = errors.New("coalesced wake must wait with both answers")
			}
			errorsCh <- err
		}()
	}
	close(start)
	for range 8 {
		if err := <-errorsCh; err != nil {
			t.Fatal(err)
		}
	}
	f.wantNoNewTask(t)
	f.finishAskingRun(t)
	for range 3 {
		drainQuestionWait(t, f)
	}
	var taskID pgtype.UUID
	f.fx.QueryRow(t, `SELECT id FROM agent_task_queue WHERE issue_id = $1 AND trigger_evidence_kind = 'question_wake'`, f.issue).Scan(&taskID)
	task, err := f.svc.Queries.GetAgentTask(context.Background(), taskID)
	if err != nil || task.OriginatorUserID != f.anouk || task.TriggerEvidenceRefID != util.MustParseUUID(first.ID) || task.TriggerSummary.String != "Woken by Anouk with 2 answers" {
		t.Fatalf("drained task = %+v, %v", task, err)
	}
	anchor, err := QuestionWakeAnswerAnchor(task)
	if err != nil || !anchor.Valid || anchor.InfinityModifier != pending.AnswerAnchor.InfinityModifier || !anchor.Time.Equal(pending.AnswerAnchor.Time) {
		t.Fatalf("retained anchor = %+v, %v; want %+v", anchor, err, pending.AnswerAnchor)
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM question_wake_pending WHERE issue_id = $1`, f.issue); n != 0 {
		t.Fatalf("consumed wait rows = %d", n)
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM activity_log WHERE issue_id = $1 AND action = 'question_wake' AND actor_id = $2`, f.issue, f.anouk); n != 1 {
		t.Fatalf("first-waker activities = %d", n)
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM question_event WHERE workspace_id = $1 AND kind = 'woken'`, f.workspace); n != 1 {
		t.Fatalf("woken events = %d", n)
	}
}

func TestQuestionAnswerAndAskBackWaitThroughSameDrain(t *testing.T) {
	for _, entry := range []string{"answer", "ask_back", "without_wake"} {
		t.Run(entry, func(t *testing.T) {
			f := newQuestionFixture(t)
			row := f.ask(t)
			id := util.MustParseUUID(row.ID)
			var err error
			if entry == "ask_back" {
				_, err = f.svc.AskBack(context.Background(), f.workspace, id, f.answerer, "Which customer?")
			} else {
				_, err = f.svc.Answer(context.Background(), f.workspace, id, f.answerer, AnswerQuestionInput{Value: json.RawMessage(`{"option":"per line"}`), Wake: entry == "answer"})
			}
			if err != nil {
				t.Fatal(err)
			}
			f.wantNoNewTask(t)
			want := 1
			if entry == "without_wake" {
				want = 0
			}
			if n := f.fx.Count(t, `SELECT count(*) FROM question_wake_pending WHERE issue_id = $1`, f.issue); n != want {
				t.Fatalf("durable intent count = %d, want %d", n, want)
			}
			f.finishAskingRun(t)
			drainQuestionWait(t, f)
			if n := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND trigger_evidence_kind = 'question_wake'`, f.issue); n != want {
				t.Fatalf("follow-up tasks = %d, want %d", n, want)
			}
		})
	}
}

func TestQuestionWaitingWakeSubtractsHandledAnswerAndAskBack(t *testing.T) {
	for _, kind := range []string{"answer", "ask_back"} {
		t.Run(kind, func(t *testing.T) {
			f := newQuestionFixture(t)
			f.fx.Exec(t, `UPDATE agent_runtime SET provider = 'omp' WHERE id = $1`, f.runtime)
			enableRunInputTestTask(t, f.fx, f.task, f.issue)
			f.svc.RunInputs = &RunInputService{Queries: f.svc.Queries}
			row := f.ask(t)
			if kind == "answer" {
				f.answer(t, row, f.answerer, "per line")
				acceptQuestionWait(t, f, row, f.answerer)
			} else if _, err := f.svc.AskBack(context.Background(), f.workspace, util.MustParseUUID(row.ID), f.answerer, "Which customer?"); err != nil {
				t.Fatal(err)
			}
			if n := f.fx.Count(t, `SELECT count(*) FROM task_supplement WHERE task_id = $1 AND item_kind = 'answer'`, f.task); n != 1 {
				t.Fatalf("live answer receipts = %d", n)
			}
			f.fx.Exec(t, `UPDATE task_supplement SET handled_at = clock_timestamp() WHERE task_id = $1 AND item_kind = 'answer'`, f.task)
			f.finishAskingRun(t)
			drainQuestionWait(t, f)
			f.wantNoNewTask(t)
			if n := f.fx.Count(t, `SELECT count(*) FROM question_wake_pending WHERE issue_id = $1`, f.issue); n != 0 {
				t.Fatalf("handled wait not retired: %d", n)
			}
		})
	}
}

func TestQuestionWaitingWakeRetainsUnavailableAndRetiresDeletedTargets(t *testing.T) {
	for _, state := range []string{"archived", "no_runtime", "deleted_source", "deleted_issue"} {
		t.Run(state, func(t *testing.T) {
			f := newQuestionFixture(t)
			row := f.ask(t)
			f.answer(t, row, f.answerer, "per line")
			acceptQuestionWait(t, f, row, f.answerer)
			f.finishAskingRun(t)
			switch state {
			case "archived":
				f.fx.Exec(t, `UPDATE agent SET archived_at = now() WHERE id = $1`, f.agent)
			case "no_runtime":
				f.fx.Exec(t, `UPDATE agent SET runtime_id = NULL WHERE id = $1`, f.agent)
			case "deleted_source":
				f.fx.Exec(t, `DELETE FROM question WHERE id = $1`, row.ID)
			case "deleted_issue":
				f.fx.Exec(t, `DELETE FROM issue WHERE id = $1`, f.issue)
			}
			drainQuestionWait(t, f)
			if n := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id = $1 AND id <> $2`, f.agent, f.task); n != 0 {
				t.Fatalf("unavailable target created %d tasks", n)
			}
			want := 0
			if state == "archived" || state == "no_runtime" {
				want = 1
			}
			if n := f.fx.Count(t, `SELECT count(*) FROM question_wake_pending WHERE issue_id = $1`, f.issue); n != want {
				t.Fatalf("pending count = %d, want %d", n, want)
			}
		})
	}
}

func TestQuestionWaitingWakeRollbackRetainsIntentAndPublishesNothing(t *testing.T) {
	for _, write := range []string{"MergeQuestionWakeTaskAnchor", "CreateActivity", "CreateQuestionEvent"} {
		t.Run(write, func(t *testing.T) {
			f := newQuestionFixture(t)
			row := f.ask(t)
			f.answer(t, row, f.answerer, "per line")
			acceptQuestionWait(t, f, row, f.other)
			f.finishAskingRun(t)
			starter := f.svc.TxStarter
			f.svc.TxStarter = questionFailingTxStarter{TxStarter: starter, queryName: write}
			published := 0
			f.svc.Bus.Subscribe(protocol.EventQuestionUpdated, func(events.Event) { published++ })
			f.svc.Bus.Subscribe(protocol.EventTaskQueued, func(events.Event) { published++ })
			recorder := &questionWakeRecorder{}
			f.svc.Tasks.Wakeup = recorder
			if err := f.svc.DrainQuestionWake(context.Background(), f.workspace, f.agent, f.issue); err == nil {
				t.Fatal("injected drain write must fail")
			}
			f.wantNoNewTask(t)
			if published != 0 || recorder.calls != 0 {
				t.Fatalf("rollback published=%d notified=%d", published, recorder.calls)
			}
			if n := f.fx.Count(t, `SELECT count(*) FROM question_wake_pending WHERE issue_id = $1 AND actor_user_id = $2`, f.issue, f.anouk); n != 1 {
				t.Fatalf("rollback lost first-waker wait: %d", n)
			}
			f.svc.TxStarter = starter
			drainQuestionWait(t, f)
			if recorder.calls != 1 {
				t.Fatalf("successful retry notifications = %d", recorder.calls)
			}
		})
	}
}

func TestQuestionWaitingWakeRecoveryUsesPersistedWaitAndFreshAnswer(t *testing.T) {
	f := newQuestionFixture(t)
	row := f.ask(t)
	f.answer(t, row, f.answerer, "per line")
	acceptQuestionWait(t, f, row, f.other)
	f.answer(t, row, f.answerer, "per total")
	f.finishAskingRun(t)
	// A new service has no in-memory knowledge and terminal notification was missed.
	restarted := &QuestionService{Queries: f.svc.Queries, TxStarter: f.svc.TxStarter, Tasks: f.svc.Tasks, Bus: f.svc.Bus}
	for range 2 {
		if err := restarted.ReconcilePendingQuestionWakes(context.Background(), 1); err != nil {
			t.Fatal(err)
		}
	}
	var taskID pgtype.UUID
	f.fx.QueryRow(t, `SELECT id FROM agent_task_queue WHERE issue_id = $1 AND trigger_evidence_kind = 'question_wake'`, f.issue).Scan(&taskID)
	task, err := f.svc.Queries.GetAgentTask(context.Background(), taskID)
	if err != nil {
		t.Fatal(err)
	}
	anchor, err := QuestionWakeAnswerAnchor(task)
	if err != nil {
		t.Fatal(err)
	}
	answers, err := f.svc.Queries.ListUnseenQuestionAnswers(context.Background(), db.ListUnseenQuestionAnswersParams{WorkspaceID: f.workspace, IssueID: f.issue, AgentID: f.agent, QuestionIds: []pgtype.UUID{util.MustParseUUID(row.ID)}, ExcludeTaskID: taskID, AnswerAnchor: anchor})
	if err != nil || len(answers) != 1 || !strings.Contains(string(answers[0].Value), "per total") {
		t.Fatalf("fresh drain input = %+v, %v", answers, err)
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, f.issue); n != 2 {
		t.Fatalf("repeated restart drain task count = %d", n)
	}
}

func TestQuestionWaitingWakeSettlesRetryGapOnceAndRetainsAnswer(t *testing.T) {
	f := newQuestionFixture(t)
	row := f.ask(t)
	f.answer(t, row, f.answerer, "per line")
	pending := acceptQuestionWait(t, f, row, f.answerer)
	// An orphan sweeper committed failure but has not made its retry decision.
	f.fx.Exec(t, `UPDATE agent_task_queue SET status = 'failed', failure_reason = 'agent_error.provider_network', attempt = 2, max_attempts = 3, completed_at = clock_timestamp() WHERE id = $1`, f.task)
	parent, err := f.svc.Queries.GetAgentTask(context.Background(), f.task)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-start
		results <- f.svc.DrainQuestionWake(context.Background(), f.workspace, f.agent, f.issue)
	}()
	go func() {
		<-start
		_, err := f.svc.Tasks.MaybeRetryFailedTask(context.Background(), parent)
		results <- err
	}()
	close(start)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE retry_of_task_id = $1 AND status = 'deferred'`, f.task); n != 1 {
		t.Fatalf("deferred retry children = %d, want exactly one", n)
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND trigger_evidence_kind = 'question_wake'`, f.issue); n != 0 {
		t.Fatalf("waiting wake jumped ahead of retry: %d", n)
	}
	var retryID pgtype.UUID
	f.fx.QueryRow(t, `SELECT id FROM agent_task_queue WHERE retry_of_task_id = $1`, f.task).Scan(&retryID)
	f.fx.Exec(t, `UPDATE agent_task_queue SET status = 'running', started_at = clock_timestamp(), dispatched_at = clock_timestamp() WHERE id = $1`, retryID)
	drainQuestionWait(t, f)
	f.fx.Exec(t, `UPDATE agent_task_queue SET status = 'completed', completed_at = clock_timestamp() WHERE id = $1`, retryID)
	drainQuestionWait(t, f)
	var wakeID pgtype.UUID
	f.fx.QueryRow(t, `SELECT id FROM agent_task_queue WHERE issue_id = $1 AND trigger_evidence_kind = 'question_wake'`, f.issue).Scan(&wakeID)
	wake, err := f.svc.Queries.GetAgentTask(context.Background(), wakeID)
	if err != nil {
		t.Fatal(err)
	}
	anchor, err := QuestionWakeAnswerAnchor(wake)
	if err != nil || !anchor.Valid || anchor.InfinityModifier != pending.AnswerAnchor.InfinityModifier || !anchor.Time.Equal(pending.AnswerAnchor.Time) {
		t.Fatalf("retry moved wait anchor: %+v, %v", anchor, err)
	}
	answers, err := f.svc.Queries.ListUnseenQuestionAnswers(context.Background(), db.ListUnseenQuestionAnswersParams{WorkspaceID: f.workspace, IssueID: f.issue, AgentID: f.agent, QuestionIds: []pgtype.UUID{util.MustParseUUID(row.ID)}, ExcludeTaskID: wake.ID, AnswerAnchor: anchor})
	if err != nil || len(answers) != 1 {
		t.Fatalf("ordinary retry consumed accepted answer: %+v, %v", answers, err)
	}
}

func TestQuestionWaitingWakeDoesNotReplayHistoricalOrSupersededFailure(t *testing.T) {
	for _, kind := range []string{"historical", "superseded"} {
		t.Run(kind, func(t *testing.T) {
			f := newQuestionFixture(t)
			row := f.ask(t)
			f.answer(t, row, f.answerer, "per line")
			if kind == "historical" {
				f.fx.Exec(t, `UPDATE agent_task_queue SET status = 'failed', failure_reason = 'runtime_offline', completed_at = clock_timestamp() - interval '1 minute' WHERE id = $1`, f.task)
				f.fx.Task(t, util.UUIDToString(f.agent), testutil.Cols{"runtime_id": f.runtime, "issue_id": f.issue, "status": "running"})
			}
			acceptQuestionWait(t, f, row, f.answerer)
			if kind == "superseded" {
				f.fx.Exec(t, `UPDATE agent_task_queue SET status = 'failed', failure_reason = 'runtime_offline', completed_at = clock_timestamp() WHERE id = $1`, f.task)
				f.fx.Task(t, util.UUIDToString(f.agent), testutil.Cols{"runtime_id": f.runtime, "issue_id": f.issue, "status": "completed"})
			} else {
				f.fx.Exec(t, `UPDATE agent_task_queue SET status = 'completed', completed_at = clock_timestamp() WHERE issue_id = $1 AND id <> $2`, f.issue, f.task)
			}
			drainQuestionWait(t, f)
			if n := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE retry_of_task_id = $1`, f.task); n != 0 {
				t.Fatalf("replayed %s failure: %d children", kind, n)
			}
			if n := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND trigger_evidence_kind = 'question_wake'`, f.issue); n != 1 {
				t.Fatalf("idle wait was not drained: %d", n)
			}
		})
	}
}

func TestQuestionWakeAnswerAnchor(t *testing.T) {
	for _, tc := range []struct {
		name, context  string
		valid, wantErr bool
		infinity       pgtype.InfinityModifier
	}{
		{name: "ordinary"},
		{name: "other context", context: `{"head_sha":"abc"}`},
		{name: "timestamp", context: `{"head_sha":"abc","question_wake_anchor":"2026-10-16T12:34:56.123456+00:00"}`, valid: true},
		{name: "unbounded", context: `{"question_wake_anchor":"-infinity"}`, valid: true, infinity: pgtype.NegativeInfinity},
		{name: "invalid timestamp", context: `{"question_wake_anchor":"bad"}`, wantErr: true},
		{name: "invalid type", context: `{"question_wake_anchor":42}`, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			anchor, err := QuestionWakeAnswerAnchor(db.AgentTaskQueue{Context: []byte(tc.context)})
			if (err != nil) != tc.wantErr || anchor.Valid != tc.valid || anchor.InfinityModifier != tc.infinity {
				t.Fatalf("anchor = %+v, %v", anchor, err)
			}
			if tc.name == "timestamp" && anchor.Time.Format(time.RFC3339Nano) != "2026-10-16T12:34:56.123456Z" {
				t.Fatalf("parsed timestamp = %v", anchor.Time)
			}
		})
	}
}

func TestQuestionWaitingWakeAcceptanceFailureDoesNotCreateIntent(t *testing.T) {
	f := newQuestionFixture(t)
	row := f.ask(t)
	f.answer(t, row, f.answerer, "per line")
	f.svc.TxStarter = questionFailingTxStarter{TxStarter: f.svc.TxStarter, queryName: "UpsertQuestionWakePending"}
	_, err := f.svc.Wake(context.Background(), f.workspace, util.MustParseUUID(row.ID), f.answerer, pgtype.UUID{})
	if err == nil {
		t.Fatal("failed acceptance must not succeed")
	}
	_, err = f.svc.Queries.GetQuestionWakePending(context.Background(), db.GetQuestionWakePendingParams{WorkspaceID: f.workspace, IssueID: f.issue, AgentID: f.agent})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("failed acceptance intent = %v", err)
	}
	f.wantNoNewTask(t)
}

type questionEnqueueRaceStarter struct {
	TxStarter
	beforeInsert func()
}

type questionEnqueueRaceTx struct {
	pgx.Tx
	beforeInsert func()
}

func (s questionEnqueueRaceStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &questionEnqueueRaceTx{Tx: tx, beforeInsert: s.beforeInsert}, nil
}

func (tx *questionEnqueueRaceTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.HasPrefix(sql, "-- name: CreateAgentTask ") && tx.beforeInsert != nil {
		callback := tx.beforeInsert
		tx.beforeInsert = nil
		callback()
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}

func TestQuestionIdleWakeRetainsAcceptanceWhenRetryPromotionWinsSlot(t *testing.T) {
	f := newQuestionFixture(t)
	row := f.ask(t)
	f.answer(t, row, f.answerer, "per line")
	f.finishAskingRun(t)
	starter := f.svc.TxStarter
	competing := util.MustParseUUID(f.fx.Task(t, util.UUIDToString(f.agent), testutil.Cols{"runtime_id": f.runtime, "issue_id": f.issue, "status": "deferred"}))
	f.svc.TxStarter = questionEnqueueRaceStarter{TxStarter: starter, beforeInsert: func() {
		// Promotion can occupy the pending slot without changing its issue key.
		f.fx.Exec(t, `UPDATE agent_task_queue SET status = 'queued' WHERE id = $1`, competing)
	}}
	result, err := f.svc.Wake(context.Background(), f.workspace, util.MustParseUUID(row.ID), f.other, pgtype.UUID{})
	if err != nil || result.TaskID != "" || result.AnswerCount != 1 || !competing.Valid {
		t.Fatalf("contended acceptance = %+v, %v; competing=%+v", result, err, competing)
	}
	pending, err := f.svc.Queries.GetQuestionWakePending(context.Background(), db.GetQuestionWakePendingParams{WorkspaceID: f.workspace, IssueID: f.issue, AgentID: f.agent})
	if err != nil || pending.ActorUserID != f.anouk || pending.SourceQuestionID != util.MustParseUUID(row.ID) {
		t.Fatalf("contended first-waker intent = %+v, %v", pending, err)
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM activity_log WHERE issue_id = $1 AND action = 'question_wake'`, f.issue); n != 0 {
		t.Fatalf("contended wake prematurely wrote %d activities", n)
	}
	f.svc.TxStarter = starter
	f.fx.Exec(t, `UPDATE agent_task_queue SET status = 'completed', dispatched_at = clock_timestamp(), started_at = clock_timestamp(), completed_at = clock_timestamp() WHERE id = $1`, competing)
	drainQuestionWait(t, f)
	if n := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND trigger_evidence_kind = 'question_wake' AND originator_user_id = $2`, f.issue, f.anouk); n != 1 {
		t.Fatalf("accepted collision did not eventually wake first member: %d", n)
	}
}

func TestQuestionWakeRecoveryPagesPastUnavailableWaits(t *testing.T) {
	unavailable := newQuestionFixture(t)
	first := unavailable.ask(t)
	unavailable.answer(t, first, unavailable.answerer, "per line")
	acceptQuestionWait(t, unavailable, first, unavailable.answerer)
	unavailable.finishAskingRun(t)
	unavailable.fx.Exec(t, `UPDATE agent SET archived_at = now() WHERE id = $1`, unavailable.agent)

	ready := newQuestionFixture(t)
	second := ready.ask(t)
	ready.answer(t, second, ready.answerer, "per total")
	acceptQuestionWait(t, ready, second, ready.answerer)
	ready.finishAskingRun(t)
	if err := ready.svc.ReconcilePendingQuestionWakes(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if n := unavailable.fx.Count(t, `SELECT count(*) FROM question_wake_pending WHERE issue_id = $1`, unavailable.issue); n != 1 {
		t.Fatalf("unavailable wait was lost: %d", n)
	}
	if n := ready.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1 AND trigger_evidence_kind = 'question_wake'`, ready.issue); n != 1 {
		t.Fatalf("ready later wait was starved: %d", n)
	}
}

func TestQuestionPendingDrainCollisionRollsBackWithoutConsumingIntent(t *testing.T) {
	f := newQuestionFixture(t)
	row := f.ask(t)
	f.answer(t, row, f.answerer, "per line")
	acceptQuestionWait(t, f, row, f.other)
	f.finishAskingRun(t)
	starter := f.svc.TxStarter
	competing := util.MustParseUUID(f.fx.Task(t, util.UUIDToString(f.agent), testutil.Cols{"runtime_id": f.runtime, "issue_id": f.issue, "status": "deferred"}))
	f.svc.TxStarter = questionEnqueueRaceStarter{TxStarter: starter, beforeInsert: func() {
		f.fx.Exec(t, `UPDATE agent_task_queue SET status = 'queued' WHERE id = $1`, competing)
	}}
	err := f.svc.DrainQuestionWake(context.Background(), f.workspace, f.agent, f.issue)
	if !errors.Is(err, ErrDuplicatePendingTask) {
		t.Fatalf("contended drain = %v", err)
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM question_wake_pending WHERE issue_id = $1 AND actor_user_id = $2`, f.issue, f.anouk); n != 1 {
		t.Fatalf("collision consumed first-waker intent: %d", n)
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM question_event WHERE workspace_id = $1 AND kind = 'woken'`, f.workspace); n != 0 {
		t.Fatalf("collision committed %d woken events", n)
	}
	f.svc.TxStarter = starter
	f.fx.Exec(t, `UPDATE agent_task_queue SET status = 'completed', completed_at = clock_timestamp() WHERE id = $1`, competing)
	drainQuestionWait(t, f)
}
