package handler

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestWorklaneRequestRuntimeAccess(t *testing.T) {
	for _, tc := range []struct {
		name        string
		role        string
		ownsRuntime bool
		allowed     bool
	}{
		{"workspace admin", "admin", false, true},
		{"workspace owner", "owner", false, true},
		{"runtime owner", "member", true, true},
		{"unrelated member", "member", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			caller := dbfx.User(t, tc.name, uuid.NewString()+"@multica.test")
			dbfx.Member(t, testWorkspaceID, caller, tc.role)
			owner := testUserID
			if tc.ownsRuntime {
				owner = caller
			}
			daemon := uuid.NewString()
			runtime := dbfx.Runtime(t, "Worklane machine", testutil.Cols{
				"daemon_id": daemon, "runtime_mode": "local", "owner_id": owner,
				"metadata": testutil.Raw(`'{"capabilities":["worklane-worktree-v1"]}'::jsonb`),
			})
			project := dbfx.Project(t, "Worklane access")
			repo := dbfx.Insert(t, "repository", testutil.Cols{
				"workspace_id": testWorkspaceID, "project_id": project, "github_repo_id": 1,
				"full_name": "test/" + uuid.NewString(), "private": true, "default_branch": "main",
				"checkout_daemon_id": daemon, "checkout_path": "/test", "install_command": "pnpm install",
				"version_policy": "none", "last_tag": "",
			})
			h := *testHandler
			store := NewInMemoryWorklaneRequestStore()
			h.WorklaneRequestStore = store
			call := func(handler http.HandlerFunc, r *http.Request) *testutil.Response {
				return testutil.Call(t, func(w http.ResponseWriter, r *http.Request) {
					middleware.RequireWorkspaceMember(h.Queries)(handler).ServeHTTP(w, r)
				}, r)
			}
			want := http.StatusForbidden
			if tc.allowed {
				want = http.StatusCreated
			}
			var created WorklaneRequest
			response := call(h.CreateWorklaneRequest, newRequestAs(caller, http.MethodPost, "/api/worklane-requests", map[string]string{
				"repository_id": repo, "name": "Probe lane", "folder": "probe-lane", "daemon_id": daemon,
			})).Want(want)
			if tc.allowed {
				response.JSON(&created)
				if created.ID == "" || created.RuntimeID != runtime || created.CreatedBy == nil || *created.CreatedBy != caller {
					t.Fatalf("created request = %+v", created)
				}
				pending, err := store.PopPending(ctx, runtime)
				if err != nil || pending == nil || pending.ID != created.ID {
					t.Fatalf("queued request = %+v, error = %v", pending, err)
				}
			}
			failed := &WorklaneRequest{
				ID: uuid.NewString(), Kind: protocol.WorklaneRequestKindCreate,
				WorkspaceID: testWorkspaceID, RepositoryID: repo, DaemonID: daemon,
				Status: worklaneRequestFailed, UpdatedAt: time.Now(),
				Steps: []WorklaneRequestStep{{Key: worklaneStepFetch, State: worklaneStepFailed}},
			}
			if err := store.Save(ctx, failed); err != nil {
				t.Fatal(err)
			}
			want = http.StatusForbidden
			if tc.allowed {
				want = http.StatusOK
			}
			call(h.RetryWorklaneRequest, withURLParam(newRequestAs(caller, http.MethodPost,
				"/api/worklane-requests/"+failed.ID+"/retry", nil), "id", failed.ID)).Want(want)
			pending, err := store.HasPending(ctx, runtime)
			if err != nil || pending != tc.allowed {
				t.Fatalf("pending work = %v, error = %v; allowed = %v", pending, err, tc.allowed)
			}
		})
	}
}
