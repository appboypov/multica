package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/multica-ai/multica/server/internal/workpath"
)

// taskProjectFolder is where the task's project keeps its repositories:
// {work_root}/{organisation_key}/{project_slug} (ADR-0037).
func taskProjectFolder(task Task) workpath.Project {
	return workpath.Project{Root: task.WorkRoot, Organisation: task.OrganisationKey, Slug: task.ProjectSlug}
}

// issueWorktreePath is the recorded path when there is one, else
// {project}/worktrees/{repo}/{issue key} (ADR-0037).
func issueWorktreePath(task Task, fullName, recorded string, worklanePaths []string) (string, error) {
	if recorded != "" {
		return execenv.ExpandWorklanePath(recorded)
	}
	key := task.IssueKey
	if key == "" {
		key = strings.ToLower(task.IssueIdentifier)
	}
	if key == "" || filepath.Base(key) != key || key == ".." {
		return "", fmt.Errorf("invalid issue key %q", key)
	}
	path, err := taskProjectFolder(task).Worktree(fullName, key)
	if err != nil {
		return "", fmt.Errorf("issue worktree of %s: %w", fullName, err)
	}
	path, err = execenv.ExpandWorklanePath(path)
	if err != nil {
		return "", err
	}
	return execenv.IssueWorktreePath(path, key, worklanePaths)
}

func (d *Daemon) prepareIssueWorktrees(ctx context.Context, task *Task, logger *slog.Logger) error {
	if task.IssueID == "" {
		return nil
	}
	for i := range task.Repositories {
		repo := &task.Repositories[i]
		if repo.Worklane != nil {
			if repo.Worklane.DaemonID != d.cfg.DaemonID {
				return fmt.Errorf("worklane %s lives on another machine", repo.Worklane.Name)
			}
			if err := d.prepareWorklaneTask(ctx, task, repo, logger); err != nil {
				return err
			}
			continue
		}
		if repo.CheckoutDaemonID != d.cfg.DaemonID {
			continue
		}
		path, err := issueWorktreePath(*task, repo.FullName, repo.WorktreePath, repo.WorklanePaths)
		if err != nil {
			return err
		}
		worklanePaths := repo.WorklanePaths
		if repo.WorktreePath != "" {
			worklanePaths = nil // Later tasks reuse the recorded issue path.
		}
		path, err = execenv.PrepareIssueWorktree(ctx, execenv.IssueWorktreeParams{
			CheckoutPath: repo.CheckoutPath, Path: path, Branch: task.IssueKey, DefaultBranch: repo.DefaultBranch,
			WorklanePaths: worklanePaths,
		}, logger)
		if err != nil {
			return err
		}
		branch, err := checkoutGit(ctx, path, "symbolic-ref", "--quiet", "--short", "HEAD")
		if err != nil {
			return fmt.Errorf("issue worktree has no branch: %w", err)
		}
		if err := d.client.RecordIssueWorktree(ctx, task.ID, repo.ID, path, branch); err != nil {
			return err
		}
		repo.WorktreePath = path
	}
	return nil
}

func primaryIssueWorktree(task Task, daemonID string) string {
	if task.IssueID == "" {
		return ""
	}
	for _, repo := range task.Repositories {
		if repo.Worklane != nil && repo.Worklane.DaemonID == daemonID {
			return repo.WorktreePath
		}
	}
	path := ""
	for _, repo := range task.Repositories {
		if repo.CheckoutDaemonID != daemonID {
			continue
		}
		if path != "" {
			return ""
		}
		path = repo.WorktreePath
	}
	return path
}

// resolveRealPath resolves existing parents as well as the path itself, so a
// destination has the same lock key before and after a worktree is created.
func resolveRealPath(absPath string) (string, error) {
	path := filepath.Clean(absPath)
	var missing []string
	for {
		real, err := filepath.EvalSymlinks(path)
		if err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				real = filepath.Join(real, missing[i])
			}
			return real, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		// A dangling symlink is occupied, not a missing parent to create.
		if _, statErr := os.Lstat(path); statErr == nil || !os.IsNotExist(statErr) {
			return "", err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", err
		}
		missing = append(missing, filepath.Base(path))
		path = parent
	}
}

