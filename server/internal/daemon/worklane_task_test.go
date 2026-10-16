package daemon

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

func laneRoutingGit(t *testing.T, path string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", path}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestWorklanePullRequestRunPreservesActiveIssue(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	lane := filepath.Join(root, "active lane")
	if err := os.Mkdir(lane, 0755); err != nil {
		t.Fatal(err)
	}
	laneRoutingGit(t, lane, "init", "-b", "main")
	laneRoutingGit(t, lane, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "initial")
	pr := filepath.Join(root, ".pull-requests", "work-1021")
	laneRoutingGit(t, lane, "worktree", "add", "-b", "work-1021", pr)
	laneRoutingGit(t, lane, "switch", "-c", "work-1022")
	laneRoutingGit(t, lane, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-m", "unpushed active work")
	if err := os.WriteFile(filepath.Join(lane, "unfinished.txt"), []byte("active work"), 0644); err != nil {
		t.Fatal(err)
	}
	head := laneRoutingGit(t, lane, "rev-parse", "HEAD")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	d := &Daemon{cfg: Config{DaemonID: "local"}, localPathLocks: NewLocalPathLocker()}
	lanePath, err := resolveRealPath(lane)
	if err != nil {
		t.Fatal(err)
	}
	activeRelease, err := d.localPathLocks.Acquire(context.Background(), lanePath, "active-task", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer activeRelease()

	task := Task{ID: "pr-task", IssueID: "issue", IssueIdentifier: "WORK-1021", Repositories: []protocol.ClaimRepository{{FullName: "test/repo", Worklane: &protocol.ClaimWorklane{Path: lane, DaemonID: "local", Name: "work", DefaultBranch: "main"}}}}
	if selected, err := worklaneTaskPath(context.Background(), task, task.Repositories[0]); err != nil || selected != pr {
		t.Fatalf("selected worktree = %q, %v", selected, err)
	}
	release, abort := d.acquireIssueWorktreeLocks(context.Background(), &task, logger)
	if abort {
		t.Fatal("PR run waited for the active lane")
	}
	defer release()
	if err := d.prepareIssueWorktrees(context.Background(), &task, logger); err != nil {
		t.Fatal(err)
	}
	if got := primaryIssueWorktree(task, "local"); got != pr {
		t.Fatalf("run directory = %q, want %q", got, pr)
	}
	if got := laneRoutingGit(t, lane, "branch", "--show-current"); got != "work-1022" {
		t.Fatalf("active lane branch = %s", got)
	}
	if got := laneRoutingGit(t, lane, "rev-parse", "HEAD"); got != head {
		t.Fatalf("active lane head changed: %s", got)
	}
	if got, err := os.ReadFile(filepath.Join(lane, "unfinished.txt")); err != nil || string(got) != "active work" {
		t.Fatalf("active work changed: %q, %v", got, err)
	}
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
