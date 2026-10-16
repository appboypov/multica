package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/attribution"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func nativeReleaseTaskFixture(t *testing.T, state string) (releaseID, agentID, runtimeID, taskID string, generation time.Time) {
	t.Helper()
	if testHandler == nil {
		t.Fatal("PostgreSQL is required for native release tests")
	}
	_, _, releaseID = milestoneReleaseFixture(t, testWorkspaceID)
	runtimeID = dbfx.Runtime(t, "release-runtime", testutil.Cols{"metadata": testutil.Raw(`'{"capabilities":["release-coordination-v1"]}'::jsonb`)})
	agentID = dbfx.Agent(t, "Native release coordinator", runtimeID, testutil.Cols{"instructions": "Keep this selected agent identity."})
	dbfx.Exec(t, "UPDATE release SET owner_type='agent',owner_id=$2 WHERE id=$1", releaseID, agentID)
	generation = time.Date(2026, 9, 17, 10, 0, 0, 123456000, time.UTC)
	taskID = dbfx.Task(t, agentID, testutil.Cols{"release_id": releaseID, "runtime_id": runtimeID, "status": state, "dispatched_at": generation, "context": testutil.Raw(`'{"wakeup_evidence":[{"event_type":"release.owner_changed","actor_type":"member"}]}'::jsonb`)})
	return
}

func releaseStartRequest(taskID, runtimeID string, generation time.Time, legacy, capable bool) *http.Request {
	var body any
	if !legacy {
		body = map[string]string{"runtime_id": runtimeID, "dispatched_at": generation.Format(time.RFC3339Nano)}
	}
	req := withURLParam(newDaemonTokenRequest("POST", "/api/daemon/tasks/"+taskID+"/start", body, testWorkspaceID, "release-start"), "taskId", taskID)
	if capable {
		req.Header.Set("X-Client-Capabilities", protocol.DaemonCapabilityReleaseCoordinationV1)
	}
	return req
}

func TestNativeReleaseClaimHydratesScopeWithoutQuickCreate(t *testing.T) {
	releaseID, agentID, runtimeID, taskID, _ := nativeReleaseTaskFixture(t, "dispatched")
	first := dbfx.Issue(t, "First fixed release issue", testutil.Cols{"release_id": releaseID, "created_at": time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC), "assignee_type": "agent", "assignee_id": agentID})
	second := dbfx.Issue(t, "Second fixed release issue", testutil.Cols{"release_id": releaseID, "created_at": time.Date(2026, 1, 1, 2, 0, 0, 0, time.UTC), "parent_issue_id": first, "status": "cancelled"})
	comment := dbfx.Insert(t, "comment", testutil.Cols{"workspace_id": testWorkspaceID, "release_id": releaseID, "author_type": "member", "author_id": testUserID, "content": "Coordinate these existing issues.", "type": "comment"})
	dbfx.Exec(t, "UPDATE agent_task_queue SET trigger_comment_id=$2,coalesced_comment_ids=ARRAY[$2::uuid] WHERE id=$1", taskID, comment)
	task, err := testHandler.Queries.GetAgentTask(context.Background(), parseUUID(taskID))
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := testHandler.Queries.GetAgentRuntime(context.Background(), parseUUID(runtimeID))
	if err != nil {
		t.Fatal(err)
	}
	req := newDaemonTokenRequest("POST", "/claim", nil, testWorkspaceID, "release-claim")
	req.Header.Set("X-Client-Capabilities", protocol.DaemonCapabilityReleaseCoordinationV1)
	resp, _, _, _, _, failure := testHandler.buildClaimedTaskResponse(req, &task, runtime, runtimeID, testWorkspaceID)
	if failure != nil {
		t.Fatalf("release claim failed: %+v", failure)
	}
	if resp.Kind != "release" || resp.ReleaseID != releaseID || resp.Release == nil || resp.Release.ID != releaseID || resp.IssueID != "" || resp.QuickCreatePrompt != "" || resp.QuestionsEnabled {
		t.Fatalf("wrong native claim: %+v", resp)
	}
	if resp.Release.RepositoryID == "" || resp.Release.Version == "" || !json.Valid(resp.Release.Issues) || !strings.Contains(string(resp.ReleaseTriggers), "release.owner_changed") {
		t.Fatalf("missing fixed scope/source reasons: %+v", resp.Release)
	}
	if !strings.Contains(resp.Agent.Instructions, "Keep this selected agent identity.") || resp.IsLeaderTask {
		t.Fatalf("agent identity/leader role changed: %+v", resp.Agent)
	}
	var issues []ReleaseIssueResponse
	if err := json.Unmarshal(resp.Release.Issues, &issues); err != nil {
		t.Fatal(err)
	}
	if len(issues) != 2 || issues[0].ID != first || issues[1].ID != second || resp.Release.AllSuccessful || resp.Release.RemainingOpen != 1 {
		t.Fatalf("wrong ordered/cancelled issue projection: %+v", issues)
	}
	if resp.TriggerCommentID == nil || *resp.TriggerCommentID != comment || resp.TriggerAuthorType != "member" || resp.TriggerCommentContent != "Coordinate these existing issues." {
		t.Fatalf("missing delivered source actor/body: %+v", resp)
	}
}

