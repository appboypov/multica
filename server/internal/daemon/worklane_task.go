package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/multica-ai/multica/server/internal/worklanehead"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

var errWorklaneBranchHeld = errors.New("worklane branch holds work")

type worklaneBranchHeldError struct{ message string }

func (e *worklaneBranchHeldError) Error() string { return e.message }
func (e *worklaneBranchHeldError) Unwrap() error { return errWorklaneBranchHeld }

// worklaneTaskPath resolves the PR destination before taking any task lock,
// including when the linked PR's worktree has not been created yet.
func worklaneTaskPath(ctx context.Context, task Task, repo protocol.ClaimRepository) (string, error) {
	path, err := execenv.ExpandWorklanePath(repo.Worklane.Path)
	if err != nil {
		return "", err
	}
	wanted := worklaneTaskBranch(task, repo)
	if wanted == "" {
		return "", fmt.Errorf("worklane issue task has no issue identifier")
	}
	checkout, err := execenv.ExpandWorklanePath(worklaneTaskCheckout(repo))
	if err != nil {
		return "", err
	}
	if repo.Worklane.PullRequestBranch != "" {
		branch, err := checkoutGit(ctx, checkout, "check-ref-format", "--branch", wanted)
		if err != nil || branch != wanted {
			return "", fmt.Errorf("invalid pull-request branch %q", wanted)
		}
	}
	selected := ""
	if repo.WorktreePath != "" {
		recorded, err := execenv.ExpandWorklanePath(repo.WorktreePath)
		if err != nil {
			return "", err
		}
		if isPullRequestTaskPath(recorded, wanted) {
			selected = recorded
		}
	}
	if selected == "" {
		held, err := checkoutGit(ctx, checkout, "for-each-ref", "--format=%(refname)\t%(worktreepath)", "refs/heads/"+wanted)
		if err != nil {
			return "", fmt.Errorf("find the PR branch's worktree: %w", err)
		}
		for held != "" {
			var ref string
			ref, held, _ = strings.Cut(held, "\n")
			name, tree, _ := strings.Cut(ref, "\t")
			if name == "refs/heads/"+wanted && isPullRequestTaskPath(tree, wanted) {
				selected = tree
				break
			}
		}
	}
	if selected == "" && repo.Worklane.PullRequestBranch != "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		_, name, ok := strings.Cut(repo.FullName, "/")
		if !ok || name == "" || name == "." || name == ".." || filepath.Base(name) != name {
			return "", fmt.Errorf("invalid repository %q", repo.FullName)
		}
		org := task.OrganisationKey
		if org == "." || org == ".." || filepath.Base(org) != org && org != "" {
			return "", fmt.Errorf("invalid organisation key %q", org)
		}
		root, err := resolveRealPath(filepath.Join(home, "Worktrees", org, name, ".pull-requests"))
		if err != nil {
			return "", err
		}
		selected, err = resolveRealPath(filepath.Join(root, filepath.FromSlash(wanted)))
		if err != nil {
			return "", err
		}
		relative, err := filepath.Rel(root, selected)
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("pull-request branch %q escapes its worktree root", wanted)
		}
	}
	if selected == "" {
		return path, nil
	}
	selected, err = resolveRealPath(selected)
	if err != nil {
		return "", err
	}
	lanePath, err := resolveRealPath(path)
	if err != nil {
		return "", err
	}
	if selected == lanePath {
		return "", fmt.Errorf("pull-request worktree %s is the active worklane", selected)
	}
	return selected, nil
}

func worklaneTaskBranch(task Task, repo protocol.ClaimRepository) string {
	if repo.Worklane.PullRequestBranch != "" {
		return repo.Worklane.PullRequestBranch
	}
	return worklanehead.Branch(task.IssueIdentifier)
}

func worklaneTaskCheckout(repo protocol.ClaimRepository) string {
	if repo.CheckoutPath != "" && repo.CheckoutDaemonID == repo.Worklane.DaemonID {
		return repo.CheckoutPath
	}
	return repo.Worklane.Path
}

func isPullRequestTaskPath(path, branch string) bool {
	suffix := filepath.Join(".pull-requests", filepath.FromSlash(branch))
	return path != "" && strings.HasSuffix(filepath.Clean(path), string(filepath.Separator)+suffix)
}

// prepareWorklaneTask runs with the selected worktree's path lock held.
func (d *Daemon) prepareWorklaneTask(ctx context.Context, task *Task, repo *protocol.ClaimRepository, logger *slog.Logger) error {
	lane := repo.Worklane
	path, err := execenv.ExpandWorklanePath(repo.WorktreePath)
	if err != nil {
		return err
	}
	wanted := worklaneTaskBranch(*task, *repo)
	if wanted == "" {
		return fmt.Errorf("worklane issue task has no issue identifier")
	}
	lanePath, err := execenv.ExpandWorklanePath(lane.Path)
	if err != nil {
		return err
	}
	if repo.Worklane.PullRequestBranch != "" || path != lanePath {
		path, err = execenv.PreparePullRequestWorktree(ctx, worklaneTaskCheckout(*repo), path, wanted, logger)
		if err != nil {
			return err
		}
		if err := d.client.RecordIssueWorktree(ctx, task.ID, repo.ID, path, wanted); err != nil {
			return fmt.Errorf("record the pull-request worktree: %w", err)
		}
		repo.WorktreePath = path
		logger.Debug("using pull-request worktree", "path", path, "branch", wanted, "worklane", lane.Name)
		return nil
	}
	branch, err := execenv.WorklaneBranch(path)
	if err != nil {
		return err
	}
	if branch != wanted {
		if err := execenv.CanLeave(path); err != nil {
			var held *execenv.WorklaneError
			if errors.As(err, &held) {
				if held.Code == execenv.WorklaneCodeUncommitted {
					return &worklaneBranchHeldError{message: fmt.Sprintf("%s holds uncommitted changes on %s", lane.Name, branch)}
				}
				if held.Code == execenv.WorklaneCodeUnpushed {
					return &worklaneBranchHeldError{message: fmt.Sprintf("%s holds %d commits on %s that are not pushed", lane.Name, held.Count, branch)}
				}
			}
			return err
		}
		if err := execenv.SwitchBranch(ctx, path, wanted, lane.DefaultBranch, logger); err != nil {
			return err
		}
	}
	// Report even if a previous attempt switched locally but its report was
	// lost. The server records no duplicate activity when the branch matches.
	if err := d.client.ReportWorklaneBranch(ctx, task.ID, wanted); err != nil {
		return fmt.Errorf("report the worklane branch: %w", err)
	}
	repo.WorktreePath = path
	return nil
}
