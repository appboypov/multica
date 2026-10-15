package service

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// QuestionReach keeps link provenance while projecting the live parent tree.
// IssueID and identifiers describe the original link, not the reached object.
type QuestionReach struct {
	QuestionID      pgtype.UUID
	LinkType        string
	LinkID          pgtype.UUID
	Position        int32
	ObjectType      string
	ObjectID        pgtype.UUID
	ObjectTitle     string
	ObjectStatus    string
	AssigneeType    string
	AssigneeID      pgtype.UUID
	IssueID         pgtype.UUID
	ParentIssueID   pgtype.UUID
	Identifier      string
	IssueIdentifier string
}

func (s *QuestionService) Reach(ctx context.Context, workspaceID pgtype.UUID, questionIDs []pgtype.UUID) ([]QuestionReach, error) {
	return s.reach(ctx, s.Queries, workspaceID, questionIDs)
}

func (s *QuestionService) reach(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID, questionIDs []pgtype.UUID) ([]QuestionReach, error) {
	if len(questionIDs) == 0 {
		return []QuestionReach{}, nil
	}
	rows, err := q.ListQuestionReach(ctx, db.ListQuestionReachParams{WorkspaceID: workspaceID, QuestionIds: questionIDs})
	if err != nil {
		return nil, err
	}
	result := make([]QuestionReach, len(rows))
	for i, row := range rows {
		result[i] = QuestionReach{
			QuestionID: row.QuestionID, LinkType: row.LinkType, LinkID: row.LinkID, Position: row.Position,
			ObjectType: row.ObjectType, ObjectID: row.ObjectID, ObjectTitle: row.ObjectTitle, ObjectStatus: row.ObjectStatus,
			AssigneeType: row.AssigneeType, AssigneeID: row.AssigneeID, IssueID: row.IssueID,
			ParentIssueID: row.ParentIssueID, Identifier: row.Identifier, IssueIdentifier: row.IssueIdentifier,
		}
	}
	return result, nil
}

// QuestionWakeScope selects exactly one place where the agent continues.
type QuestionWakeScope struct {
	IssueID       pgtype.UUID
	ChatSessionID pgtype.UUID
}

func questionTaskScope(task db.AgentTaskQueue) QuestionWakeScope {
	if task.ChatSessionID.Valid {
		return QuestionWakeScope{ChatSessionID: task.ChatSessionID}
	}
	return QuestionWakeScope{IssueID: task.IssueID}
}

func (s *QuestionService) WakeScope(ctx context.Context, workspaceID, agentID pgtype.UUID, scope QuestionWakeScope) ([]pgtype.UUID, error) {
	return s.wakeScope(ctx, s.Queries, workspaceID, agentID, scope)
}

func (s *QuestionService) wakeScope(ctx context.Context, q *db.Queries, workspaceID, agentID pgtype.UUID, scope QuestionWakeScope) ([]pgtype.UUID, error) {
	if scope.IssueID.Valid == scope.ChatSessionID.Valid {
		return nil, questionRefusal(422, "wake requires one issue or chat scope")
	}
	if scope.ChatSessionID.Valid {
		return q.ListChatQuestionIDs(ctx, db.ListChatQuestionIDsParams{WorkspaceID: workspaceID, AgentID: agentID, ChatSessionID: scope.ChatSessionID})
	}
	return q.ListQuestionWakeScope(ctx, db.ListQuestionWakeScopeParams{WorkspaceID: workspaceID, AgentID: agentID, IssueID: scope.IssueID})
}

// CountUnseenAnswers counts answers and cancellations against the canonical
// scope cutoff, including an issue's durable wait's retained anchor. At claim,
// excludeTaskID excludes the task and retry ancestors.
func (s *QuestionService) CountUnseenAnswers(ctx context.Context, workspaceID, agentID pgtype.UUID, scope QuestionWakeScope, excludeTaskID pgtype.UUID) (int64, error) {
	return s.countUnseenAnswers(ctx, s.Queries, workspaceID, agentID, scope, excludeTaskID)
}

func (s *QuestionService) countUnseenAnswers(ctx context.Context, q *db.Queries, workspaceID, agentID pgtype.UUID, scope QuestionWakeScope, excludeTaskID pgtype.UUID) (int64, error) {
	ids, err := s.wakeScope(ctx, q, workspaceID, agentID, scope)
	if err != nil || len(ids) == 0 {
		return 0, err
	}
	count, err := q.CountUnseenQuestionAnswers(ctx, db.CountUnseenQuestionAnswersParams{
		WorkspaceID: workspaceID, QuestionIds: ids, AgentID: agentID,
		IssueID: scope.IssueID, ChatSessionID: scope.ChatSessionID, ExcludeTaskID: excludeTaskID, AnswerAnchor: pgtype.Timestamptz{},
	})
	if err != nil {
		return 0, err
	}
	events, err := q.ListUnseenQuestionEvents(ctx, db.ListUnseenQuestionEventsParams{
		WorkspaceID: workspaceID, QuestionIds: ids, AgentID: agentID,
		IssueID: scope.IssueID, ChatSessionID: scope.ChatSessionID, ExcludeTaskID: excludeTaskID, AnswerAnchor: pgtype.Timestamptz{},
	})
	if err != nil {
		return 0, err
	}
	for _, event := range events {
		if event.Kind == "cancelled" {
			count++
		}
	}
	return count, nil
}

