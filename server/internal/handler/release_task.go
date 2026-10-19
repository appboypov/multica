package handler

import (
	"net/http"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// ListReleaseTasks returns the runs of a release, newest first: the
// execution history behind the release page, in the shape ListTasksByIssue
// gives an issue page.
func (h *Handler) ListReleaseTasks(w http.ResponseWriter, r *http.Request) {
	rel, ok := h.loadRelease(w, r)
	if !ok {
		return
	}
	tasks, err := h.Queries.ListTasksByRelease(r.Context(), rel.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list release runs")
		return
	}
	tasks = visibleTaskHistory(tasks)
	workspaceID := uuidToString(rel.WorkspaceID)
	resp := make([]AgentTaskResponse, len(tasks))
	for i, t := range tasks {
		resp[i] = taskToResponse(t, workspaceID)
	}
	h.hydrateTaskAttributions(r.Context(), attributionsOf(resp))
	h.hydrateReleaseTaskUsage(r, rel, resp)
	writeJSON(w, http.StatusOK, resp)
}

// hydrateReleaseTaskUsage attaches each run's token usage. Usage is display
// metadata: a failed read leaves every run without a usage figure.
func (h *Handler) hydrateReleaseTaskUsage(r *http.Request, rel db.Release, resp []AgentTaskResponse) {
	if len(resp) == 0 {
		return
	}
	rows, err := h.Queries.ListReleaseTaskUsage(r.Context(), rel.ID)
	if err != nil || len(rows) == 0 {
		return
	}
	byTask := make(map[string][]TaskUsageData, len(resp))
	for _, row := range rows {
		appendTaskUsage(byTask, row.TaskID, row.Provider, row.Model,
			row.InputTokens, row.OutputTokens, row.CacheReadTokens,
			row.CacheWriteTokens, row.CostUsdTicks)
	}
	attachTaskUsage(resp, byTask)
}
