package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func runInputReactionRequest(f supplementFixture, itemID, emoji string) *http.Request {
	r := withURLParams(newRequest(http.MethodPost, "/reactions", map[string]any{"emoji": emoji}), "taskId", f.taskID, "itemId", itemID)
	r.Header.Set("X-Actor-Source", "task_token")
	r.Header.Set("X-Agent-ID", f.agentID)
	r.Header.Set("X-Task-ID", f.taskID)
	return r
}

func TestRunInputReactionsRequireTheReceivingRun(t *testing.T) {
	f := newSupplementFixture(t, "codex", "running", true)
	var comment CommentResponse
	supplementRequest(t, f, "0199a4e8-22ce-7b01-bba5-999999999991", "a notice").Want(http.StatusCreated).JSON(&comment)
	// A receipt must be acknowledged as delivered, not merely pending.
	testutil.Call(t, testHandler.ReactToRunInput, runInputReactionRequest(f, comment.ID, "👀")).Want(http.StatusNotFound)
	dbfx.Exec(t, `UPDATE task_supplement SET status='delivered', delivered_at=now() WHERE task_id=$1`, f.taskID)
	human := withURLParams(newRequest(http.MethodPost, "/reactions", map[string]any{"emoji": "✅"}), "taskId", f.taskID, "itemId", comment.ID)
	testutil.Call(t, testHandler.ReactToRunInput, human).Want(http.StatusUnauthorized)
	other := newSupplementFixture(t, "codex", "running", true)
	testutil.Call(t, testHandler.ReactToRunInput, runInputReactionRequest(other, comment.ID, "✅")).Want(http.StatusNotFound)
	forged := runInputReactionRequest(other, comment.ID, "✅")
	forged = withURLParam(forged, "taskId", f.taskID)
	testutil.Call(t, testHandler.ReactToRunInput, forged).Want(http.StatusNotFound)
	bad := testutil.Call(t, testHandler.ReactToRunInput, runInputReactionRequest(f, comment.ID, "👍")).Want(http.StatusBadRequest).Text()
	if !strings.Contains(bad, "react takes 👀 or ✅") {
		t.Fatalf("unexpected emoji error: %s", bad)
	}
	h := *testHandler
	h.Bus = events.New()
	var published []protocol.RunInputReactedPayload
	h.Bus.Subscribe(protocol.EventRunInputReacted, func(event events.Event) {
		payload, ok := event.Payload.(protocol.RunInputReactedPayload)
		if !ok {
			t.Fatalf("reaction payload type = %T", event.Payload)
		}
		published = append(published, payload)
	})
	var seen, handled, duplicate protocol.RunInputReactedPayload
	testutil.Call(t, h.ReactToRunInput, runInputReactionRequest(f, comment.ID, "👀")).Want(http.StatusOK).JSON(&seen)
	testutil.Call(t, h.ReactToRunInput, runInputReactionRequest(f, comment.ID, "✅")).Want(http.StatusOK).JSON(&handled)
	testutil.Call(t, h.ReactToRunInput, runInputReactionRequest(f, comment.ID, "✅")).Want(http.StatusOK).JSON(&duplicate)
	if seen.SeenAt == nil || seen.HandledAt != nil || handled.HandledAt == nil || duplicate.HandledAt == nil || *handled.HandledAt != *duplicate.HandledAt {
		t.Fatalf("marks not independent and idempotent: seen=%+v handled=%+v duplicate=%+v", seen, handled, duplicate)
	}
	if len(published) != 3 || published[2].IssueID != f.issueID || published[2].AgentID != f.agentID || published[2].TaskID != f.taskID || published[2].ItemID != comment.ID || published[2].ItemKind != "comment" {
		t.Fatalf("reaction event lost identity: %+v", published)
	}
	var timeline []TimelineEntry
	testutil.Call(t, h.ListTimeline, withURLParam(newRequest(http.MethodGet, "/timeline", nil), "id", f.issueID)).Want(http.StatusOK).JSON(&timeline)
	found := false
	for _, entry := range timeline {
		if entry.ID == comment.ID {
			found = true
			if len(entry.Supplements) != 1 || entry.Supplements[0].SeenAt == nil || entry.Supplements[0].HandledAt == nil || entry.Supplements[0].ItemKind != "comment" {
				t.Fatalf("comment marks missing: %+v", entry.Supplements)
			}
		}
	}
	if !found {
		t.Fatal("received comment missing from timeline")
	}
}

