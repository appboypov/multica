package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/attribution"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type questionFixture struct {
	fx                                                   *testutil.Fixture
	svc                                                  *QuestionService
	workspace, issue, task, runtime, agent, brian, anouk pgtype.UUID
	asker, answerer, other                               QuestionActor
}

func newQuestionFixture(t *testing.T) *questionFixture {
	t.Helper()
	pool := sharedTestPool(t)
	bootstrap := testutil.New(pool, "", "")
	suffix := fmt.Sprint(time.Now().UnixNano())
	brian := bootstrap.User(t, "Brian", "question-brian-"+suffix+"@example.test")
	anouk := bootstrap.User(t, "Anouk", "question-anouk-"+suffix+"@example.test")
	workspace := bootstrap.Workspace(t, "Questions", "questions-"+suffix)
	fx := testutil.New(pool, workspace, brian)
	fx.Member(t, workspace, brian, "owner")
	fx.Member(t, workspace, anouk, "member")
	runtime := fx.Runtime(t, "question-runtime")
	agent := fx.Agent(t, "Pieter", runtime)
	issue := fx.Issue(t, "Invoice rules", testutil.Cols{"assignee_type": "agent", "assignee_id": agent, "status": "in_progress"})
	task := fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "issue_id": issue, "status": "running", "started_at": testutil.Raw("clock_timestamp() - interval '1 hour'")})
	// Service writes have no cascading deletes. Remove dependents before the
	// fixture-owned issue, task, agent and workspace rows are cleaned up.
	fx.Cleanup(t, `DELETE FROM agent_task_queue WHERE issue_id = $1 AND id <> $2`, issue, task)
	fx.Cleanup(t, `DELETE FROM activity_log WHERE issue_id = $1`, issue)
	fx.Cleanup(t, `DELETE FROM question WHERE workspace_id = $1`, workspace)
	fx.Cleanup(t, `DELETE FROM question_link WHERE workspace_id = $1`, workspace)
	fx.Cleanup(t, `DELETE FROM question_answerer WHERE workspace_id = $1`, workspace)
	fx.Cleanup(t, `DELETE FROM question_answer WHERE workspace_id = $1`, workspace)
	fx.Cleanup(t, `DELETE FROM question_event WHERE workspace_id = $1`, workspace)
	queries := db.New(pool)
	bus := events.New()
	f := &questionFixture{fx: fx, workspace: util.MustParseUUID(workspace), issue: util.MustParseUUID(issue), task: util.MustParseUUID(task), runtime: util.MustParseUUID(runtime), agent: util.MustParseUUID(agent), brian: util.MustParseUUID(brian), anouk: util.MustParseUUID(anouk)}
	f.asker = QuestionActor{Type: "agent", ID: f.agent, TaskID: f.task}
	f.answerer = QuestionActor{Type: "member", ID: f.brian}
	f.other = QuestionActor{Type: "member", ID: f.anouk}
	f.svc = &QuestionService{Queries: queries, TxStarter: pool, Bus: bus, Tasks: &TaskService{Queries: queries, TxStarter: pool, Bus: bus}}
	return f
}

func (f *questionFixture) ask(t *testing.T, answerers ...pgtype.UUID) QuestionDTO {
	t.Helper()
	if len(answerers) == 0 {
		answerers = []pgtype.UUID{f.brian}
	}
	row, err := f.svc.Ask(context.Background(), f.workspace, f.asker, AskQuestionInput{Prompt: "Round per line or per total?", ContextLine: "Invoice rounding", AnswerType: "single_choice", Settings: json.RawMessage(`{"options":["per line","per total"],"allow_other":false}`), SuggestedAnswer: json.RawMessage(`{"option":"per line"}`), AnswererIDs: answerers, DueDate: "2026-10-14", BlocksWork: true})
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	return row
}

func (f *questionFixture) finishAskingRun(t *testing.T) {
	t.Helper()
	f.fx.Exec(t, `UPDATE agent_task_queue SET status = 'completed', completed_at = clock_timestamp() WHERE id = $1`, f.task)
}

func (f *questionFixture) answer(t *testing.T, row QuestionDTO, actor QuestionActor, option string) QuestionDTO {
	t.Helper()
	value, err := json.Marshal(map[string]string{"option": option})
	if err != nil {
		t.Fatal(err)
	}
	answered, err := f.svc.Answer(context.Background(), f.workspace, util.MustParseUUID(row.ID), actor, AnswerQuestionInput{Value: value, Note: "Keep the old rule"})
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	return answered
}

