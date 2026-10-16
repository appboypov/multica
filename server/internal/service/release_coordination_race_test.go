package service

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestReleaseCoordinationSourceRollbackLeavesNoReceipt(t *testing.T) {
	f, _, rel, _ := releaseCoordinationFixture(t)
	issue := f.Issue(t, "Rollback terminal", testutil.Cols{"release_id": rel.ID, "status": "in_progress"})
	ctx := context.Background()
	tx, err := f.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	q := f.q.WithTx(tx)
	if _, err = q.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{ID: parseTestUUID(t, issue), WorkspaceID: rel.WorkspaceID, Status: "done"}); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if n := f.Count(t, "SELECT count(*) FROM issue_wakeup_receipt WHERE payload->>'issue_id'=$1 AND event_type='release.issue.ended'", issue); n != 0 {
		t.Fatalf("rolled-back transition retained %d receipts", n)
	}
}

func TestReleaseDispatchCannotPassRepositoryTeardownBarrier(t *testing.T) {
	f, s, rel, _ := releaseCoordinationFixture(t)
	ctx := context.Background()
	rule, err := f.q.GetReleaseCoordinationWakeup(ctx, rel.ID)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := f.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	q := f.q.WithTx(tx)
	if _, err = q.LockReleaseLineOwners(ctx, db.LockReleaseLineOwnersParams{ID: rel.RepositoryID, WorkspaceID: rel.WorkspaceID}); err != nil {
		t.Fatal(err)
	}
	if _, err = q.LockRepositoryForLine(ctx, db.LockRepositoryForLineParams{ID: rel.RepositoryID, WorkspaceID: rel.WorkspaceID}); err != nil {
		t.Fatal(err)
	}
	// The dispatcher uses a bounded lock attempt, not a timer-based race. This
	// foreground call must lose while teardown owns the repository boundary.
	if err = s.dispatchRelease(ctx, rule); err == nil {
		t.Fatal("dispatch bypassed repository fence")
	}
	if _, err = q.CancelUnstartedReleaseTasks(ctx, db.CancelUnstartedReleaseTasksParams{WorkspaceID: rel.WorkspaceID, ReleaseIds: []pgtype.UUID{rel.ID}}); err != nil {
		t.Fatal(err)
	}
	if err = q.DeleteReleaseWakeups(ctx, db.DeleteReleaseWakeupsParams{WorkspaceID: rel.WorkspaceID, ReleaseIds: []pgtype.UUID{rel.ID}}); err != nil {
		t.Fatal(err)
	}
	if err = q.DeleteRelease(ctx, db.DeleteReleaseParams{ID: rel.ID, WorkspaceID: rel.WorkspaceID}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.dispatchRelease(ctx, rule); err != nil {
		t.Fatal(err)
	}
	if n := f.Count(t, "SELECT count(*) FROM agent_task_queue WHERE release_id=$1 AND status IN ('queued','dispatched','running')", rel.ID); n != 0 {
		t.Fatalf("teardown left %d runnable tasks", n)
	}
}

func TestReleaseOwnerABARejectsPreviouslyDispatchedStart(t *testing.T) {
	f, s, rel, agent := releaseCoordinationFixture(t)
	ctx := context.Background()
	if err := s.ProcessReleaseCoordination(ctx, rel.ID); err != nil {
		t.Fatal(err)
	}
	task, err := f.q.FindPendingReleaseTask(ctx, rel.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.Exec(t, "UPDATE agent_task_queue SET status='dispatched',dispatched_at=now() WHERE id=$1", task.ID)
	other := f.privateAgentOwnedBy(t, f.UserID, "replacement")
	for _, owner := range []string{other, agent} {
		tx, err := f.Pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		q := f.q.WithTx(tx)
		if _, err = q.LockReleaseLineOwners(ctx, db.LockReleaseLineOwnersParams{ID: rel.RepositoryID, WorkspaceID: rel.WorkspaceID}); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if _, err = q.LockRepositoryForLine(ctx, db.LockRepositoryForLineParams{ID: rel.RepositoryID, WorkspaceID: rel.WorkspaceID}); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if _, err = q.UpdateReleaseOwner(ctx, db.UpdateReleaseOwnerParams{ID: rel.ID, WorkspaceID: rel.WorkspaceID, OwnerType: pgtype.Text{String: "agent", Valid: true}, OwnerID: parseTestUUID(t, owner)}); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if _, err = q.CancelUnstartedReleaseTasks(ctx, db.CancelUnstartedReleaseTasksParams{WorkspaceID: rel.WorkspaceID, ReleaseIds: []pgtype.UUID{rel.ID}}); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	started, err := s.Tasks.StartTask(ctx, task.ID)
	if err == nil && started != nil {
		t.Fatalf("A-B-A restarted cancelled dispatched task: %+v", started)
	}
	if n := f.Count(t, "SELECT count(*) FROM agent_task_queue WHERE id=$1 AND started_at IS NOT NULL", task.ID); n != 0 {
		t.Fatal("stale task acquired a start")
	}
}

func TestReleaseSquadLeaderReplacementRejectsOldLeaderStart(t *testing.T) {
	f, s, rel, agent := releaseCoordinationFixture(t)
	ctx := context.Background()
	squad := f.Squad(t, "Release leadership", agent)
	f.Exec(t, "UPDATE release SET owner_type='squad',owner_id=$2 WHERE id=$1", rel.ID, squad)
	if err := s.ProcessReleaseCoordination(ctx, rel.ID); err != nil {
		t.Fatal(err)
	}
	task, err := f.q.FindPendingReleaseTask(ctx, rel.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.Exec(t, "UPDATE agent_task_queue SET status='dispatched',dispatched_at=now() WHERE id=$1", task.ID)
	replacement := f.privateAgentOwnedBy(t, f.UserID, "new-leader")
	tx, err := f.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "UPDATE squad SET leader_id=$2 WHERE id=$1", squad, replacement); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	started, err := s.Tasks.StartTask(ctx, task.ID)
	if err == nil && started != nil {
		t.Fatal("former squad leader started release coordination")
	}
	if n := f.Count(t, "SELECT count(*) FROM agent_task_queue WHERE id=$1 AND started_at IS NOT NULL", task.ID); n != 0 {
		t.Fatal("stale leader acquired a start")
	}
}

func TestReleaseCommentMergeCannotMutateClaimedInput(t *testing.T) {
	f, s, rel, _ := releaseCoordinationFixture(t)
	ctx := context.Background()
	if err := s.ProcessReleaseCoordination(ctx, rel.ID); err != nil {
		t.Fatal(err)
	}
	task, err := f.q.FindPendingReleaseTask(ctx, rel.ID)
	if err != nil {
		t.Fatal(err)
	}
	comment := f.Insert(t, "comment", testutil.Cols{"workspace_id": f.WorkspaceID, "release_id": rel.ID, "author_type": "member", "author_id": f.UserID, "content": "Claim race input"})
	rule, err := f.q.GetReleaseCoordinationWakeup(ctx, rel.ID)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := f.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT id FROM agent_task_queue WHERE id=$1 FOR UPDATE", task.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.dispatchRelease(ctx, rule); err == nil {
		t.Fatal("merge passed the claim task barrier")
	}
	if _, err = tx.Exec(ctx, "UPDATE agent_task_queue SET status='dispatched',dispatched_at=now() WHERE id=$1", task.ID); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.dispatchRelease(ctx, rule); err != nil {
		t.Fatal(err)
	}
	current, err := f.q.GetAgentTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(current.Context) != string(task.Context) || current.TriggerCommentID != task.TriggerCommentID {
		t.Fatal("claim input changed")
	}
	if n := f.Count(t, "SELECT count(*) FROM issue_wakeup_receipt WHERE payload->>'comment_id'=$1 AND task_id IS NULL AND processed_at IS NULL", comment); n != 1 {
		t.Fatalf("late comment has %d pending receipts", n)
	}
}

func TestReleaseTerminalCaptureCannotRecreateShippedCoordination(t *testing.T) {
	f, s, rel, _ := releaseCoordinationFixture(t)
	ctx := context.Background()
	issue := f.Issue(t, "Ship capture race", testutil.Cols{"release_id": rel.ID, "status": "in_progress"})
	tx, err := f.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	q := f.q.WithTx(tx)
	if _, err = q.LockReleaseLineOwners(ctx, db.LockReleaseLineOwnersParams{ID: rel.RepositoryID, WorkspaceID: rel.WorkspaceID}); err != nil {
		t.Fatal(err)
	}
	if _, err = q.LockRepositoryForLine(ctx, db.LockRepositoryForLineParams{ID: rel.RepositoryID, WorkspaceID: rel.WorkspaceID}); err != nil {
		t.Fatal(err)
	}
	if _, err = q.GetReleaseInWorkspaceForUpdate(ctx, db.GetReleaseInWorkspaceForUpdateParams{ID: rel.ID, WorkspaceID: rel.WorkspaceID}); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "UPDATE release SET shipped_at=now() WHERE id=$1", rel.ID); err != nil {
		t.Fatal(err)
	}
	if err = q.DeleteReleaseWakeups(ctx, db.DeleteReleaseWakeupsParams{WorkspaceID: rel.WorkspaceID, ReleaseIds: []pgtype.UUID{rel.ID}}); err != nil {
		t.Fatal(err)
	}
	source, err := f.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Rollback(ctx)
	if _, err = source.Exec(ctx, "SET LOCAL lock_timeout='50ms'"); err != nil {
		t.Fatal(err)
	}
	if _, err = source.Exec(ctx, "UPDATE issue SET status='done' WHERE id=$1", issue); err == nil {
		t.Fatal("terminal source bypassed release shipping fence")
	}
	if err = source.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = f.q.UpdateIssueStatus(ctx, db.UpdateIssueStatusParams{ID: parseTestUUID(t, issue), WorkspaceID: rel.WorkspaceID, Status: "done"}); err != nil {
		t.Fatal(err)
	}
	if err = s.ProcessReleaseCoordination(ctx, rel.ID); err != nil {
		t.Fatal(err)
	}
	if n := f.Count(t, "SELECT count(*) FROM issue_wakeup WHERE release_id=$1", rel.ID); n != 0 {
		t.Fatalf("terminal callback recreated %d shipped rules", n)
	}
	if n := f.Count(t, "SELECT count(*) FROM agent_task_queue WHERE release_id=$1", rel.ID); n != 0 {
		t.Fatal("shipped release restarted")
	}
}
