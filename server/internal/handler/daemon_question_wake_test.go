package handler

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestClaim_QuestionWakeCarriesAnswerEditedAfterWake(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	id := askQuestionForTest(t, fx)
	dbfx.Exec(t, "UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1", fx.TaskID)
	answer := func(option, note string) {
		t.Helper()
		testutil.Call(t, testHandler.AnswerQuestion, withURLParam(newRequest("POST", "/", map[string]any{
			"value": map[string]any{"option": option}, "note": note,
		}), "id", id)).Want(200)
	}
	answer("per line", "original")
	var wake service.QuestionWakeResult
	testutil.Call(t, testHandler.WakeQuestion, withURLParam(newRequest("POST", "/", nil), "id", id)).Want(202).JSON(&wake)
	answer("per total", "edited after wake")
	testutil.Call(t, testHandler.AskBackQuestion, withURLParam(newRequest("POST", "/", map[string]any{"text": "Does this also apply to credits?"}), "id", id)).Want(200)
	r := withURLParam(newDaemonTokenRequest("POST", "/api/daemon/runtimes/"+fx.RuntimeID+"/tasks/claim", nil, testWorkspaceID, "question-wake-claim"), "runtimeId", fx.RuntimeID)
	var out struct {
		Task *AgentTaskResponse `json:"task"`
	}
	testutil.Call(t, testHandler.ClaimTaskByRuntime, r).Want(200).JSON(&out)
	if out.Task == nil || out.Task.ID != wake.TaskID {
		t.Fatalf("missing woken task: %+v", out.Task)
	}
	qa := out.Task.QuestionAnswers
	if qa == nil || qa.AnswerCount != 1 || len(qa.Answers) != 1 || len(qa.AskBacks) != 1 {
		t.Fatalf("question answers not hydrated: %+v", qa)
	}
	if qa.WakerName != handlerTestName {
		t.Fatalf("waker: %q", qa.WakerName)
	}
	a := qa.Answers[0]
	if a.QuestionID != id || a.DisplayText != "per total" || a.Note != "edited after wake" || a.AnswererName != handlerTestName || a.UpdatedAt == "" {
		t.Fatalf("claim used enqueue-time answer: %+v", a)
	}
	if qa.AskBacks[0].QuestionID != id || qa.AskBacks[0].Text != "Does this also apply to credits?" || qa.AskBacks[0].ActorName != handlerTestName {
		t.Fatalf("ask-back missing: %+v", qa.AskBacks)
	}
}

func TestClaim_QuestionWakeExcludesCurrentStartedTaskFromAnchor(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	id := askQuestionForTest(t, fx)
	testutil.Call(t, testHandler.AnswerQuestion, withURLParam(newRequest("POST", "/", map[string]any{"value": map[string]any{"option": "per line"}}), "id", id)).Want(200)
	dbfx.Exec(t, "UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1", fx.TaskID)
	currentID := dbfx.Task(t, fx.AgentID, testutil.Cols{
		"runtime_id": fx.RuntimeID, "issue_id": fx.IssueID, "status": "running",
		"started_at": testutil.Raw("now()+interval '1 hour'"), "trigger_evidence_kind": service.ActivityQuestionWake,
		"dispatched_at": testutil.Raw("now()+interval '1 hour'"),
	})
	current, err := testHandler.Queries.GetAgentTask(context.Background(), parseUUID(currentID))
	if err != nil {
		t.Fatal(err)
	}
	qa, err := testHandler.loadClaimQuestionAnswers(context.Background(), current, parseUUID(testWorkspaceID), handlerTestName)
	if err != nil {
		t.Fatal(err)
	}
	if qa == nil || qa.AnswerCount != 1 || qa.Answers[0].QuestionID != id {
		t.Fatalf("current run hid unseen answer: %+v", qa)
	}
	// A different started run really does advance the anchor.
	dbfx.Task(t, fx.AgentID, testutil.Cols{"runtime_id": fx.RuntimeID, "issue_id": fx.IssueID, "status": "completed", "started_at": testutil.Raw("now()+interval '30 minutes'"), "dispatched_at": testutil.Raw("now()+interval '30 minutes'")})
	qa, err = testHandler.loadClaimQuestionAnswers(context.Background(), current, parseUUID(testWorkspaceID), handlerTestName)
	if err != nil {
		t.Fatal(err)
	}
	if qa.AnswerCount != 0 || len(qa.Answers) != 0 {
		t.Fatalf("already seen answer replayed: %+v", qa)
	}
}

func TestClaim_NonQuestionWakeCarriesNoAnswers(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	id := askQuestionForTest(t, fx)
	testutil.Call(t, testHandler.AnswerQuestion, withURLParam(newRequest("POST", "/", map[string]any{"value": map[string]any{"option": "per line"}}), "id", id)).Want(200)
	dbfx.Exec(t, "UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1", fx.TaskID)
	currentID := dbfx.Task(t, fx.AgentID, testutil.Cols{"runtime_id": fx.RuntimeID, "issue_id": fx.IssueID, "status": "queued"})
	r := withURLParam(newDaemonTokenRequest("POST", "/api/daemon/runtimes/"+fx.RuntimeID+"/tasks/claim", nil, testWorkspaceID, "non-question-claim"), "runtimeId", fx.RuntimeID)
	response := testutil.Call(t, testHandler.ClaimTaskByRuntime, r).Want(200)
	var out struct {
		Task *AgentTaskResponse `json:"task"`
	}
	response.JSON(&out)
	if out.Task == nil || out.Task.ID != currentID || out.Task.QuestionAnswers != nil {
		t.Fatalf("ordinary claim leaked question block: %+v", out.Task)
	}
	var raw struct {
		Task map[string]json.RawMessage `json:"task"`
	}
	response.JSON(&raw)
	if _, ok := raw.Task["question_answers"]; ok {
		t.Fatal("ordinary claim must omit question_answers")
	}
	qa, err := testHandler.loadClaimQuestionAnswers(context.Background(), db.AgentTaskQueue{}, parseUUID(testWorkspaceID), "")
	if err != nil || qa != nil {
		t.Fatalf("non wake hydration: %+v %v", qa, err)
	}
}

