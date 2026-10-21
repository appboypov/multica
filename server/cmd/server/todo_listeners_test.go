package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/handler"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/todo"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"
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
	return fx, todo.Service{Queries: db.New(testPool), TxStarter: testPool, Bus: events.New()}
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

func TestTodoQuestionWithoutIssue(t *testing.T) {
	fx, svc := todoListenerFixture(t)
	id := dbid.NewV7()
	fx.Insert(t, "question", testutil.Cols{"id": id, "ask_id": id, "ask_position": 0, "workspace_id": fx.WorkspaceID, "asker_agent_id": dbid.NewV7(), "asked_in_task_id": dbid.NewV7(), "prompt": "Approve?", "answer_type": "approval"})
	fx.Exec(t, "INSERT INTO question_answerer (question_id, user_id, workspace_id) VALUES ($1, $2, $3)", id, fx.UserID, fx.WorkspaceID)
	event := events.Event{Type: protocol.EventQuestionCreated, WorkspaceID: fx.WorkspaceID, Payload: protocol.QuestionEventPayload{QuestionID: util.UUIDToString(id)}}
	if err := questionTodos(context.Background(), svc, event); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, "SELECT count(*) FROM todo WHERE workspace_id = $1 AND member_id = $2 AND kind = 'question' AND issue_id IS NULL AND title = 'Approve?' AND done_at IS NULL", fx.WorkspaceID, fx.UserID) != 1 {
		t.Fatal("question without an issue opened no to-do")
	}
}