func (s *QuestionService) hasUnseenEvents(ctx context.Context, q *db.Queries, workspaceID, agentID pgtype.UUID, scope QuestionWakeScope) (bool, error) {
	ids, err := s.wakeScope(ctx, q, workspaceID, agentID, scope)
	if err != nil || len(ids) == 0 {
		return false, err
	}
	events, err := q.ListUnseenQuestionEvents(ctx, db.ListUnseenQuestionEventsParams{
		WorkspaceID: workspaceID, QuestionIds: ids, AgentID: agentID,
		IssueID: scope.IssueID, ChatSessionID: scope.ChatSessionID, AnswerAnchor: pgtype.Timestamptz{},
	})
	return len(events) > 0, err
}

func (s *QuestionService) hasActiveWakeTask(ctx context.Context, q *db.Queries, workspaceID, agentID pgtype.UUID, scope QuestionWakeScope) (bool, error) {
	if scope.ChatSessionID.Valid {
		return q.HasActiveChatTaskForAgent(ctx, db.HasActiveChatTaskForAgentParams{WorkspaceID: workspaceID, AgentID: agentID, ChatSessionID: scope.ChatSessionID})
	}
	return q.HasActiveQuestionWakeTask(ctx, db.HasActiveQuestionWakeTaskParams{WorkspaceID: workspaceID, AgentID: agentID, IssueID: scope.IssueID})
}

func (s *QuestionService) WakeTargets(ctx context.Context, workspaceID pgtype.UUID, row db.Question) ([]QuestionWakeTargetDTO, error) {
	return s.wakeTargets(ctx, s.Queries, workspaceID, row)
}

func (s *QuestionService) wakeTargets(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID, row db.Question) ([]QuestionWakeTargetDTO, error) {
	result := make([]QuestionWakeTargetDTO, 0)
	if row.WorkspaceID != workspaceID || !row.AskedInTaskID.Valid {
		return result, nil
	}
	// Any card can wake with unseen answers elsewhere in its target's scope.
	// The scope query excludes ended questions, not the card used to wake.
	asking, err := q.GetAgentTaskInWorkspace(ctx, db.GetAgentTaskInWorkspaceParams{ID: row.AskedInTaskID, WorkspaceID: workspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	askingScope := questionTaskScope(asking)
	if !askingScope.IssueID.Valid && !askingScope.ChatSessionID.Valid {
		return result, nil
	}
	if askingScope.ChatSessionID.Valid {
		if _, err := q.GetChatSessionInWorkspace(ctx, db.GetChatSessionInWorkspaceParams{ID: askingScope.ChatSessionID, WorkspaceID: workspaceID}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return result, nil
			}
			return nil, err
		}
	} else if _, err := q.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: askingScope.IssueID, WorkspaceID: workspaceID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return result, nil
		}
		return nil, err
	}
	reach, err := s.reach(ctx, q, workspaceID, []pgtype.UUID{row.ID})
	if err != nil {
		return nil, err
	}
	type pair struct {
		agent pgtype.UUID
		scope QuestionWakeScope
	}
	pairs := []pair{{row.AskerAgentID, askingScope}}
	seen := map[pair]bool{pairs[0]: true}
	for _, r := range reach {
		if r.AssigneeType != "agent" || !r.AssigneeID.Valid || (askingScope.ChatSessionID.Valid && r.AssigneeID == row.AskerAgentID) {
			continue
		}
		target := pair{agent: r.AssigneeID, scope: QuestionWakeScope{IssueID: asking.IssueID}}
		switch r.ObjectType {
		case "issue":
			target.scope.IssueID = r.ObjectID
		case "project", "autopilot":
			if !target.scope.IssueID.Valid {
				continue
			}
		default:
			continue
		}
		if !seen[target] {
			seen[target] = true
			pairs = append(pairs, target)
		}
	}
	agents := make(map[pgtype.UUID]db.Agent)
	for _, target := range pairs {
		agent, found := agents[target.agent]
		if !found {
			agent, err = q.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: target.agent, WorkspaceID: workspaceID})
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			if err != nil {
				return nil, err
			}
			agents[target.agent] = agent
		}
		if agent.ArchivedAt.Valid {
			continue
		}
		count, err := s.countUnseenAnswers(ctx, q, workspaceID, target.agent, target.scope, pgtype.UUID{})
		if err != nil {
			return nil, err
		}
		active, err := s.hasActiveWakeTask(ctx, q, workspaceID, target.agent, target.scope)
		if err != nil {
			return nil, err
		}
		unseen := count > 0
		if !unseen {
			unseen, err = s.hasUnseenEvents(ctx, q, workspaceID, target.agent, target.scope)
			if err != nil {
				return nil, err
			}
		}
		result = append(result, QuestionWakeTargetDTO{
			AgentID: util.UUIDToString(target.agent), IssueID: util.UUIDToString(target.scope.IssueID),
			ChatSessionID:     util.UUIDToString(target.scope.ChatSessionID),
			UnseenAnswerCount: count, WakeAvailable: unseen && !active && agent.RuntimeID.Valid,
		})
	}
	return result, nil
}