func TestNativeReleaseStartRejectsUnsupportedStaleAndShipped(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		for _, cause := range []string{"unsupported", "owner", "shipped", "removed"} {
			t.Run(strings.Join([]string{map[bool]string{true: "legacy", false: "generation"}[legacy], cause}, "/"), func(t *testing.T) {
				releaseID, _, runtimeID, taskID, generation := nativeReleaseTaskFixture(t, "dispatched")
				switch cause {
				case "owner":
					dbfx.Exec(t, "UPDATE release SET owner_type=NULL,owner_id=NULL WHERE id=$1", releaseID)
				case "shipped":
					dbfx.Exec(t, "UPDATE release SET shipped_at=now() WHERE id=$1", releaseID)
				case "removed":
					dbfx.Exec(t, "DELETE FROM release WHERE id=$1", releaseID)
				}
				result := testutil.Call(t, testHandler.StartTask, releaseStartRequest(taskID, runtimeID, generation, legacy, cause != "unsupported"))
				result.Want(http.StatusConflict)
				if n := dbfx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE id=$1 AND started_at IS NOT NULL", taskID); n != 0 {
					t.Fatal("stale/unsupported release task launched")
				}
			})
		}
	}
}

func TestNativeReleaseCompletionStoresActualResultAndDeduplicates(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{true: "explicit report", false: "terminal result"}[explicit], func(t *testing.T) {
			releaseID, agentID, _, taskID, _ := nativeReleaseTaskFixture(t, "running")
			body := "Actual release outcome\n\nPreserve literal \\n and 中文."
			if explicit {
				dbfx.Insert(t, "comment", testutil.Cols{"workspace_id": testWorkspaceID, "release_id": releaseID, "author_type": "agent", "author_id": agentID, "source_task_id": taskID, "content": body, "type": "comment"})
			}
			output := body
			if explicit {
				output = ""
			}
			payload, _ := json.Marshal(protocol.TaskCompletedPayload{Output: output})
			_, changed, err := testHandler.TaskService.CompleteTaskWithTransition(context.Background(), parseUUID(taskID), payload, "", "", "", false, "", "")
			if err != nil || !changed {
				t.Fatalf("completion changed=%v err=%v", changed, err)
			}
			_, changed, err = testHandler.TaskService.CompleteTaskWithTransition(context.Background(), parseUUID(taskID), payload, "", "", "", false, "", "")
			if err != nil || changed {
				t.Fatalf("completion replay changed=%v err=%v", changed, err)
			}
			if n := dbfx.Count(t, "SELECT count(*) FROM comment WHERE release_id=$1 AND source_task_id=$2 AND deleted_at IS NULL", releaseID, taskID); n != 1 {
				t.Fatalf("reports=%d", n)
			}
			var stored string
			dbfx.QueryRow(t, "SELECT content FROM comment WHERE release_id=$1 AND source_task_id=$2 AND deleted_at IS NULL", releaseID, taskID).Scan(&stored)
			if stored != body {
				t.Fatalf("real result changed: %q", stored)
			}
		})
	}
}

