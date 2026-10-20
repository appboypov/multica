package execenv

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Worklane worktrees (worklanes-of-a-repository design.md decisions 3 and 4).
// A worklane's worktree is a linked worktree of the repository's checkout,
// detached at origin/{default_branch}, or a checkout of the repository that
// already sits at the worklane's path and is used as it is.

const (
	// worklaneFetchTimeout bounds `git fetch origin {default_branch}`.
	worklaneFetchTimeout = 10 * time.Minute
	// WorklaneInstallTimeout bounds the install command.
	WorklaneInstallTimeout = 20 * time.Minute
	// worklaneInstallOutputLimit keeps the last 8 KB of install output.
	worklaneInstallOutputLimit = 8 * 1024
	// worklaneErrorLines is how many of git's last output lines a failed
	// step carries.
	worklaneErrorLines = 5
)

// Refusal and failure codes a worklane request reports.
const (
	WorklaneCodeFetchFailed       = "fetch_failed"
	WorklaneCodePathIssueWorktree = "path_issue_worktree"
	WorklaneCodePathNotCheckout   = "path_not_checkout"
	WorklaneCodeAddFailed         = "add_worktree_failed"
	WorklaneCodeInstallFailed     = "install_failed"
	WorklaneCodeBusy              = "busy"
	WorklaneCodeUncommitted       = "uncommitted"
	WorklaneCodeUnpushed          = "unpushed"
	WorklaneCodeStashed           = "stashed"
	WorklaneCodeRemoveFailed      = "remove_failed"
)

// WorklaneError is a step's failure or refusal: Code is one of the
// WorklaneCode constants, Detail the sentence the dialog shows, Output what
// the command printed, and Count the number of unpushed commits.
type WorklaneError struct {
	Code   string
	Detail string
	Output string
	Count  int
}

func (e *WorklaneError) Error() string {
	if e.Output != "" {
		return e.Detail + ": " + e.Output
	}
	return e.Detail
}

// ExpandWorklanePath expands a leading `~` to the runtime user's home and
// returns the absolute, clean path.
func ExpandWorklanePath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home: %w", err)
		}
		path = home + strings.TrimPrefix(path, "~")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

// FetchDefaultBranch fetches origin's default branch into the checkout and
// resolves refs/remotes/origin/{branch} to a commit. took is how long the
// fetch ran. A failure carries git's last error lines.
func FetchDefaultBranch(ctx context.Context, checkoutPath, defaultBranch string) (commit string, took time.Duration, err error) {
	start := time.Now()
	fetchCtx, cancel := context.WithTimeout(ctx, worklaneFetchTimeout)
	defer cancel()
	cmd := exec.CommandContext(fetchCtx, "git", "-C", checkoutPath, "fetch", "origin", defaultBranch)
	cmd.WaitDelay = 5 * time.Second
	out, runErr := cmd.CombinedOutput()
	took = time.Since(start)
	if runErr != nil {
		return "", took, &WorklaneError{
			Code:   WorklaneCodeFetchFailed,
			Detail: fmt.Sprintf("git fetch origin %s failed", defaultBranch),
			Output: lastLines(string(out), worklaneErrorLines),
		}
	}
	commit, err = ResolveOriginDefaultBranch(checkoutPath, defaultBranch)
	return commit, took, err
}

// ResolveOriginDefaultBranch resolves refs/remotes/origin/{branch} in the
// checkout to a commit, without fetching: a retry from add_worktree uses the
// commit the earlier fetch brought.
func ResolveOriginDefaultBranch(checkoutPath, defaultBranch string) (string, error) {
	commit, err := runGitTrimmed(checkoutPath, "rev-parse", "--verify", "refs/remotes/origin/"+defaultBranch+"^{commit}")
	if err != nil {
		return "", &WorklaneError{
			Code:   WorklaneCodeFetchFailed,
			Detail: fmt.Sprintf("origin/%s does not resolve to a commit", defaultBranch),
			Output: lastLines(err.Error(), worklaneErrorLines),
		}
	}
	return commit, nil
}

// WorklaneIssueWorktree is an issue worktree of the repository on this
// machine; a worklane never takes its path (ADR-0009).
type WorklaneIssueWorktree struct {
	Path     string
	IssueKey string
}

