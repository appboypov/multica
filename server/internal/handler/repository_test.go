package handler

import (
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestUpdateRepository_WorkInCheckoutRoundTrip(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	pemBytes, _ := generateTestRSAKeyPEM(t)
	t.Setenv("GITHUB_APP_ID", "424242")
	t.Setenv("GITHUB_APP_PRIVATE_KEY", string(pemBytes))
	name := uuid.NewString()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/access_tokens"):
			writeJSON(w, http.StatusCreated, map[string]any{"token": "installation-secret"})
		case r.Method == http.MethodGet && r.URL.Path == "/repos/test/"+name:
			writeJSON(w, http.StatusOK, map[string]any{"id": 1, "full_name": "test/" + name, "private": true, "default_branch": "main"})
		case r.Method == http.MethodDelete && r.URL.Path == "/installation/token":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	oldBase := githubAPIBase
	githubAPIBase = srv.URL
	t.Cleanup(func() { githubAPIBase = oldBase })

	dbfx.Insert(t, "github_installation", testutil.Cols{
		"workspace_id": testWorkspaceID, "installation_id": rand.Int64N(1 << 40), "account_login": "test",
	})
	repoID := dbfx.Insert(t, "repository", testutil.Cols{
		"workspace_id": testWorkspaceID, "github_repo_id": 1, "full_name": "test/" + name, "private": true,
		"default_branch": "main", "checkout_daemon_id": uuid.NewString(), "checkout_path": "/test/checkout",
		"version_policy": "none",
	})
	for _, want := range []bool{true, false} {
		var out RepositoryResponse
		req := withURLParam(newRequest(http.MethodPut, "/api/repositories/"+repoID, map[string]any{"work_in_checkout": want}), "id", repoID)
		testutil.Call(testHandler.UpdateRepository, req).Want(http.StatusOK).JSON(&out)
		var stored bool
		dbfx.QueryRow(t, "SELECT work_in_checkout FROM repository WHERE id = $1", repoID).Scan(&stored)
		if out.WorkInCheckout != want || stored != want {
			t.Fatalf("work_in_checkout = %v, stored %v, want %v", out.WorkInCheckout, stored, want)
		}
	}
}