func wantQuestionRefusal(t *testing.T, err error, status int) {
	t.Helper()
	var refusal *QuestionError
	if !errors.As(err, &refusal) || refusal.Status != status {
		t.Fatalf("error = %v, want QuestionError status %d", err, status)
	}
}

func (f *questionFixture) wantNoNewTask(t *testing.T) {
	t.Helper()
	if got := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, f.issue); got != 1 {
		t.Fatalf("question action created a task: got %d, want only asking run", got)
	}
	if got := f.fx.Count(t, `SELECT count(*) FROM activity_log WHERE issue_id = $1 AND action = 'question_wake'`, f.issue); got != 0 {
		t.Fatalf("question action created %d wake activities", got)
	}
}

func TestQuestionLifecycleNeverEnqueues(t *testing.T) {
	ctx := context.Background()
	f := newQuestionFixture(t)
	row := f.ask(t)
	id := util.MustParseUUID(row.ID)
	if row.Status != "open" || !row.BlocksWork || row.DueDate == nil || *row.DueDate != "2026-10-14" {
		t.Fatalf("ask aggregate = %+v", row)
	}
	if len(row.Links) != 2 || row.Links[0].LinkType != "issue" || row.Links[0].LinkID != util.UUIDToString(f.issue) || row.Links[1].LinkID != util.UUIDToString(f.task) {
		t.Fatalf("ask links = %+v", row.Links)
	}
	if got := f.fx.Count(t, `SELECT count(*) FROM comment WHERE issue_id = $1`, f.issue); got != 0 {
		t.Fatalf("ask created %d comments", got)
	}
	f.wantNoNewTask(t)
	_, err := f.svc.Close(ctx, f.workspace, id, f.asker)
	wantQuestionRefusal(t, err, 422)
	row, err = f.svc.AskBack(ctx, f.workspace, id, f.answerer, "Which customer?")
	if err != nil || row.Status != "open" || row.UnseenAnswerCount != 0 {
		t.Fatalf("ask back = %+v, %v", row, err)
	}
	f.wantNoNewTask(t)
	_, err = f.svc.Reassign(ctx, f.workspace, id, f.answerer, f.anouk, "  ")
	wantQuestionRefusal(t, err, 422)
	row, err = f.svc.Reassign(ctx, f.workspace, id, f.answerer, f.anouk, "You set the invoice rules")
	if err != nil || len(row.Answerers) != 1 || row.Answerers[0].UserID != util.UUIDToString(f.anouk) {
		t.Fatalf("reassign = %+v, %v", row, err)
	}
	_, err = f.svc.Answer(ctx, f.workspace, id, f.answerer, AnswerQuestionInput{Value: json.RawMessage(`{"option":"per line"}`)})
	wantQuestionRefusal(t, err, 403)
	f.wantNoNewTask(t)
	row = f.answer(t, row, f.other, "per line")
	firstID := row.Answers[0].ID
	if row.Status != "answered" || len(row.Answers) != 1 || row.Answers[0].DisplayText != "per line" {
		t.Fatalf("first answer = %+v", row)
	}
	f.wantNoNewTask(t)
	row = f.answer(t, row, f.other, "per total")
	if len(row.Answers) != 1 || row.Answers[0].ID != firstID || row.Answers[0].DisplayText != "per total" {
		t.Fatalf("edited answer = %+v", row.Answers)
	}
	f.wantNoNewTask(t)
	row, err = f.svc.AskBack(ctx, f.workspace, id, f.other, "Does that cover credits?")
	if err != nil || row.Status != "answered" {
		t.Fatalf("answered ask-back = %+v, %v", row, err)
	}
	_, err = f.svc.Reassign(ctx, f.workspace, id, f.other, f.brian, "Back to you")
	wantQuestionRefusal(t, err, 422)
	row, err = f.svc.Close(ctx, f.workspace, id, f.asker)
	if err != nil || row.Status != "closed" || row.UnseenAnswerCount != 0 || row.WakeAvailable {
		t.Fatalf("close = %+v, %v", row, err)
	}
	_, err = f.svc.Answer(ctx, f.workspace, id, f.other, AnswerQuestionInput{Value: json.RawMessage(`{"option":"per line"}`)})
	wantQuestionRefusal(t, err, 422)
	_, err = f.svc.AskBack(ctx, f.workspace, id, f.other, "More?")
	wantQuestionRefusal(t, err, 422)
	_, err = f.svc.Withdraw(ctx, f.workspace, id, f.asker)
	wantQuestionRefusal(t, err, 422)
	f.wantNoNewTask(t)
	wantKinds := []string{"asked", "ask_back", "reassigned", "answered", "answer_edited", "ask_back", "closed"}
	if len(row.Events) != len(wantKinds) {
		t.Fatalf("history = %+v", row.Events)
	}
	for i, kind := range wantKinds {
		if row.Events[i].Kind != kind {
			t.Errorf("history[%d] = %s, want %s", i, row.Events[i].Kind, kind)
		}
		if kind == "reassigned" {
			var details map[string]string
			if err := json.Unmarshal(row.Events[i].Details, &details); err != nil {
				t.Fatal(err)
			}
			if details["user_id"] != util.UUIDToString(f.anouk) || details["note"] != "You set the invoice rules" {
				t.Errorf("reassignment details = %+v", details)
			}
		}
	}
	listed, err := f.svc.ListForIssue(ctx, f.workspace, f.issue)
	if err != nil || len(listed) != 1 || listed[0].Status != "closed" {
		t.Fatalf("list after close = %+v, %v", listed, err)
	}
}