func TestRunInputClaimReturnsEveryKind(t *testing.T) {
	for _, kind := range []string{"comment", "comment_edit", "answer", "issue_change"} {
		t.Run(kind, func(t *testing.T) {
			f := newSupplementFixture(t, "codex", "running", true)
			itemID := f.triggerID
			commentID := parseUUID(f.triggerID)
			if kind == "comment_edit" {
				itemID = "0199a4e8-22ce-7b01-bba5-999999999992"
			} else if kind != "comment" {
				commentID.Valid = false
			}
			_, err := testHandler.Queries.CreateRunInputSupplement(t.Context(), db.CreateRunInputSupplementParams{
				TaskID: parseUUID(f.taskID), IssueID: parseUUID(f.issueID), WorkspaceID: parseUUID(testWorkspaceID),
				AuthorID: parseUUID(testUserID), ItemKind: kind, ItemID: parseUUID(itemID), CommentID: commentID,
				Content: pgtype.Text{String: "snapshotted notice", Valid: true},
			})
			if err != nil {
				t.Fatal(err)
			}
			r := withURLParam(newDaemonTokenRequest(http.MethodPost, "/claim", nil, testWorkspaceID, "legit-daemon"), "taskId", f.taskID)
			var claimed protocol.TaskSupplementClaimResponse
			testutil.Call(t, testHandler.ClaimTaskSupplement, r).Want(http.StatusOK).JSON(&claimed)
			if claimed.ItemKind != kind || claimed.ItemID != itemID || claimed.Content != "snapshotted notice" {
				t.Fatalf("claim lost notice snapshot: %+v", claimed)
			}
			ack := withURLParams(newDaemonTokenRequest(http.MethodPost, "/ack", map[string]any{"delivered": true}, testWorkspaceID, "legit-daemon"), "taskId", f.taskID, "commentId", itemID)
			testutil.Call(t, testHandler.AckTaskSupplement, ack).Want(http.StatusOK)
			testutil.Call(t, testHandler.ReactToRunInput, runInputReactionRequest(f, itemID, "✅")).Want(http.StatusOK)
		})
	}
}

