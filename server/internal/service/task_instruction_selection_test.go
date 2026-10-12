package service

import (
	"context"
	"testing"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestInstructionSelectionFailureNeverRetries(t *testing.T) {
	pool := newResolveOriginatorPool(t)
	ctx := context.Background()
	workspaceID, userID, agentID, issueID := seedAttributionFixture(t, pool)
	fx := testutil.New(pool, workspaceID, userID)
	var runtimeID string
	fx.QueryRow(t, `SELECT runtime_id::text FROM agent WHERE id=$1`, agentID).Scan(&runtimeID)
	taskID := fx.Task(t, agentID, testutil.Cols{
		"runtime_id": runtimeID, "issue_id": issueID, "status": "running", "attempt": 1, "max_attempts": 5,
	})
	bus := events.New()
	var failure events.Event
	var queued int
	bus.Subscribe(protocol.EventTaskFailed, func(event events.Event) { failure = event })
	bus.Subscribe(protocol.EventTaskQueued, func(events.Event) { queued++ })
	svc := NewTaskService(db.New(pool), pool, nil, bus)
	const reason = "instruction_selection_error"
	const message = "instruction selection: /notes/gone.md: does not exist"
	failed, err := svc.FailTask(ctx, util.MustParseUUID(taskID), message, "", "", "", reason, false, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if failed.Status != "failed" || failed.FailureReason.String != reason || failed.Error.String != message {
		t.Fatalf("failed task = %+v", failed)
	}
	if child, err := svc.MaybeRetryFailedTask(ctx, *failed); err != nil || child != nil {
		t.Fatalf("sweeper retry = %+v, %v; want no retry", child, err)
	}
	var retries int
	fx.QueryRow(t, `SELECT count(*) FROM agent_task_queue WHERE parent_task_id=$1`, taskID).Scan(&retries)
	if retries != 0 || queued != 0 {
		t.Fatalf("retry children = %d, queued events = %d; want zero", retries, queued)
	}
	payload, ok := failure.Payload.(map[string]any)
	if !ok || payload["failure_reason"] != reason || payload["retry_pending"] != false || payload["error"] != message {
		t.Fatalf("failure event = %+v", failure)
	}
}
