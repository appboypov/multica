package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/releaseline"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// repositoryCodeHasUnshippedReleases refuses a repository that stops
// releasing while its line holds unshipped releases (design.md decision 6).
const repositoryCodeHasUnshippedReleases = "has_unshipped_releases"

// checkLastTag is a passed check's last tag as repository.last_tag stores it:
// without the `v`, or '' when the check found no tag (design.md decision 5).
func checkLastTag(result protocol.RepositoryCheckoutResult) pgtype.Text {
	tag := ""
	if v, ok := releaseline.ParseVersion(result.LastTag); ok {
		tag = v.String()
	}
	return pgtype.Text{String: tag, Valid: true}
}

// backfillRepositoryLastTag stores a passed check's last tag on the
// repositories of its checkout machine and path whose last tag is not known
// yet, and publishes repository:updated for each one it set. A failure only
// logs: the check itself has been recorded.
func (h *Handler) backfillRepositoryLastTag(ctx context.Context, req *RuntimeRepositoryCheckoutRequest, result protocol.RepositoryCheckoutResult) {
	wsUUID, err := util.ParseUUID(req.WorkspaceID)
	if err != nil {
		return
	}
	rows, err := h.Queries.BackfillRepositoryLastTag(ctx, db.BackfillRepositoryLastTagParams{
		LastTag:          checkLastTag(result),
		WorkspaceID:      wsUUID,
		FullName:         req.FullName,
		CheckoutDaemonID: req.DaemonID,
		CheckoutPath:     result.ResolvedPath,
	})
	if err != nil {
		slog.Error("backfill repository last tag failed", "error", err, "workspace_id", req.WorkspaceID, "full_name", req.FullName)
		return
	}
	for _, repo := range rows {
		h.publish(protocol.EventRepositoryUpdated, req.WorkspaceID, "system", "", protocol.RepositoryEventPayload{
			RepositoryID: uuidToString(repo.ID),
			ProjectID:    uuidToString(repo.ProjectID),
		})
	}
}

// refuseStopReleasing locks the repository's line and answers 409
// has_unshipped_releases, naming them, when it holds unshipped releases. It
// reports whether the repository may stop releasing.
func refuseStopReleasing(ctx context.Context, w http.ResponseWriter, qtx *db.Queries, repo db.Repository) bool {
	if _, err := qtx.LockRepositoryForLine(ctx, db.LockRepositoryForLineParams{ID: repo.ID, WorkspaceID: repo.WorkspaceID}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to lock repository")
		return false
	}
	heads, err := qtx.ListUnshippedReleaseHeads(ctx, db.ListUnshippedReleaseHeadsParams{WorkspaceID: repo.WorkspaceID, RepositoryID: repo.ID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read release line")
		return false
	}
	if len(heads) == 0 {
		return true
	}
	releases := make([]map[string]string, len(heads))
	for i, head := range heads {
		releases[i] = map[string]string{"version": head.Version, "title": head.Title}
	}
	writeJSON(w, http.StatusConflict, map[string]any{
		"error":    "remove its unshipped releases first",
		"code":     repositoryCodeHasUnshippedReleases,
		"releases": releases,
	})
	return false
}
