package handler

import (
	"context"
	"encoding/json"
	"net/http"
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

func claimQuestionTaskForTest(t *testing.T, fx questionAPIFixture) *AgentTaskResponse {
	t.Helper()
	r := withURLParam(newDaemonTokenRequest("POST", "/", nil, testWorkspaceID, "question-wake-lifecycle"), "runtimeId", fx.RuntimeID)
	var out struct {
		Task *AgentTaskResponse `json:"task"`
	}
	testutil.Call(t, testHandler.ClaimTaskByRuntime, r).Want(http.StatusOK).JSON(&out)
	if out.Task == nil {
		t.Fatal("no task available for question wake lifecycle")
	}
	return out.Task
}

func startQuestionTaskForTest(t *testing.T, fx questionAPIFixture, task *AgentTaskResponse) {
	t.Helper()
	if task.DispatchedAt == nil {
		t.Fatal("claimed task has no dispatch generation")
	}
	r := withURLParam(newDaemonTokenRequest("POST", "/", map[string]any{
		"runtime_id": fx.RuntimeID, "dispatched_at": *task.DispatchedAt,
	}, testWorkspaceID, "question-wake-lifecycle"), "taskId", task.ID)
	testutil.Call(t, testHandler.StartTask, r).Want(http.StatusOK)
}

func TestQuestionWake_WaitsForTerminalAPIAndClaimsLatestAnswer(t *testing.T) {
	for _, entry := range []string{"answer_with_wake", "manual_wake"} {
		for _, terminal := range []string{"complete", "fail"} {
			t.Run(entry+"/"+terminal, func(t *testing.T) {
				fx := newQuestionAPIFixture(t)
				id := askQuestionForTest(t, fx)
				dbfx.Exec(t, "UPDATE agent_task_queue SET max_attempts=1 WHERE id=$1", fx.TaskID)
				testutil.Call(t, testHandler.AnswerQuestion, withURLParam(newRequest("POST", "/", map[string]any{
					"value": map[string]any{"option": "per line"}, "wake": entry == "answer_with_wake",
				}), "id", id)).Want(http.StatusOK)
				if entry == "manual_wake" {
					var waiting service.QuestionWakeResult
					testutil.Call(t, testHandler.WakeQuestion, withURLParam(newRequest("POST", "/", nil), "id", id)).Want(http.StatusAccepted).JSON(&waiting)
					if waiting.TaskID != "" || waiting.AnswerCount != 1 {
						t.Fatalf("active-run wake was not accepted as waiting: %+v", waiting)
					}
				}
				if n := dbfx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE agent_id=$1 AND issue_id=$2", fx.AgentID, fx.IssueID); n != 1 {
					t.Fatalf("wake queued work before the running task ended: %d", n)
				}
				if n := dbfx.Count(t, "SELECT count(*) FROM question_wake_pending WHERE agent_id=$1 AND issue_id=$2", fx.AgentID, fx.IssueID); n != 1 {
					t.Fatalf("wake was not persisted while run was ending: %d", n)
				}
				if n := dbfx.Count(t, "SELECT count(*) FROM question_event WHERE question_id=$1 AND kind='woken'", id); n != 0 {
					t.Fatalf("waiting wake emitted %d premature woken events", n)
				}
				if n := dbfx.Count(t, "SELECT count(*) FROM activity_log WHERE issue_id=$1 AND action=$2", fx.IssueID, service.ActivityQuestionWake); n != 0 {
					t.Fatalf("waiting wake emitted %d premature timeline activities", n)
				}
				testutil.Call(t, testHandler.AnswerQuestion, withURLParam(newRequest("POST", "/", map[string]any{
					"value": map[string]any{"option": "per total"}, "note": "latest actual answer",
				}), "id", id)).Want(http.StatusOK)
				if terminal == "complete" {
					r := completeTaskViaHandler(t, fx.TaskID, "Finished before reading the answer")
					if r.Code != http.StatusOK {
						t.Fatalf("complete ending run: %d %s", r.Code, r.Body.String())
					}
				} else {
					r := failTaskViaHandler(t, fx.TaskID)
					if r.Code != http.StatusOK {
						t.Fatalf("fail ending run: %d %s", r.Code, r.Body.String())
					}
				}
				if n := queuedTaskCountForAgentIssue(t, fx.IssueID, fx.AgentID); n != 1 {
					t.Fatalf("terminal API queued %d followups, want one", n)
				}
				if n := dbfx.Count(t, "SELECT count(*) FROM question_wake_pending WHERE agent_id=$1 AND issue_id=$2", fx.AgentID, fx.IssueID); n != 0 {
					t.Fatalf("terminal API did not consume accepted intent: %d", n)
				}
				task := claimQuestionTaskForTest(t, fx)
				qa := task.QuestionAnswers
				if task.ID == fx.TaskID || qa == nil || qa.AnswerCount != 1 || len(qa.Answers) != 1 {
					t.Fatalf("next claim missing unseen input: %+v", task)
				}
				if qa.WakerName != handlerTestName || qa.Answers[0].QuestionID != id || qa.Answers[0].DisplayText != "per total" || qa.Answers[0].Note != "latest actual answer" {
					t.Fatalf("next claim did not hydrate current answer and waker: %+v", qa)
				}
				if n := dbfx.Count(t, "SELECT count(*) FROM question_event WHERE question_id=$1 AND kind='woken' AND actor_id=$2", id, testUserID); n != 1 {
					t.Fatalf("actual wake missing first-waker attribution: %d events", n)
				}
				if n := dbfx.Count(t, "SELECT count(*) FROM activity_log WHERE issue_id=$1 AND action=$2 AND actor_id=$3 AND details->>'answer_count'='1'", fx.IssueID, service.ActivityQuestionWake, testUserID); n != 1 {
					t.Fatalf("actual wake missing attributed one-answer activity: %d", n)
				}
				startQuestionTaskForTest(t, fx, task)
				r := completeTaskViaHandler(t, task.ID, "Processed answer")
				if r.Code != http.StatusOK {
					t.Fatalf("complete wake: %d %s", r.Code, r.Body.String())
				}
				if n := pendingTaskCountForAgentIssue(t, fx.IssueID, fx.AgentID); n != 0 {
					t.Fatalf("consumed wake queued another task: %d", n)
				}
			})
		}
	}
}

