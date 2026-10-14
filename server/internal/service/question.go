package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/question"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// QuestionError describes an expected refusal at the service boundary.
type QuestionError struct {
	Status  int
	Message string
}

func (e *QuestionError) Error() string { return e.Message }

func questionRefusal(status int, message string) error {
	return &QuestionError{Status: status, Message: message}
}

// QuestionActor comes from authenticated member or task-token identity, never
// from a request body. Agent writes require their authenticated task as well.
type QuestionActor struct {
	Type   string
	ID     pgtype.UUID
	TaskID pgtype.UUID
}

type AskQuestionInput struct {
	Prompt          string
	ContextLine     string
	AnswerType      string
	Settings        json.RawMessage
	SuggestedAnswer json.RawMessage
	AnswererIDs     []pgtype.UUID
	DueDate         string
	BlocksWork      bool
}

type AnswerQuestionInput struct {
	Value json.RawMessage
	Note  string
}

type QuestionWakeResult struct {
	TaskID      string `json:"task_id"`
	AnswerCount int64  `json:"answer_count"`
}

type QuestionLinkDTO struct {
	LinkType string `json:"link_type"`
	LinkID   string `json:"link_id"`
	Position int32  `json:"position"`
}

type QuestionAnswererDTO struct {
	UserID     string `json:"user_id"`
	AssignedAt string `json:"assigned_at"`
}

type QuestionAnswerDTO struct {
	ID             string          `json:"id"`
	AnswererUserID string          `json:"answerer_user_id"`
	Value          json.RawMessage `json:"value"`
	DisplayText    string          `json:"display_text"`
	Note           *string         `json:"note"`
	CreatedAt      string          `json:"created_at"`
	UpdatedAt      string          `json:"updated_at"`
}

type QuestionEventDTO struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	ActorType string          `json:"actor_type"`
	ActorID   *string         `json:"actor_id"`
	Details   json.RawMessage `json:"details"`
	CreatedAt string          `json:"created_at"`
}

// QuestionWakeTargetDTO is an agent/issue pair offered Wake, with its unseen answers.
type QuestionWakeTargetDTO struct {
	AgentID           string `json:"agent_id"`
	IssueID           string `json:"issue_id"`
	UnseenAnswerCount int64  `json:"unseen_answer_count"`
	WakeAvailable     bool   `json:"wake_available"`
}

// QuestionDTO is the aggregate wire shape shared with core/types/question.ts.
type QuestionDTO struct {
	ID                string                  `json:"id"`
	WorkspaceID       string                  `json:"workspace_id"`
	AskerAgentID      string                  `json:"asker_agent_id"`
	AskedInTaskID     string                  `json:"asked_in_task_id"`
	Prompt            string                  `json:"prompt"`
	ContextLine       *string                 `json:"context_line"`
	AnswerType        string                  `json:"answer_type"`
	Settings          json.RawMessage         `json:"settings"`
	SuggestedAnswer   json.RawMessage         `json:"suggested_answer"`
	Status            string                  `json:"status"`
	DueDate           *string                 `json:"due_date"`
	BlocksWork        bool                    `json:"blocks_work"`
	Links             []QuestionLinkDTO       `json:"links"`
	Answerers         []QuestionAnswererDTO   `json:"answerers"`
	Answers           []QuestionAnswerDTO     `json:"answers"`
	Events            []QuestionEventDTO      `json:"events"`
	UnseenAnswerCount int64                   `json:"unseen_answer_count"`
	WakeAvailable     bool                    `json:"wake_available"`
	WakeTargets       []QuestionWakeTargetDTO `json:"wake_targets"`
	CreatedAt         string                  `json:"created_at"`
	UpdatedAt         string                  `json:"updated_at"`
}

type QuestionService struct {
	Queries   *db.Queries
	TxStarter TxStarter
	Bus       *events.Bus
	Tasks     *TaskService
}

func questionText(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

func questionTime(t pgtype.Timestamptz) string { return t.Time.UTC().Format(time.RFC3339Nano) }

func questionLoadError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return questionRefusal(404, "question not found")
	}
	return err
}

