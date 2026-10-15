package middleware

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/redis/go-redis/v9"
)

const clientGateUserID = "01972f7e-7e8d-77ef-a13d-1b0ce3e9c001"
const clientGateError = `{"error":"client accounts can only use the session routes"}`

func TestClientSessionRoutes(t *testing.T) {
	allowed := []struct{ method, path string }{
		{"GET", "/api/me"},
		{"POST", "/api/auth/refresh"},
		{"POST", "/api/cli-token"},
		{"POST", "/api/tokens"},
		{"GET", "/api/me/memberships"},
		{"GET", "/api/invitations"},
		{"GET", "/api/invitations/invitation-id"},
		{"POST", "/api/invitations/invitation-id/accept"},
		{"POST", "/api/invitations/invitation-id/decline"},
	}
	for _, route := range allowed {
		t.Run(route.method+route.path, func(t *testing.T) {
			if !isClientSessionRoute(route.method, route.path) {
				t.Fatal("session route refused")
			}
		})
	}
	denied := []struct{ method, path string }{
		{"PATCH", "/api/me"}, {"HEAD", "/api/me"},
		{"GET", "/api/me/"}, {"GET", "/api/me/onboarding"},
		{"GET", "/api/me/memberships/extra"}, {"GET", "/api/auth/refresh"},
		{"GET", "/api/cli-token"}, {"GET", "/api/tokens"},
		{"DELETE", "/api/tokens"}, {"POST", "/api/invitations"},
		{"GET", "/api/invitations/x/y"}, {"GET", "/api/invitations/x/accept"},
		{"POST", "/api/invitations/x"}, {"DELETE", "/api/invitations/x"},
		{"POST", "/api/invitations/x/y/accept"}, {"POST", "/api/invitations/x/accept/"},
		{"POST", "/api/invitations/x/y/decline"}, {"POST", "/api/invitations/x/decline/extra"},
		{"GET", "/api/workspaces"}, {"GET", "/api/issues"},
		{"GET", "/api/attachments/id/download"}, {"POST", "/api/share-links/join"},
		{"GET", "/api/search"}, {"GET", "/api/inbox"},
		{"POST", "/api/daemon/register"}, {"GET", "/ws"},
		{"GET", "/api/future-resource"},
	}
	for _, route := range denied {
		t.Run(route.method+route.path, func(t *testing.T) {
			if isClientSessionRoute(route.method, route.path) {
				t.Fatal("non-session route allowed")
			}
		})
	}
}

// The DBTX fake is entirely in-process: neither credential tests nor lookup
// failure tests need a database, Redis, or an external Fleet service.
type clientGateDB struct {
	db.DBTX
	t         *testing.T
	isClient  bool
	lookupErr error
	gateCalls int
}

type clientGateRow func(...any) error

func (row clientGateRow) Scan(dest ...any) error { return row(dest...) }

func (f *clientGateDB) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	return clientGateRow(func(dest ...any) error {
		id, err := util.ParseUUID(clientGateUserID)
		if err != nil {
			return err
		}
		switch {
		case strings.HasPrefix(query, "-- name: IsClientAccount :one"):
			f.gateCalls++
			if len(args) != 1 || args[0] != id {
				f.t.Errorf("gate queried wrong principal: %v", args)
			}
			if f.lookupErr != nil {
				return f.lookupErr
			}
			*dest[0].(*bool) = f.isClient
		case strings.HasPrefix(query, "-- name: GetPersonalAccessTokenByHash :one"):
			*dest[1].(*pgtype.UUID) = id
		case strings.HasPrefix(query, "-- name: GetTaskTokenByHash :one"):
			*dest[5].(*pgtype.UUID) = id
		case strings.HasPrefix(query, "-- name: GetUser :one"):
			*dest[0].(*pgtype.UUID) = id
		case strings.HasPrefix(query, "-- name: GetDaemonTokenByHash :one"):
			*dest[2].(*pgtype.UUID) = id
			*dest[3].(*string) = "staff-daemon"
		default:
			f.t.Errorf("unexpected query: %s", query)
			return errors.New("unexpected query")
		}
		return nil
	})
}