func TestQuestionWake_QueuedClaimConsumesWaitAndPostClaimInputFollowsUp(t *testing.T) {
	for _, postClaimInput := range []bool{false, true} {
		name := "claim_consumes_all_input"
		if postClaimInput {
			name = "post_claim_input_starts_followup"
		}
		t.Run(name, func(t *testing.T) {
			fx := newQuestionAPIFixture(t)
			id := askQuestionForTest(t, fx)
			dbfx.Exec(t, "UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1", fx.TaskID)
			answer := func(option, note string) {
				t.Helper()
				testutil.Call(t, testHandler.AnswerQuestion, withURLParam(newRequest("POST", "/", map[string]any{
					"value": map[string]any{"option": option}, "note": note, "wake": true,
				}), "id", id)).Want(http.StatusOK)
			}
			answer("per line", "before enqueue")
			answer("per total", "before claim")
			if n := queuedTaskCountForAgentIssue(t, fx.IssueID, fx.AgentID); n != 1 {
				t.Fatalf("input before claim created %d tasks", n)
			}
			if n := dbfx.Count(t, "SELECT count(*) FROM question_wake_pending WHERE agent_id=$1 AND issue_id=$2", fx.AgentID, fx.IssueID); n != 1 {
				t.Fatalf("wake behind queued task missing intent: %d", n)
			}
			first := claimQuestionTaskForTest(t, fx)
			qa := first.QuestionAnswers
			if qa == nil || qa.AnswerCount != 1 || len(qa.Answers) != 1 || qa.Answers[0].DisplayText != "per total" || qa.Answers[0].Note != "before claim" {
				t.Fatalf("already queued wake did not receive waiting input: %+v", qa)
			}
			startQuestionTaskForTest(t, fx, first)
			if postClaimInput {
				answer("per line", "after claim")
			}
			r := completeTaskViaHandler(t, first.ID, "Finished queued wake")
			if r.Code != http.StatusOK {
				t.Fatalf("complete queued wake: %d %s", r.Code, r.Body.String())
			}
			if n := dbfx.Count(t, "SELECT count(*) FROM question_wake_pending WHERE agent_id=$1 AND issue_id=$2", fx.AgentID, fx.IssueID); n != 0 {
				t.Fatalf("terminal API did not settle waiting intent: %d", n)
			}
			if !postClaimInput {
				if n := pendingTaskCountForAgentIssue(t, fx.IssueID, fx.AgentID); n != 0 {
					t.Fatalf("claim consumed all input but left %d redundant followups", n)
				}
				if n := dbfx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE agent_id=$1 AND issue_id=$2", fx.AgentID, fx.IssueID); n != 2 {
					t.Fatalf("claim consumed all input but created %d total tasks", n)
				}
				return
			}
			if n := queuedTaskCountForAgentIssue(t, fx.IssueID, fx.AgentID); n != 1 {
				t.Fatalf("post-claim input queued %d followups", n)
			}
			next := claimQuestionTaskForTest(t, fx)
			qa = next.QuestionAnswers
			if next.ID == first.ID || qa == nil || qa.AnswerCount != 1 || len(qa.Answers) != 1 || qa.Answers[0].DisplayText != "per line" || qa.Answers[0].Note != "after claim" {
				t.Fatalf("post-claim answer not carried into followup: %+v", next)
			}
		})
	}
}

