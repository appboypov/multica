package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func releaseCoordinationFixture(t *testing.T) (principalFixture, *IssueWakeupService, db.Release, string) {
	t.Helper()
	f, s, _, agent := wakeFixture(t)
	project := f.Project(t, "Release coordination")
	repo := f.Insert(t, "repository", testutil.Cols{"workspace_id": f.WorkspaceID, "project_id": project, "github_repo_id": 1, "full_name": "release/coordination", "private": false, "default_branch": "main", "checkout_daemon_id": "fixture", "checkout_path": "/test", "version_policy": "tags", "last_tag": "1.0.0"})
	id := f.Insert(t, "release", testutil.Cols{"workspace_id": f.WorkspaceID, "repository_id": repo, "position": 0, "version": "1.1.0", "title": "Coordinate", "owner_type": "agent", "owner_id": agent, "created_by": f.UserID})
	f.Cleanup(t, "DELETE FROM issue_wakeup WHERE release_id=$1", id)
	f.Cleanup(t, "DELETE FROM issue_wakeup_receipt WHERE wakeup_id IN(SELECT id FROM issue_wakeup WHERE release_id=$1)", id)
	f.Cleanup(t, "DELETE FROM comment WHERE release_id=$1", id)
	f.Cleanup(t, "DELETE FROM agent_task_queue WHERE release_id=$1", id)
	rel, err := f.q.GetReleaseInWorkspace(context.Background(), db.GetReleaseInWorkspaceParams{WorkspaceID: parseTestUUID(t, f.WorkspaceID), ID: parseTestUUID(t, id)})
	if err != nil {
		t.Fatal(err)
	}
	if err = CaptureReleaseOwnerChange(context.Background(), f.q, rel, "member", parseTestUUID(t, f.UserID), parseTestUUID(t, f.UserID)); err != nil {
		t.Fatal(err)
	}
	return f, s, rel, agent
}