func (f *clientGateDB) Exec(_ context.Context, query string, _ ...any) (pgconn.CommandTag, error) {
	if !strings.HasPrefix(query, "-- name: UpdatePersonalAccessTokenLastUsed :exec") {
		return pgconn.CommandTag{}, errors.New("unexpected write")
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func TestClientGate(t *testing.T) {
	for _, tc := range []struct {
		name, method, path string
		isClient           bool
		want               int
	}{
		{"client read", "GET", "/api/issues", true, 403},
		{"client write", "POST", "/api/issues", true, 403},
		{"staff", "POST", "/api/issues", false, 204},
		{"no memberships", "GET", "/api/workspaces", false, 204},
		{"client session", "GET", "/api/me", true, 204},
		{"session wrong method", "PATCH", "/api/me", true, 403},
		{"session extra segment", "GET", "/api/invitations/x/y", true, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &clientGateDB{t: t, isClient: tc.isClient}
			called := false
			h := clientGate(db.New(fake), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			}))
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("X-User-ID", clientGateUserID)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != tc.want || called != (tc.want == 204) || fake.gateCalls != 1 {
				t.Fatalf("status=%d called=%t lookups=%d body=%s", w.Code, called, fake.gateCalls, w.Body.String())
			}
			if tc.want == 403 && strings.TrimSpace(w.Body.String()) != clientGateError {
				t.Fatalf("wrong refusal: %s", w.Body.String())
			}
		})
	}
}

