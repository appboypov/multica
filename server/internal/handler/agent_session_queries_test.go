package handler

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The agent_session queries carry every rule of an
// agent's session list: which sessions are listed, how a run finds its
// repository and branch, and which daemon may change a session. They are
// tested here on the database, since the list is the one place these rules hold.

// agentSessionTestRuntime inserts a runtime of a new daemon in workspaceID with
// the given status and returns the runtime and the daemon id.
func agentSessionTestRuntime(t *testing.T, workspaceID, status string) (runtimeID, daemonID string) {
	t.Helper()
	daemonID = "agent-session-daemon-" + uuid.NewString()
	runtimeID = dbfx.Runtime(t, "Agent session runtime "+daemonID, testutil.Cols{
		"workspace_id": workspaceID,
		"runtime_mode": "local",
		"provider":     "omp",
		"daemon_id":    daemonID,
		"status":       status,
	})
	return runtimeID, daemonID
}

// agentSessionTestAgent inserts an agent in workspaceID and removes the
// sessions registered for it when the test ends.
func agentSessionTestAgent(t *testing.T, workspaceID, runtimeID string) string {
	t.Helper()
	agentID := dbfx.Agent(t, "Agent session agent "+uuid.NewString(), runtimeID, testutil.Cols{
		"workspace_id": workspaceID,
		"runtime_mode": "local",
	})
	dbfx.Cleanup(t, `DELETE FROM agent_session WHERE agent_id = $1`, agentID)
	return agentID
}

func agentSessionTestUpsert(workspaceID, agentID, daemonID string, pid int32, workDir string) db.UpsertAgentSessionParams {
	return db.UpsertAgentSessionParams{
		WorkspaceID:      parseUUID(workspaceID),
		AgentID:          parseUUID(agentID),
		DaemonID:         daemonID,
		Pid:              pid,
		ProcessStartedAt: pgtype.Timestamptz{Time: time.Now().Add(-time.Minute), Valid: true},
		HerdrPaneID:      strToText("pane-" + uuid.NewString()),
		WorkDir:          workDir,
		Repository:       strToText("example/agent-sessions"),
		Branch:           strToText("main"),
		Status:           "idle",
	}
}

func registerTestAgentSession(t *testing.T, params db.UpsertAgentSessionParams) db.AgentSession {
	t.Helper()
	session, err := db.New(testPool).UpsertAgentSession(context.Background(), params)
	if err != nil {
		t.Fatalf("UpsertAgentSession: %v", err)
	}
	dbfx.Cleanup(t, `DELETE FROM agent_session WHERE id = $1`, uuidToString(session.ID))
	return session
}

func listTestAgentSessions(t *testing.T, workspaceID, agentID string) []db.ListAgentSessionsRow {
	t.Helper()
	rows, err := db.New(testPool).ListAgentSessions(context.Background(), db.ListAgentSessionsParams{
		WorkspaceID: parseUUID(workspaceID),
		AgentID:     parseUUID(agentID),
	})
	if err != nil {
		t.Fatalf("ListAgentSessions: %v", err)
	}
	return rows
}

func findAgentSessionRow(rows []db.ListAgentSessionsRow, id string) (db.ListAgentSessionsRow, bool) {
	for _, row := range rows {
		if uuidToString(row.ID) == id {
			return row, true
		}
	}
	return db.ListAgentSessionsRow{}, false
}

func agentSessionTestRepository(t *testing.T, workspaceID string) (repositoryID, fullName string) {
	t.Helper()
	projectID := dbfx.Project(t, "Agent session project", testutil.Cols{"workspace_id": workspaceID})
	fullName = "example/" + uuid.NewString()
	repositoryID = dbfx.Insert(t, "repository", testutil.Cols{
		"workspace_id": workspaceID, "project_id": projectID, "github_repo_id": 1,
		"full_name": fullName, "private": false, "default_branch": "main",
		"checkout_daemon_id": "agent-session-checkout", "checkout_path": "/checkout/" + fullName,
		"version_policy": "none",
	})
	return repositoryID, fullName
}

func agentSessionTestWorktree(t *testing.T, issueID, repositoryID, path, branch, state string) {
	t.Helper()
	dbfx.Insert(t, "issue_worktree", testutil.Cols{
		"workspace_id": testWorkspaceID, "issue_id": issueID, "repository_id": repositoryID,
		"daemon_id": "agent-session-checkout", "path": path, "branch": branch, "state": state,
	})
}

