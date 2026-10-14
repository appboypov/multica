package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/question"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

type questionAPIFixture struct{ AgentID, IssueID, TaskID, RuntimeID string }

func newQuestionAPIFixture(t *testing.T) questionAPIFixture {
	t.Helper()
	runtime := dbfx.Runtime(t, t.Name())
	agent := dbfx.Agent(t, t.Name()+"-"+runtime, runtime)
	issue := dbfx.Issue(t, t.Name())
	task := dbfx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "issue_id": issue, "status": "running", "started_at": testutil.Raw("now()-interval '1 hour'"), "questions_enabled": true})
	// Task tokens and activity rows are run-owned, without database cascades.
	dbfx.Cleanup(t, "DELETE FROM task_token WHERE task_id IN (SELECT id FROM agent_task_queue WHERE agent_id=$1)", agent)
	dbfx.Cleanup(t, "DELETE FROM agent_task_queue WHERE agent_id=$1", agent)
	dbfx.Cleanup(t, "DELETE FROM activity_log WHERE issue_id=$1", issue)
	for _, table := range []string{"question", "question_link", "question_answerer", "question_answer", "question_event"} {
		if table == "question" {
			dbfx.Cleanup(t, "DELETE FROM question WHERE asker_agent_id=$1", agent)
		} else {
			dbfx.Cleanup(t, "DELETE FROM "+table+" WHERE question_id IN (SELECT id FROM question WHERE asker_agent_id=$1)", agent)
		}
	}
	return questionAPIFixture{agent, issue, task, runtime}
}

func (fx questionAPIFixture) agentRequest(method, id string, body any) *http.Request {
	r := withURLParam(newRequest(method, "/api/questions/"+id, body), "id", id)
	r.Header.Set("X-Actor-Source", "task_token")
	r.Header.Set("X-Agent-ID", fx.AgentID)
	r.Header.Set("X-Task-ID", fx.TaskID)
	return r
}

func questionAskBody(answerers ...string) map[string]any {
	return map[string]any{
		"questions": []map[string]any{{
			"prompt": "Round per line or per total?", "answer_type": "single_choice",
			"settings": map[string]any{"options": []string{"per line", "per total"}, "allow_other": false},
		}},
		"answerer_ids": answerers,
	}
}

func askQuestionForTest(t *testing.T, fx questionAPIFixture) string {
	t.Helper()
	var out service.QuestionAskResult
	testutil.Call(t, testHandler.AskQuestion, fx.agentRequest("POST", "", questionAskBody(testUserID))).Want(201).JSON(&out)
	if len(out.Questions) != 1 || out.Questions[0].ID == "" || out.AskID != out.Questions[0].ID {
		t.Fatalf("missing one-question ask result: %+v", out)
	}
	return out.Questions[0].ID
}

func TestQuestionAPI_MemberCannotAsk(t *testing.T) {
	testutil.Call(t, testHandler.AskQuestion, newRequest("POST", "/api/questions", questionAskBody(testUserID))).Want(403)
}

func TestQuestionAPI_AgentAnswererRejectedByName(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	joeri := dbfx.Agent(t, "Joeri", fx.RuntimeID)
	out := testutil.Call(t, testHandler.AskQuestion, fx.agentRequest("POST", "", questionAskBody(joeri))).Want(422).Text()
	if !strings.Contains(out, "Joeri") || !strings.Contains(out, "agent") {
		t.Fatalf("answerer error lost agent name: %s", out)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM question WHERE asker_agent_id=$1", fx.AgentID); n != 0 {
		t.Fatalf("refused ask created %d questions", n)
	}
}

