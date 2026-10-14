package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/issueplacement"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	"github.com/multica-ai/multica/server/internal/issuetemplate"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const (
	issueActionPlannedInRelease     = "planned_in_release"
	issueActionTypeSet              = "type_set"
	issueActionAddedToRelease       = "added_to_release"
	issueActionRemovedFromRelease   = "removed_from_release"
	issueActionWorklaneChanged      = "worklane_changed"
	issueActionReleaseGone          = "release_gone"
	issueCodeTypeInvalid            = "type_invalid"
	issueCodeTypeFixed              = "type_fixed"
	issueCodeReleaseNeedsWorklane   = "release_needs_worklane"
	issueCodeReleaseNotOfRepository = "release_not_of_repository"
	issueCodeReleaseShipped         = "release_shipped"
	issueCodeNotInRepository        = "issue_not_in_repository"
	issueCodeInRelease              = "issue_in_release"
)

type IssueRefusalIssue struct {
	ID         string `json:"id"`
	Identifier string `json:"identifier"`
	Title      string `json:"title,omitempty"`
}

type issueRefusalError struct {
	status  int
	payload map[string]any
}

func (e *issueRefusalError) Error() string { return fmt.Sprint(e.payload["error"]) }

func writeInvalidIssueType(w http.ResponseWriter) {
	writeJSON(w, http.StatusBadRequest, map[string]any{"code": issueCodeTypeInvalid, "error": "type must be one of " + strings.Join(issuetemplate.Types(), ", "), "types": issuetemplate.Types()})
}
func optionalPlacementUUID(w http.ResponseWriter, value *string, field string) (pgtype.UUID, bool) {
	if value == nil {
		return pgtype.UUID{}, true
	}
	return parseUUIDOrBadRequest(w, *value, field)
}
func writePlacementError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, service.ErrIssueTypeInvalid) {
		writeInvalidIssueType(w)
		return true
	}
	var refusal *issueRefusalError
	if errors.As(err, &refusal) {
		writeJSON(w, refusal.status, refusal.payload)
		return true
	}
	if errors.Is(err, service.ErrWorklaneNotFound) || errors.Is(err, service.ErrReleaseNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return true
	}
	var p *service.IssuePlacementRefusal
	if !errors.As(err, &p) {
		return false
	}
	status := http.StatusConflict
	message := "release is not of this worklane's repository"
	if p.Code == issueCodeReleaseNeedsWorklane {
		status = http.StatusBadRequest
		message = "a release needs a worklane"
	}
	if p.Code == issueCodeReleaseShipped {
		message = p.Version + " " + p.Title + " is shipped"
	}
	if p.Code == issueCodeReleaseNotOfRepository {
		message = p.Version + " is not a release of " + p.Repository
	}
	writeJSON(w, status, map[string]any{"code": p.Code, "error": message, "version": p.Version, "title": p.Title, "repository": p.Repository})
	return true
}