// AddWorklaneWorktreeParams describes the add_worktree step.
type AddWorklaneWorktreeParams struct {
	CheckoutPath       string
	FullName           string
	Path               string // as the server computed it; `~` allowed
	Commit             string
	IssueWorktreePaths []WorklaneIssueWorktree
}

// AddWorklaneWorktreeResult is what add_worktree made or found.
type AddWorklaneWorktreeResult struct {
	Path   string // absolute and clean
	Reused bool   // a checkout of the repository was already there
}

// AddWorklaneWorktree makes the worklane's worktree under the checkout's repo
// lock. A missing path or an empty folder gets `worktree add --detach` at
// Commit; a checkout of the repository at the path is used as it is; anything
// else is refused and left untouched.
func AddWorklaneWorktree(p AddWorklaneWorktreeParams, logger *slog.Logger) (AddWorklaneWorktreeResult, error) {
	path, err := ExpandWorklanePath(p.Path)
	if err != nil {
		return AddWorklaneWorktreeResult{}, err
	}
	for _, iw := range p.IssueWorktreePaths {
		issuePath, err := ExpandWorklanePath(iw.Path)
		if err != nil {
			continue
		}
		if sameWorklanePath(issuePath, path) {
			return AddWorklaneWorktreeResult{}, &WorklaneError{
				Code:   WorklaneCodePathIssueWorktree,
				Detail: fmt.Sprintf("%s is the worktree of %s. Pick another folder.", path, iw.IssueKey),
			}
		}
	}

	unlock, err := lockGitRoot(p.CheckoutPath, logger)
	if err != nil {
		return AddWorklaneWorktreeResult{}, &WorklaneError{
			Code:   WorklaneCodeAddFailed,
			Detail: fmt.Sprintf("the repository at %s is busy", p.CheckoutPath),
			Output: err.Error(),
		}
	}
	defer unlock()

	empty, exists, err := worklanePathState(path)
	if err != nil {
		return AddWorklaneWorktreeResult{}, &WorklaneError{Code: WorklaneCodeAddFailed, Detail: fmt.Sprintf("cannot read %s", path), Output: err.Error()}
	}
	if exists && !empty {
		if isCheckoutOfRepository(path, p.FullName) {
			return AddWorklaneWorktreeResult{Path: path, Reused: true}, nil
		}
		return AddWorklaneWorktreeResult{}, &WorklaneError{
			Code:   WorklaneCodePathNotCheckout,
			Detail: fmt.Sprintf("%s holds something that is not a checkout of %s. Move it or pick another folder.", path, p.FullName),
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return AddWorklaneWorktreeResult{}, &WorklaneError{Code: WorklaneCodeAddFailed, Detail: fmt.Sprintf("cannot make %s", filepath.Dir(path)), Output: err.Error()}
	}
	if out, err := runGit(p.CheckoutPath, "worktree", "add", "--detach", path, p.Commit); err != nil {
		return AddWorklaneWorktreeResult{}, &WorklaneError{
			Code:   WorklaneCodeAddFailed,
			Detail: fmt.Sprintf("git worktree add %s failed", path),
			Output: lastLines(out, worklaneErrorLines),
		}
	}
	return AddWorklaneWorktreeResult{Path: path}, nil
}

// worklanePathState reports whether path exists and, when it is a folder,
// whether it is empty. A file at the path exists and is not empty.
func worklanePathState(path string) (empty, exists bool, err error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	if !info.IsDir() {
		return false, true, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, true, err
	}
	return len(entries) == 0, true, nil
}

// isCheckoutOfRepository is true when path is the top level of a git working
// tree whose origin names fullName.
func isCheckoutOfRepository(path, fullName string) bool {
	top, err := runGitTrimmed(path, "rev-parse", "--show-toplevel")
	if err != nil || !sameWorklanePath(top, path) {
		return false
	}
	origin, err := runGitTrimmed(path, "remote", "get-url", "origin")
	if err != nil {
		return false
	}
	return strings.EqualFold(GitHubFullNameFromURL(origin), fullName)
}

// GitHubFullNameFromURL reads owner/repo from a GitHub HTTPS, SSH or
// scp-style remote, ignoring case and a trailing .git. Other hosts and
// malformed repository paths do not identify a GitHub repository.
func GitHubFullNameFromURL(remote string) string {
	remote = strings.TrimSpace(remote)
	var host, path string
	if strings.Contains(remote, "://") {
		u, err := url.Parse(remote)
		if err != nil || (u.Scheme != "https" && u.Scheme != "ssh") || u.RawQuery != "" || u.Fragment != "" {
			return ""
		}
		host, path = u.Hostname(), u.Path
	} else {
		var ok bool
		host, path, ok = strings.Cut(remote, ":")
		if !ok {
			return ""
		}
		if _, hostname, ok := strings.Cut(host, "@"); ok {
			host = hostname
		}
	}
	if !strings.EqualFold(host, "github.com") {
		return ""
	}
	path = strings.Trim(strings.TrimSpace(path), "/")
	if len(path) >= 4 && strings.EqualFold(path[len(path)-4:], ".git") {
		path = path[:len(path)-4]
	}
	owner, repo, ok := strings.Cut(path, "/")
	if !ok || owner == "" || repo == "" || strings.Contains(repo, "/") {
		return ""
	}
	return owner + "/" + repo
}

func sameWorklanePath(a, b string) bool {
	if a == b {
		return true
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && filepath.Clean(ra) == filepath.Clean(rb)
}

// RunInstall runs the install command with `sh -c` in the worktree, with the
// daemon's environment and a 20-minute limit. output is the last 8 KB of what
// it printed. A non-zero exit is a WorklaneError with the exit code; the
// caller holds the worktree path's lock.
func RunInstall(ctx context.Context, dir, command string) (output string, err error) {
	installCtx, cancel := context.WithTimeout(ctx, WorklaneInstallTimeout)
	defer cancel()
	buf := &tailBuffer{limit: worklaneInstallOutputLimit}
	cmd := exec.CommandContext(installCtx, "sh", "-c", command)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	cmd.Stdout = buf
	cmd.Stderr = buf
	cmd.WaitDelay = 5 * time.Second
	runErr := cmd.Run()
	output = buf.String()
	if runErr == nil {
		return output, nil
	}
	if errors.Is(installCtx.Err(), context.DeadlineExceeded) {
		return output, &WorklaneError{
			Code:   WorklaneCodeInstallFailed,
			Detail: fmt.Sprintf("%s ran longer than %s", command, WorklaneInstallTimeout),
			Output: output,
		}
	}
	code := -1
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		code = exitErr.ExitCode()
	}
	return output, &WorklaneError{
		Code:   WorklaneCodeInstallFailed,
		Detail: fmt.Sprintf("%s exited with %s", command, strconv.Itoa(code)),
		Output: output,
	}
}

// RemoveWorklaneWorktreeParams describes a remove request. The caller holds
// the worktree path's lock, and refuses with busy when it cannot take it.
type RemoveWorklaneWorktreeParams struct {
	CheckoutPath  string
	Path          string
	DefaultBranch string
}

// RemoveWorklaneWorktree removes a worklane's worktree when nothing would be
// lost (design.md decision 4). A missing path is removed already. Uncommitted
// changes, commits not on origin/{default_branch} and, for a standalone clone,
// stashes refuse with a WorklaneError. No branch is deleted.
func RemoveWorklaneWorktree(ctx context.Context, p RemoveWorklaneWorktreeParams, logger *slog.Logger) error {
	path, err := ExpandWorklanePath(p.Path)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		pruneWorktrees(p.CheckoutPath, logger)
		return nil
	} else if err != nil {
		return &WorklaneError{Code: WorklaneCodeRemoveFailed, Detail: fmt.Sprintf("cannot read %s", path), Output: err.Error()}
	}
	top, err := runGitTrimmed(path, "rev-parse", "--show-toplevel")
	if err != nil || !sameWorklanePath(top, path) || sameWorklanePath(path, p.CheckoutPath) {
		return &WorklaneError{Code: WorklaneCodePathNotCheckout, Detail: fmt.Sprintf("%s is not a separate worklane checkout", path)}
	}

	status, err := runGitStdout(path, "status", "--porcelain")
	if err != nil {
		return &WorklaneError{Code: WorklaneCodeRemoveFailed, Detail: fmt.Sprintf("git status in %s failed", path), Output: lastLines(err.Error(), worklaneErrorLines)}
	}
	if strings.TrimSpace(status) != "" {
		return &WorklaneError{Code: WorklaneCodeUncommitted, Detail: "the worktree holds uncommitted changes"}
	}

	// A failed fetch is ignored: a stale origin ref only refuses more.
	fetchCtx, cancel := context.WithTimeout(ctx, worklaneFetchTimeout)
	fetch := exec.CommandContext(fetchCtx, "git", "-C", path, "fetch", "origin", p.DefaultBranch)
	fetch.WaitDelay = 5 * time.Second
	if out, err := fetch.CombinedOutput(); err != nil && logger != nil {
		logger.Warn("execenv: worklane remove fetch failed; checking against the local origin ref",
			"path", path, "output", lastLines(string(out), worklaneErrorLines), "error", err)
	}
	cancel()
	countOut, err := runGitTrimmed(path, "rev-list", "--count", "HEAD", "--not", "refs/remotes/origin/"+p.DefaultBranch)
	if err != nil {
		return &WorklaneError{Code: WorklaneCodeUnpushed, Detail: fmt.Sprintf("the worktree's commits cannot be compared with origin/%s", p.DefaultBranch), Output: lastLines(err.Error(), worklaneErrorLines)}
	}
	if count, _ := strconv.Atoi(countOut); count > 0 {
		return &WorklaneError{Code: WorklaneCodeUnpushed, Detail: fmt.Sprintf("the worktree holds commits that are not on origin/%s", p.DefaultBranch), Count: count}
	}

	unlock, err := lockGitRoot(p.CheckoutPath, logger)
	if err != nil {
		return &WorklaneError{Code: WorklaneCodeBusy, Detail: fmt.Sprintf("the repository at %s is busy", p.CheckoutPath)}
	}
	defer unlock()

	if isLinkedWorktreeOf(path, p.CheckoutPath) {
		if out, err := runGit(p.CheckoutPath, "worktree", "remove", path); err != nil {
			return &WorklaneError{Code: WorklaneCodeRemoveFailed, Detail: fmt.Sprintf("git worktree remove %s failed", path), Output: lastLines(out, worklaneErrorLines)}
		}
		return nil
	}

	// A standalone clone used as it is: its stashes live only in it.
	stashes, err := runGitTrimmed(path, "stash", "list")
	if err != nil {
		return &WorklaneError{Code: WorklaneCodeRemoveFailed, Detail: fmt.Sprintf("git stash list in %s failed", path), Output: lastLines(err.Error(), worklaneErrorLines)}
	}
	if stashes != "" {
		return &WorklaneError{Code: WorklaneCodeStashed, Detail: "the worktree holds stashed changes"}
	}
	if err := os.RemoveAll(path); err != nil {
		return &WorklaneError{Code: WorklaneCodeRemoveFailed, Detail: fmt.Sprintf("cannot remove %s", path), Output: err.Error()}
	}
	return nil
}