func TestNativeReleaseEmptyResultDoesNotFabricateSuccess(t *testing.T) {
	releaseID, _, _, taskID, _ := nativeReleaseTaskFixture(t, "running")
	payload, _ := json.Marshal(protocol.TaskCompletedPayload{Output: "  "})
	_, changed, err := testHandler.TaskService.CompleteTaskWithTransition(context.Background(), parseUUID(taskID), payload, "", "", "", false, "", "")
	if changed || !errors.Is(err, service.ErrReleaseResultEmpty) {
		t.Fatalf("empty completion changed=%v err=%v", changed, err)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM comment WHERE release_id=$1", releaseID); n != 0 {
		t.Fatal("fabricated release result")
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status='running'", taskID); n != 1 {
		t.Fatal("empty completion committed terminal success")
	}
}

func TestNativeReleaseSquadBriefingTargetsCurrentLeaderOnly(t *testing.T) {
	releaseID, agentID, runtimeID, taskID, _ := nativeReleaseTaskFixture(t, "dispatched")
	squadID := dbfx.Squad(t, "Release owners", agentID, testutil.Cols{"instructions": "Carry this squad's delivery workflow."})
	dbfx.SquadMember(t, squadID, "agent", agentID)
	dbfx.Exec(t, "UPDATE release SET owner_type='squad',owner_id=$2 WHERE id=$1", releaseID, squadID)
	dbfx.Exec(t, "UPDATE agent_task_queue SET squad_id=$2 WHERE id=$1", taskID, squadID)
	ctx := context.Background()
	task, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(taskID))
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := testHandler.Queries.GetAgentRuntime(ctx, parseUUID(runtimeID))
	if err != nil {
		t.Fatal(err)
	}
	resp := taskToResponse(task, testWorkspaceID)
	agent, err := testHandler.Queries.GetAgent(ctx, parseUUID(agentID))
	if err != nil {
		t.Fatal(err)
	}
	resp.Agent = &TaskAgentData{ID: agentID, Name: agent.Name, Instructions: agent.Instructions}
	if _, err := testHandler.hydrateReleaseClaim(ctx, &task, runtime, &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.IsLeaderTask || resp.SquadID != squadID || !strings.Contains(resp.Agent.Instructions, "Carry this squad's delivery workflow.") {
		t.Fatalf("missing native squad guidance: %+v", resp)
	}
	replacement := dbfx.Agent(t, "Next release leader", runtimeID)
	dbfx.Exec(t, "UPDATE squad SET leader_id=$2 WHERE id=$1", squadID, replacement)
	if _, err := testHandler.hydrateReleaseClaim(ctx, &task, runtime, &resp); err == nil {
		t.Fatal("former squad leader consumed release claim")
	}
}

func TestNativeReleaseClaimRejectsMissingCapability(t *testing.T) {
	_, _, runtimeID, taskID, _ := nativeReleaseTaskFixture(t, "dispatched")
	ctx := context.Background()
	task, err := testHandler.Queries.GetAgentTask(ctx, parseUUID(taskID))
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := testHandler.Queries.GetAgentRuntime(ctx, parseUUID(runtimeID))
	if err != nil {
		t.Fatal(err)
	}
	req := newDaemonTokenRequest("POST", "/claim", nil, testWorkspaceID, "old-daemon")
	resp, _, _, _, _, failure := testHandler.buildClaimedTaskResponse(req, &task, runtime, runtimeID, testWorkspaceID)
	if failure == nil || failure.status != http.StatusConflict || resp.QuickCreatePrompt != "" {
		t.Fatalf("unsupported daemon claim=%+v failure=%+v", resp, failure)
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status='queued'", taskID); n != 1 {
		t.Fatal("unsupported claim was not returned to capable daemons")
	}
}

