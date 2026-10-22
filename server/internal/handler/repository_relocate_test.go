package handler

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// A relocate moves the repository's checkout and rewrites the recorded paths
// of its worklanes and issue worktrees, kept ones included, with the check's
// prefixes (ADR-0037). It refuses while a task may work in the repository.
func TestRelocateRepository(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	const home = "/home/fixture"
	orgID, orgKey := slugTestOrganisation(t, "Relocate org")
	project := dbfx.Project(t, "Relocate project", testutil.Cols{"organisation_id": orgID, "slug": "relocate"})
	daemon := "relocate-daemon-" + uuid.NewString()
	runtime := dbfx.Runtime(t, "Relocate machine", testutil.Cols{"daemon_id": daemon, "runtime_mode": "local", "owner_id": testUserID})

	repoName := "r" + uuid.NewString()[:8]
	fullName := "test/" + repoName
	oldCheckout := "~/Repos/" + repoName
	oldWorktrees := "~/Worktrees/" + repoName
	projectDir := "~/hq/" + orgKey + "/relocate"
	newCheckout := home + "/hq/" + orgKey + "/relocate/repos/" + repoName
	repository := func(fullName string) string {
		return dbfx.Insert(t, "repository", testutil.Cols{
			"workspace_id": testWorkspaceID, "project_id": project, "github_repo_id": 1,
			"full_name": fullName, "private": true, "default_branch": "main",
			"checkout_daemon_id": daemon, "checkout_path": oldCheckout, "version_policy": "none",
		})
	}
	repo := repository(fullName)
	otherRepo := repository("test/r" + uuid.NewString())

	lane := func(repositoryID, name, path string, main bool, position int) string {
		return dbfx.Insert(t, "worklane", testutil.Cols{
			"workspace_id": testWorkspaceID, "repository_id": repositoryID, "name": name,
			"folder": name, "daemon_id": daemon, "is_main": main, "position": position, "path": path,
		})
	}
	issueWorktree := func(repositoryID, path, state string) string {
		cols := testutil.Cols{
			"workspace_id": testWorkspaceID, "issue_id": dbfx.Issue(t, "Relocate issue", testutil.Cols{"project_id": project}),
			"repository_id": repositoryID, "daemon_id": daemon, "path": path, "branch": "work", "state": state,
		}
		if state == "kept" {
			cols["kept_reason"] = "uncommitted changes"
		}
		return dbfx.Insert(t, "issue_worktree", cols)
	}

	// Each row's recorded path and where it must land.
	lanes := map[string][2]string{
		lane(repo, "main", oldCheckout, true, 0):                                  {oldCheckout, "~/hq/" + orgKey + "/relocate/repos/" + repoName},
		lane(repo, "feature", oldWorktrees+"/feature", false, 1):                  {oldWorktrees + "/feature", projectDir + "/worktrees/" + repoName + "/feature"},
		lane(repo, "absolute", home+"/Worktrees/"+repoName+"/absolute", false, 2): {home + "/Worktrees/" + repoName + "/absolute", home + "/hq/" + orgKey + "/relocate/worktrees/" + repoName + "/absolute"},
		lane(repo, "outside", "/srv/elsewhere/outside", false, 3):                 {"/srv/elsewhere/outside", "/srv/elsewhere/outside"},
		lane(otherRepo, "intruder", oldWorktrees+"/intruder", false, 0):           {oldWorktrees + "/intruder", oldWorktrees + "/intruder"},
		lane(otherRepo, "other-main", oldCheckout, true, 1):                       {oldCheckout, oldCheckout},
	}
	keptLanding := projectDir + "/worktrees/" + repoName + "/work-2"
	trees := map[string][2]string{
		issueWorktree(repo, oldWorktrees+"/work-1", "active"):      {oldWorktrees + "/work-1", projectDir + "/worktrees/" + repoName + "/work-1"},
		issueWorktree(repo, oldWorktrees+"/work-2", "kept"):        {oldWorktrees + "/work-2", keptLanding},
		issueWorktree(otherRepo, oldWorktrees+"/work-3", "active"): {oldWorktrees + "/work-3", oldWorktrees + "/work-3"},
	}
	prefixes := []protocol.WorktreePrefix{{From: oldWorktrees + "/", To: projectDir + "/worktrees/" + repoName}}

	// The relocate check, as the CLI starts it: the server attaches the
	// recorded paths of this repository's rows only.
	var started RepositoryCheckoutResponse
	testutil.Call(t, testHandler.InitiateRepositoryCheckout, withURLParam(newRequest(http.MethodPost, "/api/runtimes/"+runtime+"/repository-checkouts", map[string]any{
		"full_name": fullName, "default_branch": "main", "path": newCheckout,
		"repository_id": repo, "worktree_prefixes": prefixes,
	}), "runtimeId", runtime)).Want(http.StatusOK).JSON(&started)
	pending, err := testHandler.RepositoryCheckoutStore.PopPending(ctx, runtime)
	if err != nil || pending == nil || pending.ID != started.ID || pending.Relocation == nil {
		t.Fatalf("pending relocate check = %+v, error = %v", pending, err)
	}
	if pending.Relocation.OldCheckout != oldCheckout || len(pending.Relocation.Paths) != 6 ||
		slices.Contains(pending.Relocation.Paths, oldWorktrees+"/intruder") || slices.Contains(pending.Relocation.Paths, oldWorktrees+"/work-3") {
		t.Fatalf("relocation probe = %+v, want the six paths of this repository", pending.Relocation)
	}
	if err := testHandler.RepositoryCheckoutStore.Complete(ctx, started.ID, protocol.RepositoryCheckoutResult{
		Status: "completed", ResolvedPath: newCheckout, Passed: true, Home: home, MissingPaths: []string{keptLanding},
	}); err != nil {
		t.Fatal(err)
	}

	relocate := func(want int) *testutil.Response {
		return testutil.Call(t, testHandler.RelocateRepository, withURLParam(newRequest(http.MethodPost, "/api/repositories/"+repo+"/relocate", map[string]any{
			"checkout_check_id": started.ID, "worktree_prefixes": prefixes,
		}), "id", repo)).Want(want)
	}
	assertPaths := func(t *testing.T, landed bool) {
		t.Helper()
		pick := func(paths [2]string) string {
			if landed {
				return paths[1]
			}
			return paths[0]
		}
		for id, paths := range lanes {
			var got string
			dbfx.QueryRow(t, `SELECT path FROM worklane WHERE id = $1`, id).Scan(&got)
			if want := pick(paths); got != want {
				t.Errorf("worklane %s path = %q, want %q", id, got, want)
			}
		}
		for id, paths := range trees {
			var got string
			dbfx.QueryRow(t, `SELECT path FROM issue_worktree WHERE id = $1`, id).Scan(&got)
			if want := pick(paths); got != want {
				t.Errorf("issue worktree %s path = %q, want %q", id, got, want)
			}
		}
		var checkout string
		dbfx.QueryRow(t, `SELECT checkout_path FROM repository WHERE id = $1`, repo).Scan(&checkout)
		if want := map[bool]string{false: oldCheckout, true: newCheckout}[landed]; checkout != want {
			t.Errorf("checkout_path = %q, want %q", checkout, want)
		}
		dbfx.QueryRow(t, `SELECT checkout_path FROM repository WHERE id = $1`, otherRepo).Scan(&checkout)
		if checkout != oldCheckout {
			t.Errorf("other repository checkout_path = %q, want %q", checkout, oldCheckout)
		}
	}

	agent := dbfx.Agent(t, "Relocate agent", runtime)
	task := dbfx.Task(t, agent, testutil.Cols{
		"runtime_id": runtime, "issue_id": dbfx.Issue(t, "Busy issue", testutil.Cols{"project_id": project}),
	})
	for _, status := range []string{"dispatched", "running"} {
		t.Run("refused while "+status, func(t *testing.T) {
			dbfx.Exec(t, `UPDATE agent_task_queue SET status = $2 WHERE id = $1`, task, status)
			wantCodeOf(t, relocate(http.StatusConflict), "repository_busy")
			assertPaths(t, false)
		})
	}
	dbfx.Exec(t, `UPDATE agent_task_queue SET status = 'completed' WHERE id = $1`, task)

	var response RelocateRepositoryResponse
	relocate(http.StatusOK).JSON(&response)
	assertPaths(t, true)
	if response.Repository.CheckoutPath != newCheckout {
		t.Fatalf("response checkout_path = %q, want %q", response.Repository.CheckoutPath, newCheckout)
	}
	if len(response.Worklanes) != 3 || len(response.IssueWorktrees) != 2 {
		t.Fatalf("response moved %d worklanes and %d issue worktrees, want 3 and 2", len(response.Worklanes), len(response.IssueWorktrees))
	}
	for _, tree := range response.IssueWorktrees {
		if tree.Path == keptLanding && tree.State != "kept" {
			t.Fatalf("kept issue worktree state = %q after relocate", tree.State)
		}
	}
	if !slices.Equal(response.MissingPaths, []string{keptLanding}) {
		t.Fatalf("missing_paths = %v, want the landing path the runtime did not find", response.MissingPaths)
	}
}
