package handler

import (
 "net/http"
 "github.com/multica-ai/multica/server/internal/issuetemplate"
)

func (h *Handler) ListIssueTemplates(w http.ResponseWriter, r *http.Request) {
 writeJSON(w,http.StatusOK,map[string]any{"templates":issuetemplate.All()})
}
