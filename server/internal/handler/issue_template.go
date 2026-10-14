package handler

import (
	"github.com/multica-ai/multica/server/internal/issuetemplate"
	"net/http"
)

func (h *Handler) ListIssueTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"templates": issuetemplate.All()})
}
