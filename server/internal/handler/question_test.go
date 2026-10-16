package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/question"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

type questionAPIFixture struct{ AgentID, IssueID, TaskID, RuntimeID string }

func newQuestionAPIFixture(t *testing.T) questionAPIFixture {
	t.Helper()
	runtime := dbfx.Runtime(t, t.Name())
	agent := dbfx.Agent(t, t.Name()+"-"+runtime, runtime)
	issue := dbfx.Issue(t, t.Name())
	task := dbfx.Task(t, agent, testutil.Cols{"runtime_id": runtime, "issue_id": issue, "status": "running", "started_at": testutil.Raw("now()-interval '1 hour'"), "questions_enabled": true})
	// Task tokens and activity rows are run-owned, without database cascades.
	dbfx.Cleanup(t, "DELETE FROM agent_task_queue WHERE agent_id=$1", agent)
	dbfx.Cleanup(t, "DELETE FROM task_token WHERE task_id IN (SELECT id FROM agent_task_queue WHERE agent_id=$1)", agent)
	dbfx.Cleanup(t, "DELETE FROM activity_log WHERE issue_id=$1", issue)
	dbfx.Cleanup(t, "DELETE FROM question_wake_pending WHERE agent_id=$1", agent)
	dbfx.Cleanup(t, "DELETE FROM task_supplement_capability WHERE task_id IN (SELECT id FROM agent_task_queue WHERE agent_id=$1)", agent)
	dbfx.Cleanup(t, "DELETE FROM task_supplement WHERE task_id IN (SELECT id FROM agent_task_queue WHERE agent_id=$1)", agent)
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

func TestQuestionAPI_MemberNeedsIssueLink(t *testing.T) {
	testutil.Call(t, testHandler.AskQuestion, newRequest("POST", "/api/questions", questionAskBody(testUserID))).Want(422)
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

func TestQuestionAPI_MemberWakeAcceptedImmediatelyAndWhileQueued(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	id := askQuestionForTest(t, fx)
	dbfx.Exec(t, "UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1", fx.TaskID)
	testutil.Call(t, testHandler.AnswerQuestion, withURLParam(newRequest("POST", "/", map[string]any{"value": map[string]any{"option": "per line"}}), "id", id)).Want(200)
	var wake service.QuestionWakeResult
	testutil.Call(t, testHandler.WakeQuestion, withURLParam(newRequest("POST", "/", nil), "id", id)).Want(202).JSON(&wake)
	if wake.TaskID == "" || wake.AnswerCount != 1 {
		t.Fatalf("wake response: %+v", wake)
	}
	var waiting service.QuestionWakeResult
	testutil.Call(t, testHandler.WakeQuestion, withURLParam(newRequest("POST", "/", nil), "id", id)).Want(202).JSON(&waiting)
	if waiting.TaskID != "" || waiting.AnswerCount != 1 {
		t.Fatalf("queued wake response: %+v", waiting)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE agent_id=$1 AND issue_id=$2 AND status='queued'", fx.AgentID, fx.IssueID); n != 1 {
		t.Fatalf("repeated wake created %d queued tasks", n)
	}
}

func TestQuestionAPI_WaitingWakeAcceptedAndAuthorizationPreserved(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	id := askQuestionForTest(t, fx)
	testutil.Call(t, testHandler.AnswerQuestion, withURLParam(newRequest("POST", "/", map[string]any{
		"value": map[string]any{"option": "per line"},
	}), "id", id)).Want(200)
	token, _ := insertTestPAT(t, time.Now().Add(time.Hour))
	protected := middleware.Auth(testHandler.Queries, nil, nil, nil)(http.HandlerFunc(testHandler.WakeQuestion))
	r := withURLParam(newRequest("POST", "/api/questions/"+id+"/wake", nil), "id", id)
	r.Header.Set("Authorization", "Bearer "+token)
	var waiting service.QuestionWakeResult
	testutil.Call(t, protected.ServeHTTP, r).Want(http.StatusAccepted).JSON(&waiting)
	if waiting.TaskID != "" || waiting.AnswerCount != 1 {
		t.Fatalf("waiting wake response: %+v", waiting)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE agent_id=$1 AND issue_id=$2", fx.AgentID, fx.IssueID); n != 1 {
		t.Fatalf("waiting wake created an extra task: %d", n)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM question_wake_pending WHERE workspace_id=$1 AND issue_id=$2 AND agent_id=$3", testWorkspaceID, fx.IssueID, fx.AgentID); n != 1 {
		t.Fatalf("accepted wake missing durable intent: %d", n)
	}

	unauthenticated := withURLParam(newRequest("POST", "/", nil), "id", id)
	testutil.Call(t, protected.ServeHTTP, unauthenticated).Want(http.StatusUnauthorized)
	testutil.Call(t, testHandler.WakeQuestion, fx.agentRequest("POST", id, nil)).Want(http.StatusForbidden)
	machine := withURLParam(newRequest("POST", "/", nil), "id", id)
	machine.Header.Set("X-Actor-Source", "cloud_pat")
	testutil.Call(t, testHandler.WakeQuestion, machine).Want(http.StatusForbidden)
	outsider := dbfx.User(t, "Wake outsider", "wake-outsider-"+fx.IssueID+"@multica.test")
	forbidden := withURLParam(newRequest("POST", "/", nil), "id", id)
	forbidden.Header.Set("X-User-ID", outsider)
	testutil.Call(t, testHandler.WakeQuestion, forbidden).Want(http.StatusForbidden)
	wrongWorkspace := dbfx.Workspace(t, "Wake other workspace", "wake-other-"+fx.IssueID)
	dbfx.Member(t, wrongWorkspace, testUserID, "member")
	crossWorkspace := withURLParam(newRequest("POST", "/", nil), "id", id)
	crossWorkspace.Header.Set("X-Workspace-ID", wrongWorkspace)
	testutil.Call(t, testHandler.WakeQuestion, crossWorkspace).Want(http.StatusNotFound)
	if n := dbfx.Count(t, "SELECT count(*) FROM question_wake_pending WHERE agent_id=$1", fx.AgentID); n != 1 {
		t.Fatalf("unauthorized wake changed accepted intent: %d", n)
	}
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

func cleanupMemberQuestionAsk(t *testing.T, askID string) {
	t.Helper()
	dbfx.Cleanup(t, `DELETE FROM question WHERE ask_id = $1`, askID)
	for _, table := range []string{"question_link", "question_answerer", "question_answer", "question_event"} {
		dbfx.Cleanup(t, `DELETE FROM `+table+` WHERE question_id IN (SELECT id FROM question WHERE ask_id = $1)`, askID)
	}
}

func TestQuestionAPI_AuthenticatedMemberPATAsksOnIssue(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	token, _ := insertTestPAT(t, time.Now().Add(time.Hour))
	protected := middleware.Auth(testHandler.Queries, nil, nil, nil)(http.HandlerFunc(testHandler.AskQuestion))
	body := questionAskBody(testUserID)
	body["links"] = []QuestionLinkRequest{{Type: "issue", ID: fx.IssueID}}
	r := newRequest("POST", "/api/questions", body)
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Del("X-User-ID")
	// A member PAT cannot turn itself into a run by replaying these headers.
	r.Header.Set("X-Actor-Source", "task_token")
	r.Header.Set("X-Agent-ID", fx.AgentID)
	r.Header.Set("X-Task-ID", fx.TaskID)
	var out service.QuestionAskResult
	testutil.Call(t, protected.ServeHTTP, r).Want(201).JSON(&out)
	cleanupMemberQuestionAsk(t, out.AskID)
	if len(out.Questions) != 1 || out.AskID == "" {
		t.Fatalf("member PAT ask = %+v", out)
	}
	row := out.Questions[0]
	if row.AskerUserID != testUserID || row.AskerAgentID != "" || row.AskedInTaskID != "" || len(row.Links) != 1 || row.Links[0].LinkType != "issue" || row.Links[0].LinkID != fx.IssueID || row.WakeAvailable || len(row.WakeTargets) != 0 {
		t.Fatalf("member PAT wire identity or links = %+v", row)
	}
	listProtected := middleware.Auth(testHandler.Queries, nil, nil, nil)(middleware.RequireWorkspaceMember(testHandler.Queries)(http.HandlerFunc(testHandler.ListQuestions)))
	listRequest := newRequest("GET", "/api/questions?tab=all_open&asker_id="+testUserID+"&link_id="+fx.IssueID, nil)
	listRequest.Header.Set("Authorization", "Bearer "+token)
	var listed QuestionRowPageResponse
	testutil.Call(t, listProtected.ServeHTTP, listRequest).Want(200).JSON(&listed)
	if len(listed.Rows) != 1 || listed.Rows[0].ID != row.ID || listed.Rows[0].AskerUserID != testUserID || listed.Rows[0].AskerAgentID != "" {
		t.Fatalf("member asker list response = %+v", listed)
	}
}

func TestQuestionAPI_CloudCredentialCannotAskOrSpoofMember(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	body := questionAskBody(testUserID)
	body["links"] = []QuestionLinkRequest{{Type: "issue", ID: fx.IssueID}}
	// Pin the handler backstop for an already-authenticated machine request.
	r := newRequest("POST", "/api/questions", body)
	r.Header.Set("X-Actor-Source", "cloud_pat")
	testutil.Call(t, testHandler.AskQuestion, r).Want(403)
	verified := 0
	fleet := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		verified++
		if r.Method != "POST" || r.URL.Path != "/api/v1/pat/verify" {
			t.Errorf("unexpected fleet request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"valid": true, "owner_id": testUserID, "instance_id": "question-test-node", "instance_record_id": fx.RuntimeID})
	}))
	defer fleet.Close()
	verifier := auth.NewCloudPATVerifier(auth.CloudPATVerifierConfig{FleetBaseURL: fleet.URL})
	protected := middleware.Auth(testHandler.Queries, nil, verifier, nil)(http.HandlerFunc(testHandler.AskQuestion))
	r = newRequest("POST", "/api/questions", body)
	r.Header.Set("Authorization", "Bearer mcn_question_test")
	r.Header.Set("X-Actor-Source", "member")
	r.Header.Set("X-Agent-ID", fx.AgentID)
	r.Header.Set("X-Task-ID", fx.TaskID)
	testutil.Call(t, protected.ServeHTTP, r).Want(403)
	if verified != 1 {
		t.Fatalf("cloud credential did not reach real authentication: verify calls=%d", verified)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM question q JOIN question_link l ON l.question_id = q.id WHERE l.link_type = 'issue' AND l.link_id = $1`, fx.IssueID); got != 0 {
		t.Fatalf("refused machine asks persisted %d questions", got)
	}
}

func TestQuestionAPI_AuthenticatedTaskTokenKeepsAgentAndRunLink(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	token, err := auth.GenerateAgentTaskToken()
	if err != nil {
		t.Fatal(err)
	}
	_, err = testHandler.Queries.CreateTaskToken(context.Background(), db.CreateTaskTokenParams{
		ID: dbid.NewV7(), TokenHash: auth.HashToken(token), TaskID: parseUUID(fx.TaskID),
		AgentID: parseUUID(fx.AgentID), WorkspaceID: parseUUID(testWorkspaceID), UserID: parseUUID(testUserID),
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	protected := middleware.Auth(testHandler.Queries, nil, nil, nil)(http.HandlerFunc(testHandler.AskQuestion))
	r := newRequest("POST", "/api/questions", questionAskBody(testUserID))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Del("X-User-ID")
	r.Header.Set("X-Actor-Source", "member")
	var out service.QuestionAskResult
	testutil.Call(t, protected.ServeHTTP, r).Want(201).JSON(&out)
	if len(out.Questions) != 1 {
		t.Fatalf("task-token ask = %+v", out)
	}
	row := out.Questions[0]
	if row.AskerUserID != "" || row.AskerAgentID != fx.AgentID || row.AskedInTaskID != fx.TaskID || len(row.Links) != 2 || row.Links[0].LinkID != fx.IssueID || row.Links[1].LinkType != "task" || row.Links[1].LinkID != fx.TaskID {
		t.Fatalf("task-token identity or run link changed: %+v", row)
	}
}

func TestQuestionAPI_MemberQuestionMutationAndRoleGates(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	token, _ := insertTestPAT(t, time.Now().Add(time.Hour))
	body := questionSetBody(3, testUserID)
	body["links"] = []QuestionLinkRequest{{Type: "issue", ID: fx.IssueID}}
	var asked service.QuestionAskResult
	testutil.Call(t, testHandler.AskQuestion, newRequest("POST", "/api/questions", body)).Want(201).JSON(&asked)
	cleanupMemberQuestionAsk(t, asked.AskID)
	if len(asked.Questions) != 3 {
		t.Fatalf("member set = %+v", asked)
	}
	memberRequest := func(id string, body any) *http.Request {
		r := withURLParam(newRequest("POST", "/api/questions/"+id, body), "id", id)
		r.Header.Set("Authorization", "Bearer "+token)
		return r
	}
	callMember := func(action http.HandlerFunc, id string, body any, status int) *testutil.Response {
		t.Helper()
		protected := middleware.Auth(testHandler.Queries, nil, nil, nil)(action)
		return testutil.Call(t, protected.ServeHTTP, memberRequest(id, body)).Want(status)
	}
	id := asked.Questions[0].ID
	for i, choice := range []string{"yes", "no"} {
		var row service.QuestionDTO
		callMember(testHandler.AnswerQuestion, id, map[string]any{"value": map[string]any{"choice": choice}, "wake": true}, 200).JSON(&row)
		reason := "answered"
		if i > 0 {
			reason = "answer_edited"
		}
		if row.Status != "answered" || len(row.Answers) != 1 || row.Answers[0].DisplayText == "" || row.Events[len(row.Events)-1].Kind != reason {
			t.Fatalf("member answer HTTP response = %+v", row)
		}
		var value question.YesNoAnswerValue
		if err := json.Unmarshal(row.Answers[0].Value, &value); err != nil || value.Choice != choice {
			t.Fatalf("member answer value = %s, %v", row.Answers[0].Value, err)
		}
	}
	var replied service.QuestionDTO
	callMember(testHandler.AskBackQuestion, id, map[string]any{"text": "Which plan?"}, 200).JSON(&replied)
	if replied.Events[len(replied.Events)-1].Kind != "ask_back" {
		t.Fatalf("member ask back HTTP response = %+v", replied)
	}
	var declined service.QuestionDTO
	callMember(testHandler.AnswerQuestion, asked.Questions[1].ID, map[string]any{"value": map[string]any{"declined": true}, "wake": true}, 200).JSON(&declined)
	if declined.Status != "answered" || len(declined.Answers) != 1 || !question.Declined(declined.Answers[0].Value) {
		t.Fatalf("member decline HTTP response = %+v", declined)
	}
	reassigned := dbfx.User(t, "New question answerer", "member-question-reassign-"+fx.IssueID+"@multica.test")
	dbfx.Member(t, testWorkspaceID, reassigned, "member")
	var moved service.QuestionDTO
	callMember(testHandler.ReassignQuestion, asked.Questions[2].ID, map[string]any{"user_id": reassigned, "note": "Your decision"}, 200).JSON(&moved)
	if len(moved.Answerers) != 1 || moved.Answerers[0].UserID != reassigned {
		t.Fatalf("member reassign HTTP response = %+v", moved)
	}
	callMember(testHandler.CloseQuestion, id, nil, 403)
	callMember(testHandler.WithdrawQuestion, id, nil, 403)
	callMember(testHandler.AddQuestionLinks, id, map[string]any{"links": []QuestionLinkRequest{{Type: "issue", ID: fx.IssueID}}}, 403)
	callMember(testHandler.WakeQuestion, id, nil, 422)
	callMember(testHandler.WakeQuestion, id, map[string]any{"agent_id": fx.AgentID}, 422)
	for _, action := range []http.HandlerFunc{testHandler.AnswerQuestion, testHandler.AskBackQuestion, testHandler.ReassignQuestion, testHandler.WakeQuestion} {
		testutil.Call(t, action, fx.agentRequest("POST", id, nil)).Want(403)
		machine := memberRequest(id, nil)
		machine.Header.Set("X-Actor-Source", "cloud_pat")
		testutil.Call(t, action, machine).Want(403)
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, fx.IssueID); got != 1 {
		t.Fatalf("member HTTP mutations queued a run: %d tasks", got)
	}
}

func TestQuestionAPI_AskerRequiresAuthenticatedMemberOrTaskToken(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	for _, tc := range []struct {
		name    string
		request func() *http.Request
		status  int
	}{
		{"unauthenticated", func() *http.Request {
			r := newRequest("POST", "/api/questions", questionAskBody(testUserID))
			r.Header.Del("X-User-ID")
			return r
		}, 401},
		{"agent without task token", func() *http.Request {
			r := fx.agentRequest("POST", "", questionAskBody(testUserID))
			r.Header.Del("X-Actor-Source")
			return r
		}, 403},
		{"task token without run", func() *http.Request {
			r := fx.agentRequest("POST", "", questionAskBody(testUserID))
			r.Header.Del("X-Task-ID")
			return r
		}, 403},
		{"malformed task UUID", func() *http.Request {
			r := fx.agentRequest("POST", "", questionAskBody(testUserID))
			r.Header.Set("X-Task-ID", "not-a-uuid")
			return r
		}, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testutil.Call(t, testHandler.AskQuestion, tc.request()).Want(tc.status)
		})
	}
	if got := dbfx.Count(t, `SELECT count(*) FROM question WHERE asker_agent_id = $1`, fx.AgentID); got != 0 {
		t.Fatalf("refused task asks persisted %d questions", got)
	}
}