func TestReleaseCoordinationQueuesOnceAndMergesMemberComments(t *testing.T) {
	f, s, rel, agent := releaseCoordinationFixture(t)
	ctx := context.Background()
	if err := s.ProcessReleaseCoordination(ctx, rel.ID); err != nil {
		t.Fatal(err)
	}
	first, err := f.q.FindPendingReleaseTask(ctx, rel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.AgentID != parseTestUUID(t, agent) || first.IssueID.Valid || first.ReleaseID != rel.ID {
		t.Fatalf("not a native release task: %+v", first)
	}
	comment := f.Insert(t, "comment", testutil.Cols{"workspace_id": f.WorkspaceID, "release_id": rel.ID, "author_type": "member", "author_id": f.UserID, "content": "Start only after review"})
	if err = s.ProcessReleaseCoordination(ctx, rel.ID); err != nil {
		t.Fatal(err)
	}
	merged, err := f.q.FindPendingReleaseTask(ctx, rel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if merged.ID != first.ID || merged.TriggerCommentID != parseTestUUID(t, comment) || merged.OriginatorUserID != parseTestUUID(t, f.UserID) {
		t.Fatalf("comment failed to merge with its principal: %+v", merged)
	}
	if n := f.Count(t, "SELECT count(*) FROM agent_task_queue WHERE release_id=$1 AND status='queued'", rel.ID); n != 1 {
		t.Fatalf("queued release runs = %d", n)
	}
}

func TestReleaseCoordinationQueuesForAnOfflineOwner(t *testing.T) {
	f, s, rel, agent := releaseCoordinationFixture(t)
	ctx := context.Background()
	f.Exec(t, "UPDATE agent_runtime SET status='offline' WHERE id=(SELECT runtime_id FROM agent WHERE id=$1)", agent)
	if err := s.ProcessReleaseCoordination(ctx, rel.ID); err != nil {
		t.Fatal(err)
	}
	task, err := f.q.FindPendingReleaseTask(ctx, rel.ID)
	if err != nil {
		t.Fatalf("an offline owner must get its run queued like an assignment: %v", err)
	}
	if n := f.Count(t, "SELECT count(*) FROM issue_wakeup_receipt WHERE task_id=$1 AND processed_at IS NULL", task.ID); n != 1 {
		t.Fatalf("owner facts reserved for the waiting run = %d", n)
	}
}

func TestReleaseCoordinationRateLimitRestsAndResumes(t *testing.T) {
	f, s, rel, agent := releaseCoordinationFixture(t)
	ctx := context.Background()
	runtime := testutil.Raw(fmt.Sprintf("(SELECT runtime_id FROM agent WHERE id='%s')", agent))
	for i := range wakeupHourlyRunLimit {
		f.Task(t, agent, testutil.Cols{"runtime_id": runtime, "release_id": rel.ID, "status": "completed", "created_at": testutil.Raw(fmt.Sprintf("now() - interval '%d minutes'", 10+i))})
	}
	if err := s.ProcessReleaseCoordination(ctx, rel.ID); err != nil {
		t.Fatal(err)
	}
	if n := f.Count(t, "SELECT count(*) FROM agent_task_queue WHERE release_id=$1 AND status='queued'", rel.ID); n != 0 {
		t.Fatalf("rate-limited release queued %d runs", n)
	}
	rule, err := f.q.GetReleaseCoordinationWakeup(ctx, rel.ID)
	if err != nil {
		t.Fatal(err)
	}
	var limitedSince time.Time
	f.QueryRow(t, "SELECT created_at FROM agent_task_queue WHERE release_id=$1 ORDER BY created_at DESC OFFSET $2 LIMIT 1", rel.ID, wakeupHourlyRunLimit-1).Scan(&limitedSince)
	if !rule.Enabled || rule.DisabledAt.Valid || rule.PausedReason.Valid || !rule.NextFireAt.Time.Equal(limitedSince.Add(time.Hour)) {
		t.Fatalf("rule must rest until the quota resets: %+v", rule)
	}
	if n := f.Count(t, "SELECT count(*) FROM issue_wakeup_receipt WHERE wakeup_id=$1 AND processed_at IS NULL", rule.ID); n != 1 {
		t.Fatalf("pending facts while rate-limited = %d", n)
	}
	ready, err := f.q.ListReadyWakeups(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range ready {
		if w.ID == rule.ID {
			t.Fatal("a resting rule must not be polled before its quota resets")
		}
	}

	f.Exec(t, "UPDATE agent_task_queue SET created_at=created_at - interval '1 hour' WHERE release_id=$1", rel.ID)
	f.Exec(t, "UPDATE issue_wakeup SET next_fire_at=now() - interval '1 second' WHERE id=$1", rule.ID)
	if err = s.ProcessReleaseCoordination(ctx, rel.ID); err != nil {
		t.Fatal(err)
	}
	if n := f.Count(t, "SELECT count(*) FROM agent_task_queue WHERE release_id=$1 AND status='queued'", rel.ID); n != 1 {
		t.Fatalf("coordination did not resume after the quota reset: %d queued", n)
	}
}

func TestReleaseDeferredRetryWaitsForThePendingSlotWithoutBlockingOtherRuntimes(t *testing.T) {
	f, s, rel, agent := releaseCoordinationFixture(t)
	ctx := context.Background()
	if err := s.ProcessReleaseCoordination(ctx, rel.ID); err != nil {
		t.Fatal(err)
	}
	pending, err := f.q.FindPendingReleaseTask(ctx, rel.ID)
	if err != nil {
		t.Fatal(err)
	}
	runtime := util.UUIDToString(pending.RuntimeID)
	due := testutil.Raw("now() - interval '1 minute'")
	retry := f.Task(t, agent, testutil.Cols{"runtime_id": runtime, "release_id": rel.ID, "status": "deferred", "fire_at": due, "retry_of_task_id": pending.ID})
	otherRuntime := f.Runtime(t, "unrelated runtime")
	otherAgent := f.Agent(t, "unrelated agent", otherRuntime)
	otherIssue := f.Issue(t, "Unrelated deferred work")
	f.Cleanup(t, "DELETE FROM agent_task_queue WHERE issue_id=$1", otherIssue)
	unrelated := f.Task(t, otherAgent, testutil.Cols{"runtime_id": otherRuntime, "issue_id": otherIssue, "status": "deferred", "fire_at": due})

	runtimes := []pgtype.UUID{pending.RuntimeID, parseTestUUID(t, otherRuntime)}
	promoted, err := f.q.PromoteDueDeferredTasksForRuntimes(ctx, db.PromoteDueDeferredTasksForRuntimesParams{RuntimeIds: runtimes, RuntimeStaleSecs: RuntimeClaimFreshnessSeconds})
	if err != nil {
		t.Fatalf("an occupied release slot must not fail promotion for every runtime: %v", err)
	}
	if len(promoted) != 1 || util.UUIDToString(promoted[0].ID) != unrelated {
		t.Fatalf("promoted = %+v, want only the unrelated runtime's task", promoted)
	}
	if err = s.Tasks.PromoteDueDeferredTasksForRuntime(ctx, pending.RuntimeID); err != nil {
		t.Fatalf("singular promotion on the release runtime: %v", err)
	}
	next, err := f.q.NextDeferredTaskFireAtForRuntimes(ctx, db.NextDeferredTaskFireAtForRuntimesParams{RuntimeIds: []pgtype.UUID{pending.RuntimeID}, RuntimeStaleSecs: RuntimeClaimFreshnessSeconds})
	if err != nil || next.Valid {
		t.Fatalf("a blocked retry must not advertise an immediate claim: %v, %v", next, err)
	}
	if n := f.Count(t, "SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status='deferred'", retry); n != 1 {
		t.Fatal("the retry must stay deferred while the release slot is held")
	}

	f.Exec(t, "UPDATE agent_task_queue SET status='completed', completed_at=now() WHERE id=$1", pending.ID)
	if err = s.Tasks.PromoteDueDeferredTasksForRuntime(ctx, pending.RuntimeID); err != nil {
		t.Fatal(err)
	}
	if n := f.Count(t, "SELECT count(*) FROM agent_task_queue WHERE id=$1 AND status='queued'", retry); n != 1 {
		t.Fatal("the retry must take the freed release slot")
	}
}

func TestReleaseCoordinationKeepsPostClaimFactsPending(t *testing.T) {
	f, s, rel, _ := releaseCoordinationFixture(t)
	ctx := context.Background()
	if err := s.ProcessReleaseCoordination(ctx, rel.ID); err != nil {
		t.Fatal(err)
	}
	task, err := f.q.FindPendingReleaseTask(ctx, rel.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.Exec(t, "UPDATE agent_task_queue SET status='dispatched',dispatched_at=now() WHERE id=$1", task.ID)
	before, err := f.q.GetAgentTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	comment := f.Insert(t, "comment", testutil.Cols{"workspace_id": f.WorkspaceID, "release_id": rel.ID, "author_type": "member", "author_id": f.UserID, "content": "After the claim"})
	if err = s.ProcessReleaseCoordination(ctx, rel.ID); err != nil {
		t.Fatal(err)
	}
	after, err := f.q.GetAgentTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(after.Context) != string(before.Context) || after.TriggerCommentID != before.TriggerCommentID {
		t.Fatal("dispatched input mutated")
	}
	if n := f.Count(t, "SELECT count(*) FROM issue_wakeup_receipt WHERE payload->>'comment_id'=$1 AND processed_at IS NULL AND task_id IS NULL", comment); n != 1 {
		t.Fatalf("late fact pending = %d", n)
	}
}

func TestReleaseCoordinationTerminalTransitionsCaptureWithoutSelfSuppression(t *testing.T) {
	f, s, rel, _ := releaseCoordinationFixture(t)
	issue := f.Issue(t, "Released issue", testutil.Cols{"release_id": rel.ID, "status": "in_progress"})
	ctx := context.Background()
	for _, status := range []string{"done", "in_progress", "cancelled"} {
		if _, err := f.q.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{WorkspaceID: rel.WorkspaceID, ID: parseTestUUID(t, issue), Status: status}); err != nil {
			t.Fatal(err)
		}
	}
	if n := f.Count(t, "SELECT count(*) FROM issue_wakeup_receipt WHERE payload->>'issue_id'=$1 AND event_type='release.issue.ended'", issue); n != 2 {
		t.Fatalf("terminal transition facts = %d", n)
	}
	if err := s.ProcessReleaseCoordination(ctx, rel.ID); err != nil {
		t.Fatal(err)
	}
	if n := f.Count(t, "SELECT count(*) FROM agent_task_queue WHERE release_id=$1", rel.ID); n != 1 {
		t.Fatalf("terminal facts coalesced into %d tasks", n)
	}
	if n := f.Count(t, "SELECT count(*) FROM release WHERE id=$1 AND shipped_at IS NULL", rel.ID); n != 1 {
		t.Fatal("coordination shipped a release")
	}
}

func TestReleaseWakeupUsesCurrentOwnerAndRejectsIssueOnlyConditions(t *testing.T) {
	f, s, rel, _ := releaseCoordinationFixture(t)
	ctx := context.Background()
	for _, raw := range []string{`{"type":"children_done"}`, `{"type":"issue_field","field":"status","value":"done"}`, `{"type":"pull_request","event":"merged"}`} {
		_, err := s.CreateRelease(ctx, rel, parseTestUUID(t, f.UserID), pgtype.UUID{}, WakeupInput{Kind: "event", Instruction: "Wait", Condition: json.RawMessage(raw)})
		if err == nil {
			t.Fatalf("accepted issue-only release condition %s", raw)
		}
	}
	w, err := s.CreateRelease(ctx, rel, parseTestUUID(t, f.UserID), pgtype.UUID{}, WakeupInput{Kind: "at", AfterSeconds: 60, Instruction: "Check release"})
	if err != nil {
		t.Fatal(err)
	}
	other := f.privateAgentOwnedBy(t, f.UserID, "new-release-owner")
	f.Exec(t, "UPDATE release SET owner_id=$2 WHERE id=$1", rel.ID, other)
	f.Exec(t, "UPDATE issue_wakeup SET next_fire_at=now()-interval '1 second' WHERE id=$1", w.ID)
	w, err = f.q.LocklessWakeup(ctx, w.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.dispatchRelease(ctx, w); err != nil {
		t.Fatal(err)
	}
	task, err := f.q.FindPendingReleaseTask(ctx, rel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if task.AgentID != parseTestUUID(t, other) {
		t.Fatalf("wait targeted old owner: %+v", task)
	}
}

func TestReleaseTimedRuleMergeKeepsEarlierFacts(t *testing.T) {
	f, s, rel, _ := releaseCoordinationFixture(t)
	ctx := context.Background()
	f.Insert(t, "comment", testutil.Cols{"workspace_id": f.WorkspaceID, "release_id": rel.ID, "author_type": "member", "author_id": f.UserID, "content": "Ship after the docs"})
	if err := s.ProcessReleaseCoordination(ctx, rel.ID); err != nil {
		t.Fatal(err)
	}
	w, err := s.CreateRelease(ctx, rel, parseTestUUID(t, f.UserID), pgtype.UUID{}, WakeupInput{Kind: "at", AfterSeconds: 60, Instruction: "Check release"})
	if err != nil {
		t.Fatal(err)
	}
	f.Exec(t, "UPDATE issue_wakeup SET next_fire_at=now()-interval '1 second' WHERE id=$1", w.ID)
	if w, err = f.q.LocklessWakeup(ctx, w.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.dispatchRelease(ctx, w); err != nil {
		t.Fatal(err)
	}
	task, err := f.q.FindPendingReleaseTask(ctx, rel.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"release.owner_changed", "release.comment.created", "time.due"} {
		if !strings.Contains(string(task.Context), want) {
			t.Fatalf("merged run lost %q: %s", want, task.Context)
		}
	}
}

func TestReleaseQueuedRunFollowsTheCurrentOwner(t *testing.T) {
	f, s, rel, _ := releaseCoordinationFixture(t)
	ctx := context.Background()
	if err := s.ProcessReleaseCoordination(ctx, rel.ID); err != nil {
		t.Fatal(err)
	}
	first, err := f.q.FindPendingReleaseTask(ctx, rel.ID)
	if err != nil {
		t.Fatal(err)
	}
	other := f.privateAgentOwnedBy(t, f.UserID, "next-release-owner")
	f.Exec(t, "UPDATE release SET owner_id=$2 WHERE id=$1", rel.ID, other)
	comment := f.Insert(t, "comment", testutil.Cols{"workspace_id": f.WorkspaceID, "release_id": rel.ID, "author_type": "member", "author_id": f.UserID, "content": "For the new owner"})
	if err = s.ProcessReleaseCoordination(ctx, rel.ID); err != nil {
		t.Fatal(err)
	}
	merged, err := f.q.FindPendingReleaseTask(ctx, rel.ID)
	if err != nil {
		t.Fatal(err)
	}
	var runtime pgtype.UUID
	f.QueryRow(t, "SELECT runtime_id FROM agent WHERE id=$1", other).Scan(&runtime)
	if merged.ID != first.ID || merged.AgentID != parseTestUUID(t, other) || merged.RuntimeID != runtime || merged.TriggerCommentID != parseTestUUID(t, comment) {
		t.Fatalf("queued run kept the previous owner: %+v", merged)
	}
}
