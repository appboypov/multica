package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Keep subscriptions local: the suite's shared bus has no unsubscribe operation.
// A fresh task service also keeps its terminal drainer on this handler's bus.
func newQuestionWakeLifecycleHandler() (*Handler, *[]events.Event) {
	h := *testHandler
	h.Bus = events.New()
	h.TaskService = service.NewTaskService(h.Queries, testPool, nil, h.Bus)
	h.TaskService.Analytics = testHandler.TaskService.Analytics
	h.QuestionService = &service.QuestionService{
		Queries: h.Queries, TxStarter: testPool, Bus: h.Bus,
		Tasks: h.TaskService, RunInputs: h.RunInputService,
	}
	h.TaskService.QuestionWakeDrainer = h.QuestionService
	var queued []events.Event
	h.Bus.Subscribe(protocol.EventTaskQueued, func(event events.Event) {
		queued = append(queued, event)
	})
	return &h, &queued
}

func acceptLifecycleQuestionWake(t *testing.T, fx questionAPIFixture, questionID string) {
	t.Helper()
	testutil.Call(t, testHandler.AnswerQuestion, withURLParam(newRequest(http.MethodPost, "/", map[string]any{
		"value": map[string]any{"option": "per line"},
	}), "id", questionID)).Want(http.StatusOK)
	var waiting service.QuestionWakeResult
	testutil.Call(t, testHandler.WakeQuestion, withURLParam(newRequest(http.MethodPost, "/", nil), "id", questionID)).
		Want(http.StatusAccepted).JSON(&waiting)
	if waiting.TaskID != "" || waiting.AnswerCount != 1 {
		t.Fatalf("wake must wait behind existing work: %+v", waiting)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM question_wake_pending WHERE issue_id=$1 AND agent_id=$2`, fx.IssueID, fx.AgentID); n != 1 {
		t.Fatalf("accepted wake has %d pending intents, want one", n)
	}
}

// No foreign key links this audit to the issue or task. It therefore catches a
// wake inserted during cancellation even if deletion subsequently cascades it.
// UUID-derived names and an issue-specific WHEN make this safe beside other tests.
func installLifecycleWakeInsertAudit(t *testing.T, issueID string) string {
	t.Helper()
	suffix := strings.ReplaceAll(issueID, "-", "")
	table := "question_wake_audit_" + suffix
	function := "question_wake_audit_fn_" + suffix
	trigger := "question_wake_audit_tr_" + suffix
	dbfx.Cleanup(t, "DROP TABLE IF EXISTS "+table)
	dbfx.Cleanup(t, "DROP FUNCTION IF EXISTS "+function+"()")
	dbfx.Cleanup(t, "DROP TRIGGER IF EXISTS "+trigger+" ON agent_task_queue")
	dbfx.Exec(t, "CREATE TABLE "+table+" (task_id uuid NOT NULL)")
	dbfx.Exec(t, fmt.Sprintf(`
CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
	INSERT INTO %s (task_id) VALUES (NEW.id);
	RETURN NEW;
END;
$$`, function, table))
	dbfx.Exec(t, fmt.Sprintf(`
CREATE TRIGGER %s AFTER INSERT ON agent_task_queue
FOR EACH ROW WHEN (NEW.issue_id = '%s'::uuid AND NEW.trigger_evidence_kind = 'question_wake')
EXECUTE FUNCTION %s()`, trigger, issueID, function))
	return table
}

func assertLifecycleDeletedWithoutWake(t *testing.T, fx questionAPIFixture, questionID, auditTable string) {
	t.Helper()
	for _, check := range []struct {
		name  string
		query string
		arg   string
	}{
		{"issue", `SELECT count(*) FROM issue WHERE id=$1`, fx.IssueID},
		{"task", `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, fx.IssueID},
		{"pending intent", `SELECT count(*) FROM question_wake_pending WHERE issue_id=$1`, fx.IssueID},
		{"woken event", `SELECT count(*) FROM question_event WHERE question_id=$1 AND kind='woken'`, questionID},
	} {
		if n := dbfx.Count(t, check.query, check.arg); n != 0 {
			t.Errorf("deleted issue retained %d %s rows", n, check.name)
		}
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM "+auditTable); n != 0 {
		t.Errorf("deletion transiently inserted %d question wakes before cascading their rows", n)
	}
}