// Deleting one of two linked issues keeps the question open; its open to-do
// follows the issue it still reaches, and a to-do already marked done stays done.
func TestTodoQuestionSurvivesLinkedIssueDeletion(t *testing.T) {
	fx, svc := todoListenerFixture(t)
	ctx := context.Background()
	other := fx.User(t, "Anouk", dbid.NewV7().String()+"@todo.test")
	fx.Member(t, fx.WorkspaceID, other, "member")
	deleted, kept := fx.Issue(t, "Deleted"), fx.Issue(t, "Kept")
	id := dbid.NewV7()
	fx.Insert(t, "question", testutil.Cols{"id": id, "ask_id": id, "ask_position": 0, "workspace_id": fx.WorkspaceID, "asker_agent_id": dbid.NewV7(), "asked_in_task_id": dbid.NewV7(), "prompt": "Approve?", "answer_type": "approval"})
	for position, issue := range []string{deleted, kept} {
		fx.Exec(t, "INSERT INTO question_link (question_id, workspace_id, link_type, link_id, position) VALUES ($1, $2, 'issue', $3, $4)", id, fx.WorkspaceID, issue, position)
	}
	for _, user := range []string{fx.UserID, other} {
		fx.Exec(t, "INSERT INTO question_answerer (question_id, user_id, workspace_id) VALUES ($1, $2, $3)", id, user, fx.WorkspaceID)
	}
	created := events.Event{Type: protocol.EventQuestionCreated, WorkspaceID: fx.WorkspaceID, Payload: protocol.QuestionEventPayload{QuestionID: util.UUIDToString(id), IssueID: deleted}}
	if err := questionTodos(ctx, svc, created); err != nil {
		t.Fatal(err)
	}
	fx.Exec(t, "UPDATE todo SET done_at = now(), done_by = 'member' WHERE workspace_id = $1 AND member_id = $2", fx.WorkspaceID, other)

	var listenerErr error
	svc.Bus.Subscribe(protocol.EventQuestionUpdated, func(e events.Event) {
		if err := questionTodos(ctx, svc, e); err != nil {
			listenerErr = err
		}
	})
	var opened []string
	svc.Bus.Subscribe(protocol.EventTodoOpened, func(e events.Event) {
		opened = append(opened, e.Payload.(map[string]any)["member_id"].(string))
	})
	questions := &service.QuestionService{Queries: svc.Queries, TxStarter: testPool, Bus: svc.Bus}
	workspace := parseUUID(fx.WorkspaceID)
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	questionIDs, err := questions.DropLinksOfIssues(ctx, svc.Queries.WithTx(tx), workspace, []pgtype.UUID{parseUUID(deleted)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "DELETE FROM issue WHERE id = $1", deleted); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	questions.PublishLinkChanges(ctx, workspace, questionIDs)
	if listenerErr != nil {
		t.Fatal(listenerErr)
	}

	rows, err := svc.Queries.ListOpenTodos(ctx, db.ListOpenTodosParams{WorkspaceID: workspace, MemberID: parseUUID(fx.UserID)})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || util.UUIDToString(rows[0].IssueID) != kept || rows[0].Title != "Kept" || util.UUIDToString(rows[0].QuestionID) != util.UUIDToString(id) {
		t.Fatalf("open question left the answerer's Inbox: %+v", rows)
	}
	if !reflect.DeepEqual(opened, []string{fx.UserID}) {
		t.Fatalf("todo opened events = %v, want one for the answerer", opened)
	}
	if fx.Count(t, "SELECT count(*) FROM todo WHERE workspace_id = $1 AND member_id = $2 AND done_at IS NULL", fx.WorkspaceID, other) != 0 ||
		fx.Count(t, "SELECT count(*) FROM todo WHERE workspace_id = $1 AND member_id = $2 AND issue_id = $3 AND done_by = 'member'", fx.WorkspaceID, other, deleted) != 1 {
		t.Fatal("a to-do marked done came back or moved")
	}
}

func TestTodoMentionLifecycle(t *testing.T) {
	fx, svc := todoListenerFixture(t)
	issue := fx.Issue(t, "Mention")
	author := fx.User(t, "Pieter", dbid.NewV7().String()+"@todo.test")
	fx.Member(t, fx.WorkspaceID, author, "member")
	count := func() int {
		return fx.Count(t, "SELECT count(*) FROM todo WHERE workspace_id = $1 AND member_id = $2 AND done_at IS NULL", fx.WorkspaceID, fx.UserID)
	}
	comment := handler.CommentResponse{ID: dbid.NewV7().String(), IssueID: issue, AuthorType: "member", AuthorID: author, Content: "[Brian](mention://member/" + fx.UserID + ")"}
	event := events.Event{Type: protocol.EventCommentCreated, WorkspaceID: fx.WorkspaceID, ActorType: "member", ActorID: author, Payload: map[string]any{"comment": comment}}
	apply := func() {
		t.Helper()
		if err := mentionTodos(context.Background(), svc, event); err != nil {
			t.Fatal(err)
		}
	}
	apply()
	if count() != 1 {
		t.Fatal("mention not opened")
	}
	fx.Exec(t, "UPDATE todo SET done_at = now(), done_by = 'member' WHERE workspace_id = $1", fx.WorkspaceID)
	comment.ID = dbid.NewV7().String()
	event.Payload = map[string]any{"comment": comment}
	apply()
	if count() != 1 {
		t.Fatal("later mention after Mark done did not open")
	}
	comment.ID, comment.AuthorID = dbid.NewV7().String(), fx.UserID
	event.ActorID, event.Payload = fx.UserID, map[string]any{"comment": comment}
	apply()
	if count() != 0 {
		t.Fatal("reply did not close mention, or own mention reopened it")
	}
	description := "[Brian](mention://member/" + fx.UserID + ")"
	event.Type, event.ActorID = protocol.EventIssueCreated, author
	event.Payload = map[string]any{"issue": handler.IssueResponse{ID: issue, Description: &description}}
	apply()
	if count() != 1 {
		t.Fatal("description mention not opened")
	}
	fx.Exec(t, "UPDATE todo SET done_at = now(), done_by = 'member' WHERE workspace_id = $1 AND done_at IS NULL", fx.WorkspaceID)
	event.Type = protocol.EventIssueUpdated
	event.Payload = map[string]any{"issue": handler.IssueResponse{ID: issue, Description: &description}, "description_changed": true, "prev_description": &description}
	apply()
	if count() != 0 {
		t.Fatal("unchanged description mention reopened dismissed todo")
	}
	previous := ""
	event.Payload.(map[string]any)["prev_description"] = &previous
	apply()
	if count() != 1 {
		t.Fatal("new description mention not opened")
	}
}

func TestTodoAssignedLifecycle(t *testing.T) {
	fx, svc := todoListenerFixture(t)
	id := fx.Issue(t, "Assignment")
	memberType := "member"
	issue := handler.IssueResponse{ID: id, Title: "Assignment", Status: "backlog", AssigneeType: &memberType, AssigneeID: &fx.UserID}
	event := events.Event{Type: protocol.EventIssueCreated, WorkspaceID: fx.WorkspaceID}
	apply := func(status string, changed bool) {
		t.Helper()
		issue.Status = status
		event.Payload = map[string]any{"issue": issue, "status_changed": changed}
		if err := assignedTodos(context.Background(), svc, event); err != nil {
			t.Fatal(err)
		}
	}
	count := func(user string) int {
		return fx.Count(t, "SELECT count(*) FROM todo WHERE issue_id = $1 AND member_id = $2 AND done_at IS NULL", id, user)
	}
	apply("backlog", false)
	if count(fx.UserID) != 0 {
		t.Fatal("backlog opened assignment")
	}
	apply("todo", false)
	if count(fx.UserID) != 1 {
		t.Fatal("todo assignment missing")
	}
	event.Type = protocol.EventIssueUpdated
	apply("in_review", true)
	if count(fx.UserID) != 1 || fx.Count(t, "SELECT count(*) FROM todo WHERE issue_id = $1 AND done_at IS NOT NULL", id) != 1 {
		t.Fatal("status inside set did not replace assignment")
	}
	apply("in_progress", true)
	if count(fx.UserID) != 0 {
		t.Fatal("status outside set did not close assignment")
	}
	apply("blocked", true)
	other := fx.User(t, "Anouk", dbid.NewV7().String()+"@todo.test")
	fx.Member(t, fx.WorkspaceID, other, "member")
	issue.AssigneeID = &other
	event.Payload = map[string]any{"issue": issue, "assignee_changed": true}
	if err := assignedTodos(context.Background(), svc, event); err != nil {
		t.Fatal(err)
	}
	if count(fx.UserID) != 0 || count(other) != 1 {
		t.Fatal("reassignment did not move todo")
	}
}

func TestTodoAgentFailures(t *testing.T) {
	fx, svc := todoListenerFixture(t)
	issue := fx.Issue(t, "Agent issue")
	runtime := fx.Runtime(t, "Todo runtime")
	agent := fx.Agent(t, "Joeri", runtime)
	task := fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "issue_id": issue, "originator_user_id": fx.UserID, "accountable_user_id": fx.UserID})
	kind := "agent"
	response := handler.IssueResponse{ID: issue, Status: "blocked", AssigneeType: &kind, AssigneeID: &agent}
	event := events.Event{Type: protocol.EventIssueUpdated, WorkspaceID: fx.WorkspaceID, ActorType: "member", ActorID: fx.UserID, Payload: map[string]any{"issue": response, "status_changed": true, "source_task_id": task}}
	apply := func() {
		t.Helper()
		if err := agentTodos(context.Background(), svc, event); err != nil {
			t.Fatal(err)
		}
	}
	count := func() int {
		return fx.Count(t, "SELECT count(*) FROM todo WHERE issue_id = $1 AND done_at IS NULL", issue)
	}
	apply()
	if count() != 0 {
		t.Fatal("member blocking agent opened agent blocked")
	}
	event.ActorType, event.ActorID = "agent", agent
	apply()
	if count() != 1 {
		t.Fatal("agent blocking in run did not open")
	}
	event.Type = protocol.EventTaskFailed
	event.Payload = map[string]any{"issue_id": issue, "task_id": task, "retry_pending": true}
	apply()
	if count() != 1 {
		t.Fatal("failed attempt with a retry pending opened run failed")
	}
	event.Payload = map[string]any{"issue_id": issue, "task_id": task, "retry_pending": false}
	apply()
	if count() != 2 {
		t.Fatal("failed run did not open separate todo")
	}
	event.Type = protocol.EventTaskDispatch
	apply()
	if count() != 0 {
		t.Fatal("run starting did not close both failure kinds")
	}
	event.Type = protocol.EventTaskFailed
	apply()
	response.Status = "in_progress"
	event.Type = protocol.EventIssueUpdated
	event.Payload = map[string]any{"issue": response, "status_changed": true}
	apply()
	if count() != 0 {
		t.Fatal("leaving blocked did not close failure")
	}
	quick := fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "context": `{"type":"quick_create"}`, "originator_user_id": fx.UserID, "accountable_user_id": fx.UserID})
	event.Type = protocol.EventTaskFailed
	event.Payload = map[string]any{"task_id": quick, "issue_id": ""}
	apply()
	if count() != 0 {
		t.Fatal("quick create opened run failed")
	}
}

