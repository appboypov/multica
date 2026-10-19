package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// A sub-issue that turns blocked always reaches someone who can unblock it.
// It wakes the first agent up its parent chain that is not the blocked
// issue's own agent, and not the agent whose run set it blocked: an agent
// that blocked its own step cannot unblock it. When the chain holds no such
// agent, the workspace owner gets an inbox item. Nothing is woken when the
// blocked issue already has a planned wakeup of its own for turning blocked,
// so nobody is woken twice. It cannot be turned off.

// blockedChainLimit bounds the walk up the parent chain.
const blockedChainLimit = 32

// unblockReach is what reaching an unblocker produced, for publishing after
// the transaction commits.
type unblockReach struct {
	task  db.AgentTaskQueue
	inbox *db.InboxItem
	issue string // status of the issue the inbox item names
}

// UnblockNote is the handoff note the woken agent's run receives.
func UnblockNote(child db.Issue, reason string) string {
	return fmt.Sprintf("Sub-issue %q (%s) %s. You are the first agent up its parent chain other than the agent that is stuck on it. "+
		"Read its latest comments for the cause, then clear it: start or unblock the work it waits on, hand it to whoever can, "+
		"or ask the person who must decide. Leave a comment on it saying what you did.",
		child.Title, util.UUIDToString(child.ID), reason)
}

// unblocker returns the first ancestor of child whose wake target is a
// runnable agent outside excluded. ok is false when the chain has none.
func unblocker(ctx context.Context, q *db.Queries, child db.Issue, excluded map[pgtype.UUID]bool) (db.Issue, wakeTarget, bool, error) {
	seen := map[pgtype.UUID]bool{child.ID: true}
	id := child.ParentIssueID
	for i := 0; i < blockedChainLimit && id.Valid && !seen[id]; i++ {
		seen[id] = true
		ancestor, err := q.GetIssue(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			break
		}
		if err != nil {
			return db.Issue{}, wakeTarget{}, false, err
		}
		target, err := resolveWakeTarget(ctx, q, ancestor)
		if err != nil {
			return db.Issue{}, wakeTarget{}, false, err
		}
		if target.Agent.ID.Valid && !excluded[target.Agent.ID] {
			return ancestor, target, true, nil
		}
		id = ancestor.ParentIssueID
	}
	return db.Issue{}, wakeTarget{}, false, nil
}

