package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/question"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

type runInputNotifications struct {
	calls map[string]string
}

func (n *runInputNotifications) NotifyTaskSupplementAvailable(runtimeID, taskID string) {
	if n.calls == nil {
		n.calls = make(map[string]string)
	}
	n.calls[taskID] = runtimeID
}

func enableRunInputTestTask(t *testing.T, fx *testutil.Fixture, task, issue pgtype.UUID) {
	t.Helper()
	fx.InsertNoID(t, "task_supplement_capability", testutil.Cols{
		"task_id": task, "workspace_id": fx.WorkspaceID, "issue_id": issue, "capability": "task-supplement-v1",
	}, "task_id = $1", task)
	fx.Cleanup(t, "DELETE FROM task_supplement WHERE task_id = $1", task)
}

func TestRunInputDeliverSelectsOnlyEligibleRuns(t *testing.T) {
	f := newQuestionFixture(t)
	ctx := context.Background()
	f.fx.Exec(t, "UPDATE agent_runtime SET provider = 'omp' WHERE id = $1", f.runtime)
	enableRunInputTestTask(t, f.fx, f.task, f.issue)
	secondIssue := util.MustParseUUID(f.fx.Issue(t, "Second linked issue"))
	unrelatedIssue := util.MustParseUUID(f.fx.Issue(t, "Unrelated issue"))
	newTask := func(name, status string, issue pgtype.UUID, capable bool) pgtype.UUID {
		t.Helper()
		agent := f.fx.Agent(t, name, util.UUIDToString(f.runtime))
		task := util.MustParseUUID(f.fx.Task(t, agent, testutil.Cols{"runtime_id": f.runtime, "issue_id": issue, "status": status}))
		if capable {
			enableRunInputTestTask(t, f.fx, task, issue)
		}
		return task
	}
	first := newTask("Thomas", "running", f.issue, true)
	second := newTask("Guus", "running", secondIssue, true)
	newTask("Queued", "queued", f.issue, true)
	newTask("Starting", "dispatched", f.issue, true)
	newTask("Without capability", "running", f.issue, false)
	newTask("Elsewhere", "running", unrelatedIssue, true)

	notifications := &runInputNotifications{}
	svc := &RunInputService{Queries: f.svc.Queries, Notifier: notifications}
	item := RunInputItem{WorkspaceID: f.workspace, Kind: "issue_change", ID: dbid.NewV7(), Content: "Pieter changed the priority from Medium to High"}
	for range 2 {
		reached, err := svc.Deliver(ctx, []pgtype.UUID{f.issue, secondIssue, f.issue}, RunInputActor{Type: "agent", ID: f.agent}, item, RunInputChoice{})
		if err != nil {
			t.Fatal(err)
		}
		got := make(map[pgtype.UUID]bool)
		for _, task := range reached {
			got[task.ID] = true
		}
		if len(reached) != 2 || !got[first] || !got[second] {
			t.Fatalf("reached = %v, want only Thomas and Guus", reached)
		}
	}
	rows, err := f.svc.Queries.ListRunInputMarksByItems(ctx, db.ListRunInputMarksByItemsParams{WorkspaceID: f.workspace, ItemIds: []pgtype.UUID{item.ID}})
	if err != nil || len(rows) != 2 {
		t.Fatalf("receipts = %v, %v; want exactly two after repeated delivery", rows, err)
	}
	for _, row := range rows {
		if row.Status != "pending" || row.ItemKind != item.Kind || row.Content.String != item.Content || row.ClientRequestID != item.ID || row.CommentID.Valid {
			t.Fatalf("wrong receipt snapshot: %+v", row)
		}
		if notifications.calls[util.UUIDToString(row.TaskID)] != util.UUIDToString(f.runtime) {
			t.Fatalf("missing exact runtime/task notification: %v", notifications.calls)
		}
	}
	if len(notifications.calls) != 2 {
		t.Fatalf("ineligible run received a notification: %v", notifications.calls)
	}
}