// agentSessionTestRun queues a task of agentID in status; an empty issueID or
// workDir leaves the column NULL.
func agentSessionTestRun(t *testing.T, agentID, runtimeID, issueID, status, workDir string) string {
	t.Helper()
	cols := testutil.Cols{
		"runtime_id":    runtimeID,
		"status":        status,
		"issue_id":      nil,
		"work_dir":      nil,
		"dispatched_at": testutil.Raw("now() - interval '2 minutes'"),
	}
	if issueID != "" {
		cols["issue_id"] = issueID
	}
	if workDir != "" {
		cols["work_dir"] = workDir
	}
	switch status {
	case "running":
		cols["started_at"] = testutil.Raw("now() - interval '1 minute'")
	case "completed":
		cols["started_at"] = testutil.Raw("now() - interval '1 minute'")
		cols["completed_at"] = testutil.Raw("now()")
	}
	return dbfx.Task(t, agentID, cols)
}

func TestListAgentSessionsListsTerminalSessionsOfOnlineDaemonsOnly(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}
	runtimeID, onlineDaemon := agentSessionTestRuntime(t, testWorkspaceID, "online")
	_, offlineDaemon := agentSessionTestRuntime(t, testWorkspaceID, "offline")
	agentID := agentSessionTestAgent(t, testWorkspaceID, runtimeID)

	online := registerTestAgentSession(t, agentSessionTestUpsert(testWorkspaceID, agentID, onlineDaemon, 4101, "/work/online"))
	offline := registerTestAgentSession(t, agentSessionTestUpsert(testWorkspaceID, agentID, offlineDaemon, 4102, "/work/offline"))

	rows := listTestAgentSessions(t, testWorkspaceID, agentID)
	if _, ok := findAgentSessionRow(rows, uuidToString(offline.ID)); ok {
		t.Fatalf("session of an offline daemon is listed: %+v", rows)
	}
	row, ok := findAgentSessionRow(rows, uuidToString(online.ID))
	if !ok {
		t.Fatalf("session of an online daemon is not listed: %+v", rows)
	}
	if row.Kind != "terminal" || !row.Pid.Valid || row.Pid.Int32 != 4101 {
		t.Fatalf("terminal row kind/pid = %q/%+v, want terminal/4101", row.Kind, row.Pid)
	}
	if row.HerdrPaneID != online.HerdrPaneID || row.WorkDir != "/work/online" || row.Status != "idle" {
		t.Fatalf("terminal row = %+v, want the registered pane, folder and status", row)
	}
	if row.Repository.String != "example/agent-sessions" || row.Branch.String != "main" {
		t.Fatalf("terminal row repository/branch = %+v/%+v", row.Repository, row.Branch)
	}
	if row.RuntimeID.Valid || row.TaskID.Valid || row.IssueID.Valid || row.IssueIdentifier != "" {
		t.Fatalf("terminal row carries run fields: %+v", row)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want only the online session: %+v", len(rows), rows)
	}
}

