package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/attribution"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// dispatchRelease is a native subject branch of the existing leased dispatcher.
// It acquires the repository before the rule or queue, like claim and shipping.
func (s *IssueWakeupService) dispatchRelease(ctx context.Context, prev db.IssueWakeup) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	snapshot, err := s.Tasks.Queries.GetReleaseInWorkspace(ctx, db.GetReleaseInWorkspaceParams{ID: prev.ReleaseID, WorkspaceID: prev.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	skip := func(reason string) {
		slog.Info("release coordination skipped", "release_id", util.UUIDToString(snapshot.ID), "wakeup_id", util.UUIDToString(prev.ID), "reason", reason)
	}
	drop := func(reason string, facts int) {
		slog.Warn("release coordination dropped facts", "release_id", util.UUIDToString(snapshot.ID), "wakeup_id", util.UUIDToString(prev.ID), "reason", reason, "facts", facts)
	}
	candidate, _, candidateErr := ResolveReleaseTaskOwner(ctx, s.Tasks.Queries, snapshot)
	if candidateErr != nil && !errors.Is(candidateErr, ErrReleaseOwnerUnavailable) {
		return candidateErr
	}
	preReceipts, err := s.Tasks.Queries.ListPendingWakeupReceipts(ctx, db.ListPendingWakeupReceiptsParams{WakeupID: prev.ID, Revision: prev.Revision})
	if err != nil {
		return err
	}
	preTask, err := s.Tasks.Queries.FindPendingReleaseTask(ctx, snapshot.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var prepared attribution.Result
	var preparedTrigger pgtype.UUID
	var overlay, connectedApps []byte
	if candidate.ID.Valid {
		prepared, preparedTrigger, _, err = s.releaseRunAttribution(ctx, s.Tasks.Queries, preTask, snapshot, candidate, prev, preReceipts)
		if err != nil {
			return err
		}
		overlay, connectedApps = s.Tasks.BuildRuntimeMCPOverlayForMerge(ctx, prepared.UserID, candidate)
	}
	tx, err := s.Tasks.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SET LOCAL lock_timeout = '50ms'"); err != nil {
		return err
	}
	q := s.Tasks.Queries.WithTx(tx)
	if _, err = q.LockReleaseLineOwners(ctx, db.LockReleaseLineOwnersParams{ID: snapshot.RepositoryID, WorkspaceID: snapshot.WorkspaceID}); err != nil {
		return err
	}
	if _, err = q.LockRepositoryForLine(ctx, db.LockRepositoryForLineParams{ID: snapshot.RepositoryID, WorkspaceID: snapshot.WorkspaceID}); err != nil {
		return err
	}
	rel, err := q.GetReleaseInWorkspace(ctx, db.GetReleaseInWorkspaceParams{ID: snapshot.ID, WorkspaceID: snapshot.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	agent, squadID, ownerErr := ResolveReleaseTaskOwner(ctx, q, rel)
	if ownerErr != nil && !errors.Is(ownerErr, ErrReleaseOwnerUnavailable) {
		return ownerErr
	}
	if agent.ID.Valid {
		var fenced bool
		if err = tx.QueryRow(ctx, "SELECT lock_task_owner_rows($1,$2,$3)", agent.ID, pgtype.UUID{}, agent.RuntimeID).Scan(&fenced); err != nil {
			return err
		}
		if !fenced {
			skip("owner rows are being torn down")
			return tx.Commit(ctx)
		}
	}
	w, err := q.LockIssueWakeup(ctx, prev.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	if w.Revision != prev.Revision || w.DisabledAt.Valid || rel.ShippedAt.Valid {
		skip("rule changed, disabled or release shipped")
		return tx.Commit(ctx)
	}
	var now time.Time
	if err = tx.QueryRow(ctx, "SELECT now()").Scan(&now); err != nil {
		return err
	}
	next, enabled, err := s.advanceReleaseWakeup(ctx, tx, q, w, now)
	if err != nil {
		return err
	}
	receipts, err := q.ListPendingWakeupReceipts(ctx, db.ListPendingWakeupReceiptsParams{WakeupID: w.ID, Revision: w.Revision})
	if err != nil {
		return err
	}
	receipts, _, _, err = takenReceipts(ctx, q, receipts)
	if err != nil {
		return err
	}
	advance := func(taskID pgtype.UUID) error {
		return q.AdvanceIssueWakeup(ctx, db.AdvanceIssueWakeupParams{ID: w.ID, Enabled: enabled, NextFireAt: next, LastTaskID: taskID})
	}
	if len(receipts) == 0 {
		if err = advance(w.LastTaskID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if ownerErr != nil {
		drop("release has no runnable owner", len(receipts))
		if err = q.ConsumeWakeupReceipts(ctx, db.ConsumeWakeupReceiptsParams{Ids: receiptIDs(receipts)}); err != nil {
			return err
		}
		if w.Mode == "once" {
			enabled = false
		}
		if err = advance(w.LastTaskID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	pending, err := q.FindPendingReleaseTask(ctx, rel.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err == nil && pending.Status != "queued" {
		// Input is frozen at dispatch; facts remain undelivered for the next turn.
		skip("pending run already dispatched")
		if err = advance(w.LastTaskID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	attr, triggerComment, commentIDs, err := s.releaseRunAttribution(ctx, q, pending, rel, agent, w, receipts)
	if err != nil {
		return err
	}
	if candidate.ID != agent.ID || candidate.RuntimeID != agent.RuntimeID || prepared != attr || preparedTrigger != triggerComment {
		// Credentials were prepared before locks. A newer deliberate trigger
		// keeps its receipts until its complete authority can be prepared.
		skip("owner or trigger changed while preparing the run")
		if err = advance(w.LastTaskID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if !CanMemberInvokeAgent(ctx, q, agent, attr.UserID, rel.WorkspaceID) {
		drop("trigger principal cannot invoke the owner", len(receipts))
		if err = q.ConsumeWakeupReceipts(ctx, db.ConsumeWakeupReceiptsParams{Ids: receiptIDs(receipts)}); err != nil {
			return err
		}
		if err = advance(w.LastTaskID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if !pending.ID.Valid {
		// The limit-th newest run of the last hour ages out first; until then the
		// facts stay pending and the rule rests, so it resumes on its own.
		var limitedSince time.Time
		err = tx.QueryRow(ctx, "SELECT created_at FROM agent_task_queue WHERE release_id=$1 AND created_at>$2 ORDER BY created_at DESC OFFSET $3 LIMIT 1", rel.ID, now.Add(-time.Hour), wakeupHourlyRunLimit-1).Scan(&limitedSince)
		if err == nil {
			resume := limitedSince.Add(time.Hour)
			slog.Info("release coordination deferred by hourly run limit", "release_id", util.UUIDToString(rel.ID), "wakeup_id", util.UUIDToString(w.ID), "limit", wakeupHourlyRunLimit, "resume_at", resume)
			if err = q.AdvanceIssueWakeup(ctx, db.AdvanceIssueWakeupParams{ID: w.ID, Enabled: enabled, NextFireAt: pgtype.Timestamptz{Time: resume, Valid: true}, LastTaskID: w.LastTaskID}); err != nil {
				return err
			}
			return tx.Commit(ctx)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}
	note, evidence := mergeWakeupEvidence(w, pending, receipts)
	context := map[string]any{"release_id": util.UUIDToString(rel.ID), "wakeup_id": util.UUIDToString(w.ID), "wakeup_revision": w.Revision, "wakeup_evidence": evidence, "handoff_note": note, "wakeup_system": w.SystemRule.String}
	task, err := s.Tasks.EnqueueReleaseTaskTx(ctx, q, ReleaseTaskEnqueueParams{Release: rel, Agent: agent, SquadID: squadID, Context: context, Attribution: attr, TriggerCommentID: triggerComment, CommentIDs: commentIDs, ReplaceAttribution: len(commentIDs) > 0, RuntimeMCPOverlay: overlay, ConnectedApps: connectedApps})
	if errors.Is(err, ErrReleaseTaskDispatched) {
		skip("pending run was claimed meanwhile")
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	if err = q.ReserveWakeupReceipts(ctx, db.ReserveWakeupReceiptsParams{Ids: receiptIDs(receipts), TaskID: task.ID}); err != nil {
		return err
	}
	if err = advance(task.ID); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	s.Tasks.broadcastTaskEvent(ctx, protocol.EventTaskQueued, task)
	s.Tasks.NotifyTaskEnqueued(ctx, task)
	return nil
}

func (s *IssueWakeupService) advanceReleaseWakeup(ctx context.Context, tx pgx.Tx, q *db.Queries, w db.IssueWakeup, now time.Time) (pgtype.Timestamptz, bool, error) {
	next, enabled := w.NextFireAt, w.Enabled
	if w.Mode == "once" && w.LastTaskID.Valid {
		last, err := q.GetAgentTask(ctx, w.LastTaskID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return next, enabled, err
		}
		if err == nil && last.StartedAt.Valid {
			return next, false, nil
		}
	}
	timedOut := w.Enabled && w.ExpiresAt.Valid && !w.ExpiresAt.Time.After(now)
	if timedOut {
		enabled, next = false, pgtype.Timestamptz{}
		if w.Kind == "event" && w.OnTimeout.String == "wake" {
			stamp := w.ExpiresAt.Time.UTC().Format(time.RFC3339Nano)
			payload, _ := json.Marshal(map[string]any{"expires_at": stamp, "instruction": w.Instruction})
			if _, err := q.RecordWakeupReceipt(ctx, db.RecordWakeupReceiptParams{ID: dbid.NewV7(), WakeupID: w.ID, Revision: w.Revision, EventKey: "timeout:" + stamp, EventType: wakeupTimeoutEventType, Payload: payload}); err != nil {
				return next, enabled, err
			}
		}
		if err := q.MarkWakeupTimedOut(ctx, w.ID); err != nil {
			return next, enabled, err
		}
	}
	if len(w.Condition) > 0 && string(w.Condition) != "null" && w.Enabled && !timedOut && !w.SystemRule.Valid {
		var err error
		next, err = pollCondition(ctx, tx, q, w, now)
		if err != nil {
			return next, enabled, err
		}
	}
	if !w.Enabled || timedOut || w.Kind == "event" || !next.Valid || next.Time.After(now) {
		return next, enabled, nil
	}
	planned := next.Time
	switch w.Kind {
	case "at":
		enabled, next = false, pgtype.Timestamptz{}
	case "every":
		step := time.Duration(w.IntervalSeconds.Int64) * time.Second
		planned = planned.Add(now.Sub(planned) / step * step)
		next.Time = planned.Add(step)
	case "cron":
		var err error
		next.Time, err = NextOccurrenceAfterUTC(w.CronExpression.String, w.Timezone, now)
		if err != nil {
			return next, enabled, err
		}
		if next.Time.IsZero() {
			next.Valid, enabled = false, false
		}
	}
	payload, _ := json.Marshal(map[string]any{"planned_at": planned.UTC().Format(time.RFC3339Nano), "kind": w.Kind})
	_, err := q.RecordWakeupReceipt(ctx, db.RecordWakeupReceiptParams{ID: dbid.NewV7(), WakeupID: w.ID, Revision: w.Revision, EventKey: planned.UTC().Format(time.RFC3339Nano), EventType: "time.due", Payload: payload})
	return next, enabled, err
}

func (s *IssueWakeupService) releaseRunAttribution(ctx context.Context, q *db.Queries, pending db.AgentTaskQueue, rel db.Release, agent db.Agent, w db.IssueWakeup, receipts []db.IssueWakeupReceipt) (attribution.Result, pgtype.UUID, []pgtype.UUID, error) {
	var attr attribution.Result
	var trigger pgtype.UUID
	var comments []pgtype.UUID
	if pending.ID.Valid {
		attr = attribution.Result{UserID: pending.OriginatorUserID, AccountableUserID: pending.AccountableUserID, Source: attribution.Source(pending.OriginatorSource.String), DelegatedFromTaskID: pending.DelegatedFromTaskID, RuleVersionID: pending.RuleVersionID, EvidenceKind: attribution.EvidenceKind(pending.TriggerEvidenceKind.String), EvidenceRefID: pending.TriggerEvidenceRefID}
		trigger = pending.TriggerCommentID
	} else {
		principal := w.CreatedBy
		if !principal.Valid {
			var err error
			principal, err = q.GetReleaseOwnerOriginator(ctx, db.GetReleaseOwnerOriginatorParams{WorkspaceID: rel.WorkspaceID, ReleaseID: rel.ID})
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return attr, trigger, comments, err
			}
			if !principal.Valid {
				principal = rel.CreatedBy
			}
		}
		attr = attribution.DirectHumanRun(principal, attribution.EvidenceKind("release"), rel.ID)
	}
	if !pending.ID.Valid && w.ParentCommentID.Valid {
		trigger = w.ParentCommentID
	}
	for _, receipt := range receipts {
		if receipt.TaskID.Valid {
			continue
		}
		var fact struct {
			CommentID  string `json:"comment_id"`
			ActorType  string `json:"actor_type"`
			Originator string `json:"originator_user_id"`
		}
		if json.Unmarshal(receipt.Payload, &fact) != nil {
			continue
		}
		if fact.ActorType == "member" && fact.CommentID != "" {
			id, err := util.ParseUUID(fact.CommentID)
			if err != nil {
				return attr, trigger, comments, err
			}
			attr, err = s.Tasks.AttributionForMergedComment(ctx, rel.WorkspaceID, id, false, agent)
			if err != nil {
				return attr, trigger, comments, err
			}
			trigger = id
			comments = append(comments, id)
		} else if receipt.EventType == "release.owner_changed" && !pending.ID.Valid {
			principal, _ := util.ParseUUID(fact.Originator)
			attr = attribution.DirectHumanRun(principal, attribution.EvidenceKind("release"), rel.ID)
		}
	}
	attr, err := s.Tasks.applyAttributionFallback(ctx, attr, agent)
	return attr, trigger, comments, err
}