func TestRunInputDeliverLimitsAChosenAgentToItsChosenRuns(t *testing.T) {
	f := newQuestionFixture(t)
	enableRunInputTestTask(t, f.fx, f.task, f.issue)
	// The author chose Pieter's run. It ends and Pieter's next run starts
	// before the comment is delivered.
	f.fx.Exec(t, "UPDATE agent_task_queue SET status = 'completed', completed_at = now() WHERE id = $1", f.task)
	replacement := util.MustParseUUID(f.fx.Task(t, util.UUIDToString(f.agent), testutil.Cols{"runtime_id": f.runtime, "issue_id": f.issue, "status": "running"}))
	enableRunInputTestTask(t, f.fx, replacement, f.issue)
	otherAgent := util.MustParseUUID(f.fx.Agent(t, "Thomas", util.UUIDToString(f.runtime)))
	other := util.MustParseUUID(f.fx.Task(t, util.UUIDToString(otherAgent), testutil.Cols{"runtime_id": f.runtime, "issue_id": f.issue, "status": "running"}))
	enableRunInputTestTask(t, f.fx, other, f.issue)

	svc := &RunInputService{Queries: f.svc.Queries}
	item := RunInputItem{WorkspaceID: f.workspace, Kind: "issue_change", ID: dbid.NewV7(), Content: "Only change the original task"}
	choice := RunInputChoice{AgentIDs: []pgtype.UUID{f.agent}, TaskIDs: []pgtype.UUID{f.task}}
	reached, err := svc.Deliver(context.Background(), []pgtype.UUID{f.issue}, RunInputActor{Type: "member", ID: f.brian}, item, choice)
	if err != nil || len(reached) != 1 || reached[0].ID != other || reached[0].AgentID != otherAgent {
		t.Fatalf("reached = %+v, %v; want only the run of the agent outside the choice", reached, err)
	}
	rows, err := f.svc.Queries.ListRunInputMarksByItems(context.Background(), db.ListRunInputMarksByItemsParams{WorkspaceID: f.workspace, ItemIds: []pgtype.UUID{item.ID}})
	if err != nil || len(rows) != 1 || rows[0].TaskID != other {
		t.Fatalf("receipts = %+v, %v; the replacing run must get none", rows, err)
	}
}

func TestRunInputDeliverPreservesCommentAndEditIdentity(t *testing.T) {
	f := newQuestionFixture(t)
	enableRunInputTestTask(t, f.fx, f.task, f.issue)
	comment := util.MustParseUUID(f.fx.Comment(t, util.UUIDToString(f.issue), "First text"))
	svc := &RunInputService{Queries: f.svc.Queries}
	items := []RunInputItem{
		{WorkspaceID: f.workspace, Kind: "comment", ID: comment, CommentID: comment, Content: "First text"},
		{WorkspaceID: f.workspace, Kind: "comment_edit", ID: dbid.NewV7(), CommentID: comment, Content: "Changed text"},
	}
	for _, item := range items {
		reached, err := svc.Deliver(context.Background(), []pgtype.UUID{f.issue}, RunInputActor{Type: "member", ID: f.brian}, item, RunInputChoice{})
		if err != nil || len(reached) != 1 || reached[0].ID != f.task {
			t.Fatalf("deliver %s: %v, %v", item.Kind, reached, err)
		}
		receipt, err := f.svc.Queries.GetTaskSupplementForRun(context.Background(), db.GetTaskSupplementForRunParams{WorkspaceID: f.workspace, TaskID: f.task, ItemID: item.ID})
		if err != nil || receipt.CommentID != comment || receipt.ItemKind != item.Kind || receipt.Content.String != item.Content {
			t.Fatalf("receipt %s = %+v, %v", item.Kind, receipt, err)
		}
	}
}

