package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/multica-ai/multica/server/internal/worklanehead"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

var errWorklaneBranchHeld = errors.New("worklane branch holds work")

type worklaneBranchHeldError struct{ message string }

func (e *worklaneBranchHeldError) Error() string { return e.message }
func (e *worklaneBranchHeldError) Unwrap() error { return errWorklaneBranchHeld }

// prepareWorklaneTask is called with the worklane's path lock held for the
// entire task. Its repository uses that worktree, never an issue worktree.
func (d *Daemon) prepareWorklaneTask(ctx context.Context, task *Task, repo *protocol.ClaimRepository, logger *slog.Logger) error {
	lane := repo.Worklane
	path, err := execenv.ExpandWorklanePath(lane.Path)
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