func TestDeleteIssue_WaitingQuestionWakeNeverEnqueuedOrPublished(t *testing.T) {
	for _, status := range []string{"queued", "dispatched", "running"} {
		t.Run(status, func(t *testing.T) {
			fx := newQuestionAPIFixture(t)
			questionID := askQuestionForTest(t, fx)
			acceptLifecycleQuestionWake(t, fx, questionID)
			dbfx.Exec(t, `UPDATE agent_task_queue SET status=$2 WHERE id=$1`, fx.TaskID, status)
			audit := installLifecycleWakeInsertAudit(t, fx.IssueID)
			h, queued := newQuestionWakeLifecycleHandler()
			testutil.Call(t, h.DeleteIssue, withURLParam(newRequest(http.MethodDelete, "/", nil), "id", fx.IssueID)).
				Want(http.StatusNoContent)
			assertLifecycleDeletedWithoutWake(t, fx, questionID, audit)
			if len(*queued) != 0 {
				t.Fatalf("deletion published queued tasks: %+v", *queued)
			}
		})
	}
}

func TestBatchDeleteIssues_WaitingQuestionWakesNeverEnqueuedOrPublished(t *testing.T) {
	var fixtures []questionAPIFixture
	var questionIDs, audits, issueIDs []string
	for _, status := range []string{"queued", "dispatched", "running"} {
		fx := newQuestionAPIFixture(t)
		questionID := askQuestionForTest(t, fx)
		acceptLifecycleQuestionWake(t, fx, questionID)
		dbfx.Exec(t, `UPDATE agent_task_queue SET status=$2 WHERE id=$1`, fx.TaskID, status)
		fixtures = append(fixtures, fx)
		questionIDs = append(questionIDs, questionID)
		audits = append(audits, installLifecycleWakeInsertAudit(t, fx.IssueID))
		issueIDs = append(issueIDs, fx.IssueID)
	}
	h, queued := newQuestionWakeLifecycleHandler()
	var result struct {
		Deleted int `json:"deleted"`
	}
	testutil.Call(t, h.BatchDeleteIssues, newRequest(http.MethodPost, "/", map[string]any{"issue_ids": issueIDs})).
		Want(http.StatusOK).JSON(&result)
	if result.Deleted != len(fixtures) {
		t.Fatalf("deleted %d issues, want %d", result.Deleted, len(fixtures))
	}
	for i, fx := range fixtures {
		assertLifecycleDeletedWithoutWake(t, fx, questionIDs[i], audits[i])
	}
	if len(*queued) != 0 {
		t.Fatalf("batch deletion published queued tasks: %+v", *queued)
	}
}

func newLifecycleCommentWait(t *testing.T, status string) (questionAPIFixture, string, string, string, string) {
	t.Helper()
	fx := newQuestionAPIFixture(t)
	questionID := askQuestionForTest(t, fx)
	// The asking run ended earlier; this separate, unstarted comment run is the
	// blocker whose intermediate cancellation must not drain the accepted wake.
	dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed',completed_at=now() WHERE id=$1`, fx.TaskID)
	content := agentMention("Lifecycle agent", fx.AgentID) + " original instruction"
	commentID := dbfx.Comment(t, fx.IssueID, content)
	taskID := dbfx.Task(t, fx.AgentID, testutil.Cols{
		"runtime_id": fx.RuntimeID, "issue_id": fx.IssueID,
		"trigger_comment_id": commentID, "status": status,
	})
	acceptLifecycleQuestionWake(t, fx, questionID)
	return fx, questionID, commentID, taskID, content
}

func assertLifecycleWaitRetained(t *testing.T, h *Handler, fx questionAPIFixture, questionID string) {
	t.Helper()
	// Exercise the recovery entry point as well as the automatic terminal drain.
	if err := h.QuestionService.DrainQuestionWake(context.Background(), parseUUID(testWorkspaceID), parseUUID(fx.AgentID), parseUUID(fx.IssueID)); err != nil {
		t.Fatalf("recover waiting wake: %v", err)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM question_wake_pending WHERE issue_id=$1 AND agent_id=$2`, fx.IssueID, fx.AgentID); n != 1 {
		t.Fatalf("replacement lost waiting intent: %d", n)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND trigger_evidence_kind='question_wake'`, fx.IssueID); n != 0 {
		t.Fatalf("replacement lifecycle queued %d early question wakes", n)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM question_event WHERE question_id=$1 AND kind='woken'`, questionID); n != 0 {
		t.Fatalf("replacement lifecycle emitted %d early woken events", n)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM activity_log WHERE issue_id=$1 AND action=$2`, fx.IssueID, service.ActivityQuestionWake); n != 0 {
		t.Fatalf("replacement lifecycle emitted %d early wake activities", n)
	}
}