func TestQuestionAnyOneAndWithdrawal(t *testing.T) {
	ctx := context.Background()
	for _, status := range []string{"open", "answered"} {
		t.Run(status, func(t *testing.T) {
			f := newQuestionFixture(t)
			row := f.ask(t, f.brian, f.anouk)
			id := util.MustParseUUID(row.ID)
			if status == "answered" {
				row = f.answer(t, row, f.answerer, "per line")
				_, err := f.svc.Answer(ctx, f.workspace, id, f.other, AnswerQuestionInput{Value: json.RawMessage(`{"option":"per total"}`)})
				wantQuestionRefusal(t, err, 422)
			}
			row, err := f.svc.Withdraw(ctx, f.workspace, id, f.asker)
			if err != nil || row.Status != "withdrawn" || row.UnseenAnswerCount != 0 || row.WakeAvailable {
				t.Fatalf("withdraw = %+v, %v", row, err)
			}
			_, err = f.svc.Answer(ctx, f.workspace, id, f.answerer, AnswerQuestionInput{Value: json.RawMessage(`{"option":"per line"}`)})
			wantQuestionRefusal(t, err, 422)
			_, err = f.svc.Wake(ctx, f.workspace, id, f.answerer)
			wantQuestionRefusal(t, err, 422)
			f.wantNoNewTask(t)
		})
	}
}