func TestListAgentSessionsListsActiveRunsWithFolderAndIssue(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}
	runtimeID, _ := agentSessionTestRuntime(t, testWorkspaceID, "online")
	agentID := agentSessionTestAgent(t, testWorkspaceID, runtimeID)
	issueID := dbfx.Issue(t, "Agent session run issue")
	var identifier string
	dbfx.QueryRow(t, `
		SELECT w.issue_prefix || '-' || i.number
		FROM issue i JOIN workspace w ON w.id = i.workspace_id
		WHERE i.id = $1`, issueID).Scan(&identifier)

	running := agentSessionTestRun(t, agentID, runtimeID, issueID, "running", "/work/running")
	dispatched := agentSessionTestRun(t, agentID, runtimeID, "", "dispatched", "")
	completed := agentSessionTestRun(t, agentID, runtimeID, issueID, "completed", "/work/completed")

	rows := listTestAgentSessions(t, testWorkspaceID, agentID)
	if _, ok := findAgentSessionRow(rows, completed); ok {
		t.Fatalf("completed task is listed: %+v", rows)
	}
	row, ok := findAgentSessionRow(rows, running)
	if !ok {
		t.Fatalf("running task is not listed: %+v", rows)
	}
	if row.Kind != "run" || row.Pid.Valid || row.HerdrPaneID.Valid || uuidToString(row.TaskID) != running {
		t.Fatalf("run row = %+v, want kind run with its task and no pid or pane", row)
	}
	if row.Status != "working" || row.WorkDir != "/work/running" {
		t.Fatalf("run status/work_dir = %q/%q, want working//work/running", row.Status, row.WorkDir)
	}
	if uuidToString(row.IssueID) != issueID || row.IssueIdentifier != identifier {
		t.Fatalf("run issue = %s/%q, want %s/%q", uuidToString(row.IssueID), row.IssueIdentifier, issueID, identifier)
	}
	if uuidToString(row.RuntimeID) != runtimeID || !row.RuntimeName.Valid || row.RuntimeName.String == "" {
		t.Fatalf("run runtime = %s/%+v, want %s with its name", uuidToString(row.RuntimeID), row.RuntimeName, runtimeID)
	}
	if !row.StartedAt.Valid {
		t.Fatalf("run started_at is empty")
	}

	starting, ok := findAgentSessionRow(rows, dispatched)
	if !ok {
		t.Fatalf("dispatched task is not listed: %+v", rows)
	}
	if starting.Status != "starting" || starting.IssueID.Valid || starting.IssueIdentifier != "" || starting.WorkDir != "" {
		t.Fatalf("dispatched row = %+v, want starting without issue or folder", starting)
	}
	if !starting.StartedAt.Valid {
		t.Fatalf("dispatched row started_at does not fall back to dispatched_at")
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want the running and the dispatched task: %+v", len(rows), rows)
	}
}

func TestListAgentSessionsGivesRunsTheRepositoryAndBranchOfTheirWorktree(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}
	runtimeID, _ := agentSessionTestRuntime(t, testWorkspaceID, "online")
	agentID := agentSessionTestAgent(t, testWorkspaceID, runtimeID)
	repoA, nameA := agentSessionTestRepository(t, testWorkspaceID)
	repoB, nameB := agentSessionTestRepository(t, testWorkspaceID)

	// Two active worktrees: the one at the run's folder wins.
	atFolderIssue := dbfx.Issue(t, "Agent session worktree at folder")
	agentSessionTestWorktree(t, atFolderIssue, repoA, "/wt/folder/a", "work-a", "active")
	agentSessionTestWorktree(t, atFolderIssue, repoB, "/wt/folder/b", "work-b", "active")
	atFolder := agentSessionTestRun(t, agentID, runtimeID, atFolderIssue, "running", "/wt/folder/b")

	// One active worktree elsewhere (the other is being removed): it is the run's.
	onlyIssue := dbfx.Issue(t, "Agent session only worktree")
	agentSessionTestWorktree(t, onlyIssue, repoA, "/wt/only/a", "only-a", "active")
	agentSessionTestWorktree(t, onlyIssue, repoB, "/wt/only/b", "only-b", "removing")
	only := agentSessionTestRun(t, agentID, runtimeID, onlyIssue, "running", "/work/elsewhere")

	// Two active worktrees, neither at the folder: neither is the run's.
	ambiguousIssue := dbfx.Issue(t, "Agent session ambiguous worktrees")
	agentSessionTestWorktree(t, ambiguousIssue, repoA, "/wt/ambiguous/a", "ambiguous-a", "active")
	agentSessionTestWorktree(t, ambiguousIssue, repoB, "/wt/ambiguous/b", "ambiguous-b", "active")
	ambiguous := agentSessionTestRun(t, agentID, runtimeID, ambiguousIssue, "running", "/work/elsewhere")

	rows := listTestAgentSessions(t, testWorkspaceID, agentID)
	for _, tc := range []struct {
		name, taskID, repository, branch string
	}{
		{"worktree at the folder", atFolder, nameB, "work-b"},
		{"only active worktree", only, nameA, "only-a"},
		{"two worktrees elsewhere", ambiguous, "", ""},
	} {
		row, ok := findAgentSessionRow(rows, tc.taskID)
		if !ok {
			t.Fatalf("%s: run not listed: %+v", tc.name, rows)
		}
		if row.Repository.String != tc.repository || row.Branch.String != tc.branch {
			t.Errorf("%s: repository/branch = %+v/%+v, want %q/%q", tc.name, row.Repository, row.Branch, tc.repository, tc.branch)
		}
		if row.Repository.Valid != (tc.repository != "") || row.Branch.Valid != (tc.branch != "") {
			t.Errorf("%s: repository/branch validity = %v/%v", tc.name, row.Repository.Valid, row.Branch.Valid)
		}
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want one per run: %+v", len(rows), rows)
	}
}

