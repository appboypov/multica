package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/service"
)

func (h *Handler) OpenCheck(w http.ResponseWriter, r *http.Request) {
	ws, actor, ok := h.questionActor(w, r, questionRoleAgent)
	if !ok {
		return
	}
	var body struct {
		IssueID   string `json:"issue_id"`
		ProjectID string `json:"project_id"`
		MemberID  string `json:"member_id"`
		Kind      string `json:"kind"`
		Link      string `json:"link"`
		Title     string `json:"title"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		writeError(w, 400, "invalid request body")
		return
	}
	issue, ok := h.loadIssueForUser(w, r, body.IssueID)
	if !ok {
		return
	}
	project, ok := parseUUIDOrBadRequest(w, body.ProjectID, "project_id")
	if !ok {
		return
	}
	member, ok := parseUUIDOrBadRequest(w, body.MemberID, "member_id")
	if !ok {
		return
	}
	result, err := h.QuestionService.OpenCheck(r.Context(), ws, actor, service.CheckInput{IssueID: issue.ID, ProjectID: project, MemberID: member, Kind: body.Kind, Link: body.Link, Title: body.Title})
	if err != nil {
		writeQuestionError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) GetCheck(w http.ResponseWriter, r *http.Request) {
	ws, actor, ok := h.questionActor(w, r, questionRoleReader)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "todo_id")
	if !ok {
		return
	}
	result, err := h.QuestionService.GetCheck(r.Context(), ws, id, actor)
	if err != nil {
		writeQuestionError(w, err)
		return
	}
	writeJSON(w, 200, result)
}

func (h *Handler) AnswerCheck(w http.ResponseWriter, r *http.Request) {
	ws, actor, ok := h.questionActor(w, r, questionRoleMember)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "todo_id")
	if !ok {
		return
	}
	var body struct {
		Answer string `json:"answer"`
		Note   string `json:"note"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		writeError(w, 400, "invalid request body")
		return
	}
	result, err := h.QuestionService.AnswerCheck(r.Context(), ws, id, actor, body.Answer, body.Note)
	if err != nil {
		writeQuestionError(w, err)
		return
	}
	writeJSON(w, 200, result)
}

func (h *Handler) ListCheckTrust(w http.ResponseWriter, r *http.Request) {
	ws, actor, ok := h.questionActor(w, r, questionRoleReader)
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "project_id")
	if !ok {
		return
	}
	result, err := h.QuestionService.ListCheckTrust(r.Context(), ws, id)
	if err != nil {
		writeQuestionError(w, err)
		return
	}
	for index := range result {
		result[index].CanManage = actor.Type == "member" && result[index].MemberID == uuidToString(actor.ID)
	}
	writeJSON(w, 200, result)
}

func (h *Handler) CheckSettings(w http.ResponseWriter, r *http.Request) {
	role := questionRoleReader
	if r.Method == http.MethodPut {
		role = questionRoleMember
	}
	ws, actor, ok := h.questionActor(w, r, role)
	if !ok {
		return
	}
	var input *service.CheckSettings
	if r.Method == http.MethodPut {
		input = &service.CheckSettings{}
		if json.NewDecoder(r.Body).Decode(input) != nil {
			writeError(w, 400, "invalid request body")
			return
		}
	}
	result, err := h.QuestionService.CheckSettings(r.Context(), ws, actor, input)
	if err != nil {
		writeQuestionError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
