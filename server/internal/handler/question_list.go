package handler

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/question"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

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

// questionRowCursor is the wire cursor of GET /api/questions: the last row's
// sort key and the scope of the list it was issued for.
type questionRowCursor struct {
	Scope string `json:"scope"`
	Time  string `json:"time"`
	ID    string `json:"id"`
}

// questionRowScope ties a cursor to the workspace, viewer, tab and filters, so
// it cannot continue another tab, another viewer's list or changed filters.
func questionRowScope(workspaceID, viewerID pgtype.UUID, tab string, filters service.QuestionRowFilters) string {
	encoded, _ := json.Marshal(struct {
		Workspace pgtype.UUID
		Viewer    pgtype.UUID
		Tab       string
		Filters   service.QuestionRowFilters
	}{workspaceID, viewerID, tab, filters})
	return fmt.Sprintf("%x", sha256.Sum256(encoded))
}

// questionViewer reads the workspace and the member whose list is read; the
// RequireWorkspaceMember middleware has already checked membership.
func questionViewer(w http.ResponseWriter, r *http.Request) (pgtype.UUID, pgtype.UUID, bool) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return pgtype.UUID{}, pgtype.UUID{}, false
	}
	workspaceID, ok := parseUUIDOrBadRequest(w, ctxWorkspaceID(r.Context()), "workspace id")
	if !ok {
		return pgtype.UUID{}, pgtype.UUID{}, false
	}
	return workspaceID, parseUUID(userID), true
}

// questionFilterValues returns a repeated query parameter sorted and without
// duplicates, so the same selection always yields the same cursor scope.
func questionFilterValues(q url.Values, name string) []string {
	values := q[name]
	slices.Sort(values)
	return slices.Compact(values)
}

func parseQuestionRowFilters(w http.ResponseWriter, q url.Values) (service.QuestionRowFilters, bool) {
	var filters service.QuestionRowFilters
	filters.AnswerTypes = questionFilterValues(q, "answer_type")
	for _, answerType := range filters.AnswerTypes {
		if !question.ValidAnswerType(answerType) {
			writeError(w, http.StatusBadRequest, "invalid answer_type")
			return filters, false
		}
	}
	for _, param := range []struct {
		name   string
		target *[]pgtype.UUID
	}{
		{"asker_id", &filters.AskerIDs},
		{"answerer_id", &filters.AnswererIDs},
		{"link_id", &filters.LinkIDs},
	} {
		ids, ok := parseUUIDSliceOrBadRequest(w, questionFilterValues(q, param.name), param.name)
		if !ok {
			return filters, false
		}
		*param.target = ids
	}
	if raw := q.Get("blocks_work"); raw != "" {
		if raw != "true" && raw != "false" {
			writeError(w, http.StatusBadRequest, "invalid blocks_work")
			return filters, false
		}
		filters.BlocksWork = raw == "true"
	}
	return filters, true
}

// parseQuestionParent resolves a page object before it becomes part of the
// cursor scope; an issue identifier and its UUID therefore share one scope.
func (h *Handler) parseQuestionParent(w http.ResponseWriter, r *http.Request, q url.Values, workspaceID pgtype.UUID, tab string) (*service.QuestionPageObject, bool) {
	values, present := q["parent"]
	if !present {
		return nil, true
	}
	if tab != "all_open" {
		writeError(w, http.StatusBadRequest, "parent requires tab=all_open")
		return nil, false
	}
	if len(values) != 1 {
		writeError(w, http.StatusBadRequest, "invalid parent")
		return nil, false
	}
	parentType, id, found := strings.Cut(values[0], ":")
	if !found || id == "" {
		writeError(w, http.StatusBadRequest, "parent must be issue:{id} or project:{id}")
		return nil, false
	}
	switch parentType {
	case "issue":
		issue, ok := h.loadIssueForUser(w, r, id)
		if !ok {
			return nil, false
		}
		return &service.QuestionPageObject{Type: "issue", ID: issue.ID}, true
	case "project":
		projectID, ok := parseUUIDOrBadRequest(w, id, "project id")
		if !ok {
			return nil, false
		}
		project, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{
			ID: projectID, WorkspaceID: workspaceID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "project not found")
			return nil, false
		}
		if err != nil {
			writeQuestionError(w, err)
			return nil, false
		}
		return &service.QuestionPageObject{Type: "project", ID: project.ID}, true
	default:
		writeError(w, http.StatusBadRequest, "parent must be issue:{id} or project:{id}")
		return nil, false
	}
}

