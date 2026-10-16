package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/question"
	"github.com/multica-ai/multica/server/internal/service"
)

// QuestionLinkRequest references an object in the question's workspace.
type QuestionLinkRequest struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// AskRequest is the wire body of POST /api/questions.
type AskRequest struct {
	Title       string                 `json:"title"`
	Questions   []question.AskQuestion `json:"questions"`
	AnswererIDs []string               `json:"answerer_ids"`
	SquadID     string                 `json:"squad_id"`
	Policy      string                 `json:"answer_policy"`
	DueDate     string                 `json:"due_date"`
	BlocksWork  *bool                  `json:"blocks_work"`
	Links       []QuestionLinkRequest  `json:"links,omitempty"`
}

// AddLinksRequest is the wire body of POST /api/questions/{id}/links.
type AddLinksRequest struct {
	Links []QuestionLinkRequest `json:"links"`
}

// WakeRequest selects a target agent; an absent agent_id means the asker.
type WakeRequest struct {
	AgentID *string `json:"agent_id,omitempty"`
}

type questionRole uint8

const (
	questionRoleReader questionRole = iota
	questionRoleAgent
	questionRoleMember
	questionRoleAsker
)

func (h *Handler) questionActor(w http.ResponseWriter, r *http.Request, role questionRole) (pgtype.UUID, service.QuestionActor, bool) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return pgtype.UUID{}, service.QuestionActor{}, false
	}
	workspaceID := h.resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return pgtype.UUID{}, service.QuestionActor{}, false
	}
	ws, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return pgtype.UUID{}, service.QuestionActor{}, false
	}
	actorType, actorID := h.resolveActor(r, userID, workspaceID)
	if role == questionRoleAgent && (actorType != "agent" || r.Header.Get("X-Actor-Source") != "task_token") {
		writeError(w, http.StatusForbidden, "only agents ask questions")
		return ws, service.QuestionActor{}, false
	}
	if role == questionRoleMember && (actorType != "member" || isMachineCredentialActor(r)) {
		writeError(w, http.StatusForbidden, "only members answer, ask back, reassign or wake questions")
		return ws, service.QuestionActor{}, false
	}
	if role == questionRoleAsker {
		agentInRun := actorType == "agent" && r.Header.Get("X-Actor-Source") == "task_token" && r.Header.Get("X-Task-ID") != ""
		member := actorType == "member" && !isMachineCredentialActor(r)
		if !agentInRun && !member {
			writeError(w, http.StatusForbidden, "only agents in a run or members ask questions")
			return ws, service.QuestionActor{}, false
		}
	}
	id, ok := parseUUIDOrBadRequest(w, actorID, "actor_id")
	if !ok {
		return ws, service.QuestionActor{}, false
	}
	actor := service.QuestionActor{Type: actorType, ID: id}
	if role == questionRoleAgent || (role == questionRoleAsker && actorType == "agent") {
		actor.TaskID, ok = parseUUIDOrBadRequest(w, r.Header.Get("X-Task-ID"), "task_id")
		if !ok {
			return ws, actor, false
		}
	}
	return ws, actor, true
}

func writeQuestionError(w http.ResponseWriter, err error) {
	var questionErr *service.QuestionError
	switch {
	case errors.As(err, &questionErr):
		writeError(w, questionErr.Status, questionErr.Message)
	case errors.Is(err, service.ErrDuplicatePendingTask):
		writeError(w, http.StatusConflict, "agent already has a pending task on this issue")
	case errors.Is(err, pgx.ErrNoRows):
		writeError(w, http.StatusNotFound, "question not found")
	default:
		slog.Error("question request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "question request failed")
	}
}

func questionID(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
	return parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "question_id")
}

