package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

// QuestionWakeAnswerAnchor reads the retained selection boundary carried by a
// waiting wake and its retries. Tasks without that context use the normal cutoff.
func QuestionWakeAnswerAnchor(task db.AgentTaskQueue) (pgtype.Timestamptz, error) {
	var contextData struct {
		Anchor *string `json:"question_wake_anchor"`
	}
	if len(task.Context) == 0 {
		return pgtype.Timestamptz{}, nil
	}
	if err := json.Unmarshal(task.Context, &contextData); err != nil {
		return pgtype.Timestamptz{}, fmt.Errorf("read question wake anchor: %w", err)
	}
	if contextData.Anchor == nil {
		return pgtype.Timestamptz{}, nil
	}
	switch *contextData.Anchor {
	case "-infinity":
		return pgtype.Timestamptz{InfinityModifier: pgtype.NegativeInfinity, Valid: true}, nil
	case "infinity":
		return pgtype.Timestamptz{InfinityModifier: pgtype.Infinity, Valid: true}, nil
	}
	anchor, err := time.Parse(time.RFC3339Nano, *contextData.Anchor)
	if err != nil {
		return pgtype.Timestamptz{}, fmt.Errorf("parse question wake anchor: %w", err)
	}
	return pgtype.Timestamptz{Time: anchor, Valid: true}, nil
}

func (s *QuestionService) questionWakeIssue(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID, row db.Question, agentID pgtype.UUID) (pgtype.UUID, error) {
	if !row.AskerAgentID.Valid {
		return pgtype.UUID{}, questionRefusal(422, "a question a member asked wakes no agent")
	}
	targets, err := s.wakeTargets(ctx, q, workspaceID, row)
	if err != nil {
		return pgtype.UUID{}, err
	}
	for _, target := range targets {
		if target.AgentID == util.UUIDToString(agentID) {
			return util.ParseUUID(target.IssueID)
		}
	}
	// Preserve the asker's availability refusal even when it is archived.
	if agentID == row.AskerAgentID && row.AskedInTaskID.Valid {
		asking, err := q.GetAgentTaskInWorkspace(ctx, db.GetAgentTaskInWorkspaceParams{ID: row.AskedInTaskID, WorkspaceID: workspaceID})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return pgtype.UUID{}, err
		}
		if asking.IssueID.Valid {
			return asking.IssueID, nil
		}
	}
	return pgtype.UUID{}, questionRefusal(422, "agent is not offered Wake on this question")
}

func (s *QuestionService) settleQuestionWakeRetries(ctx context.Context, workspaceID, agentID, issueID pgtype.UUID) error {
	candidates, err := s.Queries.ListQuestionWakeRetryCandidates(ctx, db.ListQuestionWakeRetryCandidatesParams{WorkspaceID: workspaceID, IssueID: issueID, AgentID: agentID})
	if err != nil {
		return err
	}
	for _, candidate := range candidates {
		if !retryEligible(candidate.FailureReason.String, candidate) {
			continue
		}
		if _, err := s.Tasks.MaybeRetryFailedTask(ctx, candidate); err != nil {
			return fmt.Errorf("settle question wake retry: %w", err)
		}
	}
	return nil
}

func (s *QuestionService) questionWakeBlocked(ctx context.Context, q *db.Queries, workspaceID, agentID, issueID pgtype.UUID) (bool, error) {
	active, err := q.HasActiveQuestionWakeTask(ctx, db.HasActiveQuestionWakeTaskParams{WorkspaceID: workspaceID, IssueID: issueID, AgentID: agentID})
	if err != nil || active {
		return active, err
	}
	// A failure can commit between settlement and the issue lock. Never take
	// the retry's slot; the next drain settles this newly visible failed tail.
	candidates, err := q.ListQuestionWakeRetryCandidates(ctx, db.ListQuestionWakeRetryCandidatesParams{WorkspaceID: workspaceID, IssueID: issueID, AgentID: agentID})
	if err != nil {
		return false, err
	}
	for _, candidate := range candidates {
		if retryEligible(candidate.FailureReason.String, candidate) {
			return true, nil
		}
	}
	return false, nil
}

