package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// newChatHistoryTask inserts a chat task bound to a fresh chat session and
// returns the task id. With chatSession=false it inserts a non-chat task.
func newChatHistoryTask(t *testing.T, chatSession bool) string {
	t.Helper()
	agentID := createHandlerTestAgent(t, "ChatHistoryAgent", []byte("[]"))
	runtimeID := handlerTestRuntimeID(t)
	var sessionArg any
	if chatSession {
		sessionArg = createHandlerTestChatSession(t, agentID)
	}
	var taskID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_task_queue (agent_id, runtime_id, status, priority, chat_session_id)
		VALUES ($1, $2, 'completed', 0, $3)
		RETURNING id
	`, agentID, runtimeID, sessionArg).Scan(&taskID); err != nil {
		t.Fatalf("insert chat history task: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, taskID)
	})
	return taskID
}

// newChatHistoryTaskForSession inserts a chat task bound to a specific chat
// session and returns the task id.
func newChatHistoryTaskForSession(t *testing.T, sessionID string) string {
	t.Helper()
	agentID := createHandlerTestAgent(t, "ChatHistorySessionAgent-"+uuid.NewString(), []byte("[]"))
	runtimeID := handlerTestRuntimeID(t)
	var taskID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_task_queue (agent_id, runtime_id, status, priority, chat_session_id)
		VALUES ($1, $2, 'completed', 0, $3)
		RETURNING id
	`, agentID, runtimeID, sessionID).Scan(&taskID); err != nil {
		t.Fatalf("insert chat history task for session: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, taskID)
	})
	return taskID
}

// taskActorReq builds a request as the Auth middleware would leave it for a mat_
// task token: the server-set X-Actor-Source=task_token + the authoritative
// X-Task-ID. target may carry query params (e.g. "?limit=3").
func taskActorReq(target, taskID string) *http.Request {
	req := newRequest("GET", target, nil)
	req.Header.Set("X-Actor-Source", "task_token")
	req.Header.Set("X-Task-ID", taskID)
	return req
}

// insertChatHistoryMessage stores one transcript row with an explicit role and
// timestamp, so the read-back order and role mapping are deterministic.
func insertChatHistoryMessage(t *testing.T, sessionID, role, content string, createdAt time.Time) {
	t.Helper()
	if _, err := testPool.Exec(context.Background(), `
		INSERT INTO chat_message (chat_session_id, role, content, created_at)
		VALUES ($1, $2, $3, $4)
	`, sessionID, role, content, createdAt); err != nil {
		t.Fatalf("insert chat message role=%q %q: %v", role, content, err)
	}
}

// A fresh session has no messages: an empty page, no cursor.
func TestGetChatHistory_EmptySession(t *testing.T) {
	if testHandler == nil {
		t.Skip("requires test database")
	}
	taskID := newChatHistoryTask(t, true)
	w := httptest.NewRecorder()
	testHandler.GetChatHistory(w, taskActorReq("/api/chat/history", taskID))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var resp ChatHistoryResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Messages) != 0 || resp.NextCursor != "" {
		t.Fatalf("expected empty transcript, got %+v", resp)
	}
}

// The stored chat_message rows come back oldest-first, so an agent can rebuild
// the conversation after a lost resume.
func TestGetChatHistory_ReadsStoredTranscript(t *testing.T) {
	if testHandler == nil {
		t.Skip("requires test database")
	}
	agentID := createHandlerTestAgent(t, "ChatHistoryTranscriptAgent", []byte("[]"))
	sessionID := createHandlerTestChatSession(t, agentID)
	taskID := newChatHistoryTaskForSession(t, sessionID)
	base := time.Date(2026, 8, 10, 1, 0, 0, 0, time.UTC)
	insertChatHistoryMessage(t, sessionID, "user", "first user turn", base)
	insertChatHistoryMessage(t, sessionID, "assistant", "assistant reply", base.Add(time.Second))
	insertChatHistoryMessage(t, sessionID, "user", "second user turn", base.Add(2*time.Second))

	w := httptest.NewRecorder()
	testHandler.GetChatHistory(w, taskActorReq("/api/chat/history", taskID))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var resp ChatHistoryResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d: %+v", len(resp.Messages), resp.Messages)
	}
	if resp.Messages[0].Text != "first user turn" || resp.Messages[1].Text != "assistant reply" || resp.Messages[2].Text != "second user turn" {
		t.Fatalf("transcript not oldest-first: %+v", resp.Messages)
	}
	if resp.Messages[1].Role != "assistant" || resp.Messages[0].Role != "user" {
		t.Errorf("roles = %q/%q, want user/assistant", resp.Messages[0].Role, resp.Messages[1].Role)
	}
	if resp.Messages[0].Author != "User" || resp.Messages[1].Author != "Bot" || resp.Messages[2].Author != "User" {
		t.Errorf("transcript authors = %q/%q/%q, want User/Bot/User",
			resp.Messages[0].Author, resp.Messages[1].Author, resp.Messages[2].Author)
	}
	if got := resp.Messages[0].TS; got != base.UTC().Format(time.RFC3339Nano) {
		t.Errorf("transcript TS = %q, want %q", got, base.UTC().Format(time.RFC3339Nano))
	}
}

