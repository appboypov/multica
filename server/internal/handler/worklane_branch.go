package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/middleware"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/multica-ai/multica/server/pkg/taskfailure"
)

// Branch reports and kept worktrees are worklane activity (ADR-0014).
const (
	worklaneActionSwitched = "worklane_switched"
	worklaneActionKept     = "worklane_kept"
)

// worklaneMachine is where a worklane's worktree lives: for main the
// repository's checkout, else the worklane's own path and daemon.
func worklaneLocation(wl db.Worklane, repo db.Repository) (daemonID, path string) {
	if wl.IsMain {
		return repo.CheckoutDaemonID, repo.CheckoutPath
	}
	return wl.DaemonID.String, wl.Path.String
}

// ReportWorklaneBranchRequest is the branch the daemon switched a worklane's
// worktree to before it started a task there.
type ReportWorklaneBranchRequest struct {
	Branch string `json:"branch"`
}

// ReportWorklaneBranch records the branch of the worklane of the task's
// issue: POST /api/daemon/tasks/{taskId}/worklane-branch. The task's runtime
// must sit on the worklane's machine (UJ-6 design.md decision 6).
func (h *Handler) ReportWorklaneBranch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	taskID := chi.URLParam(r, "taskId")
	task, workspaceID, ok := h.requireDaemonTaskAccessWithWorkspace(w, r, taskID)
	if !ok {
		return
	}
	var req ReportWorklaneBranchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	branch := strings.TrimSpace(req.Branch)
	if branch == "" {
		writeError(w, http.StatusBadRequest, "branch is required")
		return
	}
	if !task.IssueID.Valid {
		writeError(w, http.StatusConflict, "task has no issue")
		return
	}
	wsUUID := parseUUID(workspaceID)
	issue, err := h.Queries.GetIssueInWorkspace(ctx, db.GetIssueInWorkspaceParams{ID: task.IssueID, WorkspaceID: wsUUID})
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "issue not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load issue")
		return
	}
	if !issue.WorklaneID.Valid {
		writeError(w, http.StatusConflict, "issue is not in a worklane")
		return
	}
	wl, err := h.Queries.GetWorklaneInWorkspace(ctx, db.GetWorklaneInWorkspaceParams{ID: issue.WorklaneID, WorkspaceID: wsUUID})
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "worklane not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load worklane")
		return
	}
	repo, err := h.Queries.GetRepositoryInWorkspace(ctx, db.GetRepositoryInWorkspaceParams{ID: wl.RepositoryID, WorkspaceID: wsUUID})
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, "repository not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load repository")
		return
	}
	daemonID, _ := worklaneLocation(wl, repo)
	runtime, err := h.Queries.GetAgentRuntime(ctx, task.RuntimeID)
	if err != nil && !isNotFound(err) {
		writeError(w, http.StatusInternalServerError, "failed to load runtime")
		return
	}
	caller := middleware.DaemonIDFromContext(ctx)
	if err != nil || runtime.DaemonID.String != daemonID || (caller != "" && caller != daemonID) {
		writeError(w, http.StatusForbidden, "the task does not run on the worklane's machine")
		return
	}
	updated, err := h.recordWorklaneBranch(ctx, wl, pgtype.Text{String: branch, Valid: true}, issue.ID, "agent", task.AgentID)
	if err != nil {
		slog.Warn("worklane branch report failed", "task_id", taskID, "worklane_id", uuidToString(wl.ID), "error", err)
		writeError(w, http.StatusInternalServerError, "failed to record worklane branch")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"worklane_id": uuidToString(updated.ID),
		"branch":      textToPtr(updated.Branch),
	})
}

