package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/multica-ai/multica/server/pkg/taskfailure"
)

type worklaneClaimFixture struct {
	runtimeID string
	daemonID  string
	repoID    string
	repoName  string
	laneID    string
	issueID   string
	taskID    string
}

func newWorklaneClaimFixture(t *testing.T, repositoryInProject, mainLane bool) worklaneClaimFixture {
	t.Helper()
	f := worklaneClaimFixture{daemonID: uuid.NewString(), repoName: uuid.NewString()}
	f.runtimeID = dbfx.Runtime(t, "Claim worklane machine", testutil.Cols{
		"daemon_id": f.daemonID, "runtime_mode": "local",
	})
	agentID := dbfx.Agent(t, "Claim worklane agent", f.runtimeID, testutil.Cols{"runtime_mode": "local"})
	projectID := dbfx.Project(t, "Claim worklane project")
	f.repoID = dbfx.Insert(t, "repository", testutil.Cols{
		"workspace_id": testWorkspaceID, "project_id": projectID, "github_repo_id": 1,
		"full_name": "test/" + f.repoName, "private": true, "default_branch": "main",
		"checkout_daemon_id": f.daemonID, "checkout_path": "/test/checkout", "version_policy": "none",
	})
	f.laneID = dbfx.Insert(t, "worklane", testutil.Cols{
		"workspace_id": testWorkspaceID, "repository_id": f.repoID, "name": "Claim lane", "is_main": mainLane,
		"daemon_id": f.daemonID, "path": "/test/lane", "branch": "work-next-issue",
	})
	if !repositoryInProject {
		projectID = dbfx.Project(t, "Claim project without the lane repository")
	}
	f.issueID = dbfx.Issue(t, "Claim issue with linked PR", testutil.Cols{
		"project_id": projectID, "worklane_id": f.laneID,
	})
	f.taskID = dbfx.Task(t, agentID, testutil.Cols{"runtime_id": f.runtimeID, "issue_id": f.issueID})
	return f
}

type worklaneClaimPullRequest struct {
	cols             testutil.Cols
	foreignWorkspace bool
	unlinked         bool
}

func (f worklaneClaimFixture) pullRequests(t *testing.T, rows ...worklaneClaimPullRequest) {
	t.Helper()
	for i, row := range rows {
		created := time.Date(2026, time.October, 16, 12, i, 0, 0, time.UTC)
		cols := testutil.Cols{
			"workspace_id": testWorkspaceID, "installation_id": 1,
			"repo_owner": "test", "repo_name": f.repoName, "pr_number": i + 1,
			"title": "Linked claim PR", "state": "open", "branch": "review/actual-head",
			"html_url":      "https://github.com/test/" + f.repoName + "/pull/1",
			"pr_created_at": created, "pr_updated_at": created,
		}
		for key, value := range row.cols {
			cols[key] = value
		}
		if row.foreignWorkspace {
			cols["workspace_id"] = dbfx.Workspace(t, "Foreign PR workspace", "claim-pr-"+uuid.NewString())
		}
		prID := dbfx.Insert(t, "github_pull_request", cols)
		if !row.unlinked {
			dbfx.InsertNoID(t, "issue_pull_request", testutil.Cols{
				"issue_id": f.issueID, "pull_request_id": prID, "linked_by_type": "member",
			}, "issue_id = $1 AND pull_request_id = $2", f.issueID, prID)
		}
	}
}

func (f worklaneClaimFixture) claim(t *testing.T, h *Handler, workspaceID string, status int) *testutil.Response {
	t.Helper()
	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+f.runtimeID+"/claim", nil, workspaceID, f.daemonID)
	return testutil.Call(t, h.ClaimTaskByRuntime, withURLParam(req, "runtimeId", f.runtimeID)).Want(status)
}