func (h *Handler) SetIssueType(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	var req struct {
		Type string `json:"type"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeError(w, 400, "invalid request body")
		return
	}
	if !issuetemplate.IsType(req.Type) {
		writeInvalidIssueType(w)
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, 500, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	updated, err := q.SetIssueTypeOnce(r.Context(), db.SetIssueTypeOnceParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID, Type: strToText(req.Type)})
	if errors.Is(err, pgx.ErrNoRows) {
		writeErrorCode(w, 409, issueCodeTypeFixed, "the issue type is already set")
		return
	}
	if err != nil {
		writeError(w, 500, "failed to set issue type")
		return
	}
	actorType, actorID := h.resolveActor(r, requestUserID(r), uuidToString(issue.WorkspaceID))
	activities, err := service.WriteIssueEntries(r.Context(), q, issue.WorkspaceID, issue.ID, actorType, actorID, []issueplacement.Entry{{Action: issueActionTypeSet, Details: map[string]any{"type": req.Type}}})
	if err != nil {
		writeError(w, 500, "failed to record issue type")
		return
	}
	if tx.Commit(r.Context()) != nil {
		writeError(w, 500, "failed to commit issue type")
		return
	}
	h.publishIssueSnapshots(r.Context(), []db.Issue{updated}, actorType, actorID)
	service.PublishIssueActivities(h.Bus, issue.WorkspaceID, actorType, actorID, activities)
	resp := issueToResponse(updated, h.getIssuePrefix(r.Context(), issue.WorkspaceID))
	h.fillStatusCategory(r.Context(), issue.WorkspaceID, &resp)
	writeJSON(w, 200, resp)
}

func (h *Handler) SetIssuePlacement(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "id"))
	if !ok {
		return
	}
	var body map[string]json.RawMessage
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		writeError(w, 400, "invalid request body")
		return
	}
	var worklane, release *string
	for _, f := range []struct {
		key    string
		target **string
	}{{"worklane_id", &worklane}, {"release_id", &release}} {
		raw, present := body[f.key]
		if !present || json.Unmarshal(raw, f.target) != nil {
			writeError(w, 400, "worklane_id and release_id are required and nullable")
			return
		}
	}
	wlID, ok := optionalPlacementUUID(w, worklane, "worklane_id")
	if !ok {
		return
	}
	relID, ok := optionalPlacementUUID(w, release, "release_id")
	if !ok {
		return
	}
	ctx := r.Context()
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		writeError(w, 500, "failed to start transaction")
		return
	}
	defer tx.Rollback(ctx)
	q := h.Queries.WithTx(tx)
	repositories := map[string]pgtype.UUID{}
	// Include releases too: a closed issue can retain a release after its worklane is removed.
	for _, id := range []pgtype.UUID{issue.WorklaneID, wlID} {
		if !id.Valid {
			continue
		}
		wl, err := service.LoadPlacementWorklane(ctx, q, issue.WorkspaceID, id)
		if err != nil {
			if !writePlacementError(w, err) {
				writeError(w, 500, "failed to load worklane")
			}
			return
		}
		repositories[wl.RepositoryID] = parseUUID(wl.RepositoryID)
	}
	for _, id := range []pgtype.UUID{issue.ReleaseID, relID} {
		if !id.Valid {
			continue
		}
		rel, err := service.LoadPlacementRelease(ctx, q, issue.WorkspaceID, id)
		if err != nil {
			if !writePlacementError(w, err) {
				writeError(w, 500, "failed to load release")
			}
			return
		}
		repositories[rel.RepositoryID] = parseUUID(rel.RepositoryID)
	}
	keys := make([]string, 0, len(repositories))
	for id := range repositories {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, err := q.LockRepositoryForLine(ctx, db.LockRepositoryForLineParams{ID: repositories[key], WorkspaceID: issue.WorkspaceID}); err != nil {
			writeError(w, 404, "repository not found")
			return
		}
	}
	rows, err := q.ListIssuesForRelease(ctx, db.ListIssuesForReleaseParams{WorkspaceID: issue.WorkspaceID, IssueIds: []pgtype.UUID{issue.ID}})
	if err != nil {
		writeError(w, 500, "failed to lock issue")
		return
	}
	if len(rows) != 1 {
		writeError(w, 404, "issue not found")
		return
	}
	if rows[0].WorklaneID != issue.WorklaneID || rows[0].ReleaseID != issue.ReleaseID {
		writeError(w, 409, "issue placement changed; reload the issue")
		return
	}
	issue, err = q.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		writeError(w, 404, "issue not found")
		return
	}
	input := issueplacement.Input{Current: issueplacement.Placement{WorklaneID: uuidToString(issue.WorklaneID), ReleaseID: uuidToString(issue.ReleaseID)}, CurrentProjectID: uuidToString(issue.ProjectID), Want: issueplacement.Placement{WorklaneID: uuidToString(wlID), ReleaseID: uuidToString(relID)}}
	if wlID.Valid {
		input.Worklane, err = service.LoadPlacementWorklane(ctx, q, issue.WorkspaceID, wlID)
		if err != nil {
			if !writePlacementError(w, err) {
				writeError(w, 500, "failed to load worklane")
			}
			return
		}
	}
	if relID.Valid {
		input.Release, err = service.LoadPlacementRelease(ctx, q, issue.WorkspaceID, relID)
		if err != nil {
			if !writePlacementError(w, err) {
				writeError(w, 500, "failed to load release")
			}
			return
		}
	}
	if issue.ReleaseID.Valid {
		input.CurrentRelease, err = service.LoadPlacementRelease(ctx, q, issue.WorkspaceID, issue.ReleaseID)
		if err != nil {
			if !writePlacementError(w, err) {
				writeError(w, 500, "failed to load release")
			}
			return
		}
	}
	result, refusal := issueplacement.Resolve(input)
	if refusal != nil {
		writePlacementError(w, &service.IssuePlacementRefusal{Refusal: *refusal})
		return
	}
	if !result.Changed {
		resp := issueToResponse(issue, h.getIssuePrefix(ctx, issue.WorkspaceID))
		h.fillStatusCategory(ctx, issue.WorkspaceID, &resp)
		writeJSON(w, 200, resp)
		return
	}
	updated, err := q.UpdateIssuePlacement(ctx, db.UpdateIssuePlacementParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID, WorklaneID: wlID, ReleaseID: relID, ProjectID: parseUUIDOrNull(result.ProjectID)})
	if err != nil {
		writeError(w, 500, "failed to update issue placement")
		return
	}
	actorType, actorID := h.resolveActor(r, requestUserID(r), uuidToString(issue.WorkspaceID))
	activities, err := service.WriteIssueEntries(ctx, q, issue.WorkspaceID, issue.ID, actorType, actorID, result.Entries)
	if err != nil {
		writeError(w, 500, "failed to record issue placement")
		return
	}
	if tx.Commit(ctx) != nil {
		writeError(w, 500, "failed to commit issue placement")
		return
	}
	h.publishIssueSnapshots(ctx, []db.Issue{updated}, actorType, actorID)
	service.PublishIssueActivities(h.Bus, issue.WorkspaceID, actorType, actorID, activities)
	resp := issueToResponse(updated, h.getIssuePrefix(ctx, issue.WorkspaceID))
	h.fillStatusCategory(ctx, issue.WorkspaceID, &resp)
	writeJSON(w, 200, resp)
}

func parseIssuePlacementIDs(values []string) ([]pgtype.UUID, error) {
	ids := make([]pgtype.UUID, 0, len(values))
	seen := map[pgtype.UUID]bool{}
	for _, value := range values {
		id, err := util.ParseUUID(value)
		if err != nil {
			return nil, err
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	return ids, nil
}

// placeIssuesInRelease runs under the repository lock, validating the entire selection before writing any issue.
func (h *Handler) placeIssuesInRelease(ctx context.Context, q *db.Queries, repo db.Repository, rel db.Release, ids []pgtype.UUID, actorType, actorID string) ([]db.Issue, []db.ActivityLog, error) {
	if rel.ShippedAt.Valid {
		return nil, nil, &service.IssuePlacementRefusal{Refusal: issueplacement.Refusal{Code: issueCodeReleaseShipped, Version: rel.Version, Title: rel.Title}}
	}
	rows, err := q.ListIssuesForRelease(ctx, db.ListIssuesForReleaseParams{WorkspaceID: repo.WorkspaceID, IssueIds: ids})
	if err != nil {
		return nil, nil, err
	}
	if len(rows) != len(ids) {
		return nil, nil, &issueRefusalError{404, map[string]any{"error": "issue not found"}}
	}
	wrong := []IssueRefusalIssue{}
	placed := []IssueRefusalIssue{}
	for _, row := range rows {
		ref := IssueRefusalIssue{ID: uuidToString(row.ID), Identifier: row.Identifier}
		if !row.WorklaneID.Valid {
			wrong = append(wrong, ref)
		} else {
			wl, err := q.GetWorklaneInWorkspace(ctx, db.GetWorklaneInWorkspaceParams{ID: row.WorklaneID, WorkspaceID: repo.WorkspaceID})
			if errors.Is(err, pgx.ErrNoRows) {
				wrong = append(wrong, ref)
			} else if err != nil {
				return nil, nil, err
			} else if wl.RepositoryID != repo.ID {
				wrong = append(wrong, ref)
			}
		}
		if row.ReleaseID.Valid {
			placed = append(placed, ref)
		}
	}
	if len(wrong) > 0 {
		return nil, nil, &issueRefusalError{409, map[string]any{"code": issueCodeNotInRepository, "error": "issues are not in this repository", "issues": wrong}}
	}
	if len(placed) > 0 {
		return nil, nil, &issueRefusalError{409, map[string]any{"code": issueCodeInRelease, "error": "issues are already in a release", "issues": placed}}
	}
	changed, err := q.SetReleaseOfIssues(ctx, db.SetReleaseOfIssuesParams{WorkspaceID: repo.WorkspaceID, ReleaseID: rel.ID, IssueIds: ids})
	if err != nil {
		return nil, nil, err
	}
	return h.releaseIssueEntries(ctx, q, repo.WorkspaceID, changed, actorType, actorID, issueActionAddedToRelease, rel)
}
func (h *Handler) releaseIssueEntries(ctx context.Context, q *db.Queries, ws pgtype.UUID, ids []pgtype.UUID, actorType, actorID, action string, rel db.Release) ([]db.Issue, []db.ActivityLog, error) {
	issues := make([]db.Issue, 0, len(ids))
	activities := []db.ActivityLog{}
	for _, id := range ids {
		issue, err := q.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: id, WorkspaceID: ws})
		if err != nil {
			return nil, nil, err
		}
		issues = append(issues, issue)
		entries, err := service.WriteIssueEntries(ctx, q, ws, id, actorType, actorID, []issueplacement.Entry{{Action: action, Details: issueplacement.ReleaseDetails(*service.PlacementRelease(rel, ""))}})
		if err != nil {
			return nil, nil, err
		}
		activities = append(activities, entries...)
	}
	return issues, activities, nil
}
func (h *Handler) AddIssuesToRelease(w http.ResponseWriter, r *http.Request) {
	rel, ok := h.loadRelease(w, r)
	if !ok {
		return
	}
	var req struct {
		IssueIDs []string `json:"issue_ids"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeError(w, 400, "invalid request body")
		return
	}
	ids, err := parseIssuePlacementIDs(req.IssueIDs)
	if err != nil {
		writeError(w, 400, "invalid issue id")
		return
	}
	ctx := r.Context()
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		writeError(w, 500, "failed to start transaction")
		return
	}
	defer tx.Rollback(ctx)
	q := h.Queries.WithTx(tx)
	repo, err := q.LockRepositoryForLine(ctx, db.LockRepositoryForLineParams{ID: rel.RepositoryID, WorkspaceID: rel.WorkspaceID})
	if err != nil {
		writeError(w, 404, "repository not found")
		return
	}
	rel, err = q.GetReleaseInWorkspace(ctx, db.GetReleaseInWorkspaceParams{ID: rel.ID, WorkspaceID: rel.WorkspaceID})
	if err != nil {
		writeError(w, 404, "release not found")
		return
	}
	actorType, actorID := h.resolveActor(r, requestUserID(r), uuidToString(rel.WorkspaceID))
	issues, activities, err := h.placeIssuesInRelease(ctx, q, repo, rel, ids, actorType, actorID)
	if err != nil {
		if !writePlacementError(w, err) {
			writeError(w, 500, "failed to add issues to release")
		}
		return
	}
	if tx.Commit(ctx) != nil {
		writeError(w, 500, "failed to commit release issues")
		return
	}
	h.publishIssueSnapshots(ctx, issues, actorType, actorID)
	service.PublishIssueActivities(h.Bus, repo.WorkspaceID, actorType, actorID, activities)
	h.publish(protocol.EventReleaseLineUpdated, uuidToString(repo.WorkspaceID), actorType, actorID, protocol.ReleaseLineEventPayload{RepositoryID: uuidToString(repo.ID)})
	responses := make([]IssueResponse, len(issues))
	fill := h.newStatusCategoryFiller(ctx, repo.WorkspaceID)
	prefix := h.getIssuePrefix(ctx, repo.WorkspaceID)
	for i, issue := range issues {
		responses[i] = issueToResponse(issue, prefix)
		fill(&responses[i])
	}
	writeJSON(w, 200, map[string]any{"issues": responses})
}
func (h *Handler) ListWorklaneIssues(w http.ResponseWriter, r *http.Request) {
	ws, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace id")
	if !ok {
		return
	}
	id, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "worklane id")
	if !ok {
		return
	}
	if _, err := h.Queries.GetWorklaneInWorkspace(r.Context(), db.GetWorklaneInWorkspaceParams{ID: id, WorkspaceID: ws}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, 404, "worklane not found")
		} else {
			writeError(w, 500, "failed to load worklane")
		}
		return
	}
	rows, err := h.Queries.ListWorklaneIssues(r.Context(), db.ListWorklaneIssuesParams{WorkspaceID: ws, WorklaneID: id})
	if err != nil {
		writeError(w, 500, "failed to list issues")
		return
	}
	responses := make([]IssueResponse, len(rows))
	fill := h.newStatusCategoryFiller(r.Context(), ws)
	prefix := h.getIssuePrefix(r.Context(), ws)
	for i, row := range rows {
		responses[i] = issueListRowToResponse(db.ListIssuesRow(row), prefix)
		fill(&responses[i])
	}
	writeJSON(w, 200, map[string]any{"issues": responses})
}
func (h *Handler) ListReleaseProgress(w http.ResponseWriter, r *http.Request) {
	ws, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace id")
	if !ok {
		return
	}
	terminal, err := h.terminalIssueStatusKeys(r.Context(), ws)
	if err != nil {
		writeError(w, 500, "failed to resolve terminal statuses")
		return
	}
	rows, err := h.Queries.ListReleaseProgress(r.Context(), db.ListReleaseProgressParams{WorkspaceID: ws, TerminalStatusKeys: terminal})
	if err != nil {
		writeError(w, 500, "failed to read release progress")
		return
	}
	totals, err := h.Queries.ListReleaseTotals(r.Context(), db.ListReleaseTotalsParams{WorkspaceID: ws, TerminalStatusKeys: terminal})
	if err != nil {
		writeError(w, 500, "failed to read release totals")
		return
	}
	result := make([]map[string]any, len(rows))
	for i, row := range rows {
		result[i] = map[string]any{"worklane_id": uuidToString(row.WorklaneID), "release_id": uuidToString(row.ReleaseID), "done": row.Done, "total": row.Total}
	}
	releases := make([]map[string]any, len(totals))
	for i, row := range totals {
		releases[i] = map[string]any{"release_id": uuidToString(row.ReleaseID), "done": row.Done, "total": row.Total}
	}
	writeJSON(w, 200, map[string]any{"rows": result, "releases": releases})
}

