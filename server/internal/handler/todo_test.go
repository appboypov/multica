package handler

import (
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func todoWorkspace(t *testing.T) string {
	t.Helper()
	id := dbfx.Workspace(t, "Todo API", "todo-api-"+uuid.NewString(), testutil.Cols{"issue_prefix": "TODO"})
	dbfx.Member(t, id, testUserID, "owner")
	return id
}

func insertHandlerTodo(t *testing.T, workspaceID, kind string, overrides testutil.Cols) string {
	t.Helper()
	cols := testutil.Cols{
		"workspace_id": workspaceID,
		"member_id":    testUserID,
		"kind":         kind,
		"source_key":   uuid.NewString(),
		"title":        "Snapshot title",
	}
	for key, value := range overrides {
		cols[key] = value
	}
	return dbfx.Insert(t, "todo", cols)
}

func todoDoneRequest(id, workspaceID string) *http.Request {
	return testutil.WithURLParams(inboxRequest(http.MethodPost, "/api/todos/"+id+"/done", workspaceID), "id", id)
}

func TestTodoListOrderOverdueAndLiveProjection(t *testing.T) {
	workspaceID := todoWorkspace(t)
	issueID := dbfx.Issue(t, "Current issue title", testutil.Cols{
		"workspace_id": workspaceID, "status": "in_review", "due_date": testutil.Raw("CURRENT_DATE + 1"),
	})
	questionID := uuid.NewString()
	dbfx.Insert(t, "question", testutil.Cols{
		"id": questionID, "ask_id": questionID, "ask_position": 0,
		"workspace_id": workspaceID, "asker_agent_id": uuid.NewString(),
		"asked_in_task_id": uuid.NewString(), "prompt": "Approve?", "answer_type": "approval",
		"due_date": testutil.Raw("CURRENT_DATE - 1"),
	})
	question := insertHandlerTodo(t, workspaceID, "question", testutil.Cols{
		"issue_id": issueID, "question_id": questionID, "actor_type": "agent", "actor_id": uuid.NewString(),
		"created_at": testutil.Raw("now() - interval '7 days'"),
	})
	assigned := insertHandlerTodo(t, workspaceID, "assigned", testutil.Cols{
		"issue_id": issueID, "created_at": testutil.Raw("now() - interval '1 day'"),
	})
	mention := insertHandlerTodo(t, workspaceID, "mention", testutil.Cols{
		"issue_id": issueID, "comment_id": uuid.NewString(), "created_at": testutil.Raw("now() - interval '1 hour'"),
	})
	insertHandlerTodo(t, workspaceID, "review", testutil.Cols{"issue_id": issueID})
	insertHandlerTodo(t, workspaceID, "mention", testutil.Cols{"issue_id": uuid.NewString()})
	insertHandlerTodo(t, workspaceID, "mention", testutil.Cols{"member_id": uuid.NewString()})
	insertHandlerTodo(t, workspaceID, "mention", testutil.Cols{"done_at": testutil.Raw("now()"), "done_by": "member"})
	insertHandlerTodo(t, todoWorkspace(t), "mention", nil)

	var rows []map[string]any
	testutil.Call(t, inboxWorkspaceHandler(testHandler.ListTodos), inboxRequest(http.MethodGet, "/api/todos", workspaceID)).Want(http.StatusOK).JSON(&rows)
	if len(rows) != 3 || rows[0]["id"] != question || rows[1]["id"] != mention || rows[2]["id"] != assigned {
		t.Fatalf("want overdue question then newest mention and assignment, got %+v", rows)
	}
	first := rows[0]
	if first["overdue"] != true || first["question_answer_type"] != "approval" || first["issue_identifier"] != "TODO-1" || first["title"] != "Current issue title" || first["issue_title"] != "Current issue title" || first["issue_status"] != "in_review" {
		t.Fatalf("live source fields missing: %+v", first)
	}
	if _, err := time.Parse("2006-01-02", first["due_date"].(string)); err != nil {
		t.Fatalf("due_date must be date-only: %v", err)
	}
	if _, err := time.Parse(time.RFC3339Nano, first["created_at"].(string)); err != nil {
		t.Fatalf("created_at must be RFC3339: %v", err)
	}
	if rows[1]["due_date"] != nil || rows[1]["overdue"] != false || rows[2]["due_date"] == nil || rows[2]["overdue"] != false {
		t.Fatalf("due dates must follow each kind, got %+v", rows)
	}
	for _, key := range []string{"done_at", "done_by", "body", "task_id", "autopilot_id", "comment_id"} {
		if value, exists := first[key]; !exists || value != nil {
			t.Errorf("%s = %v (present=%v), want explicit null", key, value, exists)
		}
	}
	if details, ok := first["details"].(map[string]any); !ok || len(details) != 0 {
		t.Fatalf("details = %v, want empty object", first["details"])
	}
	if _, leaked := first["member_id"]; leaked {
		t.Fatal("response leaked an internal field")
	}
}

func TestTodoReviewHiddenInListAndSummaryUntilQuestionDone(t *testing.T) {
	workspaceID := todoWorkspace(t)
	issueID := dbfx.Issue(t, "Review", testutil.Cols{"workspace_id": workspaceID, "status": "in_review"})
	reviewID := insertHandlerTodo(t, workspaceID, "review", testutil.Cols{"issue_id": issueID})
	questionID := insertHandlerTodo(t, workspaceID, "question", testutil.Cols{"issue_id": issueID})
	for _, want := range []string{questionID, reviewID} {
		var rows []map[string]any
		testutil.Call(t, inboxWorkspaceHandler(testHandler.ListTodos), inboxRequest(http.MethodGet, "/api/todos", workspaceID)).Want(http.StatusOK).JSON(&rows)
		if len(rows) != 1 || rows[0]["id"] != want {
			t.Fatalf("visible row = %+v, want %s", rows, want)
		}
		var summary []struct {
			WorkspaceID string `json:"workspace_id"`
			Count       int64  `json:"count"`
		}
		testutil.Call(t, testHandler.TodoSummary, inboxRequest(http.MethodGet, "/api/todos/summary", "")).Want(http.StatusOK).JSON(&summary)
		if len(summary) != 1 || summary[0].WorkspaceID != workspaceID || summary[0].Count != 1 {
			t.Fatalf("summary must match visible list: %+v", summary)
		}
		if want == questionID {
			testutil.Call(t, inboxWorkspaceHandler(testHandler.MarkTodoDone), todoDoneRequest(questionID, workspaceID)).Want(http.StatusOK)
		}
	}
}

func TestTodoSummaryAcrossCurrentMemberships(t *testing.T) {
	first, second, empty := todoWorkspace(t), todoWorkspace(t), todoWorkspace(t)
	left := dbfx.Workspace(t, "Left workspace", "todo-left-"+uuid.NewString())
	insertHandlerTodo(t, first, "mention", nil)
	insertHandlerTodo(t, first, "assigned", nil)
	insertHandlerTodo(t, second, "quota_reached", nil)
	insertHandlerTodo(t, left, "mention", nil)
	insertHandlerTodo(t, empty, "mention", testutil.Cols{"member_id": uuid.NewString()})
	insertHandlerTodo(t, first, "mention", testutil.Cols{"done_at": testutil.Raw("now()"), "done_by": "handled"})
	insertHandlerTodo(t, first, "mention", testutil.Cols{"issue_id": uuid.NewString()})
	for _, selectedWorkspace := range []string{"", first, left} {
		var rows []struct {
			WorkspaceID string `json:"workspace_id"`
			Count       int64  `json:"count"`
		}
		testutil.Call(t, testHandler.TodoSummary, inboxRequest(http.MethodGet, "/api/todos/summary", selectedWorkspace)).Want(http.StatusOK).JSON(&rows)
		counts := map[string]int64{}
		for _, row := range rows {
			counts[row.WorkspaceID] = row.Count
		}
		if !reflect.DeepEqual(counts, map[string]int64{first: 2, second: 1}) {
			t.Fatalf("summary with workspace %q = %v", selectedWorkspace, counts)
		}
	}
}

func TestTodoMarkDoneOwnershipIdempotenceAndRealtime(t *testing.T) {
	workspaceID, otherWorkspace := todoWorkspace(t), todoWorkspace(t)
	id := insertHandlerTodo(t, workspaceID, "quick_create_failed", testutil.Cols{
		"title": "Create an issue", "body": "Run failed", "task_id": uuid.NewString(),
		"details": testutil.Raw(`'{"prompt":"Create an issue","reason":"failed"}'::jsonb`),
	})
	otherMember := insertHandlerTodo(t, workspaceID, "mention", testutil.Cols{"member_id": uuid.NewString()})
	bus := events.New()
	h := &Handler{Queries: testHandler.Queries, Bus: bus}
	closed := 0
	bus.Subscribe(protocol.EventTodoClosed, func(e events.Event) {
		closed++
		payload := e.Payload.(map[string]any)
		if e.WorkspaceID != workspaceID || payload["member_id"] != testUserID || payload["todo_id"] != id || payload["kind"] != "quick_create_failed" || payload["done_by"] != "member" {
			t.Fatalf("wrong closed event: %+v", e)
		}
	})
	mark := middleware.RequireWorkspaceMember(h.Queries)(http.HandlerFunc(h.MarkTodoDone)).ServeHTTP
	for _, denied := range []struct{ id, workspace string }{{otherMember, workspaceID}, {id, otherWorkspace}, {uuid.NewString(), workspaceID}} {
		testutil.Call(t, mark, todoDoneRequest(denied.id, denied.workspace)).Want(http.StatusNotFound)
	}
	testutil.Call(t, mark, todoDoneRequest("not-a-uuid", workspaceID)).Want(http.StatusBadRequest)
	var first map[string]any
	for attempt := range 2 {
		var row map[string]any
		testutil.Call(t, mark, todoDoneRequest(id, workspaceID)).Want(http.StatusOK).JSON(&row)
		if attempt == 0 {
			first = row
		} else if !reflect.DeepEqual(first, row) {
			t.Fatalf("repeated completion changed response: first=%v repeated=%v", first, row)
		}
		if row["id"] != id || row["done_by"] != "member" || row["done_at"] == nil || row["title"] != "Create an issue" || row["body"] != "Run failed" || row["overdue"] != false {
			t.Fatalf("wrong completed response: %+v", row)
		}
		if row["details"].(map[string]any)["prompt"] != "Create an issue" || row["issue_id"] != nil || row["actor_id"] != nil || row["actor_type"] != nil || row["task_id"] == nil {
			t.Fatalf("system action fields lost: %+v", row)
		}
	}
	if closed != 1 {
		t.Fatalf("closed events = %d, want one real transition", closed)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM todo WHERE id = $1 AND done_at IS NULL", otherMember); n != 1 {
		t.Fatal("another member's todo was closed")
	}
	handledID := insertHandlerTodo(t, workspaceID, "mention", testutil.Cols{"done_at": testutil.Raw("now()"), "done_by": "handled"})
	var handled map[string]any
	testutil.Call(t, mark, todoDoneRequest(handledID, workspaceID)).Want(http.StatusOK).JSON(&handled)
	if handled["done_by"] != "handled" || closed != 1 {
		t.Fatalf("already handled completion changed: row=%+v events=%d", handled, closed)
	}
	var rows []map[string]any
	testutil.Call(t, inboxWorkspaceHandler(testHandler.ListTodos), inboxRequest(http.MethodGet, "/api/todos", workspaceID)).Want(http.StatusOK).JSON(&rows)
	if rows == nil || len(rows) != 0 {
		t.Fatalf("completed todos must leave an empty array: %+v", rows)
	}
	var summary []map[string]any
	testutil.Call(t, h.TodoSummary, inboxRequest(http.MethodGet, "/api/todos/summary", "")).Want(http.StatusOK).JSON(&summary)
	if summary == nil || len(summary) != 0 {
		t.Fatalf("completed todos must leave an empty summary array: %+v", summary)
	}
}

func TestTodoEndpointsRequireMember(t *testing.T) {
	workspaceID := todoWorkspace(t)
	for _, handler := range []http.HandlerFunc{testHandler.ListTodos, testHandler.TodoSummary, testHandler.MarkTodoDone} {
		testutil.Call(t, handler, testutil.JSONRequest(http.MethodGet, "/api/todos", nil)).Want(http.StatusUnauthorized)
	}
	outsider := dbfx.Workspace(t, "Not a member", "todo-outsider-"+uuid.NewString())
	for _, handler := range []http.HandlerFunc{testHandler.ListTodos, testHandler.MarkTodoDone} {
		testutil.Call(t, inboxWorkspaceHandler(handler), inboxRequest(http.MethodGet, "/api/todos", outsider)).Want(http.StatusNotFound)
		testutil.Call(t, inboxWorkspaceHandler(handler), inboxRequest(http.MethodGet, "/api/todos", "")).Want(http.StatusBadRequest)
	}
	var rows []map[string]any
	testutil.Call(t, inboxWorkspaceHandler(testHandler.ListTodos), inboxRequest(http.MethodGet, "/api/todos", workspaceID)).Want(http.StatusOK).JSON(&rows)
	if rows == nil || len(rows) != 0 {
		t.Fatalf("empty list = %+v, want []", rows)
	}
}
