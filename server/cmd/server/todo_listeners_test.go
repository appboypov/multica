package main

import (
	"context"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/todo"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"testing"
)

func TestTodoRealtimeIsMemberPrivate(t *testing.T) {
	for _, kind := range []string{protocol.EventTodoOpened, protocol.EventTodoClosed} {
		t.Run(kind, func(t *testing.T) {
			bus := events.New()
			broadcaster := &fakeBroadcaster{}
			registerListeners(bus, broadcaster)
			bus.Publish(events.Event{Type: kind, WorkspaceID: "workspace", Payload: map[string]any{"member_id": "member"}})
			if len(broadcaster.userCalls) != 1 || len(broadcaster.workspaceCalls) != 0 || broadcaster.broadcastCalled != 0 {
				t.Fatalf("wrong todo audience: %+v", broadcaster)
			}
		})
	}
}

func todoListenerFixture(t *testing.T) (*testutil.Fixture, todo.Service) {
	t.Helper()
	fx := testutil.New(testPool, "", "")
	suffix := dbid.NewV7().String()
	fx.UserID = fx.User(t, "Brian", suffix+"@todo.test")
	fx.WorkspaceID = fx.Workspace(t, "Todo listeners", "todos-"+suffix)
	fx.Member(t, fx.WorkspaceID, fx.UserID, "owner")
	fx.Cleanup(t, "DELETE FROM todo WHERE workspace_id = $1", fx.WorkspaceID)
	return fx, todo.Service{Queries: db.New(testPool), Bus: events.New()}
}

func TestTodoQuestionTransitions(t *testing.T) {
	for _, policy := range []string{"any_one", "each"} {
		t.Run(policy, func(t *testing.T) {
			fx, svc := todoListenerFixture(t)
			other := fx.User(t, "Anouk", dbid.NewV7().String()+"@todo.test")
			fx.Member(t, fx.WorkspaceID, other, "member")
			issue := fx.Issue(t, "Approval")
			id := dbid.NewV7()
			fx.Insert(t, "question", testutil.Cols{"id": id, "ask_id": id, "ask_position": 0, "workspace_id": fx.WorkspaceID, "asker_agent_id": dbid.NewV7(), "asked_in_task_id": dbid.NewV7(), "prompt": "Approve?", "answer_type": "approval", "answer_policy": policy})
			for _, user := range []string{fx.UserID, other} {
				fx.Exec(t, "INSERT INTO question_answerer (question_id, user_id, workspace_id) VALUES ($1, $2, $3)", id, user, fx.WorkspaceID)
			}
			payload := protocol.QuestionEventPayload{QuestionID: util.UUIDToString(id), IssueID: issue}
			event := events.Event{Type: protocol.EventQuestionCreated, WorkspaceID: fx.WorkspaceID, Payload: payload}
			apply := func() {
				t.Helper()
				if err := questionTodos(context.Background(), svc, event); err != nil {
					t.Fatal(err)
				}
			}
			count := func(user string) int {
				return fx.Count(t, "SELECT count(*) FROM todo WHERE workspace_id = $1 AND member_id = $2 AND done_at IS NULL", fx.WorkspaceID, user)
			}
			apply()
			if count(fx.UserID) != 1 || count(other) != 1 {
				t.Fatal("ask did not reach both answerers")
			}
			status := "answered"
			if policy == "each" {
				status = "partly_answered"
			}
			fx.Exec(t, "UPDATE question SET status = $2 WHERE id = $1", id, status)
			payload.Reason = protocol.QuestionReasonAnswered
			event.Type, event.ActorID, event.Payload = protocol.EventQuestionUpdated, fx.UserID, payload
			apply()
			wantOther := 0
			if policy == "each" {
				wantOther = 1
			}
			if count(fx.UserID) != 0 || count(other) != wantOther {
				t.Fatal("answer closed wrong recipients")
			}
			fx.Exec(t, "UPDATE question SET status = 'withdrawn' WHERE id = $1", id)
			payload.Reason = protocol.QuestionReasonWithdrawn
			event.Payload = payload
			apply()
			if count(other) != 0 {
				t.Fatal("withdraw did not close all recipients")
			}
		})
	}
}

func TestTodoQuestionReassigned(t *testing.T) {
	fx, svc := todoListenerFixture(t)
	other := fx.User(t, "Anouk", dbid.NewV7().String()+"@todo.test")
	fx.Member(t, fx.WorkspaceID, other, "member")
	issue := fx.Issue(t, "Reassign")
	id := dbid.NewV7()
	fx.Insert(t, "question", testutil.Cols{"id": id, "ask_id": id, "ask_position": 0, "workspace_id": fx.WorkspaceID, "asker_agent_id": dbid.NewV7(), "asked_in_task_id": dbid.NewV7(), "prompt": "Approve?", "answer_type": "approval"})
	fx.Exec(t, "INSERT INTO question_answerer (question_id, user_id, workspace_id) VALUES ($1, $2, $3)", id, fx.UserID, fx.WorkspaceID)
	payload := protocol.QuestionEventPayload{QuestionID: util.UUIDToString(id), IssueID: issue}
	event := events.Event{Type: protocol.EventQuestionCreated, WorkspaceID: fx.WorkspaceID, Payload: payload}
	if err := questionTodos(context.Background(), svc, event); err != nil {
		t.Fatal(err)
	}
	fx.Exec(t, "UPDATE question_answerer SET user_id = $2 WHERE question_id = $1", id, other)
	eventID := fx.Insert(t, "question_event", testutil.Cols{"id": dbid.NewV7(), "question_id": id, "workspace_id": fx.WorkspaceID, "kind": "reassigned", "actor_type": "member", "actor_id": fx.UserID, "details": `{"user_id":"` + other + `"}`})
	payload.Reason, payload.EventID = protocol.QuestionReasonReassigned, eventID
	event.Type, event.ActorID, event.Payload = protocol.EventQuestionUpdated, fx.UserID, payload
	if err := questionTodos(context.Background(), svc, event); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, "SELECT count(*) FROM todo WHERE workspace_id = $1 AND member_id = $2 AND done_at IS NULL", fx.WorkspaceID, fx.UserID) != 0 ||
		fx.Count(t, "SELECT count(*) FROM todo WHERE workspace_id = $1 AND member_id = $2 AND done_at IS NULL", fx.WorkspaceID, other) != 1 {
		t.Fatal("reassignment kept wrong recipient")
	}
}
