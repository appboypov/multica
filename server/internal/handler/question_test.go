package handler

import (
	"bytes"
	"encoding/json"
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
	task := dbfx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "issue_id": issue, "status": "running", "started_at": testutil.Raw("now()-interval '1 hour'")})
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
	return map[string]any{"prompt": "Round per line or per total?", "answer_type": "single_choice",
		"settings": map[string]any{"options": []string{"per line", "per total"}, "allow_other": false}, "answerer_ids": answerers}
}

func askQuestionForTest(t *testing.T, fx questionAPIFixture) string {
	t.Helper()
	out := testutil.Call(t, testHandler.AskQuestion, fx.agentRequest("POST", "", questionAskBody(testUserID))).Want(201).Map()
	id, ok := out["id"].(string)
	if !ok || id == "" {
		t.Fatalf("missing question id: %+v", out)
	}
	return id
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
	for _, field := range []string{"links", "answerers", "answers", "events", "unseen_answer_count", "wake_available"} {
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