func TestClaimTask_WorklanePullRequestBranch(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	for _, repositoryInProject := range []bool{true, false} {
		name := "existing repository"
		if !repositoryInProject {
			name = "appended repository"
		}
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct {
				name string
				rows []worklaneClaimPullRequest
				want string
			}{
				{name: "linked open", rows: []worklaneClaimPullRequest{{}}, want: "review/actual-head"},
				{
					name: "newest matching open by creation time",
					rows: []worklaneClaimPullRequest{
						{cols: testutil.Cols{"branch": "review/older", "pr_updated_at": time.Date(2026, time.October, 17, 0, 0, 0, 0, time.UTC)}},
						{},
						{cols: testutil.Cols{"state": "closed", "branch": "review/closed"}},
						{cols: testutil.Cols{"state": "merged", "branch": "review/merged"}},
						{cols: testutil.Cols{"branch": "review/foreign"}, foreignWorkspace: true},
						{cols: testutil.Cols{"repo_name": "other-repository", "branch": "review/other-repo"}},
						{cols: testutil.Cols{"repo_owner": "other-owner", "branch": "review/other-owner"}},
						{cols: testutil.Cols{"branch": ""}},
						{cols: testutil.Cols{"branch": nil}},
						{cols: testutil.Cols{"branch": "review/not-linked"}, unlinked: true},
					},
					want: "review/actual-head",
				},
				{name: "closed only", rows: []worklaneClaimPullRequest{{cols: testutil.Cols{"state": "closed"}}}},
				{name: "merged only", rows: []worklaneClaimPullRequest{{cols: testutil.Cols{"state": "merged"}}}},
				{name: "draft only", rows: []worklaneClaimPullRequest{{cols: testutil.Cols{"state": "draft"}}}},
				{name: "wrong repository", rows: []worklaneClaimPullRequest{{cols: testutil.Cols{"repo_name": "other-repository"}}}},
				{name: "wrong owner", rows: []worklaneClaimPullRequest{{cols: testutil.Cols{"repo_owner": "other-owner"}}}},
				{name: "foreign workspace", rows: []worklaneClaimPullRequest{{foreignWorkspace: true}}},
				{name: "empty branch", rows: []worklaneClaimPullRequest{{cols: testutil.Cols{"branch": ""}}}},
				{name: "null branch", rows: []worklaneClaimPullRequest{{cols: testutil.Cols{"branch": nil}}}},
				{name: "not linked", rows: []worklaneClaimPullRequest{{unlinked: true}}},
				{name: "no pull requests"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					f := newWorklaneClaimFixture(t, repositoryInProject, false)
					f.pullRequests(t, tc.rows...)
					response := f.claim(t, testHandler, testWorkspaceID, http.StatusOK)
					var out struct {
						Task *AgentTaskResponse `json:"task"`
					}
					response.JSON(&out)
					if out.Task == nil || len(out.Task.Repositories) != 1 {
						t.Fatalf("expected one claimed repository, got %+v", out.Task)
					}
					repo := out.Task.Repositories[0]
					if repo.ID != f.repoID || repo.FullName != "test/"+f.repoName || repo.CheckoutPath != "/test/checkout" {
						t.Fatalf("repository context changed: %+v", repo)
					}
					lane := repo.Worklane
					if lane == nil || lane.ID != f.laneID || lane.Path != "/test/lane" || lane.DaemonID != f.daemonID || lane.DefaultBranch != "main" {
						t.Fatalf("worklane context = %+v", lane)
					}
					if lane.PullRequestBranch != tc.want {
						t.Errorf("pull request branch = %q, want %q", lane.PullRequestBranch, tc.want)
					}
					if lane.Branch == nil || *lane.Branch != "work-next-issue" {
						t.Errorf("lane branch changed: %+v", lane.Branch)
					}
					if present := strings.Contains(response.Text(), `"pull_request_branch"`); present != (tc.want != "") {
						t.Errorf("optional pull_request_branch presence = %v, want %v", present, tc.want != "")
					}
				})
			}
		})
	}
}

func TestClaimTask_WorklanePullRequestLoadFailureRequeues(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	f := newWorklaneClaimFixture(t, true, false)
	f.pullRequests(t, worklaneClaimPullRequest{})
	h := *testHandler
	h.Queries = db.New(failQueryDBTX{
		DBTX: testPool, failOn: "-- name: ListPullRequestsByIssue", err: errors.New("transient PR read failure"),
	})
	response := f.claim(t, &h, testWorkspaceID, http.StatusInternalServerError)
	if strings.Contains(response.Text(), `"task"`) || strings.Contains(response.Text(), "review/actual-head") {
		t.Fatalf("failed PR lookup emitted claim context: %s", response.Text())
	}
	var status string
	var dispatchedAt *time.Time
	dbfx.QueryRow(t, "SELECT status, dispatched_at FROM agent_task_queue WHERE id = $1", f.taskID).Scan(&status, &dispatchedAt)
	if status != "queued" || dispatchedAt != nil {
		t.Fatalf("failed claim status = %q, dispatched_at = %v; want queued and cleared", status, dispatchedAt)
	}
	var out struct {
		Task *AgentTaskResponse `json:"task"`
	}
	f.claim(t, testHandler, testWorkspaceID, http.StatusOK).JSON(&out)
	if out.Task == nil || out.Task.ID != f.taskID || len(out.Task.Repositories) != 1 || out.Task.Repositories[0].Worklane == nil || out.Task.Repositories[0].Worklane.PullRequestBranch != "review/actual-head" {
		t.Fatalf("redelivered claim = %+v", out.Task)
	}
}