func (h *Handler) AskQuestion(w http.ResponseWriter, r *http.Request) {
	ws, actor, ok := h.questionActor(w, r, questionRoleAsker)
	if !ok {
		return
	}
	var body AskRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	answerers, ok := parseUUIDSliceOrBadRequest(w, body.AnswererIDs, "answerer_ids")
	if !ok {
		return
	}
	var squadID pgtype.UUID
	if body.SquadID != "" {
		squadID, ok = parseUUIDOrBadRequest(w, body.SquadID, "squad_id")
		if !ok {
			return
		}
	}
	links, ok := parseQuestionLinks(w, body.Links)
	if !ok {
		return
	}
	result, err := h.QuestionService.Ask(r.Context(), ws, actor, service.AskInput{
		Title: body.Title, Questions: body.Questions, AnswererIDs: answerers,
		SquadID: squadID, Policy: body.Policy, DueDate: body.DueDate, BlocksWork: body.BlocksWork, Links: links,
	})
	if err != nil {
		writeQuestionError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) ListIssueQuestions(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.questionActor(w, r, questionRoleReader); !ok {
		return
	}
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	result, err := h.QuestionService.ListForIssue(r.Context(), issue.WorkspaceID, issue.ID)
	if err != nil {
		writeQuestionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.questionResponses(r, issue.WorkspaceID, result))
}

func (h *Handler) GetQuestion(w http.ResponseWriter, r *http.Request) {
	ws, _, ok := h.questionActor(w, r, questionRoleReader)
	if !ok {
		return
	}
	id, ok := questionID(w, r)
	if !ok {
		return
	}
	result, err := h.QuestionService.Get(r.Context(), ws, id)
	if err != nil {
		writeQuestionError(w, err)
		return
	}
	h.writeQuestionResponse(w, r, ws, result)
}

func (h *Handler) AnswerQuestion(w http.ResponseWriter, r *http.Request) {
	ws, actor, ok := h.questionActor(w, r, questionRoleMember)
	if !ok {
		return
	}
	id, ok := questionID(w, r)
	if !ok {
		return
	}
	var body service.AnswerQuestionInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.QuestionService.Answer(r.Context(), ws, id, actor, body)
	if err != nil {
		writeQuestionError(w, err)
		return
	}
	h.writeQuestionResponse(w, r, ws, result)
}

func (h *Handler) AskBackQuestion(w http.ResponseWriter, r *http.Request) {
	ws, actor, ok := h.questionActor(w, r, questionRoleMember)
	if !ok {
		return
	}
	id, ok := questionID(w, r)
	if !ok {
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.QuestionService.AskBack(r.Context(), ws, id, actor, body.Text)
	if err != nil {
		writeQuestionError(w, err)
		return
	}
	h.writeQuestionResponse(w, r, ws, result)
}

func (h *Handler) ReassignQuestion(w http.ResponseWriter, r *http.Request) {
	ws, actor, ok := h.questionActor(w, r, questionRoleMember)
	if !ok {
		return
	}
	id, ok := questionID(w, r)
	if !ok {
		return
	}
	var body struct {
		UserID string `json:"user_id"`
		Note   string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	userID, ok := parseUUIDOrBadRequest(w, body.UserID, "user_id")
	if !ok {
		return
	}
	result, err := h.QuestionService.Reassign(r.Context(), ws, id, actor, userID, body.Note)
	if err != nil {
		writeQuestionError(w, err)
		return
	}
	h.writeQuestionResponse(w, r, ws, result)
}

func (h *Handler) CloseQuestion(w http.ResponseWriter, r *http.Request) {
	ws, actor, ok := h.questionActor(w, r, questionRoleAgent)
	if !ok {
		return
	}
	id, ok := questionID(w, r)
	if !ok {
		return
	}
	result, err := h.QuestionService.Close(r.Context(), ws, id, actor)
	if err != nil {
		writeQuestionError(w, err)
		return
	}
	h.writeQuestionResponse(w, r, ws, result)
}

func (h *Handler) WithdrawQuestion(w http.ResponseWriter, r *http.Request) {
	ws, actor, ok := h.questionActor(w, r, questionRoleAgent)
	if !ok {
		return
	}
	id, ok := questionID(w, r)
	if !ok {
		return
	}
	result, err := h.QuestionService.Withdraw(r.Context(), ws, id, actor)
	if err != nil {
		writeQuestionError(w, err)
		return
	}
	h.writeQuestionResponse(w, r, ws, result)
}

func (h *Handler) WakeQuestion(w http.ResponseWriter, r *http.Request) {
	ws, actor, ok := h.questionActor(w, r, questionRoleMember)
	if !ok {
		return
	}
	id, ok := questionID(w, r)
	if !ok {
		return
	}
	var body WakeRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}
	var agentID pgtype.UUID
	if body.AgentID != nil {
		agentID, ok = parseUUIDOrBadRequest(w, *body.AgentID, "agent_id")
		if !ok {
			return
		}
	}
	result, err := h.QuestionService.Wake(r.Context(), ws, id, actor, agentID)
	if err != nil {
		writeQuestionError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}

type questionAnswerResponse struct {
	service.QuestionAnswerDTO
	RunMarks []RunInputMarkResponse `json:"run_marks,omitempty"`
}

type questionResponse struct {
	service.QuestionDTO
	Answers []questionAnswerResponse `json:"answers"`
}

func (h *Handler) writeQuestionResponse(w http.ResponseWriter, r *http.Request, workspaceID pgtype.UUID, dto service.QuestionDTO) {
	writeJSON(w, http.StatusOK, h.questionResponses(r, workspaceID, []service.QuestionDTO{dto})[0])
}

// Answers display only marks for their latest version, matching Wake's query.
func (h *Handler) questionResponses(r *http.Request, workspaceID pgtype.UUID, dtos []service.QuestionDTO) []questionResponse {
	out := make([]questionResponse, len(dtos))
	itemIDs := make([]pgtype.UUID, 0)
	answerItems := make([]map[string]string, len(dtos))
	for i, dto := range dtos {
		out[i] = questionResponse{QuestionDTO: dto, Answers: make([]questionAnswerResponse, len(dto.Answers))}
		latest := make(map[string]string)
		for j := len(dto.Events) - 1; j >= 0; j-- {
			event := dto.Events[j]
			if event.ActorType != "member" || event.ActorID == nil || (event.Kind != "answered" && event.Kind != "answer_edited") {
				continue
			}
			if _, exists := latest[*event.ActorID]; !exists {
				latest[*event.ActorID] = event.ID
				itemIDs = append(itemIDs, parseUUID(event.ID))
			}
		}
		answerItems[i] = latest
	}
	marks := h.listRunInputMarks(r.Context(), workspaceID, itemIDs)
	for i, dto := range dtos {
		for j, answer := range dto.Answers {
			out[i].Answers[j] = questionAnswerResponse{QuestionAnswerDTO: answer, RunMarks: marks[answerItems[i][answer.AnswererUserID]]}
		}
	}
	return out
}
