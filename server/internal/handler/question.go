package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
)

func (h *Handler) questionActor(w http.ResponseWriter, r *http.Request, agentOnly, humanOnly bool) (pgtype.UUID, service.QuestionActor, bool) {
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
	if agentOnly && (actorType != "agent" || r.Header.Get("X-Actor-Source") != "task_token") {
		writeError(w, http.StatusForbidden, "only agents ask questions")
		return ws, service.QuestionActor{}, false
	}
	if humanOnly && (actorType != "member" || isMachineCredentialActor(r)) {
		writeError(w, http.StatusForbidden, "only members answer, ask back, reassign or wake questions")
		return ws, service.QuestionActor{}, false
	}
	id, ok := parseUUIDOrBadRequest(w, actorID, "actor_id")
	if !ok {
		return ws, service.QuestionActor{}, false
	}
	actor := service.QuestionActor{Type: actorType, ID: id}
	if agentOnly {
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
	ws, actor, ok := h.questionActor(w, r, true, false)
	if !ok {
		return
	}
	var body struct {
		Prompt          string          `json:"prompt"`
		ContextLine     string          `json:"context_line"`
		AnswerType      string          `json:"answer_type"`
		Settings        json.RawMessage `json:"settings"`
		SuggestedAnswer json.RawMessage `json:"suggested_answer"`
		AnswererIDs     []string        `json:"answerer_ids"`
		DueDate         string          `json:"due_date"`
		BlocksWork      bool            `json:"blocks_work"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	answerers, ok := parseUUIDSliceOrBadRequest(w, body.AnswererIDs, "answerer_ids")
	if !ok {
		return
	}
	result, err := h.QuestionService.Ask(r.Context(), ws, actor, service.AskQuestionInput{
		Prompt: body.Prompt, ContextLine: body.ContextLine, AnswerType: body.AnswerType,
		Settings: body.Settings, SuggestedAnswer: body.SuggestedAnswer, AnswererIDs: answerers,
		DueDate: body.DueDate, BlocksWork: body.BlocksWork,
	})
	if err != nil {
		writeQuestionError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) ListIssueQuestions(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.questionActor(w, r, false, false); !ok {
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
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GetQuestion(w http.ResponseWriter, r *http.Request) {
	ws, _, ok := h.questionActor(w, r, false, false)
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
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) AnswerQuestion(w http.ResponseWriter, r *http.Request) {
	ws, actor, ok := h.questionActor(w, r, false, true)
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
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) AskBackQuestion(w http.ResponseWriter, r *http.Request) {
	ws, actor, ok := h.questionActor(w, r, false, true)
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
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) ReassignQuestion(w http.ResponseWriter, r *http.Request) {
	ws, actor, ok := h.questionActor(w, r, false, true)
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
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) CloseQuestion(w http.ResponseWriter, r *http.Request) {
	ws, actor, ok := h.questionActor(w, r, true, false)
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
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) WithdrawQuestion(w http.ResponseWriter, r *http.Request) {
	ws, actor, ok := h.questionActor(w, r, true, false)
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
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) WakeQuestion(w http.ResponseWriter, r *http.Request) {
	ws, actor, ok := h.questionActor(w, r, false, true)
	if !ok {
		return
	}
	id, ok := questionID(w, r)
	if !ok {
		return
	}
	result, err := h.QuestionService.Wake(r.Context(), ws, id, actor)
	if err != nil {
		writeQuestionError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}