func assertLifecycleCommentReplacement(t *testing.T, fx questionAPIFixture, oldTaskID, commentID, content string) string {
	t.Helper()
	old, err := testHandler.Queries.GetAgentTask(context.Background(), parseUUID(oldTaskID))
	if err != nil {
		t.Fatal(err)
	}
	if old.Status != "cancelled" || !taskToResponse(old, testWorkspaceID).CancelledByCommentChange {
		t.Fatalf("original comment task was not cancelled by its change: %+v", old)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND agent_id=$2 AND status IN ('queued','dispatched','running','waiting_local_directory','deferred')`, fx.IssueID, fx.AgentID); n != 1 {
		t.Fatalf("comment edit produced %d active tasks, want exactly one replacement", n)
	}
	var replacementID, actualContent string
	dbfx.QueryRow(t, `SELECT task.id::text, comment.content
FROM agent_task_queue task JOIN comment ON comment.id=task.trigger_comment_id
WHERE task.issue_id=$1 AND task.agent_id=$2 AND task.trigger_comment_id=$3 AND task.status='queued'`, fx.IssueID, fx.AgentID, commentID).
		Scan(&replacementID, &actualContent)
	if replacementID == oldTaskID || actualContent != content {
		t.Fatalf("replacement id/content = %s/%q, want new task for %q", replacementID, actualContent, content)
	}
	return replacementID
}

func assertLifecycleReplacementStages(t *testing.T, h *Handler, fx questionAPIFixture, questionID, replacementID, content string) {
	t.Helper()
	assertLifecycleWaitRetained(t, h, fx, questionID)
	task := claimQuestionTaskForTest(t, fx)
	if task.ID != replacementID || !strings.Contains(task.TriggerCommentContent, content) {
		t.Fatalf("claim did not receive replacement input: %+v, want %s with %q", task, replacementID, content)
	}
	var status string
	dbfx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id=$1`, replacementID).Scan(&status)
	if status != "dispatched" {
		t.Fatalf("claimed replacement status=%q, want dispatched", status)
	}
	assertLifecycleWaitRetained(t, h, fx, questionID)
	startQuestionTaskForTest(t, fx, task)
	dbfx.QueryRow(t, `SELECT status FROM agent_task_queue WHERE id=$1`, replacementID).Scan(&status)
	if status != "running" {
		t.Fatalf("started replacement status=%q, want running", status)
	}
	assertLifecycleWaitRetained(t, h, fx, questionID)
}

func TestUpdateComment_WaitingQuestionWakeStaysBehindReplacement(t *testing.T) {
	for _, mode := range []string{"strict", "legacy"} {
		for _, status := range []string{"queued", "dispatched"} {
			t.Run(mode+"/"+status, func(t *testing.T) {
				fx, questionID, commentID, oldTaskID, original := newLifecycleCommentWait(t, status)
				edited := agentMention("Lifecycle agent", fx.AgentID) + " edited instruction"
				body := map[string]any{"content": edited}
				if mode == "strict" {
					body["content_base"] = original
				}
				h, _ := newQuestionWakeLifecycleHandler()
				testutil.Call(t, h.UpdateComment, withURLParam(newRequest(http.MethodPut, "/", body), "commentId", commentID)).Want(http.StatusOK)
				replacementID := assertLifecycleCommentReplacement(t, fx, oldTaskID, commentID, edited)
				assertLifecycleReplacementStages(t, h, fx, questionID, replacementID, edited)
			})
		}
	}
}

func TestUpdateComment_WithoutReplacementDrainsWaitingQuestionWake(t *testing.T) {
	for _, mode := range []string{"strict", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			fx, questionID, commentID, oldTaskID, original := newLifecycleCommentWait(t, "queued")
			body := map[string]any{"content": "edited without an agent mention"}
			if mode == "strict" {
				body["content_base"] = original
			}
			h, _ := newQuestionWakeLifecycleHandler()
			testutil.Call(t, h.UpdateComment, withURLParam(newRequest(http.MethodPut, "/", body), "commentId", commentID)).Want(http.StatusOK)
			if n := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status='cancelled'`, oldTaskID); n != 1 {
				t.Fatal("comment edit did not cancel original task")
			}
			assertLifecycleQuestionWakeDrained(t, fx, questionID)
		})
	}
}

func TestUpdateComment_LegacyFailureRecoversBeforeWaitingQuestionWake(t *testing.T) {
	for _, status := range []string{"queued", "dispatched"} {
		t.Run(status, func(t *testing.T) {
			fx, questionID, commentID, oldTaskID, original := newLifecycleCommentWait(t, status)
			suffix := strings.ReplaceAll(commentID, "-", "")
			function := "question_edit_fail_fn_" + suffix
			trigger := "question_edit_fail_tr_" + suffix
			dbfx.Cleanup(t, "DROP FUNCTION IF EXISTS "+function+"()")
			dbfx.Cleanup(t, "DROP TRIGGER IF EXISTS "+trigger+" ON comment")
			dbfx.Exec(t, fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
	RAISE EXCEPTION 'injected lifecycle comment update failure';
END;
$$`, function))
			dbfx.Exec(t, fmt.Sprintf(`CREATE TRIGGER %s BEFORE UPDATE ON comment
FOR EACH ROW WHEN (OLD.id='%s'::uuid AND NEW.content IS DISTINCT FROM OLD.content)
EXECUTE FUNCTION %s()`, trigger, commentID, function))
			h, _ := newQuestionWakeLifecycleHandler()
			testutil.Call(t, h.UpdateComment, withURLParam(newRequest(http.MethodPut, "/", map[string]any{
				"content": agentMention("Lifecycle agent", fx.AgentID) + " rejected edit",
			}), "commentId", commentID)).Want(http.StatusInternalServerError)
			replacementID := assertLifecycleCommentReplacement(t, fx, oldTaskID, commentID, original)
			assertLifecycleReplacementStages(t, h, fx, questionID, replacementID, original)
		})
	}
}