type questionWakeDelivery struct {
	result QuestionWakeResult
	task   db.AgentTaskQueue
	row    db.Question
	actor  QuestionActor
	event  string
}

func (s *QuestionService) enqueueQuestionWake(ctx context.Context, tx pgx.Tx, issue db.Issue, row db.Question, actor QuestionActor, agentID pgtype.UUID, count int64, anchor pgtype.Timestamptz) (*questionWakeDelivery, error) {
	q := s.Queries.WithTx(tx)
	task, err := s.Tasks.EnqueueTaskForQuestionWake(ctx, tx, issue, agentID, actor.ID, row.ID, count)
	if err != nil {
		return nil, err
	}
	if anchor.Valid {
		task, err = q.MergeQuestionWakeTaskAnchor(ctx, db.MergeQuestionWakeTaskAnchorParams{ID: task.ID, AnswerAnchor: anchor})
		if err != nil {
			return nil, err
		}
	}
	result := QuestionWakeResult{TaskID: util.UUIDToString(task.ID), AnswerCount: count}
	details, err := json.Marshal(struct {
		AgentID     string `json:"agent_id"`
		AnswerCount int64  `json:"answer_count"`
		TaskID      string `json:"task_id"`
	}{util.UUIDToString(agentID), count, result.TaskID})
	if err != nil {
		return nil, err
	}
	if _, err := q.CreateActivity(ctx, db.CreateActivityParams{ID: dbid.NewV7(), WorkspaceID: issue.WorkspaceID, IssueID: issue.ID, ActorType: pgtype.Text{String: "member", Valid: true}, ActorID: actor.ID, Action: ActivityQuestionWake, Details: details}); err != nil {
		return nil, err
	}
	event, err := s.writeEvent(ctx, q, row, actor, "woken", result)
	if err != nil {
		return nil, err
	}
	if _, err := q.SetQuestionStatus(ctx, db.SetQuestionStatusParams{ID: row.ID, WorkspaceID: row.WorkspaceID, Status: row.Status, FromStatuses: []string{row.Status}}); err != nil {
		return nil, err
	}
	return &questionWakeDelivery{result: result, task: task, row: row, actor: actor, event: event}, nil
}

func (s *QuestionService) publishQuestionWake(ctx context.Context, delivery *questionWakeDelivery) {
	if delivery == nil {
		return
	}
	s.Tasks.BroadcastTaskQueued(ctx, delivery.task)
	s.Tasks.NotifyTaskEnqueued(ctx, delivery.task)
	s.publish(ctx, QuestionDTO{ID: util.UUIDToString(delivery.row.ID), AskID: util.UUIDToString(delivery.row.AskID), WorkspaceID: util.UUIDToString(delivery.row.WorkspaceID)}, delivery.actor, "woken", delivery.event)
	slog.Info("question agent woken", "question_id", util.UUIDToString(delivery.row.ID), "task_id", delivery.result.TaskID, "answer_count", delivery.result.AnswerCount)
}