// isLinkedWorktreeOf is true when path is a worktree sharing the checkout's
// common git dir.
func isLinkedWorktreeOf(path, checkoutPath string) bool {
	pathCommon, err := gitCommonDirFor(path)
	if err != nil {
		return false
	}
	checkoutCommon, err := gitCommonDirFor(checkoutPath)
	if err != nil {
		return false
	}
	return sameWorklanePath(pathCommon, checkoutCommon)
}

func pruneWorktrees(checkoutPath string, logger *slog.Logger) {
	if out, err := runGit(checkoutPath, "worktree", "prune"); err != nil && logger != nil {
		logger.Warn("execenv: git worktree prune failed", "checkout", checkoutPath, "output", out, "error", err)
	}
}

// lastLines returns the last n non-empty lines of s.
func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// tailBuffer keeps the last limit bytes written to it.
type tailBuffer struct {
	limit int
	buf   []byte
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if len(p) >= b.limit {
		b.buf = append(b.buf[:0], p[len(p)-b.limit:]...)
		return n, nil
	}
	if over := len(b.buf) + len(p) - b.limit; over > 0 {
		b.buf = append(b.buf[:0], b.buf[over:]...)
	}
	b.buf = append(b.buf, p...)
	return n, nil
}

func (b *tailBuffer) String() string { return string(b.buf) }

