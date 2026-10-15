package handler

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// Replace the run while the handler renders attachments, before delivery
// selects the runs that receive receipts.
type replaceRunBeforeDelivery struct {
	db.DBTX
	replace func()
}

func (q *replaceRunBeforeDelivery) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	if strings.HasPrefix(sql, "-- name: ListAttachmentsByCommentIDs ") && q.replace != nil {
		replace := q.replace
		q.replace = nil
		replace()
	}
	return q.DBTX.Query(ctx, sql, args...)
}

func TestCommentDeliveryUsesTheRunsReachedAfterReplacement(t *testing.T) {
	for _, mode := range []string{"after_run", "chosen_run", "default"} {
		t.Run(mode, func(t *testing.T) {
			f := newSupplementFixture(t, "codex", "running", true)
			ctx := context.Background()
			issue, err := testHandler.Queries.GetIssue(ctx, parseUUID(f.issueID))
			if err != nil {
				t.Fatal(err)
			}
			commentID := dbfx.Comment(t, f.issueID, "Use the chosen run only")
			comment, err := testHandler.Queries.GetComment(ctx, parseUUID(commentID))
			if err != nil {
				t.Fatal(err)
			}
			var successor string
			q := &replaceRunBeforeDelivery{DBTX: testPool, replace: func() {
				dbfx.Exec(t, "UPDATE agent_task_queue SET status='completed', completed_at=now() WHERE id=$1", f.taskID)
				successor = dbfx.Task(t, f.agentID, testutil.Cols{
					"issue_id": f.issueID, "runtime_id": f.runtimeID, "status": "running",
				})
				dbfx.Exec(t, `INSERT INTO task_supplement_capability (task_id, workspace_id, issue_id, capability) VALUES ($1,$2,$3,$4)`, successor, testWorkspaceID, f.issueID, protocol.DaemonCapabilityTaskSupplementV1)
				dbfx.Cleanup(t, "DELETE FROM task_supplement_capability WHERE task_id=$1", successor)
				dbfx.Cleanup(t, "DELETE FROM task_supplement WHERE task_id=$1", successor)
			}}
			h := *testHandler
			h.Queries = db.New(q)
			var chosen []pgtype.UUID
			if mode == "after_run" {
				chosen = []pgtype.UUID{}
			} else if mode == "chosen_run" {
				chosen = []pgtype.UUID{parseUUID(f.taskID)}
			}
			triggers := []commentAgentTrigger{{Agent: db.Agent{ID: parseUUID(f.agentID)}}}
			kept, steered := h.steerCommentAgentTriggers(ctx, issue, comment,
				service.RunInputActor{Type: "member", ID: parseUUID(testUserID)}, triggers, chosen,
				service.RunInputItem{WorkspaceID: issue.WorkspaceID, Kind: "comment", ID: comment.ID, CommentID: comment.ID, Content: comment.Content})
			if successor == "" {
				t.Fatal("run replacement did not execute")
			}
			receipts := dbfx.Count(t, "SELECT count(*) FROM task_supplement WHERE task_id=$1 AND comment_id=$2", successor, commentID)
			if mode == "default" {
				if receipts != 1 || len(kept) != 0 || steered[f.agentID].status != DispatchSteered {
					t.Fatalf("default delivery: receipts=%d, kept=%+v, steered=%+v", receipts, kept, steered)
				}
			} else if receipts != 0 || len(kept) != 1 || len(steered) != 0 {
				t.Fatalf("explicit choice: receipts=%d, kept=%+v, steered=%+v", receipts, kept, steered)
			}
		})
	}
}
