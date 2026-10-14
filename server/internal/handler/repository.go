package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// validVersionPolicies are the six release policies of a repository
// (ADR-0006): the version file the release line reads, tags only, or none.
var validVersionPolicies = []string{"pubspec.yaml", "Cargo.toml", "pyproject.toml", "package.json", "tags", "none"}

// RepositoryResponse is the JSON shape of a repository of a project.
type RepositoryResponse struct {
	ID               string  `json:"id"`
	WorkspaceID      string  `json:"workspace_id"`
	ProjectID        string  `json:"project_id"`
	GitHubRepoID     int64   `json:"github_repo_id"`
	FullName         string  `json:"full_name"`
	Private          bool    `json:"private"`
	DefaultBranch    string  `json:"default_branch"`
	CheckoutDaemonID string  `json:"checkout_daemon_id"`
	CheckoutPath     string  `json:"checkout_path"`
	VersionPolicy    string  `json:"version_policy"`
	InstallCommand   *string `json:"install_command"`
	LastTag          *string `json:"last_tag"`
	CreatedBy        *string `json:"created_by"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

func repositoryToResponse(r db.Repository) RepositoryResponse {
	resp := RepositoryResponse{
		ID:               uuidToString(r.ID),
		WorkspaceID:      uuidToString(r.WorkspaceID),
		ProjectID:        uuidToString(r.ProjectID),
		GitHubRepoID:     r.GithubRepoID,
		FullName:         r.FullName,
		Private:          r.Private,
		DefaultBranch:    r.DefaultBranch,
		CheckoutDaemonID: r.CheckoutDaemonID,
		CheckoutPath:     r.CheckoutPath,
		VersionPolicy:    r.VersionPolicy,
		InstallCommand:   textToPtr(r.InstallCommand),
		LastTag:          textToPtr(r.LastTag),
		CreatedAt:        timestampToString(r.CreatedAt),
		UpdatedAt:        timestampToString(r.UpdatedAt),
	}
	if r.CreatedBy.Valid {
		createdBy := uuidToString(r.CreatedBy)
		resp.CreatedBy = &createdBy
	}
	return resp
}

// CreateRepositoryRequest's InstallCommand is '' for None; absent, the
// repository takes the check's first found command, or ''.
type CreateRepositoryRequest struct {
	ProjectID       string  `json:"project_id"`
	FullName        string  `json:"full_name"`
	CheckoutCheckID string  `json:"checkout_check_id"`
	VersionPolicy   string  `json:"version_policy"`
	InstallCommand  *string `json:"install_command"`
}

type UpdateRepositoryRequest struct {
	CheckoutCheckID *string `json:"checkout_check_id"`
	VersionPolicy   *string `json:"version_policy"`
	InstallCommand  *string `json:"install_command"`
}

// foundInstallCommand is the first install command a passed check found, or
// '' (None) when it found none.
func foundInstallCommand(result *protocol.RepositoryCheckoutResult) string {
	if result == nil || len(result.InstallCommands) == 0 {
		return ""
	}
	return result.InstallCommands[0].Command
}

// ListRepositories returns the workspace's repositories by full_name.
func (h *Handler) ListRepositories(w http.ResponseWriter, r *http.Request) {
	wsUUID, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace id")
	if !ok {
		return
	}
	rows, err := h.Queries.ListRepositories(r.Context(), wsUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list repositories")
		return
	}
	resp := make([]RepositoryResponse, len(rows))
	for i, row := range rows {
		resp[i] = repositoryToResponse(row)
	}
	writeJSON(w, http.StatusOK, map[string]any{"repositories": resp, "total": len(resp)})
}

// ListAvailableRepositories merges the repositories every GitHub installation
// of the workspace sees, by full_name, and names the App's installation page
// so a missing repository can be granted.
func (h *Handler) ListAvailableRepositories(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	installations, err := h.Queries.ListGitHubInstallationsByWorkspace(r.Context(), wsUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list GitHub installations")
		return
	}
	merged := make(map[string]installationRepository)
	for _, installation := range installations {
		repositories, err := listInstallationRepositories(r.Context(), installation.InstallationID)
		if err != nil {
			slog.Warn("list installation repositories failed", "error", err, "workspace_id", workspaceID, "installation_id", installation.InstallationID)
			writeError(w, http.StatusBadGateway, "GitHub could not be read")
			return
		}
		for _, repository := range repositories {
			key := strings.ToLower(repository.FullName)
			if _, seen := merged[key]; !seen {
				merged[key] = repository
			}
		}
	}
	out := make([]installationRepository, 0, len(merged))
	for _, repository := range merged {
		out = append(out, repository)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].FullName) < strings.ToLower(out[j].FullName)
	})

	installURL := ""
	if isGitHubConfigured() {
		installURL, err = githubInstallURL(workspaceID, githubReturnToRepositories)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to sign state")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"repositories": out, "install_url": installURL})
}

// readRepositoryFromGitHub reads the repository again through the first
// installation of the workspace that sees it. found is false when none does.
func (h *Handler) readRepositoryFromGitHub(ctx context.Context, wsUUID pgtype.UUID, fullName string) (installationRepository, bool, error) {
	installations, err := h.Queries.ListGitHubInstallationsByWorkspace(ctx, wsUUID)
	if err != nil {
		return installationRepository{}, false, err
	}
	for _, installation := range installations {
		repository, found, err := getInstallationRepository(ctx, installation.InstallationID, fullName)
		if err != nil {
			return installationRepository{}, false, err
		}
		if found {
			return repository, true, nil
		}
	}
	return installationRepository{}, false, nil
}

// passedCheckout returns the check the body names when it is a completed,
// passed check of the repository on its current default branch. It writes
// the 400 itself otherwise.
func (h *Handler) passedCheckout(w http.ResponseWriter, r *http.Request, workspaceID, checkID, fullName, defaultBranch string) (*RuntimeRepositoryCheckoutRequest, bool) {
	if strings.TrimSpace(checkID) == "" {
		writeError(w, http.StatusBadRequest, "the checkout has not passed its check")
		return nil, false
	}
	check, err := h.RepositoryCheckoutStore.Get(r.Context(), checkID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load checkout check")
		return nil, false
	}
	if !check.passedFor(workspaceID, fullName, defaultBranch) || check.DaemonID == "" {
		writeError(w, http.StatusBadRequest, "the checkout has not passed its check")
		return nil, false
	}
	return check, true
}

// writeRepositoryTaken answers 409 naming the project that holds the
// repository already.
func (h *Handler) writeRepositoryTaken(w http.ResponseWriter, r *http.Request, wsUUID pgtype.UUID, fullName string) {
	rows, err := h.Queries.ListRepositories(r.Context(), wsUUID)
	if err == nil {
		for _, row := range rows {
			if !strings.EqualFold(row.FullName, fullName) {
				continue
			}
			project, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{ID: row.ProjectID, WorkspaceID: wsUUID})
			if err == nil {
				writeError(w, http.StatusConflict, "repository is in project "+project.Title)
				return
			}
		}
	}
	writeError(w, http.StatusConflict, "repository is in another project")
}

func (h *Handler) CreateRepository(w http.ResponseWriter, r *http.Request) {
	var req CreateRepositoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace id")
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	projectUUID, ok := parseUUIDOrBadRequest(w, req.ProjectID, "project_id")
	if !ok {
		return
	}
	req.FullName = strings.TrimSpace(req.FullName)
	if owner, name, found := strings.Cut(req.FullName, "/"); !found || owner == "" || name == "" || strings.Contains(name, "/") {
		writeError(w, http.StatusBadRequest, "full_name must be owner/name")
		return
	}
	if !validateProjectEnum(w, "version_policy", req.VersionPolicy, validVersionPolicies) {
		return
	}
	project, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{ID: projectUUID, WorkspaceID: wsUUID})
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	github, found, err := h.readRepositoryFromGitHub(r.Context(), wsUUID, req.FullName)
	if err != nil {
		slog.Warn("read repository from GitHub failed", "error", err, "workspace_id", workspaceID, "full_name", req.FullName)
		writeError(w, http.StatusBadGateway, "GitHub could not be read")
		return
	}
	if !found {
		writeError(w, http.StatusBadRequest, "the GitHub App cannot see this repository")
		return
	}
	check, ok := h.passedCheckout(w, r, workspaceID, req.CheckoutCheckID, github.FullName, github.DefaultBranch)
	if !ok {
		return
	}
	installCommand := foundInstallCommand(check.Result)
	if req.InstallCommand != nil {
		installCommand = strings.TrimSpace(*req.InstallCommand)
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)

	repository, err := qtx.CreateRepository(r.Context(), db.CreateRepositoryParams{
		WorkspaceID:      wsUUID,
		ProjectID:        project.ID,
		GithubRepoID:     github.GitHubRepoID,
		FullName:         github.FullName,
		Private:          github.Private,
		DefaultBranch:    github.DefaultBranch,
		CheckoutDaemonID: check.DaemonID,
		CheckoutPath:     check.Result.ResolvedPath,
		VersionPolicy:    req.VersionPolicy,
		LastTag:          checkLastTag(*check.Result),
		InstallCommand:   pgtype.Text{String: installCommand, Valid: true},
		CreatedBy:        parseUUID(userID),
	})
	if err != nil {
		if isUniqueViolation(err) {
			h.writeRepositoryTaken(w, r, wsUUID, github.FullName)
			return
		}
		slog.Error("create repository failed", "error", err, "workspace_id", workspaceID, "full_name", github.FullName)
		writeError(w, http.StatusInternalServerError, "failed to create repository")
		return
	}
	worklane, err := qtx.CreateWorklane(r.Context(), db.CreateWorklaneParams{
		WorkspaceID:  wsUUID,
		RepositoryID: repository.ID,
		Name:         "main",
		IsMain:       true,
		Position:     0,
	})
	if err != nil {
		slog.Error("create main worklane failed", "error", err, "repository_id", uuidToString(repository.ID))
		writeError(w, http.StatusInternalServerError, "failed to create worklane main")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create repository")
		return
	}

	resp := repositoryToResponse(repository)
	h.publish(protocol.EventRepositoryCreated, workspaceID, "member", userID, protocol.RepositoryEventPayload{
		RepositoryID: resp.ID,
		ProjectID:    resp.ProjectID,
	})
	h.publish(protocol.EventWorklaneCreated, workspaceID, "member", userID, protocol.WorklaneEventPayload{
		WorklaneID:   uuidToString(worklane.ID),
		RepositoryID: resp.ID,
	})
	writeJSON(w, http.StatusCreated, resp)
}

// loadRepositoryInWorkspace resolves the {id} path parameter to a repository
// of the request's workspace, writing 400 or 404 itself.
func (h *Handler) loadRepositoryInWorkspace(w http.ResponseWriter, r *http.Request) (db.Repository, bool) {
	idUUID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "repository id")
	if !ok {
		return db.Repository{}, false
	}
	wsUUID, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace id")
	if !ok {
		return db.Repository{}, false
	}
	repository, err := h.Queries.GetRepositoryInWorkspace(r.Context(), db.GetRepositoryInWorkspaceParams{ID: idUUID, WorkspaceID: wsUUID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "repository not found")
			return db.Repository{}, false
		}
		writeError(w, http.StatusInternalServerError, "failed to load repository")
		return db.Repository{}, false
	}
	return repository, true
}

// UpdateRepository edits the checkout and the version policy. The repository
// itself is fixed: no field changes full_name.
func (h *Handler) UpdateRepository(w http.ResponseWriter, r *http.Request) {
	var req UpdateRepositoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	repository, ok := h.loadRepositoryInWorkspace(w, r)
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := uuidToString(repository.WorkspaceID)
	if req.VersionPolicy != nil && !validateProjectEnum(w, "version_policy", *req.VersionPolicy, validVersionPolicies) {
		return
	}

	github, found, err := h.readRepositoryFromGitHub(r.Context(), repository.WorkspaceID, repository.FullName)
	if err != nil {
		slog.Warn("read repository from GitHub failed", "error", err, "workspace_id", workspaceID, "full_name", repository.FullName)
		writeError(w, http.StatusBadGateway, "GitHub could not be read")
		return
	}
	if !found {
		writeError(w, http.StatusBadRequest, "the GitHub App cannot see this repository")
		return
	}
	var check *RuntimeRepositoryCheckoutRequest
	if req.CheckoutCheckID != nil {
		check, ok = h.passedCheckout(w, r, workspaceID, *req.CheckoutCheckID, repository.FullName, github.DefaultBranch)
		if !ok {
			return
		}
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)
	if req.VersionPolicy != nil && *req.VersionPolicy == versionPolicyNone && !refuseStopReleasing(r.Context(), w, qtx, repository) {
		return
	}

	updated, err := qtx.UpdateRepositoryGithub(r.Context(), db.UpdateRepositoryGithubParams{
		Private:       github.Private,
		DefaultBranch: github.DefaultBranch,
		ID:            repository.ID,
		WorkspaceID:   repository.WorkspaceID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update repository")
		return
	}
	if check != nil {
		if updated, err = qtx.UpdateRepositoryCheckout(r.Context(), db.UpdateRepositoryCheckoutParams{
			CheckoutDaemonID: check.DaemonID,
			CheckoutPath:     check.Result.ResolvedPath,
			ID:               repository.ID,
			WorkspaceID:      repository.WorkspaceID,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update repository checkout")
			return
		}
	}
	if req.VersionPolicy != nil {
		if updated, err = qtx.UpdateRepositoryVersionPolicy(r.Context(), db.UpdateRepositoryVersionPolicyParams{
			VersionPolicy: *req.VersionPolicy,
			ID:            repository.ID,
			WorkspaceID:   repository.WorkspaceID,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update repository version policy")
			return
		}
	}
	if req.InstallCommand != nil {
		if updated, err = qtx.UpdateRepositoryInstallCommand(r.Context(), db.UpdateRepositoryInstallCommandParams{
			InstallCommand: pgtype.Text{String: strings.TrimSpace(*req.InstallCommand), Valid: true},
			ID:             repository.ID,
			WorkspaceID:    repository.WorkspaceID,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update repository install command")
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update repository")
		return
	}

	resp := repositoryToResponse(updated)
	h.publish(protocol.EventRepositoryUpdated, workspaceID, "member", userID, protocol.RepositoryEventPayload{
		RepositoryID: resp.ID,
		ProjectID:    resp.ProjectID,
	})
	writeJSON(w, http.StatusOK, resp)
}

// DeleteRepository removes the repository, its main worklane and its activity
// in one transaction. It refuses with 409 has_worklanes, naming them, while
// the repository has a worklane besides main. Its issue worktrees stay
// recorded, so they are still removed when their issue closes.
func (h *Handler) DeleteRepository(w http.ResponseWriter, r *http.Request) {
	repository, ok := h.loadRepositoryInWorkspace(w, r)
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	workspaceID := uuidToString(repository.WorkspaceID)

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)
	repository, err = qtx.LockRepositoryForLine(r.Context(),db.LockRepositoryForLineParams{ID:repository.ID,WorkspaceID:repository.WorkspaceID})
	if errors.Is(err,pgx.ErrNoRows) { writeError(w,404,"repository not found"); return }
	if err != nil { writeError(w,500,"failed to lock repository"); return }
	others, err := qtx.CountOtherWorklanes(r.Context(), db.CountOtherWorklanesParams{
		RepositoryID: repository.ID,
		WorkspaceID:  repository.WorkspaceID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to count worklanes")
		return
	}
	if others > 0 {
		all, err := qtx.ListWorklanes(r.Context(), repository.WorkspaceID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list worklanes")
			return
		}
		names := make([]string, 0, others)
		for _, wl := range all {
			if wl.RepositoryID == repository.ID && !wl.IsMain {
				names = append(names, wl.Name)
			}
		}
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":     "remove the repository's worklanes first: " + strings.Join(names, ", "),
			"code":      "has_worklanes",
			"worklanes": names,
		})
		return
	}
	if err := qtx.DeleteActivitiesByRepository(r.Context(), db.DeleteActivitiesByRepositoryParams{
		RepositoryID: repository.ID,
		WorkspaceID:  repository.WorkspaceID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete repository activity")
		return
	}
	if _, err := qtx.ClearPlacementOfRepositoryIssues(r.Context(), db.ClearPlacementOfRepositoryIssuesParams{
		WorkspaceID:  repository.WorkspaceID,
		RepositoryID: repository.ID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear issue placement")
		return
	}
	if err := clearRepositoryIssueReleases(r.Context(),qtx,repository.WorkspaceID,repository.ID); err != nil { writeError(w,500,"failed to clear repository releases from issues"); return }
	if err := qtx.DeleteReleasesByRepository(r.Context(), db.DeleteReleasesByRepositoryParams{
		RepositoryID: repository.ID,
		WorkspaceID:  repository.WorkspaceID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete releases")
		return
	}

	worklanes, err := qtx.DeleteWorklanesByRepository(r.Context(), db.DeleteWorklanesByRepositoryParams{
		RepositoryID: repository.ID,
		WorkspaceID:  repository.WorkspaceID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete worklanes")
		return
	}
	if err := qtx.DeleteRepository(r.Context(), db.DeleteRepositoryParams{
		ID:          repository.ID,
		WorkspaceID: repository.WorkspaceID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete repository")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete repository")
		return
	}

	repositoryID := uuidToString(repository.ID)
	h.publish(protocol.EventRepositoryDeleted, workspaceID, "member", userID, protocol.RepositoryEventPayload{
		RepositoryID: repositoryID,
		ProjectID:    uuidToString(repository.ProjectID),
	})
	for _, worklane := range worklanes {
		h.publish(protocol.EventWorklaneDeleted, workspaceID, "member", userID, protocol.WorklaneEventPayload{
			WorklaneID:   uuidToString(worklane.ID),
			RepositoryID: repositoryID,
		})
	}
	w.WriteHeader(http.StatusNoContent)
}