func (s *QuestionService) requireMember(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID, actor QuestionActor) error {
	if actor.Type != "member" || !actor.ID.Valid {
		return questionRefusal(403, "only members answer, reassign or wake questions")
	}
	_, err := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: actor.ID, WorkspaceID: workspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return questionRefusal(403, "workspace membership required")
	}
	return err
}

func (s *QuestionService) requireAgentTask(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID, actor QuestionActor) (db.AgentTaskQueue, error) {
	if actor.Type != "agent" || !actor.ID.Valid || !actor.TaskID.Valid {
		return db.AgentTaskQueue{}, questionRefusal(403, "only agents ask, close or withdraw questions with a task token")
	}
	agent, err := q.GetAgent(ctx, actor.ID)
	if err != nil {
		return db.AgentTaskQueue{}, questionLoadError(err)
	}
	if agent.WorkspaceID != workspaceID {
		return db.AgentTaskQueue{}, questionRefusal(403, "agent does not belong to this workspace")
	}
	task, err := q.GetAgentTask(ctx, actor.TaskID)
	if err != nil {
		return db.AgentTaskQueue{}, questionLoadError(err)
	}
	if task.AgentID != actor.ID {
		return db.AgentTaskQueue{}, questionRefusal(403, "task does not belong to the asking agent")
	}
	// Tasks have no workspace column: their agent and, when present, issue must
	// agree with the selected workspace before their IDs can become links.
	if task.IssueID.Valid {
		issue, err := q.GetIssue(ctx, task.IssueID)
		if err != nil {
			return db.AgentTaskQueue{}, err
		}
		if issue.WorkspaceID != workspaceID {
			return db.AgentTaskQueue{}, questionRefusal(403, "task does not belong to this workspace")
		}
	}
	return task, nil
}

func (s *QuestionService) validateAnswerer(ctx context.Context, q *db.Queries, workspaceID, userID pgtype.UUID) error {
	if !userID.Valid {
		return questionRefusal(422, "answerer must be a workspace member")
	}
	_, err := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: userID, WorkspaceID: workspaceID})
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	agent, agentErr := q.GetAgent(ctx, userID)
	if agentErr == nil && agent.WorkspaceID == workspaceID {
		return questionRefusal(422, fmt.Sprintf("%s is an agent; only members answer questions", agent.Name))
	}
	if agentErr != nil && !errors.Is(agentErr, pgx.ErrNoRows) {
		return agentErr
	}
	return questionRefusal(422, "answerer must be a workspace member")
}

func (s *QuestionService) validateReferences(ctx context.Context, q *db.Queries, row db.Question, actorID pgtype.UUID, value json.RawMessage) error {
	if row.AnswerType == "person" {
		var person question.PersonAnswerValue
		if err := json.Unmarshal(value, &person); err != nil {
			return questionRefusal(422, err.Error())
		}
		id, err := util.ParseUUID(person.ID)
		if err != nil {
			return questionRefusal(422, "person must belong to this workspace")
		}
		if person.Type == "member" {
			_, err = q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: id, WorkspaceID: row.WorkspaceID})
		} else {
			var agent db.Agent
			agent, err = q.GetAgent(ctx, id)
			if err == nil && agent.WorkspaceID != row.WorkspaceID {
				return questionRefusal(422, "person must belong to this workspace")
			}
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return questionRefusal(422, "person must belong to this workspace")
		}
		return err
	}
	if row.AnswerType != "file" && row.AnswerType != "text" {
		return nil
	}
	var files struct {
		AttachmentIDs []string `json:"attachment_ids"`
	}
	if err := json.Unmarshal(value, &files); err != nil {
		return questionRefusal(422, err.Error())
	}
	var settings question.FileAnswerSettings
	if row.AnswerType == "file" {
		if err := json.Unmarshal(row.Settings, &settings); err != nil {
			return err
		}
	}
	for _, rawID := range files.AttachmentIDs {
		id, err := util.ParseUUID(rawID)
		if err != nil {
			return questionRefusal(422, "attachment must belong to this question and its uploader")
		}
		attachment, err := q.GetAttachment(ctx, db.GetAttachmentParams{ID: id, WorkspaceID: row.WorkspaceID})
		if errors.Is(err, pgx.ErrNoRows) {
			return questionRefusal(422, "attachment must belong to this question and its uploader")
		}
		if err != nil {
			return err
		}
		if attachment.QuestionID != row.ID || attachment.UploaderID != actorID || attachment.UploaderType != "member" {
			return questionRefusal(422, "attachment must belong to this question and its uploader")
		}
		if row.AnswerType == "file" && !question.AcceptsFile(settings, attachment.ContentType, attachment.Filename) {
			return questionRefusal(422, fmt.Sprintf("file %q is not an accepted kind", attachment.Filename))
		}
	}
	return nil
}

