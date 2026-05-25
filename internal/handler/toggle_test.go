package handler

// White-box tests for toggle endpoints — same package for access to internals.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/rayjohnson/mcp-proxy/internal/auth"
	"github.com/rayjohnson/mcp-proxy/internal/store"
)

// ---------------------------------------------------------------------------
// Fake stores
// ---------------------------------------------------------------------------

// toggleFakeUpstreamStore satisfies store.UpstreamStoreI for toggle tests.
type toggleFakeUpstreamStore struct {
	configs map[string]*store.UpstreamConfig
}

func newToggleFakeUpstreamStore() *toggleFakeUpstreamStore {
	return &toggleFakeUpstreamStore{configs: make(map[string]*store.UpstreamConfig)}
}

func (f *toggleFakeUpstreamStore) CreateUpstreamConfig(_ context.Context, userID, serverType, serverURL, authType string, _ []byte) (*store.UpstreamConfig, error) {
	c := &store.UpstreamConfig{
		ID: serverType + "-" + userID, UserID: userID, ServerType: serverType,
		ServerURL: serverURL, AuthType: authType, Enabled: true,
	}
	f.configs[c.ID] = c
	return c, nil
}
func (f *toggleFakeUpstreamStore) GetUpstreamConfigsByUserID(_ context.Context, userID string) ([]*store.UpstreamConfig, error) {
	var out []*store.UpstreamConfig
	for _, c := range f.configs {
		if c.UserID == userID {
			out = append(out, c)
		}
	}
	return out, nil
}
func (f *toggleFakeUpstreamStore) GetUpstreamConfigByID(_ context.Context, id string) (*store.UpstreamConfig, error) {
	c, ok := f.configs[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return c, nil
}
func (f *toggleFakeUpstreamStore) UpdateUpstreamStatus(_ context.Context, id, status string) error {
	if c, ok := f.configs[id]; ok {
		c.Status = status
	}
	return nil
}
func (f *toggleFakeUpstreamStore) UpdateDetectedTransport(_ context.Context, _, _ string) error {
	return nil
}
func (f *toggleFakeUpstreamStore) UpdateEncryptedCreds(_ context.Context, _ string, _ []byte) error {
	return nil
}
func (f *toggleFakeUpstreamStore) DeleteUpstreamConfig(_ context.Context, id string) error {
	delete(f.configs, id)
	return nil
}
func (f *toggleFakeUpstreamStore) ToggleUpstream(_ context.Context, id string) (bool, error) {
	c, ok := f.configs[id]
	if !ok {
		return false, fmt.Errorf("not found")
	}
	c.Enabled = !c.Enabled
	return c.Enabled, nil
}

// toggleFakeCatalogStore satisfies store.ToggleStoreI for toggle tests.
type toggleFakeCatalogStore struct {
	toggles map[string]bool // key: userID+":"+catalogID
}

func newToggleFakeCatalogStore() *toggleFakeCatalogStore {
	return &toggleFakeCatalogStore{toggles: make(map[string]bool)}
}

func (f *toggleFakeCatalogStore) ToggleCatalogEntry(_ context.Context, userID, catalogID string) (bool, error) {
	key := userID + ":" + catalogID
	cur, exists := f.toggles[key]
	if !exists {
		f.toggles[key] = false
		return false, nil
	}
	f.toggles[key] = !cur
	return !cur, nil
}

func (f *toggleFakeCatalogStore) DisabledCatalogIDs(_ context.Context, userID string) (map[string]struct{}, error) {
	disabled := make(map[string]struct{})
	prefix := userID + ":"
	for key, enabled := range f.toggles {
		if !enabled && len(key) > len(prefix) && key[:len(prefix)] == prefix {
			disabled[key[len(prefix):]] = struct{}{}
		}
	}
	return disabled, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func initToggleTemplates(t *testing.T) {
	t.Helper()
	if err := InitTemplates(os.DirFS("../../web")); err != nil {
		t.Fatalf("InitTemplates: %v", err)
	}
}

func signedToggleCookie(t *testing.T, userID, role string) *http.Cookie {
	t.Helper()
	token, err := auth.SignToken(userID, role)
	if err != nil {
		t.Fatalf("auth.SignToken: %v", err)
	}
	return &http.Cookie{Name: "session", Value: token} //nolint:gosec // test cookie
}

func buildToggleMux(us store.UpstreamStoreI, ts store.ToggleStoreI) *http.ServeMux {
	th := NewToggleHandler(us, ts)
	mux := http.NewServeMux()
	mux.Handle("POST /api/upstreams/{id}/toggle",
		AuthMiddleware(http.HandlerFunc(th.ToggleUpstreamHandler)))
	mux.Handle("POST /api/catalog/{id}/toggle",
		AuthMiddleware(http.HandlerFunc(th.ToggleCatalogHandler)))
	return mux
}

func doTogglePost(mux http.Handler, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

// ---------------------------------------------------------------------------
// T010: POST /api/upstreams/{id}/toggle
// ---------------------------------------------------------------------------

func TestToggleUpstreamHandler_200(t *testing.T) {
	initToggleTemplates(t)
	us := newToggleFakeUpstreamStore()
	ts := newToggleFakeCatalogStore()

	userID := "user-toggle-1"
	us.configs["up-1"] = &store.UpstreamConfig{ID: "up-1", UserID: userID, ServerType: "srv", Enabled: true}

	mux := buildToggleMux(us, ts)
	cookie := signedToggleCookie(t, userID, "developer")

	rr := doTogglePost(mux, "/api/upstreams/up-1/toggle", cookie)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Enabled {
		t.Error("expected enabled=false after toggle")
	}
}

func TestToggleUpstreamHandler_401_Unauthenticated(t *testing.T) {
	us := newToggleFakeUpstreamStore()
	ts := newToggleFakeCatalogStore()
	mux := buildToggleMux(us, ts)
	rr := doTogglePost(mux, "/api/upstreams/up-1/toggle", nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
}

func TestToggleUpstreamHandler_403_WrongUser(t *testing.T) {
	us := newToggleFakeUpstreamStore()
	ts := newToggleFakeCatalogStore()
	us.configs["up-1"] = &store.UpstreamConfig{ID: "up-1", UserID: "owner-user", ServerType: "srv", Enabled: true}

	mux := buildToggleMux(us, ts)
	cookie := signedToggleCookie(t, "different-user", "developer")

	rr := doTogglePost(mux, "/api/upstreams/up-1/toggle", cookie)
	if rr.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rr.Code)
	}
}

func TestToggleUpstreamHandler_404_NotFound(t *testing.T) {
	us := newToggleFakeUpstreamStore()
	ts := newToggleFakeCatalogStore()
	mux := buildToggleMux(us, ts)
	cookie := signedToggleCookie(t, "user-x", "developer")

	rr := doTogglePost(mux, "/api/upstreams/no-such-id/toggle", cookie)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// T011: POST /api/catalog/{id}/toggle
// ---------------------------------------------------------------------------

func TestToggleCatalogHandler_200(t *testing.T) {
	us := newToggleFakeUpstreamStore()
	ts := newToggleFakeCatalogStore()
	mux := buildToggleMux(us, ts)
	cookie := signedToggleCookie(t, "user-c1", "developer")

	rr := doTogglePost(mux, "/api/catalog/cat-1/toggle", cookie)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Enabled {
		t.Error("expected enabled=false after first catalog toggle")
	}
}

func TestToggleCatalogHandler_401_Unauthenticated(t *testing.T) {
	us := newToggleFakeUpstreamStore()
	ts := newToggleFakeCatalogStore()
	mux := buildToggleMux(us, ts)
	rr := doTogglePost(mux, "/api/catalog/cat-1/toggle", nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
}