func TestQuestionAPI_AnswerAuthorizationAndValidation(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	id := askQuestionForTest(t, fx)
	body := map[string]any{"value": map[string]any{"option": "per line"}}
	t.Run("agent cannot answer", func(t *testing.T) {
		testutil.Call(t, testHandler.AnswerQuestion, fx.agentRequest("POST", id, body)).Want(403)
	})
	t.Run("non answerer cannot answer", func(t *testing.T) {
		outsider := dbfx.User(t, "Other member", "question-other@multica.test")
		dbfx.Member(t, testWorkspaceID, outsider, "member")
		r := withURLParam(newRequest("POST", "/", body), "id", id)
		r.Header.Set("X-User-ID", outsider)
		testutil.Call(t, testHandler.AnswerQuestion, r).Want(403)
	})
	t.Run("invalid value retains validator message", func(t *testing.T) {
		value := json.RawMessage(`{"option":"per week"}`)
		settings := json.RawMessage(`{"options":["per line","per total"],"allow_other":false}`)
		validationErr := question.ValidateAnswer("single_choice", settings, value, "")
		if validationErr == nil {
			t.Fatal("invalid value was accepted by validator")
		}
		out := testutil.Call(t, testHandler.AnswerQuestion, withURLParam(newRequest("POST", "/", map[string]any{"value": value}), "id", id)).Want(422).Map()
		if out["error"] != validationErr.Error() {
			t.Fatalf("validator message changed at HTTP boundary: %+v", out)
		}
	})
}

func TestQuestionAPI_CloseOnlyAskingAgentAndClosedAnswerCannotEdit(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	id := askQuestionForTest(t, fx)
	body := map[string]any{"value": map[string]any{"option": "per line"}}
	testutil.Call(t, testHandler.AnswerQuestion, withURLParam(newRequest("POST", "/", body), "id", id)).Want(200)
	other := newQuestionAPIFixture(t)
	testutil.Call(t, testHandler.CloseQuestion, other.agentRequest("POST", id, nil)).Want(403)
	closed := testutil.Call(t, testHandler.CloseQuestion, fx.agentRequest("POST", id, nil)).Want(200).Map()
	if closed["status"] != "closed" {
		t.Fatalf("question not closed: %+v", closed)
	}
	response := testutil.Call(t, testHandler.AnswerQuestion, withURLParam(newRequest("POST", "/", body), "id", id))
	if response.Code != 422 {
		t.Fatalf("closed answer edit: %d %s", response.Code, response.Text())
	}
}

func TestQuestionAPI_MemberWakeAcceptedThenConflict(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	id := askQuestionForTest(t, fx)
	dbfx.Exec(t, "UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1", fx.TaskID)
	testutil.Call(t, testHandler.AnswerQuestion, withURLParam(newRequest("POST", "/", map[string]any{"value": map[string]any{"option": "per line"}}), "id", id)).Want(200)
	var wake service.QuestionWakeResult
	testutil.Call(t, testHandler.WakeQuestion, withURLParam(newRequest("POST", "/", nil), "id", id)).Want(202).JSON(&wake)
	if wake.TaskID == "" || wake.AnswerCount != 1 {
		t.Fatalf("wake response: %+v", wake)
	}
	testutil.Call(t, testHandler.WakeQuestion, withURLParam(newRequest("POST", "/", nil), "id", id)).Want(409)
}

func TestQuestionAPI_WorkspaceScopeAndUUIDInput(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	id := askQuestionForTest(t, fx)
	otherWS := dbfx.Workspace(t, "question other workspace", "question-other-workspace")
	dbfx.Member(t, otherWS, testUserID, "owner")
	r := withURLParam(newRequest("GET", "/", nil), "id", id)
	r.Header.Set("X-Workspace-ID", otherWS)
	testutil.Call(t, testHandler.GetQuestion, r).Want(404)
	for _, action := range []http.HandlerFunc{testHandler.GetQuestion, testHandler.AnswerQuestion, testHandler.WakeQuestion} {
		testutil.Call(t, action, withURLParam(newRequest("POST", "/", nil), "id", "not-a-uuid")).Want(400)
	}
	testutil.Call(t, testHandler.ReassignQuestion, withURLParam(newRequest("POST", "/", map[string]any{"user_id": "bad", "note": "please answer"}), "id", id)).Want(400)
	list := testutil.Call(t, testHandler.ListIssueQuestions, withURLParam(newRequest("GET", "/", nil), "id", fx.IssueID)).Want(200)
	var questions []map[string]any
	list.JSON(&questions)
	if len(questions) != 1 || questions[0]["id"] != id {
		t.Fatalf("issue question list: %+v", questions)
	}
	for _, field := range []string{"ask_id", "ask_position", "ask_title", "answer_policy", "links", "answerers", "answers", "events", "unseen_answer_count", "wake_available"} {
		if _, ok := questions[0][field]; !ok {
			t.Fatalf("missing question contract field %s", field)
		}
	}
}