func (s *QuestionService) writeEvent(ctx context.Context, q *db.Queries, row db.Question, actor QuestionActor, kind string, details any) (string, error) {
	if details == nil {
		details = struct{}{}
	}
	encoded, err := json.Marshal(details)
	if err != nil {
		return "", err
	}
	event, err := q.CreateQuestionEvent(ctx, db.CreateQuestionEventParams{ID: dbid.NewV7(), QuestionID: row.ID, WorkspaceID: row.WorkspaceID, Kind: kind, ActorType: actor.Type, ActorID: actor.ID, Details: encoded})
	return util.UUIDToString(event.ID), err
}

func (s *QuestionService) publish(row QuestionDTO, actor QuestionActor, reason, eventID string) {
	slog.Info("question action committed", "question_id", row.ID, "workspace_id", row.WorkspaceID, "action", reason, "actor_type", actor.Type, "actor_id", util.UUIDToString(actor.ID))
	if s.Bus == nil {
		return
	}
	issueID := ""
	for _, link := range row.Links {
		if link.LinkType == "issue" {
			issueID = link.LinkID
			break
		}
	}
	eventType := protocol.EventQuestionUpdated
	if reason == "asked" {
		eventType = protocol.EventQuestionCreated
	}
	s.Bus.Publish(events.Event{Type: eventType, WorkspaceID: row.WorkspaceID, ActorType: actor.Type, ActorID: util.UUIDToString(actor.ID), Payload: protocol.QuestionEventPayload{QuestionID: row.ID, IssueID: issueID, Reason: reason, EventID: eventID}})
}