// recordWorklaneBranch sets the worklane's branch (NULL is detached), writes
// worklane_switched {from, to, issue_id} when it changed, and publishes
// worklane:updated.
func (h *Handler) recordWorklaneBranch(ctx context.Context, wl db.Worklane, branch pgtype.Text, issueID pgtype.UUID, actorType string, actorID pgtype.UUID) (db.Worklane, error) {
	changed := wl.Branch.Valid != branch.Valid || wl.Branch.String != branch.String
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return db.Worklane{}, err
	}
	defer tx.Rollback(ctx)
	qtx := h.Queries.WithTx(tx)
	updated, err := qtx.SetWorklaneBranch(ctx, db.SetWorklaneBranchParams{Branch: branch, ID: wl.ID, WorkspaceID: wl.WorkspaceID})
	if err != nil {
		return db.Worklane{}, err
	}
	if changed {
		details := map[string]any{"from": textToPtr(wl.Branch), "to": textToPtr(branch), "issue_id": nil}
		if issueID.Valid {
			details["issue_id"] = uuidToString(issueID)
		}
		if err := createWorklaneActivity(ctx, qtx, updated, worklaneActionSwitched, actorType, actorID, details); err != nil {
			return db.Worklane{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return db.Worklane{}, err
	}
	h.publishWorklaneUpdated(ctx, updated, actorType, uuidToString(actorID))
	return updated, nil
}

// recordWorklaneKept writes worklane_kept {branch, reason, count} when the
// worktree stayed on its branch, and publishes worklane:updated.
func (h *Handler) recordWorklaneKept(ctx context.Context, wl db.Worklane, reason string, count int, actorType string, actorID pgtype.UUID) error {
	details := map[string]any{"branch": textToPtr(wl.Branch), "reason": reason, "count": count}
	if err := createWorklaneActivity(ctx, h.Queries, wl, worklaneActionKept, actorType, actorID, details); err != nil {
		return err
	}
	h.publishWorklaneUpdated(ctx, wl, actorType, uuidToString(actorID))
	return nil
}

func createWorklaneActivity(ctx context.Context, q *db.Queries, wl db.Worklane, action, actorType string, actorID pgtype.UUID, details map[string]any) error {
	raw, err := json.Marshal(details)
	if err != nil {
		return err
	}
	_, err = q.CreateWorklaneActivity(ctx, db.CreateWorklaneActivityParams{
		WorkspaceID:  wl.WorkspaceID,
		WorklaneID:   wl.ID,
		RepositoryID: wl.RepositoryID,
		ActorType:    pgtype.Text{String: actorType, Valid: true},
		ActorID:      actorID,
		Action:       action,
		Details:      raw,
	})
	return err
}

// claimWorklane fills the worklane block of the claim for the repository of
// the issue's worklane, adding that repository when the project block lacks
// it. A runtime on another machine fails the task before launch with
// worklane_elsewhere (UJ-6 design.md decision 6).
func (h *Handler) claimWorklane(ctx context.Context, task *db.AgentTaskQueue, runtime db.AgentRuntime, agentName string, issue db.Issue, repos []protocol.ClaimRepository) ([]protocol.ClaimRepository, *claimBuildFailure) {
	if !issue.WorklaneID.Valid {
		return repos, nil
	}
	loadFailed := func(what string, err error) ([]protocol.ClaimRepository, *claimBuildFailure) {
		slog.Error("issue claim: load worklane context failed; preserving task for redelivery",
			"task_id", uuidToString(task.ID), "issue_id", uuidToString(issue.ID), "what", what, "error", err)
		if _, requeueErr := h.TaskService.RequeueTaskAfterClaimFailure(ctx, *task); requeueErr != nil {
			slog.Error("issue claim: requeue after worklane load failure failed", "task_id", uuidToString(task.ID), "error", requeueErr)
		}
		return repos, &claimBuildFailure{outcome: "error_worklane_context", status: http.StatusInternalServerError, message: "failed to load worklane context"}
	}
	wl, err := h.Queries.GetWorklaneInWorkspace(ctx, db.GetWorklaneInWorkspaceParams{ID: issue.WorklaneID, WorkspaceID: issue.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return repos, nil
	}
	if err != nil {
		return loadFailed("worklane", err)
	}
	repo, err := h.Queries.GetRepositoryInWorkspace(ctx, db.GetRepositoryInWorkspaceParams{ID: wl.RepositoryID, WorkspaceID: issue.WorkspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		return repos, nil
	}
	if err != nil {
		return loadFailed("repository", err)
	}
	daemonID, path := worklaneLocation(wl, repo)
	if daemonID == "" || path == "" {
		return repos, nil
	}
	machines, err := h.Queries.ListAgentRuntimes(ctx, issue.WorkspaceID)
	if err != nil {
		return loadFailed("runtimes", err)
	}
	worklaneMachineName := daemonMachineName(machines, daemonID)
	if runtime.DaemonID.String != daemonID {
		runtimeMachineName := daemonMachineName(machines, runtime.DaemonID.String)
		if runtimeMachineName == "" {
			runtimeMachineName = runtime.Name
		}
		message := fmt.Sprintf("%s runs on %s; %s lives on %s", agentName, runtimeMachineName, wl.Name, worklaneMachineName)
		return repos, h.failClaimedTaskBeforeLaunch(ctx, task, message, taskfailure.ReasonWorklaneElsewhere,
			"error_worklane_elsewhere", http.StatusConflict, "worklane lives on another machine")
	}
	pullRequests, err := h.Queries.ListPullRequestsByIssue(ctx, issue.ID)
	if err != nil {
		return loadFailed("pull requests", err)
	}
	pullRequestBranch := ""
	for _, pr := range pullRequests {
		if pr.WorkspaceID == issue.WorkspaceID && pr.State == "open" && pr.Branch.Valid && pr.Branch.String != "" &&
			strings.EqualFold(pr.RepoOwner+"/"+pr.RepoName, repo.FullName) {
			pullRequestBranch = pr.Branch.String
			break
		}
	}
	block := &protocol.ClaimWorklane{
		ID:                uuidToString(wl.ID),
		Name:              wl.Name,
		Path:              path,
		DaemonID:          daemonID,
		DaemonName:        worklaneMachineName,
		Branch:            textToPtr(wl.Branch),
		DefaultBranch:     repo.DefaultBranch,
		PullRequestBranch: pullRequestBranch,
	}
	repoID := uuidToString(repo.ID)
	for i := range repos {
		if repos[i].ID == repoID {
			repos[i].Worklane = block
			return repos, nil
		}
	}
	return append(repos, protocol.ClaimRepository{
		ID:               repoID,
		FullName:         repo.FullName,
		DefaultBranch:    repo.DefaultBranch,
		CheckoutDaemonID: repo.CheckoutDaemonID,
		CheckoutPath:     repo.CheckoutPath,
		Worklane:         block,
	}), nil
}

// daemonMachineName is the machine name of a daemon as the Runtimes page
// shows it, read from its newest runtime row: the machine's custom name, else
// the host in the runtime's name, else the device name.
func daemonMachineName(runtimes []db.AgentRuntime, daemonID string) string {
	if daemonID == "" {
		return ""
	}
	var newest *db.AgentRuntime
	for i := range runtimes {
		rt := &runtimes[i]
		if rt.DaemonID.String != daemonID {
			continue
		}
		if newest == nil || rt.UpdatedAt.Time.After(newest.UpdatedAt.Time) {
			newest = rt
		}
	}
	if newest == nil {
		return daemonID
	}
	if custom := strings.TrimSpace(newest.CustomName.String); custom != "" {
		return custom
	}
	if m := runtimeHostnamePattern.FindStringSubmatch(newest.Name); m != nil {
		return m[1]
	}
	if device := strings.TrimSpace(strings.Split(newest.DeviceInfo, " · ")[0]); device != "" {
		return device
	}
	return newest.Name
}
