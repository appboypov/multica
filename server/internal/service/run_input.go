package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// RunInputActor is the authenticated author of an item, not its recipient.
type RunInputActor struct {
	Type string
	ID   pgtype.UUID
}

// RunInputItem snapshots an item once, with one identity shared by all recipients.
// CommentID is set for both original comments and their edits.
type RunInputItem struct {
	WorkspaceID pgtype.UUID
	Kind        string
	ID          pgtype.UUID
	CommentID   pgtype.UUID
	Content     string
}

type RunInputNotifier interface {
	NotifyTaskSupplementAvailable(runtimeID, taskID string)
}

// RunInputService delivers notices without queuing or interrupting runs.
type RunInputService struct {
	Queries  *db.Queries
	Notifier RunInputNotifier
}

// Deliver returns the exact tasks that have a receipt for this item. Each insert
// locks its task before its receipt, like CreateTaskSupplement and settlement;
// a run that ended after selection gets no new receipt. Inserts commit before
// their daemon hints are sent. A failure for one recipient does not hide others.
func (s *RunInputService) Deliver(ctx context.Context, issueIDs []pgtype.UUID, actor RunInputActor, item RunInputItem, excludeTaskIDs []pgtype.UUID) ([]pgtype.UUID, error) {
	if len(issueIDs) == 0 {
		return nil, nil
	}
	excludeAgentID := pgtype.UUID{}
	if actor.Type == "agent" {
		excludeAgentID = actor.ID
	}
	tasks, err := s.Queries.ListRunningSupplementTasksForIssues(ctx, db.ListRunningSupplementTasksForIssuesParams{
		WorkspaceID: item.WorkspaceID, IssueIds: issueIDs,
		ExcludeAgentID: excludeAgentID, ExcludeTaskIds: excludeTaskIDs,
	})
	if err != nil {
		slog.Error("run input recipient lookup failed", "item_id", util.UUIDToString(item.ID), "item_kind", item.Kind, "error", err)
		return nil, err
	}
	reached := make([]pgtype.UUID, 0, len(tasks))
	var deliveryErr error
	for _, task := range tasks {
		_, err := s.Queries.CreateRunInputSupplement(ctx, db.CreateRunInputSupplementParams{
			TaskID: task.ID, IssueID: task.IssueID, WorkspaceID: item.WorkspaceID,
			AuthorID: actor.ID, ItemKind: item.Kind, ItemID: item.ID,
			CommentID: item.CommentID, Content: pgtype.Text{String: item.Content, Valid: true},
		})
		if errors.Is(err, pgx.ErrNoRows) {
			// ON CONFLICT is idempotent. An existing receipt still means this
			// exact run received the item; a terminal-race loser has none.
			_, err = s.Queries.GetTaskSupplementForRun(ctx, db.GetTaskSupplementForRunParams{
				TaskID: task.ID, ItemID: item.ID, WorkspaceID: item.WorkspaceID,
			})
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
		}
		if err != nil {
			slog.Error("run input receipt failed", "item_id", util.UUIDToString(item.ID), "item_kind", item.Kind, "task_id", util.UUIDToString(task.ID), "error", err)
			deliveryErr = errors.Join(deliveryErr, fmt.Errorf("task %s: %w", util.UUIDToString(task.ID), err))
			continue
		}
		reached = append(reached, task.ID)
		slog.Debug("run input receipt committed", "item_id", util.UUIDToString(item.ID), "item_kind", item.Kind, "issue_id", util.UUIDToString(task.IssueID), "task_id", util.UUIDToString(task.ID), "runtime_id", util.UUIDToString(task.RuntimeID))
		if s.Notifier != nil && task.RuntimeID.Valid {
			s.Notifier.NotifyTaskSupplementAvailable(util.UUIDToString(task.RuntimeID), util.UUIDToString(task.ID))
		}
	}
	slog.Info("run input delivered", "workspace_id", util.UUIDToString(item.WorkspaceID), "item_id", util.UUIDToString(item.ID), "item_kind", item.Kind, "actor_type", actor.Type, "actor_id", util.UUIDToString(actor.ID), "recipient_count", len(reached))
	return reached, deliveryErr
}
