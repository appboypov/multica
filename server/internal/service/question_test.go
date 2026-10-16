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
	"github.com/multica-ai/multica/server/internal/question"
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
	task := fx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "issue_id": issue, "status": "running", "started_at": testutil.Raw("clock_timestamp() - interval '1 hour'"), "questions_enabled": true})
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
	result, err := f.svc.Ask(context.Background(), f.workspace, f.asker, AskInput{Questions: []question.AskQuestion{{Prompt: "Round per line or per total?", ContextLine: "Invoice rounding", AnswerType: "single_choice", Settings: json.RawMessage(`{"options":["per line","per total"],"allow_other":false}`), SuggestedAnswer: json.RawMessage(`{"option":"per line"}`)}}, AnswererIDs: answerers, DueDate: "2026-10-14"})
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	return result.Questions[0]
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

func TestQuestionPartlyAnsweredRemainsUnseenAndOverdue(t *testing.T) {
	ctx := context.Background()
	f := newQuestionFixture(t)
	row := f.ask(t)
	id := util.MustParseUUID(row.ID)
	f.answer(t, row, f.answerer, "per line")
	if _, err := f.svc.AskBack(ctx, f.workspace, id, f.answerer, "Which currency?"); err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE question SET answer_policy = 'each', status = 'partly_answered' WHERE id = $1`, id)
	q := f.svc.Queries
	count, err := q.CountUnseenQuestionAnswers(ctx, db.CountUnseenQuestionAnswersParams{IssueID: f.issue, WorkspaceID: f.workspace, AgentID: f.agent, QuestionIds: []pgtype.UUID{id}})
	if err != nil || count != 1 {
		t.Fatalf("unseen count = %d, %v, want 1", count, err)
	}
	answers, err := q.ListUnseenQuestionAnswers(ctx, db.ListUnseenQuestionAnswersParams{IssueID: f.issue, WorkspaceID: f.workspace, AgentID: f.agent, QuestionIds: []pgtype.UUID{id}})
	if err != nil || len(answers) != 1 || answers[0].QuestionID != id {
		t.Fatalf("unseen answers = %+v, %v", answers, err)
	}
	replies, err := q.ListUnseenQuestionAskBacks(ctx, db.ListUnseenQuestionAskBacksParams{IssueID: f.issue, WorkspaceID: f.workspace, AgentID: f.agent, QuestionIds: []pgtype.UUID{id}})
	if err != nil || len(replies) != 1 || replies[0].QuestionID != id {
		t.Fatalf("unseen replies = %+v, %v", replies, err)
	}
	overdue, err := q.ListOpenOverdueQuestions(ctx, db.ListOpenOverdueQuestionsParams{
		BeforeDate: pgtype.Date{Time: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC), Valid: true}, MaxRows: 2147483647,
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, question := range overdue {
		if question.ID == id {
			found = question.AskID == id
		}
	}
	if !found {
		t.Fatal("partly answered question missing from overdue results with its ask id")
	}
	stamped, err := q.StampQuestionOverdueNotified(ctx, db.StampQuestionOverdueNotifiedParams{ID: id, WorkspaceID: f.workspace})
	if err != nil || stamped != 1 {
		t.Fatalf("overdue stamp = %d, %v, want 1", stamped, err)
	}
	stamped, err = q.StampQuestionOverdueNotified(ctx, db.StampQuestionOverdueNotifiedParams{ID: id, WorkspaceID: f.workspace})
	if err != nil || stamped != 0 {
		t.Fatalf("repeated overdue stamp = %d, %v, want 0", stamped, err)
	}
}

func TestQuestionAskReadsStayOrderedAndWorkspaceScoped(t *testing.T) {
	ctx := context.Background()
	f := newQuestionFixture(t)
	first, last, middle := f.ask(t), f.ask(t), f.ask(t)
	askID := util.MustParseUUID(first.ID)
	for position, row := range []QuestionDTO{first, middle, last} {
		f.fx.Exec(t, `UPDATE question SET ask_id = $1, ask_position = $2 WHERE id = $3`, askID, position, row.ID)
	}
	rows, err := f.svc.Queries.ListQuestionsByAsk(ctx, db.ListQuestionsByAskParams{WorkspaceID: f.workspace, AskID: askID})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || util.UUIDToString(rows[0].ID) != first.ID || util.UUIDToString(rows[1].ID) != middle.ID || util.UUIDToString(rows[2].ID) != last.ID {
		t.Fatalf("ask order = %+v, want %s, %s, %s", rows, first.ID, middle.ID, last.ID)
	}
	other := newQuestionFixture(t)
	rows, err = f.svc.Queries.ListQuestionsByAsk(ctx, db.ListQuestionsByAskParams{WorkspaceID: other.workspace, AskID: askID})
	if err != nil || len(rows) != 0 {
		t.Fatalf("cross-workspace ask = %+v, %v", rows, err)
	}
}

func TestQuestionCurrentAnswersExcludeReassignedMembers(t *testing.T) {
	ctx := context.Background()
	f := newQuestionFixture(t)
	row := f.ask(t)
	f.answer(t, row, f.answerer, "per line")
	id := util.MustParseUUID(row.ID)
	args := db.ListCurrentAnswersParams{WorkspaceID: f.workspace, QuestionID: id}
	answers, err := f.svc.Queries.ListCurrentAnswers(ctx, args)
	if err != nil || len(answers) != 1 || answers[0].AnswererUserID != f.brian {
		t.Fatalf("current answer = %+v, %v", answers, err)
	}
	other := newQuestionFixture(t)
	args.WorkspaceID = other.workspace
	answers, err = f.svc.Queries.ListCurrentAnswers(ctx, args)
	if err != nil || len(answers) != 0 {
		t.Fatalf("cross-workspace answers = %+v, %v", answers, err)
	}
	args.WorkspaceID = f.workspace
	f.fx.Exec(t, `UPDATE question_answerer SET user_id = $1 WHERE question_id = $2 AND workspace_id = $3`, f.anouk, id, f.workspace)
	answers, err = f.svc.Queries.ListCurrentAnswers(ctx, args)
	if err != nil || len(answers) != 0 {
		t.Fatalf("reassigned-away answer counted: %+v, %v", answers, err)
	}
	if got := f.fx.Count(t, `SELECT count(*) FROM question_answer WHERE question_id = $1`, id); got != 1 {
		t.Fatalf("answer history = %d, want 1", got)
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
			_, err = f.svc.Wake(ctx, f.workspace, id, f.answerer, pgtype.UUID{})
			if !errors.Is(err, ErrDuplicatePendingTask) {
				t.Fatalf("withdrawn wake must retain the active-task guard: %v", err)
			}
			f.wantNoNewTask(t)
		})
	}
}

func TestQuestionActorWorkspaceAndAskRefusals(t *testing.T) {
	ctx := context.Background()
	f := newQuestionFixture(t)
	input := AskInput{Questions: []question.AskQuestion{{Prompt: "Rules?", AnswerType: "yes_no", Settings: json.RawMessage(`{"why_on_no":false}`)}}, AnswererIDs: []pgtype.UUID{f.brian}}
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

func TestQuestionUnseenTracksClaimsAndAnswerEdits(t *testing.T) {
	ctx := context.Background()
	f := newQuestionFixture(t)
	row := f.ask(t)
	f.finishAskingRun(t)
	row = f.answer(t, row, f.answerer, "per line")
	if row.UnseenAnswerCount != 1 || !row.WakeAvailable {
		t.Fatalf("before claimed run: count=%d wake=%v", row.UnseenAnswerCount, row.WakeAvailable)
	}
	// A later claimed run consumes the old answer, independently of its trigger.
	f.fx.Task(t, util.UUIDToString(f.agent), testutil.Cols{"runtime_id": f.runtime, "issue_id": f.issue, "status": "completed", "dispatched_at": testutil.Raw("clock_timestamp()"), "started_at": testutil.Raw("clock_timestamp()")})
	row, err := f.svc.Get(ctx, f.workspace, util.MustParseUUID(row.ID))
	if err != nil || row.UnseenAnswerCount != 0 || row.WakeAvailable {
		t.Fatalf("after claimed run = %+v, %v", row, err)
	}
	_, err = f.svc.Wake(ctx, f.workspace, util.MustParseUUID(row.ID), f.answerer, pgtype.UUID{})
	wantQuestionRefusal(t, err, 422)
	row = f.answer(t, row, f.answerer, "per total")
	if row.UnseenAnswerCount != 1 || !row.WakeAvailable {
		t.Fatalf("edit after claim: count=%d wake=%v", row.UnseenAnswerCount, row.WakeAvailable)
	}
	answers, err := f.svc.Queries.ListUnseenQuestionAnswers(ctx, db.ListUnseenQuestionAnswersParams{WorkspaceID: f.workspace, IssueID: f.issue, AgentID: f.agent, QuestionIds: []pgtype.UUID{util.MustParseUUID(row.ID)}})
	if err != nil || len(answers) != 1 || !strings.Contains(string(answers[0].Value), "per total") {
		t.Fatalf("unseen answers = %+v, %v", answers, err)
	}
}

func TestQuestionUnseenSurvivesClaimThatNeverStarts(t *testing.T) {
	f := newQuestionFixture(t)
	row := f.ask(t)
	f.finishAskingRun(t)
	row = f.answer(t, row, f.answerer, "per line")
	f.fx.Task(t, util.UUIDToString(f.agent), testutil.Cols{
		"runtime_id": f.runtime, "issue_id": f.issue, "status": "failed",
		"dispatched_at": testutil.Raw("clock_timestamp()"),
	})
	row, err := f.svc.Get(context.Background(), f.workspace, util.MustParseUUID(row.ID))
	if err != nil || row.UnseenAnswerCount != 1 || !row.WakeAvailable {
		t.Fatalf("unstarted claim consumed answer: %+v, %v", row, err)
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
			_, err = f.svc.Wake(context.Background(), f.workspace, util.MustParseUUID(row.ID), f.answerer, pgtype.UUID{})
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
			_, err = f.svc.Wake(context.Background(), f.workspace, id, f.answerer, pgtype.UUID{})
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
	result, err := f.svc.Wake(ctx, f.workspace, id, f.other, pgtype.UUID{})
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
	_, err = f.svc.Wake(ctx, f.workspace, id, f.other, pgtype.UUID{})
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
			result, err := f.svc.Wake(ctx, f.workspace, util.MustParseUUID(id), f.answerer, pgtype.UUID{})
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
	failAt    int
}
type questionFailingTx struct {
	pgx.Tx
	queryName string
	failAt    int
	calls     *int
}
type questionFailedRow struct{}

func (questionFailedRow) Scan(...any) error { return errors.New("injected question write failure") }
func (s questionFailingTxStarter) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return questionFailingTx{Tx: tx, queryName: s.queryName, failAt: s.failAt, calls: new(int)}, nil
}
func (tx questionFailingTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.HasPrefix(sql, "-- name: "+tx.queryName+" ") {
		*tx.calls++
		if tx.failAt == 0 || *tx.calls == tx.failAt {
			return questionFailedRow{}
		}
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
			_, err := f.svc.Wake(context.Background(), f.workspace, util.MustParseUUID(row.ID), f.answerer, pgtype.UUID{})
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
		result, err := f.svc.Ask(ctx, f.workspace, f.asker, AskInput{Questions: []question.AskQuestion{{Prompt: "Supporting evidence?", AnswerType: answerType, Settings: json.RawMessage(settings)}}, AnswererIDs: []pgtype.UUID{f.brian}})
		if err != nil {
			t.Fatalf("ask %s: %v", answerType, err)
		}
		return result.Questions[0]
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
		"runtime_id":    f.runtime,
		"issue_id":      f.issue,
		"status":        "running",
		"dispatched_at": testutil.Raw("clock_timestamp()"),
		"started_at":    testutil.Raw("clock_timestamp()"),
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
	_, err := f.svc.Ask(ctx, f.workspace, f.asker, AskInput{
		Questions:   []question.AskQuestion{{Prompt: "Can we ship?", AnswerType: "yes_no", Settings: json.RawMessage(`{"why_on_no":false}`)}},
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

func questionSetInput(count int, answerers ...pgtype.UUID) AskInput {
	input := AskInput{AnswererIDs: answerers, Questions: make([]question.AskQuestion, count)}
	if count > 1 {
		input.Title = "Invoice decisions"
	}
	for i := range input.Questions {
		input.Questions[i] = question.AskQuestion{Prompt: fmt.Sprintf("Rounding decision %d?", i+1), AnswerType: "single_choice", Settings: json.RawMessage(`{"options":["per line","per total"],"allow_other":false}`)}
	}
	return input
}

func TestQuestionSetIsAtomicOrderedAndPublishesOnceAfterCommit(t *testing.T) {
	ctx := context.Background()
	f := newQuestionFixture(t)
	published := 0
	f.svc.Bus.Subscribe(protocol.EventQuestionCreated, func(event events.Event) {
		published++
		payload := event.Payload.(protocol.QuestionEventPayload)
		if len(payload.QuestionIDs) != 3 || payload.QuestionID != payload.QuestionIDs[0] || payload.AskID != payload.QuestionID || payload.IssueID != util.UUIDToString(f.issue) || event.ActorID != util.UUIDToString(f.agent) {
			t.Errorf("set event = %+v", event)
		}
		for _, table := range []string{"question", "question_event"} {
			if n := f.fx.Count(t, `SELECT count(*) FROM `+table+` WHERE workspace_id = $1`, f.workspace); n != 3 {
				t.Errorf("published before all %s rows committed: %d", table, n)
			}
		}
	})
	result, err := f.svc.Ask(ctx, f.workspace, f.asker, questionSetInput(3, f.brian))
	if err != nil || len(result.Questions) != 3 || result.AskID != result.Questions[0].ID {
		t.Fatalf("set ask = %+v, %v", result, err)
	}
	for i, row := range result.Questions {
		if row.AskID != result.AskID || row.AskPosition != int32(i) || row.AskTitle == nil || *row.AskTitle != "Invoice decisions" || row.AnswerPolicy != "any_one" || !row.BlocksWork || len(row.Links) != 2 || len(row.Answerers) != 1 || len(row.Events) != 1 {
			t.Fatalf("question %d aggregate = %+v", i+1, row)
		}
		var details map[string]any
		if err := json.Unmarshal(row.Events[0].Details, &details); err != nil {
			t.Fatal(err)
		}
		if details["ask_id"] != result.AskID || details["answer_policy"] != "any_one" {
			t.Fatalf("asked details = %+v", details)
		}
		if _, exists := details["squad_id"]; exists {
			t.Fatalf("unspecified squad recorded: %+v", details)
		}
	}
	if published != 1 {
		t.Fatalf("created publications = %d, want 1", published)
	}
	f.wantNoNewTask(t)
	f.answer(t, result.Questions[1], f.answerer, "per line")
	listed, err := f.svc.ListForIssue(ctx, f.workspace, f.issue)
	if err != nil || len(listed) != 3 {
		t.Fatalf("list set: %+v, %v", listed, err)
	}
	for i, row := range listed {
		wantStatus := "open"
		if i == 1 {
			wantStatus = "answered"
			if len(row.Answers) != 1 || row.Answers[0].DisplayText != "per line" {
				t.Fatalf("answer not visible immediately: %+v", row)
			}
		}
		if row.Status != wantStatus || row.AskID != result.AskID || row.AskPosition != int32(i) {
			t.Fatalf("independent question %d: %+v", i, row)
		}
	}
	f.wantNoNewTask(t)
}

func TestQuestionSetRefusedThirdWritesNothing(t *testing.T) {
	for _, failure := range []string{"settings", "suggestion", "reference"} {
		t.Run(failure, func(t *testing.T) {
			f := newQuestionFixture(t)
			input := questionSetInput(3, f.brian)
			switch failure {
			case "settings":
				input.Questions[2].Settings = json.RawMessage(`{"options":[],"allow_other":false}`)
			case "suggestion":
				input.Questions[2].SuggestedAnswer = json.RawMessage(`{"option":"missing"}`)
			case "reference":
				input.Questions[2] = question.AskQuestion{Prompt: "Who?", AnswerType: "person", Settings: json.RawMessage(`{"allow":"both"}`), SuggestedAnswer: json.RawMessage(fmt.Sprintf(`{"type":"member","id":"%s"}`, util.UUIDToString(f.agent)))}
			}
			published := 0
			f.svc.Bus.Subscribe(protocol.EventQuestionCreated, func(events.Event) { published++ })
			_, err := f.svc.Ask(context.Background(), f.workspace, f.asker, input)
			wantQuestionRefusal(t, err, 422)
			if !strings.HasPrefix(err.Error(), "question 3:") {
				t.Fatalf("refusal does not identify question 3: %v", err)
			}
			for _, table := range []string{"question", "question_link", "question_answerer", "question_event"} {
				if n := f.fx.Count(t, `SELECT count(*) FROM `+table+` WHERE workspace_id = $1`, f.workspace); n != 0 {
					t.Errorf("refused ask left %d %s rows", n, table)
				}
			}
			if published != 0 {
				t.Fatalf("refused set published %d events", published)
			}
			f.wantNoNewTask(t)
		})
	}
}

func TestQuestionAskSquadExpandsOnlyCurrentMembersAndDeduplicates(t *testing.T) {
	ctx := context.Background()
	f := newQuestionFixture(t)
	roos := f.fx.User(t, "Roos", "question-roos-"+fmt.Sprint(time.Now().UnixNano())+"@example.test")
	f.fx.Member(t, util.UUIDToString(f.workspace), roos, "member")
	squad := f.fx.Squad(t, "Finance", util.UUIDToString(f.agent))
	secondAgent := f.fx.Agent(t, "Thomas", util.UUIDToString(f.runtime))
	for i, member := range []struct{ kind, id string }{{"member", util.UUIDToString(f.brian)}, {"agent", util.UUIDToString(f.agent)}, {"member", util.UUIDToString(f.anouk)}, {"agent", secondAgent}, {"member", roos}} {
		f.fx.SquadMember(t, squad, member.kind, member.id, testutil.Cols{"created_at": time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC)})
	}
	stale := f.fx.User(t, "Former member", "question-former-"+fmt.Sprint(time.Now().UnixNano())+"@example.test")
	f.fx.SquadMember(t, squad, "member", stale)
	input := questionSetInput(3, f.anouk, f.anouk)
	input.SquadID = util.MustParseUUID(squad)
	result, err := f.svc.Ask(ctx, f.workspace, f.asker, input)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range result.Questions {
		if len(row.Answerers) != 3 {
			t.Fatalf("squad answerers = %+v", row.Answerers)
		}
		got := make(map[string]bool)
		for _, answerer := range row.Answerers {
			got[answerer.UserID] = true
		}
		for _, id := range []string{util.UUIDToString(f.brian), util.UUIDToString(f.anouk), roos} {
			if !got[id] {
				t.Fatalf("missing squad member %s: %+v", id, row.Answerers)
			}
		}
		var details map[string]any
		if err := json.Unmarshal(row.Events[0].Details, &details); err != nil || details["squad_id"] != squad {
			t.Fatalf("squad provenance = %+v, %v", details, err)
		}
	}
	// Removing a member later does not change the ask's captured audience.
	f.fx.Exec(t, `DELETE FROM squad_member WHERE squad_id = $1 AND member_id = $2`, squad, roos)
	row, err := f.svc.Get(ctx, f.workspace, util.MustParseUUID(result.Questions[0].ID))
	if err != nil || len(row.Answerers) != 3 {
		t.Fatalf("squad audience was not captured: %+v, %v", row, err)
	}
	f.wantNoNewTask(t)
}

func TestQuestionAskRefusesArchivedEmptyAndForeignSquads(t *testing.T) {
	for _, state := range []string{"archived", "agents only", "stale members", "foreign"} {
		t.Run(state, func(t *testing.T) {
			f := newQuestionFixture(t)
			squad := f.fx.Squad(t, "Finance", util.UUIDToString(f.agent))
			switch state {
			case "archived":
				f.fx.SquadMember(t, squad, "member", util.UUIDToString(f.brian))
				f.fx.Exec(t, `UPDATE squad SET archived_at = now() WHERE id = $1`, squad)
			case "agents only":
				f.fx.SquadMember(t, squad, "agent", util.UUIDToString(f.agent))
			case "stale members":
				stale := f.fx.User(t, "Former", "question-stale-"+fmt.Sprint(time.Now().UnixNano())+"@example.test")
				f.fx.SquadMember(t, squad, "member", stale)
			case "foreign":
				foreign := f.fx.Workspace(t, "Foreign squad", "foreign-squad-"+fmt.Sprint(time.Now().UnixNano()))
				f.fx.Exec(t, `UPDATE squad SET workspace_id = $1 WHERE id = $2`, foreign, squad)
			}
			input := questionSetInput(1, f.brian)
			input.SquadID = util.MustParseUUID(squad)
			_, err := f.svc.Ask(context.Background(), f.workspace, f.asker, input)
			wantQuestionRefusal(t, err, 422)
			if state == "agents only" && !strings.Contains(err.Error(), "squad Finance has no members, only agents") {
				t.Fatalf("empty squad refusal = %v", err)
			}
			if n := f.fx.Count(t, `SELECT count(*) FROM question WHERE workspace_id = $1`, f.workspace); n != 0 {
				t.Fatalf("refused squad wrote %d questions", n)
			}
			f.wantNoNewTask(t)
		})
	}
}

func TestQuestionAskBlocksWorkDefaultsAndOneAnswererPolicy(t *testing.T) {
	f := newQuestionFixture(t)
	for _, tc := range []struct {
		name   string
		blocks *bool
		want   bool
	}{
		{"omitted", nil, true}, {"off", new(bool), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := questionSetInput(1, f.brian)
			input.BlocksWork, input.Policy = tc.blocks, "each"
			result, err := f.svc.Ask(context.Background(), f.workspace, f.asker, input)
			if err != nil || len(result.Questions) != 1 {
				t.Fatalf("ask = %+v, %v", result, err)
			}
			row := result.Questions[0]
			if row.BlocksWork != tc.want || row.AnswerPolicy != "each" || row.AskTitle != nil || row.AskID != row.ID {
				t.Fatalf("single-question defaults = %+v", row)
			}
			row = f.answer(t, row, f.answerer, "per line")
			if row.Status != "answered" {
				t.Fatalf("one-answerer each = %+v", row)
			}
			f.wantNoNewTask(t)
		})
	}
}

func TestQuestionEachLifecycleRecomputesCurrentAnswers(t *testing.T) {
	ctx := context.Background()
	f := newQuestionFixture(t)
	roos := f.fx.User(t, "Roos", "question-each-"+fmt.Sprint(time.Now().UnixNano())+"@example.test")
	f.fx.Member(t, util.UUIDToString(f.workspace), roos, "member")
	input := questionSetInput(1, f.brian, f.anouk, util.MustParseUUID(roos))
	input.Policy = "each"
	result, err := f.svc.Ask(ctx, f.workspace, f.asker, input)
	if err != nil {
		t.Fatal(err)
	}
	row := result.Questions[0]
	id := util.MustParseUUID(row.ID)
	if row.Status != "open" {
		t.Fatalf("each initial = %+v", row)
	}
	row = f.answer(t, row, f.answerer, "per line")
	if row.Status != "partly_answered" {
		t.Fatalf("each first = %+v", row)
	}
	row = f.answer(t, row, f.answerer, "per total")
	if row.Status != "partly_answered" || len(row.Answers) != 1 || row.Events[len(row.Events)-1].Kind != "answer_edited" {
		t.Fatalf("partly answered edit = %+v", row)
	}
	row, err = f.svc.AskBack(ctx, f.workspace, id, f.other, "Which currency?")
	if err != nil || row.Status != "partly_answered" || len(row.Answers) != 1 {
		t.Fatalf("partly answered ask back = %+v, %v", row, err)
	}
	row, err = f.svc.Reassign(ctx, f.workspace, id, f.answerer, f.anouk, "Anouk can answer this")
	if err != nil || row.Status != "open" || len(row.Answerers) != 2 || len(row.Answers) != 1 {
		t.Fatalf("former answer must not count: %+v, %v", row, err)
	}
	_, err = f.svc.Answer(ctx, f.workspace, id, f.answerer, AnswerQuestionInput{Value: json.RawMessage(`{"option":"per line"}`)})
	wantQuestionRefusal(t, err, 403)
	row = f.answer(t, row, f.other, "per line")
	if row.Status != "partly_answered" {
		t.Fatalf("each remaining partial = %+v", row)
	}
	row = f.answer(t, row, QuestionActor{Type: "member", ID: util.MustParseUUID(roos)}, "per total")
	if row.Status != "answered" || len(row.Answers) != 3 {
		t.Fatalf("each complete with history = %+v", row)
	}
	row = f.answer(t, row, f.other, "per total")
	if row.Status != "answered" {
		t.Fatalf("each completed edit = %+v", row)
	}
	f.wantNoNewTask(t)
}

func TestQuestionPartlyAnsweredCanCloseOrWithdraw(t *testing.T) {
	for _, end := range []string{"closed", "withdrawn"} {
		t.Run(end, func(t *testing.T) {
			ctx := context.Background()
			f := newQuestionFixture(t)
			input := questionSetInput(1, f.brian, f.anouk)
			input.Policy = "each"
			result, err := f.svc.Ask(ctx, f.workspace, f.asker, input)
			if err != nil {
				t.Fatal(err)
			}
			row := f.answer(t, result.Questions[0], f.answerer, "per line")
			id := util.MustParseUUID(row.ID)
			if end == "closed" {
				row, err = f.svc.Close(ctx, f.workspace, id, f.asker)
			} else {
				row, err = f.svc.Withdraw(ctx, f.workspace, id, f.asker)
			}
			if err != nil || row.Status != end {
				t.Fatalf("end partly answered = %+v, %v", row, err)
			}
			_, err = f.svc.Answer(ctx, f.workspace, id, f.other, AnswerQuestionInput{Value: json.RawMessage(`{"option":"per total"}`)})
			wantQuestionRefusal(t, err, 422)
			_, err = f.svc.Answer(ctx, f.workspace, id, f.answerer, AnswerQuestionInput{Value: json.RawMessage(`{"option":"per total"}`)})
			wantQuestionRefusal(t, err, 422)
			_, err = f.svc.Reassign(ctx, f.workspace, id, f.other, f.brian, "Reopen?")
			wantQuestionRefusal(t, err, 422)
			_, err = f.svc.AskBack(ctx, f.workspace, id, f.other, "More?")
			wantQuestionRefusal(t, err, 422)
			f.wantNoNewTask(t)
		})
	}
}

func TestQuestionSetWakeFromAnyStatusKeepsStatusAndGetsAllUnseenAnswers(t *testing.T) {
	for _, status := range []string{"open", "partly_answered", "answered", "closed", "withdrawn"} {
		t.Run(status, func(t *testing.T) {
			ctx := context.Background()
			f := newQuestionFixture(t)
			input := questionSetInput(3, f.brian, f.anouk)
			input.Policy = "each"
			result, err := f.svc.Ask(ctx, f.workspace, f.asker, input)
			if err != nil {
				t.Fatal(err)
			}
			f.finishAskingRun(t)
			f.answer(t, result.Questions[0], f.answerer, "per line")
			f.answer(t, result.Questions[1], f.answerer, "per total")
			row := result.Questions[2]
			id := util.MustParseUUID(row.ID)
			if status != "open" {
				row = f.answer(t, row, f.answerer, "per line")
			}
			switch status {
			case "answered":
				row = f.answer(t, row, f.other, "per total")
			case "closed":
				row, err = f.svc.Close(ctx, f.workspace, id, f.asker)
			case "withdrawn":
				row, err = f.svc.Withdraw(ctx, f.workspace, id, f.asker)
			}
			if err != nil {
				t.Fatal(err)
			}
			wantCount := int64(2)
			switch status {
			case "partly_answered":
				wantCount = 3
			case "answered":
				wantCount = 4
			}
			row, err = f.svc.Get(ctx, f.workspace, id)
			if err != nil || row.Status != status || !row.WakeAvailable || row.UnseenAnswerCount != wantCount {
				t.Fatalf("wake must be offered from %s: %+v, %v", status, row, err)
			}
			if len(row.WakeTargets) != 1 || row.WakeTargets[0].AgentID != util.UUIDToString(f.agent) || row.WakeTargets[0].IssueID != util.UUIDToString(f.issue) || row.WakeTargets[0].UnseenAnswerCount != wantCount || !row.WakeTargets[0].WakeAvailable {
				t.Fatalf("wake targets from %s = %+v", status, row.WakeTargets)
			}
			f.wantNoNewTask(t)
			wake, err := f.svc.Wake(ctx, f.workspace, id, f.other, pgtype.UUID{})
			if err != nil || wake.AnswerCount != row.UnseenAnswerCount {
				t.Fatalf("wake from %s = %+v, %v", status, wake, err)
			}
			task, err := f.svc.Queries.GetAgentTask(ctx, util.MustParseUUID(wake.TaskID))
			if err != nil || task.IssueID != f.issue || task.AgentID != f.agent || task.TriggerEvidenceRefID != id {
				t.Fatalf("wake task = %+v, %v", task, err)
			}
			scope, err := f.svc.WakeScope(ctx, f.workspace, f.agent, f.issue)
			if err != nil {
				t.Fatal(err)
			}
			unseen, err := f.svc.Queries.ListUnseenQuestionAnswers(ctx, db.ListUnseenQuestionAnswersParams{WorkspaceID: f.workspace, IssueID: f.issue, AgentID: f.agent, QuestionIds: scope})
			if err != nil || int64(len(unseen)) != wake.AnswerCount {
				t.Fatalf("queued run must receive all unseen answers: %+v, %v", unseen, err)
			}
			if status == "closed" || status == "withdrawn" {
				for _, answer := range unseen {
					if answer.QuestionID == id {
						t.Fatalf("terminal question's own answer must not be delivered: %+v", answer)
					}
				}
			}
			row, err = f.svc.Get(ctx, f.workspace, id)
			if err != nil || row.Status != status || row.WakeAvailable {
				t.Fatalf("wake must not change question status: %+v, %v", row, err)
			}
			_, err = f.svc.Wake(ctx, f.workspace, id, f.answerer, pgtype.UUID{})
			if !errors.Is(err, ErrDuplicatePendingTask) {
				t.Fatalf("wake active guard = %v", err)
			}
			_, err = f.svc.Wake(ctx, f.workspace, util.MustParseUUID(result.Questions[0].ID), f.answerer, pgtype.UUID{})
			if !errors.Is(err, ErrDuplicatePendingTask) {
				t.Fatalf("wake active guard across cards = %v", err)
			}
		})
	}
}

func TestQuestionTerminalWakeKeepsBusyAndUnavailableGuards(t *testing.T) {
	for _, status := range []string{"closed", "withdrawn"} {
		for _, state := range []string{"queued", "dispatched", "running", "waiting_local_directory", "archived", "no runtime"} {
			t.Run(status+"/"+state, func(t *testing.T) {
				ctx := context.Background()
				f := newQuestionFixture(t)
				other := f.ask(t)
				row := f.ask(t)
				f.finishAskingRun(t)
				f.answer(t, other, f.answerer, "per line")
				row = f.answer(t, row, f.answerer, "per total")
				id := util.MustParseUUID(row.ID)
				var err error
				if status == "closed" {
					_, err = f.svc.Close(ctx, f.workspace, id, f.asker)
				} else {
					_, err = f.svc.Withdraw(ctx, f.workspace, id, f.asker)
				}
				if err != nil {
					t.Fatal(err)
				}
				switch state {
				case "archived":
					f.fx.Exec(t, `UPDATE agent SET archived_at = now() WHERE id = $1`, f.agent)
				case "no runtime":
					f.fx.Exec(t, `UPDATE agent SET runtime_id = NULL WHERE id = $1`, f.agent)
				default:
					f.fx.Task(t, util.UUIDToString(f.agent), testutil.Cols{"runtime_id": f.runtime, "issue_id": f.issue, "status": state})
				}
				row, err = f.svc.Get(ctx, f.workspace, id)
				if err != nil || row.Status != status || row.WakeAvailable || row.UnseenAnswerCount != 1 {
					t.Fatalf("terminal card with %s = %+v, %v", state, row, err)
				}
				for _, target := range row.WakeTargets {
					if target.WakeAvailable {
						t.Fatalf("terminal card offers unavailable target: %+v", target)
					}
				}
				before := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, f.issue)
				_, err = f.svc.Wake(ctx, f.workspace, id, f.answerer, pgtype.UUID{})
				if state == "archived" || state == "no runtime" {
					wantQuestionRefusal(t, err, 422)
				} else if !errors.Is(err, ErrDuplicatePendingTask) {
					t.Fatalf("terminal wake with %s = %v, want duplicate", state, err)
				}
				if got := f.fx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, f.issue); got != before {
					t.Fatalf("refused terminal wake created a task: got %d, want %d", got, before)
				}
				if got := f.fx.Count(t, `SELECT count(*) FROM activity_log WHERE issue_id = $1 AND action = 'question_wake'`, f.issue); got != 0 {
					t.Fatalf("refused terminal wake wrote %d activities", got)
				}
			})
		}
	}
}

func TestQuestionSetThirdWriteFailureRollsBackEarlierQuestions(t *testing.T) {
	for _, write := range []string{"CreateQuestion", "CreateQuestionEvent"} {
		t.Run(write, func(t *testing.T) {
			f := newQuestionFixture(t)
			f.svc.TxStarter = questionFailingTxStarter{TxStarter: f.svc.TxStarter, queryName: write, failAt: 3}
			published := 0
			f.svc.Bus.Subscribe(protocol.EventQuestionCreated, func(events.Event) { published++ })
			_, err := f.svc.Ask(context.Background(), f.workspace, f.asker, questionSetInput(3, f.brian))
			if err == nil {
				t.Fatal("ask should fail at the third question write")
			}
			for _, table := range []string{"question", "question_link", "question_answerer", "question_event"} {
				if n := f.fx.Count(t, `SELECT count(*) FROM `+table+` WHERE workspace_id = $1`, f.workspace); n != 0 {
					t.Errorf("failed set left %d %s rows", n, table)
				}
			}
			if published != 0 {
				t.Fatalf("failed set published %d events", published)
			}
			f.wantNoNewTask(t)
		})
	}
}

func TestQuestionAnyOneRefusesAnotherAnswerButAcceptsTheirAskBack(t *testing.T) {
	ctx := context.Background()
	f := newQuestionFixture(t)
	row := f.answer(t, f.ask(t, f.brian, f.anouk), f.answerer, "per line")
	id := util.MustParseUUID(row.ID)
	_, err := f.svc.Answer(ctx, f.workspace, id, f.other, AnswerQuestionInput{Value: json.RawMessage(`{"option":"per total"}`)})
	wantQuestionRefusal(t, err, 422)
	row, err = f.svc.AskBack(ctx, f.workspace, id, f.other, "Does this apply to credits?")
	if err != nil || row.Status != "answered" || len(row.Answers) != 1 || row.Events[len(row.Events)-1].Kind != "ask_back" {
		t.Fatalf("second answerer's ask back = %+v, %v", row, err)
	}
	f.wantNoNewTask(t)
}

func TestQuestionEachReassignmentCanCompleteTheCurrentAudience(t *testing.T) {
	ctx := context.Background()
	f := newQuestionFixture(t)
	input := questionSetInput(1, f.brian, f.anouk)
	input.Policy = "each"
	result, err := f.svc.Ask(ctx, f.workspace, f.asker, input)
	if err != nil {
		t.Fatal(err)
	}
	row := f.answer(t, result.Questions[0], f.answerer, "per line")
	row, err = f.svc.Reassign(ctx, f.workspace, util.MustParseUUID(row.ID), f.other, f.brian, "Brian already has this covered")
	if err != nil || row.Status != "answered" || len(row.Answerers) != 1 || len(row.Answers) != 1 {
		t.Fatalf("recomputed completed audience = %+v, %v", row, err)
	}
	f.wantNoNewTask(t)
}

func TestQuestionOpenWakeStillRequiresUnseenAnswers(t *testing.T) {
	ctx := context.Background()
	f := newQuestionFixture(t)
	row := f.ask(t)
	f.finishAskingRun(t)
	_, err := f.svc.Wake(ctx, f.workspace, util.MustParseUUID(row.ID), f.answerer, pgtype.UUID{})
	wantQuestionRefusal(t, err, 422)
	row, err = f.svc.Get(ctx, f.workspace, util.MustParseUUID(row.ID))
	if err != nil || row.Status != "open" || row.UnseenAnswerCount != 0 || row.WakeAvailable {
		t.Fatalf("open question without unseen answers = %+v, %v", row, err)
	}
	f.wantNoNewTask(t)
}
