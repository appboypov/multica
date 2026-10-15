package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/question"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// QuestionRowFilters holds repeated list filters. Empty lists and false
// BlocksWork leave the corresponding filter unset.
type QuestionRowFilters struct {
	AnswerTypes []string
	AskerIDs    []pgtype.UUID
	AnswererIDs []pgtype.UUID
	LinkIDs     []pgtype.UUID
	BlocksWork  bool
	Parent      *QuestionPageObject
}

// QuestionRowCursor is the last returned row's sort key. The handler binds it
// to the workspace, viewer, tab and filters in its base64 JSON cursor scope.
// Time is created_at for open tabs and my_answer.updated_at for Answered.
type QuestionRowCursor struct {
	Time time.Time
	ID   pgtype.UUID
}

type QuestionRowLinkDTO struct {
	Type       string  `json:"type"`
	ID         string  `json:"id"`
	Identifier string  `json:"identifier"`
	Title      string  `json:"title"`
	Status     string  `json:"status"`
	MemberID   *string `json:"member_id,omitempty"`
}

type QuestionRowAnswerDTO struct {
	Display   string `json:"display"`
	UpdatedAt string `json:"updated_at"`
}

// QuestionRowDTO is the list projection, not the full question card aggregate.
type QuestionRowDTO struct {
	ID           string                `json:"id"`
	AskID        string                `json:"ask_id"`
	AskPosition  int32                 `json:"ask_position"`
	AskTitle     *string               `json:"ask_title"`
	Prompt       string                `json:"prompt"`
	AnswerType   string                `json:"answer_type"`
	Settings     json.RawMessage       `json:"settings"`
	Status       string                `json:"status"`
	AnswerPolicy string                `json:"answer_policy"`
	AskerAgentID string                `json:"asker_agent_id"`
	AskerUserID  string                `json:"asker_user_id"`
	AnswererIDs  []string              `json:"answerer_ids"`
	DueDate      *string               `json:"due_date"`
	BlocksWork   bool                  `json:"blocks_work"`
	CreatedAt    string                `json:"created_at"`
	Link         QuestionRowLinkDTO    `json:"link"`
	MyAnswer     *QuestionRowAnswerDTO `json:"my_answer"`
	Via          *QuestionViaDTO       `json:"via"`
	OpenIssue    *QuestionOpenIssueDTO `json:"open_issue"`
}

// QuestionRowPageDTO keeps the cursor typed; the handler encodes it for the
// wire. NextCursor is nil on the last page and Rows is never nil on success.
type QuestionRowPageDTO struct {
	Rows       []QuestionRowDTO
	NextCursor *QuestionRowCursor
}