func TestTodoAgentFailureAttribution(t *testing.T) {
	for _, kind := range []string{todo.RunFailed, todo.AgentBlocked} {
		for _, first := range []int{0, 1, 2, 3, 4} {
			t.Run(kind+"/"+[]string{"originator", "accountable", "source-originator", "source-accountable", "unattributed"}[first], func(t *testing.T) {
				fx, svc := todoListenerFixture(t)
				runtime := fx.Runtime(t, "Attribution runtime")
				agent := fx.Agent(t, "Joeri", runtime)
				issue := fx.Issue(t, "Delegated work")
				var members [4]any
				for index := first; index < len(members); index++ {
					user := fx.User(t, "Member", dbid.NewV7().String()+"@todo.test")
					fx.Member(t, fx.WorkspaceID, user, "member")
					members[index] = user
				}
				if members[0] != nil {
					members[1] = members[0]
				}
				if members[2] != nil {
					members[3] = members[2]
				}
				source := fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "originator_user_id": members[2], "accountable_user_id": members[3]})
				task := fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "issue_id": issue, "status": "failed", "originator_user_id": members[0], "accountable_user_id": members[1], "delegated_from_task_id": source})
				event := events.Event{Type: protocol.EventTaskFailed, WorkspaceID: fx.WorkspaceID, ActorType: "agent", ActorID: agent, Payload: map[string]any{"issue_id": issue, "task_id": task}}
				if kind == todo.AgentBlocked {
					assignee := "agent"
					event.Type = protocol.EventIssueUpdated
					event.Payload = map[string]any{"issue": handler.IssueResponse{ID: issue, Status: "blocked", AssigneeType: &assignee, AssigneeID: &agent}, "status_changed": true, "source_task_id": task}
				}
				if err := agentTodos(context.Background(), svc, event); err != nil {
					t.Fatal(err)
				}
				count := fx.Count(t, "SELECT count(*) FROM todo WHERE issue_id=$1 AND kind=$2 AND done_at IS NULL", issue, kind)
				if first == len(members) {
					if count != 0 {
						t.Fatal("unattributed work opened a to-do")
					}
					return
				}
				if count != 1 || fx.Count(t, "SELECT count(*) FROM todo WHERE issue_id=$1 AND kind=$2 AND member_id=$3 AND task_id=$4 AND done_at IS NULL", issue, kind, members[first], task) != 1 {
					t.Fatal("to-do did not reach the first attributed member")
				}
			})
		}
	}
}