func TestQuestionUpload_StoresQuestionOwnerAndRejectsOtherWorkspace(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	id := askQuestionForTest(t, fx)
	h := *testHandler
	h.Storage = &mockStorage{}
	dbfx.Cleanup(t, "DELETE FROM attachment WHERE question_id=$1", id)
	request := func(workspace, questionID string) *http.Request {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		file, err := writer.CreateFormFile("file", "invoice.txt")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte("invoice rules")); err != nil {
			t.Fatal(err)
		}
		if err := writer.WriteField("question_id", questionID); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("POST", "/api/upload-file", &body)
		r.Header.Set("Content-Type", writer.FormDataContentType())
		r.Header.Set("X-User-ID", testUserID)
		r.Header.Set("X-Workspace-ID", workspace)
		return r
	}
	out := testutil.Call(t, h.UploadFile, request(testWorkspaceID, id)).Want(200).Map()
	attachmentID, ok := out["id"].(string)
	if !ok {
		t.Fatalf("upload response: %+v", out)
	}
	var owner, workspace, uploader string
	dbfx.QueryRow(t, "SELECT question_id::text,workspace_id::text,uploader_id::text FROM attachment WHERE id=$1", attachmentID).Scan(&owner, &workspace, &uploader)
	if owner != id || workspace != testWorkspaceID || uploader != testUserID {
		t.Fatalf("wrong question attachment ownership: %s %s %s", owner, workspace, uploader)
	}
	otherWS := dbfx.Workspace(t, "question upload elsewhere", "question-upload-elsewhere")
	dbfx.Member(t, otherWS, testUserID, "owner")
	testutil.Call(t, h.UploadFile, request(otherWS, id)).Want(403)
	testutil.Call(t, h.UploadFile, request(testWorkspaceID, "invalid")).Want(400)
	testutil.Call(t, h.UploadFile, request("", id)).Want(400)
	if n := dbfx.Count(t, "SELECT count(*) FROM attachment WHERE question_id=$1", id); n != 1 {
		t.Fatalf("refused upload created attachment: %d", n)
	}
}

func questionSetBody(count int, answerers ...string) map[string]any {
	body := questionAskBody(answerers...)
	questions := make([]map[string]any, count)
	for i := range questions {
		questions[i] = map[string]any{
			"prompt": fmt.Sprintf("Question %d", i+1), "answer_type": "yes_no",
			"settings": map[string]any{"why_on_no": false},
		}
	}
	body["questions"] = questions
	if count > 1 {
		body["title"] = "Three things before the CSV writer"
	}
	return body
}

func TestQuestionAPI_ThreeQuestionAskReturnsOrderedSet(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	var out service.QuestionAskResult
	testutil.Call(t, testHandler.AskQuestion, fx.agentRequest("POST", "", questionSetBody(3, testUserID))).Want(201).JSON(&out)
	if len(out.Questions) != 3 || out.AskID == "" || out.AskID != out.Questions[0].ID {
		t.Fatalf("wrong grouped ask: %+v", out)
	}
	for i, q := range out.Questions {
		if q.AskID != out.AskID || q.AskPosition != int32(i) || q.Prompt != fmt.Sprintf("Question %d", i+1) ||
			q.AskTitle == nil || *q.AskTitle != "Three things before the CSV writer" || q.AnswerPolicy != "any_one" || !q.BlocksWork {
			t.Fatalf("question %d lost grouped fields or order: %+v", i+1, q)
		}
	}
}

func TestQuestionAPI_AskRefusalsWriteNothing(t *testing.T) {
	for _, tc := range []struct {
		name, field string
		status      int
		body        func() map[string]any
	}{
		{"too many questions", "questions", 422, func() map[string]any { return questionSetBody(21, testUserID) }},
		{"set without title", "title", 422, func() map[string]any {
			body := questionSetBody(3, testUserID)
			delete(body, "title")
			return body
		}},
		{"unknown policy", "policy", 422, func() map[string]any {
			body := questionSetBody(3, testUserID)
			body["answer_policy"] = "all_at_once"
			return body
		}},
		{"invalid squad UUID", "squad_id", 400, func() map[string]any {
			body := questionSetBody(3, testUserID)
			body["squad_id"] = "not-a-uuid"
			return body
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx := newQuestionAPIFixture(t)
			out := testutil.Call(t, testHandler.AskQuestion, fx.agentRequest("POST", "", tc.body())).Want(tc.status).Text()
			if !strings.Contains(out, tc.field) {
				t.Fatalf("refusal does not name %s: %s", tc.field, out)
			}
			if n := dbfx.Count(t, "SELECT count(*) FROM question WHERE asker_agent_id=$1", fx.AgentID); n != 0 {
				t.Fatalf("refused ask created %d questions", n)
			}
		})
	}
}

