package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/issuestatus"
	"github.com/multica-ai/multica/server/internal/logger"
	"github.com/multica-ai/multica/server/internal/question"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	"github.com/multica-ai/multica/server/internal/workpath"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type ProjectResponse struct {
	ID             string  `json:"id"`
	WorkspaceID    string  `json:"workspace_id"`
	Title          string  `json:"title"`
	Description    *string `json:"description"`
	Icon           *string `json:"icon"`
	Status         string  `json:"status"`
	Priority       string  `json:"priority"`
	LeadType       *string `json:"lead_type"`
	LeadID         *string `json:"lead_id"`
	OrganisationID *string `json:"organisation_id"`
	// Slug names the project's folder under its organisation's folder in the
	// work root (ADR-0037); null until set.
	Slug *string `json:"slug"`
	// StartDate / DueDate are calendar days ("YYYY-MM-DD"), no time-of-day or
	// timezone — same contract as issue.start_date / issue.due_date.
	StartDate  *string `json:"start_date"`
	DueDate    *string `json:"due_date"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
	IssueCount int64   `json:"issue_count"`
	DoneCount  int64   `json:"done_count"`
}

func projectToResponse(p db.Project) ProjectResponse {
	return ProjectResponse{
		ID:             uuidToString(p.ID),
		WorkspaceID:    uuidToString(p.WorkspaceID),
		Title:          p.Title,
		Description:    textToPtr(p.Description),
		Icon:           textToPtr(p.Icon),
		Status:         p.Status,
		Priority:       p.Priority,
		LeadType:       textToPtr(p.LeadType),
		LeadID:         uuidToPtr(p.LeadID),
		OrganisationID: uuidToPtr(p.OrganisationID),
		Slug:           textToPtr(p.Slug),
		StartDate:      dateToPtr(p.StartDate),
		DueDate:        dateToPtr(p.DueDate),
		CreatedAt:      timestampToString(p.CreatedAt),
		UpdatedAt:      timestampToString(p.UpdatedAt),
	}
}

func (h *Handler) loadProjectIssueStats(ctx context.Context, workspaceID, projectID pgtype.UUID) (int64, int64) {
	terminalStatusKeys := h.projectTerminalIssueStatusKeys(ctx, workspaceID)
	stats, err := h.Queries.GetProjectIssueStats(ctx, db.GetProjectIssueStatsParams{
		WorkspaceID:        workspaceID,
		ProjectIds:         []pgtype.UUID{projectID},
		TerminalStatusKeys: terminalStatusKeys,
	})
	if err != nil || len(stats) == 0 {
		return 0, 0
	}
	return stats[0].TotalCount, stats[0].DoneCount
}

// projectTerminalIssueStatusKeys keeps project responses useful if the custom
// status catalog cannot be read. Canonical terminal keys are less complete
// than the workspace catalog, but they avoid rendering every project as 0/0.
func (h *Handler) projectTerminalIssueStatusKeys(ctx context.Context, workspaceID pgtype.UUID) []string {
	keys, err := h.terminalIssueStatusKeys(ctx, workspaceID)
	if err == nil {
		return keys
	}
	slog.Warn("expand project terminal status categories failed; using canonical keys",
		"workspace_id", uuidToString(workspaceID), "error", err)
	return []string{issuestatus.Done, issuestatus.Cancelled}
}

type CreateProjectRequest struct {
	Title          string  `json:"title"`
	Description    *string `json:"description"`
	Icon           *string `json:"icon"`
	Status         string  `json:"status"`
	Priority       string  `json:"priority"`
	LeadType       *string `json:"lead_type"`
	LeadID         *string `json:"lead_id"`
	StartDate      *string `json:"start_date"`
	DueDate        *string `json:"due_date"`
	OrganisationID *string `json:"organisation_id"`
	Slug           *string `json:"slug"`
}

type UpdateProjectRequest struct {
	Title          *string `json:"title"`
	Description    *string `json:"description"`
	Icon           *string `json:"icon"`
	Status         *string `json:"status"`
	Priority       *string `json:"priority"`
	LeadType       *string `json:"lead_type"`
	LeadID         *string `json:"lead_id"`
	StartDate      *string `json:"start_date"`
	DueDate        *string `json:"due_date"`
	OrganisationID *string `json:"organisation_id"`
	Slug           *string `json:"slug"`
}

func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	var statusFilter pgtype.Text
	if s := r.URL.Query().Get("status"); s != "" {
		statusFilter = pgtype.Text{String: s, Valid: true}
	}
	var priorityFilter pgtype.Text
	if p := r.URL.Query().Get("priority"); p != "" {
		priorityFilter = pgtype.Text{String: p, Valid: true}
	}
	projects, err := h.Queries.ListProjects(r.Context(), db.ListProjectsParams{
		WorkspaceID: wsUUID,
		Status:      statusFilter,
		Priority:    priorityFilter,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list projects")
		return
	}

	// Batch-fetch issue stats for all projects
	statsMap := make(map[string]db.GetProjectIssueStatsRow)
	if len(projects) > 0 {
		projectIDs := make([]pgtype.UUID, len(projects))
		for i, p := range projects {
			projectIDs[i] = p.ID
		}
		terminalStatusKeys := h.projectTerminalIssueStatusKeys(r.Context(), wsUUID)
		stats, statsErr := h.Queries.GetProjectIssueStats(r.Context(), db.GetProjectIssueStatsParams{
			WorkspaceID:        wsUUID,
			ProjectIds:         projectIDs,
			TerminalStatusKeys: terminalStatusKeys,
		})
		if statsErr == nil {
			for _, s := range stats {
				statsMap[uuidToString(s.ProjectID)] = s
			}
		}
	}

	resp := make([]ProjectResponse, len(projects))
	for i, p := range projects {
		resp[i] = projectToResponse(p)
		if s, ok := statsMap[resp[i].ID]; ok {
			resp[i].IssueCount = s.TotalCount
			resp[i].DoneCount = s.DoneCount
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": resp, "total": len(resp)})
}

func (h *Handler) GetProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	workspaceID := h.resolveWorkspaceID(r)
	idUUID, ok := parseUUIDOrBadRequest(w, id, "project id")
	if !ok {
		return
	}
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	project, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{
		ID: idUUID, WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	resp := projectToResponse(project)
	resp.IssueCount, resp.DoneCount = h.loadProjectIssueStats(r.Context(), wsUUID, project.ID)
	writeJSON(w, http.StatusOK, resp)
}

// validProjectStatuses / validProjectPriorities mirror the CHECK constraints on
// the project table (migrations 034, 035). CreateProject / UpdateProject
// pre-validate against these so an unknown enum value returns a clean 400 with
// the allowed list instead of surfacing the DB CHECK violation as a 500 — the
// exact mismatch reported in #3925 (`--status active`).
var validProjectStatuses = []string{"planned", "in_progress", "paused", "completed", "cancelled"}
var validProjectPriorities = []string{"urgent", "high", "medium", "low", "none"}

// validateProjectEnum writes a 400 and returns false when value is not in
// allowed; the caller returns immediately on false.
func validateProjectEnum(w http.ResponseWriter, field, value string, allowed []string) bool {
	for _, a := range allowed {
		if value == a {
			return true
		}
	}
	writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid %s %q; valid values: %s", field, value, strings.Join(allowed, ", ")))
	return false
}

// writeProjectWriteError maps a failed project INSERT/UPDATE to an HTTP
// response. A CHECK constraint violation is a client error (400) — pre-validation
// already covers status/priority, so this backstops any other constrained column
// (e.g. lead_type). Anything else is a genuine server fault: log the underlying
// error so transient DB failures are diagnosable (#3925 had no server-side
// signal) and return 500.
func (h *Handler) writeProjectWriteError(w http.ResponseWriter, r *http.Request, err error, action string) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "project_organisation_slug_unique" {
		writeErrorCode(w, http.StatusConflict, "project_slug_taken", "another project in this organisation uses this slug")
		return
	}
	if isCheckViolation(err) {
		writeError(w, http.StatusBadRequest, "project "+action+" rejected: a field value failed a database constraint")
		return
	}
	slog.Error("project "+action+" failed", append(logger.RequestAttrs(r), "error", err)...)
	writeError(w, http.StatusInternalServerError, "failed to "+action+" project")
}

// resolveProjectSlug reads a slug from a request: nil or blank clears it, any
// other value must follow the folder-name rule (ADR-0037). It writes the 400
// itself.
func resolveProjectSlug(w http.ResponseWriter, slug *string) (pgtype.Text, bool) {
	if slug == nil || strings.TrimSpace(*slug) == "" {
		return pgtype.Text{}, true
	}
	value := strings.TrimSpace(*slug)
	if !workpath.ValidSlug(value) {
		writeErrorCode(w, http.StatusBadRequest, "project_slug_invalid", "a slug has lowercase letters, digits and dashes, and starts with a letter or digit")
		return pgtype.Text{}, false
	}
	return pgtype.Text{String: value, Valid: true}, true
}

// writeProjectFolderRequired answers 400 when a project that holds or gets
// repositories lacks the organisation or the slug its folder needs
// (ADR-0037). It reports whether it wrote.
func writeProjectFolderRequired(w http.ResponseWriter, organisationID pgtype.UUID, slug pgtype.Text) bool {
	if !organisationID.Valid {
		writeErrorCode(w, http.StatusBadRequest, "project_organisation_required", "a project with repositories needs an organisation")
		return true
	}
	if !slug.Valid || slug.String == "" {
		writeErrorCode(w, http.StatusBadRequest, "project_slug_required", "a project with repositories needs a slug")
		return true
	}
	return false
}

func (h *Handler) resolveProjectOrganisation(w http.ResponseWriter, r *http.Request, workspaceID pgtype.UUID, id *string) (pgtype.UUID, bool) {
	if id == nil {
		return pgtype.UUID{}, true
	}
	orgUUID, ok := parseUUIDOrBadRequest(w, *id, "organisation_id")
	if !ok {
		return pgtype.UUID{}, false
	}
	_, err := h.Queries.GetOrganisationInWorkspace(r.Context(), db.GetOrganisationInWorkspaceParams{
		ID: orgUUID, WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusBadRequest, "organisation not found")
		} else {
			slog.Error("resolve project organisation failed", append(logger.RequestAttrs(r), "error", err)...)
			writeError(w, http.StatusInternalServerError, "failed to get organisation")
		}
		return pgtype.UUID{}, false
	}
	return orgUUID, true
}

func (h *Handler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	workspaceID := h.resolveWorkspaceID(r)
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	status := req.Status
	if status == "" {
		status = "planned"
	}
	if !validateProjectEnum(w, "status", status, validProjectStatuses) {
		return
	}
	priority := req.Priority
	if priority == "" {
		priority = "none"
	}
	if !validateProjectEnum(w, "priority", priority, validProjectPriorities) {
		return
	}
	var leadType pgtype.Text
	var leadID pgtype.UUID
	if req.LeadType != nil {
		leadType = pgtype.Text{String: *req.LeadType, Valid: true}
	}
	if req.LeadID != nil {
		id, ok := parseUUIDOrBadRequest(w, *req.LeadID, "lead_id")
		if !ok {
			return
		}
		leadID = id
	}
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	organisationID, ok := h.resolveProjectOrganisation(w, r, wsUUID, req.OrganisationID)
	if !ok {
		return
	}
	slug, ok := resolveProjectSlug(w, req.Slug)
	if !ok {
		return
	}

	// start_date / due_date are optional calendar days; an absent or empty
	// value leaves the column NULL. Mirrors CreateIssue's date handling.
	var startDate pgtype.Date
	if req.StartDate != nil && *req.StartDate != "" {
		d, err := util.ParseCalendarDate(*req.StartDate)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid start_date format, expected YYYY-MM-DD")
			return
		}
		startDate = d
	}
	var dueDate pgtype.Date
	if req.DueDate != nil && *req.DueDate != "" {
		d, err := util.ParseCalendarDate(*req.DueDate)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid due_date format, expected YYYY-MM-DD")
			return
		}
		dueDate = d
	}

	createParams := db.CreateProjectParams{
		WorkspaceID:    wsUUID,
		Title:          req.Title,
		Description:    ptrToText(req.Description),
		Icon:           ptrToText(req.Icon),
		Status:         status,
		LeadType:       leadType,
		LeadID:         leadID,
		Priority:       priority,
		StartDate:      startDate,
		DueDate:        dueDate,
		OrganisationID: organisationID,
		Slug:           slug,
	}

	project, err := h.Queries.CreateProject(r.Context(), createParams)
	if err != nil {
		h.writeProjectWriteError(w, r, err, "create")
		return
	}
	resp := projectToResponse(project)
	h.publish(protocol.EventProjectCreated, workspaceID, "member", userID, map[string]any{"project": resp})
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	workspaceID := h.resolveWorkspaceID(r)
	idUUID, ok := parseUUIDOrBadRequest(w, id, "project id")
	if !ok {
		return
	}
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	prevProject, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{
		ID: idUUID, WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	var req UpdateProjectRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var rawFields map[string]json.RawMessage
	json.Unmarshal(bodyBytes, &rawFields)

	params := db.UpdateProjectParams{
		ID:             prevProject.ID,
		Description:    prevProject.Description,
		Icon:           prevProject.Icon,
		LeadType:       prevProject.LeadType,
		LeadID:         prevProject.LeadID,
		StartDate:      prevProject.StartDate,
		DueDate:        prevProject.DueDate,
		OrganisationID: prevProject.OrganisationID,
		Slug:           prevProject.Slug,
	}
	if req.Title != nil {
		params.Title = pgtype.Text{String: *req.Title, Valid: true}
	}
	if req.Status != nil {
		if !validateProjectEnum(w, "status", *req.Status, validProjectStatuses) {
			return
		}
		params.Status = pgtype.Text{String: *req.Status, Valid: true}
	}
	if req.Priority != nil {
		if !validateProjectEnum(w, "priority", *req.Priority, validProjectPriorities) {
			return
		}
		params.Priority = pgtype.Text{String: *req.Priority, Valid: true}
	}
	if _, ok := rawFields["description"]; ok {
		if req.Description != nil {
			params.Description = pgtype.Text{String: *req.Description, Valid: true}
		} else {
			params.Description = pgtype.Text{Valid: false}
		}
	}
	if _, ok := rawFields["icon"]; ok {
		if req.Icon != nil {
			params.Icon = pgtype.Text{String: *req.Icon, Valid: true}
		} else {
			params.Icon = pgtype.Text{Valid: false}
		}
	}
	if _, ok := rawFields["lead_type"]; ok {
		if req.LeadType != nil {
			params.LeadType = pgtype.Text{String: *req.LeadType, Valid: true}
		} else {
			params.LeadType = pgtype.Text{Valid: false}
		}
	}
	if _, ok := rawFields["lead_id"]; ok {
		if req.LeadID != nil {
			leadUUID, ok := parseUUIDOrBadRequest(w, *req.LeadID, "lead_id")
			if !ok {
				return
			}
			params.LeadID = leadUUID
		} else {
			params.LeadID = pgtype.UUID{Valid: false}
		}
	}
	if _, ok := rawFields["organisation_id"]; ok {
		organisationID, ok := h.resolveProjectOrganisation(w, r, wsUUID, req.OrganisationID)
		if !ok {
			return
		}
		params.OrganisationID = organisationID
	}
	_, organisationSent := rawFields["organisation_id"]
	_, slugSent := rawFields["slug"]
	if slugSent {
		slug, ok := resolveProjectSlug(w, req.Slug)
		if !ok {
			return
		}
		params.Slug = slug
	}
	// A project with repositories keeps the organisation and slug its folder
	// needs (ADR-0037). Only an edit of either is checked, so other edits of an
	// older project without them still work.
	if (organisationSent || slugSent) && (!params.OrganisationID.Valid || !params.Slug.Valid) {
		repositories, err := h.Queries.ListRepositoriesByProject(r.Context(), db.ListRepositoriesByProjectParams{
			ProjectID: prevProject.ID, WorkspaceID: wsUUID,
		})
		if err != nil {
			slog.Error("list project repositories failed", append(logger.RequestAttrs(r), "error", err)...)
			writeError(w, http.StatusInternalServerError, "failed to update project")
			return
		}
		if len(repositories) > 0 && writeProjectFolderRequired(w, params.OrganisationID, params.Slug) {
			return
		}
	}
	// Dates follow the issue contract: a present key with an empty/null value
	// clears the date; an absent key leaves the prior value untouched.
	if _, ok := rawFields["start_date"]; ok {
		if req.StartDate != nil && *req.StartDate != "" {
			d, err := util.ParseCalendarDate(*req.StartDate)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid start_date format, expected YYYY-MM-DD")
				return
			}
			params.StartDate = d
		} else {
			params.StartDate = pgtype.Date{Valid: false} // explicit null = clear date
		}
	}
	if _, ok := rawFields["due_date"]; ok {
		if req.DueDate != nil && *req.DueDate != "" {
			d, err := util.ParseCalendarDate(*req.DueDate)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid due_date format, expected YYYY-MM-DD")
				return
			}
			params.DueDate = d
		} else {
			params.DueDate = pgtype.Date{Valid: false} // explicit null = clear date
		}
	}
	project, err := h.Queries.UpdateProject(r.Context(), params)
	if err != nil {
		h.writeProjectWriteError(w, r, err, "update")
		return
	}
	resp := projectToResponse(project)
	resp.IssueCount, resp.DoneCount = h.loadProjectIssueStats(r.Context(), wsUUID, project.ID)
	h.publish(protocol.EventProjectUpdated, workspaceID, "member", userID, map[string]any{"project": resp})
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	workspaceID := h.resolveWorkspaceID(r)
	idUUID, ok := parseUUIDOrBadRequest(w, id, "project id")
	if !ok {
		return
	}
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	project, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{
		ID: idUUID, WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	requester, ok := h.requireWorkspaceRole(w, r, uuidToString(project.WorkspaceID), "project not found", "owner", "admin")
	if !ok {
		return
	}
	userID := uuidToString(requester.UserID)
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)

	if _, err := qtx.LockProjectForDelete(r.Context(), db.LockProjectForDeleteParams{
		ID:          project.ID,
		WorkspaceID: project.WorkspaceID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to lock project")
		return
	}
	if err := service.DeleteCheckData(r.Context(), tx, project.WorkspaceID, project.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete project checks")
		return
	}
	repositories, err := qtx.ListRepositoriesByProject(r.Context(), db.ListRepositoriesByProjectParams{WorkspaceID: project.WorkspaceID, ProjectID: project.ID})
	if err != nil {
		writeError(w, 500, "failed to list project repositories")
		return
	}
	sort.Slice(repositories, func(i, j int) bool { return uuidToString(repositories[i].ID) < uuidToString(repositories[j].ID) })
	for _, repo := range repositories {
		if _, err := qtx.LockRepositoryForLine(r.Context(), db.LockRepositoryForLineParams{ID: repo.ID, WorkspaceID: project.WorkspaceID}); err != nil {
			writeError(w, 500, "failed to lock project repository")
			return
		}
	}
	if _, err := qtx.ClearPlacementOfProjectIssues(r.Context(), db.ClearPlacementOfProjectIssuesParams{WorkspaceID: project.WorkspaceID, ProjectID: project.ID}); err != nil {
		writeError(w, 500, "failed to clear project issue placement")
		return
	}
	for _, repo := range repositories {
		if err := clearRepositoryIssueReleases(r.Context(), qtx, project.WorkspaceID, repo.ID); err != nil {
			writeError(w, 500, "failed to clear project releases from issues")
			return
		}
	}
	// The activity of the project's repositories and worklanes is reached
	// through repository, so it goes before the repositories do.
	if err := qtx.DeleteActivitiesByProject(r.Context(), db.DeleteActivitiesByProjectParams{
		WorkspaceID: project.WorkspaceID,
		ProjectID:   project.ID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete project activity")
		return
	}
	if err := qtx.ClearChatSessionProjectByProject(r.Context(), db.ClearChatSessionProjectByProjectParams{
		ProjectID:   project.ID,
		WorkspaceID: project.WorkspaceID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear project chat context")
		return
	}
	// Project-scoped saved views live on the project page; once the project
	// is gone they are unreachable, so they go in the same transaction.
	if err := qtx.DeleteIssueViewsByProjectScope(r.Context(), db.DeleteIssueViewsByProjectScopeParams{
		WorkspaceID: project.WorkspaceID,
		ScopeID:     project.ID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete project views")
		return
	}
	questionIDs, err := h.QuestionService.DropLinksTo(r.Context(), qtx, project.WorkspaceID, question.LinkTypeProject, []pgtype.UUID{project.ID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove project question links")
		return
	}
	// Releases reference their repository, never the project, so they go
	// before the project's repositories, in the same transaction.
	releaseIDs, err := qtx.ListReleaseIDsByProject(r.Context(), db.ListReleaseIDsByProjectParams{WorkspaceID: project.WorkspaceID, ProjectID: project.ID})
	if err != nil {
		writeError(w, 500, "failed to read project releases")
		return
	}
	releaseAttachmentURLs, err := cleanupReleaseDiscussion(r.Context(), qtx, project.WorkspaceID, releaseIDs)
	if err != nil {
		writeError(w, 500, "failed to delete release discussions")
		return
	}
	if err := qtx.DeleteReleaseMilestonesByProject(r.Context(), db.DeleteReleaseMilestonesByProjectParams{
		WorkspaceID: project.WorkspaceID, ProjectID: project.ID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete release milestones")
		return
	}
	if err := qtx.DeleteReleasesByProject(r.Context(), db.DeleteReleasesByProjectParams{
		WorkspaceID: project.WorkspaceID,
		ProjectID:   project.ID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete project releases")
		return
	}
	// A project's repositories and their worklanes go with it, in the same
	// transaction (ADR-0006). Their issue worktrees stay recorded, so they
	// are still removed when their issue closes.
	if err := qtx.DeleteWorklanesByProject(r.Context(), db.DeleteWorklanesByProjectParams{
		WorkspaceID: project.WorkspaceID,
		ProjectID:   project.ID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete project worklanes")
		return
	}
	if err := qtx.DeleteRepositoriesByProject(r.Context(), db.DeleteRepositoriesByProjectParams{
		ProjectID:   project.ID,
		WorkspaceID: project.WorkspaceID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete project repositories")
		return
	}
	if err := qtx.DeleteProject(r.Context(), db.DeleteProjectParams{
		ID:          project.ID,
		WorkspaceID: project.WorkspaceID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete project")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit project delete")
		return
	}
	h.deleteS3Objects(r.Context(), releaseAttachmentURLs)
	h.QuestionService.PublishLinkChanges(r.Context(), project.WorkspaceID, questionIDs)
	h.publish(protocol.EventProjectDeleted, workspaceID, "member", userID, map[string]any{"project_id": uuidToString(project.ID)})
	w.WriteHeader(http.StatusNoContent)
}

// SearchProjectResponse extends ProjectResponse with search metadata.
type SearchProjectResponse struct {
	ProjectResponse
	MatchSource    string  `json:"match_source"`
	MatchedSnippet *string `json:"matched_snippet,omitempty"`
}

// buildProjectSearchQuery builds a dynamic SQL query for project search.
func buildProjectSearchQuery(phrase string, terms []string) (string, []any) {
	phrase = strings.ToLower(phrase)
	for i, t := range terms {
		terms[i] = strings.ToLower(t)
	}

	argIdx := 1
	args := []any{}
	nextArg := func(val any) string {
		args = append(args, val)
		s := fmt.Sprintf("$%d", argIdx)
		argIdx++
		return s
	}

	escapedPhrase := escapeLike(phrase)
	phraseParam := nextArg(escapedPhrase)
	phraseContains := "'%' || " + phraseParam + " || '%'"
	phraseStartsWith := phraseParam + " || '%'"

	wsParam := nextArg(nil) // workspace_id placeholder

	var termParams []string
	if len(terms) > 1 {
		for _, t := range terms {
			et := escapeLike(t)
			termParams = append(termParams, nextArg(et))
		}
	}

	// --- WHERE clause ---
	var whereParts []string

	// Full phrase match: title or description
	phraseMatch := fmt.Sprintf(
		"(LOWER(p.title) LIKE %s OR LOWER(COALESCE(p.description, '')) LIKE %s)",
		phraseContains, phraseContains,
	)
	whereParts = append(whereParts, phraseMatch)

	// Multi-word AND match
	if len(termParams) > 1 {
		var termConditions []string
		for _, tp := range termParams {
			tc := "'%' || " + tp + " || '%'"
			termConditions = append(termConditions, fmt.Sprintf(
				"(LOWER(p.title) LIKE %s OR LOWER(COALESCE(p.description, '')) LIKE %s)",
				tc, tc,
			))
		}
		whereParts = append(whereParts, "("+strings.Join(termConditions, " AND ")+")")
	}

	whereClause := "(" + strings.Join(whereParts, " OR ") + ")"

	// --- ORDER BY ranking ---
	var rankCases []string

	// Tier 0: Exact title match
	rankCases = append(rankCases, fmt.Sprintf("WHEN LOWER(p.title) = %s THEN 0", phraseParam))

	// Tier 1: Title starts with phrase
	rankCases = append(rankCases, fmt.Sprintf("WHEN LOWER(p.title) LIKE %s THEN 1", phraseStartsWith))

	// Tier 2: Title contains phrase
	rankCases = append(rankCases, fmt.Sprintf("WHEN LOWER(p.title) LIKE %s THEN 2", phraseContains))

	// Tier 3: Title matches all words (multi-word only)
	if len(termParams) > 1 {
		var titleTerms []string
		for _, tp := range termParams {
			titleTerms = append(titleTerms, fmt.Sprintf("LOWER(p.title) LIKE '%s' || %s || '%s'", "%", tp, "%"))
		}
		rankCases = append(rankCases, fmt.Sprintf("WHEN (%s) THEN 3", strings.Join(titleTerms, " AND ")))
	}

	// Tier 4: Description contains phrase
	rankCases = append(rankCases, fmt.Sprintf("WHEN LOWER(COALESCE(p.description, '')) LIKE %s THEN 4", phraseContains))

	rankExpr := "CASE " + strings.Join(rankCases, " ") + " ELSE 5 END"

	// --- match_source expression ---
	matchSourceExpr := fmt.Sprintf(`CASE
		WHEN LOWER(p.title) LIKE %s THEN 'title'
		ELSE 'description'
	END`, phraseContains)

	if len(termParams) > 1 {
		var titleTerms []string
		for _, tp := range termParams {
			titleTerms = append(titleTerms, fmt.Sprintf("LOWER(p.title) LIKE '%s' || %s || '%s'", "%", tp, "%"))
		}
		matchSourceExpr = fmt.Sprintf(`CASE
			WHEN LOWER(p.title) LIKE %s THEN 'title'
			WHEN (%s) THEN 'title'
			ELSE 'description'
		END`,
			phraseContains, strings.Join(titleTerms, " AND "),
		)
	}

	limitParam := nextArg(nil)
	offsetParam := nextArg(nil)

	query := fmt.Sprintf(`SELECT p.id, p.workspace_id, p.title, p.description, p.icon,
		p.status, p.priority, p.lead_type, p.lead_id,
		p.start_date, p.due_date,
		p.created_at, p.updated_at, p.organisation_id, p.slug,
		%s AS match_source
	FROM project p
	WHERE p.workspace_id = %s AND %s
	ORDER BY %s, p.updated_at DESC
	LIMIT %s OFFSET %s`,
		matchSourceExpr,
		wsParam,
		whereClause,
		rankExpr,
		limitParam,
		offsetParam,
	)

	return query, args
}

func (h *Handler) SearchProjects(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID := h.resolveWorkspaceID(r)

	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "q parameter is required")
		return
	}

	limit := 20
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > 50 {
		limit = 50
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	terms := splitSearchTerms(q)

	sqlQuery, args := buildProjectSearchQuery(q, terms)
	args[1] = wsUUID
	args[len(args)-2] = limit
	args[len(args)-1] = offset

	type projectSearchRow struct {
		project     db.Project
		matchSource string
	}

	var results []projectSearchRow
	err := runSearchQuery(ctx, h.TxStarter, sqlQuery, args, func(rows pgx.Rows) error {
		for rows.Next() {
			var row projectSearchRow
			if err := rows.Scan(
				&row.project.ID,
				&row.project.WorkspaceID,
				&row.project.Title,
				&row.project.Description,
				&row.project.Icon,
				&row.project.Status,
				&row.project.Priority,
				&row.project.LeadType,
				&row.project.LeadID,
				&row.project.StartDate,
				&row.project.DueDate,
				&row.project.CreatedAt,
				&row.project.UpdatedAt,
				&row.project.OrganisationID,
				&row.project.Slug,
				&row.matchSource,
			); err != nil {
				return fmt.Errorf("scan: %w", err)
			}
			results = append(results, row)
		}
		return rows.Err()
	})
	if err != nil {
		// Statement-timeout surfaces as SQLSTATE 57014 — same
		// fail-fast contract as SearchIssues (see runSearchQuery).
		if isSearchStatementTimeout(err) {
			slog.Warn("search projects timed out",
				"workspace_id", workspaceID,
				"query", q,
				"timeout", searchStatementTimeout)
			writeError(w, http.StatusServiceUnavailable, "search timed out; please refine your query or try again")
			return
		}
		slog.Warn("search projects failed", "error", err, "workspace_id", workspaceID, "query", q)
		writeError(w, http.StatusInternalServerError, "failed to search projects")
		return
	}

	// Batch-fetch issue stats
	statsMap := make(map[string]db.GetProjectIssueStatsRow)
	if len(results) > 0 {
		projectIDs := make([]pgtype.UUID, len(results))
		for i, r := range results {
			projectIDs[i] = r.project.ID
		}
		terminalStatusKeys := h.projectTerminalIssueStatusKeys(ctx, wsUUID)
		stats, statsErr := h.Queries.GetProjectIssueStats(ctx, db.GetProjectIssueStatsParams{
			WorkspaceID:        wsUUID,
			ProjectIds:         projectIDs,
			TerminalStatusKeys: terminalStatusKeys,
		})
		if statsErr == nil {
			for _, s := range stats {
				statsMap[uuidToString(s.ProjectID)] = s
			}
		}
	}

	resp := make([]SearchProjectResponse, len(results))
	for i, row := range results {
		pr := projectToResponse(row.project)
		if s, ok := statsMap[pr.ID]; ok {
			pr.IssueCount = s.TotalCount
			pr.DoneCount = s.DoneCount
		}
		spr := SearchProjectResponse{
			ProjectResponse: pr,
			MatchSource:     row.matchSource,
		}
		if row.matchSource == "description" {
			desc := ""
			if row.project.Description.Valid {
				desc = row.project.Description.String
			}
			if desc != "" {
				snippet := extractSnippet(desc, q)
				spr.MatchedSnippet = &snippet
			}
		}
		resp[i] = spr
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"projects": resp,
	})
}
