package service

import (
	"context"
	"testing"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestWillEnqueueRunStatusTransitions(t *testing.T) {
	pool := sharedTestPool(t)
	ctx := context.Background()
	queries := db.New(pool)
	_, _, agentID, issueID := seedAttributionFixture(t, pool)
	issue, err := queries.GetIssue(ctx, util.MustParseUUID(issueID))
	if err != nil {
		t.Fatal(err)
	}
	svc := &IssueService{Queries: queries, TaskService: &TaskService{Queries: queries}}

	for _, tc := range []struct {
		from string
		to   string
		want bool
	}{
		{"blocked", "todo", true},
		{"blocked", "in_progress", true},
		{"blocked", "in_review", true},
		{"blocked", "backlog", false},
		{"blocked", "done", false},
		{"blocked", "cancelled", false},
		{"blocked", "blocked", false},
		{"backlog", "todo", true},
		{"backlog", "blocked", true},
		{"todo", "in_progress", false},
	} {
		t.Run(tc.from+"_to_"+tc.to, func(t *testing.T) {
			postWrite := issue
			postWrite.Status = tc.to
			trigger, ok := svc.WillEnqueueRun(ctx, IssueTriggerInput{
				Issue: postWrite, PrevStatus: tc.from, StatusChanged: true,
			}, IssueTriggerProbe{})
			if ok != tc.want {
				t.Fatalf("WillEnqueueRun = %v, want %v", ok, tc.want)
			}
			if ok && (trigger.Source != RunSourceStatus || trigger.IssueID != issue.ID ||
				trigger.AgentID != util.MustParseUUID(agentID) || trigger.AssigneeType != "agent") {
				t.Fatalf("unexpected trigger: %+v", trigger)
			}
		})
	}

	issue.Status = "todo"
	input := IssueTriggerInput{Issue: issue, PrevStatus: "blocked", StatusChanged: true}
	t.Run("self_loop", func(t *testing.T) {
		if _, ok := svc.WillEnqueueRun(ctx, input, IssueTriggerProbe{IsSelfLoop: func() bool { return true }}); ok {
			t.Fatal("a self-loop must not enqueue a run")
		}
	})
	t.Run("status_not_changed", func(t *testing.T) {
		unchanged := input
		unchanged.StatusChanged = false
		if _, ok := svc.WillEnqueueRun(ctx, unchanged, IssueTriggerProbe{}); ok {
			t.Fatal("a write without a status change must not enqueue a run")
		}
	})
	t.Run("assignment_to_blocked", func(t *testing.T) {
		assigned := issue
		assigned.Status = "blocked"
		trigger, ok := svc.WillEnqueueRun(ctx, IssueTriggerInput{
			Issue: assigned, PrevStatus: "blocked", AssigneeChanged: true,
		}, IssueTriggerProbe{})
		if !ok || trigger.Source != RunSourceAssign {
			t.Fatalf("assignment must enqueue a run: trigger=%+v, ok=%v", trigger, ok)
		}
	})
}
