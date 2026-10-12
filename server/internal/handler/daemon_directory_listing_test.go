package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/daemonws"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestDirectoryListingHeartbeatDeliversOnce(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	for _, transport := range []string{"http", "websocket"} {
		t.Run(transport, func(t *testing.T) {
			ctx := context.Background()
			runtimeID := dbfx.Runtime(t, "directory listing heartbeat", testutil.Cols{"runtime_mode": "local", "provider": "omp"})
			h := *testHandler
			store := NewInMemoryDirectoryListingStore()
			h.DirectoryListingStore = store
			queued, err := store.Create(ctx, runtimeID, "~/notes")
			if err != nil {
				t.Fatal(err)
			}
			localSkills, err := h.LocalSkillListStore.Create(ctx, runtimeID)
			if err != nil {
				t.Fatal(err)
			}
			identity := daemonws.ClientIdentity{WorkspaceID: testWorkspaceID, RuntimeLeases: map[string]*daemonws.RuntimeLease{runtimeID: daemonws.NewRuntimeLease(testWorkspaceID, "online", time.Now(), true)}}
			heartbeat := func() *protocol.DaemonHeartbeatAckPayload {
				if transport == "websocket" {
					ack, err := h.HandleDaemonWSHeartbeat(ctx, identity, runtimeID, false)
					if err != nil {
						t.Fatal(err)
					}
					return ack
				}
				req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/heartbeat", map[string]string{"runtime_id": runtimeID}, testWorkspaceID, "directory-listing-daemon")
				out := testutil.Call(t, h.DaemonHeartbeat, req).Want(http.StatusOK)
				var ack protocol.DaemonHeartbeatAckPayload
				if err := json.Unmarshal(out.Body.Bytes(), &ack); err != nil {
					t.Fatal(err)
				}
				return &ack
			}
			ack := heartbeat()
			if ack.PendingDirectoryListing == nil || ack.PendingDirectoryListing.ID != queued.ID || ack.PendingDirectoryListing.Path != "~/notes" {
				t.Fatalf("listing ack = %#v", ack.PendingDirectoryListing)
			}
			if ack.PendingLocalSkills == nil || ack.PendingLocalSkills.ID != localSkills.ID {
				t.Fatalf("local skills lost from ack: %#v", ack.PendingLocalSkills)
			}
			got, err := store.Get(ctx, queued.ID)
			if err != nil || got.Status != RuntimeLocalSkillRunning {
				t.Fatalf("claimed listing = %#v, %v", got, err)
			}
			if ack := heartbeat(); ack.PendingDirectoryListing != nil {
				t.Fatalf("listing delivered twice: %#v", ack.PendingDirectoryListing)
			}
		})
	}
}
