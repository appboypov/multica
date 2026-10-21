package handler

import (
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCheckAPIOpenAnswerAndTrust(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	project := dbfx.Project(t, "Check API")
	dbfx.Exec(t, "UPDATE issue SET project_id=$2 WHERE id=$1", fx.IssueID, project)
	dbfx.Cleanup(t, "DELETE FROM todo WHERE details->>'project_id'=$1", project)
	dbfx.Cleanup(t, "DELETE FROM project_check_trust WHERE project_id=$1", project)
	body := map[string]string{"issue_id": fx.IssueID, "project_id": project, "member_id": testUserID, "kind": "design", "link": "https://example.test/design", "title": "Invoice design"}
	var row map[string]any
	testutil.Call(t, testHandler.OpenCheck, fx.agentRequest("POST", "", body)).Want(http.StatusCreated).JSON(&row)
	id := row["id"].(string)
	if row["kind"] != "check" {
		t.Fatalf("todo %+v", row)
	}
	testutil.Call(t, inboxWorkspaceHandler(testHandler.MarkTodoDone), todoDoneRequest(id, testWorkspaceID)).Want(http.StatusUnprocessableEntity)
	testutil.Call(t, testHandler.AnswerCheck, fx.agentRequest("POST", id, map[string]string{"answer": "pass"})).Want(http.StatusForbidden)
	testutil.Call(t, testHandler.AnswerCheck, withURLParam(newRequest("POST", "/", map[string]string{"answer": "feedback"}), "id", id)).Want(http.StatusUnprocessableEntity)
	testutil.Call(t, testHandler.AnswerCheck, withURLParam(newRequest("POST", "/", map[string]string{"answer": "pass"}), "id", id)).Want(http.StatusOK).JSON(&row)
	if row["done_by"] != "member" {
		t.Fatalf("answer %+v", row)
	}
	var states []map[string]any
	testutil.Call(t, testHandler.ListCheckTrust, withURLParam(newRequest("GET", "/", nil), "id", project)).Want(http.StatusOK).JSON(&states)
	if len(states) != 1 || states[0]["passed"] != float64(1) || states[0]["sampling"] != false || states[0]["can_manage"] != true {
		t.Fatalf("trust %+v", states)
	}
	body["member_id"] = "not-a-uuid"
	testutil.Call(t, testHandler.OpenCheck, fx.agentRequest("POST", "", body)).Want(http.StatusBadRequest)
}

func TestCheckAPIMemberOpensAndReadsCheck(t *testing.T) {
	issue := dbfx.Issue(t, t.Name())
	project := dbfx.Project(t, "Member check API")
	dbfx.Exec(t, "UPDATE issue SET project_id=$2 WHERE id=$1", issue, project)
	dbfx.Cleanup(t, "DELETE FROM todo WHERE details->>'project_id'=$1", project)
	dbfx.Cleanup(t, "DELETE FROM project_check_trust WHERE project_id=$1", project)
	opener := dbfx.User(t, "Sarkout", "sarkout-"+project+"@example.test")
	dbfx.Member(t, testWorkspaceID, opener, "member")
	body := map[string]string{"issue_id": issue, "project_id": project, "member_id": testUserID, "kind": "feature", "link": "https://example.test/session", "title": "Pass the invoice totals"}
	var row map[string]any
	testutil.Call(t, testHandler.OpenCheck, newRequestAs(opener, "POST", "/api/todos/checks", body)).Want(http.StatusCreated).JSON(&row)
	if row["kind"] != "check" || row["actor_type"] != "member" || row["actor_id"] != opener {
		t.Fatalf("check %+v", row)
	}
	id := row["id"].(string)
	testutil.Call(t, testHandler.AnswerCheck, withURLParam(newRequest("POST", "/", map[string]string{"answer": "feedback", "note": "Round per line"}), "id", id)).Want(http.StatusOK)
	testutil.Call(t, testHandler.GetCheck, withURLParam(newRequestAs(opener, "GET", "/", nil), "id", id)).Want(http.StatusOK).JSON(&row)
	if details, _ := row["details"].(map[string]any); details["answer"] != "feedback" || details["note"] != "Round per line" {
		t.Fatalf("answered check %+v", row)
	}
	bystander := dbfx.User(t, "Bystander", "bystander-"+project+"@example.test")
	dbfx.Member(t, testWorkspaceID, bystander, "member")
	testutil.Call(t, testHandler.GetCheck, withURLParam(newRequestAs(bystander, "GET", "/", nil), "id", id)).Want(http.StatusNotFound)
}

func TestCheckRecordAndInstructionAPI(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	project := dbfx.Project(t, "Check record API")
	dbfx.Exec(t, "UPDATE issue SET project_id=$2 WHERE id=$1", fx.IssueID, project)
	dbfx.Cleanup(t, "DELETE FROM todo WHERE details->>'project_id'=$1", project)
	dbfx.Cleanup(t, "DELETE FROM project_check_trust WHERE project_id=$1", project)
	body := map[string]string{"issue_id": fx.IssueID, "project_id": project, "member_id": testUserID, "kind": "feature", "link": "https://example.test/work", "title": "Check totals"}
	var row map[string]any
	testutil.Call(t, testHandler.OpenCheck, fx.agentRequest("POST", "", body)).Want(http.StatusCreated).JSON(&row)
	check := row["id"].(string)
	testutil.Call(t, testHandler.AnswerCheck, withURLParam(newRequest("POST", "/", map[string]string{"answer": "feedback", "note": "Round totals"}), "id", check)).Want(http.StatusOK)
	request := withURLParams(newRequest("PUT", "/", map[string]bool{"sampling": true}), "id", project, "kind", "feature")
	testutil.Call(t, testHandler.SetCheckSampling, request).Want(http.StatusOK).JSON(&row)
	if row["sampling"] != true {
		t.Fatalf("mode: %+v", row)
	}
	var history []map[string]any
	testutil.Call(t, testHandler.CheckHistory, withURLParams(newRequest("GET", "/", nil), "id", project, "kind", "feature")).Want(http.StatusOK).JSON(&history)
	if len(history) != 1 || history[0]["answer"] != "feedback" {
		t.Fatalf("history: %+v", history)
	}
	note := map[string]string{"file": "skills/totals.md", "link": "https://example.test/commit", "note": "Round totals before display"}
	testutil.Call(t, testHandler.OpenInstructionNote, fx.agentRequest("POST", check, note)).Want(http.StatusCreated).JSON(&row)
	id := row["id"].(string)
	testutil.Call(t, inboxWorkspaceHandler(testHandler.MarkTodoDone), todoDoneRequest(id, testWorkspaceID)).Want(http.StatusUnprocessableEntity)
	testutil.Call(t, testHandler.AnswerCheck, withURLParam(newRequest("POST", "/", map[string]string{"answer": "undo"}), "id", id)).Want(http.StatusOK)
	testutil.Call(t, testHandler.ReportInstructionUndo, fx.agentRequest("POST", id, map[string]string{"link": "https://example.test/revert", "note": "Reverted the instruction"})).Want(http.StatusOK)
	testutil.Call(t, testHandler.AnswerCheck, withURLParam(newRequest("POST", "/", map[string]string{"answer": "keep"}), "id", id)).Want(http.StatusOK).JSON(&row)
	if row["done_at"] == nil {
		t.Fatal("Keep must close the note")
	}
}