func TestListAgentSessionsGivesWorklaneRunsTheirRepositoryAndIssueBranch(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}
	runtimeID, _ := agentSessionTestRuntime(t, testWorkspaceID, "online")
	agentID := agentSessionTestAgent(t, testWorkspaceID, runtimeID)
	for _, tc := range []struct {
		name, laneBranch, workDir string
		isMain                    bool
	}{
		{"named lane on another issue", "another-issue-999", "/lane/named", false},
		{"detached named lane", "", "/lane/detached", false},
		{"main lane", "main", "/checkout/main", true},
		{"main lane without a task folder", "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repositoryID, fullName := agentSessionTestRepository(t, testWorkspaceID)
			cols := testutil.Cols{
				"workspace_id": testWorkspaceID, "repository_id": repositoryID,
				"name": "named", "is_main": tc.isMain, "branch": tc.laneBranch,
			}
			if tc.isMain {
				cols["name"] = "main"
			} else {
				cols["path"] = tc.workDir
				cols["daemon_id"] = "agent-session-checkout"
			}
			laneID := dbfx.Insert(t, "worklane", cols)
			issueID := dbfx.Issue(t, "Agent session worklane issue", testutil.Cols{"worklane_id": laneID})
			var identifier string
			dbfx.QueryRow(t, `
				SELECT w.issue_prefix || '-' || i.number
				FROM issue i JOIN workspace w ON w.id = i.workspace_id
				WHERE i.id = $1`, issueID).Scan(&identifier)
			runID := agentSessionTestRun(t, agentID, runtimeID, issueID, "running", tc.workDir)

			rows := listTestAgentSessions(t, testWorkspaceID, agentID)
			row, ok := findAgentSessionRow(rows, runID)
			if !ok {
				t.Fatalf("worklane run not listed: %+v", rows)
			}
			wantBranch := strings.ToLower(identifier)
			if !row.Repository.Valid || row.Repository.String != fullName || !row.Branch.Valid || row.Branch.String != wantBranch {
				t.Fatalf("worklane repository/branch = %+v/%+v, want %q/%q", row.Repository, row.Branch, fullName, wantBranch)
			}
			if row.WorkDir != tc.workDir || row.IssueIdentifier != identifier {
				t.Fatalf("worklane folder/issue = %q/%q, want %q/%q", row.WorkDir, row.IssueIdentifier, tc.workDir, identifier)
			}
		})
	}
}

func TestListAgentSessionsWorklaneRunsIgnoreIssueWorktrees(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}
	runtimeID, _ := agentSessionTestRuntime(t, testWorkspaceID, "online")
	agentID := agentSessionTestAgent(t, testWorkspaceID, runtimeID)
	laneRepo, laneName := agentSessionTestRepository(t, testWorkspaceID)
	worktreeRepo, _ := agentSessionTestRepository(t, testWorkspaceID)
	laneID := dbfx.Insert(t, "worklane", testutil.Cols{
		"workspace_id": testWorkspaceID, "repository_id": laneRepo, "name": "named",
		"path": "/lane/authoritative", "branch": "another-issue-999",
	})
	issueID := dbfx.Issue(t, "Agent session authoritative lane", testutil.Cols{"worklane_id": laneID})
	agentSessionTestWorktree(t, issueID, worktreeRepo, "/lane/authoritative", "unrelated-worktree", "active")
	runID := agentSessionTestRun(t, agentID, runtimeID, issueID, "running", "/lane/authoritative")

	rows := listTestAgentSessions(t, testWorkspaceID, agentID)
	row, ok := findAgentSessionRow(rows, runID)
	if !ok {
		t.Fatalf("worklane run not listed: %+v", rows)
	}
	if !row.Repository.Valid || row.Repository.String != laneName || !row.Branch.Valid || row.Branch.String != strings.ToLower(row.IssueIdentifier) {
		t.Fatalf("run did not use its lane repository and issue branch: %+v", row)
	}
}