func TestClaimTask_WorklanePullRequestPreservesDaemonGate(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	for _, mainLane := range []bool{false, true} {
		name := "lane worktree"
		if mainLane {
			name = "main checkout"
		}
		t.Run(name, func(t *testing.T) {
			f := newWorklaneClaimFixture(t, true, mainLane)
			f.pullRequests(t, worklaneClaimPullRequest{})
			foreignWorkspace := dbfx.Workspace(t, "Unauthorized claim workspace", "claim-access-"+uuid.NewString())
			f.claim(t, testHandler, foreignWorkspace, http.StatusNotFound)
			var status string
			dbfx.QueryRow(t, "SELECT status FROM agent_task_queue WHERE id = $1", f.taskID).Scan(&status)
			if status != "queued" {
				t.Fatalf("cross-workspace claim changed task status to %q", status)
			}
			dbfx.Exec(t, "UPDATE agent_runtime SET daemon_id = $2 WHERE id = $1", f.runtimeID, uuid.NewString())
			// An open PR must not bypass the lane's runtime-machine gate, even
			// when the PR query itself would fail if reached before the gate.
			h := *testHandler
			h.Queries = db.New(failQueryDBTX{
				DBTX: testPool, failOn: "-- name: ListPullRequestsByIssue", err: errors.New("PR query must follow daemon authorization"),
			})
			var out struct {
				Task *AgentTaskResponse `json:"task"`
			}
			f.claim(t, &h, testWorkspaceID, http.StatusOK).JSON(&out)
			if out.Task != nil {
				t.Fatalf("wrong-machine claim dispatched a task: %+v", out.Task)
			}
			var reason string
			dbfx.QueryRow(t, "SELECT status, failure_reason FROM agent_task_queue WHERE id = $1", f.taskID).Scan(&status, &reason)
			if status != "failed" || reason != taskfailure.ReasonWorklaneElsewhere.String() {
				t.Fatalf("wrong-machine task = %q, %q; want failed/worklane_elsewhere", status, reason)
			}
		})
	}
}

func TestClaimWorklane_WithoutWorklaneDoesNotLoadPullRequests(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	f := newWorklaneClaimFixture(t, true, false)
	issue, err := testHandler.Queries.GetIssue(context.Background(), parseUUID(f.issueID))
	if err != nil {
		t.Fatal(err)
	}
	issue.WorklaneID.Valid = false
	h := *testHandler
	h.Queries = db.New(failQueryDBTX{
		DBTX: testPool, failOn: "-- name: ListPullRequestsByIssue", err: errors.New("non-lane task must not load PR context"),
	})
	repos := []protocol.ClaimRepository{{ID: f.repoID, FullName: "test/" + f.repoName}}
	got, failure := h.claimWorklane(context.Background(), &db.AgentTaskQueue{}, db.AgentRuntime{}, "agent", issue, repos)
	if failure != nil || len(got) != 1 || got[0].ID != f.repoID || got[0].Worklane != nil {
		t.Fatalf("non-lane claim = %+v, failure = %+v", got, failure)
	}
}

// newCheckoutClaimFixture is a repository that works in its checkout, with
// its main worklane, and an issue of its project in the lane "Claim lane" or
// in no worklane.
func newCheckoutClaimFixture(t *testing.T, inLane bool) (worklaneClaimFixture, string) {
	t.Helper()
	f := newWorklaneClaimFixture(t, true, false)
	dbfx.Exec(t, "UPDATE repository SET work_in_checkout = true WHERE id = $1", f.repoID)
	mainID := dbfx.Insert(t, "worklane", testutil.Cols{
		"workspace_id": testWorkspaceID, "repository_id": f.repoID, "name": "main", "is_main": true,
		"branch": "main",
	})
	if !inLane {
		dbfx.Exec(t, "UPDATE issue SET worklane_id = NULL WHERE id = $1", f.issueID)
	}
	return f, mainID
}

