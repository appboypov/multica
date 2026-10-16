package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/attribution"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

var ErrReleaseTaskDispatched = errors.New("release coordination input is already dispatched")

// ReleaseTaskEnqueueParams is prepared before taking the repository boundary:
// computing the optional MCP overlay may perform network I/O. The dispatcher
// rechecks the owner and attribution after acquiring that boundary.
type ReleaseTaskEnqueueParams struct {
	Release            db.Release
	Agent              db.Agent
	SquadID            pgtype.UUID
	Context            map[string]any
	Attribution        attribution.Result
	TriggerCommentID   pgtype.UUID
	CommentIDs         []pgtype.UUID
	ReplaceAttribution bool
	RuntimeMCPOverlay  []byte
	ConnectedApps      []byte
}

// EnqueueReleaseTaskTx shares the caller's repository-before-rule-before-task
// transaction. A release has one pending coordination slot across all rules.
// Only a new deliberate member trigger replaces the complete authority snapshot;
// automated facts can add evidence but cannot inherit another member's apps.
func (s *TaskService) EnqueueReleaseTaskTx(ctx context.Context, q *db.Queries, p ReleaseTaskEnqueueParams) (db.AgentTaskQueue, error) {
	if p.Release.ShippedAt.Valid || p.Agent.WorkspaceID != p.Release.WorkspaceID || !p.Agent.RuntimeID.Valid || !p.Attribution.AccountableUserID.Valid {
		return db.AgentTaskQueue{}, ErrReleaseOwnerUnavailable
	}
	rawContext, err := json.Marshal(p.Context)
	if err != nil {
		return db.AgentTaskQueue{}, fmt.Errorf("encode release task context: %w", err)
	}
	var fields struct {
		Evidence    wakeupEvidence `json:"wakeup_evidence"`
		HandoffNote string         `json:"handoff_note"`
	}
	if err := json.Unmarshal(rawContext, &fields); err != nil {
		return db.AgentTaskQueue{}, err
	}
	comments := releaseTaskCommentIDs(p.CommentIDs, p.TriggerCommentID, fields.Evidence)
	pending, err := q.FindPendingReleaseTask(ctx, p.Release.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return db.AgentTaskQueue{}, err
	}
	source, delegated, evidenceKind, evidenceRef := attributionCreateParams(p.Attribution)
	note := pgtype.Text{String: fields.HandoffNote, Valid: fields.HandoffNote != ""}
	if pending.ID.Valid {
		if pending.Status != "queued" {
			return db.AgentTaskQueue{}, ErrReleaseTaskDispatched
		}
		replace := p.ReplaceAttribution && p.TriggerCommentID.Valid && p.Attribution.Source == attribution.SourceDirectHuman
		trigger := pgtype.UUID{}
		if replace {
			trigger = p.TriggerCommentID
		}
		task, err := q.MergeReleaseTaskInput(ctx, db.MergeReleaseTaskInputParams{
			ID: pending.ID, ReleaseID: p.Release.ID, WakeupContext: rawContext, HandoffNote: note, NewCommentIds: comments, NewTriggerCommentID: trigger, ReplaceAttribution: replace,
			NewOriginatorUserID: p.Attribution.UserID, NewAccountableUserID: p.Attribution.AccountableUserID, NewOriginatorSource: source,
			NewDelegatedFromTaskID: delegated, NewRuleVersionID: p.Attribution.RuleVersionID, NewTriggerEvidenceKind: evidenceKind, NewTriggerEvidenceRefID: evidenceRef,
			NewRuntimeMcpOverlay: p.RuntimeMCPOverlay, NewRuntimeConnectedApps: p.ConnectedApps,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return db.AgentTaskQueue{}, ErrReleaseTaskDispatched
		}
		return task, err
	}
	return q.CreateReleaseWakeupTask(ctx, db.CreateReleaseWakeupTaskParams{
		ID: dbid.NewV7(), WorkspaceID: p.Release.WorkspaceID, ReleaseID: p.Release.ID, AgentID: p.Agent.ID, RuntimeID: p.Agent.RuntimeID,
		TriggerCommentID: p.TriggerCommentID, CoalescedCommentIds: comments, TriggerSummary: pgtype.Text{String: "Release coordination", Valid: true},
		ForceFreshSession: pgtype.Bool{Bool: true, Valid: true}, IsLeaderTask: pgtype.Bool{Bool: p.SquadID.Valid, Valid: true}, SquadID: p.SquadID,
		HandoffNote: note, WakeupContext: rawContext, OriginatorUserID: p.Attribution.UserID, AccountableUserID: p.Attribution.AccountableUserID,
		OriginatorSource: source, DelegatedFromTaskID: delegated, RuleVersionID: p.Attribution.RuleVersionID, TriggerEvidenceKind: evidenceKind, TriggerEvidenceRefID: evidenceRef,
		RuntimeMcpOverlay: p.RuntimeMCPOverlay, RuntimeConnectedApps: p.ConnectedApps,
	})
}

func releaseTaskCommentIDs(explicit []pgtype.UUID, trigger pgtype.UUID, evidence wakeupEvidence) []pgtype.UUID {
	result := make([]pgtype.UUID, 0, len(explicit)+len(evidence.Facts)+1)
	seen := make(map[pgtype.UUID]struct{}, cap(result))
	add := func(id pgtype.UUID) {
		if !id.Valid {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	for _, id := range explicit {
		add(id)
	}
	for _, fact := range evidence.Facts {
		var source struct {
			CommentID string `json:"comment_id"`
		}
		if json.Unmarshal(fact.Payload, &source) != nil || source.CommentID == "" {
			continue
		}
		if id, err := util.ParseUUID(source.CommentID); err == nil {
			add(id)
		}
	}
	add(trigger)
	return result
}

// Claim finalization records the exact delivered set; only the winning start
// consumes it. A replay neither fires the rule again nor loses omitted input.
func consumeReleaseTaskInputs(ctx context.Context, q *db.Queries, task db.AgentTaskQueue) error {
	rules, err := q.ConsumeReleaseTaskReceipts(ctx, db.ConsumeReleaseTaskReceiptsParams{TaskID: task.ID, DeliveredCommentIds: task.DeliveredCommentIds})
	if err != nil {
		return fmt.Errorf("consume release task inputs: %w", err)
	}
	for _, rule := range rules {
		if rule.Enabled && rule.MaxFires.Valid && rule.FireCount >= rule.MaxFires.Int32 {
			if err := q.PauseIssueWakeup(ctx, db.PauseIssueWakeupParams{ID: rule.ID, PausedReason: pgtype.Text{String: wakeupPausedMaxFires, Valid: true}, BlockRuns: false}); err != nil {
				return fmt.Errorf("pause release wakeup at firing limit: %w", err)
			}
		}
	}
	return nil
}

func createRetryTaskForSubject(ctx context.Context, q *db.Queries, parent db.AgentTaskQueue, p db.CreateRetryTaskParams) (db.AgentTaskQueue, error) {
	if !parent.ReleaseID.Valid {
		return q.CreateRetryTask(ctx, p)
	}
	return q.CreateReleaseRetryTask(ctx, db.CreateReleaseRetryTaskParams{
		ParentTaskID: parent.ID, NewTaskID: p.NewTaskID, FireAt: p.FireAt, MaxAttempts: p.MaxAttempts,
		RuntimeMcpOverlay: p.RuntimeMcpOverlay, RuntimeConnectedApps: p.RuntimeConnectedApps,
	})
}