func TestClaim_QuestionWakeKeepsAnswerEditedWhileDispatched(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	id := askQuestionForTest(t, fx)
	dbfx.Exec(t, "UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1", fx.TaskID)
	answer := func(option string) {
		t.Helper()
		testutil.Call(t, testHandler.AnswerQuestion, withURLParam(newRequest("POST", "/", map[string]any{
			"value": map[string]any{"option": option},
		}), "id", id)).Want(200)
	}
	claim := func() *AgentTaskResponse {
		t.Helper()
		r := withURLParam(newDaemonTokenRequest("POST", "/", nil, testWorkspaceID, "question-dispatched-edit"), "runtimeId", fx.RuntimeID)
		var out struct {
			Task *AgentTaskResponse `json:"task"`
		}
		testutil.Call(t, testHandler.ClaimTaskByRuntime, r).Want(200).JSON(&out)
		if out.Task == nil {
			t.Fatal("missing claimed task")
		}
		return out.Task
	}
	answer("per line")
	testutil.Call(t, testHandler.WakeQuestion, withURLParam(newRequest("POST", "/", nil), "id", id)).Want(202)
	first := claim()
	if first.QuestionAnswers == nil || first.QuestionAnswers.AnswerCount != 1 || first.QuestionAnswers.Answers[0].DisplayText != "per line" {
		t.Fatalf("first claim: %+v", first.QuestionAnswers)
	}
	answer("per total")
	dbfx.Exec(t, "UPDATE agent_task_queue SET status='completed',started_at=clock_timestamp(),completed_at=clock_timestamp() WHERE id=$1", first.ID)
	var question service.QuestionDTO
	testutil.Call(t, testHandler.GetQuestion, withURLParam(newRequest("GET", "/", nil), "id", id)).Want(200).JSON(&question)
	if question.UnseenAnswerCount != 1 || !question.WakeAvailable {
		t.Fatalf("edit after claim was lost at start: %+v", question)
	}
	testutil.Call(t, testHandler.WakeQuestion, withURLParam(newRequest("POST", "/", nil), "id", id)).Want(202)
	second := claim()
	if second.QuestionAnswers == nil || second.QuestionAnswers.AnswerCount != 1 || second.QuestionAnswers.Answers[0].DisplayText != "per total" {
		t.Fatalf("next claim lost dispatched edit: %+v", second.QuestionAnswers)
	}
}

func TestClaim_QuestionWakeRetryKeepsAnswersAcrossRetryChain(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	id := askQuestionForTest(t, fx)
	dbfx.Exec(t, "UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1", fx.TaskID)
	testutil.Call(t, testHandler.AnswerQuestion, withURLParam(newRequest("POST", "/", map[string]any{
		"value": map[string]any{"option": "per total"},
	}), "id", id)).Want(200)
	testutil.Call(t, testHandler.AskBackQuestion, withURLParam(newRequest("POST", "/", map[string]any{
		"text": "Does this include credits?",
	}), "id", id)).Want(200)
	// The ask back itself wakes the asker.
	var parentID string
	dbfx.QueryRow(t, `SELECT id::text FROM agent_task_queue WHERE issue_id=$1 AND status='queued' AND trigger_evidence_kind=$2`, fx.IssueID, service.ActivityQuestionWake).Scan(&parentID)
	for attempt := 1; attempt <= 3; attempt++ {
		r := withURLParam(newDaemonTokenRequest("POST", "/", nil, testWorkspaceID, "question-retry-chain"), "runtimeId", fx.RuntimeID)
		var out struct {
			Task *AgentTaskResponse `json:"task"`
		}
		testutil.Call(t, testHandler.ClaimTaskByRuntime, r).Want(200).JSON(&out)
		if out.Task == nil || out.Task.ID != parentID {
			t.Fatalf("attempt %d claimed wrong task: %+v", attempt, out.Task)
		}
		qa := out.Task.QuestionAnswers
		if qa == nil || qa.AnswerCount != 1 || len(qa.Answers) != 1 || qa.Answers[0].DisplayText != "per total" || len(qa.AskBacks) != 1 || qa.AskBacks[0].Text != "Does this include credits?" {
			t.Fatalf("attempt %d lost wake payload: %+v", attempt, qa)
		}
		dbfx.Exec(t, "UPDATE agent_task_queue SET status='failed',started_at=clock_timestamp(),completed_at=clock_timestamp() WHERE id=$1", parentID)
		if attempt < 3 {
			parentID = dbfx.Task(t, fx.AgentID, testutil.Cols{
				"runtime_id": fx.RuntimeID, "issue_id": fx.IssueID, "status": "queued",
				"trigger_evidence_kind": service.ActivityQuestionWake, "retry_of_task_id": parentID,
				"parent_task_id": parentID, "attempt": attempt + 1,
			})
		}
	}
}
