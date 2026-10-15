package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/question"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ResolvedQuestionLink is a validated, workspace-local link ready for a write.
type ResolvedQuestionLink struct {
	Type string
	ID   pgtype.UUID
}

func (s *QuestionService) ResolveLinks(ctx context.Context, workspaceID pgtype.UUID, refs []question.LinkRef) ([]ResolvedQuestionLink, error) {
	return s.resolveLinks(ctx, s.Queries, workspaceID, refs)
}

func (s *QuestionService) resolveLinks(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID, refs []question.LinkRef) ([]ResolvedQuestionLink, error) {
	result := make([]ResolvedQuestionLink, 0, len(refs))
	seen := make(map[ResolvedQuestionLink]bool, len(refs))
	for _, ref := range refs {
		id, err := util.ParseUUID(ref.Ref)
		if err != nil || !question.ValidLinkType(ref.Type) {
			return nil, questionRefusal(422, fmt.Sprintf("%s %s is not in this workspace", ref.Type, ref.Ref))
		}
		link := ResolvedQuestionLink{Type: ref.Type, ID: id}
		if seen[link] {
			continue
		}
		switch ref.Type {
		case question.LinkTypeIssue:
			_, err = q.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: id, WorkspaceID: workspaceID})
		case question.LinkTypeComment:
			var comment db.Comment
			comment, err = q.GetCommentInWorkspace(ctx, db.GetCommentInWorkspaceParams{ID: id, WorkspaceID: workspaceID})
			if err == nil && comment.DeletedAt.Valid {
				err = pgx.ErrNoRows
			}
			if err == nil {
				_, err = q.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: comment.IssueID, WorkspaceID: workspaceID})
			}
		case question.LinkTypeTask:
			var task db.AgentTaskQueue
			task, err = q.GetAgentTaskInWorkspace(ctx, db.GetAgentTaskInWorkspaceParams{ID: id, WorkspaceID: workspaceID})
			if err == nil {
				switch {
				case task.ChatSessionID.Valid:
					_, err = q.GetChatSessionInWorkspace(ctx, db.GetChatSessionInWorkspaceParams{ID: task.ChatSessionID, WorkspaceID: workspaceID})
				case task.IssueID.Valid:
					_, err = q.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: task.IssueID, WorkspaceID: workspaceID})
				default:
					err = pgx.ErrNoRows
				}
			}
		case question.LinkTypeChat:
			_, err = q.GetChatSessionInWorkspace(ctx, db.GetChatSessionInWorkspaceParams{ID: id, WorkspaceID: workspaceID})
		case question.LinkTypeProject:
			_, err = q.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{ID: id, WorkspaceID: workspaceID})
		case question.LinkTypeAgent:
			_, err = q.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: id, WorkspaceID: workspaceID})
		case question.LinkTypeSquad:
			_, err = q.GetSquadInWorkspace(ctx, db.GetSquadInWorkspaceParams{ID: id, WorkspaceID: workspaceID})
		case question.LinkTypeAutopilot:
			_, err = q.GetAutopilotInWorkspace(ctx, db.GetAutopilotInWorkspaceParams{ID: id, WorkspaceID: workspaceID})
		case question.LinkTypeSkill:
			_, err = q.GetSkillInWorkspace(ctx, db.GetSkillInWorkspaceParams{ID: id, WorkspaceID: workspaceID})
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, questionRefusal(422, fmt.Sprintf("%s %s is not in this workspace", ref.Type, ref.Ref))
		}
		if err != nil {
			return nil, err
		}
		seen[link] = true
		result = append(result, link)
	}
	return result, nil
}

// QuestionPageObject selects a parent page, or MemberID selects On my work.
type QuestionPageObject struct {
	Type     string
	ID       pgtype.UUID
	MemberID pgtype.UUID
}

type QuestionViaDTO struct {
	Type            string  `json:"type"`
	ID              string  `json:"id"`
	Identifier      *string `json:"identifier"`
	IssueIdentifier *string `json:"issue_identifier"`
	MemberID        *string `json:"member_id,omitempty"`
}

type QuestionOpenIssueDTO struct {
	ID         string  `json:"id"`
	Identifier *string `json:"identifier"`
}

// Via selects the first link that reaches this page, not the first ancestor.
// Pass only one question's reach. A directly linked/owned object has no via.
func Via(reach []QuestionReach, page QuestionPageObject) *QuestionViaDTO {
	var first *QuestionReach
	for i := range reach {
		r := &reach[i]
		matches := r.ObjectType == page.Type && r.ObjectID == page.ID && page.ID.Valid
		if page.MemberID.Valid {
			matches = r.AssigneeType == "member" && r.AssigneeID == page.MemberID
		}
		direct := r.LinkType == r.ObjectType && r.LinkID == r.ObjectID
		firstDirect := first != nil && first.LinkType == first.ObjectType && first.LinkID == first.ObjectID
		if matches && (first == nil || r.Position < first.Position || (r.Position == first.Position && direct && !firstDirect)) {
			first = r
		}
	}
	if first == nil {
		return nil
	}
	if first.LinkType == first.ObjectType && first.LinkID == first.ObjectID {
		return nil
	}
	kind := first.LinkType
	if kind == question.LinkTypeIssue && first.ParentIssueID.Valid {
		kind = "sub_issue"
	}
	switch kind {
	case "issue", "sub_issue", "comment", "task":
		return &QuestionViaDTO{Type: kind, ID: util.UUIDToString(first.LinkID), Identifier: nonemptyQuestionText(first.Identifier), IssueIdentifier: nonemptyQuestionText(first.IssueIdentifier)}
	default:
		return nil
	}
}

func nonemptyQuestionText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// OpenIssue is the first issue-bearing link's issue, never a reached ancestor.
func OpenIssue(reach []QuestionReach) *QuestionOpenIssueDTO {
	var first *QuestionReach
	for i := range reach {
		r := &reach[i]
		if r.IssueID.Valid && (first == nil || r.Position < first.Position) {
			first = r
		}
	}
	if first == nil {
		return nil
	}
	return &QuestionOpenIssueDTO{ID: util.UUIDToString(first.IssueID), Identifier: nonemptyQuestionText(first.IssueIdentifier)}
}

// OpenChat selects the chat link without exposing its title or messages.
func OpenChat(links []QuestionLinkDTO) *QuestionViaDTO {
	for _, link := range links {
		if link.LinkType == question.LinkTypeChat {
			return &QuestionViaDTO{Type: question.LinkTypeChat, ID: link.LinkID, MemberID: link.MemberID}
		}
	}
	return nil
}

// CardIssueIDs is the direct timeline projection, deliberately not ancestors.
func (s *QuestionService) CardIssueIDs(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID, questionIDs []pgtype.UUID) ([]pgtype.UUID, error) {
	reach, err := s.reach(ctx, q, workspaceID, questionIDs)
	if err != nil {
		return nil, err
	}
	ids := make([]pgtype.UUID, 0)
	seen := make(map[pgtype.UUID]bool)
	for _, r := range reach {
		if r.IssueID.Valid && !seen[r.IssueID] {
			seen[r.IssueID] = true
			ids = append(ids, r.IssueID)
		}
	}
	return ids, nil
}
