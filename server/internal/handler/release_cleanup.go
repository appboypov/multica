package handler

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// cleanupReleaseCoordination runs after issue placement locks, while the caller
// holds each affected repository. A started publisher survives shipping.
func cleanupReleaseCoordination(ctx context.Context, q *db.Queries, ws pgtype.UUID, ids []pgtype.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	for _, id := range ids {
		if _, err := q.GetReleaseInWorkspaceForUpdate(ctx, db.GetReleaseInWorkspaceForUpdateParams{ID: id, WorkspaceID: ws}); err != nil {
			return err
		}
	}
	cancelled, err := q.CancelUnstartedReleaseTasks(ctx, db.CancelUnstartedReleaseTasksParams{WorkspaceID: ws, ReleaseIds: ids, Reason: "release coordination ended"})
	if err != nil {
		return err
	}
	for _, task := range cancelled {
		if err = service.SettleTerminalTaskState(ctx, q, db.AgentTaskQueue(task)); err != nil {
			return err
		}
	}
	return q.DeleteReleaseWakeups(ctx, db.DeleteReleaseWakeupsParams{WorkspaceID: ws, ReleaseIds: ids})
}

func cleanupReleaseDiscussion(ctx context.Context, q *db.Queries, ws pgtype.UUID, ids []pgtype.UUID) ([]string, error) {
	if err := cleanupReleaseCoordination(ctx, q, ws, ids); err != nil {
		return nil, err
	}
	urls, err := q.DeleteReleaseCommentAttachments(ctx, db.DeleteReleaseCommentAttachmentsParams{WorkspaceID: ws, ReleaseIds: ids})
	if err != nil {
		return nil, err
	}
	err = q.DeleteReleaseComments(ctx, db.DeleteReleaseCommentsParams{WorkspaceID: ws, ReleaseIds: ids})
	return urls, err
}