func assertLifecycleQuestionWakeDrained(t *testing.T, fx questionAPIFixture, questionID string) {
	t.Helper()
	if n := dbfx.Count(t, `SELECT count(*) FROM question_wake_pending WHERE issue_id=$1 AND agent_id=$2`, fx.IssueID, fx.AgentID); n != 0 {
		t.Fatalf("settled lifecycle retained %d waiting intents", n)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND agent_id=$2 AND trigger_evidence_kind='question_wake' AND status='queued'`, fx.IssueID, fx.AgentID); n != 1 {
		t.Fatalf("settled lifecycle queued %d question wakes, want one", n)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM question_event WHERE question_id=$1 AND kind='woken'`, questionID); n != 1 {
		t.Fatalf("settled lifecycle emitted %d woken events, want one", n)
	}
}

func TestQuestionWake_StandaloneCancellationStillDrainsWaitingIntent(t *testing.T) {
	for _, status := range []string{"queued", "dispatched", "running"} {
		t.Run(status, func(t *testing.T) {
			fx := newQuestionAPIFixture(t)
			questionID := askQuestionForTest(t, fx)
			acceptLifecycleQuestionWake(t, fx, questionID)
			dbfx.Exec(t, `UPDATE agent_task_queue SET status=$2 WHERE id=$1`, fx.TaskID, status)
			h, _ := newQuestionWakeLifecycleHandler()
			req := withURLParams(newRequest(http.MethodPost, "/", nil), "id", fx.IssueID, "taskId", fx.TaskID)
			testutil.Call(t, h.CancelTask, req).Want(http.StatusOK)
			assertLifecycleQuestionWakeDrained(t, fx, questionID)
		})
	}
}

func TestDeleteComment_WaitingQuestionWakeStaysBehindSurvivingInput(t *testing.T) {
	for _, status := range []string{"queued", "dispatched"} {
		t.Run(status, func(t *testing.T) {
			fx, questionID, commentID, oldTaskID, _ := newLifecycleCommentWait(t, status)
			root := dbfx.Comment(t, fx.IssueID, "agent-owned thread", testutil.Cols{
				"author_type": "agent", "author_id": fx.AgentID,
			})
			survivorContent := agentMention("Lifecycle agent", fx.AgentID) + " surviving instruction"
			survivorID := dbfx.Comment(t, fx.IssueID, survivorContent, testutil.Cols{"parent_id": root})
			dbfx.Exec(t, `UPDATE comment SET parent_id=$2 WHERE id=$1`, commentID, root)
			// Refresh thread scope after moving the trigger under the shared root.
			dbfx.Exec(t, `UPDATE agent_task_queue SET trigger_comment_id=trigger_comment_id,coalesced_comment_ids=ARRAY[$2::uuid] WHERE id=$1`, oldTaskID, survivorID)
			h, _ := newQuestionWakeLifecycleHandler()
			testutil.Call(t, h.DeleteComment, withURLParam(newRequest(http.MethodDelete, "/", nil), "commentId", commentID)).
				Want(http.StatusNoContent)
			if n := dbfx.Count(t, `SELECT count(*) FROM comment WHERE id=$1`, commentID); n != 0 {
				t.Fatalf("deleted trigger still exists: %d", n)
			}
			replacementID := assertLifecycleCommentReplacement(t, fx, oldTaskID, survivorID, survivorContent)
			assertLifecycleReplacementStages(t, h, fx, questionID, replacementID, survivorContent)
		})
	}
}