func claimChatQuestionTask(t *testing.T, fx questionAPIFixture) *AgentTaskResponse {
	t.Helper()
	r := withURLParam(newDaemonTokenRequest("POST", "/", nil, testWorkspaceID, "chat-question-claim"), "runtimeId", fx.RuntimeID)
	var out struct {
		Task *AgentTaskResponse `json:"task"`
	}
	testutil.Call(t, testHandler.ClaimTaskByRuntime, r).Want(200).JSON(&out)
	if out.Task == nil {
		t.Fatal("missing chat task")
	}
	return out.Task
}

func TestClaim_ChatQuestionsFollowWorkspaceDefault(t *testing.T) {
	var original []byte
	dbfx.QueryRow(t, "SELECT settings FROM workspace WHERE id=$1", testWorkspaceID).Scan(&original)
	t.Cleanup(func() {
		dbfx.Exec(t, "UPDATE workspace SET settings=$2 WHERE id=$1", testWorkspaceID, original)
	})
	for _, tc := range []struct {
		name    string
		setting string
		channel bool
		want    bool
	}{
		{"absent is on", `{}`, false, true},
		{"on", `{"agent_questions":true}`, false, true},
		{"off", `{"agent_questions":false}`, false, false},
		{"channel stays off", `{"agent_questions":true}`, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dbfx.Exec(t, "UPDATE workspace SET settings=$2::jsonb WHERE id=$1", testWorkspaceID, tc.setting)
			fx, sessionID := newChatQuestionAPIFixture(t)
			dbfx.Exec(t, "UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1", fx.TaskID)
			cols := testutil.Cols{"runtime_id": fx.RuntimeID, "chat_session_id": sessionID, "status": "queued"}
			message := testutil.Cols{"chat_session_id": sessionID, "role": "user", "content": "Continue"}
			if tc.channel {
				cols["channel_context_revision"] = int64(1)
				message["channel_context_revision"] = int64(1)
			}
			taskID := dbfx.Task(t, fx.AgentID, cols)
			dbfx.Exec(t, "UPDATE agent_task_queue SET chat_input_task_id=id WHERE id=$1", taskID)
			message["task_id"] = taskID
			dbfx.Insert(t, "chat_message", message)
			task := claimChatQuestionTask(t, fx)
			if task.ID != taskID || task.QuestionsEnabled != tc.want {
				t.Fatalf("claim questions_enabled=%v task=%s, want %v task=%s", task.QuestionsEnabled, task.ID, tc.want, taskID)
			}
			var stored bool
			dbfx.QueryRow(t, "SELECT questions_enabled FROM agent_task_queue WHERE id=$1", taskID).Scan(&stored)
			if stored != tc.want {
				t.Fatalf("persisted questions_enabled=%v, want %v", stored, tc.want)
			}
		})
	}
}