// A builder moves the issue Brian created to In Review, which opens no to-do for Brian.
func TestTodoInReviewOpensNoCreatorTodo(t *testing.T) {
	fx, svc := todoListenerFixture(t)
	bus := events.New()
	registerTodoListeners(bus, svc.Queries, testPool)
	id := fx.Issue(t, "Built", testutil.Cols{"status": "in_review"})
	bus.Publish(events.Event{Type: protocol.EventIssueUpdated, WorkspaceID: fx.WorkspaceID, ActorType: "agent", ActorID: dbid.NewV7().String(), Payload: map[string]any{
		"issue":          handler.IssueResponse{ID: id, Title: "Built", Status: "in_review", CreatorType: "member", CreatorID: fx.UserID},
		"status_changed": true,
	}})
	if n := fx.Count(t, "SELECT count(*) FROM todo WHERE issue_id = $1 AND member_id = $2", id, fx.UserID); n != 0 {
		t.Fatalf("In Review opened %d to-dos for the creator", n)
	}
}

func TestTodoReviewRealtimeVisibility(t *testing.T) {
	for _, questionFirst := range []bool{false, true} {
		name := "review_before_question"
		if questionFirst {
			name = "question_before_review"
		}
		t.Run(name, func(t *testing.T) {
			fx, svc := todoListenerFixture(t)
			ctx := context.Background()
			issueID := fx.Issue(t, "Review", testutil.Cols{"status": "in_review"})
			workspace, _ := util.ParseUUID(fx.WorkspaceID)
			member, _ := util.ParseUUID(fx.UserID)
			client := map[string]map[string]any{}
			var delivered []string
			svc.Bus.Subscribe(protocol.EventTodoOpened, func(e events.Event) {
				payload := e.Payload.(map[string]any)
				if e.WorkspaceID != fx.WorkspaceID || payload["member_id"] != fx.UserID {
					t.Fatalf("wrong audience: %+v", e)
				}
				row := payload["todo"].(map[string]any)
				id := row["id"].(string)
				if _, exists := client[id]; exists {
					t.Fatalf("opened an already visible row: %s", id)
				}
				client[id] = row
				delivered = append(delivered, "opened:"+row["kind"].(string))
			})
			svc.Bus.Subscribe(protocol.EventTodoClosed, func(e events.Event) {
				payload := e.Payload.(map[string]any)
				if e.WorkspaceID != fx.WorkspaceID || payload["member_id"] != fx.UserID {
					t.Fatalf("wrong audience: %+v", e)
				}
				id := payload["todo_id"].(string)
				if _, exists := client[id]; !exists {
					t.Fatalf("closed an invisible row: %s", id)
				}
				delete(client, id)
				delivered = append(delivered, "closed:"+payload["kind"].(string))
			})
			check := func(want ...string) {
				t.Helper()
				if !reflect.DeepEqual(delivered, want) {
					t.Fatalf("events = %v, want %v", delivered, want)
				}
				delivered = nil
				rows, err := svc.Queries.ListOpenTodos(ctx, db.ListOpenTodosParams{WorkspaceID: workspace, MemberID: member})
				if err != nil {
					t.Fatal(err)
				}
				list := map[string]map[string]any{}
				for _, row := range rows {
					list[util.UUIDToString(row.ID)] = todo.Response(db.GetTodoForMemberRow(row))
				}
				if !reflect.DeepEqual(client, list) {
					t.Fatalf("realtime rows = %v, list = %v", client, list)
				}
				counts, err := svc.Queries.CountOpenTodosByWorkspace(ctx, member)
				if err != nil {
					t.Fatal(err)
				}
				var count int64
				for _, row := range counts {
					if row.WorkspaceID == workspace {
						count = row.Count
					}
				}
				if count != int64(len(client)) {
					t.Fatalf("summary = %d, realtime count = %d", count, len(client))
				}
			}
			review := func() {
				t.Helper()
				err := svc.Open(ctx, db.OpenTodoParams{
					WorkspaceID: workspace, MemberID: member, Kind: todo.Review,
					SourceKey: "decider:bug:" + issueID, IssueID: parseUUID(issueID), Title: "Plan bug: Review",
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			question := func() string {
				t.Helper()
				id := dbid.NewV7()
				fx.Insert(t, "question", testutil.Cols{"id": id, "ask_id": id, "ask_position": 0, "workspace_id": fx.WorkspaceID, "asker_agent_id": dbid.NewV7(), "asked_in_task_id": dbid.NewV7(), "prompt": "Approve?", "answer_type": "approval"})
				fx.Exec(t, "INSERT INTO question_answerer (question_id, user_id, workspace_id) VALUES ($1, $2, $3)", id, fx.UserID, fx.WorkspaceID)
				err := questionTodos(ctx, svc, events.Event{
					Type: protocol.EventQuestionCreated, WorkspaceID: fx.WorkspaceID,
					Payload: protocol.QuestionEventPayload{QuestionID: util.UUIDToString(id), IssueID: issueID},
				})
				if err != nil {
					t.Fatal(err)
				}
				return util.UUIDToString(id)
			}
			answer := func(id string) {
				t.Helper()
				fx.Exec(t, "UPDATE question SET status = 'answered' WHERE id = $1", id)
				err := questionTodos(ctx, svc, events.Event{
					Type: protocol.EventQuestionUpdated, WorkspaceID: fx.WorkspaceID, ActorID: fx.UserID,
					Payload: protocol.QuestionEventPayload{QuestionID: id, IssueID: issueID, Reason: protocol.QuestionReasonAnswered},
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			var first string
			if questionFirst {
				first = question()
				check("opened:question")
				review()
				check()
			} else {
				review()
				check("opened:review")
				first = question()
				check("closed:review", "opened:question")
			}
			review()
			check()
			second := question()
			check("opened:question")
			answer(first)
			check("closed:question")
			answer(second)
			check("closed:question", "opened:review")
			answer(second)
			check()
			question()
			check("closed:review", "opened:question")
			issue, _ := util.ParseUUID(issueID)
			if err := svc.CloseOnIssue(ctx, db.CloseTodosOnIssueParams{
				WorkspaceID: workspace, IssueID: issue, Kinds: []string{todo.Question, todo.Review},
			}); err != nil {
				t.Fatal(err)
			}
			check("closed:question")
		})
	}
}

func TestTodoConcurrentQuestionsHideAndRevealReviewOnce(t *testing.T) {
	fx, svc := todoListenerFixture(t)
	ctx := context.Background()
	issueID := fx.Issue(t, "Review", testutil.Cols{"status": "in_review"})
	workspace, _ := util.ParseUUID(fx.WorkspaceID)
	member, _ := util.ParseUUID(fx.UserID)
	issue, _ := util.ParseUUID(issueID)
	var mu sync.Mutex
	var delivered []string
	record := func(e events.Event) {
		payload := e.Payload.(map[string]any)
		kind, _ := payload["kind"].(string)
		if row, ok := payload["todo"].(map[string]any); ok {
			kind = row["kind"].(string)
		}
		mu.Lock()
		defer mu.Unlock()
		delivered = append(delivered, e.Type+":"+kind)
	}
	svc.Bus.Subscribe(protocol.EventTodoOpened, record)
	svc.Bus.Subscribe(protocol.EventTodoClosed, record)
	listed := func() []string {
		t.Helper()
		rows, err := svc.Queries.ListOpenTodos(ctx, db.ListOpenTodosParams{WorkspaceID: workspace, MemberID: member})
		if err != nil {
			t.Fatal(err)
		}
		kinds := make([]string, len(rows))
		for i, row := range rows {
			kinds[i] = row.Kind
		}
		return kinds
	}
	err := svc.Open(ctx, db.OpenTodoParams{
		WorkspaceID: workspace, MemberID: member, Kind: todo.Review,
		SourceKey: "decider:bug:" + issueID, IssueID: issue, Title: "Plan bug: Review",
	})
	if err != nil {
		t.Fatal(err)
	}
	delivered = nil
	questions := make([]string, 2)
	for i := range questions {
		id := dbid.NewV7()
		fx.Insert(t, "question", testutil.Cols{"id": id, "ask_id": id, "ask_position": 0, "workspace_id": fx.WorkspaceID, "asker_agent_id": dbid.NewV7(), "asked_in_task_id": dbid.NewV7(), "prompt": "Approve?", "answer_type": "approval"})
		fx.Exec(t, "INSERT INTO question_answerer (question_id, user_id, workspace_id) VALUES ($1, $2, $3)", id, fx.UserID, fx.WorkspaceID)
		questions[i] = util.UUIDToString(id)
	}
	// together holds the review's visibility while both question events run, so
	// both writes land before either decides whether the review shows.
	together := func(eventType, reason string) []string {
		t.Helper()
		tx, err := testPool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if err := svc.Queries.WithTx(tx).LockTodoReviewVisibility(ctx, db.LockTodoReviewVisibilityParams{WorkspaceID: workspace, MemberID: member, IssueID: issue}); err != nil {
			t.Fatal(err)
		}
		var holder int32
		if err := tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&holder); err != nil {
			t.Fatal(err)
		}
		errs := make(chan error, len(questions))
		for _, id := range questions {
			go func() {
				errs <- questionTodos(ctx, svc, events.Event{
					Type: eventType, WorkspaceID: fx.WorkspaceID, ActorID: fx.UserID,
					Payload: protocol.QuestionEventPayload{QuestionID: id, IssueID: issueID, Reason: reason},
				})
			}()
		}
		for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(10 * time.Millisecond) {
			var waiting int64
			if err := testPool.QueryRow(ctx, "SELECT count(*) FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid))", holder).Scan(&waiting); err != nil {
				t.Fatal(err)
			}
			if waiting == int64(len(questions)) {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("%d of %d question writes wait for the review", waiting, len(questions))
			}
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		for range questions {
			if err := <-errs; err != nil {
				t.Fatal(err)
			}
		}
		mu.Lock()
		defer mu.Unlock()
		got := delivered
		delivered = nil
		slices.Sort(got)
		return got
	}

	got := together(protocol.EventQuestionCreated, "")
	want := []string{protocol.EventTodoClosed + ":" + todo.Review, protocol.EventTodoOpened + ":" + todo.Question, protocol.EventTodoOpened + ":" + todo.Question}
	slices.Sort(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("opening events = %v, want %v", got, want)
	}
	if kinds := listed(); !reflect.DeepEqual(kinds, []string{todo.Question, todo.Question}) {
		t.Fatalf("list after opening = %v", kinds)
	}

	for _, id := range questions {
		fx.Exec(t, "UPDATE question SET status = 'answered' WHERE id = $1", id)
	}
	got = together(protocol.EventQuestionUpdated, protocol.QuestionReasonAnswered)
	want = []string{protocol.EventTodoClosed + ":" + todo.Question, protocol.EventTodoClosed + ":" + todo.Question, protocol.EventTodoOpened + ":" + todo.Review}
	slices.Sort(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("closing events = %v, want %v", got, want)
	}
	if kinds := listed(); !reflect.DeepEqual(kinds, []string{todo.Review}) {
		t.Fatalf("list after closing = %v", kinds)
	}
}

func TestTodoAutopilotPauseAndResume(t *testing.T) {
	q := db.New(testPool)
	ap := seedAutopilot(t, q, "Todo pause", "member", parseUUID(testUserID), pickFixtureAgent(t))
	t.Cleanup(func() { testPool.Exec(context.Background(), "DELETE FROM todo WHERE autopilot_id = $1", ap.ID) })
	bus := events.New()
	registerTodoListeners(bus, q, testPool)
	opened, closed := 0, 0
	bus.Subscribe(protocol.EventTodoOpened, func(events.Event) { opened++ })
	bus.Subscribe(protocol.EventTodoClosed, func(events.Event) { closed++ })
	if _, err := q.SystemPauseAutopilot(context.Background(), ap.ID); err != nil {
		t.Fatal(err)
	}
	openAutopilotPausedTodos(context.Background(), q, bus, ap, db.SelectAutopilotsExceedingFailureThresholdRow{FailedRuns: 10, TotalRuns: 10}, defaultFailureMonitorConfig(), 100)
	if opened != 1 {
		t.Fatalf("pause opened=%d", opened)
	}
	if _, err := testPool.Exec(context.Background(), "UPDATE autopilot SET status = 'active' WHERE id = $1", ap.ID); err != nil {
		t.Fatal(err)
	}
	bus.Publish(events.Event{Type: protocol.EventAutopilotUpdated, WorkspaceID: testWorkspaceID, Payload: map[string]any{"autopilot_id": util.UUIDToString(ap.ID)}})
	if closed != 1 {
		t.Fatalf("resume closed=%d", closed)
	}
	if _, err := q.SystemPauseAutopilot(context.Background(), ap.ID); err != nil {
		t.Fatal(err)
	}
	openAutopilotPausedTodos(context.Background(), q, bus, ap, db.SelectAutopilotsExceedingFailureThresholdRow{FailedRuns: 10, TotalRuns: 10}, defaultFailureMonitorConfig(), 100)
	if _, err := testPool.Exec(context.Background(), "DELETE FROM autopilot WHERE id = $1", ap.ID); err != nil {
		t.Fatal(err)
	}
	bus.Publish(events.Event{Type: protocol.EventAutopilotDeleted, WorkspaceID: testWorkspaceID, Payload: map[string]any{"autopilot_id": util.UUIDToString(ap.ID)}})
	if opened != 2 || closed != 2 {
		t.Fatalf("delete opened=%d closed=%d", opened, closed)
	}
}

func TestTodoQuestionNamesItsAsker(t *testing.T) {
	fx, svc := todoListenerFixture(t)
	member := fx.User(t, "Anouk", dbid.NewV7().String()+"@todo.test")
	fx.Member(t, fx.WorkspaceID, member, "member")
	agent := dbid.NewV7().String()
	askers := []struct {
		actorType, actorID string
		cols               testutil.Cols
	}{
		{"member", member, testutil.Cols{"asker_user_id": member}},
		{"agent", agent, testutil.Cols{"asker_agent_id": agent, "asked_in_task_id": dbid.NewV7()}},
	}
	for _, asker := range askers {
		id := dbid.NewV7()
		cols := testutil.Cols{"id": id, "ask_id": id, "ask_position": 0, "workspace_id": fx.WorkspaceID, "prompt": "Approve?", "answer_type": "approval"}
		for key, value := range asker.cols {
			cols[key] = value
		}
		question := fx.Insert(t, "question", cols)
		fx.Exec(t, "INSERT INTO question_answerer (question_id, user_id, workspace_id) VALUES ($1, $2, $3)", question, fx.UserID, fx.WorkspaceID)
		event := events.Event{Type: protocol.EventQuestionCreated, WorkspaceID: fx.WorkspaceID, Payload: protocol.QuestionEventPayload{QuestionID: question}}
		if err := questionTodos(context.Background(), svc, event); err != nil {
			t.Fatal(err)
		}
		if fx.Count(t, "SELECT count(*) FROM todo WHERE question_id = $1 AND member_id = $2 AND actor_type = $3 AND actor_id = $4 AND done_at IS NULL", question, fx.UserID, asker.actorType, asker.actorID) != 1 {
			t.Fatalf("%s-asked question did not name its asker", asker.actorType)
		}
	}
}

func TestTodoFormerMemberGetsNoTodo(t *testing.T) {
	fx, svc := todoListenerFixture(t)
	former := fx.User(t, "Pieter", dbid.NewV7().String()+"@todo.test")
	fx.Member(t, fx.WorkspaceID, former, "member")
	runtime := fx.Runtime(t, "Former runtime")
	agent := fx.Agent(t, "Joeri", runtime)
	issue := fx.Issue(t, "Left behind")
	task := fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "issue_id": issue, "status": "failed", "originator_user_id": former, "accountable_user_id": former})
	fx.Exec(t, "DELETE FROM member WHERE workspace_id = $1 AND user_id = $2", fx.WorkspaceID, former)
	opened := 0
	svc.Bus.Subscribe(protocol.EventTodoOpened, func(events.Event) { opened++ })
	event := events.Event{Type: protocol.EventTaskFailed, WorkspaceID: fx.WorkspaceID, ActorType: "agent", ActorID: agent, Payload: map[string]any{"issue_id": issue, "task_id": task}}
	if err := agentTodos(context.Background(), svc, event); err != nil {
		t.Fatal(err)
	}
	if fx.Count(t, "SELECT count(*) FROM todo WHERE workspace_id = $1 AND member_id = $2", fx.WorkspaceID, former) != 0 || opened != 0 {
		t.Fatalf("former member got a to-do, %d opened events", opened)
	}
}

// Handlers publish an IssueResponse and services publish IssueToMap. A system
// actor such as PR automation or a failed run's status reset opens to-dos without an actor.
func TestTodoIssueEventsFromHandlersAndServices(t *testing.T) {
	for _, shape := range []string{"handler", "service"} {
		t.Run(shape, func(t *testing.T) {
			fx, svc := todoListenerFixture(t)
			ctx := context.Background()
			assignee := fx.User(t, "Anouk", dbid.NewV7().String()+"@todo.test")
			fx.Member(t, fx.WorkspaceID, assignee, "member")
			id := fx.Issue(t, "Shapes", testutil.Cols{"status": "in_review", "assignee_type": "member", "assignee_id": assignee, "description": "[Brian](mention://member/" + fx.UserID + ")"})
			payload := func() any {
				t.Helper()
				row, err := svc.Queries.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: parseUUID(id), WorkspaceID: parseUUID(fx.WorkspaceID)})
				if err != nil {
					t.Fatal(err)
				}
				if shape == "service" {
					return service.IssueToMap(row, "")
				}
				description, assigneeType, assigneeID := row.Description.String, row.AssigneeType.String, util.UUIDToString(row.AssigneeID)
				return handler.IssueResponse{ID: id, Title: row.Title, Status: row.Status, Description: &description, AssigneeType: &assigneeType, AssigneeID: &assigneeID, CreatorType: row.CreatorType, CreatorID: util.UUIDToString(row.CreatorID)}
			}
			apply := func(event events.Event) {
				t.Helper()
				for _, listener := range []func(context.Context, todo.Service, events.Event) error{mentionTodos, assignedTodos, agentTodos} {
					if err := listener(ctx, svc, event); err != nil {
						t.Fatal(err)
					}
				}
			}
			open := func(kind, member, actorCondition string) int {
				return fx.Count(t, "SELECT count(*) FROM todo WHERE issue_id = $1 AND kind = $2 AND member_id = $3 AND done_at IS NULL AND "+actorCondition, id, kind, member)
			}
			apply(events.Event{Type: protocol.EventIssueCreated, WorkspaceID: fx.WorkspaceID, ActorType: "agent", ActorID: dbid.NewV7().String(), Payload: map[string]any{"issue": payload()}})
			if open(todo.Mention, fx.UserID, "actor_type = 'agent'") != 1 {
				t.Fatal("description mention opened no to-do")
			}
			fx.Exec(t, "DELETE FROM todo WHERE issue_id = $1 AND kind = 'assigned'", id)
			apply(events.Event{Type: protocol.EventIssueUpdated, WorkspaceID: fx.WorkspaceID, ActorType: "system", Payload: map[string]any{"issue": payload(), "status_changed": true}})
			if open(todo.Assigned, assignee, "actor_type IS NULL AND actor_id IS NULL") != 1 {
				t.Fatal("system status change did not open an assigned to-do without an actor")
			}
			workspace, member := parseUUID(fx.WorkspaceID), parseUUID(fx.UserID)
			if err := svc.Open(ctx, db.OpenTodoParams{WorkspaceID: workspace, MemberID: member, Kind: todo.AgentBlocked, SourceKey: id, IssueID: parseUUID(id), Title: "Shapes"}); err != nil {
				t.Fatal(err)
			}
			fx.Exec(t, "UPDATE issue SET status = 'in_progress' WHERE id = $1", id)
			apply(events.Event{Type: protocol.EventIssueUpdated, WorkspaceID: fx.WorkspaceID, ActorType: "system", Payload: map[string]any{"issue": payload(), "status_changed": true}})
			if fx.Count(t, "SELECT count(*) FROM todo WHERE issue_id = $1 AND kind IN ('assigned', 'agent_blocked') AND done_at IS NULL", id) != 0 {
				t.Fatal("leaving In Review kept issue to-dos open")
			}
		})
	}
}