// ListQuestions serves GET /api/questions: one page of the viewer's question
// rows for a tab, filtered, after an optional cursor (design.md decision 1).
func (h *Handler) ListQuestions(w http.ResponseWriter, r *http.Request) {
	workspaceID, viewerID, ok := questionViewer(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	tab := q.Get("tab")
	switch tab {
	case "":
		tab = "for_me"
	case "for_me", "on_my_work", "all_open", "answered":
	default:
		writeError(w, http.StatusBadRequest, "invalid tab")
		return
	}
	filters, ok := parseQuestionRowFilters(w, q)
	if !ok {
		return
	}
	filters.Parent, ok = h.parseQuestionParent(w, r, q, workspaceID, tab)
	if !ok {
		return
	}
	limit := 50
	if raw := q.Get("limit"); raw != "" {
		var err error
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 100")
			return
		}
	}
	scope := questionRowScope(workspaceID, viewerID, tab, filters)
	var cursor *service.QuestionRowCursor
	if raw := q.Get("cursor"); raw != "" {
		if len(raw) > 2048 {
			writeError(w, http.StatusBadRequest, "invalid question cursor")
			return
		}
		var wire questionRowCursor
		decoded, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil || json.Unmarshal(decoded, &wire) != nil || wire.Scope != scope {
			writeError(w, http.StatusBadRequest, "invalid question cursor")
			return
		}
		after, err := time.Parse(time.RFC3339Nano, wire.Time)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid question cursor time")
			return
		}
		id, ok := parseUUIDOrBadRequest(w, wire.ID, "cursor id")
		if !ok {
			return
		}
		cursor = &service.QuestionRowCursor{Time: after, ID: id}
	}

	page, err := h.QuestionService.ListRows(r.Context(), workspaceID, viewerID, tab, filters, cursor, limit)
	if err != nil {
		writeQuestionError(w, err)
		return
	}
	resp := QuestionRowPageResponse{Rows: make([]QuestionRowResponse, len(page.Rows))}
	for i := range page.Rows {
		resp.Rows[i] = questionRowResponse(&page.Rows[i])
	}
	if page.NextCursor != nil {
		encoded, _ := json.Marshal(questionRowCursor{
			Scope: scope,
			Time:  page.NextCursor.Time.Format(time.RFC3339Nano),
			ID:    uuidToString(page.NextCursor.ID),
		})
		next := base64.RawURLEncoding.EncodeToString(encoded)
		resp.NextCursor = &next
	}
	writeJSON(w, http.StatusOK, resp)
}

// questionRowResponse maps the service projection onto the wire row, preserving
// the page object, its reach path and the issue where the card opens.
func questionRowResponse(row *service.QuestionRowDTO) QuestionRowResponse {
	resp := QuestionRowResponse{
		ID: row.ID, AskID: row.AskID, AskPosition: row.AskPosition, AskTitle: row.AskTitle,
		Prompt: row.Prompt, AnswerType: row.AnswerType, Settings: row.Settings,
		Status: row.Status, AnswerPolicy: row.AnswerPolicy, AskerAgentID: row.AskerAgentID,
		AnswererIDs: row.AnswererIDs, DueDate: row.DueDate, BlocksWork: row.BlocksWork,
		CreatedAt: row.CreatedAt,
		Link: QuestionRowLinkResponse{
			Type: row.Link.Type, ID: row.Link.ID,
			Title: row.Link.Title,
		},
	}
	if row.Link.Identifier != "" {
		resp.Link.Identifier = &row.Link.Identifier
	}
	if row.Link.Status != "" {
		resp.Link.Status = &row.Link.Status
	}
	if row.Via != nil {
		resp.Via = &QuestionViaResponse{
			Type: row.Via.Type, ID: row.Via.ID,
			Identifier: row.Via.Identifier, IssueIdentifier: row.Via.IssueIdentifier,
		}
	}
	if row.OpenIssue != nil {
		resp.OpenIssue.ID = row.OpenIssue.ID
		if row.OpenIssue.Identifier != nil {
			resp.OpenIssue.Identifier = *row.OpenIssue.Identifier
		}
	}
	if row.MyAnswer != nil {
		resp.MyAnswer = &QuestionRowAnswerResponse{Display: row.MyAnswer.Display, UpdatedAt: row.MyAnswer.UpdatedAt}
	}
	return resp
}

// CountOpenQuestions serves GET /api/questions/open-count: open and blocking
// counts for the viewer's tab and optional parent, without list filters.
func (h *Handler) CountOpenQuestions(w http.ResponseWriter, r *http.Request) {
	workspaceID, viewerID, ok := questionViewer(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	tab := q.Get("tab")
	switch tab {
	case "":
		tab = "for_me"
	case "for_me", "on_my_work", "all_open":
	default:
		writeError(w, http.StatusBadRequest, "invalid tab")
		return
	}
	parent, ok := h.parseQuestionParent(w, r, q, workspaceID, tab)
	if !ok {
		return
	}
	count, err := h.QuestionService.CountOpen(r.Context(), workspaceID, viewerID, tab, parent)
	if err != nil {
		writeQuestionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, QuestionOpenCountResponse{Count: count.Count, BlockingCount: count.BlockingCount})
}