func TestListAgentSessionsKeepsWorklaneRunRepositoriesInTheirWorkspace(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}
	runtimeID, _ := agentSessionTestRuntime(t, testWorkspaceID, "online")
	agentID := agentSessionTestAgent(t, testWorkspaceID, runtimeID)
	otherWorkspaceID := dbfx.Workspace(t, "Agent session foreign lane workspace", "agent-session-lane-"+uuid.NewString())
	localRepo, _ := agentSessionTestRepository(t, testWorkspaceID)
	foreignRepo, _ := agentSessionTestRepository(t, otherWorkspaceID)
	for _, tc := range []struct {
		name, laneWorkspace, repositoryID string
		missingLane                       bool
	}{
		{"foreign lane with local repository", otherWorkspaceID, localRepo, false},
		{"foreign lane with foreign repository", otherWorkspaceID, foreignRepo, false},
		{"local lane with foreign repository", testWorkspaceID, foreignRepo, false},
		{"local lane with missing repository", testWorkspaceID, uuid.NewString(), false},
		{"missing lane", "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			laneID := uuid.NewString()
			if !tc.missingLane {
				laneID = dbfx.Insert(t, "worklane", testutil.Cols{
					"workspace_id": tc.laneWorkspace, "repository_id": tc.repositoryID,
					"name": "isolated", "path": "/lane/isolation", "branch": "foreign-branch",
				})
			}
			issueID := dbfx.Issue(t, "Agent session isolated lane", testutil.Cols{"worklane_id": laneID})
			// A lane issue never takes repository or branch from an issue worktree,
			// even when its lane or repository cannot be read in this workspace.
			agentSessionTestWorktree(t, issueID, localRepo, "/lane/isolation", "unrelated-worktree", "active")
			runID := agentSessionTestRun(t, agentID, runtimeID, issueID, "running", "/lane/isolation")

			rows := listTestAgentSessions(t, testWorkspaceID, agentID)
			row, ok := findAgentSessionRow(rows, runID)
			if !ok {
				t.Fatalf("own run not listed: %+v", rows)
			}
			if row.Repository.Valid || row.Branch.Valid {
				t.Fatalf("run resolved an invalid lane/repository: %+v/%+v", row.Repository, row.Branch)
			}
			if uuidToString(row.IssueID) != issueID || row.IssueIdentifier == "" {
				t.Fatalf("run lost its own issue: %+v", row)
			}
			if foreignRows := listTestAgentSessions(t, otherWorkspaceID, agentID); len(foreignRows) != 0 {
				t.Fatalf("foreign workspace listed this agent's runs: %+v", foreignRows)
			}
		})
	}
}

