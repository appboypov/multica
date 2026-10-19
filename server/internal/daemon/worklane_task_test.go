package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

func laneRoutingGit(t *testing.T, path string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", path}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

type laneRoutingFixture struct {
	lane   string
	pr     string
	branch string
	head   string
}

func newLaneRoutingFixture(t *testing.T, branch string) laneRoutingFixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "runtime home")
	lane := filepath.Join(home, "Worktrees", "org", "repo", "active lane")
	if err := os.MkdirAll(lane, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	// Keep the fixture independent of the developer's Git configuration.
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	origin := filepath.Join(root, "origin.git")
	laneRoutingGit(t, root, "init", "--bare", "-b", "main", origin)
	laneRoutingGit(t, lane, "init", "-b", "main")
	laneRoutingGit(t, lane, "config", "user.name", "Test")
	laneRoutingGit(t, lane, "config", "user.email", "test@example.com")
	laneRoutingWrite(t, lane, "shared.txt", "base\n")
	laneRoutingGit(t, lane, "add", "shared.txt")
	laneRoutingGit(t, lane, "commit", "-m", "initial")
	laneRoutingGit(t, lane, "remote", "add", "origin", origin)
	laneRoutingGit(t, lane, "push", "-u", "origin", "main")
	laneRoutingGit(t, lane, "switch", "-c", branch)
	laneRoutingWrite(t, lane, "pr.txt", "pushed PR content\n")
	laneRoutingGit(t, lane, "add", "pr.txt")
	laneRoutingGit(t, lane, "commit", "-m", "PR changes")
	head := laneRoutingGit(t, lane, "rev-parse", "HEAD")
	laneRoutingGit(t, lane, "push", "-u", "origin", branch)
	laneRoutingGit(t, lane, "switch", "-c", "work-1022", "main")
	laneRoutingWrite(t, lane, "builder.txt", "unpushed builder commit\n")
	laneRoutingGit(t, lane, "add", "builder.txt")
	laneRoutingGit(t, lane, "commit", "-m", "unpushed active work")
	laneRoutingWrite(t, lane, "shared.txt", "dirty builder change\n")
	laneRoutingWrite(t, lane, "unfinished.txt", "active work\n")
	return laneRoutingFixture{lane: lane, pr: filepath.Join(filepath.Dir(lane), ".pull-requests", filepath.FromSlash(branch)), branch: branch, head: head}
}

