package main

import (
	"context"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/handler"
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
	apply("back_from_qa", true)
	if count(fx.UserID) != 1 || fx.Count(t, "SELECT count(*) FROM todo WHERE issue_id = $1 AND done_at IS NOT NULL", id) != 1 {
		t.Fatal("status inside set did not replace assignment")
	}
	apply("in_progress", true)
	if count(fx.UserID) != 0 {
		t.Fatal("status outside set did not close assignment")
	}
	apply("blocked", true)
	other := fx.User(t, "Anouk", dbid.NewV7().String()+"@todo.test")
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
	event.Payload = map[string]any{"issue_id": issue, "task_id": task}
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

func TestTodoReviewLifecycle(t *testing.T) {
	fx, svc := todoListenerFixture(t)
	id := fx.Issue(t, "Review", testutil.Cols{"status": "in_review"})
	response := handler.IssueResponse{ID: id, Status: "in_review", CreatorType: "member", CreatorID: fx.UserID}
	event := events.Event{Type: protocol.EventIssueCreated, WorkspaceID: fx.WorkspaceID, Payload: map[string]any{"issue": response}}
	apply := func() {
		t.Helper()
		event.Payload.(map[string]any)["issue"] = response
		if err := reviewTodos(context.Background(), svc, event); err != nil {
			t.Fatal(err)
		}
	}
	workspace, _ := util.ParseUUID(fx.WorkspaceID)
	member, _ := util.ParseUUID(fx.UserID)
	issue, _ := util.ParseUUID(id)
	visible := func(want int) {
		t.Helper()
		rows, err := svc.Queries.ListOpenTodos(context.Background(), db.ListOpenTodosParams{WorkspaceID: workspace, MemberID: member})
		if err != nil || len(rows) != want {
			t.Fatalf("visible todos: %v, error %v, want %d", rows, err, want)
		}
		counts, err := svc.Queries.CountOpenTodosByWorkspace(context.Background(), member)
		if err != nil {
			t.Fatal(err)
		}
		total := int64(0)
		for _, count := range counts {
			if count.WorkspaceID == workspace {
				total = count.Count
			}
		}
		if total != int64(want) {
			t.Fatalf("summary=%d, want %d", total, want)
		}
	}
	apply()
	visible(1)
	qid := dbid.NewV7()
	fx.Insert(t, "question", testutil.Cols{"id": qid, "ask_id": qid, "ask_position": 0, "workspace_id": fx.WorkspaceID, "asker_agent_id": dbid.NewV7(), "asked_in_task_id": dbid.NewV7(), "prompt": "Merge?", "answer_type": "approval"})
	if err := svc.Open(context.Background(), db.OpenTodoParams{WorkspaceID: workspace, MemberID: member, Kind: todo.Question, SourceKey: util.UUIDToString(qid), IssueID: issue, QuestionID: qid, Title: "Review"}); err != nil {
		t.Fatal(err)
	}
	visible(1)
	rows, err := svc.Queries.ListOpenTodos(context.Background(), db.ListOpenTodosParams{WorkspaceID: workspace, MemberID: member})
	if err != nil || rows[0].Kind != todo.Question {
		t.Fatal("question did not hide review")
	}
	if err := svc.Close(context.Background(), db.CloseTodosParams{WorkspaceID: workspace, Kind: todo.Question, SourceKey: util.UUIDToString(qid)}); err != nil {
		t.Fatal(err)
	}
	visible(1)
	event.Type = protocol.EventIssueUpdated
	event.Payload.(map[string]any)["status_changed"] = true
	response.Status = "waiting_for_qa"
	apply()
	visible(0)
	response.Status = "in_review"
	memberType := "member"
	response.AssigneeType, response.AssigneeID = &memberType, &fx.UserID
	apply()
	visible(0)
}

func TestTodoAutopilotPauseAndResume(t *testing.T) {
	q := db.New(testPool)
	ap := seedAutopilot(t, q, "Todo pause", "member", parseUUID(testUserID), pickFixtureAgent(t))
	t.Cleanup(func() { testPool.Exec(context.Background(), "DELETE FROM todo WHERE autopilot_id = $1", ap.ID) })
	bus := events.New()
	registerTodoListeners(bus, q)
	opened, closed := 0, 0
	bus.Subscribe(protocol.EventTodoOpened, func(events.Event) { opened++ })
	bus.Subscribe(protocol.EventTodoClosed, func(events.Event) { closed++ })
	if _, err := q.SystemPauseAutopilot(context.Background(), ap.ID); err != nil {
		t.Fatal(err)
	}
	emitAutopilotPausedNotifications(context.Background(), q, bus, ap, db.SelectAutopilotsExceedingFailureThresholdRow{FailedRuns: 10, TotalRuns: 10}, defaultFailureMonitorConfig(), 100)
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
}