func (h *Handler) openWorklaneIssues(ctx context.Context, q *db.Queries, ws, id pgtype.UUID) ([]IssueRefusalIssue, error) {
	terminal, err := issuestatus.ExpandCategories(ctx, q, ws, []string{issuestatus.CategoryDone, issuestatus.CategoryClosed})
	if err != nil {
		return nil, err
	}
	rows, err := q.ListOpenIssuesOfWorklane(ctx, db.ListOpenIssuesOfWorklaneParams{WorkspaceID: ws, WorklaneID: id, TerminalStatusKeys: terminal})
	if err != nil {
		return nil, err
	}
	prefix := h.getIssuePrefix(ctx, ws)
	issues := make([]IssueRefusalIssue, len(rows))
	for i, row := range rows {
		issues[i] = IssueRefusalIssue{ID: uuidToString(row.ID), Identifier: fmt.Sprintf("%s-%d", prefix, row.Number), Title: row.Title}
	}
	return issues, nil
}
func (h *Handler) refuseWorklaneWithOpenIssues(w http.ResponseWriter, ctx context.Context, q *db.Queries, ws, id pgtype.UUID) bool {
	issues, err := h.openWorklaneIssues(ctx, q, ws, id)
	if err != nil {
		writeError(w, 500, "failed to list open issues")
		return true
	}
	if len(issues) == 0 {
		return false
	}
	writeJSON(w, 409, map[string]any{"code": "worklane_has_open_issues", "error": "worklane still has open issues", "issues": issues})
	return true
}
func clearClosedWorklaneIssues(ctx context.Context, q *db.Queries, ws, id pgtype.UUID) ([]pgtype.UUID, error) {
	terminal, err := issuestatus.ExpandCategories(ctx, q, ws, []string{issuestatus.CategoryDone, issuestatus.CategoryClosed})
	if err != nil {
		return nil, err
	}
	return q.ClearWorklaneOfIssues(ctx, db.ClearWorklaneOfIssuesParams{WorkspaceID: ws, WorklaneID: id, TerminalStatusKeys: terminal})
}

