package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rayjohnson/mcp-proxy/internal/aitools"
	"github.com/rayjohnson/mcp-proxy/internal/auth"
	"github.com/rayjohnson/mcp-proxy/internal/handler"
	"github.com/rayjohnson/mcp-proxy/internal/store"
)

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

// fakeAIUserStore satisfies handler.aitoolsUserStore (matched structurally).
type fakeAIUserStore struct {
	token string
	err   error
}

func (f *fakeAIUserStore) GetUserByID(_ context.Context, id string) (*store.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &store.User{ID: id, ProxyToken: f.token}, nil
}

// fakeTool is a controllable aitools.Configurer.
type fakeTool struct {
	id           string
	detectResult aitools.AITool
	configureErr error
	configured   bool
}

func (f *fakeTool) ID() string          { return f.id }
func (f *fakeTool) Detect() aitools.AITool { return f.detectResult }
func (f *fakeTool) Configure(mcpURL string) error {
	if f.configureErr != nil {
		return f.configureErr
	}
	f.configured = true
	f.detectResult.Status = aitools.StatusConfigured
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func buildAIToolsMux(t *testing.T, tools []aitools.Configurer, us *fakeAIUserStore, localMode bool) *http.ServeMux {
	t.Helper()
	h := handler.NewAIToolsHandler(tools, us, "http://localhost:9753", localMode)
	mux := http.NewServeMux()
	mux.Handle("GET /api/tools",
		handler.AuthMiddleware(http.HandlerFunc(h.StatusAPI)))
	mux.Handle("POST /api/tools/{id}/configure",
		handler.AuthMiddleware(http.HandlerFunc(h.ConfigureAPI)))
	return mux
}

func signedCookie(t *testing.T, userID, role string) *http.Cookie {
	t.Helper()
	token, err := auth.SignToken(userID, role)
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}
	return &http.Cookie{Name: "session", Value: token} //nolint:gosec // test cookie, security attributes not required
}

func doReq(mux http.Handler, method, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

// ---------------------------------------------------------------------------
// GET /api/tools
// ---------------------------------------------------------------------------

func TestStatusAPI_NonLocalMode_Returns503(t *testing.T) {
	tools := []aitools.Configurer{
		&fakeTool{id: "test", detectResult: aitools.AITool{ID: "test", Status: aitools.StatusConfigured}},
	}
	mux := buildAIToolsMux(t, tools, &fakeAIUserStore{token: "tok"}, false)
	cookie := signedCookie(t, "user-1", "developer")

	rr := doReq(mux, http.MethodGet, "/api/tools", cookie)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rr.Code)
	}
}

func TestStatusAPI_RequiresAuth(t *testing.T) {
	mux := buildAIToolsMux(t, nil, &fakeAIUserStore{}, true)
	rr := doReq(mux, http.MethodGet, "/api/tools", nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (API path returns JSON error, not redirect)", rr.Code)
	}
}

func TestStatusAPI_ReturnsToolList(t *testing.T) {
	tools := []aitools.Configurer{
		&fakeTool{id: "claude-desktop", detectResult: aitools.AITool{
			ID: "claude-desktop", DisplayName: "Claude Desktop", Status: aitools.StatusConfigured,
		}},
		&fakeTool{id: "gemini-cli", detectResult: aitools.AITool{
			ID: "gemini-cli", DisplayName: "Gemini CLI", Status: aitools.StatusNotInstalled,
		}},
	}
	mux := buildAIToolsMux(t, tools, &fakeAIUserStore{token: "tok"}, true)
	cookie := signedCookie(t, "user-1", "developer")

	rr := doReq(mux, http.MethodGet, "/api/tools", cookie)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}

	var result []aitools.AITool
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	if result[0].ID != "claude-desktop" || result[0].Status != aitools.StatusConfigured {
		t.Errorf("result[0] = %+v", result[0])
	}
	if result[1].ID != "gemini-cli" || result[1].Status != aitools.StatusNotInstalled {
		t.Errorf("result[1] = %+v", result[1])
	}
}

func TestStatusAPI_ErrorMessageIncluded(t *testing.T) {
	tools := []aitools.Configurer{
		&fakeTool{id: "broken", detectResult: aitools.AITool{
			ID: "broken", Status: aitools.StatusError, ErrorMessage: "disk read failed",
		}},
	}
	mux := buildAIToolsMux(t, tools, &fakeAIUserStore{token: "tok"}, true)
	cookie := signedCookie(t, "user-1", "developer")

	rr := doReq(mux, http.MethodGet, "/api/tools", cookie)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "disk read failed") {
		t.Errorf("error_message missing from response: %s", rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// POST /api/tools/{id}/configure
// ---------------------------------------------------------------------------

func TestConfigureAPI_NonLocalMode_Returns503(t *testing.T) {
	tools := []aitools.Configurer{
		&fakeTool{id: "claude-desktop", detectResult: aitools.AITool{
			ID: "claude-desktop", Status: aitools.StatusUnconfigured,
		}},
	}
	mux := buildAIToolsMux(t, tools, &fakeAIUserStore{token: "tok"}, false)
	cookie := signedCookie(t, "user-1", "developer")

	rr := doReq(mux, http.MethodPost, "/api/tools/claude-desktop/configure", cookie)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rr.Code)
	}
}

