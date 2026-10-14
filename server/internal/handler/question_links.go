package handler

import (
	"encoding/json"
	"net/http"

	"github.com/multica-ai/multica/server/internal/question"
)

func parseQuestionLinks(w http.ResponseWriter, links []QuestionLinkRequest) ([]question.LinkRef, bool) {
	refs := make([]question.LinkRef, 0, len(links))
	for _, link := range links {
		id, ok := parseUUIDOrBadRequest(w, link.ID, "link_id")
		if !ok {
			return nil, false
		}
		refs = append(refs, question.LinkRef{Type: link.Type, Ref: uuidToString(id)})
	}
	return refs, true
}

func (h *Handler) AddQuestionLinks(w http.ResponseWriter, r *http.Request) {
	workspaceID, actor, ok := h.questionActor(w, r, true, false)
	if !ok {
		return
	}
	id, ok := questionID(w, r)
	if !ok {
		return
	}
	var body AddLinksRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(body.Links) == 0 {
		writeError(w, http.StatusBadRequest, "links must not be empty")
		return
	}
	refs, ok := parseQuestionLinks(w, body.Links)
	if !ok {
		return
	}
	result, err := h.QuestionService.AddLinks(r.Context(), workspaceID, id, actor, refs)
	if err != nil {
		writeQuestionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
