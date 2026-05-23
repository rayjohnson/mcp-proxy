package handler

// White-box tests: same package so we can construct AuthHandler with fake stores
// without exporting interfaces. These tests spin up a real httptest.Server and
// drive it with an http.Client+CookieJar, catching bugs that unit tests miss
// (cookie not set, redirect not followed, JWT not accepted, etc.)

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/rayjohnson/mcp-proxy/internal/store"
)

const templatesDir = "../../web/templates"

// ---------------------------------------------------------------------------
// Fake store implementations
// ---------------------------------------------------------------------------

type fakeUserStore struct {
	mu    sync.Mutex
	byEmail map[string]*store.User
	byID    map[string]*store.User
	seq     int
}

func newFakeUserStore() *fakeUserStore {
	return &fakeUserStore{
		byEmail: make(map[string]*store.User),
		byID:    make(map[string]*store.User),
	}
}

func (f *fakeUserStore) CountUsers(_ context.Context) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.byEmail), nil
}

func (f *fakeUserStore) CreateUser(_ context.Context, email, hash, role string) (*store.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.byEmail[email]; exists {
		return nil, store.ErrDuplicateEmail
	}
	f.seq++
	id := url.PathEscape(email)
	u := &store.User{
		ID:           id,
		Email:        email,
		PasswordHash: &hash,
		Role:         role,
		ProxyToken:   "proxy-" + id,
	}
	f.byEmail[email] = u
	f.byID[id] = u
	return u, nil
}

func (f *fakeUserStore) GetUserByEmail(_ context.Context, email string) (*store.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.byEmail[email]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}

func (f *fakeUserStore) GetUserByID(_ context.Context, id string) (*store.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.byID[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}

type fakeCatalog struct{}

func (f *fakeCatalog) ListActiveCatalogEntries(_ context.Context) ([]*store.CatalogEntry, error) {
	return nil, nil
}

type fakeSuggestions struct{}

func (f *fakeSuggestions) CreateSuggestionForAllUsers(_ context.Context, _ string) error {
	return nil
}

// ---------------------------------------------------------------------------
// Test server builder
// ---------------------------------------------------------------------------

func newAuthTestServer(t *testing.T, users *fakeUserStore) *httptest.Server {
	t.Helper()
	if err := InitTemplates(templatesDir); err != nil {
		t.Fatalf("InitTemplates: %v", err)
	}

	authH := &AuthHandler{
		users:       users,
		catalog:     &fakeCatalog{},
		suggestions: &fakeSuggestions{},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/login", http.StatusFound)
	})
	mux.HandleFunc("GET /login", LoginPage)
	mux.HandleFunc("GET /register", RegisterPage)
	mux.Handle("GET /dashboard", AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		renderTemplate(w, "dashboard.html", DashboardData{
			PageBase: PageBase{IsAdmin: false},
			ProxyURL: "http://localhost:8080/mcp/test-token",
		})
	})))
	mux.HandleFunc("POST /api/auth/register", authH.Register)
	mux.HandleFunc("POST /api/auth/login", authH.Login)
	mux.HandleFunc("GET /health", HealthHandler)

	return httptest.NewServer(mux)
}