func TestEditedCommentCreatesANewRunInput(t *testing.T) {
	f := newSupplementFixture(t, "codex", "running", true)
	dbfx.Cleanup(t, `DELETE FROM agent_task_queue WHERE issue_id=$1`, f.issueID)
	var original CommentResponse
	testutil.Call(t, testHandler.CreateComment, withURLParam(newRequest(http.MethodPost, "/comments", map[string]any{"content": agentMention("Thomas", f.agentID)+" keep the toggle"}), "id", f.issueID)).Want(http.StatusCreated).JSON(&original)
	dbfx.Exec(t, `UPDATE task_supplement SET status='delivered', handled_at=now() WHERE task_id=$1`, f.taskID)
	body := map[string]any{"content": agentMention("Thomas", f.agentID)+" keep the toggle in the header", "expected_revision": original.Revision}
	var edited CommentResponse
	testutil.Call(t, testHandler.UpdateComment, withURLParam(newRequest(http.MethodPut, "/comments/"+original.ID, body), "commentId", original.ID)).Want(http.StatusOK).JSON(&edited)
	rows, err := testHandler.Queries.ListTaskSupplementsByCommentIDs(t.Context(), db.ListTaskSupplementsByCommentIDsParams{WorkspaceID: parseUUID(testWorkspaceID), CommentIds: []pgtype.UUID{parseUUID(original.ID)}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[1].ItemKind != "comment_edit" || rows[1].ItemID == rows[0].ItemID || !strings.Contains(rows[1].Content.String, "in the header") || rows[1].HandledAt.Valid {
		t.Fatalf("edit did not get an independent notice: %+v", rows)
	}
	if len(edited.Supplements) != 2 {
		t.Fatalf("edit response lost receipts: %+v", edited.Supplements)
	}
	body["expected_revision"] = edited.Revision
	testutil.Call(t, testHandler.UpdateComment, withURLParam(newRequest(http.MethodPut, "/comments/"+original.ID, body), "commentId", original.ID)).Want(http.StatusOK)
	if n := dbfx.Count(t, `SELECT count(*) FROM task_supplement WHERE comment_id=$1`, original.ID); n != 2 {
		t.Fatalf("unchanged text created a notice: %d", n)
	}
}

func TestQuestionAnswerMarksFollowLatestEvent(t *testing.T) {
	fx := newQuestionAPIFixture(t)
	id := askQuestionForTest(t, fx)
	var answer service.QuestionDTO
	request := func(value string) *http.Request {
		return withURLParam(newRequest(http.MethodPost, "/answer", map[string]any{"value": map[string]any{"option": value}}), "id", id)
	}
	testutil.Call(t, testHandler.AnswerQuestion, request("per line")).Want(http.StatusOK).JSON(&answer)
	latest := answer.Events[len(answer.Events)-1].ID
	dbfx.Exec(t, `INSERT INTO task_supplement (task_id,workspace_id,issue_id,author_id,client_request_id,item_id,item_kind,status,seen_at,handled_at) VALUES ($1,$2,$3,$4,$5,$5,'answer','delivered',now(),now())`, fx.TaskID, testWorkspaceID, fx.IssueID, testUserID, latest)
	dbfx.Cleanup(t, `DELETE FROM task_supplement WHERE task_id=$1`, fx.TaskID)
	var marked questionResponse
	testutil.Call(t, testHandler.GetQuestion, withURLParam(newRequest(http.MethodGet, "/question", nil), "id", id)).Want(http.StatusOK).JSON(&marked)
	if len(marked.Answers) != 1 || len(marked.Answers[0].RunMarks) != 1 || marked.Answers[0].RunMarks[0].HandledAt == nil {
		t.Fatalf("answer marks missing: %+v", marked.Answers)
	}
	var changed questionResponse
	testutil.Call(t, testHandler.AnswerQuestion, request("per total")).Want(http.StatusOK).JSON(&changed)
	if len(changed.Answers) != 1 || len(changed.Answers[0].RunMarks) != 0 {
		t.Fatalf("new answer inherited previous marks: %+v", changed.Answers)
	}
	// Assert the flattened boundary contract, not nested embedded DTO objects.
	encoded, err := json.Marshal(marked)
	if err != nil || !strings.Contains(string(encoded), `"run_marks"`) || strings.Contains(string(encoded), `"QuestionDTO"`) {
		t.Fatalf("invalid question wire projection: %s / %v", encoded, err)
	}
}

func TestCommentFilesReachRunningRunAsDownloadNotices(t *testing.T) {
	f := newSupplementFixture(t, "codex", "running", true)
	dbfx.Cleanup(t, `DELETE FROM agent_task_queue WHERE issue_id=$1`, f.issueID)
	dbfx.Exec(t, `UPDATE issue SET assignee_type='agent',assignee_id=$2 WHERE id=$1`, f.issueID, f.agentID)
	attachmentID := unlinkedIssueAttachment(t, f.issueID)
	var created CommentResponse
	createCommentWithAttachment(t, testHandler, f.issueID, attachmentID).Want(http.StatusCreated).JSON(&created)
	if len(created.Supplements) != 1 || created.Supplements[0].TaskID != f.taskID {
		t.Fatalf("files blocked steering: %+v", created.Supplements)
	}
	claimed, err := testHandler.Queries.ClaimNextTaskSupplement(t.Context(), parseUUID(f.taskID))
	if err != nil || !strings.Contains(claimed.Content, "shot.png") || !strings.Contains(claimed.Content, "multica attachment download "+attachmentID) {
		t.Fatalf("file notice lost its download identity: %+v / %v", claimed, err)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM agent_task_queue WHERE agent_id=$1`, f.agentID); n != 1 {
		t.Fatalf("file comment created %d runs", n)
	}
}

func TestIssueChangeMarksAppearOnTimeline(t *testing.T) {
	f := newSupplementFixture(t, "codex", "running", true)
	activityID := dbfx.Insert(t, "activity_log", testutil.Cols{
		"workspace_id": testWorkspaceID, "issue_id": f.issueID,
		"actor_type": "member", "actor_id": testUserID, "action": "priority_changed",
		"details": `{"from":"medium","to":"high"}`,
	})
	_, err := testHandler.Queries.CreateRunInputSupplement(t.Context(), db.CreateRunInputSupplementParams{
		TaskID: parseUUID(f.taskID), WorkspaceID: parseUUID(testWorkspaceID), IssueID: parseUUID(f.issueID),
		AuthorID: parseUUID(testUserID), ItemKind: "issue_change", ItemID: parseUUID(activityID),
		Content: pgtype.Text{String: "Brian raised the priority", Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Exec(t, `UPDATE task_supplement SET status='delivered' WHERE task_id=$1`, f.taskID)
	testutil.Call(t, testHandler.ReactToRunInput, runInputReactionRequest(f, activityID, "👀")).Want(http.StatusOK)
	var timeline []TimelineEntry
	testutil.Call(t, testHandler.ListTimeline, withURLParam(newRequest(http.MethodGet, "/timeline", nil), "id", f.issueID)).Want(http.StatusOK).JSON(&timeline)
	for _, entry := range timeline {
		if entry.ID == activityID {
			if len(entry.RunMarks) != 1 || entry.RunMarks[0].AgentID != f.agentID || entry.RunMarks[0].SeenAt == nil || entry.RunMarks[0].HandledAt != nil {
				t.Fatalf("activity marks missing: %+v", entry.RunMarks)
			}
			return
		}
	}
	t.Fatal("marked activity missing from timeline")
}
