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
// sources are the task's local Git repositories; the first one creates a
// missing tree. An existing tree is reused when it is a worktree of any
// source, without changing its refs, index or files.
// A missing tree starts at the PR branch's history, never the default branch.
func PreparePullRequestWorktree(ctx context.Context, sources []string, path, branch string, logger *slog.Logger) (string, error) {
	expanded := make([]string, len(sources))
	for i, source := range sources {
		var err error
		if expanded[i], err = ExpandWorklanePath(source); err != nil {
			return "", err
		}
	}
	sources = expanded
	checkoutPath := sources[0]
	path, err := ExpandWorklanePath(path)
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
		if !isWorktreeOfSources(path, sources) {
			return "", fmt.Errorf("%s is not a worktree of %s", path, strings.Join(sources, " or "))
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

// isWorktreeOfSources is true when path is the top level of a worktree that
// shares a source's Git directory without being any source's own tree.
func isWorktreeOfSources(path string, sources []string) bool {
	root, err := runGitTrimmed(path, "rev-parse", "--show-toplevel")
	if err != nil || !sameWorklanePath(root, path) {
		return false
	}
	common, err := runGitTrimmed(path, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return false
	}
	shared := false
	for _, source := range sources {
		sourceRoot, err := runGitTrimmed(source, "rev-parse", "--show-toplevel")
		if err != nil || sameWorklanePath(sourceRoot, path) {
			return false
		}
		sourceCommon, err := runGitTrimmed(source, "rev-parse", "--path-format=absolute", "--git-common-dir")
		if err != nil {
			return false
		}
		shared = shared || sameWorklanePath(sourceCommon, common)
	}
	return shared
}
