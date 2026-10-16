package handler

import (
	"context"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) fetchReleaseCommentsForList(ctx context.Context, args fetchCommentsArgs) (fetchCommentsResult, error) {
	rel := args.Release
	if args.ThreadAnchor != "" {
		anchor, err := util.ParseUUID(args.ThreadAnchor)
		if err != nil {
			return fetchCommentsResult{}, errCommentThreadBadID
		}
		limit := commentHardCap
		if args.ThreadTailSet {
			limit = args.ThreadTail + 1
		}
		rows, err := h.Queries.ListThreadCommentsForReleasePaged(ctx, db.ListThreadCommentsForReleasePagedParams{WorkspaceID: rel.WorkspaceID, ReleaseID: rel.ID, AnchorID: anchor, HasCursor: args.HasCursor, BeforeAt: args.BeforeAt, BeforeID: args.BeforeID, ReplyLimit: int32(limit)})
		if err != nil {
			return fetchCommentsResult{}, err
		}
		if len(rows) == 0 {
			return fetchCommentsResult{}, errCommentThreadNotFound
		}
		result := fetchCommentsResult{Comments: []db.Comment{}}
		var root *db.Comment
		var replies []db.Comment
		for i := range rows {
			if !rows[i].ParentID.Valid {
				root = &rows[i]
			} else {
				replies = append(replies, rows[i])
			}
		}
		hasMore := len(replies) >= limit
		if hasMore {
			replies = replies[1:]
		}
		if root != nil && (args.ThreadTailSet || !args.Since.Valid || root.CreatedAt.Time.After(args.Since.Time)) {
			result.Comments = append(result.Comments, *root)
		}
		for _, reply := range replies {
			if !args.Since.Valid || reply.CreatedAt.Time.After(args.Since.Time) {
				result.Comments = append(result.Comments, reply)
			}
		}
		if args.ThreadTailSet && hasMore && len(replies) > 0 && (!args.Since.Valid || replies[0].CreatedAt.Time.After(args.Since.Time)) {
			result.NextBefore = replies[0].CreatedAt.Time.UTC().Format(time.RFC3339Nano)
			result.NextBeforeID = uuidToString(replies[0].ID)
		}
		if !args.ThreadTailSet {
			result.CommentsTruncated, result.FoldUnsafe = hasMore, hasMore
		}
		return result, nil
	}
	if args.RecentN > 0 {
		rows, err := h.Queries.ListRecentThreadCommentsForRelease(ctx, db.ListRecentThreadCommentsForReleaseParams{WorkspaceID: rel.WorkspaceID, ReleaseID: rel.ID, HasCursor: args.HasCursor, BeforeAt: args.BeforeAt, BeforeID: args.BeforeID, ThreadLimit: int32(args.RecentN)})
		if err != nil {
			return fetchCommentsResult{}, err
		}
		result := fetchCommentsResult{Comments: []db.Comment{}}
		roots := map[pgtype.UUID]bool{}
		for _, row := range rows {
			roots[row.ThreadRootID] = true
			if !args.Since.Valid || row.Comment.CreatedAt.Time.After(args.Since.Time) {
				result.Comments = append(result.Comments, row.Comment)
			}
		}
		if len(rows) > 0 && len(roots) >= args.RecentN && (!args.Since.Valid || rows[0].ThreadLastActivityAt.Time.After(args.Since.Time)) {
			result.NextBefore = rows[0].ThreadLastActivityAt.Time.UTC().Format(time.RFC3339Nano)
			result.NextBeforeID = uuidToString(rows[0].ThreadRootID)
		}
		return result, nil
	}
	if args.RootsOnly {
		rows, err := h.Queries.ListRootCommentsForRelease(ctx, db.ListRootCommentsForReleaseParams{WorkspaceID: rel.WorkspaceID, ReleaseID: rel.ID, RowLimit: commentProbeLimit()})
		if err != nil {
			return fetchCommentsResult{}, err
		}
		result := fetchCommentsResult{Comments: []db.Comment{}, RootStats: map[string]rootStat{}}
		if len(rows) > commentHardCap {
			result.CommentsTruncated = true
			rows = rows[len(rows)-commentHardCap:]
		}
		for _, row := range rows {
			if args.Since.Valid && !row.Comment.CreatedAt.Time.After(args.Since.Time) {
				continue
			}
			result.Comments = append(result.Comments, row.Comment)
			result.RootStats[uuidToString(row.Comment.ID)] = rootStat{ReplyCount: int(row.ReplyCount), LastActivityAt: row.LastActivityAt}
		}
		return result, nil
	}
	rows, err := h.Queries.ListCommentsForRelease(ctx, db.ListCommentsForReleaseParams{WorkspaceID: rel.WorkspaceID, ReleaseID: rel.ID, RowLimit: commentProbeLimit()})
	if err != nil {
		return fetchCommentsResult{}, err
	}
	result := fetchCommentsResult{Comments: []db.Comment{}}
	if len(rows) > commentHardCap {
		result.CommentsTruncated = true
		rows = rows[len(rows)-commentHardCap:]
	}
	if args.Since.Valid {
		for _, row := range rows {
			if row.CreatedAt.Time.After(args.Since.Time) {
				result.Comments = append(result.Comments, row)
			}
		}
		return result, nil
	}
	if !result.CommentsTruncated {
		result.Comments = rows
		return result, nil
	}
	// Complete capped threads as units under the same shared context budget.
	// Every ancestor lookup repeats both workspace and release predicates.
	byID := make(map[pgtype.UUID]db.Comment, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	roots := map[pgtype.UUID]bool{}
	budget := commentHardCap + commentThreadContextBudget
	for _, row := range rows {
		root := row
		complete := true
		for depth := 0; root.ParentID.Valid; depth++ {
			if depth >= commentThreadMaxDepth {
				complete = false
				break
			}
			parent, ok := byID[root.ParentID]
			if !ok {
				parent, err = h.Queries.GetReleaseComment(ctx, db.GetReleaseCommentParams{ID: root.ParentID, WorkspaceID: rel.WorkspaceID, ReleaseID: rel.ID})
				if err != nil {
					return fetchCommentsResult{}, err
				}
				byID[parent.ID] = parent
			}
			root = parent
		}
		if !complete || roots[root.ID] {
			continue
		}
		roots[root.ID] = true
		thread, err := h.Queries.ListThreadCommentsForReleasePaged(ctx, db.ListThreadCommentsForReleasePagedParams{WorkspaceID: rel.WorkspaceID, ReleaseID: rel.ID, AnchorID: root.ID, ReplyLimit: int32(budget)})
		if err != nil {
			return fetchCommentsResult{}, err
		}
		if len(thread) > budget {
			continue
		}
		result.Comments = append(result.Comments, thread...)
		budget -= len(thread)
	}
	sort.Slice(result.Comments, func(i, j int) bool {
		a, b := result.Comments[i], result.Comments[j]
		if a.CreatedAt.Time.Equal(b.CreatedAt.Time) {
			return uuidToString(a.ID) < uuidToString(b.ID)
		}
		return a.CreatedAt.Time.Before(b.CreatedAt.Time)
	})
	return result, nil
}
