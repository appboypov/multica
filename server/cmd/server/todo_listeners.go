package main

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/question"
	"github.com/multica-ai/multica/server/internal/todo"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func registerTodoListeners(bus *events.Bus, queries *db.Queries) {
	svc := todo.Service{Queries: queries, Bus: bus}
	for _, kind := range []string{protocol.EventQuestionCreated, protocol.EventQuestionUpdated} {
		bus.Subscribe(kind, func(e events.Event) {
			if err := questionTodos(context.Background(), svc, e); err != nil {
				slog.Error("question todo transition failed", "error", err)
			}
		})
	}
}

func questionTodos(ctx context.Context, svc todo.Service, e events.Event) error {
	p, ok := e.Payload.(protocol.QuestionEventPayload)
	if !ok {
		return nil
	}
	workspace, err := util.ParseUUID(e.WorkspaceID)
	if err != nil {
		return err
	}
	ids := p.QuestionIDs
	if len(ids) == 0 {
		ids = []string{p.QuestionID}
	}
	for _, id := range ids {
		questionID, err := util.ParseUUID(id)
		if err != nil {
			return err
		}
		row, err := svc.Queries.GetQuestion(ctx, db.GetQuestionParams{ID: questionID, WorkspaceID: workspace})
		if err != nil {
			return err
		}
		close := db.CloseTodosParams{WorkspaceID: workspace, Kind: todo.Question, SourceKey: id}
		if !question.IsOpen(row.Status) {
			if err := svc.Close(ctx, close); err != nil {
				return err
			}
			continue
		}
		if p.Reason == protocol.QuestionReasonAnswered || p.Reason == protocol.QuestionReasonReassigned {
			close.MemberID, err = util.ParseUUID(e.ActorID)
			if err != nil {
				return err
			}
			if err := svc.Close(ctx, close); err != nil {
				return err
			}
		}
		if e.Type != protocol.EventQuestionCreated && p.Reason != protocol.QuestionReasonReassigned {
			continue
		}
		answerers, err := svc.Queries.ListQuestionAnswerersByQuestions(ctx, db.ListQuestionAnswerersByQuestionsParams{WorkspaceID: workspace, QuestionIds: []pgtype.UUID{questionID}})
		if err != nil {
			return err
		}
		answers, err := svc.Queries.ListQuestionAnswersByQuestions(ctx, db.ListQuestionAnswersByQuestionsParams{WorkspaceID: workspace, QuestionIds: []pgtype.UUID{questionID}})
		if err != nil {
			return err
		}
		answered := make(map[pgtype.UUID]bool, len(answers))
		for _, answer := range answers {
			answered[answer.AnswererUserID] = true
		}
		var reassignedTo string
		if p.Reason == protocol.QuestionReasonReassigned {
			history, err := svc.Queries.ListQuestionEventsByQuestions(ctx, db.ListQuestionEventsByQuestionsParams{WorkspaceID: workspace, QuestionIds: []pgtype.UUID{questionID}})
			if err != nil {
				return err
			}
			for _, event := range history {
				if util.UUIDToString(event.ID) != p.EventID {
					continue
				}
				var details struct {
					UserID string `json:"user_id"`
				}
				if err := json.Unmarshal(event.Details, &details); err != nil {
					return err
				}
				reassignedTo = details.UserID
			}
		}
		issueID, err := util.ParseUUID(p.IssueID)
		if err != nil {
			return err
		}
		issue, err := svc.Queries.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: issueID, WorkspaceID: workspace})
		if err != nil {
			return err
		}
		for _, answerer := range answerers {
			if p.Reason == protocol.QuestionReasonReassigned && util.UUIDToString(answerer.UserID) != reassignedTo {
				continue
			}
			if answered[answerer.UserID] {
				continue
			}
			if err := svc.Open(ctx, db.OpenTodoParams{WorkspaceID: workspace, MemberID: answerer.UserID, Kind: todo.Question, SourceKey: id, IssueID: issueID, QuestionID: questionID, ActorType: util.StrToText("agent"), ActorID: row.AskerAgentID, Title: issue.Title, Body: util.StrToText(row.Prompt)}); err != nil {
				return err
			}
		}
	}
	return nil
}
