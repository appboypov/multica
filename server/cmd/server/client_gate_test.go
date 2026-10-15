package main

import (
	"context"
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
	rows.Member(t, workspaceID, clientID, "client", testutil.Cols{"organisation_id": organisationID})
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
	if protected == 0 || daemon == 0 || bridge == 0 || sessions != 9 {
		t.Fatalf("route coverage missing: protected=%d daemon=%d bridge=%d sessions=%d", protected, daemon, bridge, sessions)
	}
}

func TestClientGateRouterSessions(t *testing.T) {
	fixture := newClientGateFixture(t)
	invitationID := fixture.rows.Insert(t, "workspace_invitation", testutil.Cols{
		"workspace_id": fixture.workspaceID, "inviter_id": fixture.staffID,
		"invitee_email": fixture.clientEmail, "invitee_user_id": fixture.clientID,
		"role": "client", "organisation_id": fixture.organisationID,
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