// issueWorktreeLockPaths resolves both the run destination and any branch owner
// that must be detached. Every path is canonical and appears once.
func (d *Daemon) issueWorktreeLockPaths(ctx context.Context, task *Task) ([]string, error) {
	var paths []string
	if task.IssueID == "" {
		return paths, nil
	}
	for i := range task.Repositories {
		repo := &task.Repositories[i]
		var path string
		var err error
		if repo.Worklane != nil {
			if repo.Worklane.DaemonID != d.cfg.DaemonID {
				continue
			}
			path, err = worklaneTaskPath(ctx, *task, *repo)
			if err == nil && !repo.Worklane.InCheckout {
				owner, ownerErr := execenv.BranchWorktree(ctx, worklaneTaskSources(*repo), worklaneTaskBranch(*task, *repo))
				if ownerErr != nil {
					return nil, ownerErr
				}
				if owner != "" {
					owner, err = resolveRealPath(owner)
					paths = append(paths, owner)
				}
			}
		} else {
			if repo.CheckoutDaemonID != d.cfg.DaemonID {
				continue
			}
			path, err = issueWorktreePath(*task, repo.FullName, repo.WorktreePath, repo.WorklanePaths)
			if err == nil {
				owner, ownerErr := execenv.BranchWorktree(ctx, []string{repo.CheckoutPath}, task.IssueKey)
				if ownerErr != nil {
					return nil, ownerErr
				}
				if isPullRequestTaskPath(owner, task.IssueKey) {
					path = owner
				}
			}
		}
		if err != nil {
			return nil, err
		}
		repo.WorktreePath = path
		path, err = resolveRealPath(path)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return slices.Compact(paths), nil
}

// acquireIssueWorktreeLocks takes locks in canonical path order. Resolution is
// repeated after waiting because a handoff can move the branch while queued.
func (d *Daemon) acquireIssueWorktreeLocks(ctx context.Context, task *Task, logger *slog.Logger) (func(), bool) {
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	for {
		paths, err := d.issueWorktreeLockPaths(waitCtx, task)
		if err != nil {
			d.reportTerminalTask(ctx, terminalTaskReport{kind: terminalTaskReportFail, taskID: task.ID, errorMessage: err.Error(), failureReason: "environment_prepare_failed"})
			return nil, true
		}
		releases := make([]func(), 0, len(paths))
		releaseAll := func() {
			for i := len(releases) - 1; i >= 0; i-- {
				releases[i]()
			}
		}
		for _, path := range paths {
			counted := false
			release, err := d.localPathLocks.Acquire(waitCtx, path, task.ID, func(holder string) {
				d.resourceWaitTasks.Add(1)
				counted = true
				reason := filepath.Base(path)
				if holder != "" {
					reason += " (held by task " + shortID(holder) + ")"
				}
				if err := d.client.MarkTaskWaitingLocalDirectory(waitCtx, task.ID, reason); err != nil {
					logger.Warn("mark issue worktree waiting failed", "error", err)
				}
				cancelled := d.watchTaskCancellation(waitCtx, task.ID, 5*time.Second, logger)
				go func() {
					select {
					case <-cancelled:
						cancel()
					case <-waitCtx.Done():
					}
				}()
			})
			if counted {
				d.resourceWaitTasks.Add(-1)
			}
			if err != nil {
				releaseAll()
				return nil, true
			}
			releases = append(releases, release)
		}
		resolved, err := d.issueWorktreeLockPaths(waitCtx, task)
		if err != nil {
			releaseAll()
			d.reportTerminalTask(ctx, terminalTaskReport{kind: terminalTaskReportFail, taskID: task.ID, errorMessage: err.Error(), failureReason: "environment_prepare_failed"})
			return nil, true
		}
		if slices.Equal(paths, resolved) {
			return releaseAll, false
		}
		releaseAll()
	}
}

// LocalPathLocker serialises agent tasks that share the same on-disk path.
// The lock is owned for the entire lifetime of a task (claim → context
// write → agent execution → result report), not just the agent execution
// window, because the context files and skill scratch directories the
// daemon writes at task-prepare time can race with a sibling task on the
// same path.
//
// Implementation: per-key sync.Mutex inside a map guarded by mu. When a
// task can't take the lock immediately, the waiter blocks on the per-key
// Mutex itself — that gives FIFO-ish behaviour from the Go scheduler
// (sufficient for our load; the issue body asks for a wait queue, not a
// strict-priority queue). Holder bookkeeping (current holder task id) is
// surfaced via Holder so callers can build a UI-friendly wait_reason.
type LocalPathLocker struct {
	mu    sync.Mutex
	locks map[string]*pathLockEntry
}

type pathLockEntry struct {
	mu       sync.Mutex // serialises holders for this key
	mu2      sync.Mutex // guards holderID under contention
	holderID string     // current owner, for UI hints; empty when free
}

// NewLocalPathLocker returns an empty locker. Safe for concurrent use.
func NewLocalPathLocker() *LocalPathLocker {
	return &LocalPathLocker{locks: make(map[string]*pathLockEntry)}
}

// Holder returns the task id currently holding the lock for realPath, or
// "" if no task holds it. Used to populate the wait_reason hint the daemon
// posts to the server when it parks a task — the UI then shows "waiting for
// <path> (held by task <short id>)".
func (l *LocalPathLocker) Holder(realPath string) string {
	l.mu.Lock()
	entry, ok := l.locks[realPath]
	l.mu.Unlock()
	if !ok {
		return ""
	}
	entry.mu2.Lock()
	defer entry.mu2.Unlock()
	return entry.holderID
}

// Acquire takes the lock for realPath on behalf of taskID. If the lock is
// already held, onWait is invoked (synchronously, before this goroutine
// blocks) with the current holder id so callers can flip the task into the
// server-side waiting_local_directory state. onWait may be nil for callers
// that don't need the side effect.
//
// Returns a release func that the caller must invoke (typically deferred)
// to free the lock. The release is idempotent.
//
// Acquire is cancellable via ctx. When ctx is cancelled while the goroutine
// is blocked on the lock, Acquire returns ctx.Err() and the lock is NOT
// taken. This is the same contract as sync.Mutex.Lock paired with
// context-aware cancellation — a daemon shutdown won't wedge inside the
// per-path wait queue.
func (l *LocalPathLocker) Acquire(ctx context.Context, realPath, taskID string, onWait func(holder string)) (func(), error) {
	if realPath == "" {
		return nil, errors.New("local_directory: realpath required for lock")
	}
	if taskID == "" {
		return nil, errors.New("local_directory: taskID required for lock")
	}

	l.mu.Lock()
	entry, ok := l.locks[realPath]
	if !ok {
		entry = &pathLockEntry{}
		l.locks[realPath] = entry
	}
	l.mu.Unlock()

	// Try the fast path first — no allocation, no waiter goroutine.
	if entry.mu.TryLock() {
		entry.mu2.Lock()
		entry.holderID = taskID
		entry.mu2.Unlock()
		return l.releaser(realPath, entry), nil
	}

	// Slow path: somebody else holds the lock. Fire onWait once with the
	// current holder so the daemon can stamp the server-side wait state,
	// then block until either we win the lock or ctx is cancelled.
	if onWait != nil {
		entry.mu2.Lock()
		holder := entry.holderID
		entry.mu2.Unlock()
		onWait(holder)
	}

	acquired := make(chan struct{})
	go func() {
		entry.mu.Lock()
		close(acquired)
	}()

	select {
	case <-acquired:
		entry.mu2.Lock()
		entry.holderID = taskID
		entry.mu2.Unlock()
		return l.releaser(realPath, entry), nil
	case <-ctx.Done():
		// We lost the wait — the goroutine above will still complete and
		// take the lock. Spin off a clean-up goroutine that releases it
		// the moment the acquire returns so a future caller isn't stuck
		// behind a phantom holder. The bookkeeping is best-effort: no
		// holder id is set, since this task never owned the lock.
		go func() {
			<-acquired
			entry.mu.Unlock()
		}()
		return nil, ctx.Err()
	}
}

// TryAcquire takes the lock for realPath on behalf of ownerID only when it
// is free. ok is false when another holder has it; the caller then refuses
// rather than waits (a worklane remove answers busy).
func (l *LocalPathLocker) TryAcquire(realPath, ownerID string) (release func(), ok bool) {
	l.mu.Lock()
	entry, found := l.locks[realPath]
	if !found {
		entry = &pathLockEntry{}
		l.locks[realPath] = entry
	}
	l.mu.Unlock()
	if !entry.mu.TryLock() {
		return nil, false
	}
	entry.mu2.Lock()
	entry.holderID = ownerID
	entry.mu2.Unlock()
	return l.releaser(realPath, entry), true
}

// releaser returns the unlock callback. Idempotent via a once flag so a
// deferred release is safe even when the caller has already explicitly
// released after task completion.
func (l *LocalPathLocker) releaser(realPath string, entry *pathLockEntry) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			entry.mu2.Lock()
			entry.holderID = ""
			entry.mu2.Unlock()
			entry.mu.Unlock()
			// We deliberately keep the entry in the map even when nothing
			// is queued. The cost is one *pathLockEntry per distinct path
			// the daemon has ever served, which is bounded by the number
			// of local_directory project resources a workspace has — tiny
			// in practice. Pruning would race with a sibling caller that
			// just looked up the same entry and is about to TryLock.
			_ = realPath
		})
	}
}
