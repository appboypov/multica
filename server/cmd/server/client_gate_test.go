package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/realtime"
	"github.com/multica-ai/multica/server/internal/testutil"
)

const routerClientGateError = `{"error":"client accounts can only use the session routes"}`

type clientGateFixture struct {
	router                         chi.Router
	rows                           *testutil.Fixture
	workspaceID, organisationID    string
	clientID, clientEmail, staffID string
	clientJWT, clientPAT, staffJWT string
}

func newClientGateFixture(t *testing.T) clientGateFixture {
	t.Helper()
	if testPool == nil {
		t.Skip("no database connection")
	}
	// Never inherit integrations or storage destinations from the developer's
	// environment. The route walk never executes resource handlers either.
	for _, name := range []string{
		"MULTICA_CLOUD_URL", "COMPOSIO_API_KEY", "S3_BUCKET", "CLOUDFRONT_KEY_PAIR_ID",
		"MULTICA_LARK_SECRET_KEY", "MULTICA_SLACK_SECRET_KEY", "MULTICA_DINGTALK_SECRET_KEY",
		"MULTICA_WECOM_SECRET_KEY", "MULTICA_TELEGRAM_SECRET_KEY", "MULTICA_VCS_SECRET_KEY",
		"MULTICA_PLUGIN_SECRET_KEY", "MULTICA_LLM_API_KEY", "MULTICA_LLM_BASE_URL",
	} {
		t.Setenv(name, "")
	}
	t.Setenv("LOCAL_UPLOAD_DIR", t.TempDir())
	rows := testutil.New(testPool, "", "")
	suffix := uuid.NewString()
	workspaceID := rows.Workspace(t, "Client gate test", "client-gate-"+suffix)
	staffID := rows.User(t, "Gate staff", "gate-staff-"+suffix+"@example.invalid")
	clientEmail := "gate-client-" + suffix + "@example.invalid"
	clientID := rows.User(t, "Gate client", clientEmail)
	rows.WorkspaceID, rows.UserID = workspaceID, staffID
	organisationID := rows.Insert(t, "organisation", testutil.Cols{
		"workspace_id": workspaceID, "name": "Gate organisation", "key": "gate-organisation",
	})
	rows.Member(t, workspaceID, staffID, "owner")
	rows.Member(t, workspaceID, clientID, "client", testutil.Cols{"organisation_ids": []string{organisationID}})
	clientJWT, err := generateTestJWT(clientID, clientEmail, "Gate client")
	if err != nil {
		t.Fatal(err)
	}
	staffJWT, err := generateTestJWT(staffID, "gate-staff-"+suffix+"@example.invalid", "Gate staff")
	if err != nil {
		t.Fatal(err)
	}
	clientPAT := "mul_gate_" + uuid.NewString()
	rows.Insert(t, "personal_access_token", testutil.Cols{
		"user_id": clientID, "name": "Gate PAT", "token_hash": auth.HashToken(clientPAT), "token_prefix": clientPAT[:12],
	})
	// POST /api/tokens below creates a second PAT owned only by this fixture.
	rows.Cleanup(t, "DELETE FROM personal_access_token WHERE user_id = $1", clientID)
	router, _ := NewRouterWithOptions(testPool, realtime.NewHub(), events.New(), analytics.NoopClient{}, nil, RouterOptions{})
	return clientGateFixture{
		router: router, rows: rows, workspaceID: workspaceID, organisationID: organisationID,
		clientID: clientID, clientEmail: clientEmail, staffID: staffID,
		clientJWT: clientJWT, clientPAT: clientPAT, staffJWT: staffJWT,
	}
}

// This independent, explicit table is the approved session contract. If the
// middleware opens another route, the walk must fail until that contract changes.
func routerClientSession(method, requestPath string) bool {
	for _, route := range []struct{ method, pattern string }{
		{"GET", "/api/me"}, {"POST", "/api/auth/refresh"},
		{"POST", "/api/cli-token"}, {"POST", "/api/tokens"},
		{"GET", "/api/me/memberships"}, {"GET", "/api/invitations"},
		{"GET", "/api/me/waits"}, {"GET", "/api/me/waits/*"},
		{"GET", "/api/me/planned-work"},
		{"GET", "/api/invitations/*"}, {"POST", "/api/invitations/*/accept"},
		{"POST", "/api/invitations/*/decline"},
	} {
		if method == route.method {
			if matched, _ := path.Match(route.pattern, requestPath); matched {
				return true
			}
		}
	}
	return false
}

