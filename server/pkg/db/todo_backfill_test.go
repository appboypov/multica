package db_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

func applyTodoBackfill(t *testing.T, fx *testutil.Fixture) pgx.Tx {
	t.Helper()
	migration, err := os.ReadFile("../../migrations/744_todo_backfill.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := fx.Pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// The migration is table-wide. Roll it back so concurrent suites' rows
	// cannot be archived or backfilled by this test.
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	for range 2 {
		if _, err := tx.Exec(context.Background(), string(migration)); err != nil {
			t.Fatalf("apply todo backfill: %v", err)
		}
	}
	return tx
}

func backfillCount(t *testing.T, tx pgx.Tx, sql string, args ...any) int {
	t.Helper()
	var count int
	if err := tx.QueryRow(context.Background(), sql, args...).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func wantBackfilled(t *testing.T, tx pgx.Tx, fx *testutil.Fixture, kind, source string, want int) {
	t.Helper()
	got := backfillCount(t, tx, `SELECT count(*) FROM todo WHERE workspace_id=$1 AND member_id=$2 AND kind=$3 AND source_key=$4 AND done_at IS NULL`, fx.WorkspaceID, fx.UserID, kind, source)
	if got != want {
		t.Errorf("%s source %s: got %d open rows, want %d", kind, source, got, want)
	}
}

func backfillTask(t *testing.T, fx *testutil.Fixture, agent, runtime string, cols testutil.Cols) string {
	t.Helper()
	defaults := testutil.Cols{"runtime_id": runtime, "originator_user_id": fx.UserID, "accountable_user_id": fx.UserID}
	for key, value := range cols {
		defaults[key] = value
	}
	return fx.Task(t, agent, defaults)
}

func backfillNotice(t *testing.T, fx *testutil.Fixture, kind string, details map[string]string, cols testutil.Cols) string {
	t.Helper()
	encoded, err := json.Marshal(details)
	if err != nil {
		t.Fatal(err)
	}
	defaults := testutil.Cols{"workspace_id": fx.WorkspaceID, "recipient_type": "member", "recipient_id": fx.UserID, "type": kind, "title": "Notice", "body": "Explanation", "details": encoded}
	for key, value := range cols {
		defaults[key] = value
	}
	return fx.Insert(t, "inbox_item", defaults)
}

func TestTodoBackfillQuestionsAssignmentsAndReviews(t *testing.T) {
	fx, _ := todoFixture(t)
	issue := fx.Issue(t, "Question issue", testutil.Cols{"status": "in_review"})
	other := fx.User(t, "Other answerer", "backfill-"+dbid.NewV7().String()+"@example.test")
	fx.Member(t, fx.WorkspaceID, other, "member")
	question := func(status, policy string, answered bool) string {
		id := dbid.NewV7()
		qid := fx.Insert(t, "question", testutil.Cols{"id": id, "workspace_id": fx.WorkspaceID, "ask_id": id, "ask_position": 0, "asker_agent_id": dbid.NewV7(), "asked_in_task_id": dbid.NewV7(), "prompt": "Approve?", "answer_type": "approval", "answer_policy": policy, "status": status})
		fx.InsertNoID(t, "question_link", testutil.Cols{"workspace_id": fx.WorkspaceID, "question_id": qid, "link_type": "issue", "link_id": issue, "position": 0}, "question_id=$1", qid)
		for _, user := range []string{fx.UserID, other} {
			fx.InsertNoID(t, "question_answerer", testutil.Cols{"workspace_id": fx.WorkspaceID, "question_id": qid, "user_id": user}, "question_id=$1 AND user_id=$2", qid, user)
		}
		if answered {
			fx.Insert(t, "question_answer", testutil.Cols{"id": dbid.NewV7(), "workspace_id": fx.WorkspaceID, "question_id": qid, "answerer_user_id": other, "value": `true`})
		}
		return qid
	}
	open := question("open", "any_one", false)
	partial := question("partly_answered", "each", true)
	answered := question("answered", "any_one", true)
	withdrawn := question("withdrawn", "each", false)
	closed := question("closed", "any_one", false)
	selfReview := fx.Issue(t, "Own review", testutil.Cols{"status": "in_review", "assignee_type": "member", "assignee_id": fx.UserID})
	waitingQA := fx.Issue(t, "Waiting for QA", testutil.Cols{"status": "waiting_for_qa"})
	assigned := make(map[string]string)
	for _, status := range []string{"todo", "back_from_qa", "blocked", "in_review", "backlog", "in_progress", "done"} {
		assigned[status] = fx.Issue(t, status, testutil.Cols{"status": status, "assignee_type": "member", "assignee_id": fx.UserID})
	}
	// Existing rows win over the backfill, including their stable IDs/times.
	existing := fx.Insert(t, "todo", testutil.Cols{"workspace_id": fx.WorkspaceID, "member_id": fx.UserID, "kind": "assigned", "source_key": assigned["todo"], "issue_id": assigned["todo"], "title": "Existing"})
	tx := applyTodoBackfill(t, fx)
	for _, id := range []string{open, partial} {
		wantBackfilled(t, tx, fx, "question", id, 1)
	}
	for _, id := range []string{answered, withdrawn, closed} {
		wantBackfilled(t, tx, fx, "question", id, 0)
	}
	if got := backfillCount(t, tx, `SELECT count(*) FROM todo WHERE workspace_id=$1 AND kind='question' AND question_id=$2 AND member_id=$3`, fx.WorkspaceID, partial, other); got != 0 {
		t.Errorf("answered member has %d partial-question rows", got)
	}
	wantBackfilled(t, tx, fx, "review", issue, 1) // Stored even while a question hides it.
	wantBackfilled(t, tx, fx, "review", selfReview, 0)
	wantBackfilled(t, tx, fx, "review", waitingQA, 0)
	for status, id := range assigned {
		want := 0
		if status == "todo" || status == "back_from_qa" || status == "blocked" || status == "in_review" {
			want = 1
		}
		wantBackfilled(t, tx, fx, "assigned", id, want)
	}
	if got := backfillCount(t, tx, `SELECT count(*) FROM todo WHERE id=$1 AND title='Existing'`, existing); got != 1 {
		t.Error("backfill replaced an existing open row")
	}
}

func TestTodoBackfillAgentFailures(t *testing.T) {
	fx, _ := todoFixture(t)
	runtime := fx.Runtime(t, "Backfill runtime")
	agent := fx.Agent(t, "Backfill agent", runtime)
	squad := fx.Squad(t, "Backfill squad", agent)
	now := time.Now().UTC()
	failedIssue := fx.Issue(t, "Latest run failed")
	failed := backfillTask(t, fx, agent, runtime, testutil.Cols{"issue_id": failedIssue, "status": "failed", "created_at": now.Add(-time.Hour), "completed_at": now})
	resumedIssue := fx.Issue(t, "Resumed")
	stale := backfillTask(t, fx, agent, runtime, testutil.Cols{"issue_id": resumedIssue, "status": "failed", "created_at": now.Add(-2 * time.Hour)})
	backfillTask(t, fx, agent, runtime, testutil.Cols{"issue_id": resumedIssue, "status": "completed", "created_at": now.Add(-time.Hour)})
	unattributedIssue := fx.Issue(t, "No human")
	unattributed := backfillTask(t, fx, agent, runtime, testutil.Cols{"issue_id": unattributedIssue, "status": "failed", "originator_user_id": nil})
	blocked := func(assigneeType, assigneeID, actorType string, resumed bool) string {
		issue := fx.Issue(t, "Blocked", testutil.Cols{"status": "blocked", "assignee_type": assigneeType, "assignee_id": assigneeID})
		backfillTask(t, fx, agent, runtime, testutil.Cols{"issue_id": issue, "status": "completed", "created_at": now.Add(-2 * time.Hour), "dispatched_at": now.Add(-90 * time.Minute), "started_at": now.Add(-time.Hour)})
		actor := agent
		if actorType == "member" {
			actor = fx.UserID
		}
		fx.Insert(t, "activity_log", testutil.Cols{"workspace_id": fx.WorkspaceID, "issue_id": issue, "actor_type": actorType, "actor_id": actor, "action": "status_changed", "details": `{"from":"in_progress","to":"blocked"}`, "created_at": now.Add(-30 * time.Minute)})
		if resumed {
			backfillTask(t, fx, agent, runtime, testutil.Cols{"issue_id": issue, "status": "running", "created_at": now.Add(-10 * time.Minute), "dispatched_at": now.Add(-5 * time.Minute), "started_at": now})
		}
		return issue
	}
	agentBlocked := blocked("agent", agent, "agent", false)
	squadBlocked := blocked("squad", squad, "agent", false)
	memberBlocked := blocked("agent", agent, "member", false)
	runningBlocked := blocked("agent", agent, "agent", true)
	queuedBlocked := blocked("agent", agent, "agent", false)
	backfillTask(t, fx, agent, runtime, testutil.Cols{"issue_id": queuedBlocked, "status": "queued", "created_at": now})
	tx := applyTodoBackfill(t, fx)
	wantBackfilled(t, tx, fx, "run_failed", failed, 1)
	wantBackfilled(t, tx, fx, "run_failed", stale, 0)
	wantBackfilled(t, tx, fx, "run_failed", unattributed, 0)
	wantBackfilled(t, tx, fx, "agent_blocked", agentBlocked, 1)
	wantBackfilled(t, tx, fx, "agent_blocked", squadBlocked, 1)
	wantBackfilled(t, tx, fx, "agent_blocked", memberBlocked, 0)
	wantBackfilled(t, tx, fx, "agent_blocked", runningBlocked, 0)
	wantBackfilled(t, tx, fx, "agent_blocked", queuedBlocked, 1)
}

func TestTodoBackfillMentionsAndArchives(t *testing.T) {
	fx, _ := todoFixture(t)
	issue := fx.Issue(t, "Mentioned")
	now := time.Now().UTC()
	comment := fx.Insert(t, "comment", testutil.Cols{"workspace_id": fx.WorkspaceID, "issue_id": issue, "author_type": "agent", "author_id": dbid.NewV7(), "content": "Please look", "created_at": now.Add(-time.Hour)})
	notice := backfillNotice(t, fx, "mentioned", map[string]string{"comment_id": comment}, testutil.Cols{"issue_id": issue, "created_at": now})
	backfillNotice(t, fx, "mentioned", map[string]string{"comment_id": comment}, testutil.Cols{"issue_id": issue, "created_at": now.Add(time.Minute)})
	descriptionIssue := fx.Issue(t, "Description mention")
	backfillNotice(t, fx, "mentioned", map[string]string{}, testutil.Cols{"issue_id": descriptionIssue})
	selfIssue := fx.Issue(t, "Self mention")
	backfillNotice(t, fx, "mentioned", map[string]string{}, testutil.Cols{"issue_id": selfIssue, "actor_type": "member", "actor_id": fx.UserID})
	repliedIssue := fx.Issue(t, "Replied")
	backfillNotice(t, fx, "mentioned", map[string]string{}, testutil.Cols{"issue_id": repliedIssue, "created_at": now.Add(-time.Hour)})
	fx.Insert(t, "comment", testutil.Cols{"workspace_id": fx.WorkspaceID, "issue_id": repliedIssue, "author_type": "member", "author_id": fx.UserID, "content": "Handled", "created_at": now})
	archivedIssue := fx.Issue(t, "Archived mention")
	backfillNotice(t, fx, "mentioned", map[string]string{}, testutil.Cols{"issue_id": archivedIssue, "archived": true})
	missing := dbid.NewV7().String()
	backfillNotice(t, fx, "mentioned", map[string]string{"comment_id": missing}, testutil.Cols{"issue_id": issue})
	kept := backfillNotice(t, fx, "new_comment", map[string]string{}, testutil.Cols{"issue_id": issue})
	for _, kind := range []string{"question_asked", "question_reassigned", "question_overdue", "task_failed", "autopilot_paused", "autopilot_quota_exceeded", "quick_create_failed", "quick_create_unconfirmed"} {
		backfillNotice(t, fx, kind, map[string]string{}, nil)
	}
	tx := applyTodoBackfill(t, fx)
	wantBackfilled(t, tx, fx, "mention", comment, 1)
	wantBackfilled(t, tx, fx, "mention", descriptionIssue, 1)
	for _, id := range []string{selfIssue, repliedIssue, archivedIssue, missing} {
		wantBackfilled(t, tx, fx, "mention", id, 0)
	}
	if got := backfillCount(t, tx, `SELECT count(*) FROM inbox_item WHERE workspace_id=$1 AND NOT archived AND id<>$2`, fx.WorkspaceID, kept); got != 0 {
		t.Errorf("%d moved notices remain unarchived", got)
	}
	if got := backfillCount(t, tx, `SELECT count(*) FROM inbox_item WHERE id=$1 AND NOT archived`, kept); got != 1 {
		t.Error("informational update was archived")
	}
	if err := tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := fx.Count(t, `SELECT count(*) FROM inbox_item WHERE id=$1 AND NOT archived`, notice); got != 1 {
		t.Error("rollback did not preserve original notice")
	}
}

func TestTodoBackfillSystemProblems(t *testing.T) {
	fx, _ := todoFixture(t)
	runtime := fx.Runtime(t, "System backfill runtime")
	agent := fx.Agent(t, "System backfill agent", runtime)
	now := time.Now().UTC().Truncate(time.Second)
	paused := fx.Insert(t, "autopilot", testutil.Cols{"workspace_id": fx.WorkspaceID, "title": "Paused autopilot", "assignee_type": "agent", "assignee_id": agent, "created_by_type": "member", "created_by_id": fx.UserID, "status": "paused"})
	active := fx.Insert(t, "autopilot", testutil.Cols{"workspace_id": fx.WorkspaceID, "title": "Resumed autopilot", "assignee_type": "agent", "assignee_id": agent, "created_by_type": "member", "created_by_id": fx.UserID, "status": "active"})
	for _, id := range []string{paused, active} {
		backfillNotice(t, fx, "autopilot_paused", map[string]string{"autopilot_id": id, "reason": "auto_paused_high_failure_rate"}, nil)
	}
	start, end := now.Add(-24*time.Hour), now.Add(24*time.Hour)
	fx.InsertNoID(t, "autopilot_quota_period", testutil.Cols{"workspace_id": fx.WorkspaceID, "period_start": start, "period_end": end}, "workspace_id=$1", fx.WorkspaceID)
	backfillNotice(t, fx, "autopilot_quota_exceeded", map[string]string{"reset_at": end.Format(time.RFC3339), "source": "schedule"}, testutil.Cols{"created_at": now})
	quick := func(kind string) string {
		task := backfillTask(t, fx, agent, runtime, testutil.Cols{"status": "failed", "context": `{"type":"quick_create","prompt":"Create issue"}`})
		backfillNotice(t, fx, kind, map[string]string{"task_id": task, "agent_id": agent, "original_prompt": "Create issue", "error": "Failure", "source_context_id": "context"}, nil)
		return task
	}
	failed := quick("quick_create_failed")
	unconfirmed := quick("quick_create_unconfirmed")
	retried := quick("quick_create_failed")
	retry := backfillTask(t, fx, agent, runtime, testutil.Cols{"status": "failed", "retry_of_task_id": retried})
	success := backfillTask(t, fx, agent, runtime, testutil.Cols{"status": "completed", "rerun_of_task_id": retry})
	fx.Issue(t, "Successful retry", testutil.Cols{"origin_type": "quick_create", "origin_id": success})
	tx := applyTodoBackfill(t, fx)
	wantBackfilled(t, tx, fx, "autopilot_paused", paused, 1)
	wantBackfilled(t, tx, fx, "autopilot_paused", active, 0)
	source := start.Format(time.RFC3339) + "/" + end.Format(time.RFC3339)
	wantBackfilled(t, tx, fx, "quota_reached", source, 1)
	wantBackfilled(t, tx, fx, "quick_create_failed", failed, 1)
	wantBackfilled(t, tx, fx, "quick_create_failed", unconfirmed, 1)
	wantBackfilled(t, tx, fx, "quick_create_failed", retried, 0)
	if got := backfillCount(t, tx, `SELECT count(*) FROM todo WHERE workspace_id=$1 AND kind='quick_create_failed' AND source_key=$2 AND title='Create issue' AND body='Explanation' AND details->>'reason'='unconfirmed' AND details->>'source_context_id'='context' AND actor_type IS NULL`, fx.WorkspaceID, unconfirmed); got != 1 {
		t.Error("unconfirmed quick create lost its action payload or neutral reason")
	}
	// An admitted run handles quota obligations, even across period boundaries.
	if _, err := tx.Exec(context.Background(), `INSERT INTO autopilot_quota_reservation (workspace_id, period_start, period_end, policy_revision, subscription_version, source, idempotency_key, created_at) VALUES ($1,$2,$3,1,1,'schedule','backfill-admitted',$4)`, fx.WorkspaceID, start, end, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(context.Background(), `DELETE FROM todo WHERE workspace_id=$1 AND kind='quota_reached'`, fx.WorkspaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(context.Background(), `UPDATE inbox_item SET archived=false WHERE workspace_id=$1 AND type='autopilot_quota_exceeded'`, fx.WorkspaceID); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../migrations/744_todo_backfill.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(context.Background(), string(migration)); err != nil {
		t.Fatal(err)
	}
	wantBackfilled(t, tx, fx, "quota_reached", source, 0)
}
