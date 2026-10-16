package service

import (
	"bytes"
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// BeginQuestionWakeTransition keeps accepted wakes waiting through an issue's
// cancellation and deletion/replacement, including failure recovery. The caller
// defers finish until the enclosing operation has settled. Shared advisory locks
// permit nested transitions and do not lock issue rows needed by their writes.
func (s *QuestionService) BeginQuestionWakeTransition(ctx context.Context, workspaceID pgtype.UUID, issueIDs []pgtype.UUID) (finish func(), err error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, err
	}
	ids := slices.Clone(issueIDs)
	slices.SortFunc(ids, func(a, b pgtype.UUID) int { return bytes.Compare(a.Bytes[:], b.Bytes[:]) })
	ids = slices.Compact(ids)
	q := s.Queries.WithTx(tx)
	for _, id := range ids {
		if err := q.LockQuestionWakeTransition(ctx, id); err != nil {
			rollbackQuestionWakeGuard(tx)
			return nil, err
		}
	}
	return func() {
		rollbackQuestionWakeGuard(tx)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		pending, err := s.Queries.ListQuestionWakePendingForIssues(ctx, db.ListQuestionWakePendingForIssuesParams{WorkspaceID: workspaceID, IssueIds: ids})
		if err != nil {
			slog.Error("list question wakes after lifecycle transition", "error", err)
			return
		}
		for _, row := range pending {
			if err := s.DrainQuestionWake(ctx, row.WorkspaceID, row.AgentID, row.IssueID); err != nil {
				slog.Error("drain question wake after lifecycle transition", "error", err)
			}
		}
	}, nil
}

// A delivery takes an exclusive nonblocking lock. Without it, explicit Wake can
// persist acceptance in the returned transaction but cannot consume the intent.
func (s *QuestionService) beginQuestionWakeDelivery(ctx context.Context, issueID pgtype.UUID) (pgx.Tx, bool, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	q := s.Queries.WithTx(tx)
	acquired, err := q.TryLockQuestionWakeDelivery(ctx, issueID)
	if err != nil {
		rollbackQuestionWakeGuard(tx)
		return nil, false, err
	}
	return tx, acquired, nil
}

func rollbackQuestionWakeGuard(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tx.Rollback(ctx); err != nil {
		slog.Error("release question wake lifecycle guard", "error", err)
	}
}