// drainQuestionWakeLocked shares the transactional consumer with explicit Wake.
// The caller holds the issue and pending locks, in that order.
func (s *QuestionService) drainQuestionWakeLocked(ctx context.Context, tx pgx.Tx, issue db.Issue, pending db.QuestionWakePending) (*questionWakeDelivery, error) {
	q := s.Queries.WithTx(tx)
	remove := func() error {
		return q.DeleteQuestionWakePending(ctx, db.DeleteQuestionWakePendingParams{WorkspaceID: pending.WorkspaceID, IssueID: pending.IssueID, AgentID: pending.AgentID})
	}
	row, err := q.GetQuestionForUpdate(ctx, db.GetQuestionForUpdateParams{ID: pending.SourceQuestionID, WorkspaceID: pending.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, remove()
	}
	if err != nil {
		return nil, err
	}
	agent, err := q.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: pending.AgentID, WorkspaceID: pending.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, remove()
	}
	if err != nil {
		return nil, err
	}
	if agent.ArchivedAt.Valid || !agent.RuntimeID.Valid {
		return nil, nil
	}
	target, err := s.questionWakeIssue(ctx, q, pending.WorkspaceID, row, pending.AgentID)
	var refusal *QuestionError
	if errors.As(err, &refusal) || (err == nil && target != issue.ID) {
		return nil, remove()
	}
	if err != nil {
		return nil, err
	}
	blocked, err := s.questionWakeBlocked(ctx, q, pending.WorkspaceID, pending.AgentID, pending.IssueID)
	if err != nil || blocked {
		return nil, err
	}
	count, err := s.countUnseenAnswers(ctx, q, pending.WorkspaceID, pending.AgentID, pending.IssueID, pgtype.UUID{})
	if err != nil {
		return nil, err
	}
	askBacks, err := s.hasUnseenAskBacks(ctx, q, pending.WorkspaceID, pending.AgentID, pending.IssueID)
	if err != nil {
		return nil, err
	}
	if count == 0 && !askBacks {
		return nil, remove()
	}
	delivery, err := s.enqueueQuestionWake(ctx, tx, issue, row, QuestionActor{Type: "member", ID: pending.ActorUserID}, pending.AgentID, count, pending.AnswerAnchor)
	if err != nil {
		return nil, err
	}
	if err := remove(); err != nil {
		return nil, err
	}
	return delivery, nil
}

// DrainQuestionWake reconciles a durable wait after terminal side effects, or
// from recovery. Failed transactions leave the intent available for retry.
func (s *QuestionService) DrainQuestionWake(ctx context.Context, workspaceID, agentID, issueID pgtype.UUID) error {
	if err := s.settleQuestionWakeRetries(ctx, workspaceID, agentID, issueID); err != nil {
		return err
	}
	tx, acquired, err := s.beginQuestionWakeDelivery(ctx, issueID)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if !acquired {
		return nil
	}
	q := s.Queries.WithTx(tx)
	issue, err := q.LockIssueForDescriptionUpdate(ctx, db.LockIssueForDescriptionUpdateParams{ID: issueID, WorkspaceID: workspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		if err := q.DeleteQuestionWakePending(ctx, db.DeleteQuestionWakePendingParams{WorkspaceID: workspaceID, IssueID: issueID, AgentID: agentID}); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	pending, err := q.GetQuestionWakePendingForUpdate(ctx, db.GetQuestionWakePendingForUpdateParams{WorkspaceID: workspaceID, IssueID: issueID, AgentID: agentID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	delivery, err := s.drainQuestionWakeLocked(ctx, tx, issue, pending)
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.publishQuestionWake(ctx, delivery)
	return nil
}

// ReconcilePendingQuestionWakes visits durable waits in bounded database pages.
// Cursor pagination prevents unavailable old waits starving later ready pairs.
func (s *QuestionService) ReconcilePendingQuestionWakes(ctx context.Context, limit int32) error {
	if limit <= 0 {
		return errors.New("question wake reconciliation requires a positive batch size")
	}
	params := db.ListQuestionWakePendingParams{MaxRows: limit}
	var failures []error
	for {
		rows, err := s.Queries.ListQuestionWakePending(ctx, params)
		if err != nil {
			return errors.Join(append(failures, err)...)
		}
		for _, row := range rows {
			if err := s.DrainQuestionWake(ctx, row.WorkspaceID, row.AgentID, row.IssueID); err != nil {
				slog.Error("reconcile pending question wake", "issue_id", util.UUIDToString(row.IssueID), "agent_id", util.UUIDToString(row.AgentID), "error", err)
				failures = append(failures, err)
			}
		}
		if len(rows) < int(limit) {
			return errors.Join(failures...)
		}
		last := rows[len(rows)-1]
		params.AfterCreatedAt, params.AfterWorkspaceID, params.AfterIssueID, params.AfterAgentID = last.CreatedAt, last.WorkspaceID, last.IssueID, last.AgentID
	}
}
