package handler

import (
	"context"
	"encoding/json"
	"net/http"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type ReleasePullRequestResponse struct {
	ID       string  `json:"id"`
	Provider string  `json:"provider"`
	Number   int32   `json:"number"`
	Title    string  `json:"title"`
	State    string  `json:"state"`
	URL      string  `json:"url"`
	Branch   *string `json:"branch"`
	MergedAt *string `json:"merged_at"`
}

type ReleaseIssueResponse struct {
	ID                string                       `json:"id"`
	Number            int32                        `json:"number"`
	Identifier        string                       `json:"identifier"`
	Title             string                       `json:"title"`
	Status            string                       `json:"status"`
	StatusCategory    string                       `json:"status_category"`
	EffectiveStatus   string                       `json:"effective_status"`
	AssigneeType      *string                      `json:"assignee_type"`
	AssigneeID        *string                      `json:"assignee_id"`
	AssigneeName      *string                      `json:"assignee_name"`
	AssigneeAvatarURL *string                      `json:"assignee_avatar_url"`
	ParentIssueID     *string                      `json:"parent_issue_id"`
	ParentIdentifier  *string                      `json:"parent_identifier"`
	ParentTitle       *string                      `json:"parent_title"`
	WorklaneID        *string                      `json:"worklane_id"`
	WorklaneName      *string                      `json:"worklane_name"`
	PullRequests      []ReleasePullRequestResponse `json:"pull_requests"`
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (h *Handler) releaseIssues(ctx context.Context, q *db.Queries, rel db.Release) ([]ReleaseIssueResponse, error) {
	rows, err := q.ListReleaseIssues(ctx, db.ListReleaseIssuesParams{WorkspaceID: rel.WorkspaceID, ReleaseID: rel.ID})
	if err != nil {
		return nil, err
	}
	out := make([]ReleaseIssueResponse, len(rows))
	positions := make(map[string]int, len(rows))
	for i, row := range rows {
		out[i] = ReleaseIssueResponse{ID: uuidToString(row.ID), Number: row.Number, Identifier: row.Identifier, Title: row.Title, Status: row.Status, StatusCategory: row.StatusCategory, EffectiveStatus: row.EffectiveStatus,
			AssigneeType: textToPtr(row.AssigneeType), AssigneeID: uuidToPtr(row.AssigneeID), AssigneeName: optionalString(row.AssigneeName), AssigneeAvatarURL: h.resolveAvatarURLPtr(textToPtr(row.AssigneeAvatarUrl)),
			ParentIssueID: uuidToPtr(row.ParentIssueID), ParentIdentifier: optionalString(row.ParentIdentifier), ParentTitle: textToPtr(row.ParentTitle), WorklaneID: uuidToPtr(row.WorklaneID), WorklaneName: textToPtr(row.WorklaneName), PullRequests: []ReleasePullRequestResponse{}}
		positions[out[i].ID] = i
	}
	prs, err := q.ListReleaseIssuePullRequests(ctx, db.ListReleaseIssuePullRequestsParams{WorkspaceID: rel.WorkspaceID, ReleaseID: rel.ID})
	if err != nil {
		return nil, err
	}
	for _, pr := range prs {
		i, ok := positions[uuidToString(pr.IssueID)]
		if !ok {
			continue
		}
		out[i].PullRequests = append(out[i].PullRequests, ReleasePullRequestResponse{ID: uuidToString(pr.ID), Provider: pr.Provider, Number: pr.Number, Title: pr.Title, State: pr.State, URL: pr.Url, Branch: textToPtr(pr.Branch), MergedAt: timestampToPtr(pr.MergedAt)})
	}
	return out, nil
}

func (h *Handler) ListReleaseIssues(w http.ResponseWriter, r *http.Request) {
	rel, ok := h.loadRelease(w, r)
	if !ok {
		return
	}
	issues, err := h.releaseIssues(r.Context(), h.Queries, rel)
	if err != nil {
		writeError(w, 500, "failed to read release issues")
		return
	}
	writeJSON(w, 200, map[string]any{"issues": issues})
}

func (h *Handler) ListReleaseActivity(w http.ResponseWriter, r *http.Request) {
	rel, ok := h.loadRelease(w, r)
	if !ok {
		return
	}
	rows, err := h.Queries.ListReleaseOwnerActivity(r.Context(), db.ListReleaseOwnerActivityParams{WorkspaceID: rel.WorkspaceID, RepositoryID: rel.RepositoryID, ReleaseID: rel.ID})
	if err != nil {
		writeError(w, 500, "failed to read release activity")
		return
	}
	entries := make([]map[string]any, len(rows))
	for i, row := range rows {
		entries[i] = map[string]any{"id": uuidToString(row.ID), "actor_type": textToPtr(row.ActorType), "actor_id": uuidToPtr(row.ActorID), "action": row.Action, "details": json.RawMessage(row.Details), "created_at": timestampToString(row.CreatedAt)}
	}
	writeJSON(w, 200, map[string]any{"entries": entries})
}