// stuckAgents are the agents that cannot unblock issue: its own agent (a
// squad's leader for a squad) and the agent of the run that got it stuck.
func stuckAgents(ctx context.Context, q *db.Queries, issue db.Issue, sourceTaskID pgtype.UUID) (map[pgtype.UUID]bool, error) {
	excluded := map[pgtype.UUID]bool{}
	own, err := resolveWakeTarget(ctx, q, issue)
	if err != nil {
		return nil, err
	}
	if own.Agent.ID.Valid {
		excluded[own.Agent.ID] = true
	}
	if issue.AssigneeType.String == "agent" && issue.AssigneeID.Valid {
		excluded[issue.AssigneeID] = true
	}
	if sourceTaskID.Valid {
		task, err := q.GetAgentTask(ctx, sourceTaskID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if err == nil {
			excluded[task.AgentID] = true
		}
	}
	return excluded, nil
}

// reachUnblocker wakes the unblocker of a stuck issue, or tells the workspace
// owner when there is none, through the caller's transaction.
func (s *IssueWakeupService) reachUnblocker(ctx context.Context, q *db.Queries, issue db.Issue, sourceTaskID pgtype.UUID, reason string) (unblockReach, error) {
	excluded, err := stuckAgents(ctx, q, issue, sourceTaskID)
	if err != nil {
		return unblockReach{}, err
	}
	ancestor, target, ok, err := unblocker(ctx, q, issue, excluded)
	if err != nil {
		return unblockReach{}, err
	}
	if ok {
		task, err := s.Tasks.enqueueMentionTaskWithQueries(ctx, q, ancestor, target.Agent.ID, pgtype.UUID{}, nil,
			target.Type == "squad", target.SquadID, false, UnblockNote(issue, reason), pgtype.UUID{}, pgtype.UUID{}, OriginDerived, nil, false)
		if err == nil {
			return unblockReach{task: task}, nil
		}
		if !errors.Is(err, ErrIssueInTriage) {
			return unblockReach{}, err
		}
	}
	owner, err := workspaceOwner(ctx, q, issue.WorkspaceID)
	if err != nil || !owner.Valid {
		return unblockReach{}, err
	}
	details, _ := json.Marshal(map[string]any{"reason": reason, "outcome": "no_unblocker"})
	item, err := q.CreateInboxItem(ctx, db.CreateInboxItemParams{
		ID: dbid.NewV7(), WorkspaceID: issue.WorkspaceID, RecipientType: "member", RecipientID: owner,
		Type: "agent_blocked", Severity: "action_required", IssueID: issue.ID, Title: issue.Title,
		Body:      pgtype.Text{String: "This issue " + reason + " and no agent up its parent chain can unblock it.", Valid: true},
		ActorType: pgtype.Text{String: "system", Valid: true}, Details: details,
	})
	if err != nil {
		return unblockReach{}, err
	}
	return unblockReach{inbox: &item, issue: issue.Status}, nil
}

func workspaceOwner(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID) (pgtype.UUID, error) {
	members, err := q.ListMembersWithUser(ctx, workspaceID)
	if err != nil {
		return pgtype.UUID{}, err
	}
	for _, m := range members {
		if m.Role == "owner" {
			return m.UserID, nil
		}
	}
	return pgtype.UUID{}, nil
}

// reachBlockedChild handles one recorded "blocked" sub-issue event.
func (s *IssueWakeupService) reachBlockedChild(ctx context.Context, q *db.Queries, e db.IssueChildEvent) (unblockReach, bool, error) {
	child, err := q.GetIssue(ctx, e.ChildID)
	if errors.Is(err, pgx.ErrNoRows) {
		return unblockReach{}, false, nil
	}
	if err != nil {
		return unblockReach{}, false, err
	}
	// Unblocked again before this ran: nothing waits.
	if child.Status != "blocked" {
		return unblockReach{}, false, nil
	}
	planned, err := q.HasPlannedBlockedWakeup(ctx, child.ID)
	if err != nil || planned {
		return unblockReach{}, false, err
	}
	reach, err := s.reachUnblocker(ctx, q, child, e.SourceTaskID, "is blocked")
	return reach, err == nil, err
}

// publishUnblockReach announces what reaching an unblocker created, after
// its transaction committed.
func (s *IssueWakeupService) publishUnblockReach(ctx context.Context, reach unblockReach) {
	if reach.task.ID.Valid {
		s.Tasks.broadcastTaskEvent(ctx, protocol.EventTaskQueued, reach.task)
		s.Tasks.NotifyTaskEnqueued(ctx, reach.task)
	}
	if reach.inbox != nil {
		s.publishSystemInbox(*reach.inbox, reach.issue)
	}
}

// reachUnblockerForFailedStart wakes the unblocker of the issue whose run
// failed before it started. Best-effort: the failure itself is already
// recorded, so an error here is logged.
func (s *TaskService) reachUnblockerForFailedStart(ctx context.Context, task db.AgentTaskQueue, failureReason string) {
	if s.TxStarter == nil {
		return
	}
	fail := func(err error) {
		slog.Warn("failed start: reaching the unblocker failed", "task_id", util.UUIDToString(task.ID), "error", err)
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		fail(err)
		return
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	issue, err := q.GetIssue(ctx, task.IssueID)
	if err != nil {
		fail(err)
		return
	}
	reason := "could not start its run"
	if failureReason != "" {
		reason += " (" + failureReason + ")"
	}
	w := &IssueWakeupService{Tasks: s}
	reach, err := w.reachUnblocker(ctx, q, issue, task.ID, reason)
	if err != nil {
		fail(err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		fail(err)
		return
	}
	w.publishUnblockReach(ctx, reach)
}
