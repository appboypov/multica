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
}

// QuestionRowCursor is the last returned row's sort key. The handler binds it
// to the workspace, viewer, tab and filters in its base64 JSON cursor scope.
// Time is created_at for open tabs and my_answer.updated_at for Answered.
type QuestionRowCursor struct {
	Time time.Time
	ID   pgtype.UUID
}

type QuestionRowLinkDTO struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
	Status     string `json:"status"`
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
	AnswererIDs  []string              `json:"answerer_ids"`
	DueDate      *string               `json:"due_date"`
	BlocksWork   bool                  `json:"blocks_work"`
	CreatedAt    string                `json:"created_at"`
	Link         QuestionRowLinkDTO    `json:"link"`
	MyAnswer     *QuestionRowAnswerDTO `json:"my_answer"`
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
		AnswerType: filters.AnswerTypes, AskerAgentID: filters.AskerIDs,
		AnswererUserID: filters.AnswererIDs, LinkIds: filters.LinkIDs,
		BlocksWork: pgtype.Bool{Bool: filters.BlocksWork, Valid: filters.BlocksWork},
		Limit:      int32(limit + 1),
	}
	if cursor != nil {
		params.CursorTime = pgtype.Timestamptz{Time: cursor.Time, Valid: true}
		params.CursorID = cursor.ID
	}

	var page QuestionRowPageDTO
	var issueIDs []pgtype.UUID
	switch tab {
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
		issueIDs = make([]pgtype.UUID, len(rows))
		for i, row := range rows {
			page.Rows[i] = questionRowDTO(row)
			issueIDs[i] = row.LinkID
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
		issueIDs = make([]pgtype.UUID, len(rows))
		for i, row := range rows {
			page.Rows[i] = questionRowDTO(db.ListQuestionRowsForMemberRow(row))
			issueIDs[i] = row.LinkID
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
		issueIDs = make([]pgtype.UUID, len(rows))
		for i, row := range rows {
			page.Rows[i] = questionRowDTO(db.ListQuestionRowsForMemberRow(row))
			issueIDs[i] = row.LinkID
		}
	default:
		return page, questionRefusal(400, "invalid question tab")
	}
	if len(page.Rows) == 0 {
		return page, nil
	}

	refs, err := s.Queries.ListIssueRefsInWorkspace(ctx, db.ListIssueRefsInWorkspaceParams{WorkspaceID: workspaceID, Ids: issueIDs})
	if err != nil {
		return QuestionRowPageDTO{}, fmt.Errorf("load question issue references: %w", err)
	}
	workspace, err := s.Queries.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return QuestionRowPageDTO{}, fmt.Errorf("load question workspace: %w", err)
	}
	issues := make(map[pgtype.UUID]db.ListIssueRefsInWorkspaceRow, len(refs))
	for _, ref := range refs {
		issues[ref.ID] = ref
	}
	for i := range page.Rows {
		link := &page.Rows[i].Link
		issue, ok := issues[issueIDs[i]]
		if !ok {
			return QuestionRowPageDTO{}, fmt.Errorf("question %s links to missing issue %s in workspace", page.Rows[i].ID, link.ID)
		}
		link.Identifier = IssueIdentifier(workspace.IssuePrefix, issue.Number)
		link.Title = issue.Title
		link.Status = issue.Status
	}
	return page, nil
}

func questionRowDTO(row db.ListQuestionRowsForMemberRow) QuestionRowDTO {
	result := QuestionRowDTO{
		ID: util.UUIDToString(row.ID), AskID: util.UUIDToString(row.AskID),
		AskPosition: row.AskPosition, AskTitle: questionText(row.AskTitle),
		Prompt: row.Prompt, AnswerType: row.AnswerType, Settings: row.Settings,
		Status: row.Status, AnswerPolicy: row.AnswerPolicy,
		AskerAgentID: util.UUIDToString(row.AskerAgentID),
		AnswererIDs:  make([]string, len(row.AnswererIds)),
		BlocksWork:   row.BlocksWork, CreatedAt: questionTime(row.CreatedAt),
		Link: QuestionRowLinkDTO{Type: row.LinkType, ID: util.UUIDToString(row.LinkID)},
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

// CountOpenForMember returns the unfiltered For me count, one per question,
// using the same SQL eligibility rule as ListRows("for_me").
func (s *QuestionService) CountOpenForMember(ctx context.Context, workspaceID, viewerID pgtype.UUID) (int64, error) {
	count, err := s.Queries.CountQuestionsOpenForMember(ctx, db.CountQuestionsOpenForMemberParams{WorkspaceID: workspaceID, ViewerID: viewerID})
	if err != nil {
		return 0, fmt.Errorf("count open questions for member: %w", err)
	}
	return count, nil
}
