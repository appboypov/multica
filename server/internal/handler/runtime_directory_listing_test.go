package handler

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestDirectoryListingOwnerRoundTrip(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	runtimeID := dbfx.Runtime(t, "directory listing", testutil.Cols{"runtime_mode": "local", "provider": "omp", "daemon_id": "directory-listing-daemon"})
	h := *testHandler
	h.DirectoryListingStore = NewInMemoryDirectoryListingStore()
	notifier := &runtimeLocalSkillPendingWorkRecorder{}
	h.DaemonPendingWork = notifier
	path := "/api/runtimes/" + runtimeID + "/directory-listings"
	var initiated RuntimeDirectoryListingRequest
	testutil.Call(t, h.InitiateDirectoryListing, withURLParam(newRequestAsUser(testUserID, http.MethodPost, path, map[string]string{"path": "~/notes"}), "runtimeId", runtimeID)).Want(http.StatusOK).JSON(&initiated)
	if initiated.ID == "" || initiated.RuntimeID != runtimeID || initiated.Path != "~/notes" || initiated.Status != RuntimeLocalSkillPending || initiated.Entries == nil {
		t.Fatalf("initiated = %#v", initiated)
	}
	if !reflect.DeepEqual(notifier.hints, []string{runtimeID + ":" + protocol.PendingWorkKindDirectoryListing}) {
		t.Fatalf("wakeup = %v", notifier.hints)
	}
	poll := func() RuntimeDirectoryListingRequest {
		var out RuntimeDirectoryListingRequest
		testutil.Call(t, h.GetDirectoryListingRequest, withURLParams(newRequestAsUser(testUserID, http.MethodGet, path+"/"+initiated.ID, nil), "runtimeId", runtimeID, "requestId", initiated.ID)).Want(http.StatusOK).JSON(&out)
		return out
	}
	if got := poll(); got.Status != RuntimeLocalSkillPending {
		t.Fatalf("pending poll = %#v", got)
	}
	entries := []RuntimeDirectoryEntry{{Name: ".notes", Path: "/home/notes/.notes", Kind: "folder", Hidden: true}, {Name: "a.md", Path: "/home/notes/a.md", Kind: "file"}}
	report := func(body any, workspace string, status int) {
		req := withURLParams(newDaemonTokenRequest(http.MethodPost, "/api/daemon/runtimes/"+runtimeID+"/directory-listings/"+initiated.ID+"/result", body, workspace, "directory-listing-daemon"), "runtimeId", runtimeID, "requestId", initiated.ID)
		testutil.Call(t, h.ReportDirectoryListingResult, req).Want(status)
	}
	report(map[string]any{"status": "completed", "path": "/home/notes", "entries": entries}, uuid.NewString(), http.StatusNotFound)
	if got := poll(); got.Status != RuntimeLocalSkillPending {
		t.Fatalf("unauthorized report changed request: %#v", got)
	}
	report(map[string]any{"status": "completed", "path": "/home/notes", "entries": entries}, testWorkspaceID, http.StatusOK)
	if got := poll(); got.Status != RuntimeLocalSkillCompleted || got.Path != "/home/notes" || got.ResolvedPath != "/home/notes" || !reflect.DeepEqual(got.Entries, entries) {
		t.Fatalf("completed poll = %#v", got)
	}
	report(map[string]string{"status": "failed", "error": "late report"}, testWorkspaceID, http.StatusOK)
	if got := poll(); got.Status != RuntimeLocalSkillCompleted || got.Error != "" {
		t.Fatalf("stale report = %#v", got)
	}

	failed, err := h.DirectoryListingStore.Create(context.Background(), runtimeID, "/missing")
	if err != nil {
		t.Fatal(err)
	}
	initiated = *failed
	report(map[string]string{"status": "failed", "error": "path does not exist: /missing"}, testWorkspaceID, http.StatusOK)
	if got := poll(); got.Status != RuntimeLocalSkillFailed || got.Error != "path does not exist: /missing" {
		t.Fatalf("failed poll = %#v", got)
	}

	var bare RuntimeDirectoryListingRequest
	testutil.Call(t, h.InitiateDirectoryListing, withURLParam(newRequestAsUser(testUserID, http.MethodPost, path, nil), "runtimeId", runtimeID)).Want(http.StatusOK).JSON(&bare)
	if bare.Path != "" || bare.Status != RuntimeLocalSkillPending {
		t.Fatalf("bare request = %#v", bare)
	}
}

func TestDirectoryListingOwnerAndOnlineGates(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	memberID := dbfx.User(t, "directory listing member", "directory-listing-"+uuid.NewString()+"@example.com")
	dbfx.Member(t, testWorkspaceID, memberID, "member")
	runtimeID := dbfx.Runtime(t, "public directory listing", testutil.Cols{"runtime_mode": "local", "provider": "omp", "visibility": "public"})
	h := *testHandler
	store := NewInMemoryDirectoryListingStore()
	h.DirectoryListingStore = store
	notifier := &runtimeLocalSkillPendingWorkRecorder{}
	h.DaemonPendingWork = notifier
	path := "/api/runtimes/" + runtimeID + "/directory-listings"
	testutil.Call(t, h.InitiateDirectoryListing, withURLParam(newRequestAsUser(memberID, http.MethodPost, path, map[string]string{"path": "/private"}), "runtimeId", runtimeID)).Want(http.StatusForbidden)
	if pending, err := store.HasPending(context.Background(), runtimeID); err != nil || pending || len(notifier.hints) != 0 {
		t.Fatalf("unauthorized listing queued: pending=%v error=%v wakeups=%v", pending, err, notifier.hints)
	}
	owned, err := store.Create(context.Background(), runtimeID, "/private")
	if err != nil {
		t.Fatal(err)
	}
	testutil.Call(t, h.GetDirectoryListingRequest, withURLParams(newRequestAsUser(memberID, http.MethodGet, path+"/"+owned.ID, nil), "runtimeId", runtimeID, "requestId", owned.ID)).Want(http.StatusForbidden)
	otherRuntimeID := dbfx.Runtime(t, "other directory listing", testutil.Cols{"runtime_mode": "local", "provider": "omp"})
	testutil.Call(t, h.GetDirectoryListingRequest, withURLParams(newRequestAsUser(testUserID, http.MethodGet, path+"/"+owned.ID, nil), "runtimeId", otherRuntimeID, "requestId", owned.ID)).Want(http.StatusNotFound)
	testutil.Call(t, h.ReportDirectoryListingResult, withURLParams(newDaemonTokenRequest(http.MethodPost, path+"/"+owned.ID+"/result", map[string]string{"status": "completed"}, testWorkspaceID, "directory-listing-daemon"), "runtimeId", otherRuntimeID, "requestId", owned.ID)).Want(http.StatusNotFound)
	dbfx.Exec(t, `UPDATE agent_runtime SET status = 'offline' WHERE id = $1`, runtimeID)
	before := len(store.requests)
	testutil.Call(t, h.InitiateDirectoryListing, withURLParam(newRequestAsUser(testUserID, http.MethodPost, path, map[string]string{"path": ""}), "runtimeId", runtimeID)).Want(http.StatusServiceUnavailable)
	testutil.Call(t, h.GetDirectoryListingRequest, withURLParams(newRequestAsUser(testUserID, http.MethodGet, path+"/"+owned.ID, nil), "runtimeId", runtimeID, "requestId", owned.ID)).Want(http.StatusServiceUnavailable)
	if len(store.requests) != before || len(notifier.hints) != 0 {
		t.Fatalf("offline listing queued: requests=%d wakeups=%v", len(store.requests), notifier.hints)
	}
}