// WorklaneBranch is empty while the worktree is detached.
func WorklaneBranch(path string) (string, error) {
	branch, err := runGitTrimmed(path, "rev-parse", "--abbrev-ref", "HEAD")
	if branch == "HEAD" {
		branch = ""
	}
	return branch, err
}

// CanLeave refuses a branch switch if it would abandon uncommitted work or
// commits that are missing from every remote. Untracked runtime paths are not
// work. Detached worktrees do not hold an issue branch.
func CanLeave(path string) error {
	status, err := runGitStdout(path, "status", "--porcelain", "-z")
	if err != nil {
		return err
	}
	if holdsWork(status) {
		return &WorklaneError{Code: WorklaneCodeUncommitted, Detail: "the worktree holds uncommitted changes"}
	}
	branch, err := WorklaneBranch(path)
	if err != nil || branch == "" {
		return err
	}
	out, err := runGitTrimmed(path, "rev-list", "--count", "HEAD", "--not", "--remotes")
	if err != nil {
		return err
	}
	count, err := strconv.Atoi(out)
	if err != nil {
		return fmt.Errorf("read unpushed commit count %q: %w", out, err)
	}
	if count > 0 {
		return &WorklaneError{Code: WorklaneCodeUnpushed, Count: count, Detail: "the worktree holds commits that are not pushed"}
	}
	return nil
}

