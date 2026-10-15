package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

// DropLinksTo runs on the deleting transaction, before the linked object goes.
// Return every affected question, even when another link keeps it alive, so the
// caller can publish the changed aggregate only after committing that deletion.
func (s *QuestionService) DropLinksTo(ctx context.Context, qtx *db.Queries, workspaceID pgtype.UUID, linkType string, ids []pgtype.UUID) ([]pgtype.UUID, error) {
	if len(ids) == 0 {
		return []pgtype.UUID{}, nil
	}
	questionIDs, err := qtx.DeleteQuestionLinksTo(ctx, db.DeleteQuestionLinksToParams{WorkspaceID: workspaceID, LinkType: linkType, Ids: ids})
	if err != nil {
		return nil, err
	}
	return s.finishLinkDeletion(ctx, qtx, workspaceID, questionIDs)
}

// DropLinksOfIssues also drops links to their comments and runs, including
// tombstoned comments. It must run before those rows are physically removed.
func (s *QuestionService) DropLinksOfIssues(ctx context.Context, qtx *db.Queries, workspaceID pgtype.UUID, issueIDs []pgtype.UUID) ([]pgtype.UUID, error) {
	if len(issueIDs) == 0 {
		return []pgtype.UUID{}, nil
	}
	questionIDs, err := qtx.DeleteQuestionLinksOfIssues(ctx, db.DeleteQuestionLinksOfIssuesParams{WorkspaceID: workspaceID, IssueIds: issueIDs})
	if err != nil {
		return nil, err
	}
	return s.finishLinkDeletion(ctx, qtx, workspaceID, questionIDs)
}

// DropLinksOfChat removes the session and its runs before the chat is deleted.
// Withdrawal is derived only after both kinds of link have been removed.
func (s *QuestionService) DropLinksOfChat(ctx context.Context, qtx *db.Queries, workspaceID, sessionID pgtype.UUID) ([]pgtype.UUID, error) {
	tasks, err := qtx.ListChatTaskIDs(ctx, db.ListChatTaskIDsParams{WorkspaceID: workspaceID, ChatSessionID: sessionID})
	if err != nil {
		return nil, err
	}
	ids, err := qtx.DeleteQuestionLinksTo(ctx, db.DeleteQuestionLinksToParams{
		WorkspaceID: workspaceID, LinkType: "chat", Ids: []pgtype.UUID{sessionID},
	})
	if err != nil {
		return nil, err
	}
	if len(tasks) > 0 {
		runIDs, err := qtx.DeleteQuestionLinksTo(ctx, db.DeleteQuestionLinksToParams{
			WorkspaceID: workspaceID, LinkType: "task", Ids: tasks,
		})
		if err != nil {
			return nil, err
		}
		ids = append(ids, runIDs...)
	}
	return s.finishLinkDeletion(ctx, qtx, workspaceID, ids)
}

func (s *QuestionService) finishLinkDeletion(ctx context.Context, qtx *db.Queries, workspaceID pgtype.UUID, ids []pgtype.UUID) ([]pgtype.UUID, error) {
	unique := make([]pgtype.UUID, 0, len(ids))
	seen := make(map[pgtype.UUID]bool, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	if _, err := s.WithdrawUnlinked(ctx, qtx, workspaceID, unique); err != nil {
		return nil, err
	}
	if err := qtx.PromoteFirstQuestionLinks(ctx, db.PromoteFirstQuestionLinksParams{WorkspaceID: workspaceID, QuestionIds: unique}); err != nil {
		return nil, err
	}
	return unique, nil
}

// WithdrawUnlinked writes status and its history in the caller's transaction.
// Terminal questions stay unchanged; a surviving link prevents withdrawal.
func (s *QuestionService) WithdrawUnlinked(ctx context.Context, qtx *db.Queries, workspaceID pgtype.UUID, questionIDs []pgtype.UUID) ([]pgtype.UUID, error) {
	if len(questionIDs) == 0 {
		return []pgtype.UUID{}, nil
	}
	withdrawn, err := qtx.WithdrawQuestionsWithoutLinks(ctx, db.WithdrawQuestionsWithoutLinksParams{WorkspaceID: workspaceID, QuestionIds: questionIDs})
	if err != nil {
		return nil, err
	}
	for _, id := range withdrawn {
		_, err := qtx.CreateQuestionEvent(ctx, db.CreateQuestionEventParams{
			ID: dbid.NewV7(), WorkspaceID: workspaceID, QuestionID: id,
			Kind: "withdrawn", ActorType: "system", Details: []byte(`{"reason":"links_deleted"}`),
		})
		if err != nil {
			return nil, err
		}
	}
	return withdrawn, nil
}
