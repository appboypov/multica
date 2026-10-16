package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

// CreateRelease preserves the shared schedule, deadline and condition machinery;
// only the subject and dynamically resolved owner differ from issue rules.
func (s *IssueWakeupService) CreateRelease(ctx context.Context, release db.Release, member, source pgtype.UUID, in WakeupInput) (db.IssueWakeup, error) {
	var out db.IssueWakeup
	if in.AgentID != "" || len(in.EventTypes) != 0 || in.FilterAgentID != "" || in.FilterTaskID != "" || in.FilterActorType != "" || in.FilterActorID != "" {
		return out, fmt.Errorf("%w: release wakeups target the current owner and accept schedules or other_issue conditions only", ErrWakeupInput)
	}
	if in.Kind == "event" && !hasWakeupCondition(in.Condition) {
		return out, fmt.Errorf("%w: release event wakeups require an other_issue condition", ErrWakeupInput)
	}
	tx, err := s.Tasks.TxStarter.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	q := s.Tasks.Queries.WithTx(tx)
	if _, err = q.LockReleaseLineOwners(ctx, db.LockReleaseLineOwnersParams{ID: release.RepositoryID, WorkspaceID: release.WorkspaceID}); err != nil {
		return out, err
	}
	if _, err = q.LockRepositoryForLine(ctx, db.LockRepositoryForLineParams{ID: release.RepositoryID, WorkspaceID: release.WorkspaceID}); err != nil {
		return out, err
	}
	release, err = q.GetReleaseInWorkspace(ctx, db.GetReleaseInWorkspaceParams{ID: release.ID, WorkspaceID: release.WorkspaceID})
	if err != nil {
		return out, err
	}
	if release.ShippedAt.Valid {
		return out, fmt.Errorf("%w: release is shipped", ErrWakeupInput)
	}
	membership, err := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: member, WorkspaceID: release.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrWakeupForbidden
	}
	if err != nil {
		return out, err
	}
	_ = membership
	// Ownership may persist without a runtime. Registration is authorized using
	// normal invoke permissions, not transient runtime availability.
	var agent db.Agent
	switch release.OwnerType.String {
	case "agent":
		agent, err = q.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: release.OwnerID, WorkspaceID: release.WorkspaceID})
	case "squad":
		var squad db.Squad
		squad, err = q.GetSquadInWorkspace(ctx, db.GetSquadInWorkspaceParams{ID: release.OwnerID, WorkspaceID: release.WorkspaceID})
		if err == nil {
			agent, err = q.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: squad.LeaderID, WorkspaceID: release.WorkspaceID})
		}
	default:
		return out, ErrWakeupForbidden
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrWakeupForbidden
	}
	if err != nil {
		return out, err
	}
	if !CanMemberInvokeAgent(ctx, q, agent, member, release.WorkspaceID) {
		return out, ErrWakeupForbidden
	}
	if source.Valid {
		task, err := q.GetAgentTask(ctx, source)
		if err != nil {
			return out, err
		}
		if task.ReleaseID != release.ID || task.AgentID != agent.ID || task.OriginatorUserID != member {
			return out, ErrWakeupForbidden
		}
	}
	var parentID pgtype.UUID
	if in.ParentCommentID != "" {
		parentID, err = util.ParseUUID(in.ParentCommentID)
		if err != nil {
			return out, fmt.Errorf("%w: invalid parent_comment_id", ErrWakeupInput)
		}
		parent, parentErr := q.GetReleaseComment(ctx, db.GetReleaseCommentParams{ID: parentID, WorkspaceID: release.WorkspaceID, ReleaseID: release.ID})
		if parentErr != nil || parent.DeletedAt.Valid {
			return out, fmt.Errorf("%w: parent comment must belong to this release", ErrWakeupInput)
		}
	}
	var now time.Time
	if err = tx.QueryRow(ctx, "SELECT now()").Scan(&now); err != nil {
		return out, err
	}
	next, err := s.Validate(&in, now)
	if err != nil {
		return out, err
	}
	expires, expirySeconds, err := s.Expiry(&in, now)
	if err != nil {
		return out, err
	}
	if hasWakeupCondition(in.Condition) {
		in.Condition, err = validateReleaseCondition(ctx, tx, release.WorkspaceID, in.Condition)
		if err != nil {
			return out, err
		}
		next = pgtype.Timestamptz{Time: now, Valid: true}
	}
	var maxFires pgtype.Int4
	if in.Mode == "continuous" {
		maxFires = pgtype.Int4{Int32: in.MaxFires, Valid: true}
		if maxFires.Int32 == 0 {
			maxFires.Int32 = wakeupDefaultMaxFires
		}
	}
	out, err = q.CreateReleaseWakeup(ctx, db.CreateReleaseWakeupParams{ID: dbid.NewV7(), WorkspaceID: release.WorkspaceID, ReleaseID: release.ID, CreatedBy: member, SourceTaskID: source, ParentCommentID: parentID,
		Instruction: in.Instruction, Kind: in.Kind, Mode: in.Mode, EventTypes: []string{}, IntervalSeconds: pgtype.Int8{Int64: in.IntervalSeconds, Valid: in.IntervalSeconds > 0}, CronExpression: pgtype.Text{String: in.CronExpression, Valid: in.CronExpression != ""}, Timezone: in.Timezone,
		NextFireAt: next, ExpiresAt: expires, ExpirySeconds: expirySeconds, OnTimeout: pgtype.Text{String: in.OnTimeout, Valid: in.OnTimeout != ""}, Condition: in.Condition, MaxFires: maxFires})
	if err != nil {
		return out, err
	}
	if err = tx.Commit(ctx); err != nil {
		return out, err
	}
	return out, nil
}

