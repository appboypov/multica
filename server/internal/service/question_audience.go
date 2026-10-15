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

func (s *QuestionService) WakeScope(ctx context.Context, workspaceID, agentID, issueID pgtype.UUID) ([]pgtype.UUID, error) {
	return s.wakeScope(ctx, s.Queries, workspaceID, agentID, issueID)
}

func (s *QuestionService) wakeScope(ctx context.Context, q *db.Queries, workspaceID, agentID, issueID pgtype.UUID) ([]pgtype.UUID, error) {
	return q.ListQuestionWakeScope(ctx, db.ListQuestionWakeScopeParams{WorkspaceID: workspaceID, AgentID: agentID, IssueID: issueID})
}

// CountUnseenAnswers uses the canonical pair cutoff, including a durable wait's
// retained anchor. At claim, excludeTaskID excludes the task and retry ancestors.
func (s *QuestionService) CountUnseenAnswers(ctx context.Context, workspaceID, agentID, issueID, excludeTaskID pgtype.UUID) (int64, error) {
	return s.countUnseenAnswers(ctx, s.Queries, workspaceID, agentID, issueID, excludeTaskID)
}

func (s *QuestionService) countUnseenAnswers(ctx context.Context, q *db.Queries, workspaceID, agentID, issueID, excludeTaskID pgtype.UUID) (int64, error) {
	ids, err := s.wakeScope(ctx, q, workspaceID, agentID, issueID)
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	return q.CountUnseenQuestionAnswers(ctx, db.CountUnseenQuestionAnswersParams{
		WorkspaceID: workspaceID, QuestionIds: ids, AgentID: agentID, IssueID: issueID, ExcludeTaskID: excludeTaskID, AnswerAnchor: pgtype.Timestamptz{},
	})
}

// hasUnseenAskBacks tells whether the agent has ask-backs on the issue it has
// not seen, against the same scope and anchor as claim delivery.
func (s *QuestionService) hasUnseenAskBacks(ctx context.Context, q *db.Queries, workspaceID, agentID, issueID pgtype.UUID) (bool, error) {
	ids, err := s.wakeScope(ctx, q, workspaceID, agentID, issueID)
	if err != nil || len(ids) == 0 {
		return false, err
	}
	askBacks, err := q.ListUnseenQuestionEvents(ctx, db.ListUnseenQuestionEventsParams{
		WorkspaceID: workspaceID, QuestionIds: ids, AgentID: agentID, IssueID: issueID, AnswerAnchor: pgtype.Timestamptz{},
	})
	return len(askBacks) > 0, err
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
	if !asking.IssueID.Valid {
		return result, nil
	}
	if _, err := q.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: asking.IssueID, WorkspaceID: workspaceID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return result, nil
		}
		return nil, err
	}
	reach, err := s.reach(ctx, q, workspaceID, []pgtype.UUID{row.ID})
	if err != nil {
		return nil, err
	}
	type pair struct{ agent, issue pgtype.UUID }
	pairs := []pair{{row.AskerAgentID, asking.IssueID}}
	seen := map[pair]bool{pairs[0]: true}
	for _, r := range reach {
		if r.AssigneeType != "agent" || !r.AssigneeID.Valid {
			continue
		}
		target := pair{agent: r.AssigneeID, issue: asking.IssueID}
		switch r.ObjectType {
		case "issue":
			target.issue = r.ObjectID
		case "project", "autopilot":
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
		count, err := s.countUnseenAnswers(ctx, q, workspaceID, target.agent, target.issue, pgtype.UUID{})
		if err != nil {
			return nil, err
		}
		active, err := q.HasActiveQuestionWakeTask(ctx, db.HasActiveQuestionWakeTaskParams{WorkspaceID: workspaceID, AgentID: target.agent, IssueID: target.issue})
		if err != nil {
			return nil, err
		}
		result = append(result, QuestionWakeTargetDTO{
			AgentID: util.UUIDToString(target.agent), IssueID: util.UUIDToString(target.issue),
			UnseenAnswerCount: count, WakeAvailable: count > 0 && !active && agent.RuntimeID.Valid,
		})
	}
	return result, nil
}
