package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// TestArchivingAChatSessionLeavesItsTasksRunning: archive is the reversible
// organizing action the UI presents — the row offers Stop OR Archive, never
// both; Stop is `danger` behind a confirmation and Archive fires immediately
// with none; unarchive undoes it and hard delete is offered only afterwards.
// Cancelling here would make that immediate, unconfirmed click destructive, and
// irreversibly so: the Stop button cancels through CancelTaskByUser, which
// returns the prompt via cancelled_chat_message.restore_to_input or writes a
// durable chat_draft_restore row, while CancelAgentTasksByChatSession does
// neither and unarchive cannot undo a cancel.
func TestArchivingAChatSessionLeavesItsTasksRunning(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	f := newArchiveFixture(t, "Archive Leaves Chat")

	f.archive(t)

	for _, tc := range []struct{ label, id, want string }{
		{"queued", f.queuedTaskID, "queued"},
		{"running", f.runningTaskID, "running"},
	} {
		if status := f.taskStatus(t, tc.id); status != tc.want {
			t.Fatalf("%s task status after archiving a chat = %q, want %q — cancelling destroys the user's typed prompt: this path writes no chat_draft_restore row and returns no restore_to_input, so the text is gone and unarchiving cannot bring it back", tc.label, status, tc.want)
		}
	}

	if agentStatus := f.agentStatus(t); agentStatus != "working" {
		t.Errorf("agent status after archiving a chat = %q, want working — its turn is still running", agentStatus)
	}

	if seen := f.drainCancelledEvents(); len(seen) != 0 {
		t.Errorf("archiving a chat emitted task:cancelled for %d task(s) — every client drops a turn that is still running", len(seen))
	}

	if live := f.liveTaskTokens(t); live != 1 {
		t.Errorf("running task has %d live task token(s) after archiving a chat, want 1 — revoking it strands the agent process mid-turn", live)
	}

	var status string
	if err := testPool.QueryRow(ctx,
		`SELECT status FROM chat_session WHERE id = $1`, f.sessionID).Scan(&status); err != nil {
		t.Fatalf("reload chat session: %v", err)
	}
	if status != "archived" {
		t.Fatalf("chat session status = %q, want archived", status)
	}
}

// archiveFixture is one agent marked 'working' with one chat session carrying
// two in-flight turns — one queued, one already running — plus a live task
// token for the running one and a filtered task:cancelled subscription.
type archiveFixture struct {
	agentID       string
	sessionID     string
	queuedTaskID  string
	runningTaskID string
	cancelled     chan string
}

func newArchiveFixture(t *testing.T, agentName string) *archiveFixture {
	t.Helper()
	ctx := context.Background()

	f := &archiveFixture{}
	f.agentID = createHandlerTestAgent(t, agentName, []byte("[]"))
	f.sessionID = createHandlerTestChatSession(t, f.agentID)
	f.queuedTaskID = queueArchiveTestTask(t, f.agentID, f.sessionID, "queued")
	f.runningTaskID = queueArchiveTestTask(t, f.agentID, f.sessionID, "running")

	if _, err := testPool.Exec(ctx,
		`UPDATE agent SET status = 'working' WHERE id = $1`, f.agentID); err != nil {
		t.Fatalf("mark agent working: %v", err)
	}

	// A live task token for the running task: captureTaskCancelled would revoke
	// it, so it must survive the archive untouched.
	if _, err := testPool.Exec(ctx, `
		INSERT INTO task_token (token_hash, task_id, agent_id, workspace_id, user_id, expires_at)
		VALUES ($1, $2, $3, $4, $5, now() + interval '24 hours')
	`, "archive-test-"+f.runningTaskID, f.runningTaskID, f.agentID, testWorkspaceID, testUserID); err != nil {
		t.Fatalf("mint task token: %v", err)
	}

	// Bus handlers are never unsubscribed in this suite, so filter to this
	// fixture's own task ids and never block a later publisher.
	f.cancelled = make(chan string, 4)
	testHandler.Bus.Subscribe(protocol.EventTaskCancelled, func(e events.Event) {
		payload, ok := e.Payload.(map[string]any)
		if !ok {
			return
		}
		id, _ := payload["task_id"].(string)
		if id != f.queuedTaskID && id != f.runningTaskID {
			return
		}
		select {
		case f.cancelled <- id:
		default:
		}
	})

	return f
}

// archive goes through the real endpoint, so everything the handler does
// happens exactly as it does for a user clicking Archive.
func (f *archiveFixture) archive(t *testing.T) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/api/chat/sessions/"+f.sessionID+"/archived",
		bytes.NewReader([]byte(`{"archived":true}`)))
	req.Header.Set("X-User-ID", testUserID)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", f.sessionID)
	req = withChatTestWorkspaceCtx(t, req)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	testHandler.SetChatSessionArchived(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("archive returned %d: %s", w.Code, w.Body.String())
	}
}

func (f *archiveFixture) taskStatus(t *testing.T, taskID string) string {
	t.Helper()
	var status string
	if err := testPool.QueryRow(context.Background(),
		`SELECT status FROM agent_task_queue WHERE id = $1`, taskID).Scan(&status); err != nil {
		t.Fatalf("reload task %s: %v", taskID, err)
	}
	return status
}

func (f *archiveFixture) agentStatus(t *testing.T) string {
	t.Helper()
	var status string
	if err := testPool.QueryRow(context.Background(),
		`SELECT status FROM agent WHERE id = $1`, f.agentID).Scan(&status); err != nil {
		t.Fatalf("reload agent: %v", err)
	}
	return status
}

func (f *archiveFixture) liveTaskTokens(t *testing.T) int {
	t.Helper()
	var n int
	if err := testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM task_token WHERE task_id = $1`, f.runningTaskID).Scan(&n); err != nil {
		t.Fatalf("count task tokens: %v", err)
	}
	return n
}

// drainCancelledEvents reads what the bus already delivered. events.Bus.Publish
// is synchronous, so by the time the handler has returned every event this
// archive was going to emit is already in the channel — an empty drain means
// none were emitted, not that they are still in flight.
func (f *archiveFixture) drainCancelledEvents() map[string]bool {
	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case id := <-f.cancelled:
			seen[id] = true
		default:
			return seen
		}
	}
	return seen
}

func queueArchiveTestTask(t *testing.T, agentID, sessionID, status string) string {
	t.Helper()
	var taskID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_task_queue (agent_id, chat_session_id, status, runtime_id)
		VALUES ($1, $2, $3, (SELECT runtime_id FROM agent WHERE id = $1))
		RETURNING id
	`, agentID, sessionID, status).Scan(&taskID); err != nil {
		t.Fatalf("queue a %s task: %v", status, err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM task_token WHERE task_id = $1`, taskID)
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, taskID)
	})
	return taskID
}
