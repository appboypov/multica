package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// waitForBlockedBackends waits until n backends park on a lock, so the test
// controls which request queues first on the chat_session row.
func waitForBlockedBackends(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var blocked int
		if err := testPool.QueryRow(context.Background(), `
			SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'
		`).Scan(&blocked); err == nil && blocked >= n {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("fewer than %d backends blocked on a lock", n)
}

// Wake and chat deletion both lock the chat before the question. A held chat
// lock queues both requests in a chosen order; on release neither may abort
// with a deadlock, whichever goes first.
func TestWakeChatQuestionAndDeleteChat_DoNotDeadlock(t *testing.T) {
	for _, tc := range []struct {
		name       string
		deleteWins bool
		wakeCode   int
		taskStatus string
	}{
		{name: "delete queued first", deleteWins: true, wakeCode: http.StatusUnprocessableEntity},
		{name: "wake queued first", wakeCode: http.StatusAccepted, taskStatus: "cancelled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			fx, sessionID := newChatQuestionAPIFixture(t)
			id := askQuestionForTest(t, fx)
			dbfx.Exec(t, "UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1", fx.TaskID)
			dbfx.Exec(t, "DELETE FROM question_link WHERE question_id=$1 AND link_type='task'", id)
			answer := map[string]any{"value": map[string]any{"option": "per line"}}
			rec := httptest.NewRecorder()
			testHandler.AnswerQuestion(rec, withURLParam(newRequest("POST", "/", answer), "id", id))
			if rec.Code != http.StatusOK {
				t.Fatalf("answer: %d %s", rec.Code, rec.Body)
			}

			holder, err := testPool.Begin(ctx)
			if err != nil {
				t.Fatalf("begin lock holder: %v", err)
			}
			defer holder.Rollback(ctx)
			if _, err := holder.Exec(ctx, "SELECT id FROM chat_session WHERE id=$1 FOR UPDATE", sessionID); err != nil {
				t.Fatalf("lock chat session: %v", err)
			}

			deleteRec, wakeRec := httptest.NewRecorder(), httptest.NewRecorder()
			deleteReq := withChatTestWorkspaceCtx(t, withURLParam(newRequest("DELETE", "/", nil), "sessionId", sessionID))
			wakeReq := withURLParam(newRequest("POST", "/", nil), "id", id)
			deleteDone, wakeDone := make(chan struct{}), make(chan struct{})
			runDelete := func() {
				go func() { defer close(deleteDone); testHandler.DeleteChatSession(deleteRec, deleteReq) }()
			}
			runWake := func() {
				go func() { defer close(wakeDone); testHandler.WakeQuestion(wakeRec, wakeReq) }()
			}
			if tc.deleteWins {
				runDelete()
				waitForBlockedBackends(t, 1)
				runWake()
			} else {
				runWake()
				waitForBlockedBackends(t, 1)
				runDelete()
			}
			waitForBlockedBackends(t, 2)
			if err := holder.Rollback(ctx); err != nil {
				t.Fatalf("release chat session lock: %v", err)
			}
			<-deleteDone
			<-wakeDone

			if deleteRec.Code != http.StatusNoContent {
				t.Fatalf("delete: %d %s", deleteRec.Code, deleteRec.Body)
			}
			if wakeRec.Code != tc.wakeCode {
				t.Fatalf("wake: %d %s, want %d", wakeRec.Code, wakeRec.Body, tc.wakeCode)
			}
			if n := dbfx.Count(t, "SELECT count(*) FROM chat_session WHERE id=$1", sessionID); n != 0 {
				t.Fatalf("chat survived its delete")
			}
			var status string
			dbfx.QueryRow(t, "SELECT status FROM question WHERE id=$1", id).Scan(&status)
			if status != "withdrawn" {
				t.Fatalf("question status=%s, want withdrawn", status)
			}
			var taskStatus string
			dbfx.QueryRow(t, `SELECT coalesce(max(status), '') FROM agent_task_queue
				WHERE agent_id=$1 AND id<>$2`, fx.AgentID, fx.TaskID).Scan(&taskStatus)
			if taskStatus != tc.taskStatus {
				t.Fatalf("wake turn status=%q, want %q", taskStatus, tc.taskStatus)
			}
		})
	}
}
