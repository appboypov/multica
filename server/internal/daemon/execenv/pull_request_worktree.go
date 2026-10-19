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
// handoffPath is the branch owner's path with its task path lock also held.
// sources are the task's local Git repositories. An existing tree is reused
// without changing its refs or files. A missing tree starts at the PR branch.
func PreparePullRequestWorktree(ctx context.Context, sources []string, path, branch, handoffPath string, logger *slog.Logger) (string, error) {
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
	owner, err := BranchWorktree(ctx, sources, branch)
	if err != nil {
		return "", err
	}
	if owner != "" {
		checkoutPath = owner
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
		ref, err := fetchPullRequestBranch(ctx, checkoutPath, branch)
		if err != nil {
			return "", err
		}
		args = append(args, "-b", branch, path, ref)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	owner, err = BranchWorktree(ctx, []string{checkoutPath}, branch)
	if err != nil {
		return "", err
	}
	if owner != "" {
		if handoffPath == "" || !sameWorklanePath(owner, handoffPath) {
			return "", fmt.Errorf("pull-request branch %q is held by unlocked worktree %s", branch, owner)
		}
		if err := CanLeave(owner); err != nil {
			return "", fmt.Errorf("cannot detach %s: %w", owner, err)
		}
		if out, err := runGit(owner, "switch", "--detach", "HEAD"); err != nil {
			return "", fmt.Errorf("detach branch owner %s: %s: %w", owner, strings.TrimSpace(out), err)
		}
		if logger != nil {
			logger.Debug("detached pull-request branch owner", "path", owner, "branch", branch)
		}
	}
	if out, err := runGit(checkoutPath, args...); err != nil {
		if owner != "" {
			if restoreOut, restoreErr := runGit(owner, "switch", branch); restoreErr != nil {
				return "", fmt.Errorf("create pull-request worktree: %s: %w; restore %s: %s: %v", strings.TrimSpace(out), err, owner, strings.TrimSpace(restoreOut), restoreErr)
			}
		}
		return "", fmt.Errorf("create pull-request worktree: %s: %w", strings.TrimSpace(out), err)
	}
	return path, nil
}

// BranchWorktree returns the worktree holding the exact local branch.
func BranchWorktree(ctx context.Context, sources []string, branch string) (string, error) {
	for _, source := range sources {
		path, err := ExpandWorklanePath(source)
		if err != nil {
			return "", err
		}
		cmd := exec.CommandContext(ctx, "git", "-C", path, "for-each-ref", "--format=%(refname)\t%(worktreepath)", "refs/heads/"+branch)
		out, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("find branch %q worktree in %s: %w", branch, path, err)
		}
		for line := range strings.SplitSeq(string(out), "\n") {
			ref, tree, _ := strings.Cut(line, "\t")
			if ref == "refs/heads/"+branch && tree != "" {
				return tree, nil
			}
		}
	}
	return "", nil
}

// SwitchPullRequestBranch switches a checkout to an open pull request's
// branch in place (ADR-0031): its local branch when it exists, else a branch
// made from the freshly fetched origin branch, never from the default branch.
// The caller holds the checkout's path lock and has checked CanLeave.
func SwitchPullRequestBranch(ctx context.Context, path, branch string, logger *slog.Logger) error {
	checked, err := runGitTrimmed(path, "check-ref-format", "--branch", branch)
	if err != nil || checked != branch {
		return fmt.Errorf("invalid pull-request branch %q", branch)
	}
	unlock, err := lockGitRoot(path, logger)
	if err != nil {
		return err
	}
	defer unlock()
	args := []string{"switch", branch}
	if _, err := runGitTrimmed(path, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err != nil {
		ref, err := fetchPullRequestBranch(ctx, path, branch)
		if err != nil {
			return err
		}
		args = []string{"switch", "-c", branch, ref}
	}
	if out, err := runGit(path, args...); err != nil {
		return fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(out), err)
	}
	return nil
}

// fetchPullRequestBranch refreshes only the PR branch with an explicit
// refspec, even when the checkout's configured fetch refspec excludes it, and
// returns its remote-tracking ref. A failed fetch must not reuse a stale
// remote-tracking ref.
func fetchPullRequestBranch(ctx context.Context, checkoutPath, branch string) (string, error) {
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
	return ref, nil
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