// ListRows reads one viewer-scoped page. Membership and request parsing belong
// to the handler; SQL owns tab eligibility, filters and ordering (ADR-0006).
func (s *QuestionService) ListRows(ctx context.Context, workspaceID, viewerID pgtype.UUID, tab string, filters QuestionRowFilters, cursor *QuestionRowCursor, limit int) (QuestionRowPageDTO, error) {
	if limit < 1 || limit > 100 {
		return QuestionRowPageDTO{}, questionRefusal(400, "limit must be between 1 and 100")
	}
	params := db.ListQuestionRowsForMemberParams{
		WorkspaceID: workspaceID, ViewerID: viewerID,
		AnswerType: filters.AnswerTypes, AskerIds: filters.AskerIDs,
		AnswererUserID: filters.AnswererIDs, LinkIds: filters.LinkIDs,
		BlocksWork: pgtype.Bool{Bool: filters.BlocksWork, Valid: filters.BlocksWork},
		Limit:      int32(limit + 1),
	}
	if cursor != nil {
		params.CursorTime = pgtype.Timestamptz{Time: cursor.Time, Valid: true}
		params.CursorID = cursor.ID
	}

	var page QuestionRowPageDTO
	var questionIDs []pgtype.UUID
	rowTab := tab
	if filters.Parent != nil {
		if tab != "all_open" {
			return page, questionRefusal(400, "parent requires tab=all_open")
		}
		rowTab = "on_my_work"
	}
	switch rowTab {
	case "on_my_work":
		issueIDs, projectIDs, err := s.questionWorkSeeds(ctx, workspaceID, viewerID, filters.Parent)
		if err != nil {
			return page, err
		}
		rows, err := s.Queries.ListQuestionRowsBelow(ctx, db.ListQuestionRowsBelowParams{
			WorkspaceID: workspaceID, ViewerID: viewerID,
			IssueIds: issueIDs, ProjectIds: projectIDs,
			AnswerType: params.AnswerType, AskerIds: params.AskerIds,
			AnswererUserID: params.AnswererUserID, LinkIds: params.LinkIds,
			BlocksWork: params.BlocksWork, CursorTime: params.CursorTime,
			CursorID: params.CursorID, Limit: params.Limit,
		})
		if err != nil {
			return page, fmt.Errorf("list questions below work: %w", err)
		}
		if len(rows) > limit {
			rows = rows[:limit]
			last := rows[len(rows)-1]
			page.NextCursor = &QuestionRowCursor{Time: last.CreatedAt.Time, ID: last.ID}
		}
		page.Rows = make([]QuestionRowDTO, len(rows))
		questionIDs = make([]pgtype.UUID, len(rows))
		for i, row := range rows {
			page.Rows[i] = questionRowDTO(db.ListQuestionRowsForMemberRow(row))
			questionIDs[i] = row.ID
		}
	case "for_me":
		rows, err := s.Queries.ListQuestionRowsForMember(ctx, params)
		if err != nil {
			return page, fmt.Errorf("list questions for member: %w", err)
		}
		if len(rows) > limit {
			rows = rows[:limit]
			last := rows[len(rows)-1]
			page.NextCursor = &QuestionRowCursor{Time: last.CreatedAt.Time, ID: last.ID}
		}
		page.Rows = make([]QuestionRowDTO, len(rows))
		questionIDs = make([]pgtype.UUID, len(rows))
		for i, row := range rows {
			page.Rows[i] = questionRowDTO(row)
			questionIDs[i] = row.ID
		}
	case "all_open":
		rows, err := s.Queries.ListOpenQuestionRows(ctx, db.ListOpenQuestionRowsParams(params))
		if err != nil {
			return page, fmt.Errorf("list open questions: %w", err)
		}
		if len(rows) > limit {
			rows = rows[:limit]
			last := rows[len(rows)-1]
			page.NextCursor = &QuestionRowCursor{Time: last.CreatedAt.Time, ID: last.ID}
		}
		page.Rows = make([]QuestionRowDTO, len(rows))
		questionIDs = make([]pgtype.UUID, len(rows))
		for i, row := range rows {
			page.Rows[i] = questionRowDTO(db.ListQuestionRowsForMemberRow(row))
			questionIDs[i] = row.ID
		}
	case "answered":
		rows, err := s.Queries.ListQuestionRowsAnsweredBy(ctx, db.ListQuestionRowsAnsweredByParams(params))
		if err != nil {
			return page, fmt.Errorf("list answered questions: %w", err)
		}
		if len(rows) > limit {
			rows = rows[:limit]
			last := rows[len(rows)-1]
			page.NextCursor = &QuestionRowCursor{Time: last.MyAnswerUpdatedAt.Time, ID: last.ID}
		}
		page.Rows = make([]QuestionRowDTO, len(rows))
		questionIDs = make([]pgtype.UUID, len(rows))
		for i, row := range rows {
			page.Rows[i] = questionRowDTO(db.ListQuestionRowsForMemberRow(row))
			questionIDs[i] = row.ID
		}
	default:
		return page, questionRefusal(400, "invalid question tab")
	}
	if len(page.Rows) == 0 {
		return page, nil
	}

	reach, err := s.Reach(ctx, workspaceID, questionIDs)
	if err != nil {
		return QuestionRowPageDTO{}, fmt.Errorf("load question row reach: %w", err)
	}
	byQuestion := make(map[pgtype.UUID][]QuestionReach, len(page.Rows))
	for start := 0; start < len(reach); {
		end := start + 1
		for end < len(reach) && reach[end].QuestionID == reach[start].QuestionID {
			end++
		}
		byQuestion[reach[start].QuestionID] = reach[start:end]
		start = end
	}
	pageObject := QuestionPageObject{}
	if filters.Parent != nil {
		pageObject = *filters.Parent
	} else if tab == "on_my_work" {
		pageObject.MemberID = viewerID
	}
	for i := range page.Rows {
		row := &page.Rows[i]
		if row.Link.Type == question.LinkTypeChat {
			continue
		}
		links := byQuestion[questionIDs[i]]
		row.OpenIssue = OpenIssue(links)
		row.Via = Via(links, pageObject)
		for j := range links {
			link := &links[j]
			if link.Position != links[0].Position {
				break
			}
			direct := link.ObjectType == link.LinkType && link.ObjectID == link.LinkID
			if link.LinkType == "issue" || link.LinkType == "comment" || link.LinkType == "task" {
				direct = link.ObjectType == "issue" && link.ObjectID == link.IssueID
			}
			if direct {
				row.Link = QuestionRowLinkDTO{
					Type: link.ObjectType, ID: util.UUIDToString(link.ObjectID),
					Identifier: link.Identifier, Title: link.ObjectTitle, Status: link.ObjectStatus,
				}
				if link.ObjectType == "issue" {
					row.Link.Identifier = link.IssueIdentifier
				}
				break
			}
		}
	}
	if err := s.linkRowsToTheirChat(ctx, workspaceID, page.Rows, questionIDs); err != nil {
		return QuestionRowPageDTO{}, err
	}
	return page, nil
}

