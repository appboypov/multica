package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
)

func branchHandoffFixture(t *testing.T) laneRoutingFixture {
	t.Helper()
	f := newLaneRoutingFixture(t, "work-1021")
	laneRoutingWrite(t, f.lane, "shared.txt", "base\n")
	if err := os.Remove(filepath.Join(f.lane, "unfinished.txt")); err != nil {
		t.Fatal(err)
	}
	laneRoutingGit(t, f.lane, "switch", f.branch)
	return f
}

func TestPullRequestHandoffPreservesCommit(t *testing.T) {
	for _, owner := range []string{"lane", "issue"} {
		t.Run(owner, func(t *testing.T) {
			f := branchHandoffFixture(t)
			held := f.lane
			if owner == "issue" {
				held = filepath.Join(filepath.Dir(f.lane), f.branch)
				laneRoutingGit(t, f.lane, "switch", "main")
				laneRoutingGit(t, f.lane, "worktree", "add", held, f.branch)
			}
			d, events := newLaneRoutingDaemon(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			result := awaitLaneRoutingTask(t, ctx, startLaneRoutingTask(ctx, d, f.task("handoff")))
			if got := laneRoutingGit(t, held, "branch", "--show-current"); got != "" {
				t.Fatalf("owner still holds branch %q", got)
			}
			if got := laneRoutingGit(t, held, "rev-parse", "HEAD"); got != f.head {
				t.Fatalf("detached owner commit = %s, want %s", got, f.head)
			}
			if got := laneRoutingGit(t, f.pr, "rev-parse", "HEAD"); got != f.head {
				t.Fatalf("PR commit = %s, want %s", got, f.head)
			}
			if got := laneRoutingGit(t, f.pr, "branch", "--show-current"); got != f.branch {
				t.Fatalf("PR branch = %s, want %s", got, f.branch)
			}
			assertLaneRoutingRecord(t, events, "handoff", f.pr, f.branch)
			result.release()
			for _, routing := range []string{"issue", "lane"} {
				next := f.task("later-" + routing)
				next.Repositories[0].WorktreePath = held
				if routing == "issue" {
					next.Repositories[0].Worklane = nil
				} else {
					next.Repositories[0].Worklane.PullRequestBranch = ""
				}
				nextResult := awaitLaneRoutingTask(t, ctx, startLaneRoutingTask(ctx, d, next))
				if got := primaryIssueWorktree(nextResult.task, "local"); got != f.pr {
					t.Fatalf("later %s directory = %s, want %s", routing, got, f.pr)
				}
				if got := d.localPathLocks.Holder(f.pr); got != next.ID {
					t.Fatalf("PR owner = %s, want %s", got, next.ID)
				}
				assertLaneRoutingRecord(t, events, next.ID, f.pr, f.branch)
				nextResult.release()
			}
		})
	}
}

func TestPullRequestHandoffRefusesWork(t *testing.T) {
	for _, tc := range []struct {
		name   string
		code   string
		change func(*testing.T, laneRoutingFixture)
	}{
		{name: "uncommitted", code: execenv.WorklaneCodeUncommitted, change: func(t *testing.T, f laneRoutingFixture) {
			laneRoutingWrite(t, f.lane, "shared.txt", "unsaved\n")
		}},
		{name: "untracked", code: execenv.WorklaneCodeUncommitted, change: func(t *testing.T, f laneRoutingFixture) {
			laneRoutingWrite(t, f.lane, "unsaved.txt", "unsaved\n")
		}},
		{name: "unpushed", code: execenv.WorklaneCodeUnpushed, change: func(t *testing.T, f laneRoutingFixture) {
			laneRoutingGit(t, f.lane, "commit", "--allow-empty", "-m", "unpushed")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := branchHandoffFixture(t)
			tc.change(t, f)
			head := laneRoutingGit(t, f.lane, "rev-parse", "HEAD")
			status := laneRoutingGit(t, f.lane, "status", "--porcelain")
			d, events := newLaneRoutingDaemon(t)
			task := f.task("refused")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			release, abort := d.acquireIssueWorktreeLocks(ctx, &task, d.logger)
			if abort {
				t.Fatal("lock acquisition aborted")
			}
			defer release()
			err := d.prepareIssueWorktrees(ctx, &task, d.logger)
			var held *execenv.WorklaneError
			if !errors.As(err, &held) || held.Code != tc.code {
				t.Fatalf("handoff error = %v, want %s", err, tc.code)
			}
			if got := laneRoutingGit(t, f.lane, "branch", "--show-current"); got != f.branch {
				t.Fatalf("owner branch changed to %q", got)
			}
			if got := laneRoutingGit(t, f.lane, "rev-parse", "HEAD"); got != head {
				t.Fatalf("owner HEAD changed to %s", got)
			}
			if got := laneRoutingGit(t, f.lane, "status", "--porcelain"); got != status {
				t.Fatalf("owner work changed to %s", got)
			}
			if _, err := os.Lstat(f.pr); !os.IsNotExist(err) {
				t.Fatalf("refused handoff created destination: %v", err)
			}
			select {
			case event := <-events:
				t.Fatalf("refused handoff reported success: %+v", event)
			default:
			}
		})
	}
}

func TestPullRequestHandoffWaitsForActiveOwner(t *testing.T) {
	f := branchHandoffFixture(t)
	d, events := newLaneRoutingDaemon(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	release, err := d.localPathLocks.Acquire(ctx, f.lane, "active-builder", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	done := startLaneRoutingTask(ctx, d, f.task("handoff"))
	select {
	case event := <-events:
		if event.path != "/api/daemon/tasks/handoff/wait-local-directory" {
			t.Fatalf("handoff did not wait for active owner: %+v", event)
		}
	case result := <-done:
		if result.release != nil {
			result.release()
		}
		t.Fatalf("handoff entered active owner's worktree: %v", result.err)
	case <-ctx.Done():
		t.Fatal("handoff did not report waiting")
	}
	if got := laneRoutingGit(t, f.lane, "branch", "--show-current"); got != f.branch {
		t.Fatalf("active owner detached to %q", got)
	}
	// The builder can finish and push while the handoff is queued.
	laneRoutingGit(t, f.lane, "commit", "--allow-empty", "-m", "builder finishes")
	laneRoutingGit(t, f.lane, "push", "origin", f.branch)
	head := laneRoutingGit(t, f.lane, "rev-parse", "HEAD")
	release()
	awaitLaneRoutingTask(t, ctx, done)
	if got := laneRoutingGit(t, f.pr, "rev-parse", "HEAD"); got != head {
		t.Fatalf("handoff lost completed builder commit: %s, want %s", got, head)
	}
	assertLaneRoutingRecord(t, events, "handoff", f.pr, f.branch)
}

func TestIssueRunQueuedBeforeHandoffUsesBranchOwner(t *testing.T) {
	f := branchHandoffFixture(t)
	d, events := newLaneRoutingDaemon(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pr := f.task("handoff")
	release, abort := d.acquireIssueWorktreeLocks(ctx, &pr, d.logger)
	if abort {
		t.Fatal("PR locks aborted")
	}
	defer release()
	next := f.task("queued-issue")
	next.Repositories[0].Worklane = nil
	next.Repositories[0].WorktreePath = f.lane
	done := startLaneRoutingTask(ctx, d, next)
	select {
	case event := <-events:
		if event.path != "/api/daemon/tasks/queued-issue/wait-local-directory" {
			t.Fatalf("issue run did not wait for source: %+v", event)
		}
	case <-ctx.Done():
		t.Fatal("issue run did not queue on source")
	}
	if err := d.prepareIssueWorktrees(ctx, &pr, d.logger); err != nil {
		t.Fatal(err)
	}
	assertLaneRoutingRecord(t, events, "handoff", f.pr, f.branch)
	release()
	result := awaitLaneRoutingTask(t, ctx, done)
	if got := primaryIssueWorktree(result.task, "local"); got != f.pr {
		t.Fatalf("queued issue used detached path %s", got)
	}
	if got := d.localPathLocks.Holder(f.pr); got != next.ID {
		t.Fatalf("queued issue did not lock branch owner: %s", got)
	}
	assertLaneRoutingRecord(t, events, next.ID, f.pr, f.branch)
}

func TestWorklaneRunPreservesConfiguredPathAlias(t *testing.T) {
	f := branchHandoffFixture(t)
	alias := filepath.Join(t.TempDir(), "lane alias")
	if err := os.Symlink(f.lane, alias); err != nil {
		t.Fatal(err)
	}
	task := f.task("alias-run")
	task.Repositories[0].Worklane.Path = alias
	task.Repositories[0].Worklane.PullRequestBranch = ""
	d, _ := newLaneRoutingDaemon(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result := awaitLaneRoutingTask(t, ctx, startLaneRoutingTask(ctx, d, task))
	if got := primaryIssueWorktree(result.task, "local"); got != alias {
		t.Fatalf("run path = %s, want configured alias %s", got, alias)
	}
	if got := d.localPathLocks.Holder(f.lane); got != task.ID {
		t.Fatalf("canonical lane lock holder = %s, want %s", got, task.ID)
	}
}