func TestClaimTask_WorkInCheckoutRoutesToMain(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	for _, tc := range []struct {
		name   string
		inLane bool
		pr     bool
		want   string
	}{
		{name: "issue in no worklane", want: ""},
		{name: "issue in another worklane", inLane: true, want: ""},
		{name: "issue with an open pull request", pr: true, want: "review/actual-head"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, mainID := newCheckoutClaimFixture(t, tc.inLane)
			if tc.pr {
				f.pullRequests(t, worklaneClaimPullRequest{})
			}
			var out struct {
				Task *AgentTaskResponse `json:"task"`
			}
			f.claim(t, testHandler, testWorkspaceID, http.StatusOK).JSON(&out)
			if out.Task == nil || len(out.Task.Repositories) != 1 {
				t.Fatalf("expected one claimed repository, got %+v", out.Task)
			}
			lane := out.Task.Repositories[0].Worklane
			if lane == nil || lane.ID != mainID || !lane.InCheckout || lane.Path != "/test/checkout" || lane.DaemonID != f.daemonID {
				t.Fatalf("worklane context = %+v, want main in the checkout", lane)
			}
			if lane.PullRequestBranch != tc.want {
				t.Errorf("pull request branch = %q, want %q", lane.PullRequestBranch, tc.want)
			}
		})
	}
}

func TestClaimTask_WorkInCheckoutOnAnotherMachine(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	for _, inLane := range []bool{false, true} {
		t.Run(map[bool]string{false: "issue in no worklane", true: "issue in a worklane"}[inLane], func(t *testing.T) {
			f, _ := newCheckoutClaimFixture(t, inLane)
			dbfx.Exec(t, "UPDATE repository SET checkout_daemon_id = $2 WHERE id = $1", f.repoID, uuid.NewString())
			var out struct {
				Task *AgentTaskResponse `json:"task"`
			}
			f.claim(t, testHandler, testWorkspaceID, http.StatusOK).JSON(&out)
			var status, reason string
			dbfx.QueryRow(t, "SELECT status, COALESCE(failure_reason, '') FROM agent_task_queue WHERE id = $1", f.taskID).Scan(&status, &reason)
			if inLane {
				if out.Task != nil || status != "failed" || reason != taskfailure.ReasonWorklaneElsewhere.String() {
					t.Fatalf("lane task on another machine: task %+v, %q/%q, want failed/worklane_elsewhere", out.Task, status, reason)
				}
				return
			}
			if out.Task == nil || len(out.Task.Repositories) != 1 || out.Task.Repositories[0].Worklane != nil || status == "failed" {
				t.Fatalf("issue without a lane on another machine: task %+v, status %q, want no worklane block", out.Task, status)
			}
		})
	}
}

func TestReportWorklaneBranch_WorkInCheckout(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	report := func(f worklaneClaimFixture, worklaneID string) *testutil.Response {
		req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+f.taskID+"/worklane-branch",
			map[string]string{"branch": "work-1400", "worklane_id": worklaneID}, testWorkspaceID, f.daemonID)
		return testutil.Call(testHandler.ReportWorklaneBranch, withURLParam(req, "taskId", f.taskID))
	}
	t.Run("main of a repository that works in its checkout", func(t *testing.T) {
		f, mainID := newCheckoutClaimFixture(t, false)
		report(f, mainID).Want(http.StatusOK)
		var branch string
		dbfx.QueryRow(t, "SELECT branch FROM worklane WHERE id = $1", mainID).Scan(&branch)
		if branch != "work-1400" {
			t.Fatalf("main branch = %q, want work-1400", branch)
		}
	})
	t.Run("main of a repository that works in worktrees", func(t *testing.T) {
		f, mainID := newCheckoutClaimFixture(t, true)
		dbfx.Exec(t, "UPDATE repository SET work_in_checkout = false WHERE id = $1", f.repoID)
		report(f, mainID).Want(http.StatusConflict)
	})
}