func checkProjectFollowsWorklane(ctx context.Context, q *db.Queries, issue db.Issue, project pgtype.UUID) error {
	if !issue.WorklaneID.Valid {
		return nil
	}
	wl, err := service.LoadPlacementWorklane(ctx, q, issue.WorkspaceID, issue.WorklaneID)
	if err != nil {
		return err
	}
	if wl.ProjectID != uuidToString(project) {
		return &issueRefusalError{409, map[string]any{"code": "project_follows_worklane", "error": "the issue's project follows its worklane"}}
	}
	return nil
}

type worklaneOpenIssuesError struct{ issues []IssueRefusalIssue }

func (e *worklaneOpenIssuesError) Error() string {
	identifiers := make([]string, len(e.issues))
	for i, issue := range e.issues {
		identifiers[i] = issue.Identifier
	}
	return "worklane still has the open issues " + strings.Join(identifiers, ", ")
}

// Closed issues of removed worklanes may still point at a repository's releases.
// Callers hold the repository lock and clear these before deleting the line.
func clearRepositoryIssueReleases(ctx context.Context, q *db.Queries, ws, repo pgtype.UUID) error {
	releases, err := q.ListReleaseLine(ctx, db.ListReleaseLineParams{WorkspaceID: ws, RepositoryID: repo})
	if err != nil {
		return err
	}
	for _, rel := range releases {
		if _, err := q.ClearReleaseOfIssues(ctx, db.ClearReleaseOfIssuesParams{WorkspaceID: ws, ReleaseID: rel.ID}); err != nil {
			return err
		}
	}
	return nil
}