func TestConfigureAPI_RequiresAuth(t *testing.T) {
	mux := buildAIToolsMux(t, nil, &fakeAIUserStore{}, true)
	rr := doReq(mux, http.MethodPost, "/api/tools/claude-desktop/configure", nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (API path returns JSON error, not redirect)", rr.Code)
	}
}

func TestConfigureAPI_UnknownTool_Returns400(t *testing.T) {
	mux := buildAIToolsMux(t, nil, &fakeAIUserStore{token: "tok"}, true)
	cookie := signedCookie(t, "user-1", "developer")

	rr := doReq(mux, http.MethodPost, "/api/tools/no-such-tool/configure", cookie)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestConfigureAPI_NotInstalled_Returns404(t *testing.T) {
	tools := []aitools.Configurer{
		&fakeTool{id: "claude-desktop", detectResult: aitools.AITool{
			ID: "claude-desktop", Status: aitools.StatusNotInstalled,
		}},
	}
	mux := buildAIToolsMux(t, tools, &fakeAIUserStore{token: "tok"}, true)
	cookie := signedCookie(t, "user-1", "developer")

	rr := doReq(mux, http.MethodPost, "/api/tools/claude-desktop/configure", cookie)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestConfigureAPI_Success_Returns200WithUpdatedStatus(t *testing.T) {
	tool := &fakeTool{id: "claude-desktop", detectResult: aitools.AITool{
		ID: "claude-desktop", DisplayName: "Claude Desktop", Status: aitools.StatusUnconfigured,
	}}
	mux := buildAIToolsMux(t, []aitools.Configurer{tool}, &fakeAIUserStore{token: "mytoken"}, true)
	cookie := signedCookie(t, "user-1", "developer")

	rr := doReq(mux, http.MethodPost, "/api/tools/claude-desktop/configure", cookie)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	if !tool.configured {
		t.Error("Configure was not called on the tool")
	}

	var result aitools.AITool
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Status != aitools.StatusConfigured {
		t.Errorf("result.Status = %q, want configured", result.Status)
	}
}

func TestConfigureAPI_ConfigureFails_Returns500(t *testing.T) {
	tool := &fakeTool{
		id:           "claude-desktop",
		configureErr: errFake("write permission denied"),
		detectResult: aitools.AITool{ID: "claude-desktop", Status: aitools.StatusUnconfigured},
	}
	mux := buildAIToolsMux(t, []aitools.Configurer{tool}, &fakeAIUserStore{token: "tok"}, true)
	cookie := signedCookie(t, "user-1", "developer")

	rr := doReq(mux, http.MethodPost, "/api/tools/claude-desktop/configure", cookie)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "failed to write config") {
		t.Errorf("error body = %s, want 'failed to write config'", rr.Body.String())
	}
}

func TestConfigureAPI_MCPURLContainsProxyToken(t *testing.T) {
	var gotURL string
	tool := &fakeTool{
		id:           "claude-desktop",
		detectResult: aitools.AITool{ID: "claude-desktop", Status: aitools.StatusUnconfigured},
	}
	// Wrap Configure to capture the URL passed in.
	wrapped := &captureTool{fakeTool: tool, onConfigure: func(u string) { gotURL = u }}

	mux := buildAIToolsMux(t, []aitools.Configurer{wrapped}, &fakeAIUserStore{token: "secret-token"}, true)
	cookie := signedCookie(t, "user-1", "developer")
	doReq(mux, http.MethodPost, "/api/tools/claude-desktop/configure", cookie)

	if !strings.Contains(gotURL, "secret-token") {
		t.Errorf("mcpURL = %q, want it to contain the user's proxy token", gotURL)
	}
	if !strings.HasSuffix(gotURL, "/mcp/secret-token") {
		t.Errorf("mcpURL = %q, want suffix /mcp/secret-token", gotURL)
	}
}

// ---------------------------------------------------------------------------
// Additional fakes
// ---------------------------------------------------------------------------

type errFake string

func (e errFake) Error() string { return string(e) }

// captureTool wraps a fakeTool and records the mcpURL passed to Configure.
type captureTool struct {
	*fakeTool
	onConfigure func(string)
}

func (c *captureTool) Configure(mcpURL string) error {
	c.onConfigure(mcpURL)
	return c.fakeTool.Configure(mcpURL)
}