func TestQuestionActorWorkspaceAndAskRefusals(t *testing.T) {
	ctx := context.Background()
	f := newQuestionFixture(t)
	input := AskQuestionInput{Prompt: "Rules?", AnswerType: "yes_no", Settings: json.RawMessage(`{"why_on_no":false}`), AnswererIDs: []pgtype.UUID{f.brian}}
	_, err := f.svc.Ask(ctx, f.workspace, f.answerer, input)
	wantQuestionRefusal(t, err, 403)
	input.AnswererIDs = []pgtype.UUID{f.agent}
	_, err = f.svc.Ask(ctx, f.workspace, f.asker, input)
	wantQuestionRefusal(t, err, 422)
	if !strings.Contains(err.Error(), "Pieter") {
		t.Fatalf("agent answerer error does not name agent: %v", err)
	}
	input.AnswererIDs = []pgtype.UUID{f.brian}
	badAgent := f.fx.Agent(t, "Joeri", util.UUIDToString(f.runtime))
	badTask := f.fx.Task(t, badAgent, testutil.Cols{"runtime_id": f.runtime, "issue_id": f.issue, "status": "running", "started_at": testutil.Raw("now() - interval '1 hour'")})
	_, err = f.svc.Ask(ctx, f.workspace, QuestionActor{Type: "agent", ID: f.agent, TaskID: util.MustParseUUID(badTask)}, input)
	wantQuestionRefusal(t, err, 403)
	chatTask := f.fx.Task(t, util.UUIDToString(f.agent), testutil.Cols{"runtime_id": f.runtime, "status": "running", "started_at": testutil.Raw("now() - interval '1 hour'")})
	_, err = f.svc.Ask(ctx, f.workspace, QuestionActor{Type: "agent", ID: f.agent, TaskID: util.MustParseUUID(chatTask)}, input)
	wantQuestionRefusal(t, err, 422)
	if got := f.fx.Count(t, `SELECT count(*) FROM question WHERE workspace_id = $1`, f.workspace); got != 0 {
		t.Fatalf("refused asks persisted %d questions", got)
	}
	row := f.ask(t)
	id := util.MustParseUUID(row.ID)
	_, err = f.svc.Answer(ctx, f.workspace, id, f.asker, AnswerQuestionInput{Value: json.RawMessage(`{"option":"per line"}`)})
	wantQuestionRefusal(t, err, 403)
	_, err = f.svc.Answer(ctx, f.workspace, id, f.other, AnswerQuestionInput{Value: json.RawMessage(`{"option":"per line"}`)})
	wantQuestionRefusal(t, err, 403)
	_, err = f.svc.AskBack(ctx, f.workspace, id, f.other, "Why?")
	wantQuestionRefusal(t, err, 403)
	_, err = f.svc.Close(ctx, f.workspace, id, QuestionActor{Type: "agent", ID: util.MustParseUUID(badAgent), TaskID: util.MustParseUUID(badTask)})
	wantQuestionRefusal(t, err, 403)
	_, err = f.svc.Withdraw(ctx, f.workspace, id, f.answerer)
	wantQuestionRefusal(t, err, 403)
	foreignWorkspace := f.fx.Workspace(t, "Foreign", "foreign-question-"+fmt.Sprint(time.Now().UnixNano()))
	foreign := util.MustParseUUID(foreignWorkspace)
	_, err = f.svc.Get(ctx, foreign, id)
	wantQuestionRefusal(t, err, 404)
	_, err = f.svc.ListForIssue(ctx, foreign, f.issue)
	wantQuestionRefusal(t, err, 404)
	_, err = f.svc.Answer(ctx, foreign, id, f.answerer, AnswerQuestionInput{Value: json.RawMessage(`{"option":"per line"}`)})
	wantQuestionRefusal(t, err, 403)
}

func TestQuestionUnseenTracksStartedRunsAndAnswerEdits(t *testing.T) {
	ctx := context.Background()
	f := newQuestionFixture(t)
	row := f.ask(t)
	f.finishAskingRun(t)
	row = f.answer(t, row, f.answerer, "per line")
	if row.UnseenAnswerCount != 1 || !row.WakeAvailable {
		t.Fatalf("before started run: count=%d wake=%v", row.UnseenAnswerCount, row.WakeAvailable)
	}
	// A later started run consumes the old answer, independently of its trigger.
	f.fx.Task(t, util.UUIDToString(f.agent), testutil.Cols{"runtime_id": f.runtime, "issue_id": f.issue, "status": "completed", "started_at": testutil.Raw("clock_timestamp()")})
	row, err := f.svc.Get(ctx, f.workspace, util.MustParseUUID(row.ID))
	if err != nil || row.UnseenAnswerCount != 0 || row.WakeAvailable {
		t.Fatalf("after started run = %+v, %v", row, err)
	}
	_, err = f.svc.Wake(ctx, f.workspace, util.MustParseUUID(row.ID), f.answerer)
	wantQuestionRefusal(t, err, 422)
	row = f.answer(t, row, f.answerer, "per total")
	if row.UnseenAnswerCount != 1 || !row.WakeAvailable {
		t.Fatalf("edit after start: count=%d wake=%v", row.UnseenAnswerCount, row.WakeAvailable)
	}
	answers, err := f.svc.Queries.ListUnseenQuestionAnswers(ctx, db.ListUnseenQuestionAnswersParams{WorkspaceID: f.workspace, IssueID: f.issue, AgentID: f.agent})
	if err != nil || len(answers) != 1 || !strings.Contains(string(answers[0].Value), "per total") {
		t.Fatalf("unseen answers = %+v, %v", answers, err)
	}
}