func TestQuestionAnswersReachOtherRunningAgentsOnEveryLinkedIssue(t *testing.T) {
	f := newQuestionFixture(t)
	ctx := context.Background()
	f.fx.Exec(t, "UPDATE agent SET name = 'Skuddy' WHERE id = $1", f.agent)
	f.fx.Exec(t, "UPDATE agent_runtime SET provider = 'omp' WHERE id = $1", f.runtime)
	secondIssue := util.MustParseUUID(f.fx.Issue(t, "Also linked"))
	thomas := f.fx.Agent(t, "Thomas", util.UUIDToString(f.runtime))
	tasks := make([]pgtype.UUID, 0, 2)
	for _, issue := range []pgtype.UUID{f.issue, secondIssue} {
		task := util.MustParseUUID(f.fx.Task(t, thomas, testutil.Cols{"runtime_id": f.runtime, "issue_id": issue, "status": "running"}))
		enableRunInputTestTask(t, f.fx, task, issue)
		tasks = append(tasks, task)
	}
	f.svc.RunInputs = &RunInputService{Queries: f.svc.Queries}
	asked, err := f.svc.Ask(ctx, f.workspace, f.asker, AskInput{
		Questions:   []question.AskQuestion{{Prompt: "Which toolbar?", AnswerType: "single_choice", Settings: json.RawMessage(`{"options":["header","footer"],"allow_other":false}`)}},
		AnswererIDs: []pgtype.UUID{f.brian},
		Links:       []question.LinkRef{{Type: "issue", Ref: util.UUIDToString(f.issue)}, {Type: "issue", Ref: util.UUIDToString(secondIssue)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	id := util.MustParseUUID(asked.Questions[0].ID)
	for _, tc := range []struct {
		kind, want string
		action     func() (QuestionDTO, error)
	}{
		{"answered", "Brian answered Skuddy's question", func() (QuestionDTO, error) {
			return f.svc.Answer(ctx, f.workspace, id, f.answerer, AnswerQuestionInput{Value: json.RawMessage(`{"option":"header"}`), Note: "Keep the copy button"})
		}},
		{"answer_edited", "Brian changed the answer to Skuddy's question", func() (QuestionDTO, error) {
			return f.svc.Answer(ctx, f.workspace, id, f.answerer, AnswerQuestionInput{Value: json.RawMessage(`{"option":"footer"}`)})
		}},
		{"ask_back", "Brian asks back: which copy button?", func() (QuestionDTO, error) {
			return f.svc.AskBack(ctx, f.workspace, id, f.answerer, "which copy button?")
		}},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			result, err := tc.action()
			if err != nil {
				t.Fatal(err)
			}
			var eventID pgtype.UUID
			for _, event := range result.Events {
				if event.Kind == tc.kind {
					eventID = util.MustParseUUID(event.ID)
				}
			}
			if !eventID.Valid {
				t.Fatal("question event missing")
			}
			for _, task := range tasks {
				receipt, err := f.svc.Queries.GetTaskSupplementForRun(ctx, db.GetTaskSupplementForRunParams{WorkspaceID: f.workspace, TaskID: task, ItemID: eventID})
				if err != nil || receipt.ItemKind != "answer" || !strings.Contains(receipt.Content.String, tc.want) || !strings.Contains(receipt.Content.String, "Which toolbar?") {
					t.Fatalf("Thomas receipt = %+v, %v", receipt, err)
				}
				if tc.kind == "answered" && (!strings.Contains(receipt.Content.String, "header") || !strings.Contains(receipt.Content.String, "Keep the copy button")) {
					t.Fatalf("answer lost its display value or note: %s", receipt.Content.String)
				}
				if tc.kind == "answer_edited" && !strings.Contains(receipt.Content.String, "footer") {
					t.Fatalf("edit delivered stale content: %s", receipt.Content.String)
				}
			}
		})
	}
	var count int
	if err := f.fx.Pool.QueryRow(ctx, "SELECT count(*) FROM agent_task_queue WHERE issue_id = ANY($1::uuid[])", []pgtype.UUID{f.issue, secondIssue}).Scan(&count); err != nil || count != 3 {
		t.Fatalf("answers queued tasks: count = %d, %v; want the original three", count, err)
	}
}

func TestRunInputDeliverCannotCrossWorkspace(t *testing.T) {
	local := newQuestionFixture(t)
	foreign := newQuestionFixture(t)
	enableRunInputTestTask(t, foreign.fx, foreign.task, foreign.issue)
	item := RunInputItem{WorkspaceID: local.workspace, Kind: "answer", ID: dbid.NewV7(), Content: "Workspace-local answer"}
	svc := &RunInputService{Queries: local.svc.Queries}
	reached, err := svc.Deliver(context.Background(), []pgtype.UUID{foreign.issue}, RunInputActor{Type: "member", ID: local.brian}, item, RunInputChoice{})
	if err != nil || len(reached) != 0 {
		t.Fatalf("cross-workspace delivery = %v, %v; want no recipients", reached, err)
	}
	rows, err := foreign.svc.Queries.ListRunInputMarksByItems(context.Background(), db.ListRunInputMarksByItemsParams{WorkspaceID: foreign.workspace, ItemIds: []pgtype.UUID{item.ID}})
	if err != nil || len(rows) != 0 {
		t.Fatalf("foreign receipts = %v, %v; want none", rows, err)
	}
}