// linkRowsToTheirChat gives a row without an issue, a chat question asked with
// --on a run or project, its chat as link, so it opens and groups as the chat
// it was asked in, like a row whose first link is that chat.
func (s *QuestionService) linkRowsToTheirChat(ctx context.Context, workspaceID pgtype.UUID, rows []QuestionRowDTO, questionIDs []pgtype.UUID) error {
	index := make(map[pgtype.UUID]int)
	for i := range rows {
		if rows[i].OpenIssue == nil && rows[i].Link.Type != question.LinkTypeChat {
			index[questionIDs[i]] = i
		}
	}
	if len(index) == 0 {
		return nil
	}
	ids := make([]pgtype.UUID, 0, len(index))
	for id := range index {
		ids = append(ids, id)
	}
	links, err := s.Queries.ListQuestionLinksByQuestions(ctx, db.ListQuestionLinksByQuestionsParams{WorkspaceID: workspaceID, QuestionIds: ids})
	if err != nil {
		return fmt.Errorf("load question row chat links: %w", err)
	}
	for _, link := range links {
		if link.LinkType != question.LinkTypeChat {
			continue
		}
		session, err := s.Queries.GetChatSessionInWorkspace(ctx, db.GetChatSessionInWorkspaceParams{ID: link.LinkID, WorkspaceID: workspaceID})
		if err != nil {
			return fmt.Errorf("load question row chat: %w", err)
		}
		memberID := util.UUIDToString(session.CreatorID)
		rows[index[link.QuestionID]].Link = QuestionRowLinkDTO{Type: question.LinkTypeChat, ID: util.UUIDToString(link.LinkID), MemberID: &memberID}
	}
	return nil
}