func TestQuestionWakeRefusesEveryActiveTask(t *testing.T) {
	for _, status := range []string{"queued", "dispatched", "running", "waiting_local_directory"} {
		t.Run(status, func(t *testing.T) {
			f := newQuestionFixture(t)
			row := f.ask(t)
			f.finishAskingRun(t)
			row = f.answer(t, row, f.answerer, "per line")
			f.fx.Task(t, util.UUIDToString(f.agent), testutil.Cols{"runtime_id": f.runtime, "issue_id": f.issue, "status": status})
			row, err := f.svc.Get(context.Background(), f.workspace, util.MustParseUUID(row.ID))
			if err != nil || row.WakeAvailable {
				t.Fatalf("active card = %+v, %v", row, err)
			}
			_, err = f.svc.Wake(context.Background(), f.workspace, util.MustParseUUID(row.ID), f.answerer)
			if !errors.Is(err, ErrDuplicatePendingTask) {
				t.Fatalf("wake with %s = %v, want duplicate", status, err)
			}
			if got := f.fx.Count(t, `SELECT count(*) FROM activity_log WHERE issue_id = $1 AND action = 'question_wake'`, f.issue); got != 0 {
				t.Fatalf("refused wake wrote %d activities", got)
			}
		})
	}
}

func TestQuestionWakeRefusesUnavailableAgent(t *testing.T) {
	for _, state := range []string{"archived", "no runtime"} {
		t.Run(state, func(t *testing.T) {
			f := newQuestionFixture(t)
			row := f.ask(t)
			f.finishAskingRun(t)
			f.answer(t, row, f.answerer, "per line")
			if state == "archived" {
				f.fx.Exec(t, "UPDATE agent SET archived_at=now() WHERE id=$1", f.agent)
			} else {
				f.fx.Exec(t, "UPDATE agent SET runtime_id=NULL WHERE id=$1", f.agent)
			}
			id := util.MustParseUUID(row.ID)
			row, err := f.svc.Get(context.Background(), f.workspace, id)
			if err != nil {
				t.Fatal(err)
			}
			if row.WakeAvailable || row.UnseenAnswerCount != 1 {
				t.Errorf("unavailable agent card = %+v", row)
			}
			_, err = f.svc.Wake(context.Background(), f.workspace, id, f.answerer)
			wantQuestionRefusal(t, err, 422)
			f.wantNoNewTask(t)
		})
	}
}

func TestQuestionWakeCommitsOneAttributedTaskActivityAndEvent(t *testing.T) {
	ctx := context.Background()
	f := newQuestionFixture(t)
	row := f.ask(t)
	id := util.MustParseUUID(row.ID)
	f.finishAskingRun(t)
	row = f.answer(t, row, f.answerer, "per line")
	_, err := f.svc.AskBack(ctx, f.workspace, id, f.answerer, "Also apply this to credits?")
	if err != nil {
		t.Fatal(err)
	}
	published := 0
	f.svc.Bus.Subscribe(protocol.EventQuestionUpdated, func(event events.Event) {
		payload, ok := event.Payload.(protocol.QuestionEventPayload)
		if !ok || payload.Reason != "woken" {
			return
		}
		published++
		if event.ActorID != util.UUIDToString(f.anouk) || event.ActorType != "member" || payload.IssueID != util.UUIDToString(f.issue) {
			t.Errorf("wake event = %+v", event)
		}
		// A subscriber on a separate pool connection must see every committed row.
		if n := f.fx.Count(t, `SELECT count(*) FROM activity_log WHERE issue_id = $1 AND action = 'question_wake'`, f.issue); n != 1 {
			t.Errorf("published before activity commit: %d", n)
		}
		if n := f.fx.Count(t, `SELECT count(*) FROM question_event WHERE question_id = $1 AND kind = 'woken'`, id); n != 1 {
			t.Errorf("published before question event commit: %d", n)
		}
	})
	// Any workspace member may wake, not just a current answerer.
	result, err := f.svc.Wake(ctx, f.workspace, id, f.other)
	if err != nil || result.AnswerCount != 1 {
		t.Fatalf("wake = %+v, %v", result, err)
	}
	if published != 1 {
		t.Fatalf("woken publications = %d, want 1", published)
	}
	task, err := f.svc.Queries.GetAgentTask(ctx, util.MustParseUUID(result.TaskID))
	if err != nil {
		t.Fatal(err)
	}
	if task.AgentID != f.agent || task.IssueID != f.issue || task.TriggerCommentID.Valid || task.TriggerEvidenceKind.String != string(attribution.EvidenceQuestionWake) || task.TriggerEvidenceRefID != id {
		t.Fatalf("wake task identity = %+v", task)
	}
	if task.OriginatorUserID != f.anouk || task.AccountableUserID != f.anouk || task.TriggerSummary.String != "Woken by Anouk with 1 answer" {
		t.Fatalf("wake attribution = %+v", task)
	}
	var details []byte
	var actorID string
	f.fx.QueryRow(t, `SELECT details, actor_id FROM activity_log WHERE issue_id = $1 AND action = 'question_wake'`, f.issue).Scan(&details, &actorID)
	var activity struct {
		AgentID     string `json:"agent_id"`
		AnswerCount int64  `json:"answer_count"`
		TaskID      string `json:"task_id"`
	}
	if err := json.Unmarshal(details, &activity); err != nil {
		t.Fatal(err)
	}
	if actorID != util.UUIDToString(f.anouk) || activity.AgentID != util.UUIDToString(f.agent) || activity.TaskID != result.TaskID || activity.AnswerCount != 1 {
		t.Fatalf("wake activity actor=%s details=%+v", actorID, activity)
	}
	_, err = f.svc.Wake(ctx, f.workspace, id, f.other)
	if !errors.Is(err, ErrDuplicatePendingTask) {
		t.Fatalf("second wake = %v", err)
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, f.issue); n != 2 {
		t.Fatalf("task count = %d, want asking run plus one wake", n)
	}
	row, err = f.svc.Get(ctx, f.workspace, id)
	if err != nil || row.WakeAvailable || row.Status != "answered" || row.UnseenAnswerCount != 1 {
		t.Fatalf("card after wake = %+v, %v", row, err)
	}
}