func TestNativeReleaseRunningCompletionAfterShipOrRemoval(t *testing.T) {
	for _, removed := range []bool{false, true} {
		t.Run(map[bool]string{false: "shipped", true: "removed"}[removed], func(t *testing.T) {
			releaseID, _, _, taskID, _ := nativeReleaseTaskFixture(t, "running")
			if removed {
				dbfx.Exec(t, "DELETE FROM release WHERE id=$1", releaseID)
			} else {
				dbfx.Exec(t, "UPDATE release SET shipped_at=now() WHERE id=$1", releaseID)
			}
			payload, _ := json.Marshal(protocol.TaskCompletedPayload{Output: "Real shipper result."})
			_, changed, err := testHandler.TaskService.CompleteTaskWithTransition(context.Background(), parseUUID(taskID), payload, "", "", "", false, "", "")
			if err != nil || !changed {
				t.Fatalf("running completion changed=%v err=%v", changed, err)
			}
			want := 1
			if removed {
				want = 0
			}
			if n := dbfx.Count(t, "SELECT count(*) FROM comment WHERE release_id=$1", releaseID); n != want {
				t.Fatalf("reports after teardown=%d want=%d", n, want)
			}
			if removed && dbfx.Count(t, "SELECT count(*) FROM release WHERE id=$1", releaseID) != 0 {
				t.Fatal("completion recreated removed release")
			}
		})
	}
}

func TestNativeReleaseStartConsumesExactReceiptsOnce(t *testing.T) {
	releaseID, _, runtimeID, taskID, generation := nativeReleaseTaskFixture(t, "dispatched")
	visible := dbfx.Insert(t, "comment", testutil.Cols{"workspace_id": testWorkspaceID, "release_id": releaseID, "author_type": "member", "author_id": testUserID, "content": "Delivered source.", "type": "comment"})
	omitted := dbfx.Insert(t, "comment", testutil.Cols{"workspace_id": testWorkspaceID, "release_id": releaseID, "author_type": "member", "author_id": testUserID, "content": "Omitted source.", "type": "comment"})
	dbfx.Exec(t, "UPDATE agent_task_queue SET delivered_comment_ids=ARRAY[$2::uuid] WHERE id=$1", taskID, visible)
	ruleID := dbfx.Insert(t, "issue_wakeup", testutil.Cols{"id": uuid.NewString(), "workspace_id": testWorkspaceID, "release_id": releaseID, "instruction": "Coordinate sources.", "kind": "event", "mode": "continuous", "event_types": []string{"release.comment.created"}, "created_by": testUserID})
	receipt := func(key, commentID string) string {
		payload, _ := json.Marshal(map[string]string{"comment_id": commentID})
		return dbfx.Insert(t, "issue_wakeup_receipt", testutil.Cols{"id": uuid.NewString(), "wakeup_id": ruleID, "revision": 1, "event_key": key, "event_type": "release.comment.created", "payload": string(payload), "task_id": taskID})
	}
	deliveredReceipt := receipt("visible", visible)
	omittedReceipt := receipt("omitted", omitted)
	testutil.Call(t, testHandler.StartTask, releaseStartRequest(taskID, runtimeID, generation, false, true)).Want(http.StatusOK)
	testutil.Call(t, testHandler.StartTask, releaseStartRequest(taskID, runtimeID, generation, false, true)).Want(http.StatusOK)
	if n := dbfx.Count(t, "SELECT count(*) FROM issue_wakeup_receipt WHERE id=$1 AND processed_at IS NOT NULL AND task_id=$2", deliveredReceipt, taskID); n != 1 {
		t.Fatal("delivered source not consumed")
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM issue_wakeup_receipt WHERE id=$1 AND processed_at IS NULL AND task_id IS NULL", omittedReceipt); n != 1 {
		t.Fatal("omitted source lost instead of retained for follow-up")
	}
	if n := dbfx.Count(t, "SELECT count(*) FROM issue_wakeup WHERE id=$1 AND fire_count=1", ruleID); n != 1 {
		t.Fatal("start replay consumed the rule twice")
	}
}

