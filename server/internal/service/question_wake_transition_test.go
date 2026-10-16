package service

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestQuestionWakeTransitionDefersTerminalAndRecoveryUntilReplacementSettles(t *testing.T) {
	for _, replacement := range []bool{false, true} {
		name := "without_replacement"
		if replacement {
			name = "with_replacement"
		}
		t.Run(name, func(t *testing.T) {
			f := newQuestionFixture(t)
			row := f.ask(t)
			f.answer(t, row, f.answerer, "per line")
			acceptQuestionWait(t, f, row, f.answerer)
			f.svc.Tasks.QuestionWakeDrainer = f.svc
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			finish, err := f.svc.BeginQuestionWakeTransition(ctx, f.workspace, []pgtype.UUID{f.issue, f.issue})
			if err != nil {
				t.Fatal(err)
			}
			finished := false
			defer func() {
				if !finished {
					finish()
				}
			}()
			if _, err := f.svc.Tasks.CancelTask(ctx, f.task); err != nil {
				t.Fatal(err)
			}
			// Recovery in another service instance must respect the same DB fence.
			restarted := &QuestionService{Queries: f.svc.Queries, TxStarter: f.svc.TxStarter, Tasks: f.svc.Tasks, Bus: f.svc.Bus}
			if err := restarted.ReconcilePendingQuestionWakes(ctx, 1); err != nil {
				t.Fatal(err)
			}
			f.wantNoNewTask(t)
			if n := f.fx.Count(t, `SELECT count(*) FROM question_wake_pending WHERE issue_id=$1`, f.issue); n != 1 {
				t.Fatalf("in-flight lifecycle lost wait: %d", n)
			}
			var replacementID pgtype.UUID
			if replacement {
				replacementID = util.MustParseUUID(f.fx.Task(t, util.UUIDToString(f.agent), testutil.Cols{
					"runtime_id": f.runtime, "issue_id": f.issue, "status": "queued",
				}))
			}
			// Request cancellation must not strand the guard or skip its final drain.
			cancel()
			finish()
			finished = true
			want := 1
			if replacement {
				want = 0
				if n := f.fx.Count(t, `SELECT count(*) FROM question_wake_pending WHERE issue_id=$1`, f.issue); n != 1 {
					t.Fatalf("queued replacement lost wait: %d", n)
				}
			}
			if n := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND trigger_evidence_kind='question_wake'`, f.issue); n != want {
				t.Fatalf("post-transition wakes = %d, want %d", n, want)
			}
			if replacement {
				if _, err := f.svc.Tasks.CancelTask(context.Background(), replacementID); err != nil {
					t.Fatal(err)
				}
				if n := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND trigger_evidence_kind='question_wake'`, f.issue); n != 1 {
					t.Fatalf("standalone cancellation did not deliver accepted wake: %d", n)
				}
			}
			if n := f.fx.Count(t, `SELECT count(*) FROM question_wake_pending WHERE issue_id=$1`, f.issue); n != 0 {
				t.Fatalf("delivered wake retained intent: %d", n)
			}
		})
	}
}

func TestQuestionWakeTransitionAllowsNestedCancellationCallback(t *testing.T) {
	f := newQuestionFixture(t)
	row := f.ask(t)
	f.answer(t, row, f.answerer, "per line")
	f.svc.Tasks.QuestionWakeDrainer = f.svc
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	finish, err := f.svc.BeginQuestionWakeTransition(ctx, f.workspace, []pgtype.UUID{f.issue})
	if err != nil {
		t.Fatal(err)
	}
	finished := false
	defer func() {
		if !finished {
			finish()
		}
	}()
	called := false
	f.svc.Bus.Subscribe(protocol.EventTaskCancelled, func(events.Event) {
		called = true
		// A synchronous subscriber can save another edit and request Wake.
		nested, err := f.svc.BeginQuestionWakeTransition(ctx, f.workspace, []pgtype.UUID{f.issue})
		if err != nil {
			t.Errorf("nested transition: %v", err)
			return
		}
		defer nested()
		result, err := f.svc.Wake(ctx, f.workspace, util.MustParseUUID(row.ID), f.answerer, pgtype.UUID{})
		if err != nil || result.TaskID != "" || result.AnswerCount != 1 {
			t.Errorf("callback wake must wait for enclosing transition: %+v, %v", result, err)
		}
	})
	if _, err := f.svc.Tasks.CancelTask(ctx, f.task); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("cancellation callback did not run")
	}
	f.wantNoNewTask(t)
	if n := f.fx.Count(t, `SELECT count(*) FROM question_wake_pending WHERE issue_id=$1`, f.issue); n != 1 {
		t.Fatalf("nested finish lost enclosing wait: %d", n)
	}
	finish()
	finished = true
	if n := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND trigger_evidence_kind='question_wake'`, f.issue); n != 1 {
		t.Fatalf("settled transition queued %d question wakes, want one", n)
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM question_wake_pending WHERE issue_id=$1`, f.issue); n != 0 {
		t.Fatalf("delivered wake retained %d intents", n)
	}
}