func TestQuestionConcurrentWakesAcrossCardsDeduplicate(t *testing.T) {
	ctx := context.Background()
	f := newQuestionFixture(t)
	first, second := f.ask(t), f.ask(t)
	f.finishAskingRun(t)
	f.answer(t, first, f.answerer, "per line")
	f.answer(t, second, f.answerer, "per total")
	start := make(chan struct{})
	type outcome struct {
		result QuestionWakeResult
		err    error
	}
	results := make(chan outcome, 2)
	for _, row := range []QuestionDTO{first, second} {
		go func(id string) {
			<-start
			result, err := f.svc.Wake(ctx, f.workspace, util.MustParseUUID(id), f.answerer)
			results <- outcome{result, err}
		}(row.ID)
	}
	close(start)
	wins, duplicates := 0, 0
	for range 2 {
		out := <-results
		if out.err == nil {
			wins++
			if out.result.AnswerCount != 2 {
				t.Errorf("wake count = %d, want both answers", out.result.AnswerCount)
			}
		} else if errors.Is(out.err, ErrDuplicatePendingTask) {
			duplicates++
		} else {
			t.Errorf("concurrent wake: %v", out.err)
		}
	}
	if wins != 1 || duplicates != 1 {
		t.Fatalf("concurrent wake wins=%d duplicates=%d", wins, duplicates)
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, f.issue); n != 2 {
		t.Fatalf("concurrent wake tasks = %d", n)
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM activity_log WHERE issue_id = $1 AND action = 'question_wake'`, f.issue); n != 1 {
		t.Fatalf("concurrent wake activities = %d", n)
	}
	if n := f.fx.Count(t, `SELECT count(*) FROM question_event WHERE workspace_id = $1 AND kind = 'woken'`, f.workspace); n != 1 {
		t.Fatalf("concurrent wake events = %d", n)
	}
}

// Fault injection leaves the transaction real: only the selected write fails.
type questionFailingTxStarter struct {
	TxStarter
	queryName string
}
type questionFailingTx struct {
	pgx.Tx
	queryName string
}
type questionFailedRow struct{}

func (questionFailedRow) Scan(...any) error { return errors.New("injected question write failure") }
func (s questionFailingTxStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return questionFailingTx{Tx: tx, queryName: s.queryName}, nil
}
func (tx questionFailingTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.HasPrefix(sql, "-- name: "+tx.queryName+" ") {
		return questionFailedRow{}
	}
	return tx.Tx.QueryRow(ctx, sql, args...)
}

type questionWakeRecorder struct{ calls int }

func (r *questionWakeRecorder) NotifyTaskAvailable(string, string) { r.calls++ }

func TestQuestionWakeRollsBackWithoutPublishingOrNotifying(t *testing.T) {
	for _, write := range []string{"CreateActivity", "CreateQuestionEvent"} {
		t.Run(write, func(t *testing.T) {
			f := newQuestionFixture(t)
			row := f.ask(t)
			f.finishAskingRun(t)
			f.answer(t, row, f.answerer, "per line")
			f.svc.TxStarter = questionFailingTxStarter{TxStarter: f.svc.TxStarter, queryName: write}
			published := 0
			f.svc.Bus.Subscribe(protocol.EventQuestionUpdated, func(events.Event) { published++ })
			f.svc.Bus.Subscribe(protocol.EventTaskQueued, func(events.Event) { published++ })
			recorder := &questionWakeRecorder{}
			f.svc.Tasks.Wakeup = recorder
			_, err := f.svc.Wake(context.Background(), f.workspace, util.MustParseUUID(row.ID), f.answerer)
			if err == nil {
				t.Fatal("wake should fail at injected write")
			}
			f.wantNoNewTask(t)
			if published != 0 || recorder.calls != 0 {
				t.Fatalf("failed wake published=%d notified=%d", published, recorder.calls)
			}
			if n := f.fx.Count(t, `SELECT count(*) FROM question_event WHERE question_id = $1 AND kind = 'woken'`, row.ID); n != 0 {
				t.Fatalf("failed wake wrote %d woken events", n)
			}
		})
	}
}

func TestQuestionAttachmentsAndPersonWorkspaceValidation(t *testing.T) {
	ctx := context.Background()
	f := newQuestionFixture(t)
	askType := func(answerType, settings string) QuestionDTO {
		t.Helper()
		row, err := f.svc.Ask(ctx, f.workspace, f.asker, AskQuestionInput{Prompt: "Supporting evidence?", AnswerType: answerType, Settings: json.RawMessage(settings), AnswererIDs: []pgtype.UUID{f.brian}})
		if err != nil {
			t.Fatalf("ask %s: %v", answerType, err)
		}
		return row
	}
	file := askType("file", `{"accept":[".pdf"]}`)
	text := askType("text", `{"length":"short"}`)
	insertAttachment := func(questionID string, uploader pgtype.UUID, name, mime string) string {
		return f.fx.Insert(t, "attachment", testutil.Cols{"workspace_id": f.workspace, "question_id": questionID, "uploader_type": "member", "uploader_id": uploader, "filename": name, "url": "https://example.test/" + name, "content_type": mime, "size_bytes": 10})
	}
	valid := insertAttachment(file.ID, f.brian, "invoice.pdf", "application/pdf")
	wrongOwner := insertAttachment(file.ID, f.anouk, "other.pdf", "application/pdf")
	wrongType := insertAttachment(file.ID, f.brian, "image.png", "image/png")
	wrongQuestion := insertAttachment(text.ID, f.brian, "text.pdf", "application/pdf")
	for _, id := range []string{wrongOwner, wrongType, wrongQuestion} {
		value, _ := json.Marshal(map[string]any{"attachment_ids": []string{id}})
		_, err := f.svc.Answer(ctx, f.workspace, util.MustParseUUID(file.ID), f.answerer, AnswerQuestionInput{Value: value})
		wantQuestionRefusal(t, err, 422)
	}
	value, _ := json.Marshal(map[string]any{"attachment_ids": []string{valid}})
	answered, err := f.svc.Answer(ctx, f.workspace, util.MustParseUUID(file.ID), f.answerer, AnswerQuestionInput{Value: value})
	if err != nil || answered.Status != "answered" {
		t.Fatalf("accepted file answer = %+v, %v", answered, err)
	}
	value, _ = json.Marshal(map[string]any{"text": "Attached", "attachment_ids": []string{valid}})
	_, err = f.svc.Answer(ctx, f.workspace, util.MustParseUUID(text.ID), f.answerer, AnswerQuestionInput{Value: value})
	wantQuestionRefusal(t, err, 422)
	value, _ = json.Marshal(map[string]any{"text": "Attached", "attachment_ids": []string{wrongQuestion}})
	_, err = f.svc.Answer(ctx, f.workspace, util.MustParseUUID(text.ID), f.answerer, AnswerQuestionInput{Value: value})
	if err != nil {
		t.Fatalf("text attachment: %v", err)
	}
	person := askType("person", `{"allow":"both"}`)
	foreignUser := f.fx.User(t, "Outside", "outside-question-"+fmt.Sprint(time.Now().UnixNano())+"@example.test")
	value, _ = json.Marshal(map[string]string{"type": "member", "id": foreignUser})
	_, err = f.svc.Answer(ctx, f.workspace, util.MustParseUUID(person.ID), f.answerer, AnswerQuestionInput{Value: value})
	wantQuestionRefusal(t, err, 422)
	value, _ = json.Marshal(map[string]string{"type": "agent", "id": util.UUIDToString(f.agent)})
	_, err = f.svc.Answer(ctx, f.workspace, util.MustParseUUID(person.ID), f.answerer, AnswerQuestionInput{Value: value})
	if err != nil {
		t.Fatalf("workspace agent person: %v", err)
	}
	value, _ = json.Marshal(map[string]string{"type": "member", "id": util.UUIDToString(f.anouk)})
	_, err = f.svc.Answer(ctx, f.workspace, util.MustParseUUID(person.ID), f.answerer, AnswerQuestionInput{Value: value})
	if err != nil {
		t.Fatalf("workspace member person: %v", err)
	}
	f.wantNoNewTask(t)
}

func TestQuestionAnswerDuringRunRemainsUnseenAfterRunEnds(t *testing.T) {
	ctx := context.Background()
	f := newQuestionFixture(t)
	row := f.ask(t)
	f.finishAskingRun(t)
	row = f.answer(t, row, f.answerer, "per line")
	run := f.fx.Task(t, util.UUIDToString(f.agent), testutil.Cols{
		"runtime_id": f.runtime,
		"issue_id":   f.issue,
		"status":     "running",
		"started_at": testutil.Raw("clock_timestamp()"),
	})
	row = f.answer(t, row, f.answerer, "per total")
	if row.UnseenAnswerCount != 1 || row.WakeAvailable {
		t.Fatalf("answer during active run: count=%d wake=%v", row.UnseenAnswerCount, row.WakeAvailable)
	}
	f.fx.Exec(t, `UPDATE agent_task_queue SET status = 'completed', completed_at = clock_timestamp() WHERE id = $1`, run)
	row, err := f.svc.Get(ctx, f.workspace, util.MustParseUUID(row.ID))
	if err != nil || row.UnseenAnswerCount != 1 || !row.WakeAvailable {
		t.Fatalf("answer after active run ends = %+v, %v", row, err)
	}
}

func TestQuestionAskAndAnswerFailuresRollBackAggregate(t *testing.T) {
	ctx := context.Background()
	f := newQuestionFixture(t)
	starter := f.svc.TxStarter
	f.svc.TxStarter = questionFailingTxStarter{TxStarter: starter, queryName: "CreateQuestionEvent"}
	published := 0
	f.svc.Bus.Subscribe(protocol.EventQuestionCreated, func(events.Event) { published++ })
	f.svc.Bus.Subscribe(protocol.EventQuestionUpdated, func(events.Event) { published++ })
	_, err := f.svc.Ask(ctx, f.workspace, f.asker, AskQuestionInput{
		Prompt:      "Can we ship?",
		AnswerType:  "yes_no",
		Settings:    json.RawMessage(`{"why_on_no":false}`),
		AnswererIDs: []pgtype.UUID{f.brian},
	})
	if err == nil {
		t.Fatal("ask should fail at its event write")
	}
	for _, table := range []string{"question", "question_link", "question_answerer", "question_event"} {
		if n := f.fx.Count(t, `SELECT count(*) FROM `+table+` WHERE workspace_id = $1`, f.workspace); n != 0 {
			t.Errorf("failed ask left %d rows in %s", n, table)
		}
	}
	f.svc.TxStarter = starter
	row := f.ask(t)
	published = 0
	f.svc.TxStarter = questionFailingTxStarter{TxStarter: starter, queryName: "CreateQuestionEvent"}
	_, err = f.svc.Answer(ctx, f.workspace, util.MustParseUUID(row.ID), f.answerer, AnswerQuestionInput{Value: json.RawMessage(`{"option":"per line"}`)})
	if err == nil {
		t.Fatal("answer should fail at its event write")
	}
	row, err = f.svc.Get(ctx, f.workspace, util.MustParseUUID(row.ID))
	if err != nil || row.Status != "open" || len(row.Answers) != 0 || len(row.Events) != 1 {
		t.Fatalf("failed answer did not roll back: %+v, %v", row, err)
	}
	if published != 0 {
		t.Fatalf("failed action published %d events", published)
	}
	f.wantNoNewTask(t)
}
