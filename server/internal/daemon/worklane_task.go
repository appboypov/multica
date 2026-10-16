package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/multica-ai/multica/server/internal/worklanehead"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

var errWorklaneBranchHeld = errors.New("worklane branch holds work")

type worklaneBranchHeldError struct{ message string }

func (e *worklaneBranchHeldError) Error() string { return e.message }
func (e *worklaneBranchHeldError) Unwrap() error { return errWorklaneBranchHeld }

// worklaneTaskPath reuses the issue branch's registered pull-request worktree.
// Resolving before locking keeps an earlier issue off the lane's active tree.
func worklaneTaskPath(ctx context.Context, task Task, repo protocol.ClaimRepository) (string, error) {
	path, err := execenv.ExpandWorklanePath(repo.Worklane.Path)
	if err != nil {
		return "", err
	}
	wanted := worklanehead.Branch(task.IssueIdentifier)
	if wanted == "" {
		return "", fmt.Errorf("worklane issue task has no issue identifier")
	}
	held, err := checkoutGit(ctx, path, "for-each-ref", "--format=%(worktreepath)", "--count=1", "refs/heads/"+wanted)
	if err != nil {
		return "", fmt.Errorf("find the issue branch's worktree: %w", err)
	}
	if held != "" && filepath.Base(held) == wanted && filepath.Base(filepath.Dir(held)) == ".pull-requests" {
		return held, nil
	}
	return path, nil
}

// prepareWorklaneTask runs with the selected worktree's path lock held.
func (d *Daemon) prepareWorklaneTask(ctx context.Context, task *Task, repo *protocol.ClaimRepository, logger *slog.Logger) error {
	lane := repo.Worklane
	path, err := execenv.ExpandWorklanePath(repo.WorktreePath)
	if err != nil {
		return err
	}
	branch, err := execenv.WorklaneBranch(path)
	if err != nil {
		return err
	}
	wanted := worklanehead.Branch(task.IssueIdentifier)
	if wanted == "" {
		return fmt.Errorf("worklane issue task has no issue identifier")
	}
	lanePath, err := execenv.ExpandWorklanePath(lane.Path)
	if err != nil {
		return err
	}
	if path != lanePath {
		if branch != wanted {
			return fmt.Errorf("pull-request worktree %s holds %s, want %s", path, branch, wanted)
		}
		logger.Debug("using pull-request worktree", "path", path, "branch", wanted, "worklane", lane.Name)
		return nil
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