// newClient returns an http.Client that follows redirects and persists cookies.
func newClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestRegisterSuccessRedirectsToDashboard(t *testing.T) {
	srv := newAuthTestServer(t, newFakeUserStore())
	defer srv.Close()
	client := newClient(t)

	resp, err := client.PostForm(srv.URL+"/api/auth/register", url.Values{
		"email":    {"alice@example.com"},
		"password": {"password123"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.Request.URL.Path != "/dashboard" {
		t.Errorf("ended up at %q, want /dashboard", resp.Request.URL.Path)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("final status = %d, want 200", resp.StatusCode)
	}
}

func TestRegisterSetsSessionCookie(t *testing.T) {
	srv := newAuthTestServer(t, newFakeUserStore())
	defer srv.Close()

	// Single-request client (no redirect following) so we can inspect the register response.
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.PostForm(srv.URL+"/api/auth/register", url.Values{
		"email":    {"bob@example.com"},
		"password": {"password123"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("register status = %d, want 303", resp.StatusCode)
	}
	if resp.Header.Get("Location") != "/dashboard" {
		t.Errorf("Location = %q, want /dashboard", resp.Header.Get("Location"))
	}

	var sessionCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("no session cookie in register response")
	}
	if sessionCookie.HttpOnly != true {
		t.Error("session cookie should be HttpOnly")
	}
}

func TestRegisterDuplicateEmailRedirectsWithError(t *testing.T) {
	users := newFakeUserStore()
	srv := newAuthTestServer(t, users)
	defer srv.Close()
	client := newClient(t)

	vals := url.Values{"email": {"dup@example.com"}, "password": {"password123"}}
	if _, err := client.PostForm(srv.URL+"/api/auth/register", vals); err != nil {
		t.Fatal(err)
	}

	// Second registration with same email.
	resp, err := client.PostForm(srv.URL+"/api/auth/register", vals)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.Request.URL.Path != "/register" {
		t.Errorf("ended up at %q, want /register", resp.Request.URL.Path)
	}
	errParam := resp.Request.URL.Query().Get("error")
	if errParam == "" {
		t.Error("expected ?error= query param on /register redirect")
	}
}

func TestRegisterMissingFieldsRedirectsWithError(t *testing.T) {
	srv := newAuthTestServer(t, newFakeUserStore())
	defer srv.Close()
	client := newClient(t)

	resp, err := client.PostForm(srv.URL+"/api/auth/register", url.Values{
		"email": {"nopassword@example.com"},
		// no password
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.Request.URL.Path != "/register" {
		t.Errorf("ended up at %q, want /register", resp.Request.URL.Path)
	}
}

func TestRegisterPageShowsError(t *testing.T) {
	srv := newAuthTestServer(t, newFakeUserStore())
	defer srv.Close()
	client := newClient(t)

	resp, err := client.Get(srv.URL + "/register?error=That+email+is+already+registered.")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "That email is already registered.") {
		t.Error("register page did not show error message in body")
	}
}

func TestLoginSuccessRedirectsToDashboard(t *testing.T) {
	users := newFakeUserStore()
	srv := newAuthTestServer(t, users)
	defer srv.Close()
	client := newClient(t)

	// First register.
	if _, err := client.PostForm(srv.URL+"/api/auth/register", url.Values{
		"email":    {"charlie@example.com"},
		"password": {"password123"},
	}); err != nil {
		t.Fatal(err)
	}

	// Log out by clearing jar, then log in fresh.
	jar, _ := cookiejar.New(nil)
	client.Jar = jar

	resp, err := client.PostForm(srv.URL+"/api/auth/login", url.Values{
		"email":    {"charlie@example.com"},
		"password": {"password123"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.Request.URL.Path != "/dashboard" {
		t.Errorf("ended up at %q, want /dashboard", resp.Request.URL.Path)
	}
}

func TestLoginWrongPasswordRedirectsWithError(t *testing.T) {
	users := newFakeUserStore()
	srv := newAuthTestServer(t, users)
	defer srv.Close()
	client := newClient(t)

	// Register first.
	if _, err := client.PostForm(srv.URL+"/api/auth/register", url.Values{
		"email":    {"dave@example.com"},
		"password": {"password123"},
	}); err != nil {
		t.Fatal(err)
	}

	jar, _ := cookiejar.New(nil)
	client.Jar = jar

	resp, err := client.PostForm(srv.URL+"/api/auth/login", url.Values{
		"email":    {"dave@example.com"},
		"password": {"wrongpassword"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.Request.URL.Path != "/login" {
		t.Errorf("ended up at %q, want /login", resp.Request.URL.Path)
	}
	if resp.Request.URL.Query().Get("error") == "" {
		t.Error("expected ?error= on /login redirect")
	}
}

func TestFullAuthFlowCookieAccepted(t *testing.T) {
	srv := newAuthTestServer(t, newFakeUserStore())
	defer srv.Close()
	client := newClient(t)

	// Register — should land on /dashboard with 200.
	resp, err := client.PostForm(srv.URL+"/api/auth/register", url.Values{
		"email":    {"eve@example.com"},
		"password": {"password123"},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close() //nolint:errcheck

	if resp.Request.URL.Path != "/dashboard" {
		t.Fatalf("after register, at %q, want /dashboard", resp.Request.URL.Path)
	}

	// A subsequent GET /dashboard with the same client (cookie jar) should also succeed.
	resp2, err := client.Get(srv.URL + "/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close() //nolint:errcheck

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("GET /dashboard status = %d, want 200 (cookie not accepted)", resp2.StatusCode)
	}
}

func TestDashboardWithoutCookieRedirectsToLogin(t *testing.T) {
	srv := newAuthTestServer(t, newFakeUserStore())
	defer srv.Close()

	// Non-following client so we see the redirect itself.
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get(srv.URL + "/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Errorf("Location = %q, want /login", loc)
	}
}