func laneRoutingWrite(t *testing.T, path, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(path, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func (f laneRoutingFixture) task(id string) Task {
	return Task{ID: id, IssueID: "issue", IssueKey: "work-1021", IssueIdentifier: "WORK-1021", OrganisationKey: "org", Repositories: []protocol.ClaimRepository{{
		ID: "repository", FullName: "test/repo", CheckoutPath: f.lane, CheckoutDaemonID: "local",
		Worklane: &protocol.ClaimWorklane{Path: f.lane, DaemonID: "local", Name: "work", DefaultBranch: "main", PullRequestBranch: f.branch},
	}}}
}

type laneRoutingEvent struct {
	path string
	body map[string]any
}

func newLaneRoutingDaemon(t *testing.T) (*Daemon, <-chan laneRoutingEvent) {
	t.Helper()
	events := make(chan laneRoutingEvent, 32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode %s: %v", r.URL.Path, err)
			}
			events <- laneRoutingEvent{path: r.URL.Path, body: body}
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"running"}`)
	}))
	t.Cleanup(server.Close)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &Daemon{cfg: Config{DaemonID: "local"}, localPathLocks: NewLocalPathLocker(), client: NewClient(server.URL), logger: logger}, events
}

type laneRoutingResult struct {
	task    Task
	release func()
	err     error
}

func startLaneRoutingTask(ctx context.Context, d *Daemon, task Task) <-chan laneRoutingResult {
	done := make(chan laneRoutingResult, 1)
	go func() {
		release, abort := d.acquireIssueWorktreeLocks(ctx, &task, d.logger)
		if abort {
			done <- laneRoutingResult{task: task, err: fmt.Errorf("task lock acquisition aborted")}
			return
		}
		err := d.prepareIssueWorktrees(ctx, &task, d.logger)
		done <- laneRoutingResult{task: task, release: release, err: err}
	}()
	return done
}

func awaitLaneRoutingTask(t *testing.T, ctx context.Context, done <-chan laneRoutingResult) laneRoutingResult {
	t.Helper()
	select {
	case result := <-done:
		if result.release != nil {
			t.Cleanup(result.release)
		}
		if result.err != nil {
			t.Fatal(result.err)
		}
		return result
	case <-ctx.Done():
		t.Fatal("PR task did not start while the lane lock remained held")
		return laneRoutingResult{}
	}
}

func assertLaneRoutingRecord(t *testing.T, events <-chan laneRoutingEvent, taskID, path, branch string) {
	t.Helper()
	select {
	case event := <-events:
		if event.path != "/api/daemon/tasks/"+taskID+"/issue-worktrees" || event.body["path"] != path || event.body["branch"] != branch || event.body["repository_id"] != "repository" {
			t.Fatalf("PR worktree report = %+v", event)
		}
	default:
		t.Fatal("PR worktree path was not recorded")
	}
	select {
	case event := <-events:
		t.Fatalf("unexpected lane report, wait or failure: %+v", event)
	default:
	}
}

func assertLaneRoutingBuilderUnchanged(t *testing.T, f laneRoutingFixture, head, status string) {
	t.Helper()
	if got := laneRoutingGit(t, f.lane, "branch", "--show-current"); got != "work-1022" {
		t.Fatalf("active lane branch = %s", got)
	}
	if got := laneRoutingGit(t, f.lane, "rev-parse", "HEAD"); got != head {
		t.Fatalf("active lane HEAD = %s, want %s", got, head)
	}
	if got := laneRoutingGit(t, f.lane, "status", "--porcelain"); got != status {
		t.Fatalf("active lane status changed: %s", got)
	}
	for name, want := range map[string]string{"builder.txt": "unpushed builder commit\n", "shared.txt": "dirty builder change\n", "unfinished.txt": "active work\n"} {
		if got, err := os.ReadFile(filepath.Join(f.lane, name)); err != nil || string(got) != want {
			t.Fatalf("active file %s = %q, %v", name, got, err)
		}
	}
}

func TestWorklanePullRequestRunPreservesActiveIssue(t *testing.T) {
	for _, present := range []bool{true, false} {
		name := "missing"
		if present {
			name = "present"
		}
		t.Run(name, func(t *testing.T) {
			f := newLaneRoutingFixture(t, "review/work-1021")
			if present {
				laneRoutingGit(t, f.lane, "worktree", "add", f.pr, f.branch)
			} else {
				// A local, unpushed PR commit must win over origin's older tip,
				// and creation from local history must not need a fetch.
				f.head = laneRoutingGit(t, f.lane, "commit-tree", f.head+"^{tree}", "-p", f.head, "-m", "unpushed PR fix")
				laneRoutingGit(t, f.lane, "update-ref", "refs/heads/"+f.branch, f.head)
				laneRoutingGit(t, f.lane, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
			}
			head := laneRoutingGit(t, f.lane, "rev-parse", "HEAD")
			status := laneRoutingGit(t, f.lane, "status", "--porcelain")
			d, events := newLaneRoutingDaemon(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			activeRelease, err := d.localPathLocks.Acquire(ctx, f.lane, "active-builder", nil)
			if err != nil {
				t.Fatal(err)
			}
			defer activeRelease()
			result := awaitLaneRoutingTask(t, ctx, startLaneRoutingTask(ctx, d, f.task("verifier")))
			if got := primaryIssueWorktree(result.task, "local"); got != f.pr {
				t.Fatalf("verifier directory = %q, want %q", got, f.pr)
			}
			if got := d.localPathLocks.Holder(f.lane); got != "active-builder" {
				t.Fatalf("lane holder = %q", got)
			}
			if got := d.localPathLocks.Holder(f.pr); got != "verifier" {
				t.Fatalf("PR holder = %q", got)
			}
			if got := laneRoutingGit(t, f.pr, "branch", "--show-current"); got != f.branch {
				t.Fatalf("PR branch = %s", got)
			}
			if got := laneRoutingGit(t, f.pr, "rev-parse", "HEAD"); got != f.head {
				t.Fatalf("PR history = %s, want %s", got, f.head)
			}
			if got, err := os.ReadFile(filepath.Join(f.pr, "pr.txt")); err != nil || string(got) != "pushed PR content\n" {
				t.Fatalf("PR contents = %q, %v", got, err)
			}
			assertLaneRoutingBuilderUnchanged(t, f, head, status)
			assertLaneRoutingRecord(t, events, "verifier", f.pr, f.branch)
		})
	}
}

func TestWorklaneRegisteredPullRequestWithoutClaimContext(t *testing.T) {
	f := newLaneRoutingFixture(t, "work-1021")
	laneRoutingGit(t, f.lane, "worktree", "add", f.pr, f.branch)
	d, events := newLaneRoutingDaemon(t)
	task := f.task("registered-pr")
	task.Repositories[0].Worklane.PullRequestBranch = ""
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result := awaitLaneRoutingTask(t, ctx, startLaneRoutingTask(ctx, d, task))
	if got := primaryIssueWorktree(result.task, "local"); got != f.pr {
		t.Fatalf("registered PR directory = %s", got)
	}
	assertLaneRoutingRecord(t, events, task.ID, f.pr, f.branch)
}

func TestWorklanePullRequestSamePathSerializesRuns(t *testing.T) {
	for _, present := range []bool{true, false} {
		name := "missing"
		if present {
			name = "present"
		}
		t.Run(name, func(t *testing.T) {
			f := newLaneRoutingFixture(t, "review/work-1021")
			if present {
				laneRoutingGit(t, f.lane, "worktree", "add", f.pr, f.branch)
			}
			// The first claim uses an alias, including before the PR tree exists.
			alias := filepath.Join(t.TempDir(), "repo alias")
			if err := os.Symlink(filepath.Dir(f.lane), alias); err != nil {
				t.Fatal(err)
			}
			first := f.task("first")
			first.Repositories[0].WorktreePath = filepath.Join(alias, ".pull-requests", filepath.FromSlash(f.branch))
			d, events := newLaneRoutingDaemon(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			release, abort := d.acquireIssueWorktreeLocks(ctx, &first, d.logger)
			if abort {
				t.Fatal("first PR lock aborted")
			}
			defer release()
			second := startLaneRoutingTask(ctx, d, f.task("second"))
			select {
			case event := <-events:
				if event.path != "/api/daemon/tasks/second/wait-local-directory" || !strings.Contains(fmt.Sprint(event.body["reason"]), "work-1021") {
					t.Fatalf("second run waited for the wrong path: %+v", event)
				}
			case <-ctx.Done():
				t.Fatal("second PR task did not contend on the first task's path")
			}
			select {
			case result := <-second:
				if result.release != nil {
					result.release()
				}
				t.Fatalf("second run entered preparation before PR unlock: %v", result.err)
			default:
			}
			if !present {
				if _, err := os.Lstat(f.pr); !os.IsNotExist(err) {
					t.Fatalf("waiting run created the missing PR tree: %v", err)
				}
			}
			if err := d.prepareIssueWorktrees(ctx, &first, d.logger); err != nil {
				t.Fatal(err)
			}
			assertLaneRoutingRecord(t, events, "first", f.pr, f.branch)
			if got := d.localPathLocks.Holder(f.pr); got != "first" {
				t.Fatalf("PR lock holder before release = %q", got)
			}
			release()
			result := awaitLaneRoutingTask(t, ctx, second)
			if got := primaryIssueWorktree(result.task, "local"); got != f.pr {
				t.Fatalf("second PR directory = %s", got)
			}
			assertLaneRoutingRecord(t, events, "second", f.pr, f.branch)
		})
	}
}

func TestWorklanePullRequestRemoteOnlyBranchContents(t *testing.T) {
	f := newLaneRoutingFixture(t, "review/work-1021")
	laneRoutingGit(t, f.lane, "branch", "-D", f.branch)
	laneRoutingGit(t, f.lane, "update-ref", "-d", "refs/remotes/origin/"+f.branch)
	// The explicit PR refspec must work even with a main-only clone.
	laneRoutingGit(t, f.lane, "config", "remote.origin.fetch", "+refs/heads/main:refs/remotes/origin/main")
	d, events := newLaneRoutingDaemon(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result := awaitLaneRoutingTask(t, ctx, startLaneRoutingTask(ctx, d, f.task("remote-pr")))
	if got := primaryIssueWorktree(result.task, "local"); got != f.pr {
		t.Fatalf("remote PR directory = %s", got)
	}
	if got := laneRoutingGit(t, f.pr, "rev-parse", "HEAD"); got != f.head {
		t.Fatalf("origin PR history = %s, want %s", got, f.head)
	}
	if got, err := os.ReadFile(filepath.Join(f.pr, "pr.txt")); err != nil || string(got) != "pushed PR content\n" {
		t.Fatalf("origin PR contents = %q, %v", got, err)
	}
	if got := laneRoutingGit(t, f.lane, "branch", "--show-current"); got != "work-1022" {
		t.Fatalf("active lane branch = %s", got)
	}
	assertLaneRoutingRecord(t, events, "remote-pr", f.pr, f.branch)
}

func TestWorklanePullRequestReusesReviewFixes(t *testing.T) {
	f := newLaneRoutingFixture(t, "review/work-1021")
	// A recorded PR path wins even if it is outside the default repository folder.
	recorded, err := resolveRealPath(filepath.Join(t.TempDir(), "review archive", ".pull-requests", filepath.FromSlash(f.branch)))
	if err != nil {
		t.Fatal(err)
	}
	laneRoutingGit(t, f.lane, "worktree", "add", recorded, f.branch)
	laneRoutingWrite(t, recorded, "review.txt", "committed review fix\n")
	laneRoutingGit(t, recorded, "add", "review.txt")
	laneRoutingGit(t, recorded, "commit", "-m", "review fix")
	laneRoutingWrite(t, recorded, "pr.txt", "uncommitted review fix\n")
	laneRoutingWrite(t, recorded, "scratch.txt", "unsaved review notes\n")
	head := laneRoutingGit(t, recorded, "rev-parse", "HEAD")
	status := laneRoutingGit(t, recorded, "status", "--porcelain")
	// Reuse must not fetch or depend on origin still being available.
	laneRoutingGit(t, f.lane, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
	d, events := newLaneRoutingDaemon(t)
	task := f.task("review-fix")
	task.Repositories[0].WorktreePath = recorded
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result := awaitLaneRoutingTask(t, ctx, startLaneRoutingTask(ctx, d, task))
	if got := primaryIssueWorktree(result.task, "local"); got != recorded {
		t.Fatalf("recorded PR directory = %s", got)
	}
	if got := laneRoutingGit(t, recorded, "rev-parse", "HEAD"); got != head {
		t.Fatalf("review fix HEAD changed: %s", got)
	}
	if got := laneRoutingGit(t, recorded, "status", "--porcelain"); got != status {
		t.Fatalf("review fix status changed: %s", got)
	}
	for name, want := range map[string]string{"review.txt": "committed review fix\n", "pr.txt": "uncommitted review fix\n", "scratch.txt": "unsaved review notes\n"} {
		if got, err := os.ReadFile(filepath.Join(recorded, name)); err != nil || string(got) != want {
			t.Fatalf("review file %s = %q, %v", name, got, err)
		}
	}
	assertLaneRoutingRecord(t, events, task.ID, recorded, f.branch)
}

func TestWorklanePullRequestRejectsUnsafeBranchesBeforeLocking(t *testing.T) {
	f := newLaneRoutingFixture(t, "review/work-1021")
	for _, branch := range []string{"../escape", "/absolute", "feature/../../escape", "-option", "@{-1}", "with space", `feature\escape`, "feature/.hidden"} {
		t.Run(branch, func(t *testing.T) {
			task := f.task("unsafe-pr")
			task.Repositories[0].Worklane.PullRequestBranch = branch
			d, events := newLaneRoutingDaemon(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			release, abort := d.acquireIssueWorktreeLocks(ctx, &task, d.logger)
			if release != nil {
				release()
			}
			if !abort {
				t.Fatal("unsafe PR branch was accepted")
			}
			if got := d.localPathLocks.Holder(f.lane); got != "" {
				t.Fatalf("unsafe branch acquired lane lock: %s", got)
			}
			select {
			case event := <-events:
				if !strings.HasSuffix(event.path, "/fail") || event.body["failure_reason"] != "environment_prepare_failed" {
					t.Fatalf("unsafe branch failure = %+v", event)
				}
			default:
				t.Fatal("unsafe branch was not reported as an environment preparation failure")
			}
			if got := task.Repositories[0].WorktreePath; got != "" {
				t.Fatalf("unsafe branch selected a destination: %s", got)
			}
		})
	}
}

func TestWorklanePullRequestRejectsEscapingParentSymlink(t *testing.T) {
	f := newLaneRoutingFixture(t, "review/work-1021")
	root := filepath.Dir(filepath.Dir(f.pr))
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "review")); err != nil {
		t.Fatal(err)
	}
	task := f.task("escaping-pr")
	if selected, err := worklaneTaskPath(context.Background(), task, task.Repositories[0]); err == nil {
		t.Fatalf("escaping PR destination was accepted: %s", selected)
	}
}

func TestWorklanePullRequestDestinationWithoutOrganisation(t *testing.T) {
	f := newLaneRoutingFixture(t, "review/work-1021")
	task := f.task("no-org")
	task.OrganisationKey = ""
	want := filepath.Join(os.Getenv("HOME"), "Worktrees", "repo", ".pull-requests", filepath.FromSlash(f.branch))
	if selected, err := worklaneTaskPath(context.Background(), task, task.Repositories[0]); err != nil || selected != want {
		t.Fatalf("organisation-free PR destination = %q, %v, want %q", selected, err, want)
	}
}

func TestWorklanePullRequestPreparationFailuresNeverUseLane(t *testing.T) {
	for _, name := range []string{"occupied_file", "foreign_repository", "wrong_branch", "detached_tree", "failed_fetch_with_stale_ref"} {
		t.Run(name, func(t *testing.T) {
			f := newLaneRoutingFixture(t, "review/work-1021")
			task := f.task("failed-pr")
			switch name {
			case "occupied_file":
				if err := os.MkdirAll(filepath.Dir(f.pr), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(f.pr, []byte("occupied\n"), 0644); err != nil {
					t.Fatal(err)
				}
			case "foreign_repository":
				if err := os.MkdirAll(f.pr, 0755); err != nil {
					t.Fatal(err)
				}
				laneRoutingGit(t, f.pr, "init", "-b", f.branch)
			case "wrong_branch":
				laneRoutingGit(t, f.lane, "worktree", "add", "-b", "other-review", f.pr, "main")
			case "detached_tree":
				laneRoutingGit(t, f.lane, "worktree", "add", "--detach", f.pr, f.head)
			case "failed_fetch_with_stale_ref":
				laneRoutingGit(t, f.lane, "branch", "-D", f.branch)
				laneRoutingGit(t, f.lane, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
			}
			task.Repositories[0].WorktreePath = f.pr
			head := laneRoutingGit(t, f.lane, "rev-parse", "HEAD")
			status := laneRoutingGit(t, f.lane, "status", "--porcelain")
			d, events := newLaneRoutingDaemon(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			activeRelease, err := d.localPathLocks.Acquire(ctx, f.lane, "active-builder", nil)
			if err != nil {
				t.Fatal(err)
			}
			defer activeRelease()
			done := startLaneRoutingTask(ctx, d, task)
			select {
			case result := <-done:
				if result.release != nil {
					defer result.release()
				}
				if result.err == nil {
					t.Fatal("invalid PR worktree preparation succeeded")
				}
				if got := primaryIssueWorktree(result.task, "local"); got != f.pr {
					t.Fatalf("failed PR preparation fell back to %s", got)
				}
			case <-ctx.Done():
				t.Fatal("invalid PR run waited for the lane")
			}
			assertLaneRoutingBuilderUnchanged(t, f, head, status)
			if got := d.localPathLocks.Holder(f.lane); got != "active-builder" {
				t.Fatalf("builder lane lock changed: %q", got)
			}
			select {
			case event := <-events:
				t.Fatalf("failed preparation reported or waited on a lane: %+v", event)
			default:
			}
			if name == "occupied_file" {
				if got, err := os.ReadFile(f.pr); err != nil || string(got) != "occupied\n" {
					t.Fatalf("occupied path changed: %q, %v", got, err)
				}
			}
			if name == "failed_fetch_with_stale_ref" {
				if _, err := os.Lstat(f.pr); !os.IsNotExist(err) {
					t.Fatalf("refused PR destination was created: %v", err)
				}
			}
		})
	}
}

func TestWorklanePullRequestUsesLocalLaneWhenCheckoutIsElsewhere(t *testing.T) {
	f := newLaneRoutingFixture(t, "review/work-1021")
	task := f.task("local-pr")
	task.Repositories[0].CheckoutDaemonID = "other-machine"
	task.Repositories[0].CheckoutPath = filepath.Join(t.TempDir(), "not-on-this-machine")
	d, events := newLaneRoutingDaemon(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result := awaitLaneRoutingTask(t, ctx, startLaneRoutingTask(ctx, d, task))
	if got := primaryIssueWorktree(result.task, "local"); got != f.pr {
		t.Fatalf("local lane's PR directory = %s", got)
	}
	assertLaneRoutingRecord(t, events, task.ID, f.pr, f.branch)
}

func TestWorklanePullRequestReusesTreeOfCheckoutOrStandaloneLane(t *testing.T) {
	for _, owner := range []string{"checkout", "lane"} {
		for _, location := range []string{"default", "registered"} {
			t.Run(owner+"_"+location, func(t *testing.T) {
				f := newLaneRoutingFixture(t, "review/work-1021")
				// The lane is a standalone clone; the checkout is another clone
				// of the same repository with its own Git directory.
				checkout := filepath.Join(t.TempDir(), "checkout")
				laneRoutingGit(t, filepath.Dir(checkout), "clone", laneRoutingGit(t, f.lane, "remote", "get-url", "origin"), checkout)
				tree := f.pr
				if location == "registered" {
					var err error
					if tree, err = resolveRealPath(filepath.Join(t.TempDir(), "elsewhere", ".pull-requests", filepath.FromSlash(f.branch))); err != nil {
						t.Fatal(err)
					}
				}
				source := map[string]string{"checkout": checkout, "lane": f.lane}[owner]
				laneRoutingGit(t, source, "worktree", "add", tree, f.branch)
				laneRoutingGit(t, tree, "config", "user.name", "Test")
				laneRoutingGit(t, tree, "config", "user.email", "test@example.com")
				laneRoutingWrite(t, tree, "review.txt", "committed review fix\n")
				laneRoutingGit(t, tree, "add", "review.txt")
				laneRoutingGit(t, tree, "commit", "-m", "review fix")
				laneRoutingWrite(t, tree, "scratch.txt", "unsaved review notes\n")
				head := laneRoutingGit(t, tree, "rev-parse", "HEAD")
				status := laneRoutingGit(t, tree, "status", "--porcelain")
				laneHead := laneRoutingGit(t, f.lane, "rev-parse", "HEAD")
				laneStatus := laneRoutingGit(t, f.lane, "status", "--porcelain")
				// Reuse must not fetch, so neither clone can reach origin.
				for _, clone := range []string{checkout, f.lane} {
					laneRoutingGit(t, clone, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
				}
				d, events := newLaneRoutingDaemon(t)
				task := f.task("standalone-pr")
				task.Repositories[0].CheckoutPath = checkout
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				result := awaitLaneRoutingTask(t, ctx, startLaneRoutingTask(ctx, d, task))
				if got := primaryIssueWorktree(result.task, "local"); got != tree {
					t.Fatalf("PR directory = %s, want %s", got, tree)
				}
				if got := laneRoutingGit(t, tree, "rev-parse", "HEAD"); got != head {
					t.Fatalf("review fix HEAD changed: %s", got)
				}
				if got := laneRoutingGit(t, tree, "status", "--porcelain"); got != status {
					t.Fatalf("review fix status changed: %s", got)
				}
				assertLaneRoutingRecord(t, events, task.ID, tree, f.branch)
				assertLaneRoutingBuilderUnchanged(t, f, laneHead, laneStatus)
			})
		}
	}
}

func TestWorklanePullRequestRecordFailureNeverReportsLane(t *testing.T) {
	f := newLaneRoutingFixture(t, "review/work-1021")
	events := make(chan laneRoutingEvent, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode worktree record: %v", err)
		}
		events <- laneRoutingEvent{path: r.URL.Path, body: body}
		http.Error(w, "worktree record refused", http.StatusBadRequest)
	}))
	defer server.Close()
	d := &Daemon{cfg: Config{DaemonID: "local"}, localPathLocks: NewLocalPathLocker(), client: NewClient(server.URL), logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	task := f.task("record-failed")
	release, abort := d.acquireIssueWorktreeLocks(ctx, &task, d.logger)
	if abort {
		t.Fatal("PR path lock failed")
	}
	defer release()
	if err := d.prepareIssueWorktrees(ctx, &task, d.logger); err == nil {
		t.Fatal("failed record was ignored")
	}
	if got := task.Repositories[0].WorktreePath; got != f.pr {
		t.Fatalf("record failure fell back to %s", got)
	}
	assertLaneRoutingRecord(t, events, task.ID, f.pr, f.branch)
}

func TestWorklaneRunWithoutPullRequestUsesLane(t *testing.T) {
	lane := t.TempDir()
	laneRoutingGit(t, lane, "init", "-b", "work-1022")
	laneRoutingGit(t, lane, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "initial")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	d := &Daemon{cfg: Config{DaemonID: "local"}, localPathLocks: NewLocalPathLocker(), client: NewClient(server.URL)}
	task := Task{ID: "task", IssueID: "issue", IssueIdentifier: "WORK-1022", Repositories: []protocol.ClaimRepository{{FullName: "test/repo", Worklane: &protocol.ClaimWorklane{Path: lane, DaemonID: "local", DefaultBranch: "main"}}}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	release, abort := d.acquireIssueWorktreeLocks(context.Background(), &task, logger)
	if abort {
		t.Fatal("lane preparation aborted")
	}
	defer release()
	if err := d.prepareIssueWorktrees(context.Background(), &task, logger); err != nil {
		t.Fatal(err)
	}
	if got := primaryIssueWorktree(task, "local"); got != lane {
		t.Fatalf("run directory = %q, want %q", got, lane)
	}
}

// newCheckoutRoutingFixture is a clean checkout on main of a repository that
// works in its checkout (ADR-0031), with the PR branch review/work-1021 only
// on origin. HOME holds no Worktrees folder.
func newCheckoutRoutingFixture(t *testing.T) (checkout, home string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home = filepath.Join(root, "home")
	checkout = filepath.Join(home, "Repos", "repo")
	if err := os.MkdirAll(checkout, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	origin := filepath.Join(root, "origin.git")
	laneRoutingGit(t, root, "init", "--bare", "-b", "main", origin)
	laneRoutingGit(t, checkout, "init", "-b", "main")
	laneRoutingGit(t, checkout, "config", "user.name", "Test")
	laneRoutingGit(t, checkout, "config", "user.email", "test@example.com")
	laneRoutingWrite(t, checkout, "shared.txt", "base\n")
	laneRoutingGit(t, checkout, "add", "shared.txt")
	laneRoutingGit(t, checkout, "commit", "-m", "initial")
	laneRoutingGit(t, checkout, "remote", "add", "origin", origin)
	laneRoutingGit(t, checkout, "push", "-u", "origin", "main")
	laneRoutingGit(t, checkout, "switch", "-c", "review/work-1021")
	laneRoutingWrite(t, checkout, "pr.txt", "pushed PR content\n")
	laneRoutingGit(t, checkout, "add", "pr.txt")
	laneRoutingGit(t, checkout, "commit", "-m", "PR changes")
	laneRoutingGit(t, checkout, "push", "origin", "review/work-1021")
	laneRoutingGit(t, checkout, "switch", "main")
	laneRoutingGit(t, checkout, "branch", "-D", "review/work-1021")
	laneRoutingGit(t, checkout, "update-ref", "-d", "refs/remotes/origin/review/work-1021")
	return checkout, home
}

func checkoutRoutingTask(checkout, id, pullRequestBranch string) Task {
	return Task{ID: id, IssueID: "issue", IssueKey: "work-1021", IssueIdentifier: "WORK-1021", OrganisationKey: "org", Repositories: []protocol.ClaimRepository{{
		ID: "repository", FullName: "test/repo", CheckoutPath: checkout, CheckoutDaemonID: "local",
		Worklane: &protocol.ClaimWorklane{ID: "main-lane", Name: "main", Path: checkout, DaemonID: "local", DefaultBranch: "main", PullRequestBranch: pullRequestBranch, InCheckout: true},
	}}}
}

func assertCheckoutRoutingReport(t *testing.T, events <-chan laneRoutingEvent, taskID, branch string) {
	t.Helper()
	for {
		select {
		case event := <-events:
			if event.path != "/api/daemon/tasks/"+taskID+"/worklane-branch" {
				continue
			}
			if event.body["worklane_id"] != "main-lane" || event.body["branch"] != branch {
				t.Fatalf("branch report = %+v, want main-lane on %s", event.body, branch)
			}
			return
		default:
			t.Fatalf("task %s reported no branch for main", taskID)
		}
	}
}

func TestWorkInCheckoutRunsInTheCheckout(t *testing.T) {
	for _, tc := range []struct {
		name, pullRequestBranch, branch, file string
	}{
		{name: "issue branch", branch: "work-1021", file: "shared.txt"},
		{name: "pull-request branch only on origin", pullRequestBranch: "review/work-1021", branch: "review/work-1021", file: "pr.txt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checkout, home := newCheckoutRoutingFixture(t)
			d, events := newLaneRoutingDaemon(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			result := awaitLaneRoutingTask(t, ctx, startLaneRoutingTask(ctx, d, checkoutRoutingTask(checkout, "task", tc.pullRequestBranch)))
			if got := primaryIssueWorktree(result.task, "local"); got != checkout {
				t.Fatalf("run directory = %q, want the checkout %q", got, checkout)
			}
			if got := laneRoutingGit(t, checkout, "branch", "--show-current"); got != tc.branch {
				t.Fatalf("checkout branch = %q, want %q", got, tc.branch)
			}
			if _, err := os.Stat(filepath.Join(checkout, tc.file)); err != nil {
				t.Fatalf("checkout lacks %s: %v", tc.file, err)
			}
			if _, err := os.Stat(filepath.Join(home, "Worktrees")); !os.IsNotExist(err) {
				t.Fatalf("a worktree folder appeared: %v", err)
			}
			assertCheckoutRoutingReport(t, events, "task", tc.branch)
		})
	}
}

func TestWorkInCheckoutSerializesTasks(t *testing.T) {
	checkout, _ := newCheckoutRoutingFixture(t)
	d, events := newLaneRoutingDaemon(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	first := checkoutRoutingTask(checkout, "first", "")
	release, abort := d.acquireIssueWorktreeLocks(ctx, &first, d.logger)
	if abort {
		t.Fatal("first lock aborted")
	}
	second := startLaneRoutingTask(ctx, d, checkoutRoutingTask(checkout, "second", ""))
	select {
	case event := <-events:
		if event.path != "/api/daemon/tasks/second/wait-local-directory" {
			t.Fatalf("second task did not wait for the checkout: %+v", event)
		}
	case <-ctx.Done():
		t.Fatal("second task did not contend on the checkout")
	}
	select {
	case result := <-second:
		if result.release != nil {
			result.release()
		}
		t.Fatalf("second task started while the first held the checkout: %v", result.err)
	default:
	}
	if err := d.prepareIssueWorktrees(ctx, &first, d.logger); err != nil {
		t.Fatal(err)
	}
	release()
	awaitLaneRoutingTask(t, ctx, second)
}

func TestWorkInCheckoutRefusesToLeaveWork(t *testing.T) {
	for _, tc := range []struct {
		name  string
		leave func(t *testing.T, checkout string)
		want  string
	}{
		{name: "uncommitted change", want: "main holds uncommitted changes on main", leave: func(t *testing.T, checkout string) {
			laneRoutingWrite(t, checkout, "shared.txt", "dirty\n")
		}},
		{name: "unpushed commits", want: "main holds 2 commits on main that are not pushed", leave: func(t *testing.T, checkout string) {
			laneRoutingGit(t, checkout, "commit", "--allow-empty", "-m", "one")
			laneRoutingGit(t, checkout, "commit", "--allow-empty", "-m", "two")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checkout, _ := newCheckoutRoutingFixture(t)
			tc.leave(t, checkout)
			d, _ := newLaneRoutingDaemon(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			task := checkoutRoutingTask(checkout, "task", "")
			release, abort := d.acquireIssueWorktreeLocks(ctx, &task, d.logger)
			if abort {
				t.Fatal("lock aborted")
			}
			defer release()
			err := d.prepareIssueWorktrees(ctx, &task, d.logger)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("prepare err = %v, want %q", err, tc.want)
			}
			if got := laneRoutingGit(t, checkout, "branch", "--show-current"); got != "main" {
				t.Fatalf("checkout switched to %q", got)
			}
		})
	}
}
