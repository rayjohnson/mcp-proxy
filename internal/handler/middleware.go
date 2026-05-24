package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/rayjohnson/mcp-proxy/internal/auth"
)

type contextKey string

const claimsKey contextKey = "claims"

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil {
			slog.Warn("auth: no session cookie", "path", r.URL.Path)
			authRedirectOrError(w, r)
			return
		}
		claims, err := auth.VerifyToken(cookie.Value)
		if err != nil {
			slog.Warn("auth: invalid token", "path", r.URL.Path, "err", err)
			authRedirectOrError(w, r)
			return
		}
		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func AdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := ClaimsFromContext(r.Context())
		if claims == nil || claims.Role != "admin" {
			if isAPIRequest(r) {
				writeJSONError(w, "forbidden", http.StatusForbidden)
				return
			}
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// authRedirectOrError redirects browser requests to /login and returns 401 JSON for API calls.
func authRedirectOrError(w http.ResponseWriter, r *http.Request) {
	if isAPIRequest(r) {
		writeJSONError(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// isAPIRequest returns true for requests that expect a non-HTML response.
func isAPIRequest(r *http.Request) bool {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		return true
	}
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/html")
}

func ClaimsFromContext(ctx context.Context) *auth.Claims {
	c, _ := ctx.Value(claimsKey).(*auth.Claims)
	return c
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.status)
	})
}