// worklaneRuntimeFolders and worklaneRuntimeFiles are the paths at a
// worktree's root that agent runtimes and remote checks make.
var (
	worklaneRuntimeFolders = []string{".multica/", ".omp/", ".agent_context/", ".crabbox/"}
	worklaneRuntimeFiles   = []string{"AGENTS.md", "CLAUDE.md"}
)

// holdsWork is true when `git status --porcelain -z` lists any entry other
// than an untracked runtime path.
func holdsWork(status string) bool {
	for _, entry := range strings.Split(status, "\x00") {
		if entry == "" {
			continue
		}
		path, untracked := strings.CutPrefix(entry, "?? ")
		if !untracked || !isWorklaneRuntimePath(path) {
			return true
		}
	}
	return false
}

func isWorklaneRuntimePath(path string) bool {
	if slices.Contains(worklaneRuntimeFiles, path) {
		return true
	}
	return slices.ContainsFunc(worklaneRuntimeFolders, func(folder string) bool {
		return strings.HasPrefix(path, folder)
	})
}

// FetchWorklaneSwitch fetches the default branch under the shared git lock.
func FetchWorklaneSwitch(ctx context.Context, path, defaultBranch string, logger *slog.Logger) error {
	unlock, err := lockGitRoot(path, logger)
	if err != nil {
		return err
	}
	defer unlock()
	_, _, err = FetchDefaultBranch(ctx, path, defaultBranch)
	return err
}

// SwitchBranch fetches and switches a worktree whose caller holds its path
// lock and has checked CanLeave.
func SwitchBranch(ctx context.Context, path, branch, defaultBranch string, logger *slog.Logger) error {
	if err := FetchWorklaneSwitch(ctx, path, defaultBranch, logger); err != nil {
		return err
	}
	return SwitchFetchedBranch(path, branch, defaultBranch, logger)
}

// SwitchFetchedBranch runs the switch step after the separately reported
// fetch step of a worklane request.
func SwitchFetchedBranch(path, branch, defaultBranch string, logger *slog.Logger) error {
	unlock, err := lockGitRoot(path, logger)
	if err != nil {
		return err
	}
	defer unlock()
	args := []string{"switch", "--detach", "origin/" + defaultBranch}
	if branch != "" {
		if _, err := runGitTrimmed(path, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err == nil {
			args = []string{"switch", branch}
		} else {
			args = []string{"switch", "--no-track", "-c", branch, "origin/" + defaultBranch}
		}
	}
	if out, err := runGit(path, args...); err != nil {
		return fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(out), err)
	}
	return nil
}