func (s *IssueWakeupService) DeleteRelease(ctx context.Context, release db.Release, id, member pgtype.UUID) error {
	tx, err := s.Tasks.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.Tasks.Queries.WithTx(tx)
	if _, err = q.LockReleaseLineOwners(ctx, db.LockReleaseLineOwnersParams{ID: release.RepositoryID, WorkspaceID: release.WorkspaceID}); err != nil {
		return err
	}
	if _, err = q.LockRepositoryForLine(ctx, db.LockRepositoryForLineParams{ID: release.RepositoryID, WorkspaceID: release.WorkspaceID}); err != nil {
		return err
	}
	if _, err = q.GetReleaseInWorkspace(ctx, db.GetReleaseInWorkspaceParams{ID: release.ID, WorkspaceID: release.WorkspaceID}); err != nil {
		return err
	}
	rule, err := q.LockIssueWakeup(ctx, id)
	if err != nil {
		return err
	}
	if rule.ReleaseID != release.ID || rule.WorkspaceID != release.WorkspaceID {
		return pgx.ErrNoRows
	}
	if rule.SystemRule.Valid {
		return ErrWakeupForbidden
	}
	membership, err := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: member, WorkspaceID: release.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrWakeupForbidden
	}
	if err != nil {
		return err
	}
	if rule.CreatedBy != member && membership.Role != "owner" && membership.Role != "admin" {
		return ErrWakeupForbidden
	}
	cancelled, err := q.CancelUnstartedReleaseWakeupTasks(ctx, db.CancelUnstartedReleaseWakeupTasksParams{ReleaseID: release.ID, WakeupID: id})
	if err != nil {
		return err
	}
	for _, task := range cancelled {
		if err = SettleTerminalTaskState(ctx, q, db.AgentTaskQueue(task)); err != nil {
			return err
		}
	}
	if _, err = q.DeleteReleaseWakeup(ctx, db.DeleteReleaseWakeupParams{ID: id, WorkspaceID: release.WorkspaceID, ReleaseID: release.ID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