func TestAgentSessionQueriesKeepWorkspacesApart(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	queries := db.New(testPool)
	otherWorkspaceID := dbfx.Workspace(t, "Agent session other workspace", "agent-session-other-"+uuid.NewString()[:8])

	runtimeID, daemonID := agentSessionTestRuntime(t, testWorkspaceID, "online")
	agentID := agentSessionTestAgent(t, testWorkspaceID, runtimeID)
	issueID := dbfx.Issue(t, "Agent session isolation issue")
	running := agentSessionTestRun(t, agentID, runtimeID, issueID, "running", "/work/isolation")
	own := registerTestAgentSession(t, agentSessionTestUpsert(testWorkspaceID, agentID, daemonID, 4201, "/work/own"))

	// A row of another workspace naming this agent, on a daemon online there.
	_, otherDaemonID := agentSessionTestRuntime(t, otherWorkspaceID, "online")
	foreign := registerTestAgentSession(t, agentSessionTestUpsert(otherWorkspaceID, agentID, otherDaemonID, 4202, "/work/foreign"))

	// A daemon online only in the other workspace does not show this workspace's session.
	_, elsewhereDaemonID := agentSessionTestRuntime(t, otherWorkspaceID, "online")
	elsewhere := registerTestAgentSession(t, agentSessionTestUpsert(testWorkspaceID, agentID, elsewhereDaemonID, 4203, "/work/elsewhere"))

	// The other workspace's worktree of the same issue never gives the run a branch.
	otherRepo, _ := agentSessionTestRepository(t, otherWorkspaceID)
	dbfx.Insert(t, "issue_worktree", testutil.Cols{
		"workspace_id": otherWorkspaceID, "issue_id": issueID, "repository_id": otherRepo,
		"daemon_id": otherDaemonID, "path": "/work/isolation", "branch": "foreign-branch",
	})

	rows := listTestAgentSessions(t, testWorkspaceID, agentID)
	if _, ok := findAgentSessionRow(rows, uuidToString(foreign.ID)); ok {
		t.Fatalf("another workspace's session is listed: %+v", rows)
	}
	if _, ok := findAgentSessionRow(rows, uuidToString(elsewhere.ID)); ok {
		t.Fatalf("session of a daemon online only in another workspace is listed: %+v", rows)
	}
	if _, ok := findAgentSessionRow(rows, uuidToString(own.ID)); !ok {
		t.Fatalf("own session is not listed: %+v", rows)
	}
	run, ok := findAgentSessionRow(rows, running)
	if !ok {
		t.Fatalf("own run is not listed: %+v", rows)
	}
	if run.Repository.Valid || run.Branch.Valid {
		t.Fatalf("run took another workspace's worktree: %+v/%+v", run.Repository, run.Branch)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want the own session and run: %+v", len(rows), rows)
	}

	otherRows := listTestAgentSessions(t, otherWorkspaceID, agentID)
	if _, ok := findAgentSessionRow(otherRows, running); ok {
		t.Fatalf("a run of another workspace's agent is listed: %+v", otherRows)
	}
	if _, ok := findAgentSessionRow(otherRows, uuidToString(own.ID)); ok {
		t.Fatalf("another workspace's session is listed: %+v", otherRows)
	}

	forDaemon, err := queries.ListAgentSessionsForDaemon(ctx, db.ListAgentSessionsForDaemonParams{
		DaemonID:    daemonID,
		WorkspaceID: parseUUID(otherWorkspaceID),
	})
	if err != nil {
		t.Fatalf("ListAgentSessionsForDaemon: %v", err)
	}
	if len(forDaemon) != 0 {
		t.Fatalf("daemon list of another workspace = %+v, want none", forDaemon)
	}
	forDaemon, err = queries.ListAgentSessionsForDaemon(ctx, db.ListAgentSessionsForDaemonParams{
		DaemonID:    daemonID,
		WorkspaceID: parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatalf("ListAgentSessionsForDaemon: %v", err)
	}
	if len(forDaemon) != 1 || forDaemon[0].ID != own.ID {
		t.Fatalf("daemon list = %+v, want the own session", forDaemon)
	}

	// Another workspace's register of the same process never takes the row over.
	takeover := agentSessionTestUpsert(otherWorkspaceID, agentID, daemonID, 4201, "/work/takeover")
	if _, err := queries.UpsertAgentSession(ctx, takeover); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("upsert from another workspace err = %v, want pgx.ErrNoRows", err)
	}
	var workspaceID, workDir string
	dbfx.QueryRow(t, `SELECT workspace_id::text, work_dir FROM agent_session WHERE id = $1`, uuidToString(own.ID)).Scan(&workspaceID, &workDir)
	if workspaceID != testWorkspaceID || workDir != "/work/own" {
		t.Fatalf("own session after a foreign upsert = %s/%s", workspaceID, workDir)
	}

	// Update and delete under another workspace change nothing.
	if _, err := queries.UpdateAgentSession(ctx, db.UpdateAgentSessionParams{
		Status: "working", ID: own.ID, WorkspaceID: parseUUID(otherWorkspaceID), DaemonID: daemonID,
	}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("update under another workspace err = %v, want pgx.ErrNoRows", err)
	}
	if _, err := queries.DeleteAgentSession(ctx, db.DeleteAgentSessionParams{
		ID: own.ID, WorkspaceID: parseUUID(otherWorkspaceID), DaemonID: daemonID,
	}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("delete under another workspace err = %v, want pgx.ErrNoRows", err)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM agent_session WHERE id = $1 AND status = 'idle'`, uuidToString(own.ID)); n != 1 {
		t.Fatalf("own session changed under another workspace (count %d)", n)
	}
}

func TestUpsertAgentSessionReplacesTheRowOfTheSameProcess(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}
	runtimeID, daemonID := agentSessionTestRuntime(t, testWorkspaceID, "online")
	firstAgent := agentSessionTestAgent(t, testWorkspaceID, runtimeID)
	secondAgent := agentSessionTestAgent(t, testWorkspaceID, runtimeID)

	first := registerTestAgentSession(t, agentSessionTestUpsert(testWorkspaceID, firstAgent, daemonID, 4301, "/work/first"))

	replacement := agentSessionTestUpsert(testWorkspaceID, secondAgent, daemonID, 4301, "/work/second")
	replacement.HerdrPaneID = pgtype.Text{}
	replacement.Repository = pgtype.Text{}
	replacement.Branch = pgtype.Text{}
	replacement.Status = "working"
	replacement.ProcessStartedAt = pgtype.Timestamptz{Time: time.Now().Truncate(time.Microsecond), Valid: true}
	second := registerTestAgentSession(t, replacement)

	if second.ID != first.ID {
		t.Fatalf("upsert made a second row %s beside %s", uuidToString(second.ID), uuidToString(first.ID))
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM agent_session WHERE daemon_id = $1 AND pid = 4301`, daemonID); n != 1 {
		t.Fatalf("rows for the process = %d, want 1", n)
	}
	if uuidToString(second.AgentID) != secondAgent || second.WorkDir != "/work/second" || second.Status != "working" {
		t.Fatalf("replaced row = %+v, want the second register's agent, folder and status", second)
	}
	if second.HerdrPaneID.Valid || second.Repository.Valid || second.Branch.Valid {
		t.Fatalf("replaced row kept the first register's pane, repository or branch: %+v", second)
	}
	if !second.ProcessStartedAt.Time.Equal(replacement.ProcessStartedAt.Time) {
		t.Fatalf("process_started_at = %v, want %v", second.ProcessStartedAt.Time, replacement.ProcessStartedAt.Time)
	}
	if second.StartedAt.Time.Before(first.StartedAt.Time) {
		t.Fatalf("replaced row started_at %v is before the first register's %v", second.StartedAt.Time, first.StartedAt.Time)
	}

	if rows := listTestAgentSessions(t, testWorkspaceID, firstAgent); len(rows) != 0 {
		t.Fatalf("first agent still lists the replaced session: %+v", rows)
	}
	rows := listTestAgentSessions(t, testWorkspaceID, secondAgent)
	if len(rows) != 1 || rows[0].ID != first.ID || rows[0].Status != "working" {
		t.Fatalf("second agent's sessions = %+v, want the replaced row", rows)
	}
}

