package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestCreateIssueRunOriginatorIsCreator(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	ctx := context.Background()
	var agentID, runtimeID string
	if err := testPool.QueryRow(ctx, `SELECT id, runtime_id FROM agent WHERE workspace_id=$1 AND name='Handler Test Agent'`, testWorkspaceID).Scan(&agentID, &runtimeID); err != nil {
		t.Fatal(err)
	}
	// The run's originator is deliberately not the authenticated daemon owner.
	originator := dbfx.User(t, "Issue requester", "creator-"+dbid.NewV7().String()+"@example.test")
	dbfx.Member(t, testWorkspaceID, originator, "member")
	for _, tc := range []struct {
		name       string
		quick      bool
		originator any
	}{
		{"agent in a run", false, originator},
		{"quick create", true, originator},
		{"agent without originator", false, nil},
		{"quick create without originator", true, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			contextJSON, err := json.Marshal(service.QuickCreateContext{
				Type: service.QuickCreateContextType, WorkspaceID: testWorkspaceID, RequesterID: originator,
			})
			if err != nil {
				t.Fatal(err)
			}
			task := dbfx.Task(t, agentID, testutil.Cols{
				"runtime_id": runtimeID, "status": "running", "originator_user_id": tc.originator,
				"accountable_user_id": originator, "context": contextJSON,
			})
			h := *testHandler
			h.Bus = events.New()
			issueService := *testHandler.IssueService
			issueService.Bus = h.Bus
			h.IssueService = &issueService
			var createdEvent *events.Event
			h.Bus.Subscribe(protocol.EventIssueCreated, func(event events.Event) { createdEvent = &event })
			body := map[string]any{"title": "Creator " + tc.name + " " + dbid.NewV7().String()}
			wantOrigin := "agent_create"
			if tc.quick {
				body["origin_type"], body["origin_id"] = "quick_create", task
				wantOrigin = "quick_create"
			}
			req := newRequest(http.MethodPost, "/api/issues?workspace_id="+testWorkspaceID, body)
			req.Header.Set("X-Agent-ID", agentID)
			req.Header.Set("X-Task-ID", task)
			var created IssueResponse
			testutil.Call(t, h.CreateIssue, req).Want(http.StatusCreated).JSON(&created)
			t.Cleanup(func() { deleteTestIssue(t, created.ID) })
			wantType, wantID := "member", originator
			if tc.originator == nil {
				wantType, wantID = "agent", agentID
			}
			if created.CreatorType != wantType || created.CreatorID != wantID {
				t.Fatalf("creator = %s/%s, want %s/%s", created.CreatorType, created.CreatorID, wantType, wantID)
			}
			var creatorType, creatorID, originType, originID string
			if err := testPool.QueryRow(ctx, `SELECT creator_type, creator_id, origin_type, origin_id FROM issue WHERE id=$1`, created.ID).Scan(&creatorType, &creatorID, &originType, &originID); err != nil {
				t.Fatal(err)
			}
			if creatorType != wantType || creatorID != wantID || originType != wantOrigin || originID != task {
				t.Fatalf("persisted creator/origin = %s/%s %s/%s", creatorType, creatorID, originType, originID)
			}
			if createdEvent == nil || createdEvent.ActorType != "agent" || createdEvent.ActorID != agentID {
				t.Fatalf("issue created actor = %+v, want filing agent %s", createdEvent, agentID)
			}
			payload := createdEvent.Payload.(map[string]any)["issue"].(IssueResponse)
			if payload.CreatorType != wantType || payload.CreatorID != wantID {
				t.Fatalf("event creator = %s/%s, want %s/%s", payload.CreatorType, payload.CreatorID, wantType, wantID)
			}
		})
	}
}