func questionRowDTO(row db.ListQuestionRowsForMemberRow) QuestionRowDTO {
	result := QuestionRowDTO{
		ID: util.UUIDToString(row.ID), AskID: util.UUIDToString(row.AskID),
		AskPosition: row.AskPosition, AskTitle: questionText(row.AskTitle),
		Prompt: row.Prompt, AnswerType: row.AnswerType, Settings: row.Settings,
		Status: row.Status, AnswerPolicy: row.AnswerPolicy,
		AskerAgentID: util.UUIDToString(row.AskerAgentID),
		AskerUserID:  util.UUIDToString(row.AskerUserID),
		AnswererIDs:  make([]string, len(row.AnswererIds)),
		BlocksWork:   row.BlocksWork, CreatedAt: questionTime(row.CreatedAt),
		Link: QuestionRowLinkDTO{Type: row.LinkType, ID: util.UUIDToString(row.LinkID)},
	}
	if row.LinkMemberID.Valid {
		memberID := util.UUIDToString(row.LinkMemberID)
		result.Link.MemberID = &memberID
	}
	for i, id := range row.AnswererIds {
		result.AnswererIDs[i] = util.UUIDToString(id)
	}
	if row.DueDate.Valid {
		due := row.DueDate.Time.Format("2006-01-02")
		result.DueDate = &due
	}
	if row.MyAnswerUpdatedAt.Valid {
		result.MyAnswer = &QuestionRowAnswerDTO{
			Display:   question.Display(row.AnswerType, row.Settings, row.MyAnswerValue),
			UpdatedAt: questionTime(row.MyAnswerUpdatedAt),
		}
	}
	return result
}

type QuestionOpenCountDTO struct {
	Count         int64
	BlockingCount int64
}

// CountOpen uses the same eligibility and work seeds as the corresponding tab.
func (s *QuestionService) CountOpen(ctx context.Context, workspaceID, viewerID pgtype.UUID, tab string, parent *QuestionPageObject) (QuestionOpenCountDTO, error) {
	if parent != nil && tab != "all_open" {
		return QuestionOpenCountDTO{}, questionRefusal(400, "parent requires tab=all_open")
	}
	if tab == "on_my_work" || parent != nil {
		issueIDs, projectIDs, err := s.questionWorkSeeds(ctx, workspaceID, viewerID, parent)
		if err != nil {
			return QuestionOpenCountDTO{}, err
		}
		count, err := s.Queries.CountQuestionsBelow(ctx, db.CountQuestionsBelowParams{
			WorkspaceID: workspaceID, IssueIds: issueIDs, ProjectIds: projectIDs,
		})
		if err != nil {
			return QuestionOpenCountDTO{}, fmt.Errorf("count questions below work: %w", err)
		}
		return QuestionOpenCountDTO{Count: count.Count, BlockingCount: count.BlockingCount}, nil
	}
	switch tab {
	case "for_me":
		count, err := s.Queries.CountQuestionsOpenForMember(ctx, db.CountQuestionsOpenForMemberParams{WorkspaceID: workspaceID, ViewerID: viewerID})
		if err != nil {
			return QuestionOpenCountDTO{}, fmt.Errorf("count open questions for member: %w", err)
		}
		return QuestionOpenCountDTO{Count: count.Count, BlockingCount: count.BlockingCount}, nil
	case "all_open":
		count, err := s.Queries.CountQuestionsOpen(ctx, workspaceID)
		if err != nil {
			return QuestionOpenCountDTO{}, fmt.Errorf("count open questions: %w", err)
		}
		return QuestionOpenCountDTO{Count: count.Count, BlockingCount: count.BlockingCount}, nil
	default:
		return QuestionOpenCountDTO{}, questionRefusal(400, "invalid question tab")
	}
}

func (s *QuestionService) questionWorkSeeds(ctx context.Context, workspaceID, viewerID pgtype.UUID, parent *QuestionPageObject) ([]pgtype.UUID, []pgtype.UUID, error) {
	if parent != nil {
		switch parent.Type {
		case "issue":
			return []pgtype.UUID{parent.ID}, nil, nil
		case "project":
			return nil, []pgtype.UUID{parent.ID}, nil
		default:
			return nil, nil, questionRefusal(400, "invalid question parent")
		}
	}
	seeds, err := s.Queries.WorkSeedsForMember(ctx, db.WorkSeedsForMemberParams{WorkspaceID: workspaceID, UserID: viewerID})
	if err != nil {
		return nil, nil, fmt.Errorf("load member work seeds: %w", err)
	}
	var issues, projects []pgtype.UUID
	for _, seed := range seeds {
		if seed.ObjectType == "issue" {
			issues = append(issues, seed.ObjectID)
		} else {
			projects = append(projects, seed.ObjectID)
		}
	}
	return issues, projects, nil
}
