package queries_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

// releaseFixture is one workspace with a repository, an owned unshipped
// release, its coordination rule, an agent owner and the agent's runtime.
type releaseFixture struct {
	fx                                    *testutil.Fixture
	pool                                  *pgxpool.Pool
	q                                     *db.Queries
	workspace, member, agent, runtime     pgtype.UUID
	repository, release, coordinationRule pgtype.UUID
}

func newReleaseFixture(t *testing.T) *releaseFixture {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is required for release subject query tests")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("configured test database is unreachable: %v", err)
	}
	bootstrap := testutil.New(pool, "", "")
	suffix := fmt.Sprint(time.Now().UnixNano())
	member := bootstrap.User(t, "Brian", "release-subject-"+suffix+"@example.test")
	workspace := bootstrap.Workspace(t, "Release subjects", "release-subject-"+suffix)
	fx := testutil.New(pool, workspace, member)
	// Rows written through queries and triggers have no Insert cleanup; there
	// are no foreign keys, so one sweep by workspace removes them all.
	fx.Cleanup(t, `DELETE FROM issue_wakeup_receipt WHERE wakeup_id IN (SELECT id FROM issue_wakeup WHERE workspace_id = $1)`, workspace)
	fx.Cleanup(t, `DELETE FROM issue_wakeup WHERE workspace_id = $1`, workspace)
	fx.Cleanup(t, `DELETE FROM comment WHERE workspace_id = $1`, workspace)
	fx.Cleanup(t, `DELETE FROM agent_task_queue WHERE agent_id IN (SELECT id FROM agent WHERE workspace_id = $1)`, workspace)
	fx.Cleanup(t, `DELETE FROM issue WHERE workspace_id = $1`, workspace)
	fx.Cleanup(t, `DELETE FROM release WHERE workspace_id = $1`, workspace)
	fx.Cleanup(t, `DELETE FROM issue_status WHERE workspace_id = $1`, workspace)

	runtime := fx.Runtime(t, "omp", testutil.Cols{"provider": "omp"})
	agent := fx.Agent(t, "Thomas", runtime)
	project := fx.Project(t, "Release line")
	repository := fx.Insert(t, "repository", testutil.Cols{
		"workspace_id": workspace, "project_id": project, "github_repo_id": 1,
		"full_name": "acme/release-" + suffix, "private": false, "default_branch": "main",
		"checkout_daemon_id": "", "checkout_path": "", "version_policy": "none",
	})
	release := fx.Insert(t, "release", testutil.Cols{
		"workspace_id": workspace, "repository_id": repository, "position": 1,
		"version": "1.0.0", "title": "First", "owner_type": "agent", "owner_id": agent,
	})
	f := &releaseFixture{
		fx: fx, pool: pool, q: db.New(pool),
		workspace: util.MustParseUUID(workspace), member: util.MustParseUUID(member),
		agent: util.MustParseUUID(agent), runtime: util.MustParseUUID(runtime),
		repository: util.MustParseUUID(repository), release: util.MustParseUUID(release),
	}
	rule, err := f.q.CreateReleaseCoordinationWakeup(t.Context(), db.CreateReleaseCoordinationWakeupParams{
		ID: newID(), ReleaseID: f.release, WorkspaceID: f.workspace,
	})
	if err != nil {
		t.Fatalf("create coordination rule: %v", err)
	}
	f.coordinationRule = rule.ID
	return f
}

func newID() pgtype.UUID { return dbid.NewV7() }

func (f *releaseFixture) memberComment(t *testing.T, content string, parent pgtype.UUID) db.Comment {
	t.Helper()
	c, err := f.q.CreateReleaseComment(t.Context(), db.CreateReleaseCommentParams{
		ReleaseID: f.release, WorkspaceID: f.workspace, AuthorType: "member", AuthorID: f.member,
		Content: content, Type: "comment", ParentID: parent,
	})
	if err != nil {
		t.Fatalf("create release comment: %v", err)
	}
	return c
}

type receiptRow struct {
	EventType string
	EventKey  string
	Payload   map[string]any
	TaskID    pgtype.UUID
	Processed bool
}

