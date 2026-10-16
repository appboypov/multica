package execenv

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// PreparePullRequestWorktree runs with the destination's task path lock held.
// Existing trees are validated without changing their refs, index or files.
// A missing tree starts at the PR branch's history, never the default branch.
func PreparePullRequestWorktree(ctx context.Context, checkoutPath, path, branch string, logger *slog.Logger) (string, error) {
	checkoutPath, err := ExpandWorklanePath(checkoutPath)
	if err != nil {
		return "", err
	}
	path, err = ExpandWorklanePath(path)
	if err != nil {
		return "", err
	}
	checked, err := runGitTrimmed(checkoutPath, "check-ref-format", "--branch", branch)
	if err != nil || checked != branch {
		return "", fmt.Errorf("invalid pull-request branch %q", branch)
	}
	unlock, err := lockGitRoot(checkoutPath, logger)
	if err != nil {
		return "", err
	}
	defer unlock()
	if _, err := os.Lstat(path); err == nil {
		checkoutCommon, commonErr := runGitTrimmed(checkoutPath, "rev-parse", "--path-format=absolute", "--git-common-dir")
		pathCommon, pathErr := runGitTrimmed(path, "rev-parse", "--path-format=absolute", "--git-common-dir")
		root, rootErr := runGitTrimmed(path, "rev-parse", "--show-toplevel")
		checkoutRoot, checkoutErr := runGitTrimmed(checkoutPath, "rev-parse", "--show-toplevel")
		if commonErr != nil || pathErr != nil || rootErr != nil || checkoutErr != nil || !sameWorklanePath(root, path) || sameWorklanePath(checkoutRoot, path) || !sameWorklanePath(checkoutCommon, pathCommon) {
			return "", fmt.Errorf("%s is not a worktree of the checkout %s", path, checkoutPath)
		}
		current, err := runGitTrimmed(path, "symbolic-ref", "--quiet", "--short", "HEAD")
		if err != nil || current != branch {
			return "", fmt.Errorf("pull-request worktree %s holds %q, want %q", path, current, branch)
		}
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	args := []string{"worktree", "add"}
	if _, err := runGitTrimmed(checkoutPath, "show-ref", "--verify", "refs/heads/"+branch); err == nil {
		args = append(args, path, branch)
	} else {
		// An explicit refspec refreshes only this PR branch even when the
		// checkout's configured fetch refspec excludes it. A failed fetch
		// must not reuse a stale remote-tracking ref.
		ref := "refs/remotes/origin/" + branch
		fetchCtx, cancel := context.WithTimeout(ctx, worklaneFetchTimeout)
		cmd := exec.CommandContext(fetchCtx, "git", "-C", checkoutPath, "fetch", "--no-tags", "origin", "+refs/heads/"+branch+":"+ref)
		cmd.WaitDelay = 5 * time.Second
		out, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			return "", fmt.Errorf("fetch pull-request branch %q: %s: %w", branch, lastLines(string(out), worklaneErrorLines), err)
		}
		if _, err := runGitTrimmed(checkoutPath, "rev-parse", "--verify", ref+"^{commit}"); err != nil {
			return "", fmt.Errorf("resolve pull-request branch %q: %w", branch, err)
		}
		args = append(args, "-b", branch, path, ref)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if out, err := runGit(checkoutPath, args...); err != nil {
		return "", fmt.Errorf("create pull-request worktree: %s: %w", strings.TrimSpace(out), err)
	}
	return path, nil
}
