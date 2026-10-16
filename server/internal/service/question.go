package service

import (
	"bytes"
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

type AskInput struct {
	Title       string
	Questions   []question.AskQuestion
	AnswererIDs []pgtype.UUID
	SquadID     pgtype.UUID
	Policy      string
	DueDate     string
	BlocksWork  *bool
	Links       []question.LinkRef
}

type QuestionAskResult struct {
	AskID     string        `json:"ask_id"`
	Questions []QuestionDTO `json:"questions"`
}

type AnswerQuestionInput struct {
	Value json.RawMessage
	Note  string
	// Wake wakes the asking agent once the answer is saved.
	Wake bool
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
	AskID             string                  `json:"ask_id"`
	AskPosition       int32                   `json:"ask_position"`
	AskTitle          *string                 `json:"ask_title"`
	AnswerPolicy      string                  `json:"answer_policy"`
	WorkspaceID       string                  `json:"workspace_id"`
	AskerAgentID      string                  `json:"asker_agent_id"`
	AskerUserID       string                  `json:"asker_user_id"`
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
	CreatedAt         string                  `json:"created_at"`
	UpdatedAt         string                  `json:"updated_at"`
	WakeTargets       []QuestionWakeTargetDTO `json:"wake_targets"`
}

type QuestionService struct {
	Queries   *db.Queries
	TxStarter TxStarter
	Bus       *events.Bus
	Tasks     *TaskService
	RunInputs *RunInputService
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
	if question.Declined(value) {
		return nil
	}
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

func (s *QuestionService) publish(ctx context.Context, row QuestionDTO, actor QuestionActor, reason, eventID string) {
	slog.Info("question action committed", "question_id", row.ID, "workspace_id", row.WorkspaceID, "action", reason, "actor_type", actor.Type, "actor_id", util.UUIDToString(actor.ID))
	if s.Bus == nil {
		return
	}
	workspaceID, err := util.ParseUUID(row.WorkspaceID)
	if err != nil {
		slog.Error("question publication has invalid workspace", "error", err)
		return
	}
	questionID, err := util.ParseUUID(row.ID)
	if err != nil {
		slog.Error("question publication has invalid question", "error", err)
		return
	}
	ids, err := s.CardIssueIDs(ctx, s.Queries, workspaceID, []pgtype.UUID{questionID})
	if err != nil {
		slog.Error("question publication could not load card issues", "question_id", row.ID, "error", err)
		return
	}
	issueIDs := make([]string, len(ids))
	for i, id := range ids {
		issueIDs[i] = util.UUIDToString(id)
	}
	issueID := ""
	if len(issueIDs) > 0 {
		issueID = issueIDs[0]
	}
	s.Bus.Publish(events.Event{Type: protocol.EventQuestionUpdated, WorkspaceID: row.WorkspaceID, ActorType: actor.Type, ActorID: util.UUIDToString(actor.ID), Payload: protocol.QuestionEventPayload{QuestionID: row.ID, AskID: row.AskID, IssueID: issueID, IssueIDs: issueIDs, Reason: reason, EventID: eventID}})
}

func (s *QuestionService) Ask(ctx context.Context, workspaceID pgtype.UUID, actor QuestionActor, input AskInput) (QuestionAskResult, error) {
	if err := question.ValidateAsk(input.Title, input.Questions, input.Policy); err != nil {
		return QuestionAskResult{}, questionRefusal(422, err.Error())
	}
	policy := input.Policy
	if policy == "" {
		policy = question.PolicyAnyOne
	}
	blocksWork := input.BlocksWork == nil || *input.BlocksWork
	var due pgtype.Date
	if input.DueDate != "" {
		date, err := time.Parse("2006-01-02", input.DueDate)
		if err != nil {
			return QuestionAskResult{}, questionRefusal(422, "due_date must be YYYY-MM-DD")
		}
		due = pgtype.Date{Time: date, Valid: true}
	}
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return QuestionAskResult{}, err
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	var task db.AgentTaskQueue
	var askerAgentID, askerUserID pgtype.UUID
	switch actor.Type {
	case "agent":
		task, err = s.requireAgentTask(ctx, q, workspaceID, actor)
		if err != nil {
			return QuestionAskResult{}, err
		}
		if !task.IssueID.Valid {
			return QuestionAskResult{}, questionRefusal(422, "questions are asked during an issue run")
		}
		// Decided at claim (ADR-0005): a run claimed while questions were off
		// never asks, whatever the setting says now.
		if !task.QuestionsEnabled {
			return QuestionAskResult{}, questionRefusal(403, "questions are off for this issue")
		}
		if !task.StartedAt.Valid {
			return QuestionAskResult{}, questionRefusal(422, "questions are asked during a started issue run")
		}
		askerAgentID = actor.ID
	case "member":
		if err := s.requireMember(ctx, q, workspaceID, actor); err != nil {
			return QuestionAskResult{}, err
		}
		askerUserID = actor.ID
	default:
		return QuestionAskResult{}, questionRefusal(403, "only agents in a run or members ask questions")
	}
	answerers := make([]pgtype.UUID, 0, len(input.AnswererIDs))
	seen := make(map[pgtype.UUID]bool, len(input.AnswererIDs))
	for _, id := range input.AnswererIDs {
		if seen[id] {
			continue
		}
		if err := s.validateAnswerer(ctx, q, workspaceID, id); err != nil {
			return QuestionAskResult{}, err
		}
		answerers = append(answerers, id)
		seen[id] = true
	}
	var squadID *string
	if input.SquadID.Valid {
		squad, err := q.GetSquadInWorkspace(ctx, db.GetSquadInWorkspaceParams{ID: input.SquadID, WorkspaceID: workspaceID})
		if errors.Is(err, pgx.ErrNoRows) {
			return QuestionAskResult{}, questionRefusal(422, "squad_id must be a squad of this workspace")
		}
		if err != nil {
			return QuestionAskResult{}, err
		}
		if squad.ArchivedAt.Valid {
			return QuestionAskResult{}, questionRefusal(422, "squad "+squad.Name+" is archived")
		}
		members, err := q.ListSquadMembers(ctx, squad.ID)
		if err != nil {
			return QuestionAskResult{}, err
		}
		memberCount := 0
		for _, member := range members {
			if member.MemberType != "member" {
				continue
			}
			_, err := q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{UserID: member.MemberID, WorkspaceID: workspaceID})
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			if err != nil {
				return QuestionAskResult{}, err
			}
			memberCount++
			if !seen[member.MemberID] {
				answerers = append(answerers, member.MemberID)
				seen[member.MemberID] = true
			}
		}
		if memberCount == 0 {
			return QuestionAskResult{}, questionRefusal(422, "squad "+squad.Name+" has no members, only agents")
		}
		id := util.UUIDToString(squad.ID)
		squadID = &id
	}
	if len(answerers) == 0 {
		return QuestionAskResult{}, questionRefusal(422, "at least one answerer is required")
	}
	// Resolve every suggestion before the first write, including references
	// that pure validation cannot check against workspace membership.
	for i, item := range input.Questions {
		suggested := bytes.TrimSpace(item.SuggestedAnswer)
		if len(suggested) > 0 && !bytes.Equal(suggested, []byte("null")) {
			if err := s.validateReferences(ctx, q, db.Question{WorkspaceID: workspaceID, AnswerType: item.AnswerType, Settings: item.Settings}, actor.ID, suggested); err != nil {
				var refusal *QuestionError
				if errors.As(err, &refusal) {
					return QuestionAskResult{}, questionRefusal(refusal.Status, fmt.Sprintf("question %d: %s", i+1, refusal.Message))
				}
				return QuestionAskResult{}, err
			}
		}
	}
	if len(input.Links) > question.MaxLinksPerAsk {
		return QuestionAskResult{}, questionRefusal(422, fmt.Sprintf("at most %d links are allowed", question.MaxLinksPerAsk))
	}
	refs := input.Links
	if len(refs) == 0 && actor.Type == "agent" {
		refs = []question.LinkRef{{Type: question.LinkTypeIssue, Ref: util.UUIDToString(task.IssueID)}}
	}
	resolvedLinks, err := s.resolveLinks(ctx, q, workspaceID, refs)
	if err != nil {
		return QuestionAskResult{}, err
	}
	if actor.Type == "member" {
		if len(resolvedLinks) == 0 || resolvedLinks[0].Type != question.LinkTypeIssue {
			return QuestionAskResult{}, questionRefusal(422, "a question asked outside a run is asked on an issue")
		}
		for _, link := range resolvedLinks {
			if link.Type == question.LinkTypeTask {
				return QuestionAskResult{}, questionRefusal(422, "a question asked outside a run has no run link")
			}
		}
	} else {
		runLink := ResolvedQuestionLink{Type: question.LinkTypeTask, ID: task.ID}
		hasRun := false
		for _, link := range resolvedLinks {
			if link == runLink {
				hasRun = true
				break
			}
		}
		if !hasRun {
			resolvedLinks = append(resolvedLinks, runLink)
		}
	}
	askID := dbid.NewV7()
	rows := make([]db.Question, 0, len(input.Questions))
	ids := make([]string, 0, len(input.Questions))
	questionIDs := make([]pgtype.UUID, 0, len(input.Questions))
	firstEventID := ""
	for i, item := range input.Questions {
		id := askID
		if i > 0 {
			id = dbid.NewV7()
		}
		row, err := q.CreateQuestion(ctx, db.CreateQuestionParams{ID: id, WorkspaceID: workspaceID, AskerAgentID: askerAgentID, AskedInTaskID: task.ID, AskerUserID: askerUserID, Prompt: item.Prompt, ContextLine: pgtype.Text{String: item.ContextLine, Valid: item.ContextLine != ""}, AnswerType: item.AnswerType, Settings: item.Settings, SuggestedAnswer: item.SuggestedAnswer, DueDate: due, BlocksWork: blocksWork, AskID: askID, AskPosition: int32(i), AskTitle: pgtype.Text{String: input.Title, Valid: input.Title != ""}, AnswerPolicy: policy})
		if err != nil {
			return QuestionAskResult{}, err
		}
		for position, link := range resolvedLinks {
			if err := q.CreateQuestionLink(ctx, db.CreateQuestionLinkParams{QuestionID: row.ID, WorkspaceID: workspaceID, LinkType: link.Type, LinkID: link.ID, Position: int32(position)}); err != nil {
				return QuestionAskResult{}, err
			}
		}
		if err := q.CreateQuestionAnswerers(ctx, db.CreateQuestionAnswerersParams{QuestionID: row.ID, WorkspaceID: workspaceID, UserIds: answerers}); err != nil {
			return QuestionAskResult{}, err
		}
		eventID, err := s.writeEvent(ctx, q, row, actor, "asked", struct {
			AskID        string  `json:"ask_id"`
			AnswerPolicy string  `json:"answer_policy"`
			SquadID      *string `json:"squad_id,omitempty"`
		}{util.UUIDToString(askID), policy, squadID})
		if err != nil {
			return QuestionAskResult{}, err
		}
		if i == 0 {
			firstEventID = eventID
		}
		rows = append(rows, row)
		ids = append(ids, util.UUIDToString(id))
		questionIDs = append(questionIDs, id)
	}
	hydrated, err := s.hydrate(ctx, q, workspaceID, rows)
	if err != nil {
		return QuestionAskResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return QuestionAskResult{}, err
	}
	result := QuestionAskResult{AskID: util.UUIDToString(askID), Questions: hydrated}
	slog.Info("question ask committed", "ask_id", result.AskID, "workspace_id", util.UUIDToString(workspaceID), "question_count", len(rows), "actor_id", util.UUIDToString(actor.ID))
	if s.Bus != nil {
		cardIssues, err := s.CardIssueIDs(ctx, s.Queries, workspaceID, questionIDs)
		if err != nil {
			slog.Error("question ask publication could not load card issues", "ask_id", result.AskID, "error", err)
			return result, nil
		}
		issueIDs := make([]string, len(cardIssues))
		for i, id := range cardIssues {
			issueIDs[i] = util.UUIDToString(id)
		}
		issueID := ""
		if len(issueIDs) > 0 {
			issueID = issueIDs[0]
		}
		s.Bus.Publish(events.Event{Type: protocol.EventQuestionCreated, WorkspaceID: util.UUIDToString(workspaceID), ActorType: actor.Type, ActorID: util.UUIDToString(actor.ID), Payload: protocol.QuestionEventPayload{QuestionID: ids[0], AskID: result.AskID, QuestionIDs: ids, IssueID: issueID, IssueIDs: issueIDs, Reason: "asked", EventID: firstEventID}})
	}
	return result, nil
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
	var input RunInputItem
	var issueIDs []pgtype.UUID
	if row.AskerAgentID.Valid && s.RunInputs != nil && (reason == "answered" || reason == "answer_edited" || reason == "ask_back") {
		input, issueIDs, err = s.runInputForQuestion(ctx, q, row, result[0], actor, reason, eventID)
		if err != nil {
			return QuestionDTO{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return QuestionDTO{}, err
	}
	if len(issueIDs) > 0 {
		if _, err := s.RunInputs.Deliver(ctx, issueIDs, RunInputActor{Type: actor.Type, ID: actor.ID}, input, RunInputChoice{}); err != nil {
			// The answer is committed; a delivery error must not invite a
			// second answer write. The durable question remains available.
			slog.Error("question run input delivery failed", "question_id", result[0].ID, "event_id", eventID, "error", err)
		}
	}
	s.publish(ctx, result[0], actor, reason, eventID)
	return result[0], nil
}

func (s *QuestionService) runInputForQuestion(ctx context.Context, q *db.Queries, row db.Question, dto QuestionDTO, actor QuestionActor, reason, eventID string) (RunInputItem, []pgtype.UUID, error) {
	user, err := q.GetUser(ctx, actor.ID)
	if err != nil {
		return RunInputItem{}, nil, err
	}
	asker, err := q.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: row.AskerAgentID, WorkspaceID: row.WorkspaceID})
	if err != nil {
		return RunInputItem{}, nil, err
	}
	id, err := util.ParseUUID(eventID)
	if err != nil {
		return RunInputItem{}, nil, err
	}
	content := ""
	if reason == "ask_back" {
		for _, event := range dto.Events {
			if event.ID == eventID {
				var reply struct {
					Text string `json:"text"`
				}
				if err := json.Unmarshal(event.Details, &reply); err != nil {
					return RunInputItem{}, nil, err
				}
				content = fmt.Sprintf("%s asks back: %s (question %q by %s)", user.Name, reply.Text, row.Prompt, asker.Name)
				break
			}
		}
	} else {
		for _, answer := range dto.Answers {
			if answer.AnswererUserID != util.UUIDToString(actor.ID) {
				continue
			}
			verb := "answered"
			if reason == "answer_edited" {
				verb = "changed the answer to"
			}
			content = fmt.Sprintf("%s %s %s's question %q: %s", user.Name, verb, asker.Name, row.Prompt, answer.DisplayText)
			if answer.Note != nil && *answer.Note != "" {
				content += "\n" + *answer.Note
			}
			break
		}
	}
	issueIDs, err := s.CardIssueIDs(ctx, q, row.WorkspaceID, []pgtype.UUID{row.ID})
	return RunInputItem{WorkspaceID: row.WorkspaceID, Kind: "answer", ID: id, Content: content}, issueIDs, err
}

// recomputeStatus is called only for live questions while their row is locked.
func (s *QuestionService) recomputeStatus(ctx context.Context, q *db.Queries, row db.Question) (db.Question, error) {
	answerers, err := q.ListQuestionAnswerersByQuestions(ctx, db.ListQuestionAnswerersByQuestionsParams{WorkspaceID: row.WorkspaceID, QuestionIds: []pgtype.UUID{row.ID}})
	if err != nil {
		return row, err
	}
	answers, err := q.ListCurrentAnswers(ctx, db.ListCurrentAnswersParams{WorkspaceID: row.WorkspaceID, QuestionID: row.ID})
	if err != nil {
		return row, err
	}
	answererIDs := make([]pgtype.UUID, len(answerers))
	for i, answerer := range answerers {
		answererIDs[i] = answerer.UserID
	}
	answeredIDs := make([]pgtype.UUID, len(answers))
	for i, answer := range answers {
		answeredIDs[i] = answer.AnswererUserID
	}
	return q.SetQuestionStatus(ctx, db.SetQuestionStatusParams{ID: row.ID, WorkspaceID: row.WorkspaceID, Status: question.DeriveStatus(row.AnswerPolicy, answererIDs, answeredIDs), FromStatuses: []string{row.Status}})
}

// Answer saves the actor's answer or edit. With Wake, the asking agent starts
// once its existing tasks and retries on the issue have finished.
func (s *QuestionService) Answer(ctx context.Context, workspaceID, questionID pgtype.UUID, actor QuestionActor, input AnswerQuestionInput) (QuestionDTO, error) {
	result, err := s.mutate(ctx, workspaceID, questionID, actor, false, func(q *db.Queries, row db.Question) (db.Question, string, any, error) {
		if !question.IsOpen(row.Status) && row.Status != "answered" {
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
		row, err = s.recomputeStatus(ctx, q, row)
		reason := "answer_edited"
		if answer.Inserted {
			reason = "answered"
		}
		return row, reason, nil, err
	})
	if err != nil || !input.Wake {
		return result, err
	}
	if _, err := s.Wake(ctx, workspaceID, questionID, actor, pgtype.UUID{}); err != nil {
		slog.Info("answer did not wake the asker", "question_id", util.UUIDToString(questionID), "error", err)
	}
	return result, nil
}

// AskBack replies in the question's thread, keeps its status and requests the
// same durable wake as an answer sent with Wake.
func (s *QuestionService) AskBack(ctx context.Context, workspaceID, questionID pgtype.UUID, actor QuestionActor, text string) (QuestionDTO, error) {
	result, err := s.mutate(ctx, workspaceID, questionID, actor, false, func(q *db.Queries, row db.Question) (db.Question, string, any, error) {
		if !question.IsOpen(row.Status) && row.Status != "answered" {
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
	if err != nil {
		return result, err
	}
	if _, err := s.Wake(ctx, workspaceID, questionID, actor, pgtype.UUID{}); err != nil {
		slog.Info("ask back did not wake the asker", "question_id", util.UUIDToString(questionID), "error", err)
	}
	return result, nil
}

func (s *QuestionService) Reassign(ctx context.Context, workspaceID, questionID pgtype.UUID, actor QuestionActor, userID pgtype.UUID, note string) (QuestionDTO, error) {
	return s.mutate(ctx, workspaceID, questionID, actor, false, func(q *db.Queries, row db.Question) (db.Question, string, any, error) {
		if !question.IsOpen(row.Status) {
			return row, "", nil, questionRefusal(422, "only open or partly answered questions can be reassigned")
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
		updated, err := s.recomputeStatus(ctx, q, row)
		return updated, "reassigned", struct {
			UserID string `json:"user_id"`
			Note   string `json:"note"`
		}{util.UUIDToString(userID), note}, err
	})
}

func (s *QuestionService) Close(ctx context.Context, workspaceID, questionID pgtype.UUID, actor QuestionActor) (QuestionDTO, error) {
	return s.end(ctx, workspaceID, questionID, actor, "closed", []string{"partly_answered", "answered"})
}

func (s *QuestionService) Withdraw(ctx context.Context, workspaceID, questionID pgtype.UUID, actor QuestionActor) (QuestionDTO, error) {
	return s.end(ctx, workspaceID, questionID, actor, "withdrawn", []string{"open", "partly_answered", "answered"})
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
		result[i] = QuestionDTO{ID: util.UUIDToString(row.ID), AskID: util.UUIDToString(row.AskID), AskPosition: row.AskPosition, AskTitle: questionText(row.AskTitle), AnswerPolicy: row.AnswerPolicy, WorkspaceID: util.UUIDToString(workspaceID), AskerAgentID: util.UUIDToString(row.AskerAgentID), AskerUserID: util.UUIDToString(row.AskerUserID), AskedInTaskID: util.UUIDToString(row.AskedInTaskID), Prompt: row.Prompt, ContextLine: questionText(row.ContextLine), AnswerType: row.AnswerType, Settings: row.Settings, SuggestedAnswer: row.SuggestedAnswer, Status: row.Status, DueDate: due, BlocksWork: row.BlocksWork, Links: []QuestionLinkDTO{}, Answerers: []QuestionAnswererDTO{}, Answers: []QuestionAnswerDTO{}, Events: []QuestionEventDTO{}, CreatedAt: questionTime(row.CreatedAt), UpdatedAt: questionTime(row.UpdatedAt)}
	}
	links, err := q.ListQuestionLinksByQuestions(ctx, db.ListQuestionLinksByQuestionsParams{WorkspaceID: workspaceID, QuestionIds: ids})
	if err != nil {
		return nil, err
	}
	for _, link := range links {
		i := index[link.QuestionID]
		result[i].Links = append(result[i].Links, QuestionLinkDTO{LinkType: link.LinkType, LinkID: util.UUIDToString(link.LinkID), Position: link.Position})
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
	for i, row := range rows {
		targets, err := s.wakeTargets(ctx, q, workspaceID, row)
		if err != nil {
			return nil, err
		}
		result[i].WakeTargets = targets
		asker := util.UUIDToString(row.AskerAgentID)
		found := false
		for _, target := range targets {
			if target.AgentID == asker {
				result[i].UnseenAnswerCount = target.UnseenAnswerCount
				result[i].WakeAvailable = target.WakeAvailable
				found = true
				break
			}
		}
		if !found && row.AskedInTaskID.Valid {
			// An archived asker is no Wake target, but the card still counts the
			// answers it has not seen on its run's issue.
			asking, err := q.GetAgentTaskInWorkspace(ctx, db.GetAgentTaskInWorkspaceParams{ID: row.AskedInTaskID, WorkspaceID: workspaceID})
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return nil, err
			}
			if asking.IssueID.Valid {
				result[i].UnseenAnswerCount, err = s.countUnseenAnswers(ctx, q, workspaceID, row.AskerAgentID, asking.IssueID, pgtype.UUID{})
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return result, nil
}

func (s *QuestionService) Wake(ctx context.Context, workspaceID, questionID pgtype.UUID, actor QuestionActor, agentID pgtype.UUID) (QuestionWakeResult, error) {
	if err := s.requireMember(ctx, s.Queries, workspaceID, actor); err != nil {
		return QuestionWakeResult{}, err
	}
	// Resolve without a write lock: every writer takes issue, pending, question.
	row, err := s.Queries.GetQuestion(ctx, db.GetQuestionParams{ID: questionID, WorkspaceID: workspaceID})
	if err != nil {
		return QuestionWakeResult{}, questionLoadError(err)
	}
	if !row.AskerAgentID.Valid {
		return QuestionWakeResult{}, questionRefusal(422, "a question a member asked wakes no agent")
	}
	if !agentID.Valid {
		agentID = row.AskerAgentID
	}
	issueID, err := s.questionWakeIssue(ctx, s.Queries, workspaceID, row, agentID)
	if err != nil {
		return QuestionWakeResult{}, err
	}
	initialAgent, err := s.Queries.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: agentID, WorkspaceID: workspaceID})
	if err != nil {
		return QuestionWakeResult{}, err
	}
	if initialAgent.ArchivedAt.Valid || !initialAgent.RuntimeID.Valid {
		return QuestionWakeResult{}, questionRefusal(422, "agent must be active and have a runtime")
	}
	if err := s.settleQuestionWakeRetries(ctx, workspaceID, agentID, issueID); err != nil {
		return QuestionWakeResult{}, err
	}
	tx, canDeliver, err := s.beginQuestionWakeDelivery(ctx, issueID)
	if err != nil {
		return QuestionWakeResult{}, err
	}
	defer tx.Rollback(ctx)
	q := s.Queries.WithTx(tx)
	if err := s.requireMember(ctx, q, workspaceID, actor); err != nil {
		return QuestionWakeResult{}, err
	}
	issue, err := q.LockIssueForDescriptionUpdate(ctx, db.LockIssueForDescriptionUpdateParams{ID: issueID, WorkspaceID: workspaceID})
	if err != nil {
		return QuestionWakeResult{}, questionLoadError(err)
	}
	pending, err := q.GetQuestionWakePendingForUpdate(ctx, db.GetQuestionWakePendingForUpdateParams{WorkspaceID: workspaceID, IssueID: issueID, AgentID: agentID})
	waiting := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return QuestionWakeResult{}, err
	}
	row, err = q.GetQuestionForUpdate(ctx, db.GetQuestionForUpdateParams{ID: questionID, WorkspaceID: workspaceID})
	if err != nil {
		return QuestionWakeResult{}, questionLoadError(err)
	}
	currentIssue, err := s.questionWakeIssue(ctx, q, workspaceID, row, agentID)
	if err != nil {
		return QuestionWakeResult{}, err
	}
	if currentIssue != issueID {
		return QuestionWakeResult{}, questionRefusal(422, "Wake target changed")
	}
	agent, err := q.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: agentID, WorkspaceID: workspaceID})
	if err != nil {
		return QuestionWakeResult{}, err
	}
	if agent.ArchivedAt.Valid || !agent.RuntimeID.Valid {
		return QuestionWakeResult{}, questionRefusal(422, "agent must be active and have a runtime")
	}
	count, err := s.countUnseenAnswers(ctx, q, workspaceID, agentID, issueID, pgtype.UUID{})
	if err != nil {
		return QuestionWakeResult{}, err
	}
	askBacks, err := s.hasUnseenAskBacks(ctx, q, workspaceID, agentID, issueID)
	if err != nil {
		return QuestionWakeResult{}, err
	}
	blocked, err := s.questionWakeBlocked(ctx, q, workspaceID, agentID, issueID)
	if err != nil {
		return QuestionWakeResult{}, err
	}
	if !canDeliver || blocked {
		anchor, err := q.GetQuestionWakeAnswerAnchor(ctx, db.GetQuestionWakeAnswerAnchorParams{WorkspaceID: workspaceID, IssueID: issueID, AgentID: agentID})
		if err != nil {
			return QuestionWakeResult{}, err
		}
		if _, err := q.UpsertQuestionWakePending(ctx, db.UpsertQuestionWakePendingParams{WorkspaceID: workspaceID, IssueID: issueID, AgentID: agentID, ActorUserID: actor.ID, SourceQuestionID: row.ID, AnswerAnchor: anchor}); err != nil {
			return QuestionWakeResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return QuestionWakeResult{}, err
		}
		return QuestionWakeResult{AnswerCount: count}, nil
	}
	if !waiting && count == 0 && !askBacks {
		return QuestionWakeResult{}, questionRefusal(422, "there are no unseen answers")
	}
	if !waiting {
		anchor, err := q.GetQuestionWakeAnswerAnchor(ctx, db.GetQuestionWakeAnswerAnchorParams{WorkspaceID: workspaceID, IssueID: issueID, AgentID: agentID})
		if err != nil {
			return QuestionWakeResult{}, err
		}
		if _, err := q.UpsertQuestionWakePending(ctx, db.UpsertQuestionWakePendingParams{WorkspaceID: workspaceID, IssueID: issueID, AgentID: agentID, ActorUserID: actor.ID, SourceQuestionID: row.ID, AnswerAnchor: anchor}); err != nil {
			return QuestionWakeResult{}, err
		}
	}
	// An unrelated enqueue need not hold the issue lock. Preserve acceptance
	// if it takes the pending slot after our blocker read.
	if _, err := tx.Exec(ctx, "SAVEPOINT question_wake_enqueue"); err != nil {
		return QuestionWakeResult{}, err
	}
	var delivery *questionWakeDelivery
	if waiting {
		delivery, err = s.drainQuestionWakeLocked(ctx, tx, issue, pending)
	} else {
		delivery, err = s.enqueueQuestionWake(ctx, tx, issue, row, actor, agentID, count, pgtype.Timestamptz{})
	}
	if errors.Is(err, ErrDuplicatePendingTask) {
		if _, rollbackErr := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT question_wake_enqueue"); rollbackErr != nil {
			return QuestionWakeResult{}, rollbackErr
		}
		if err := tx.Commit(ctx); err != nil {
			return QuestionWakeResult{}, err
		}
		return QuestionWakeResult{AnswerCount: count}, nil
	}
	if err != nil {
		return QuestionWakeResult{}, err
	}
	if !waiting {
		if err := q.DeleteQuestionWakePending(ctx, db.DeleteQuestionWakePendingParams{WorkspaceID: workspaceID, IssueID: issueID, AgentID: agentID}); err != nil {
			return QuestionWakeResult{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return QuestionWakeResult{}, err
	}
	if delivery == nil {
		return QuestionWakeResult{}, questionRefusal(422, "there are no unseen answers")
	}
	s.publishQuestionWake(ctx, delivery)
	return delivery.result, nil
}
