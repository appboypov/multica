package service

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/question"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (s *QuestionService) AddLinks(ctx context.Context, workspaceID, questionID pgtype.UUID, actor QuestionActor, refs []question.LinkRef) (QuestionDTO, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return QuestionDTO{}, err
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	if _, err := s.requireAgentTask(ctx, q, workspaceID, actor); err != nil {
		return QuestionDTO{}, err
	}
	row, err := q.GetQuestionForUpdate(ctx, db.GetQuestionForUpdateParams{ID: questionID, WorkspaceID: workspaceID})
	if err != nil {
		return QuestionDTO{}, questionLoadError(err)
	}
	if row.AskerAgentID != actor.ID {
		return QuestionDTO{}, questionRefusal(403, "only the asking agent adds links")
	}
	if row.Status == "closed" || row.Status == "withdrawn" {
		return QuestionDTO{}, questionRefusal(422, "closed or withdrawn questions cannot receive links")
	}
	links, err := s.resolveLinks(ctx, q, workspaceID, refs)
	if err != nil {
		return QuestionDTO{}, err
	}
	for _, link := range links {
		_, err := q.AppendQuestionLinks(ctx, db.AppendQuestionLinksParams{WorkspaceID: workspaceID, QuestionID: questionID, LinkType: link.Type, LinkID: link.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return QuestionDTO{}, err
		}
		if _, err := s.writeEvent(ctx, q, row, actor, "linked", struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		}{link.Type, util.UUIDToString(link.ID)}); err != nil {
			return QuestionDTO{}, err
		}
	}
	result, err := s.hydrate(ctx, q, workspaceID, []db.Question{row})
	if err != nil {
		return QuestionDTO{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return QuestionDTO{}, err
	}
	s.publish(ctx, result[0], actor, "linked", "")
	return result[0], nil
}

func (s *QuestionService) PublishLinkChanges(ctx context.Context, workspaceID pgtype.UUID, questionIDs []pgtype.UUID) {
	seen := make(map[pgtype.UUID]struct{}, len(questionIDs))
	for _, id := range questionIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		row, err := s.Get(ctx, workspaceID, id)
		if err != nil {
			slog.Error("question link change publication failed", "question_id", util.UUIDToString(id), "error", err)
			continue
		}
		s.publish(ctx, row, QuestionActor{Type: "system"}, "linked", "")
	}
}
