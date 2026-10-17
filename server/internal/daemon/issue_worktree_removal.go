package daemon

import (
	"context"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func (d *Daemon) handleIssueWorktreeRemoval(ctx context.Context, runtimeID string, row protocol.DaemonHeartbeatIssueWorktreeRemoval) {
	path, err := execenv.ExpandWorklanePath(row.Path)
	if err != nil {
		d.logger.Error("resolve issue worktree removal path", "error", err)
		return
	}
	key, err := resolveRealPath(path)
	if err != nil {
		d.logger.Error("resolve issue worktree removal lock", "error", err)
		return
	}
	release, ok := d.localPathLocks.TryAcquire(key, "removal:"+row.ID)
	if !ok {
		return
	}
	defer release()
	outcome, reason, err := execenv.RemoveIssueWorktree(path, row.Branch, d.logger)
	if err != nil {
		d.logger.Error("remove issue worktree failed", "path", path, "error", err)
		return
	}
	if err := d.client.ReportIssueWorktreeRemoval(ctx, runtimeID, row.ID, outcome, reason); err != nil {
		d.logger.Error("report issue worktree removal failed", "id", row.ID, "error", err)
	}
}