func TestQuestionAPI_RefusesSquadFromAnotherWorkspace(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	ws := dbfx.Workspace(t, "Other squad workspace", "other-question-squad-workspace")
	squad := dbfx.Squad(t, "Finance elsewhere", fx.AgentID, testutil.Cols{"workspace_id": ws})
	body := questionSetBody(3, testUserID)
	body["squad_id"] = squad
	testutil.Call(t, testHandler.AskQuestion, fx.agentRequest("POST", "", body)).Want(422)
	if n := dbfx.Count(t, "SELECT count(*) FROM question WHERE asker_agent_id=$1", fx.AgentID); n != 0 {
		t.Fatalf("cross-workspace squad ask created %d questions", n)
	}
}

func TestQuestionAPI_EachAnswersBecomePartlyAnsweredThenAnswered(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	second := dbfx.User(t, "Second answerer", "question-each-api@multica.test")
	dbfx.Member(t, testWorkspaceID, second, "member")
	body := questionSetBody(3, testUserID, second)
	body["answer_policy"] = "each"
	var ask service.QuestionAskResult
	testutil.Call(t, testHandler.AskQuestion, fx.agentRequest("POST", "", body)).Want(201).JSON(&ask)
	if len(ask.Questions) != 3 {
		t.Fatalf("wrong ask size: %+v", ask)
	}
	id := ask.Questions[0].ID
	for i, userID := range []string{testUserID, second} {
		r := withURLParam(newRequest("POST", "/", map[string]any{"value": map[string]any{"choice": "yes"}}), "id", id)
		r.Header.Set("X-User-ID", userID)
		out := testutil.Call(t, testHandler.AnswerQuestion, r).Want(200).Map()
		want := []string{"partly_answered", "answered"}[i]
		if out["status"] != want || out["answer_policy"] != "each" || out["ask_id"] != ask.AskID {
			t.Fatalf("answer %d lost policy status or ask: %+v", i+1, out)
		}
	}
	for _, q := range ask.Questions[1:] {
		out := testutil.Call(t, testHandler.GetQuestion, withURLParam(newRequest("GET", "/", nil), "id", q.ID)).Want(200).Map()
		if out["status"] != "open" {
			t.Fatalf("answering first question changed sibling: %+v", out)
		}
	}
}

func TestQuestionAPI_BlocksWorkFalseSurvivesRequestParsing(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	body := questionAskBody(testUserID)
	body["blocks_work"] = false
	var out service.QuestionAskResult
	testutil.Call(t, testHandler.AskQuestion, fx.agentRequest("POST", "", body)).Want(201).JSON(&out)
	if len(out.Questions) != 1 || out.Questions[0].BlocksWork {
		t.Fatalf("explicit false blocks_work was lost: %+v", out)
	}
}

func TestQuestionAPI_SquadUUIDSuppliesAnswerers(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	squad := dbfx.Squad(t, "Question API Finance", fx.AgentID)
	dbfx.SquadMember(t, squad, "member", testUserID)
	dbfx.SquadMember(t, squad, "agent", fx.AgentID)
	body := questionSetBody(3)
	body["squad_id"] = squad
	var out service.QuestionAskResult
	testutil.Call(t, testHandler.AskQuestion, fx.agentRequest("POST", "", body)).Want(201).JSON(&out)
	if len(out.Questions) != 3 {
		t.Fatalf("squad ask lost questions: %+v", out)
	}
	for _, q := range out.Questions {
		if len(q.Answerers) != 1 || q.Answerers[0].UserID != testUserID {
			t.Fatalf("squad UUID did not supply its human answerer: %+v", q.Answerers)
		}
	}
}