func TestClientGateLookupFailureClosed(t *testing.T) {
	for _, requestPath := range []string{"/api/issues", "/api/me"} {
		t.Run(requestPath, func(t *testing.T) {
			fake := &clientGateDB{t: t, lookupErr: errors.New("database unavailable")}
			h := clientGate(db.New(fake), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("lookup error must not call handler, even for a session route")
			}))
			req := httptest.NewRequest("GET", requestPath, nil)
			req.Header.Set("X-User-ID", clientGateUserID)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != 500 || strings.Contains(w.Body.String(), "database unavailable") {
				t.Fatalf("expected sanitized 500, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestClientGateLogsRefusal(t *testing.T) {
	logs := withCapturedLogs(t)
	fake := &clientGateDB{t: t, isClient: true}
	h := clientGate(db.New(fake), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler must not run")
	}))
	req := httptest.NewRequest("GET", "/api/issues?secret=not-for-logs", nil)
	req.Header.Set("X-User-ID", clientGateUserID)
	req.Header.Set("Authorization", "Bearer not-for-logs")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !strings.Contains(logs.String(), "path=/api/issues") || !strings.Contains(logs.String(), clientGateUserID) || strings.Contains(logs.String(), "not-for-logs") {
		t.Fatalf("refusal log missing context or leaking credential/query: %s", logs.String())
	}
}

type clientGateRedis struct {
	redis.UniversalClient
	value string
}

func (f clientGateRedis) Get(context.Context, string) *redis.StringCmd {
	return redis.NewStringResult(f.value, nil)
}

type clientGateTransport func(*http.Request) (*http.Response, error)

func (f clientGateTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestClientGateCredentialBranches(t *testing.T) {
	for _, daemon := range []bool{false, true} {
		kinds := []string{"jwt", "pat", "cached pat", "cloud pat"}
		if !daemon {
			kinds = append(kinds, "cookie", "task token")
		}
		for _, kind := range kinds {
			for _, isClient := range []bool{false, true} {
				name := "auth/"
				if daemon {
					name = "daemon/"
				}
				if isClient {
					name += "client/"
				} else {
					name += "staff/"
				}
				t.Run(name+kind, func(t *testing.T) {
					fake := &clientGateDB{t: t, isClient: isClient}
					claims := validClaims()
					claims["sub"] = clientGateUserID
					token := generateToken(claims, auth.JWTSecret())
					var cache *auth.PATCache
					var cloud *auth.CloudPATVerifier
					switch kind {
					case "pat":
						token = "mul_client_gate"
					case "cached pat":
						token = "mul_client_gate_cached"
						cache = auth.NewPATCache(clientGateRedis{value: clientGateUserID})
					case "task token":
						token = "mat_client_gate"
					case "cloud pat":
						token = "mcn_client_gate"
						cloud = auth.NewCloudPATVerifier(auth.CloudPATVerifierConfig{
							FleetBaseURL: "https://fleet.invalid",
							HTTPClient: &http.Client{Transport: clientGateTransport(func(r *http.Request) (*http.Response, error) {
								return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"valid":true,"owner_id":"` + clientGateUserID + `"}`)), Request: r}, nil
							})},
						})
					}
					called := false
					next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						called = true
						if r.Header.Get("X-User-ID") != clientGateUserID {
							t.Errorf("wrong resolved principal: %s", r.Header.Get("X-User-ID"))
						}
						w.WriteHeader(204)
					})
					var h http.Handler = Auth(db.New(fake), cache, cloud, nil)(next)
					requestPath := "/api/future-resource"
					if daemon {
						h = DaemonAuth(db.New(fake), cache, nil, cloud)(next)
						requestPath = "/api/daemon/future-resource"
					}
					req := httptest.NewRequest("GET", requestPath, nil)
					if kind == "cookie" {
						req.AddCookie(&http.Cookie{Name: auth.AuthCookieName, Value: token})
					} else {
						req.Header.Set("Authorization", "Bearer "+token)
					}
					// A supplied staff principal must never override the token owner.
					req.Header.Set("X-User-ID", "00000000-0000-0000-0000-000000000001")
					w := httptest.NewRecorder()
					h.ServeHTTP(w, req)
					want := 204
					if isClient {
						want = 403
					}
					if w.Code != want || called == isClient || fake.gateCalls != 1 {
						t.Fatalf("status=%d called=%t lookups=%d body=%s", w.Code, called, fake.gateCalls, w.Body.String())
					}
					if isClient && strings.TrimSpace(w.Body.String()) != clientGateError {
						t.Fatalf("wrong refusal: %s", w.Body.String())
					}
				})
			}
		}
	}
}

func TestClientGateDaemonRuntimeCredentials(t *testing.T) {
	for _, cached := range []bool{false, true} {
		name := "database"
		if cached {
			name = "cache"
		}
		t.Run(name, func(t *testing.T) {
			fake := &clientGateDB{t: t, isClient: true}
			var cache *auth.DaemonTokenCache
			if cached {
				cache = auth.NewDaemonTokenCache(clientGateRedis{value: `{"w":"` + clientGateUserID + `","d":"staff-daemon"}`})
			}
			h := DaemonAuth(db.New(fake), nil, cache, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if DaemonIDFromContext(r.Context()) != "staff-daemon" || r.Header.Get("X-User-ID") != "" {
					t.Fatal("runtime principal lost or forged user header retained")
				}
				w.WriteHeader(204)
			}))
			req := httptest.NewRequest("POST", "/api/daemon/heartbeat", nil)
			req.Header.Set("Authorization", "Bearer mdt_staff_daemon")
			req.Header.Set("X-User-ID", clientGateUserID)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != 204 || fake.gateCalls != 0 {
				t.Fatalf("runtime token should use runtime principal, got %d, %d user lookups: %s", w.Code, fake.gateCalls, w.Body.String())
			}
		})
	}
}

func TestClientGateInvalidResolvedUserFailsClosed(t *testing.T) {
	fake := &clientGateDB{t: t}
	h := clientGate(db.New(fake), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("invalid resolved user must not reach handler")
	}))
	req := httptest.NewRequest("GET", "/api/me", nil)
	req.Header.Set("X-User-ID", "not-a-uuid")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 500 || fake.gateCalls != 0 {
		t.Fatalf("expected fail-closed 500 before query, got %d, lookups=%d: %s", w.Code, fake.gateCalls, w.Body.String())
	}
}