func clientGateRouteRequest(method, route, token, workspaceID string) *http.Request {
	routeContext := chi.NewRouteContext()
	routeContext.RoutePatterns = []string{route}
	parts := strings.Split(route, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			name := strings.SplitN(strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}"), ":", 2)[0]
			parts[i] = uuid.NewString()
			routeContext.URLParams.Add(name, parts[i])
		} else if part == "*" {
			parts[i] = uuid.NewString()
			routeContext.URLParams.Add("*", parts[i])
		}
	}
	// Chi walks a subrouter's root as /prefix/, but also mounts /prefix.
	// Use the slashless mount so exact session paths such as /api/tokens
	// exercise the same URL the login clients send.
	requestPath := strings.TrimSuffix(strings.Join(parts, "/"), "/")
	if requestPath == "" {
		requestPath = "/"
	}
	req := httptest.NewRequest(method, requestPath, strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Workspace-ID", workspaceID)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeContext))
}

func TestClientGateRouterDefaultDeny(t *testing.T) {
	fixture := newClientGateFixture(t)
	protected, daemon, bridge, sessions := 0, 0, 0, 0
	err := chi.Walk(fixture.router, func(method, route string, _ http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		// Walk the real router's middleware stacks, but stop before resource
		// handlers: a staff DELETE/POST sweep must not mutate even test state or
		// contact a configured service. This also proves the gate runs first.
		h := chi.Chain(middlewares...).HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
		unauthenticated := httptest.NewRecorder()
		h.ServeHTTP(unauthenticated, clientGateRouteRequest(method, route, "", fixture.workspaceID))
		if unauthenticated.Code != http.StatusUnauthorized {
			return nil
		}
		protected++
		if strings.HasPrefix(route, "/api/daemon/") {
			daemon++
		}
		if strings.HasPrefix(route, "/api/plugin-bridge/") {
			bridge++
		}
		t.Run(method+" "+route, func(t *testing.T) {
			for _, credential := range []struct{ name, token string }{
				{"client JWT", fixture.clientJWT}, {"client PAT", fixture.clientPAT},
			} {
				req := clientGateRouteRequest(method, route, credential.token, fixture.workspaceID)
				w := httptest.NewRecorder()
				h.ServeHTTP(w, req)
				// Installation credentials are not user credentials. A client
				// JWT/PAT must be refused by the plugin API's own authentication.
				if strings.HasPrefix(route, "/v1/") {
					if w.Code != http.StatusUnauthorized {
						t.Errorf("%s plugin route: got %d: %s", credential.name, w.Code, w.Body.String())
					}
					continue
				}
				if routerClientSession(method, req.URL.Path) {
					if w.Code != 204 {
						t.Errorf("%s session route: got %d: %s", credential.name, w.Code, w.Body.String())
					}
				} else if w.Code != 403 || strings.TrimSpace(w.Body.String()) != routerClientGateError {
					t.Errorf("%s must reach gate refusal before handler: got %d: %s", credential.name, w.Code, w.Body.String())
				}
			}
			staff := httptest.NewRecorder()
			h.ServeHTTP(staff, clientGateRouteRequest(method, route, fixture.staffJWT, fixture.workspaceID))
			if strings.TrimSpace(staff.Body.String()) == routerClientGateError {
				t.Fatalf("staff received client gate refusal: %d: %s", staff.Code, staff.Body.String())
			}
		})
		if routerClientSession(method, clientGateRouteRequest(method, route, "", fixture.workspaceID).URL.Path) {
			sessions++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if protected == 0 || daemon == 0 || bridge == 0 || sessions != 12 {
		t.Fatalf("route coverage missing: protected=%d daemon=%d bridge=%d sessions=%d", protected, daemon, bridge, sessions)
	}
}

func TestClientGateRouterSessions(t *testing.T) {
	fixture := newClientGateFixture(t)
	invitationID := fixture.rows.Insert(t, "workspace_invitation", testutil.Cols{
		"workspace_id": fixture.workspaceID, "inviter_id": fixture.staffID,
		"invitee_email": fixture.clientEmail, "invitee_user_id": fixture.clientID,
		"role": "client", "organisation_ids": []string{fixture.organisationID},
		"status": "accepted", "expires_at": time.Now().Add(time.Hour),
	})
	for _, tc := range []struct {
		method, path, body string
		want               int
	}{
		{"GET", "/api/me", "", 200},
		{"POST", "/api/auth/refresh", "{}", 200},
		{"POST", "/api/cli-token", "{}", 200},
		{"POST", "/api/tokens", `{"name":"Client gate session PAT"}`, 201},
		{"GET", "/api/me/memberships", "", 200},
		{"GET", "/api/invitations", "", 200},
		{"GET", "/api/invitations/" + invitationID, "", 200},
		{"POST", "/api/invitations/" + invitationID + "/accept", "{}", 200},
		{"POST", "/api/invitations/" + invitationID + "/decline", "{}", 204},
		{"POST", "/auth/logout", "{}", 200},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+fixture.clientJWT)
			w := httptest.NewRecorder()
			fixture.router.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("session endpoint: got %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

func TestClientGateRouterClientReads(t *testing.T) {
	fixture := newClientGateFixture(t)
	const privateDescription = "Full staff description: private gate sentinel"
	const privateProperty = "private gate property sentinel"
	propertyID := uuid.NewString()
	properties, err := json.Marshal(map[string]any{propertyID: privateProperty})
	if err != nil {
		t.Fatal(err)
	}
	releasedWait := func(workspaceID, organisationID, clientID string) (projectID, releaseID, waitID string) {
		projectID = fixture.rows.Project(t, "Gate client project", testutil.Cols{
			"workspace_id": workspaceID, "organisation_id": organisationID,
		})
		repositoryID := fixture.rows.Insert(t, "repository", testutil.Cols{
			"workspace_id": workspaceID, "project_id": projectID, "github_repo_id": 1,
			"full_name": "gate/" + uuid.NewString(), "private": true,
			"default_branch": "main", "checkout_daemon_id": "test", "checkout_path": t.TempDir(),
			"version_policy": "tags", "last_tag": "",
		})
		releaseID = fixture.rows.Insert(t, "release", testutil.Cols{
			"workspace_id": workspaceID, "repository_id": repositoryID, "position": 0,
			"version": "1.0.0", "title": "Gate release",
		})
		waitID = fixture.rows.Issue(t, "Released client wait", testutil.Cols{
			"workspace_id": workspaceID, "project_id": projectID, "release_id": releaseID,
			"type": "chore", "assignee_type": "member", "assignee_id": clientID,
			"description": privateDescription, "properties": properties,
		})
		labelID := fixture.rows.Insert(t, "issue_label", testutil.Cols{
			"workspace_id": workspaceID, "resource_type": "issue", "name": "Client wait", "color": "#3b82f6",
		})
		fixture.rows.InsertNoID(t, "issue_to_label", testutil.Cols{
			"issue_id": waitID, "label_id": labelID,
		}, "issue_id = $1 AND label_id = $2", waitID, labelID)
		return
	}
	projectID, releaseID, waitID := releasedWait(fixture.workspaceID, fixture.organisationID, fixture.clientID)
	attachmentID := fixture.rows.Insert(t, "attachment", testutil.Cols{
		"workspace_id": fixture.workspaceID, "issue_id": waitID,
		"uploader_type": "member", "uploader_id": fixture.staffID,
		"filename": "private.txt", "url": "https://example.invalid/private.txt",
		"content_type": "text/plain", "size_bytes": 1,
	})
	foreignWorkspaceID := fixture.rows.Workspace(t, "Foreign gate workspace", "gate-foreign-"+uuid.NewString())
	foreignOrganisationID := fixture.rows.Insert(t, "organisation", testutil.Cols{
		"workspace_id": foreignWorkspaceID, "name": "Foreign gate organisation", "key": "gate-foreign",
	})
	foreignClientID := fixture.rows.User(t, "Foreign gate client", "gate-foreign-"+uuid.NewString()+"@example.invalid")
	fixture.rows.Member(t, foreignWorkspaceID, foreignClientID, "client", testutil.Cols{
		"organisation_ids": []string{foreignOrganisationID},
	})
	foreignProjectID, foreignReleaseID, foreignWaitID := releasedWait(foreignWorkspaceID, foreignOrganisationID, foreignClientID)
	forgedQuery := "?workspace_id=" + foreignWorkspaceID + "&user_id=" + foreignClientID +
		"&assignee_id=" + foreignClientID + "&organisation_id=" + foreignOrganisationID +
		"&project_id=" + foreignProjectID + "&release_id=" + foreignReleaseID + "&issue_id=" + foreignWaitID +
		"&fields=*&include=properties&open_only=false"

	var staffIssue struct {
		ID          string         `json:"id"`
		Identifier  string         `json:"identifier"`
		Description string         `json:"description"`
		Properties  map[string]any `json:"properties"`
	}
	staffReq := httptest.NewRequest("GET", "/api/issues/"+waitID, nil)
	staffReq.Header.Set("Authorization", "Bearer "+fixture.staffJWT)
	staffReq.Header.Set("X-Workspace-ID", fixture.workspaceID)
	testutil.Call(t, fixture.router.ServeHTTP, staffReq).Want(200).JSON(&staffIssue)
	if staffIssue.ID != waitID || staffIssue.Identifier == "" || staffIssue.Description != privateDescription ||
		len(staffIssue.Properties) != 1 || staffIssue.Properties[propertyID] != privateProperty {
		t.Fatalf("staff issue must retain full description and properties: %+v", staffIssue)
	}

	for _, credential := range []struct{ name, token string }{
		{"JWT", fixture.clientJWT}, {"PAT", fixture.clientPAT},
	} {
		t.Run(credential.name, func(t *testing.T) {
			for _, scope := range []struct{ name, workspaceID, userID, query string }{
				{"session", "", "", ""},
				{"foreign identity and workspace", foreignWorkspaceID, foreignClientID, forgedQuery},
				{"forged staff identity", foreignWorkspaceID, fixture.staffID, forgedQuery},
				{"malformed workspace", "not-a-uuid", foreignClientID, forgedQuery},
			} {
				t.Run(scope.name, func(t *testing.T) {
					for _, endpoint := range []struct{ path, listKey string }{
						{"/api/me/waits", "waits"},
						{"/api/me/waits/" + waitID, ""},
						{"/api/me/planned-work", "issues"},
					} {
						req := httptest.NewRequest("GET", endpoint.path+scope.query, nil)
						req.Header.Set("Authorization", "Bearer "+credential.token)
						req.Header.Set("X-Workspace-ID", scope.workspaceID)
						req.Header.Set("X-User-ID", scope.userID)
						response := testutil.Call(t, fixture.router.ServeHTTP, req).Want(200).Map()
						if endpoint.listKey == "" {
							if response["id"] != waitID {
								t.Fatalf("wait detail changed authenticated principal: %v", response)
							}
							continue
						}
						rows, ok := response[endpoint.listKey].([]any)
						if !ok || len(rows) != 1 || response["total"] != float64(1) {
							t.Fatalf("%s must use session scope: %v", endpoint.path, response)
						}
						row, ok := rows[0].(map[string]any)
						if !ok || row["id"] != waitID {
							t.Fatalf("%s returned another principal's work: %v", endpoint.path, rows)
						}
					}
					req := httptest.NewRequest("GET", "/api/me/waits/"+foreignWaitID+scope.query, nil)
					req.Header.Set("Authorization", "Bearer "+credential.token)
					req.Header.Set("X-Workspace-ID", scope.workspaceID)
					req.Header.Set("X-User-ID", scope.userID)
					testutil.Call(t, fixture.router.ServeHTTP, req).Want(404)
				})
			}

			for _, endpoint := range []struct{ method, path string }{
				{"GET", "/api/issues"},
				{"GET", "/api/issues?assignee_id=" + fixture.clientID + "&release_id=" + releaseID},
				{"GET", "/api/issues/" + waitID},
				{"GET", "/api/issues/" + staffIssue.Identifier},
				{"GET", "/api/issues/" + waitID + "/comments"},
				{"POST", "/api/issues/query"},
				{"POST", "/api/issues/table/rows"},
				{"GET", "/api/projects"},
				{"GET", "/api/projects?organisation_id=" + fixture.organisationID},
				{"GET", "/api/projects/" + projectID},
				{"GET", "/api/releases"},
				{"GET", "/api/releases?project_id=" + projectID},
				{"GET", "/api/releases/" + releaseID},
				{"GET", "/api/attachments/" + attachmentID + "/download"},
			} {
				t.Run(endpoint.method+" "+endpoint.path, func(t *testing.T) {
					req := httptest.NewRequest(endpoint.method, endpoint.path, strings.NewReader("{}"))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Authorization", "Bearer "+credential.token)
					req.Header.Set("X-Workspace-ID", fixture.workspaceID)
					req.Header.Set("X-User-ID", fixture.staffID)
					response := testutil.Call(t, fixture.router.ServeHTTP, req).Want(403)
					if strings.TrimSpace(response.Text()) != routerClientGateError {
						t.Fatalf("ordinary resource must be refused before its handler: %s", response.Text())
					}
				})
			}
		})
	}
	for _, endpoint := range []string{"/api/me/waits", "/api/me/waits/" + waitID, "/api/me/planned-work"} {
		t.Run("staff refused "+endpoint, func(t *testing.T) {
			req := httptest.NewRequest("GET", endpoint+forgedQuery, nil)
			req.Header.Set("Authorization", "Bearer "+fixture.staffJWT)
			req.Header.Set("X-Workspace-ID", fixture.workspaceID)
			req.Header.Set("X-User-ID", fixture.clientID)
			response := testutil.Call(t, fixture.router.ServeHTTP, req).Want(403).Map()
			if response["error"] != "client membership required" {
				t.Fatalf("staff must fail the client membership guard: %v", response)
			}
		})
		t.Run("unauthenticated "+endpoint, func(t *testing.T) {
			req := httptest.NewRequest("GET", endpoint, nil)
			req.Header.Set("X-User-ID", fixture.clientID)
			req.Header.Set("X-Workspace-ID", fixture.workspaceID)
			testutil.Call(t, fixture.router.ServeHTTP, req).Want(401)
		})
	}
}

func TestClientGateRouterWebSocket(t *testing.T) {
	fixture := newClientGateFixture(t)
	for _, credential := range []struct{ name, token string }{
		{"JWT", fixture.clientJWT}, {"PAT", fixture.clientPAT},
	} {
		t.Run(credential.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/ws?workspace_id="+fixture.workspaceID, nil)
			req.AddCookie(&http.Cookie{Name: auth.AuthCookieName, Value: credential.token})
			w := httptest.NewRecorder()
			fixture.router.ServeHTTP(w, req)
			if w.Code != 403 || !strings.Contains(w.Body.String(), "not a member of this workspace") {
				t.Fatalf("client socket must be refused before upgrade: %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestClientGateRouterNoMembership(t *testing.T) {
	fixture := newClientGateFixture(t)
	email := "gate-unjoined-" + uuid.NewString() + "@example.invalid"
	userID := fixture.rows.User(t, "No memberships", email)
	token, err := generateTestJWT(userID, email, "No memberships")
	if err != nil {
		t.Fatal(err)
	}
	for _, requestPath := range []string{"/api/workspaces", "/api/me"} {
		req := httptest.NewRequest("GET", requestPath, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		fixture.router.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("user without memberships must pass gate on %s: %d: %s", requestPath, w.Code, w.Body.String())
		}
	}
}

func TestClientGateRouterOrganisationSetsRemainClosedAfterDeletion(t *testing.T) {
	fixture := newClientGateFixture(t)
	other := fixture.rows.Insert(t, "organisation", testutil.Cols{
		"workspace_id": fixture.workspaceID, "name": "Second gate organisation", "key": "gate-second",
	})
	fixture.rows.Exec(t, `UPDATE member SET organisation_ids=$2::uuid[] WHERE user_id=$1`, fixture.clientID, []string{fixture.organisationID, other})
	for _, remaining := range []int{2, 1, 0} {
		if remaining == 1 {
			fixture.rows.Exec(t, `DELETE FROM organisation WHERE id=$1`, fixture.organisationID)
		} else if remaining == 0 {
			fixture.rows.Exec(t, `DELETE FROM organisation WHERE id=$1`, other)
		}
		for _, endpoint := range []struct{ method, path string }{
			{"GET", "/api/issues"}, {"POST", "/api/issues"}, {"GET", "/api/workspaces"},
		} {
			req := httptest.NewRequest(endpoint.method, endpoint.path, strings.NewReader("{}"))
			req.Header.Set("Authorization", "Bearer "+fixture.clientJWT)
			req.Header.Set("X-Workspace-ID", fixture.workspaceID)
			w := httptest.NewRecorder()
			fixture.router.ServeHTTP(w, req)
			if w.Code != http.StatusForbidden || strings.TrimSpace(w.Body.String()) != routerClientGateError {
				t.Fatalf("%d organisations opened %s %s: %d %s", remaining, endpoint.method, endpoint.path, w.Code, w.Body.String())
			}
		}
		req := httptest.NewRequest("GET", "/api/me/memberships", nil)
		req.Header.Set("Authorization", "Bearer "+fixture.clientJWT)
		w := httptest.NewRecorder()
		fixture.router.ServeHTTP(w, req)
		var response struct {
			Memberships []struct {
				Role          string `json:"role"`
				Organisations []struct {
					ID string `json:"id"`
				} `json:"organisations"`
			} `json:"memberships"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if w.Code != http.StatusOK || len(response.Memberships) != 1 || response.Memberships[0].Role != "client" || response.Memberships[0].Organisations == nil || len(response.Memberships[0].Organisations) != remaining {
			t.Fatalf("survivor discovery for %d organisations: %d %s", remaining, w.Code, w.Body.String())
		}
	}
}
