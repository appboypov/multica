package handler

import (
	"context"
	"strings"
	"testing"
)

// secondTestAgentID creates an agent next to the seeded test agent, on its
// runtime, so a test has an unblocker that is not the stuck agent.
func secondTestAgentID(t *testing.T) string {
	t.Helper()
	var id string
	dbfx.QueryRow(t, `INSERT INTO agent (workspace_id, name, description, runtime_mode, runtime_config, runtime_id, visibility, max_concurrent_tasks, owner_id, instructions, custom_env, custom_args)
		SELECT workspace_id, 'unblocker ' || gen_random_uuid(), '', runtime_mode, '{}'::jsonb, runtime_id, visibility, 1, owner_id, '', '{}'::jsonb, '[]'::jsonb
		FROM agent WHERE id = $1 RETURNING id`, handlerTestAgentID(t)).Scan(&id)
	if id == "" {
		t.Fatal("create second agent")
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE agent_id = $1`, id)
		testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, id)
	})
	return id
}

// The parent's agent is the agent stuck on the child, so the blocked child
// skips it and wakes the next agent up the chain on that agent's issue.
func TestBlockedChildWakesFirstOtherAgentUpTheChain(t *testing.T) {
	fx := newChildDoneFixture(t, "in_progress")
	grand := newChildDoneFixture(t, "in_progress").parent
	dbfx.Exec(t, `UPDATE issue SET parent_issue_id = $2 WHERE id = $1`, fx.parent.ID, grand.ID)
	stuck := handlerTestAgentID(t)
	unblocker := secondTestAgentID(t)
	setIssueAssigneeDirect(t, fx.child.ID, "agent", stuck)
	setIssueAssigneeDirect(t, fx.parent.ID, "agent", stuck)
	setIssueAssigneeDirect(t, grand.ID, "agent", unblocker)

	updateChildStatus(t, fx.child.ID, "blocked")

	if got := countPendingTasksForAgent(t, grand.ID, unblocker); got != 1 {
		t.Fatalf("unblocker runs on the grandparent = %d, want 1", got)
	}
	if got := countPendingTasksForAgent(t, fx.parent.ID, stuck); got != 0 {
		t.Fatalf("stuck agent woken on the parent %d times, want 0", got)
	}
	var note string
	dbfx.QueryRow(t, `SELECT COALESCE(handoff_note, '') FROM agent_task_queue WHERE issue_id = $1 AND agent_id = $2`, grand.ID, unblocker).Scan(&note)
	if note == "" || !containsAll(note, fx.child.Title, fx.child.ID, "is blocked") {
		t.Fatalf("handoff note does not name the blocked sub-issue: %q", note)
	}
}

// A planned wakeup for the child turning blocked is the unblocker; the
// platform wakes nobody on top of it.
func TestBlockedChildWithPlannedWakeupWakesNobodyElse(t *testing.T) {
	fx := newChildDoneFixture(t, "in_progress")
	unblocker := secondTestAgentID(t)
	setIssueAssigneeDirect(t, fx.child.ID, "agent", handlerTestAgentID(t))
	setIssueAssigneeDirect(t, fx.parent.ID, "agent", unblocker)
	var creator string
	dbfx.QueryRow(t, `SELECT user_id FROM member WHERE workspace_id = $1 LIMIT 1`, testWorkspaceID).Scan(&creator)
	dbfx.Exec(t, `INSERT INTO issue_wakeup(id, workspace_id, issue_id, agent_id, created_by, instruction, kind, mode, event_types, timezone, condition)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, 'unblock it', 'event', 'once', ARRAY['issue.status_changed'], 'UTC',
		'{"type":"issue_field","field":"status","value":"blocked"}')`, testWorkspaceID, fx.child.ID, unblocker, creator)

	updateChildStatus(t, fx.child.ID, "blocked")

	if got := countPendingTasksForAgent(t, fx.parent.ID, unblocker); got != 0 {
		t.Fatalf("platform woke the parent's agent %d times next to the planned wakeup", got)
	}
}

// A run that fails before it starts, and is not retried, leaves its issue
// with nobody working on it: the unblocker up the chain is woken.
func TestFailedStartWakesUnblocker(t *testing.T) {
	fx := newChildDoneFixture(t, "in_progress")
	stuck := handlerTestAgentID(t)
	unblocker := secondTestAgentID(t)
	setIssueAssigneeDirect(t, fx.parent.ID, "agent", unblocker)
	setIssueAssigneeDirect(t, fx.child.ID, "agent", stuck)
	var taskID string
	dbfx.QueryRow(t, `INSERT INTO agent_task_queue (agent_id, runtime_id, issue_id, status, priority, dispatched_at)
		SELECT id, runtime_id, $2, 'dispatched', 0, now() FROM agent WHERE id = $1 RETURNING id`, stuck, fx.child.ID).Scan(&taskID)

	if _, err := testHandler.TaskService.FailTask(context.Background(), parseUUID(taskID), "workdir held by another branch", "", "", "", "environment_prepare_failed", false, "", ""); err != nil {
		t.Fatalf("fail task: %v", err)
	}

	if got := countPendingTasksForAgent(t, fx.parent.ID, unblocker); got != 1 {
		t.Fatalf("unblocker runs on the parent = %d, want 1", got)
	}
	var note string
	dbfx.QueryRow(t, `SELECT COALESCE(handoff_note, '') FROM agent_task_queue WHERE issue_id = $1 AND agent_id = $2`, fx.parent.ID, unblocker).Scan(&note)
	if !containsAll(note, fx.child.ID, "could not start its run", "environment_prepare_failed") {
		t.Fatalf("handoff note does not name the failed start: %q", note)
	}
}

// With no other agent up the chain, the workspace owner is told.
func TestBlockedChildWithoutUnblockerNotifiesOwner(t *testing.T) {
	fx := newChildDoneFixture(t, "in_progress")
	stuck := handlerTestAgentID(t)
	setIssueAssigneeDirect(t, fx.child.ID, "agent", stuck)
	setIssueAssigneeDirect(t, fx.parent.ID, "agent", stuck)
	var owner string
	dbfx.QueryRow(t, `SELECT user_id FROM member WHERE workspace_id = $1 AND role = 'owner' LIMIT 1`, testWorkspaceID).Scan(&owner)

	updateChildStatus(t, fx.child.ID, "blocked")

	if n := dbfx.Count(t, `SELECT count(*) FROM inbox_item WHERE recipient_id = $1 AND issue_id = $2 AND type = 'agent_blocked' AND severity = 'action_required'`, owner, fx.child.ID); n != 1 {
		t.Fatalf("owner inbox items for the blocked child = %d, want 1", n)
	}
	if got := countPendingTasksForAgent(t, fx.parent.ID, stuck); got != 0 {
		t.Fatalf("stuck agent woken %d times, want 0", got)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