func TestClaim_ChatQuestionWakeCarriesAnswersCancelAndRetryChain(t *testing.T) {
	fx, sessionID := newChatQuestionAPIFixture(t)
	body := questionAskBody(testUserID)
	body["title"] = "Invoice decisions"
	body["questions"] = []map[string]any{
		{"prompt": "First?", "answer_type": "text", "settings": map[string]any{"length": "short"}},
		{"prompt": "Second?", "answer_type": "text", "settings": map[string]any{"length": "short"}},
		{"prompt": "Third?", "answer_type": "text", "settings": map[string]any{"length": "short"}},
	}
	var ask service.QuestionAskResult
	testutil.Call(t, testHandler.AskQuestion, fx.agentRequest("POST", "", body)).Want(201).JSON(&ask)
	dbfx.Exec(t, "UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1", fx.TaskID)
	for i, value := range []string{"per line", "EUR"} {
		testutil.Call(t, testHandler.AnswerQuestion, withURLParam(newRequest("POST", "/", map[string]any{
			"value": map[string]any{"text": value},
		}), "id", ask.Questions[i].ID)).Want(200)
	}
	testutil.Call(t, testHandler.CancelQuestion, withURLParam(newRequest("POST", "/", map[string]any{
		"note": "already decided in the PRD", "wake": true,
	}), "id", ask.Questions[2].ID)).Want(200)
	if n := dbfx.Count(t, "SELECT count(*) FROM chat_message WHERE chat_session_id=$1", sessionID); n != 0 {
		t.Fatalf("wake manufactured %d chat messages", n)
	}
	var expectedID string
	dbfx.QueryRow(t, "SELECT id::text FROM agent_task_queue WHERE chat_session_id=$1 AND status='queued'", sessionID).Scan(&expectedID)
	// An unrelated issue run must not advance the chat cutoff.
	dbfx.Task(t, fx.AgentID, testutil.Cols{
		"runtime_id": fx.RuntimeID, "issue_id": fx.IssueID, "status": "completed",
		"started_at": testutil.Raw("now()+interval '1 hour'"), "dispatched_at": testutil.Raw("now()+interval '1 hour'"),
	})
	for attempt := 1; attempt <= 3; attempt++ {
		task := claimChatQuestionTask(t, fx)
		if task.ID != expectedID || task.ChatSessionID != sessionID || task.ChatMessage != "" {
			t.Fatalf("attempt %d wrong chat wake: %+v", attempt, task)
		}
		qa := task.QuestionAnswers
		if qa == nil || qa.AnswerCount != 3 || len(qa.Answers) != 2 || len(qa.Cancels) != 1 || len(qa.AskBacks) != 0 {
			t.Fatalf("attempt %d payload: %+v", attempt, qa)
		}
		if qa.Answers[0].DisplayText != "per line" || qa.Answers[1].DisplayText != "EUR" {
			t.Fatalf("answers: %+v", qa.Answers)
		}
		cancel := qa.Cancels[0]
		if cancel.QuestionID != ask.Questions[2].ID || cancel.Note != "already decided in the PRD" || cancel.ActorName != handlerTestName || cancel.CreatedAt == "" || cancel.AskerAgentID != fx.AgentID {
			t.Fatalf("cancel: %+v", cancel)
		}
		dbfx.Exec(t, "UPDATE agent_task_queue SET status='failed',started_at=clock_timestamp(),completed_at=clock_timestamp() WHERE id=$1", task.ID)
		if attempt < 3 {
			expectedID = dbfx.Task(t, fx.AgentID, testutil.Cols{
				"runtime_id": fx.RuntimeID, "chat_session_id": sessionID, "status": "queued",
				"trigger_evidence_kind": service.ActivityQuestionWake, "retry_of_task_id": task.ID,
				"parent_task_id": task.ID, "attempt": attempt + 1,
			})
			dbfx.Exec(t, "UPDATE agent_task_queue SET chat_input_task_id=id WHERE id=$1", expectedID)
		}
	}
}

func TestClaim_ChatMessageCarriesUnseenAnswerAndLeavesQuestionsOpen(t *testing.T) {
	fx, sessionID := newChatQuestionAPIFixture(t)
	id := askQuestionForTest(t, fx)
	openID := askQuestionForTest(t, fx)
	dbfx.Exec(t, "UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1", fx.TaskID)
	testutil.Call(t, testHandler.AnswerQuestion, withURLParam(newRequest("POST", "/", map[string]any{
		"value": map[string]any{"option": "per line"},
	}), "id", id)).Want(200)
	req := withChatTestWorkspaceCtx(t, withURLParam(newRequest("POST", "/", map[string]any{"content": "Continue with these answers"}), "sessionId", sessionID))
	var sent SendChatMessageResponse
	testutil.Call(t, testHandler.SendChatMessage, req).Want(201).JSON(&sent)
	task := claimChatQuestionTask(t, fx)
	if task.ID != sent.TaskID || task.ChatMessage != "Continue with these answers" || task.QuestionAnswers == nil || len(task.QuestionAnswers.Answers) != 1 || task.QuestionAnswers.Answers[0].QuestionID != id {
		t.Fatalf("message turn lost unseen answer: %+v", task)
	}
	var status string
	dbfx.QueryRow(t, "SELECT status FROM question WHERE id=$1", openID).Scan(&status)
	if status != "open" {
		t.Fatalf("message ended open question: %s", status)
	}
	dbfx.Exec(t, "UPDATE agent_task_queue SET status='completed',started_at=clock_timestamp(),completed_at=clock_timestamp() WHERE id=$1", task.ID)
	var question service.QuestionDTO
	testutil.Call(t, testHandler.GetQuestion, withURLParam(newRequest("GET", "/", nil), "id", id)).Want(200).JSON(&question)
	if question.WakeAvailable || question.UnseenAnswerCount != 0 {
		t.Fatalf("started message turn did not consume answer: %+v", question)
	}
}