// Paging with ?limit and the advertised cursor walks the whole transcript
// exactly once: a full page advertises a cursor, a short page does not.
func TestGetChatHistory_TranscriptPagesWithoutDuplicatesOrGaps(t *testing.T) {
	if testHandler == nil {
		t.Skip("requires test database")
	}
	agentID := createHandlerTestAgent(t, "ChatHistoryPagingAgent", []byte("[]"))
	sessionID := createHandlerTestChatSession(t, agentID)
	taskID := newChatHistoryTaskForSession(t, sessionID)
	base := time.Date(2026, 8, 11, 2, 0, 0, 0, time.UTC)
	const total = 7
	for i := range total {
		insertChatHistoryMessage(t, sessionID, "user", "msg "+strconv.Itoa(i), base.Add(time.Duration(i)*time.Second))
	}

	seen := make([]string, 0, total)
	before := ""
	for page := 0; ; page++ {
		target := "/api/chat/history?limit=3"
		if before != "" {
			target += "&before=" + url.QueryEscape(before)
		}
		w := httptest.NewRecorder()
		testHandler.GetChatHistory(w, taskActorReq(target, taskID))
		if w.Code != http.StatusOK {
			t.Fatalf("page %d status = %d, want 200: %s", page, w.Code, w.Body.String())
		}
		var resp ChatHistoryResponse
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		// Within a page the transcript is oldest-first.
		prev := -1
		for _, m := range resp.Messages {
			seen = append(seen, m.Text)
			n, _ := strconv.Atoi(strings.TrimPrefix(m.Text, "msg "))
			if n <= prev {
				t.Fatalf("page %d not oldest-first: %v", page, resp.Messages)
			}
			prev = n
		}
		if len(resp.Messages) < 3 {
			// A short page is the last one: no cursor advertised.
			if resp.NextCursor != "" {
				t.Fatalf("last page advertised a cursor: %q", resp.NextCursor)
			}
			break
		}
		if resp.NextCursor == "" {
			t.Fatalf("full page %d advertised no cursor", page)
		}
		before = resp.NextCursor
	}

	// The union of all pages is exactly the full transcript: no duplicates, no
	// gaps. (Across pages the sequence is NOT monotonic - paging to older
	// messages means each later page is older than the previous one.)
	if len(seen) != total {
		t.Fatalf("paged %d messages, want %d: %v", len(seen), total, seen)
	}
	dup := map[string]bool{}
	for _, text := range seen {
		if dup[text] {
			t.Fatalf("duplicate message %q across pages: %v", text, seen)
		}
		dup[text] = true
	}
	for i := range total {
		if want := "msg " + strconv.Itoa(i); !dup[want] {
			t.Fatalf("missing %q across pages: %v", want, seen)
		}
	}
}

// TestGetChatHistory_RejectsForgedTaskID: a normal request (no server-set
// X-Actor-Source) that forges X-Task-ID - what a member could do with a JWT /
// mul_ PAT, since the Auth middleware does NOT strip a client-sent X-Task-ID -
// must be rejected, never served another session's history.
func TestGetChatHistory_RejectsForgedTaskID(t *testing.T) {
	if testHandler == nil {
		t.Skip("requires test database")
	}
	taskID := newChatHistoryTask(t, true)
	req := newRequest("GET", "/api/chat/history", nil)
	req.Header.Set("X-Task-ID", taskID) // forged: no X-Actor-Source=task_token
	w := httptest.NewRecorder()
	testHandler.GetChatHistory(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestGetChatHistory_MissingTaskHeader(t *testing.T) {
	if testHandler == nil {
		t.Skip("requires test database")
	}
	// Task-token actor source but no X-Task-ID: a defensive 400.
	req := newRequest("GET", "/api/chat/history", nil)
	req.Header.Set("X-Actor-Source", "task_token")
	w := httptest.NewRecorder()
	testHandler.GetChatHistory(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestGetChatHistory_NonChatTask(t *testing.T) {
	if testHandler == nil {
		t.Skip("requires test database")
	}
	taskID := newChatHistoryTask(t, false) // task with no chat_session_id
	w := httptest.NewRecorder()
	testHandler.GetChatHistory(w, taskActorReq("/api/chat/history", taskID))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", w.Code, w.Body.String())
	}
}
