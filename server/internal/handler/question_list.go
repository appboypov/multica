package handler

import "encoding/json"

// The row list of the Questions page (ADR-0006): the wire shape shared with
// QuestionRow, QuestionRowPage and QuestionOpenCount in
// packages/core/types/question.ts.

// QuestionRowLinkResponse is the first link's page object; comments and runs
// use their issue, while other object types use the object itself.
type QuestionRowLinkResponse struct {
	Type       string  `json:"type"`
	ID         string  `json:"id"`
	Identifier *string `json:"identifier"`
	Title      string  `json:"title"`
	Status     *string `json:"status"`
}

// QuestionViaResponse identifies the first link that reaches the page's object.
type QuestionViaResponse struct {
	Type            string  `json:"type"`
	ID              string  `json:"id"`
	Identifier      *string `json:"identifier"`
	IssueIdentifier *string `json:"issue_identifier"`
}

// QuestionOpenIssueResponse is where the row's question card opens.
type QuestionOpenIssueResponse struct {
	ID         string `json:"id"`
	Identifier string `json:"identifier"`
}

// QuestionRowAnswerResponse is the viewer's own answer to the row's question.
type QuestionRowAnswerResponse struct {
	Display   string `json:"display"`
	UpdatedAt string `json:"updated_at"`
}

// QuestionRowResponse is one question projected for the viewer's list; it is
// not UJ-1's QuestionDTO.
type QuestionRowResponse struct {
	ID           string                     `json:"id"`
	AskID        string                     `json:"ask_id"`
	AskPosition  int32                      `json:"ask_position"`
	AskTitle     *string                    `json:"ask_title"`
	Prompt       string                     `json:"prompt"`
	AnswerType   string                     `json:"answer_type"`
	Settings     json.RawMessage            `json:"settings"`
	Status       string                     `json:"status"`
	AnswerPolicy string                     `json:"answer_policy"`
	AskerAgentID string                     `json:"asker_agent_id"`
	AnswererIDs  []string                   `json:"answerer_ids"`
	DueDate      *string                    `json:"due_date"`
	BlocksWork   bool                       `json:"blocks_work"`
	CreatedAt    string                     `json:"created_at"`
	Link         QuestionRowLinkResponse    `json:"link"`
	Via          *QuestionViaResponse       `json:"via"`
	OpenIssue    QuestionOpenIssueResponse  `json:"open_issue"`
	MyAnswer     *QuestionRowAnswerResponse `json:"my_answer"`
}

// QuestionRowPageResponse is the 200 body of GET /api/questions; NextCursor is
// nil on the last page.
type QuestionRowPageResponse struct {
	Rows       []QuestionRowResponse `json:"rows"`
	NextCursor *string               `json:"next_cursor"`
}

// QuestionOpenCountResponse is the 200 body of GET /api/questions/open-count,
// scoped by tab and an optional parent.
type QuestionOpenCountResponse struct {
	Count         int64 `json:"count"`
	BlockingCount int64 `json:"blocking_count"`
}
