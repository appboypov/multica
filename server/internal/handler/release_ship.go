package handler

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/releaseline"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ShipRelease records publication performed outside Multica. It shares the
// repository lock with line and issue placement writes, and never publishes
// a tag or a GitHub release itself.
func (h *Handler) ShipRelease(w http.ResponseWriter, r *http.Request) {
	rel, ok := h.loadRelease(w, r)
	if !ok {
		return
	}
	id := uuidToString(rel.ID)
	h.runReleaseLineWrite(w, r, rel.RepositoryID, releaseLineWrite{
		releaseID: rel.ID,
		change: func(line releaseline.Line) (releaseline.Line, *releaseline.Problem) {
			return releaseline.Ship(line, id)
		},
		write: func(ctx context.Context, q *db.Queries, repo db.Repository, before, after releaseline.Line) (map[string]any, error) {
			if _, err := q.LockReleaseIssuesForShipping(ctx, db.LockReleaseIssuesForShippingParams{WorkspaceID: repo.WorkspaceID, ReleaseID: rel.ID}); err != nil {
				return nil, err
			}
			blockers, err := q.ListReleaseShippingBlockers(ctx, db.ListReleaseShippingBlockersParams{WorkspaceID: repo.WorkspaceID, ReleaseID: rel.ID})
			if err != nil {
				return nil, err
			}
			if len(blockers) > 0 {
				issues := make([]IssueRefusalIssue, len(blockers))
				for i, issue := range blockers {
					issues[i] = IssueRefusalIssue{ID: uuidToString(issue.ID), Identifier: issue.Identifier}
				}
				return nil, &issueRefusalError{status: http.StatusConflict, payload: map[string]any{
					"code": "release_not_ready", "error": "every issue must be Done and every linked pull request merged before shipping", "issues": issues,
				}}
			}
			if err := cleanupReleaseCoordination(ctx, q, repo.WorkspaceID, []pgtype.UUID{rel.ID}); err != nil {
				return nil, err
			}
			row, err := q.MarkReleaseShipped(ctx, db.MarkReleaseShippedParams{ID: rel.ID, WorkspaceID: repo.WorkspaceID})
			if err != nil {
				return nil, err
			}
			return map[string]any{"release_id": id, "version": row.Version, "title": row.Title, "shipped_at": timestampToString(row.ShippedAt)}, nil
		},
		action: releaseActionShipped,
		status: http.StatusOK,
	})
}
