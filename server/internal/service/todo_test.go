package service

import (
	"context"
	"errors"
	"testing"

	"github.com/multica-ai/multica/server/internal/entitlement"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/todo"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestTodoQuotaRejectionAndAdmission(t *testing.T) {
	f := newAutopilotQuotaFixture(t, entitlement.ActionEnforce, 0)
	ctx := context.Background()
	t.Cleanup(func() { f.pool.Exec(ctx, "DELETE FROM todo WHERE workspace_id = $1", f.workspaceID) })
	f.setNotificationPolicy(0)
	opened, closed := 0, 0
	f.service.Bus.Subscribe(protocol.EventTodoOpened, func(events.Event) { opened++ })
	f.service.Bus.Subscribe(protocol.EventTodoClosed, func(events.Event) { closed++ })
	for _, key := range []string{"first", "second"} {
		_, _, err := f.service.createAutopilotRunWithQuota(ctx, f.workspaceID, "api", key, f.createRunArgs)
		var quota *AutopilotQuotaExceededError
		if !errors.As(err, &quota) {
			t.Fatalf("rejection: %v", err)
		}
	}
	if opened != 1 {
		t.Fatalf("opened=%d, want one manager todo", opened)
	}
	// A stale previous period must close too, not just the currently admitted one.
	if err := (todo.Service{Queries: f.queries, Bus: f.service.Bus}).Open(ctx, db.OpenTodoParams{WorkspaceID: f.workspaceID, MemberID: f.publisherID, Kind: todo.QuotaReached, SourceKey: "old-period", Title: "Old quota"}); err != nil {
		t.Fatal(err)
	}
	f.setPolicy(entitlement.ActionEnforce, f.periodStart, f.periodEnd, 5)
	if _, _, err := f.service.createAutopilotRunWithQuota(ctx, f.workspaceID, "api", "admitted", f.createRunArgs); err != nil {
		t.Fatal(err)
	}
	rows, err := f.queries.ListOpenTodos(ctx, db.ListOpenTodosParams{WorkspaceID: f.workspaceID, MemberID: f.publisherID})
	if err != nil || len(rows) != 0 || closed != 2 {
		t.Fatalf("admission: rows=%v closed=%d error=%v", rows, closed, err)
	}
}

func TestTodoQuickCreateOutcomesAndSuccessfulRetry(t *testing.T) {
	f := newAutopilotQuotaFixture(t, entitlement.ActionOff, 0)
	ctx := context.Background()
	fx := testutil.New(f.pool, util.UUIDToString(f.workspaceID), util.UUIDToString(f.publisherID))
	fx.Cleanup(t, "DELETE FROM todo WHERE workspace_id = $1", f.workspaceID)
	svc := &TaskService{Queries: f.queries, Bus: events.New()}
	qc := QuickCreateContext{Type: QuickCreateContextType, WorkspaceID: fx.WorkspaceID, RequesterID: fx.UserID, Prompt: "Fix login"}
	runtime := fx.Runtime(t, "Quick create runtime")
	parentID := fx.Task(t, util.UUIDToString(f.agentID), testutil.Cols{"runtime_id": runtime, "status": "failed"})
	parent, err := f.queries.GetAgentTask(ctx, util.MustParseUUID(parentID))
	if err != nil {
		t.Fatal(err)
	}
	svc.notifyQuickCreateFailed(ctx, parent, qc, "Agent failed")
	rows, err := f.queries.ListOpenTodos(ctx, db.ListOpenTodosParams{WorkspaceID: f.workspaceID, MemberID: f.publisherID})
	if err != nil || len(rows) != 1 || rows[0].Title != qc.Prompt || rows[0].ActorID.Valid {
		t.Fatalf("failure rows=%v error=%v", rows, err)
	}
	retryID := fx.Task(t, util.UUIDToString(f.agentID), testutil.Cols{"runtime_id": runtime, "status": "failed", "rerun_of_task_id": parentID})
	retry, err := f.queries.GetAgentTask(ctx, util.MustParseUUID(retryID))
	if err != nil {
		t.Fatal(err)
	}
	svc.notifyQuickCreateUnconfirmed(ctx, retry, qc)
	rows, err = f.queries.ListOpenTodos(ctx, db.ListOpenTodosParams{WorkspaceID: f.workspaceID, MemberID: f.publisherID})
	if err != nil || len(rows) != 2 {
		t.Fatalf("unconfirmed rows=%v error=%v", rows, err)
	}
	fx.Issue(t, "Login fixed", testutil.Cols{"origin_type": "quick_create", "origin_id": retryID})
	svc.notifyQuickCreateCompleted(ctx, retry, qc, nil)
	rows, err = f.queries.ListOpenTodos(ctx, db.ListOpenTodosParams{WorkspaceID: f.workspaceID, MemberID: f.publisherID})
	if err != nil || len(rows) != 0 {
		t.Fatalf("successful retry rows=%v error=%v", rows, err)
	}
}