func TestAgentSessionUpdateAndDeleteNeedTheFollowingDaemon(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	queries := db.New(testPool)
	runtimeID, daemonID := agentSessionTestRuntime(t, testWorkspaceID, "online")
	_, otherDaemonID := agentSessionTestRuntime(t, testWorkspaceID, "online")
	agentID := agentSessionTestAgent(t, testWorkspaceID, runtimeID)
	session := registerTestAgentSession(t, agentSessionTestUpsert(testWorkspaceID, agentID, daemonID, 4401, "/work/followed"))

	update := db.UpdateAgentSessionParams{
		Status:      "blocked",
		Repository:  strToText("example/moved"),
		Branch:      strToText("feature"),
		ID:          session.ID,
		WorkspaceID: parseUUID(testWorkspaceID),
		DaemonID:    otherDaemonID,
	}
	if _, err := queries.UpdateAgentSession(ctx, update); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("update by another daemon err = %v, want pgx.ErrNoRows", err)
	}
	if _, err := queries.DeleteAgentSession(ctx, db.DeleteAgentSessionParams{
		ID: session.ID, WorkspaceID: parseUUID(testWorkspaceID), DaemonID: otherDaemonID,
	}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("delete by another daemon err = %v, want pgx.ErrNoRows", err)
	}
	if n := dbfx.Count(t, `
		SELECT count(*) FROM agent_session
		WHERE id = $1 AND status = 'idle' AND repository = 'example/agent-sessions' AND branch = 'main'`,
		uuidToString(session.ID)); n != 1 {
		t.Fatalf("another daemon changed the session (count %d)", n)
	}

	update.DaemonID = daemonID
	updated, err := queries.UpdateAgentSession(ctx, update)
	if err != nil {
		t.Fatalf("update by the following daemon: %v", err)
	}
	if updated.Status != "blocked" || updated.Repository.String != "example/moved" || updated.Branch.String != "feature" {
		t.Fatalf("updated row = %+v, want blocked on example/moved feature", updated)
	}

	deletedAgent, err := queries.DeleteAgentSession(ctx, db.DeleteAgentSessionParams{
		ID: session.ID, WorkspaceID: parseUUID(testWorkspaceID), DaemonID: daemonID,
	})
	if err != nil {
		t.Fatalf("delete by the following daemon: %v", err)
	}
	if uuidToString(deletedAgent) != agentID {
		t.Fatalf("delete returned agent %s, want %s", uuidToString(deletedAgent), agentID)
	}
	if n := dbfx.Count(t, `SELECT count(*) FROM agent_session WHERE id = $1`, uuidToString(session.ID)); n != 0 {
		t.Fatalf("session survived its delete")
	}
}
