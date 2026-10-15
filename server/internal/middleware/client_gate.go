package middleware

import (
	"log/slog"
	"net/http"
	"path"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

var clientSessionRoutes = [...]struct {
	method  string
	pattern string
}{
	{http.MethodGet, "/api/me"},
	{http.MethodPost, "/api/auth/refresh"},
	{http.MethodPost, "/api/cli-token"},
	{http.MethodPost, "/api/tokens"},
	{http.MethodGet, "/api/me/memberships"},
	{http.MethodGet, "/api/invitations"},
	{http.MethodGet, "/api/invitations/*"},
	{http.MethodPost, "/api/invitations/*/accept"},
	{http.MethodPost, "/api/invitations/*/decline"},
}

func isClientSessionRoute(method, requestPath string) bool {
	for _, route := range clientSessionRoutes {
		if method != route.method {
			continue
		}
		if matched, _ := path.Match(route.pattern, requestPath); matched {
			return true
		}
	}
	return false
}

func clientGate(queries *db.Queries, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		// Runtime-bound daemon tokens have no user principal. Their revocation
		// follows the runtime owner's membership changes instead.
		if userID == "" {
			next.ServeHTTP(w, r)
			return
		}
		id, err := util.ParseUUID(userID)
		var isClient bool
		if err == nil {
			isClient, err = queries.IsClientAccount(r.Context(), id)
		}
		if err != nil {
			slog.Error("client gate: account lookup failed", "path", r.URL.Path, "user_id", userID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to check client account")
			return
		}
		if isClient && !isClientSessionRoute(r.Method, r.URL.Path) {
			slog.Debug("client gate: request refused", "path", r.URL.Path, "user_id", userID)
			writeError(w, http.StatusForbidden, "client accounts can only use the session routes")
			return
		}
		next.ServeHTTP(w, r)
	})
}
