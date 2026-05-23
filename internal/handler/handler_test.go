package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rayjohnson/mcp-proxy/internal/auth"
	"github.com/rayjohnson/mcp-proxy/internal/handler"
)

const templatesDir = "../../web/templates"
const staticDir = "../../web/static"

func initTemplates(t *testing.T) {
	t.Helper()
	if err := handler.InitTemplates(templatesDir); err != nil {
		t.Fatalf("InitTemplates: %v", err)
	}
}

// buildMux wires UI-only routes that work without a database.
func buildMux(t *testing.T) *http.ServeMux {
	t.Helper()
	initTemplates(t)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/login", http.StatusFound)
	})
	mux.HandleFunc("GET /login", handler.LoginPage)
	mux.HandleFunc("GET /register", handler.RegisterPage)
	mux.Handle("GET /dashboard", handler.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Minimal stub: tests only need it to return 200 when authenticated.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body>MCP Proxy Dashboard</body></html>"))
	})))
	mux.HandleFunc("GET /health", handler.HealthHandler)
	mux.HandleFunc("GET /healthz", handler.HealthHandler)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))
	return mux
}

func get(mux http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func getWithSession(mux http.Handler, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: token}) //nolint:gosec // test cookie, no security attributes needed
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func TestInitTemplates(t *testing.T) {
	if err := handler.InitTemplates(templatesDir); err != nil {
		t.Fatalf("InitTemplates failed: %v", err)
	}
}

func TestRootRedirectsToLogin(t *testing.T) {
	mux := buildMux(t)
	rr := get(mux, "/")
	if rr.Code != http.StatusFound {
		t.Errorf("GET / status = %d, want %d", rr.Code, http.StatusFound)
	}
	if loc := rr.Header().Get("Location"); loc != "/login" {
		t.Errorf("GET / Location = %q, want /login", loc)
	}
}

func TestUnknownPathReturns404(t *testing.T) {
	mux := buildMux(t)
	rr := get(mux, "/does-not-exist")
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /does-not-exist status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestLoginPageRenders(t *testing.T) {
	mux := buildMux(t)
	rr := get(mux, "/login")
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /login status = %d, want %d", rr.Code, http.StatusOK)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	if !strings.Contains(rr.Body.String(), "Sign In") {
		t.Error("login page body missing 'Sign In'")
	}
}

func TestRegisterPageRenders(t *testing.T) {
	mux := buildMux(t)
	rr := get(mux, "/register")
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /register status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "Create Account") {
		t.Error("register page body missing 'Create Account'")
	}
}

func TestDashboardRequiresAuth(t *testing.T) {
	mux := buildMux(t)
	rr := get(mux, "/dashboard")
	if rr.Code != http.StatusSeeOther {
		t.Errorf("GET /dashboard (no cookie) status = %d, want %d (redirect to login)", rr.Code, http.StatusSeeOther)
	}
	if loc := rr.Header().Get("Location"); loc != "/login" {
		t.Errorf("GET /dashboard (no cookie) Location = %q, want /login", loc)
	}
}

func TestDashboardRendersWhenAuthenticated(t *testing.T) {
	mux := buildMux(t)
	token, err := auth.SignToken("user-123", "developer")
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}
	rr := getWithSession(mux, "/dashboard", token)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /dashboard (authed) status = %d, want 200\nbody: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "MCP Proxy") {
		t.Error("dashboard body missing 'MCP Proxy'")
	}
}

func TestDashboardRejectsInvalidToken(t *testing.T) {
	mux := buildMux(t)
	rr := getWithSession(mux, "/dashboard", "not-a-valid-token")
	if rr.Code != http.StatusSeeOther {
		t.Errorf("GET /dashboard (bad token) status = %d, want 303 (redirect to login)", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/login" {
		t.Errorf("GET /dashboard (bad token) Location = %q, want /login", loc)
	}
}

func TestHealthEndpoints(t *testing.T) {
	mux := buildMux(t)
	for _, path := range []string{"/health", "/healthz"} {
		rr := get(mux, path)
		if rr.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, want %d", path, rr.Code, http.StatusOK)
		}
	}
}

func TestStaticFileServed(t *testing.T) {
	mux := buildMux(t)
	rr := get(mux, "/static/style.css")
	if rr.Code != http.StatusOK {
		t.Errorf("GET /static/style.css status = %d, want %d", rr.Code, http.StatusOK)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/css") {
		t.Errorf("Content-Type = %q, want text/css", ct)
	}
}
