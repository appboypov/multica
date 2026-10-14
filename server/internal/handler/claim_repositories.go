package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// claimProjectContext is the project-scoped context a daemon claim exposes to
// the agent: the project identity the prompt names, the project's repositories
// with their checkout and issue worktree (design.md decision 4), the
// organisation key the worktree path uses, and the repo list `multica repo
// checkout` reads.
type claimProjectContext struct {
	ProjectID       string
	Title           string
	Description     string
	OrganisationKey string
	Repositories    []protocol.ClaimRepository
	Repos           []RepoData
}

// applyTo copies the resolved context onto a claim response. Callers assign the
// whole context or none of it, so a claim can never carry a project's title
// without its repositories.
func (c claimProjectContext) applyTo(resp *AgentTaskResponse) {
	resp.ProjectID = c.ProjectID
	resp.ProjectTitle = c.Title
	resp.ProjectDescription = c.Description
	resp.OrganisationKey = c.OrganisationKey
	resp.Repositories = c.Repositories
	resp.Repos = c.Repos
}

// resolveClaimProjectContext loads the project context for one daemon claim.
//
// Every claim path (issue, chat, autopilot, quick-create) resolves the same
// thing from a soft project reference, so the tenant and failure rules live
// here once rather than in a copy per path:
//
//   - Every read is workspace-scoped, so a corrupt project reference cannot
//     lift another tenant's repositories or paths into a claim.
//   - A read FAILURE is not "no project". It returns an error so the caller can
//     preserve the task for redelivery; collapsing it into the workspace-repo
//     fallback is what lets a transient DB error silently run an agent against
//     the wrong repository (MUL-4351).
//   - A project that resolves to no row IS "no project": the reference is stale,
//     deleted, or points outside this workspace, and the claim degrades to
//     workspace context.
//
// Repo precedence: the project's repositories replace the workspace repos when
// it has any. With no project, no repositories, or a stale reference, the
// workspace repos are the fallback.
//
// issueID is set for an issue task: each repository then carries the issue's
// recorded worktree path. daemonID is the claiming runtime's daemon.
func (h *Handler) resolveClaimProjectContext(ctx context.Context, projectID, issueID, workspaceID pgtype.UUID, daemonID string) (claimProjectContext, error) {
	var out claimProjectContext

	if projectID.Valid {
		project, err := h.Queries.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{
			ID:          projectID,
			WorkspaceID: workspaceID,
		})
		switch {
		case err == nil:
			out.ProjectID = uuidToString(project.ID)
			out.Title = project.Title
			out.Description = project.Description.String
			if project.OrganisationID.Valid {
				org, orgErr := h.Queries.GetOrganisationInWorkspace(ctx, db.GetOrganisationInWorkspaceParams{
					ID:          project.OrganisationID,
					WorkspaceID: workspaceID,
				})
				switch {
				case orgErr == nil:
					out.OrganisationKey = org.Key
				case errors.Is(orgErr, pgx.ErrNoRows):
				default:
					return claimProjectContext{}, fmt.Errorf("get organisation: %w", orgErr)
				}
			}
			repositories, repoErr := h.buildClaimRepositories(ctx, workspaceID, project.ID, issueID, daemonID)
			if repoErr != nil {
				return claimProjectContext{}, repoErr
			}
			out.Repositories = repositories
			for _, repository := range repositories {
				out.Repos = append(out.Repos, RepoData{URL: "https://github.com/" + repository.FullName})
			}
		case errors.Is(err, pgx.ErrNoRows):
			// Stale/deleted/foreign reference: degrade to workspace context.
		default:
			return claimProjectContext{}, fmt.Errorf("get project: %w", err)
		}
	}

	if len(out.Repos) > 0 {
		return out, nil
	}

	ws, err := h.Queries.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return claimProjectContext{}, fmt.Errorf("get workspace: %w", err)
	}
	if ws.Repos != nil {
		var repos []RepoData
		if jsonErr := json.Unmarshal(ws.Repos, &repos); jsonErr != nil {
			// Corrupt stored JSON is not transient: failing the claim would
			// wedge every claim in this workspace until someone repairs the
			// row. Degrade to no repos and leave a trail instead.
			slog.Error("claim project context: workspace repos are not valid JSON; claiming without repos",
				"workspace_id", uuidToString(workspaceID), "error", jsonErr)
		} else if len(repos) > 0 {
			out.Repos = repos
		}
	}
	return out, nil
}

// buildClaimRepositories lists the project's repositories for a claim, each
// with its checkout and, for an issue task, the issue's recorded worktree path
// in that repository (ADR-0007). A chat or other task without an issue gets
// the checkouts and no worktree.
func (h *Handler) buildClaimRepositories(ctx context.Context, workspaceID, projectID, issueID pgtype.UUID, daemonID string) ([]protocol.ClaimRepository, error) {
	rows, err := h.Queries.ListRepositoriesByProject(ctx, db.ListRepositoriesByProjectParams{
		ProjectID:   projectID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("list project repositories: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}

	worktreePaths := map[string]string{}
	if issueID.Valid {
		worktrees, wtErr := h.Queries.ListIssueWorktreesForIssue(ctx, db.ListIssueWorktreesForIssueParams{
			IssueID:     issueID,
			WorkspaceID: workspaceID,
		})
		if wtErr != nil {
			return nil, fmt.Errorf("list issue worktrees: %w", wtErr)
		}
		for _, worktree := range worktrees {
			worktreePaths[uuidToString(worktree.RepositoryID)] = worktree.Path
		}
	}

	out := make([]protocol.ClaimRepository, 0, len(rows))
	for _, row := range rows {
		id := uuidToString(row.ID)
		worklanePaths, err := h.claimWorklanePaths(ctx, workspaceID, row.ID, daemonID)
		if err != nil {
			return nil, err
		}
		out = append(out, protocol.ClaimRepository{
			ID:               id,
			FullName:         row.FullName,
			DefaultBranch:    row.DefaultBranch,
			CheckoutDaemonID: row.CheckoutDaemonID,
			CheckoutPath:     row.CheckoutPath,
			WorktreePath:     worktreePaths[id],
			WorklanePaths:    worklanePaths,
		})
	}
	return out, nil
}

// claimWorklanePaths are the worktree paths of the repository's worklanes on
// the claiming daemon, so its issue worktree never takes one (ADR-0009).
func (h *Handler) claimWorklanePaths(ctx context.Context, workspaceID, repositoryID pgtype.UUID, daemonID string) ([]string, error) {
	if daemonID == "" {
		return nil, nil
	}
	worklanes, err := h.Queries.ListWorklanePathsForDaemon(ctx, db.ListWorklanePathsForDaemonParams{
		WorkspaceID:  workspaceID,
		RepositoryID: repositoryID,
		DaemonID:     pgtype.Text{String: daemonID, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("list worklane paths: %w", err)
	}
	var paths []string
	for _, wl := range worklanes {
		if wl.Path.Valid && wl.Path.String != "" {
			paths = append(paths, wl.Path.String)
		}
	}
	return paths, nil
}
