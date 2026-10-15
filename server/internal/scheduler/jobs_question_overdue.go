package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
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

// QuestionOverdueJob stamps overdue open questions once and notifies per ask.
// The persisted stamps also cover missed scheduler buckets.
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
			type overdueQuestion struct {
				id       string
				position int32
			}
			type overdueAsk struct {
				workspaceID string
				issueID     string
				chatSessionID string
				askID       string
				questions   []overdueQuestion
			}
			asks := make(map[pgtype.UUID]*overdueAsk)
			var askOrder []*overdueAsk
			// An ask can straddle read batches. Publish only after grouping the
			// sweep, including rows already stamped if a later operation fails.
			defer func() {
				for _, ask := range askOrder {
					slices.SortFunc(ask.questions, func(a, b overdueQuestion) int {
						return int(a.position - b.position)
					})
					ids := make([]string, 0, len(ask.questions))
					for _, question := range ask.questions {
						ids = append(ids, question.id)
					}
					bus.Publish(events.Event{
						Type:        protocol.EventQuestionUpdated,
						WorkspaceID: ask.workspaceID,
						ActorType:   "system",
						Payload: protocol.QuestionEventPayload{
							QuestionID:  ask.askID,
							AskID:       ask.askID,
							QuestionIDs: ids,
							IssueID:     ask.issueID,
							ChatSessionID: ask.chatSessionID,
							Reason:      protocol.QuestionReasonOverdue,
						},
					})
				}
			}()
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
					ask := asks[question.AskID]
					if ask == nil {
						ask = &overdueAsk{
							workspaceID: util.UUIDToString(question.WorkspaceID),
							issueID:     util.UUIDToString(question.IssueID),
							chatSessionID: util.UUIDToString(question.ChatSessionID),
							askID:       util.UUIDToString(question.AskID),
						}
						asks[question.AskID] = ask
						askOrder = append(askOrder, ask)
					}
					ask.questions = append(ask.questions, overdueQuestion{
						id: util.UUIDToString(question.ID), position: question.AskPosition,
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