func TestNativeReleaseQueuedMergeReplacesWholeMemberAuthorityOnly(t *testing.T) {
	releaseID, agentID, _, taskID, _ := nativeReleaseTaskFixture(t, "queued")
	secondUser := dbfx.User(t, "Second requester", "release-merge-"+taskID+"@example.test")
	dbfx.Member(t, testWorkspaceID, secondUser, "member")
	firstComment := dbfx.Insert(t, "comment", testutil.Cols{"workspace_id": testWorkspaceID, "release_id": releaseID, "author_type": "member", "author_id": testUserID, "content": "First source.", "type": "comment"})
	secondComment := dbfx.Insert(t, "comment", testutil.Cols{"workspace_id": testWorkspaceID, "release_id": releaseID, "author_type": "member", "author_id": secondUser, "content": "Second source.", "type": "comment"})
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	q := testHandler.Queries.WithTx(tx)
	if _, err := q.LockReleaseTaskRepository(ctx, parseUUID(taskID)); err != nil {
		t.Fatal(err)
	}
	rel, err := q.GetReleaseInWorkspace(ctx, db.GetReleaseInWorkspaceParams{ID: parseUUID(releaseID), WorkspaceID: parseUUID(testWorkspaceID)})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := q.GetAgent(ctx, parseUUID(agentID))
	if err != nil {
		t.Fatal(err)
	}
	p := service.ReleaseTaskEnqueueParams{Release: rel, Agent: agent, Context: map[string]any{"wakeup_evidence": map[string]any{"version": 1}, "handoff_note": "Current release facts."}, Attribution: attribution.DirectHumanRun(parseUUID(testUserID), attribution.EvidenceComment, parseUUID(firstComment)), TriggerCommentID: parseUUID(firstComment), ReplaceAttribution: true, RuntimeMCPOverlay: []byte(`{"mcpServers":{"first":{}}}`), ConnectedApps: []byte(`[{"name":"first"}]`)}
	first, err := testHandler.TaskService.EnqueueReleaseTaskTx(ctx, q, p)
	if err != nil {
		t.Fatal(err)
	}
	p.Attribution = attribution.DirectHumanRun(parseUUID(secondUser), attribution.EvidenceComment, parseUUID(secondComment))
	p.TriggerCommentID = parseUUID(secondComment)
	p.CommentIDs = []pgtype.UUID{parseUUID(firstComment), parseUUID(secondComment)}
	p.RuntimeMCPOverlay = []byte(`{"mcpServers":{"second":{}}}`)
	p.ConnectedApps = []byte(`[{"name":"second"}]`)
	p.ReplaceAttribution = false
	automated, err := testHandler.TaskService.EnqueueReleaseTaskTx(ctx, q, p)
	if err != nil {
		t.Fatal(err)
	}
	if automated.OriginatorUserID != first.OriginatorUserID || automated.AccountableUserID != first.AccountableUserID || automated.TriggerEvidenceRefID != first.TriggerEvidenceRefID || string(automated.RuntimeMcpOverlay) != string(first.RuntimeMcpOverlay) || string(automated.RuntimeConnectedApps) != string(first.RuntimeConnectedApps) {
		t.Fatal("automated input borrowed another member's authority")
	}
	p.ReplaceAttribution = true
	second, err := testHandler.TaskService.EnqueueReleaseTaskTx(ctx, q, p)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID || second.OriginatorUserID != parseUUID(secondUser) || second.AccountableUserID != parseUUID(secondUser) || second.TriggerCommentID != parseUUID(secondComment) || second.TriggerEvidenceRefID != parseUUID(secondComment) || !strings.Contains(string(second.RuntimeMcpOverlay), "second") || !strings.Contains(string(second.RuntimeConnectedApps), "second") {
		t.Fatalf("partial member authority replacement: %+v", second)
	}
	foundFirst := false
	for _, id := range second.CoalescedCommentIds {
		if id == parseUUID(firstComment) {
			foundFirst = true
		}
	}
	if !foundFirst {
		t.Fatal("queued merge lost first member's source")
	}
	if _, err := tx.Exec(ctx, "UPDATE agent_task_queue SET status='dispatched' WHERE id=$1", taskID); err != nil {
		t.Fatal(err)
	}
	if _, err := testHandler.TaskService.EnqueueReleaseTaskTx(ctx, q, p); !errors.Is(err, service.ErrReleaseTaskDispatched) {
		t.Fatalf("mutated dispatched inputs: %v", err)
	}
}
