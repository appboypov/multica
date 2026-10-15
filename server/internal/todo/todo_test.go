package todo

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestOpenAndClosePublishOnlyTransitions(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	fx := testutil.New(pool, "", "")
	suffix := dbid.NewV7().String()
	fx.UserID = fx.User(t, "Todo member", "todo-"+suffix+"@example.test")
	fx.WorkspaceID = fx.Workspace(t, "Todos", "todos-"+suffix)
	fx.Member(t, fx.WorkspaceID, fx.UserID, "owner")
	fx.Cleanup(t, "DELETE FROM todo WHERE workspace_id = $1", fx.WorkspaceID)
	workspace, _ := util.ParseUUID(fx.WorkspaceID)
	member, _ := util.ParseUUID(fx.UserID)
	bus := events.New()
	opened, closed := 0, 0
	bus.Subscribe(protocol.EventTodoOpened, func(e events.Event) {
		opened++
		payload := e.Payload.(map[string]any)
		if payload["member_id"] != fx.UserID {
			t.Fatalf("wrong recipient: %v", payload)
		}
		row := payload["todo"].(map[string]any)
		if row["kind"] != QuotaReached || row["issue_id"] != nil || row["overdue"] != false {
			t.Fatalf("wrong projection: %v", row)
		}
	})
	bus.Subscribe(protocol.EventTodoClosed, func(e events.Event) {
		closed++
		if e.Payload.(map[string]any)["done_by"] != "handled" {
			t.Fatal("wrong close reason")
		}
	})
	svc := Service{Queries: db.New(pool), Bus: bus}
	p := db.OpenTodoParams{WorkspaceID: workspace, MemberID: member, Kind: QuotaReached, SourceKey: "period", Title: "Quota"}
	for range 2 {
		if err := svc.Open(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	if opened != 1 || fx.Count(t, "SELECT count(*) FROM todo WHERE workspace_id = $1", workspace) != 1 {
		t.Fatalf("duplicate open: %d events", opened)
	}
	for range 2 {
		if err := svc.Close(ctx, db.CloseTodosParams{WorkspaceID: workspace, MemberID: pgtype.UUID{}, Kind: p.Kind, SourceKey: p.SourceKey}); err != nil {
			t.Fatal(err)
		}
	}
	if closed != 1 {
		t.Fatalf("duplicate close: %d events", closed)
	}
}
