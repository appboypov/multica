package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const (
	questionOverdueBatchSize  = 500
	questionOverdueMaxBatches = 100
)

// QuestionOverdueJob notifies once when an open question passes its UTC due
// date. The persisted stamp also covers missed scheduler buckets.
func QuestionOverdueJob(queries *db.Queries, bus *events.Bus) JobSpec {
	return JobSpec{
		Name:              "question_overdue",
		Cadence:           time.Hour,
		CatchUpMode:       CatchUpLatestOnly,
		CatchUpWindow:     24 * time.Hour,
		RunTimeout:        10 * time.Minute,
		StaleTimeout:      15 * time.Minute,
		HeartbeatInterval: 30 * time.Second,
		AllowStaleReentry: true,
		MaxAttempts:       3,
		RetryBackoff:      []time.Duration{time.Minute, 5 * time.Minute},
		Scopes:            StaticScopes(ScopeGlobal),
		Handler: func(ctx context.Context, in HandlerInput) (HandlerResult, error) {
			beforeDate := in.PlanTime.UTC().Truncate(24 * time.Hour)
			var total int64
			for range questionOverdueMaxBatches {
				questions, err := queries.ListOpenOverdueQuestions(ctx, db.ListOpenOverdueQuestionsParams{
					BeforeDate: pgtype.Date{Time: beforeDate, Valid: true},
					MaxRows:    questionOverdueBatchSize,
				})
				if err != nil {
					return HandlerResult{RowsAffected: total}, fmt.Errorf("list overdue questions: %w", err)
				}
				for _, question := range questions {
					stamped, err := queries.StampQuestionOverdueNotified(ctx, db.StampQuestionOverdueNotifiedParams{
						ID:          question.ID,
						WorkspaceID: question.WorkspaceID,
					})
					if err != nil {
						return HandlerResult{RowsAffected: total}, fmt.Errorf("stamp overdue question %s: %w", util.UUIDToString(question.ID), err)
					}
					if stamped == 0 {
						continue
					}
					total += stamped
					bus.Publish(events.Event{
						Type:        protocol.EventQuestionUpdated,
						WorkspaceID: util.UUIDToString(question.WorkspaceID),
						ActorType:   "system",
						Payload: protocol.QuestionEventPayload{
							QuestionID: util.UUIDToString(question.ID),
							IssueID:    util.UUIDToString(question.IssueID),
							Reason:     protocol.QuestionReasonOverdue,
						},
					})
				}
				if len(questions) < questionOverdueBatchSize {
					break
				}
				if in.Heartbeat != nil {
					if err := in.Heartbeat(ctx); err != nil {
						return HandlerResult{RowsAffected: total}, err
					}
				}
			}
			if total > 0 {
				slog.Info("scheduler: notified overdue questions", "questions", total, "before_date", beforeDate.Format(time.DateOnly))
			}
			return HandlerResult{RowsAffected: total}, nil
		},
	}
}