func (s *QuestionService) Ask(ctx context.Context, workspaceID pgtype.UUID, actor QuestionActor, input AskQuestionInput) (QuestionDTO, error) {
	if actor.Type != "agent" {
		return QuestionDTO{}, questionRefusal(403, "only agents ask questions")
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return QuestionDTO{}, err
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	task, err := s.requireAgentTask(ctx, q, workspaceID, actor)
	if err != nil {
		return QuestionDTO{}, err
	}
	if !task.IssueID.Valid {
		return QuestionDTO{}, questionRefusal(422, "questions are asked during an issue run")
	}
	// Decided at claim (ADR-0005): a run claimed while questions were off
	// never asks, whatever the setting says now.
	if !task.QuestionsEnabled {
		return QuestionDTO{}, questionRefusal(403, "questions are off for this issue")
	}
	if !task.StartedAt.Valid {
		return QuestionDTO{}, questionRefusal(422, "questions are asked during a started issue run")
	}
	if strings.TrimSpace(input.Prompt) == "" {
		return QuestionDTO{}, questionRefusal(422, "prompt is required")
	}
	if len(input.AnswererIDs) == 0 {
		return QuestionDTO{}, questionRefusal(422, "at least one answerer is required")
	}
	if err := question.ValidateSettings(input.AnswerType, input.Settings); err != nil {
		return QuestionDTO{}, questionRefusal(422, err.Error())
	}
	if len(input.SuggestedAnswer) > 0 && string(input.SuggestedAnswer) != "null" {
		if err := question.ValidateSuggested(input.AnswerType, input.Settings, input.SuggestedAnswer); err != nil {
			return QuestionDTO{}, questionRefusal(422, err.Error())
		}
		if err := s.validateReferences(ctx, q, db.Question{WorkspaceID: workspaceID, AnswerType: input.AnswerType, Settings: input.Settings}, actor.ID, input.SuggestedAnswer); err != nil {
			return QuestionDTO{}, err
		}
	}
	var due pgtype.Date
	if input.DueDate != "" {
		date, err := time.Parse("2006-01-02", input.DueDate)
		if err != nil {
			return QuestionDTO{}, questionRefusal(422, "due_date must be YYYY-MM-DD")
		}
		due = pgtype.Date{Time: date, Valid: true}
	}
	for _, id := range input.AnswererIDs {
		if err := s.validateAnswerer(ctx, q, workspaceID, id); err != nil {
			return QuestionDTO{}, err
		}
	}
	id := dbid.NewV7()
	row, err := q.CreateQuestion(ctx, db.CreateQuestionParams{ID: id, WorkspaceID: workspaceID, AskerAgentID: actor.ID, AskedInTaskID: task.ID, Prompt: input.Prompt, ContextLine: pgtype.Text{String: input.ContextLine, Valid: input.ContextLine != ""}, AnswerType: input.AnswerType, Settings: input.Settings, SuggestedAnswer: input.SuggestedAnswer, DueDate: due, BlocksWork: input.BlocksWork, AskID: id, AskPosition: 0, AnswerPolicy: QuestionAnswerPolicyAnyOne})
	if err != nil {
		return QuestionDTO{}, err
	}
	for i, link := range []struct {
		kind string
		id   pgtype.UUID
	}{{"issue", task.IssueID}, {"task", task.ID}} {
		if err := q.CreateQuestionLink(ctx, db.CreateQuestionLinkParams{QuestionID: row.ID, WorkspaceID: workspaceID, LinkType: link.kind, LinkID: link.id, Position: int32(i)}); err != nil {
			return QuestionDTO{}, err
		}
	}
	if err := q.CreateQuestionAnswerers(ctx, db.CreateQuestionAnswerersParams{QuestionID: row.ID, WorkspaceID: workspaceID, UserIds: input.AnswererIDs}); err != nil {
		return QuestionDTO{}, err
	}
	eventID, err := s.writeEvent(ctx, q, row, actor, "asked", nil)
	if err != nil {
		return QuestionDTO{}, err
	}
	result, err := s.hydrate(ctx, q, workspaceID, []db.Question{row})
	if err != nil {
		return QuestionDTO{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return QuestionDTO{}, err
	}
	s.publish(result[0], actor, "asked", eventID)
	return result[0], nil
}

// mutate serializes status and answerer checks with the action's writes. Its
// aggregate and event are committed before any subscriber sees the action.
func (s *QuestionService) mutate(ctx context.Context, workspaceID, questionID pgtype.UUID, actor QuestionActor, agentOnly bool, action func(*db.Queries, db.Question) (db.Question, string, any, error)) (QuestionDTO, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return QuestionDTO{}, err
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	if agentOnly {
		if _, err := s.requireAgentTask(ctx, q, workspaceID, actor); err != nil {
			return QuestionDTO{}, err
		}
	} else if err := s.requireMember(ctx, q, workspaceID, actor); err != nil {
		return QuestionDTO{}, err
	}
	row, err := q.GetQuestionForUpdate(ctx, db.GetQuestionForUpdateParams{ID: questionID, WorkspaceID: workspaceID})
	if err != nil {
		return QuestionDTO{}, questionLoadError(err)
	}
	if agentOnly {
		if row.AskerAgentID != actor.ID {
			return QuestionDTO{}, questionRefusal(403, "only the asking agent can close or withdraw this question")
		}
	} else {
		answerer, err := q.IsQuestionAnswerer(ctx, db.IsQuestionAnswererParams{QuestionID: row.ID, WorkspaceID: workspaceID, UserID: actor.ID})
		if err != nil {
			return QuestionDTO{}, err
		}
		if !answerer {
			return QuestionDTO{}, questionRefusal(403, "only a current answerer can act on this question")
		}
	}
	row, reason, details, err := action(q, row)
	if err != nil {
		return QuestionDTO{}, err
	}
	eventID, err := s.writeEvent(ctx, q, row, actor, reason, details)
	if err != nil {
		return QuestionDTO{}, err
	}
	result, err := s.hydrate(ctx, q, workspaceID, []db.Question{row})
	if err != nil {
		return QuestionDTO{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return QuestionDTO{}, err
	}
	s.publish(result[0], actor, reason, eventID)
	return result[0], nil
}

func (s *QuestionService) Answer(ctx context.Context, workspaceID, questionID pgtype.UUID, actor QuestionActor, input AnswerQuestionInput) (QuestionDTO, error) {
	return s.mutate(ctx, workspaceID, questionID, actor, false, func(q *db.Queries, row db.Question) (db.Question, string, any, error) {
		if row.Status != "open" && row.Status != "answered" {
			return row, "", nil, questionRefusal(422, "this question no longer accepts answers")
		}
		if row.Status == "answered" {
			answers, err := q.ListQuestionAnswersByQuestions(ctx, db.ListQuestionAnswersByQuestionsParams{WorkspaceID: workspaceID, QuestionIds: []pgtype.UUID{row.ID}})
			if err != nil {
				return row, "", nil, err
			}
			own := false
			for _, answer := range answers {
				if answer.AnswererUserID == actor.ID {
					own = true
					break
				}
			}
			if !own {
				return row, "", nil, questionRefusal(422, "this question has already been answered")
			}
		}
		if err := question.ValidateAnswer(row.AnswerType, row.Settings, input.Value, input.Note); err != nil {
			return row, "", nil, questionRefusal(422, err.Error())
		}
		if err := s.validateReferences(ctx, q, row, actor.ID, input.Value); err != nil {
			return row, "", nil, err
		}
		answer, err := q.UpsertQuestionAnswer(ctx, db.UpsertQuestionAnswerParams{ID: dbid.NewV7(), QuestionID: row.ID, WorkspaceID: workspaceID, AnswererUserID: actor.ID, Value: input.Value, Note: pgtype.Text{String: input.Note, Valid: input.Note != ""}})
		if err != nil {
			return row, "", nil, err
		}
		row, err = q.SetQuestionStatus(ctx, db.SetQuestionStatusParams{ID: row.ID, WorkspaceID: workspaceID, Status: "answered", FromStatuses: []string{"open", "answered"}})
		reason := "answer_edited"
		if answer.Inserted {
			reason = "answered"
		}
		return row, reason, nil, err
	})
}

func (s *QuestionService) AskBack(ctx context.Context, workspaceID, questionID pgtype.UUID, actor QuestionActor, text string) (QuestionDTO, error) {
	return s.mutate(ctx, workspaceID, questionID, actor, false, func(q *db.Queries, row db.Question) (db.Question, string, any, error) {
		if row.Status != "open" && row.Status != "answered" {
			return row, "", nil, questionRefusal(422, "this question no longer accepts replies")
		}
		if strings.TrimSpace(text) == "" {
			return row, "", nil, questionRefusal(422, "reply text is required")
		}
		updated, err := q.SetQuestionStatus(ctx, db.SetQuestionStatusParams{ID: row.ID, WorkspaceID: workspaceID, Status: row.Status, FromStatuses: []string{row.Status}})
		return updated, "ask_back", struct {
			Text string `json:"text"`
		}{text}, err
	})
}

func (s *QuestionService) Reassign(ctx context.Context, workspaceID, questionID pgtype.UUID, actor QuestionActor, userID pgtype.UUID, note string) (QuestionDTO, error) {
	return s.mutate(ctx, workspaceID, questionID, actor, false, func(q *db.Queries, row db.Question) (db.Question, string, any, error) {
		if row.Status != "open" {
			return row, "", nil, questionRefusal(422, "only open questions can be reassigned")
		}
		if strings.TrimSpace(note) == "" {
			return row, "", nil, questionRefusal(422, "a reassignment note is required")
		}
		if err := s.validateAnswerer(ctx, q, workspaceID, userID); err != nil {
			return row, "", nil, err
		}
		if userID == actor.ID {
			return row, "", nil, questionRefusal(422, "choose another member to reassign the question")
		}
		_, err := q.ReplaceQuestionAnswerer(ctx, db.ReplaceQuestionAnswererParams{QuestionID: row.ID, WorkspaceID: workspaceID, FromUserID: actor.ID, ToUserID: userID})
		if err != nil {
			return row, "", nil, err
		}
		updated, err := q.SetQuestionStatus(ctx, db.SetQuestionStatusParams{ID: row.ID, WorkspaceID: workspaceID, Status: "open", FromStatuses: []string{"open"}})
		return updated, "reassigned", struct {
			UserID string `json:"user_id"`
			Note   string `json:"note"`
		}{util.UUIDToString(userID), note}, err
	})
}

func (s *QuestionService) Close(ctx context.Context, workspaceID, questionID pgtype.UUID, actor QuestionActor) (QuestionDTO, error) {
	return s.end(ctx, workspaceID, questionID, actor, "closed", []string{"answered"})
}

func (s *QuestionService) Withdraw(ctx context.Context, workspaceID, questionID pgtype.UUID, actor QuestionActor) (QuestionDTO, error) {
	return s.end(ctx, workspaceID, questionID, actor, "withdrawn", []string{"open", "answered"})
}

func (s *QuestionService) end(ctx context.Context, workspaceID, questionID pgtype.UUID, actor QuestionActor, status string, from []string) (QuestionDTO, error) {
	return s.mutate(ctx, workspaceID, questionID, actor, true, func(q *db.Queries, row db.Question) (db.Question, string, any, error) {
		allowed := false
		for _, candidate := range from {
			if row.Status == candidate {
				allowed = true
				break
			}
		}
		if !allowed {
			return row, "", nil, questionRefusal(422, "question cannot be "+status+" from its current status")
		}
		updated, err := q.SetQuestionStatus(ctx, db.SetQuestionStatusParams{ID: row.ID, WorkspaceID: workspaceID, Status: status, FromStatuses: from})
		return updated, status, nil, err
	})
}

func (s *QuestionService) Get(ctx context.Context, workspaceID, questionID pgtype.UUID) (QuestionDTO, error) {
	row, err := s.Queries.GetQuestion(ctx, db.GetQuestionParams{ID: questionID, WorkspaceID: workspaceID})
	if err != nil {
		return QuestionDTO{}, questionLoadError(err)
	}
	result, err := s.hydrate(ctx, s.Queries, workspaceID, []db.Question{row})
	if err != nil {
		return QuestionDTO{}, err
	}
	return result[0], nil
}

func (s *QuestionService) ListForIssue(ctx context.Context, workspaceID, issueID pgtype.UUID) ([]QuestionDTO, error) {
	issue, err := s.Queries.GetIssue(ctx, issueID)
	if err != nil {
		return nil, questionLoadError(err)
	}
	if issue.WorkspaceID != workspaceID {
		return nil, questionRefusal(404, "issue not found")
	}
	rows, err := s.Queries.ListQuestionsByIssue(ctx, db.ListQuestionsByIssueParams{WorkspaceID: workspaceID, IssueID: issueID})
	if err != nil {
		return nil, err
	}
	return s.hydrate(ctx, s.Queries, workspaceID, rows)
}

func (s *QuestionService) hydrate(ctx context.Context, q *db.Queries, workspaceID pgtype.UUID, rows []db.Question) ([]QuestionDTO, error) {
	result := make([]QuestionDTO, len(rows))
	if len(rows) == 0 {
		return result, nil
	}
	ids := make([]pgtype.UUID, len(rows))
	index := make(map[pgtype.UUID]int, len(rows))
	for i, row := range rows {
		ids[i], index[row.ID] = row.ID, i
		var due *string
		if row.DueDate.Valid {
			date := row.DueDate.Time.Format("2006-01-02")
			due = &date
		}
		result[i] = QuestionDTO{ID: util.UUIDToString(row.ID), WorkspaceID: util.UUIDToString(workspaceID), AskerAgentID: util.UUIDToString(row.AskerAgentID), AskedInTaskID: util.UUIDToString(row.AskedInTaskID), Prompt: row.Prompt, ContextLine: questionText(row.ContextLine), AnswerType: row.AnswerType, Settings: row.Settings, SuggestedAnswer: row.SuggestedAnswer, Status: row.Status, DueDate: due, BlocksWork: row.BlocksWork, Links: []QuestionLinkDTO{}, Answerers: []QuestionAnswererDTO{}, Answers: []QuestionAnswerDTO{}, Events: []QuestionEventDTO{}, CreatedAt: questionTime(row.CreatedAt), UpdatedAt: questionTime(row.UpdatedAt)}
	}
	links, err := q.ListQuestionLinksByQuestions(ctx, db.ListQuestionLinksByQuestionsParams{WorkspaceID: workspaceID, QuestionIds: ids})
	if err != nil {
		return nil, err
	}
	issueIDs := make(map[pgtype.UUID]pgtype.UUID, len(rows))
	for _, link := range links {
		i := index[link.QuestionID]
		result[i].Links = append(result[i].Links, QuestionLinkDTO{LinkType: link.LinkType, LinkID: util.UUIDToString(link.LinkID), Position: link.Position})
		if link.LinkType == "issue" {
			issueIDs[link.QuestionID] = link.LinkID
		}
	}
	answerers, err := q.ListQuestionAnswerersByQuestions(ctx, db.ListQuestionAnswerersByQuestionsParams{WorkspaceID: workspaceID, QuestionIds: ids})
	if err != nil {
		return nil, err
	}
	for _, answerer := range answerers {
		i := index[answerer.QuestionID]
		result[i].Answerers = append(result[i].Answerers, QuestionAnswererDTO{UserID: util.UUIDToString(answerer.UserID), AssignedAt: questionTime(answerer.AssignedAt)})
	}
	answers, err := q.ListQuestionAnswersByQuestions(ctx, db.ListQuestionAnswersByQuestionsParams{WorkspaceID: workspaceID, QuestionIds: ids})
	if err != nil {
		return nil, err
	}
	for _, answer := range answers {
		i := index[answer.QuestionID]
		row := rows[i]
		result[i].Answers = append(result[i].Answers, QuestionAnswerDTO{ID: util.UUIDToString(answer.ID), AnswererUserID: util.UUIDToString(answer.AnswererUserID), Value: answer.Value, DisplayText: question.Display(row.AnswerType, row.Settings, answer.Value), Note: questionText(answer.Note), CreatedAt: questionTime(answer.CreatedAt), UpdatedAt: questionTime(answer.UpdatedAt)})
	}
	history, err := q.ListQuestionEventsByQuestions(ctx, db.ListQuestionEventsByQuestionsParams{WorkspaceID: workspaceID, QuestionIds: ids})
	if err != nil {
		return nil, err
	}
	for _, event := range history {
		i := index[event.QuestionID]
		var actorID *string
		if event.ActorID.Valid {
			id := util.UUIDToString(event.ActorID)
			actorID = &id
		}
		result[i].Events = append(result[i].Events, QuestionEventDTO{ID: util.UUIDToString(event.ID), Kind: event.Kind, ActorType: event.ActorType, ActorID: actorID, Details: event.Details, CreatedAt: questionTime(event.CreatedAt)})
	}
	type pair struct{ agent, issue pgtype.UUID }
	type wakeState struct {
		count     int64
		active    bool
		available bool
	}
	states := make(map[pair]wakeState)
	for i, row := range rows {
		if row.Status != "answered" && row.Status != "open" {
			continue
		}
		key := pair{row.AskerAgentID, issueIDs[row.ID]}
		state, ok := states[key]
		if !ok {
			agent, err := q.GetAgent(ctx, key.agent)
			if err != nil {
				return nil, err
			}
			state.available = !agent.ArchivedAt.Valid && agent.RuntimeID.Valid
			state.count, err = q.CountUnseenQuestionAnswers(ctx, db.CountUnseenQuestionAnswersParams{WorkspaceID: workspaceID, IssueID: key.issue, AgentID: key.agent})
			if err != nil {
				return nil, err
			}
			state.active, err = q.HasActiveTaskForIssueAndAgent(ctx, db.HasActiveTaskForIssueAndAgentParams{IssueID: key.issue, AgentID: key.agent})
			if err != nil {
				return nil, err
			}
			states[key] = state
		}
		result[i].UnseenAnswerCount = state.count
		result[i].WakeAvailable = row.Status == "answered" && state.count > 0 && !state.active && state.available
	}
	return result, nil
}

func (s *QuestionService) Wake(ctx context.Context, workspaceID, questionID pgtype.UUID, actor QuestionActor) (QuestionWakeResult, error) {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return QuestionWakeResult{}, err
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	if err := s.requireMember(ctx, q, workspaceID, actor); err != nil {
		return QuestionWakeResult{}, err
	}
	row, err := q.GetQuestionForUpdate(ctx, db.GetQuestionForUpdateParams{ID: questionID, WorkspaceID: workspaceID})
	if err != nil {
		return QuestionWakeResult{}, questionLoadError(err)
	}
	if row.Status != "answered" {
		return QuestionWakeResult{}, questionRefusal(422, "only answered questions can wake their asking agent")
	}
	agent, err := q.GetAgent(ctx, row.AskerAgentID)
	if err != nil {
		return QuestionWakeResult{}, err
	}
	if agent.ArchivedAt.Valid || !agent.RuntimeID.Valid {
		return QuestionWakeResult{}, questionRefusal(422, "asking agent must be active and have a runtime")
	}
	links, err := q.ListQuestionLinksByQuestions(ctx, db.ListQuestionLinksByQuestionsParams{WorkspaceID: workspaceID, QuestionIds: []pgtype.UUID{row.ID}})
	if err != nil {
		return QuestionWakeResult{}, err
	}
	var issueID pgtype.UUID
	for _, link := range links {
		if link.LinkType == "issue" {
			issueID = link.LinkID
			break
		}
	}
	// One issue lock serializes wakes even when they came from different cards.
	// The pending-task unique index also arbitrates against other enqueue paths.
	issue, err := q.LockIssueForDescriptionUpdate(ctx, db.LockIssueForDescriptionUpdateParams{ID: issueID, WorkspaceID: workspaceID})
	if err != nil {
		return QuestionWakeResult{}, questionLoadError(err)
	}
	active, err := q.HasActiveTaskForIssueAndAgent(ctx, db.HasActiveTaskForIssueAndAgentParams{IssueID: issue.ID, AgentID: row.AskerAgentID})
	if err != nil {
		return QuestionWakeResult{}, err
	}
	if active {
		return QuestionWakeResult{}, ErrDuplicatePendingTask
	}
	count, err := q.CountUnseenQuestionAnswers(ctx, db.CountUnseenQuestionAnswersParams{WorkspaceID: workspaceID, IssueID: issue.ID, AgentID: row.AskerAgentID})
	if err != nil {
		return QuestionWakeResult{}, err
	}
	if count == 0 {
		return QuestionWakeResult{}, questionRefusal(422, "there are no unseen answers")
	}
	task, err := s.Tasks.EnqueueTaskForQuestionWake(ctx, tx, issue, row.AskerAgentID, actor.ID, row.ID, count)
	if err != nil {
		return QuestionWakeResult{}, err
	}
	result := QuestionWakeResult{TaskID: util.UUIDToString(task.ID), AnswerCount: count}
	details, err := json.Marshal(struct {
		AgentID     string `json:"agent_id"`
		AnswerCount int64  `json:"answer_count"`
		TaskID      string `json:"task_id"`
	}{util.UUIDToString(row.AskerAgentID), count, result.TaskID})
	if err != nil {
		return QuestionWakeResult{}, err
	}
	if _, err := q.CreateActivity(ctx, db.CreateActivityParams{ID: dbid.NewV7(), WorkspaceID: workspaceID, IssueID: issue.ID, ActorType: pgtype.Text{String: "member", Valid: true}, ActorID: actor.ID, Action: ActivityQuestionWake, Details: details}); err != nil {
		return QuestionWakeResult{}, err
	}
	eventID, err := s.writeEvent(ctx, q, row, actor, "woken", result)
	if err != nil {
		return QuestionWakeResult{}, err
	}
	if _, err := q.SetQuestionStatus(ctx, db.SetQuestionStatusParams{ID: row.ID, WorkspaceID: workspaceID, Status: "answered", FromStatuses: []string{"answered"}}); err != nil {
		return QuestionWakeResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return QuestionWakeResult{}, err
	}
	s.Tasks.BroadcastTaskQueued(ctx, task)
	s.Tasks.NotifyTaskEnqueued(ctx, task)
	s.publish(QuestionDTO{ID: util.UUIDToString(row.ID), WorkspaceID: util.UUIDToString(workspaceID), Links: []QuestionLinkDTO{{LinkType: "issue", LinkID: util.UUIDToString(issue.ID)}}}, actor, "woken", eventID)
	slog.Info("question agent woken", "question_id", util.UUIDToString(row.ID), "task_id", result.TaskID, "answer_count", count)
	return result, nil
}
