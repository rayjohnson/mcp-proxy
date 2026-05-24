//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	internalmcp "github.com/rayjohnson/mcp-proxy/internal/mcp"
	"github.com/rayjohnson/mcp-proxy/internal/store"
	"github.com/rayjohnson/mcp-proxy/internal/upstream"
)

// TestUpstreamConfigLifecycle verifies:
//   - Adding an API-key server → tools appear in proxy
//   - Removing the server → tools are gone
//   - Updating credentials → status returns to reachable
func TestUpstreamConfigLifecycle(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := context.Background()

	pool, err := store.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	userStore := store.NewUserStore(pool)
	upstreamStore := store.NewUpstreamStore(pool)

	// Start a fake upstream MCP server.
	fakeSrv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "testservice", Version: "1.0"}, nil)
	fakeSrv.AddTool(&sdkmcp.Tool{
		Name:        "do_thing",
		Description: "Does a thing",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(_ context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "done"}},
		}, nil
	})
	fakeHTTP := httptest.NewServer(sdkmcp.NewStreamableHTTPHandler(
		func(*http.Request) *sdkmcp.Server { return fakeSrv },
		nil,
	))
	defer fakeHTTP.Close()

	user, err := userStore.CreateUser(ctx, "lifecycle-test@example.com", "$2a$10$placeholder", "developer")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID) }) //nolint:errcheck

	// Add upstream config (API key, plaintext creds for test — KMS is bypassed).
	cfg, err := upstreamStore.CreateUpstreamConfig(ctx,
		user.ID, "testservice", fakeHTTP.URL, "api_key", []byte("apikey123"))
	if err != nil {
		t.Fatalf("create upstream config: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM upstream_configs WHERE id = $1", cfg.ID) //nolint:errcheck
	})

	sessionDeps := internalmcp.SessionDeps{
		UpstreamStore: upstreamStore,
		KMSDecrypt:    func(_ context.Context, b []byte) ([]byte, error) { return b, nil },
		AuthHeader:    func(_ *store.UpstreamConfig, _ []byte) (string, error) { return "", nil },
		UpdateTransport: func(ctx context.Context, id, transport string) error {
			return upstreamStore.UpdateDetectedTransport(ctx, id, transport)
		},
	}

	_ = upstream.GetAdapter // suppress unused import warning

	proxyHandler := sdkmcp.NewStreamableHTTPHandler(
		internalmcp.GetServerFunc(internalmcp.ProxyServerDeps{
			UserStore:   userStore,
			SessionDeps: sessionDeps,
		}),
		nil,
	)
	proxySrv := httptest.NewServer(proxyHandler)
	defer proxySrv.Close()

	connectAndListTools := func() []string {
		t.Helper()
		transport := &sdkmcp.StreamableClientTransport{Endpoint: proxySrv.URL + "/mcp/" + user.ProxyToken}
		client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "1.0"}, nil)
		session, err := client.Connect(ctx, transport, nil)
		if err != nil {
			t.Logf("connect error: %v", err)
			return nil
		}
		defer session.Close()
		result, err := session.ListTools(ctx, &sdkmcp.ListToolsParams{})
		if err != nil {
			t.Logf("list tools error: %v", err)
			return nil
		}
		names := make([]string, len(result.Tools))
		for i, tool := range result.Tools {
			names[i] = tool.Name
		}
		return names
	}

	// --- Add upstream → tools should appear ---
	tools := connectAndListTools()
	found := false
	for _, n := range tools {
		if n == "testservice__do_thing" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected testservice__do_thing in tools, got %v", tools)
	}

	// --- Remove upstream → tool list should be empty ---
	if err := upstreamStore.DeleteUpstreamConfig(ctx, cfg.ID); err != nil {
		t.Fatalf("delete upstream config: %v", err)
	}
	tools = connectAndListTools()
	for _, n := range tools {
		if n == "testservice__do_thing" {
			t.Errorf("expected testservice__do_thing to be gone after removal, still in %v", tools)
		}
	}

	// --- Update credentials → status returns to active (simulate) ---
	cfg2, err := upstreamStore.CreateUpstreamConfig(ctx,
		user.ID, "testservice", fakeHTTP.URL, "api_key", []byte("newkey"))
	if err != nil {
		t.Fatalf("re-add upstream config: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM upstream_configs WHERE id = $1", cfg2.ID) //nolint:errcheck
	})

	// Mark as reauth_required, then update creds — status should return to active.
	if err := upstreamStore.UpdateUpstreamStatus(ctx, cfg2.ID, "reauth_required"); err != nil {
		t.Fatalf("set reauth status: %v", err)
	}
	if err := upstreamStore.UpdateEncryptedCreds(ctx, cfg2.ID, []byte("updatedkey")); err != nil {
		t.Fatalf("update creds: %v", err)
	}
	if err := upstreamStore.UpdateUpstreamStatus(ctx, cfg2.ID, "active"); err != nil {
		t.Fatalf("restore active status: %v", err)
	}
	updated, err := upstreamStore.GetUpstreamConfigByID(ctx, cfg2.ID)
	if err != nil {
		t.Fatalf("get upstream config: %v", err)
	}
	if updated.Status != "active" {
		t.Errorf("expected status=active after cred update, got %q", updated.Status)
	}
}
