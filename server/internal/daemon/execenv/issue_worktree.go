package execenv

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type IssueWorktreeParams struct {
	CheckoutPath  string
	Path          string
	Branch        string
	DefaultBranch string
	// WorklanePaths reserves worklane folders when selecting a new issue
	// worktree. Leave it nil when reusing a previously recorded issue path.
	WorklanePaths []string
}

// PrepareIssueWorktree reuses a linked worktree without resetting its history.
// Only a missing worktree fetches and starts from origin's default branch.
func PrepareIssueWorktree(ctx context.Context, p IssueWorktreeParams, logger *slog.Logger) (string, error) {
	path, err := ExpandWorklanePath(p.Path)
	if err != nil {
		return "", err
	}
	if len(p.WorklanePaths) > 0 {
		path, err = IssueWorktreePath(path, p.Branch, p.WorklanePaths)
		if err != nil {
			return "", err
		}
	}
	unlock, err := lockGitRoot(p.CheckoutPath, logger)
	if err != nil {
		return "", err
	}
	defer unlock()
	if _, err := os.Lstat(path); err == nil {
		checkoutCommon, commonErr := runGitTrimmed(p.CheckoutPath, "rev-parse", "--path-format=absolute", "--git-common-dir")
		pathCommon, pathErr := runGitTrimmed(path, "rev-parse", "--path-format=absolute", "--git-common-dir")
		root, rootErr := runGitTrimmed(path, "rev-parse", "--show-toplevel")
		checkoutRoot, checkoutErr := runGitTrimmed(p.CheckoutPath, "rev-parse", "--show-toplevel")
		if commonErr != nil || pathErr != nil || rootErr != nil || checkoutErr != nil || !sameWorklanePath(root, path) || sameWorklanePath(checkoutRoot, path) || !sameWorklanePath(checkoutCommon, pathCommon) {
			return "", fmt.Errorf("%s is not a worktree of the checkout %s", path, p.CheckoutPath)
		}
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if _, err := runGitTrimmed(p.CheckoutPath, "check-ref-format", "--branch", p.Branch); err != nil {
		return "", fmt.Errorf("invalid issue branch %q: %w", p.Branch, err)
	}
	if _, _, err := FetchDefaultBranch(ctx, p.CheckoutPath, p.DefaultBranch); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	args := []string{"worktree", "add"}
	if _, err := runGitTrimmed(p.CheckoutPath, "show-ref", "--verify", "refs/heads/"+p.Branch); err == nil {
		args = append(args, path, p.Branch)
	} else {
		args = append(args, "-b", p.Branch, path, "origin/"+p.DefaultBranch)
	}
	if out, err := runGit(p.CheckoutPath, args...); err != nil {
		return "", fmt.Errorf("create issue worktree: %s: %w", strings.TrimSpace(out), err)
	}
	return path, nil
}

// RemoveIssueWorktree never discards unsaved changes or unpushed commits.
func RemoveIssueWorktree(path, branch string, logger *slog.Logger) (outcome, reason string, err error) {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return "removed", "", nil
	} else if err != nil {
		return "", "", err
	}
	common, err := runGitTrimmed(path, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", "", err
	}
	unlock, err := lockGitRoot(path, logger)
	if err != nil {
		return "", "", err
	}
	defer unlock()
	status, err := runGitTrimmed(path, "status", "--porcelain")
	if err != nil {
		return "", "", err
	}
	if status != "" {
		return "kept", "holds uncommitted changes", nil
	}
	commits, err := runGitTrimmed(path, "log", "--format=%H", branch, "--not", "--remotes")
	if err != nil {
		return "", "", err
	}
	if commits != "" {
		return "kept", fmt.Sprintf("holds %d commits that are not pushed", len(strings.Split(commits, "\n"))), nil
	}
	if out, err := runGit(common, "worktree", "remove", path); err != nil {
		return "", "", fmt.Errorf("remove issue worktree: %s: %w", out, err)
	}
	if out, err := runGit(common, "branch", "-D", branch); err != nil {
		return "", "", fmt.Errorf("remove issue branch: %s: %w", out, err)
	}
	return "removed", "", nil
}