func (f *releaseFixture) receipts(t *testing.T) []receiptRow {
	t.Helper()
	rows, err := f.pool.Query(t.Context(), `
SELECT r.event_type, r.event_key, r.payload, r.task_id, r.processed_at IS NOT NULL
FROM issue_wakeup_receipt r JOIN issue_wakeup w ON w.id = r.wakeup_id
WHERE w.release_id = $1 ORDER BY r.created_at, r.id`, f.release)
	if err != nil {
		t.Fatalf("read receipts: %v", err)
	}
	defer rows.Close()
	var out []receiptRow
	for rows.Next() {
		var r receiptRow
		var payload []byte
		if err := rows.Scan(&r.EventType, &r.EventKey, &payload, &r.TaskID, &r.Processed); err != nil {
			t.Fatalf("scan receipt: %v", err)
		}
		if err := json.Unmarshal(payload, &r.Payload); err != nil {
			t.Fatalf("decode receipt payload: %v", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("receipts: %v", err)
	}
	return out
}

func requireCheckViolation(t *testing.T, err error, constraint string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != constraint {
		t.Fatalf("want check violation %s, got %v", constraint, err)
	}
}

func TestReleaseSubjectChecksKeepIssueRowsAndRefuseMixedSubjects(t *testing.T) {
	f := newReleaseFixture(t)
	ctx := t.Context()
	issue := f.fx.Issue(t, "Plain issue")
	// Existing issue rows stay valid with no release.
	f.fx.Comment(t, issue, "issue comment")
	f.fx.Task(t, util.UUIDToString(f.agent), testutil.Cols{"issue_id": issue, "runtime_id": f.runtime})

	_, err := f.pool.Exec(ctx, `INSERT INTO comment (workspace_id, issue_id, release_id, author_type, author_id, content, type)
VALUES ($1, $2, $3, 'member', $4, 'both', 'comment')`, f.workspace, issue, f.release, f.member)
	requireCheckViolation(t, err, "comment_subject_check")
	_, err = f.pool.Exec(ctx, `INSERT INTO comment (workspace_id, author_type, author_id, content, type)
VALUES ($1, 'member', $2, 'neither', 'comment')`, f.workspace, f.member)
	requireCheckViolation(t, err, "comment_subject_check")

	_, err = f.pool.Exec(ctx, `INSERT INTO agent_task_queue (agent_id, runtime_id, status, priority, issue_id, release_id)
VALUES ($1, $2, 'queued', 0, $3, $4)`, f.agent, f.runtime, issue, f.release)
	requireCheckViolation(t, err, "agent_task_queue_release_subject_check")

	_, err = f.pool.Exec(ctx, `INSERT INTO issue_wakeup (id, workspace_id, issue_id, release_id, instruction, kind, mode)
VALUES ($1, $2, $3, $4, '', 'event', 'continuous')`, newID(), f.workspace, issue, f.release)
	requireCheckViolation(t, err, "issue_wakeup_subject_check")
	_, err = f.pool.Exec(ctx, `INSERT INTO issue_wakeup (id, workspace_id, issue_id, instruction, kind, mode, system_rule)
VALUES ($1, $2, $3, '', 'event', 'continuous', 'release_coordination')`, newID(), f.workspace, issue)
	requireCheckViolation(t, err, "issue_wakeup_system_rule_check")

	_, err = f.pool.Exec(ctx, `UPDATE release SET owner_id = NULL WHERE id = $1`, f.release)
	requireCheckViolation(t, err, "release_owner_pair_check")
}

func TestReleaseCoordinationRuleIsOnePerRelease(t *testing.T) {
	f := newReleaseFixture(t)
	_, err := f.q.CreateReleaseCoordinationWakeup(t.Context(), db.CreateReleaseCoordinationWakeupParams{
		ID: newID(), ReleaseID: f.release, WorkspaceID: f.workspace,
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("second ensure should keep the first rule, got %v", err)
	}
	rule, err := f.q.GetReleaseCoordinationWakeup(t.Context(), f.release)
	if err != nil || rule.ID != f.coordinationRule || rule.AgentID.Valid {
		t.Fatalf("coordination rule = %+v, %v", rule, err)
	}
}

func TestReleaseMemberCommentsAreCapturedOnceEachAndAgentReportsAreNot(t *testing.T) {
	f := newReleaseFixture(t)
	root := f.memberComment(t, "Please ship the parser first", pgtype.UUID{})
	reply := f.memberComment(t, "And the docs", root.ID)
	if _, err := f.q.CreateReleaseComment(t.Context(), db.CreateReleaseCommentParams{
		ReleaseID: f.release, WorkspaceID: f.workspace, AuthorType: "agent", AuthorID: f.agent,
		Content: "Coordination report", Type: "comment",
	}); err != nil {
		t.Fatalf("agent report: %v", err)
	}

	got := f.receipts(t)
	if len(got) != 2 {
		t.Fatalf("want one receipt per member comment, got %+v", got)
	}
	for i, c := range []db.Comment{root, reply} {
		r := got[i]
		if r.EventType != "release.comment.created" || r.Payload["comment_id"] != util.UUIDToString(c.ID) ||
			r.Payload["thread_id"] != util.UUIDToString(root.ID) || r.Payload["actor_type"] != "member" ||
			r.Payload["release_id"] != util.UUIDToString(f.release) {
			t.Fatalf("receipt %d = %+v", i, r)
		}
	}
}

func TestReleaseCommentCaptureRollsBackWithItsSource(t *testing.T) {
	f := newReleaseFixture(t)
	tx, err := f.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.q.WithTx(tx).CreateReleaseComment(t.Context(), db.CreateReleaseCommentParams{
		ReleaseID: f.release, WorkspaceID: f.workspace, AuthorType: "member", AuthorID: f.member,
		Content: "never committed", Type: "comment",
	}); err != nil {
		t.Fatalf("create in tx: %v", err)
	}
	if err := tx.Rollback(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := f.receipts(t); len(got) != 0 {
		t.Fatalf("rolled-back comment left receipts: %+v", got)
	}
}

func TestUnownedOrShippedReleaseCapturesNothing(t *testing.T) {
	f := newReleaseFixture(t)
	f.fx.Exec(t, `UPDATE release SET owner_type = NULL, owner_id = NULL WHERE id = $1`, f.release)
	f.memberComment(t, "nobody owns this yet", pgtype.UUID{})
	f.fx.Exec(t, `UPDATE release SET owner_type = 'agent', owner_id = $2, shipped_at = now() WHERE id = $1`, f.release, f.agent)
	f.memberComment(t, "shipped already", pgtype.UUID{})
	if got := f.receipts(t); len(got) != 0 {
		t.Fatalf("unowned/shipped release captured %+v", got)
	}
}

func TestReleaseIssueEndingsAreCapturedForCustomDoneAndCancelled(t *testing.T) {
	f := newReleaseFixture(t)
	f.fx.Insert(t, "issue_status", testutil.Cols{
		"workspace_id": util.UUIDToString(f.workspace), "key": "verified", "name": "Verified",
		"category": "done", "color": "#00aa00",
	})
	issue := f.fx.Issue(t, "Release work", testutil.Cols{"release_id": util.UUIDToString(f.release)})
	move := func(status string) {
		f.fx.Exec(t, `UPDATE issue SET status = $2, revision = revision + 1 WHERE id = $1`, issue, status)
	}
	move("in_progress") // not terminal: nothing
	move("verified")    // custom done category: ended done
	move("done")        // same effective outcome: nothing
	move("cancelled")   // done -> cancelled is a new ending
	move("todo")        // reopen: nothing

	got := f.receipts(t)
	if len(got) != 2 {
		t.Fatalf("want two endings, got %+v", got)
	}
	if got[0].EventType != "release.issue.ended" || got[0].Payload["outcome"] != "done" || got[0].Payload["status"] != "verified" {
		t.Fatalf("custom done ending = %+v", got[0])
	}
	if got[1].Payload["outcome"] != "cancelled" || got[1].Payload["previous_outcome"] != "done" || got[1].Payload["issue_id"] != issue {
		t.Fatalf("cancelled ending = %+v", got[1])
	}
}

func TestReleaseThreadsStayInsideTheirRelease(t *testing.T) {
	f := newReleaseFixture(t)
	other := f.fx.Insert(t, "release", testutil.Cols{
		"workspace_id": util.UUIDToString(f.workspace), "repository_id": util.UUIDToString(f.repository),
		"position": 2, "version": "1.1.0", "title": "Second",
	})
	root := f.memberComment(t, "root", pgtype.UUID{})
	reply := f.memberComment(t, "reply", root.ID)

	_, err := f.q.CreateReleaseComment(t.Context(), db.CreateReleaseCommentParams{
		ReleaseID: util.MustParseUUID(other), WorkspaceID: f.workspace, AuthorType: "member", AuthorID: f.member,
		Content: "cross-release reply", Type: "comment", ParentID: root.ID,
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("reply into another release's thread must be refused, got %v", err)
	}
	var sqlRoot pgtype.UUID
	f.fx.QueryRow(t, `SELECT comment_thread_root_id($1)`, reply.ID).Scan(&sqlRoot)
	if sqlRoot != root.ID {
		t.Fatalf("comment_thread_root_id = %v", sqlRoot)
	}
}

func (f *releaseFixture) enqueue(t *testing.T) (db.AgentTaskQueue, error) {
	t.Helper()
	ctxJSON, _ := json.Marshal(map[string]any{"wakeup_id": util.UUIDToString(f.coordinationRule), "wakeup_revision": 1})
	return f.q.CreateReleaseWakeupTask(t.Context(), db.CreateReleaseWakeupTaskParams{
		AgentID: f.agent, RuntimeID: f.runtime, ReleaseID: f.release, WorkspaceID: f.workspace,
		WakeupContext: ctxJSON,
	})
}

func TestOneUnstartedCoordinationRunPerReleaseWithFollowUpBehindRunning(t *testing.T) {
	f := newReleaseFixture(t)
	first, err := f.enqueue(t)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	_, err = f.enqueue(t)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" || pgErr.ConstraintName != "idx_one_pending_task_per_release" {
		t.Fatalf("second unstarted run must conflict on the release slot, got %v", err)
	}
	f.fx.Exec(t, `UPDATE agent_task_queue SET status = 'running', dispatched_at = now(), started_at = now() WHERE id = $1`, first.ID)
	if _, err := f.enqueue(t); err != nil {
		t.Fatalf("a follow-up may queue while the run is running: %v", err)
	}

	f.fx.Exec(t, `UPDATE release SET shipped_at = now() WHERE id = $1`, f.release)
	f.fx.Exec(t, `UPDATE agent_task_queue SET status = 'cancelled' WHERE release_id = $1 AND status = 'queued'`, f.release)
	if _, err := f.enqueue(t); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("a shipped release must not get a run, got %v", err)
	}
}

func TestClaimRequiresReleaseCapabilityAndNoActiveReleaseRun(t *testing.T) {
	f := newReleaseFixture(t)
	task, err := f.enqueue(t)
	if err != nil {
		t.Fatal(err)
	}
	claim := func() (db.AgentTaskQueue, error) {
		return f.q.ClaimAgentTask(t.Context(), db.ClaimAgentTaskParams{
			AgentID: f.agent, RuntimeID: f.runtime, PrepareLeaseSecs: 60, RuntimeStaleSecs: 3600,
		})
	}
	if _, err := claim(); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("runtime without release-coordination-v1 claimed a release run: %v", err)
	}
	f.fx.Exec(t, `UPDATE agent_runtime SET metadata = '{"capabilities":["release-coordination-v1"]}'::jsonb WHERE id = $1`, f.runtime)

	// An earlier run of the same release by another agent is still running.
	otherAgent := f.fx.Agent(t, "Mara", util.UUIDToString(f.runtime))
	running := f.fx.Task(t, otherAgent, testutil.Cols{
		"runtime_id": util.UUIDToString(f.runtime), "release_id": util.UUIDToString(f.release),
		"status": "running", "dispatched_at": testutil.Raw("now()"), "started_at": testutil.Raw("now()"),
	})
	if _, err := claim(); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("claimed while another run of the release is active: %v", err)
	}
	f.fx.Exec(t, `UPDATE agent_task_queue SET status = 'completed', completed_at = now() WHERE id = $1`, running)
	got, err := claim()
	if err != nil || got.ID != task.ID || got.Status != "dispatched" {
		t.Fatalf("capable runtime claim = %+v, %v", got, err)
	}
}

func TestStartConsumesDeliveredReleaseInputsAndReturnsOmittedComments(t *testing.T) {
	f := newReleaseFixture(t)
	first := f.memberComment(t, "first", pgtype.UUID{})
	f.memberComment(t, "second", pgtype.UUID{})
	task, err := f.enqueue(t)
	if err != nil {
		t.Fatal(err)
	}
	f.fx.Exec(t, `UPDATE issue_wakeup_receipt SET task_id = $2 WHERE wakeup_id = $1`, f.coordinationRule, task.ID)

	fired, err := f.q.ConsumeReleaseTaskReceipts(t.Context(), db.ConsumeReleaseTaskReceiptsParams{
		TaskID: task.ID, DeliveredCommentIds: []pgtype.UUID{first.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(fired) != 1 || fired[0].ID != f.coordinationRule || fired[0].FireCount != 1 || fired[0].LastTaskID != task.ID || !fired[0].Enabled {
		t.Fatalf("fired rules = %+v", fired)
	}
	got := f.receipts(t)
	if len(got) != 2 || !got[0].Processed || got[1].Processed || got[1].TaskID.Valid {
		t.Fatalf("want first consumed and second returned to the rule, got %+v", got)
	}
}
