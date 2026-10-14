package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

// The dialog reads a finished check from the top level of the poll response
// (design.md §3). A nested result read as a failed check with no tag.
func TestGetRepositoryCheckout_FlattensTheResult(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}

	runtimeID := createRuntimeLocalSkillTestRuntime(t, testUserID)
	ctx := context.Background()
	req, err := testHandler.RepositoryCheckoutStore.Create(ctx, RuntimeRepositoryCheckoutRequest{
		RuntimeID:     runtimeID,
		WorkspaceID:   testWorkspaceID,
		FullName:      "appboypov/rebl-admin",
		DefaultBranch: "main",
		Path:          "~/Repos/rebl-admin",
	})
	if err != nil {
		t.Fatalf("create check: %v", err)
	}
	if err := testHandler.RepositoryCheckoutStore.Complete(ctx, req.ID, protocol.RepositoryCheckoutResult{
		Status:       "completed",
		ResolvedPath: "/home/brian/Repos/rebl-admin",
		Passed:       true,
		VersionFiles: []string{"package.json"},
		LastTag:      "v2.1.0",
	}); err != nil {
		t.Fatalf("complete check: %v", err)
	}

	w := httptest.NewRecorder()
	testHandler.GetRepositoryCheckout(w, withURLParams(
		newRequestAsUser(testUserID, http.MethodGet, "/api/runtimes/"+runtimeID+"/repository-checkouts/"+req.ID, nil),
		"runtimeId", runtimeID,
		"requestId", req.ID,
	))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["status"] != "completed" || got["passed"] != true || got["resolved_path"] != "/home/brian/Repos/rebl-admin" {
		t.Fatalf("check not flattened: %v", got)
	}
	if got["last_tag"] != "2.1.0" {
		t.Fatalf("last_tag = %v, want the bare version 2.1.0 a repository stores", got["last_tag"])
	}
	if files, _ := got["version_files"].([]any); len(files) != 1 || files[0] != "package.json" {
		t.Fatalf("version_files = %v", got["version_files"])
	}
	if failures, ok := got["failures"].([]any); !ok || len(failures) != 0 {
		t.Fatalf("failures = %v, want an empty list", got["failures"])
	}
}
