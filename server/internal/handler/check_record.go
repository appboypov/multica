package handler

import (
 "encoding/json"
 "net/http"
 "github.com/go-chi/chi/v5"
 "github.com/multica-ai/multica/server/internal/service"
)

func (h *Handler) SetCheckSampling(w http.ResponseWriter,r *http.Request) {
 ws,actor,ok:=h.questionActor(w,r,questionRoleMember);if !ok{return}
 project,ok:=parseUUIDOrBadRequest(w,chi.URLParam(r,"id"),"project_id");if !ok{return}
 var body struct { Sampling *bool `json:"sampling"` }
 if json.NewDecoder(r.Body).Decode(&body)!=nil||body.Sampling==nil {writeError(w,400,"sampling is required");return}
 result,err:=h.QuestionService.SetCheckSampling(r.Context(),ws,project,chi.URLParam(r,"kind"),actor,*body.Sampling)
 if err!=nil{writeQuestionError(w,err);return};writeJSON(w,200,result)
}

func (h *Handler) CheckHistory(w http.ResponseWriter,r *http.Request) {
 ws,_,ok:=h.questionActor(w,r,questionRoleReader);if !ok{return}
 project,ok:=parseUUIDOrBadRequest(w,chi.URLParam(r,"id"),"project_id");if !ok{return}
 result,err:=h.QuestionService.CheckHistory(r.Context(),ws,project,chi.URLParam(r,"kind"))
 if err!=nil{writeQuestionError(w,err);return};writeJSON(w,200,result)
}

func (h *Handler) OpenInstructionNote(w http.ResponseWriter,r *http.Request) {
 h.instructionReport(w,r,false)
}
func (h *Handler) ReportInstructionUndo(w http.ResponseWriter,r *http.Request) {
 h.instructionReport(w,r,true)
}
func (h *Handler) instructionReport(w http.ResponseWriter,r *http.Request,undo bool) {
 ws,actor,ok:=h.questionActor(w,r,questionRoleAgent);if !ok{return}
 id,ok:=parseUUIDOrBadRequest(w,chi.URLParam(r,"id"),"todo_id");if !ok{return}
 var body service.InstructionNoteInput
 if json.NewDecoder(r.Body).Decode(&body)!=nil{writeError(w,400,"invalid request body");return}
 var result map[string]any
 var err error
 if undo {result,err=h.QuestionService.ReportInstructionUndo(r.Context(),ws,id,actor,body)} else {result,err=h.QuestionService.OpenInstructionNote(r.Context(),ws,id,actor,body)}
 if err!=nil{writeQuestionError(w,err);return}
 status:=http.StatusOK;if !undo{status=http.StatusCreated};writeJSON(w,status,result)
}
