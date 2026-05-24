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

// TestProxyRoundtrip starts a fake upstream MCP server, wires it through the
// proxy, and asserts that tools/list returns prefixed tool names.
func TestProxyRoundtrip(t *testing.T) {
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

	// Start a fake upstream MCP server with one tool.
	fakeUpstream := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "fake", Version: "1.0"}, nil)
	fakeUpstream.AddTool(&sdkmcp.Tool{
		Name:        "hello",
		Description: "Say hello",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(_ context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "hello from fake"}},
		}, nil
	})
	upstreamSrv := httptest.NewServer(sdkmcp.NewStreamableHTTPHandler(
		func(*http.Request) *sdkmcp.Server { return fakeUpstream },
		nil,
	))
	defer upstreamSrv.Close()

	// Create test user.
	user, err := userStore.CreateUser(ctx, "roundtrip-test@example.com", "$2a$10$placeholder", "developer")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID) //nolint:errcheck
	})

	// Insert upstream config pointing to the fake server.
	cfg, err := upstreamStore.CreateUpstreamConfig(ctx,
		user.ID, "fakeservice", upstreamSrv.URL, "api_key",
		[]byte("plaintext-stub"), // not encrypted — test skips KMS
	)
	if err != nil {
		t.Fatalf("create upstream config: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM upstream_configs WHERE id = $1", cfg.ID) //nolint:errcheck
	})

	// Build proxy server.
	sessionDeps := internalmcp.SessionDeps{
		UpstreamStore: upstreamStore,
		KMSDecrypt:    func(_ context.Context, ciphertext []byte) ([]byte, error) { return ciphertext, nil },
		AuthHeader: func(c *store.UpstreamConfig, plainCreds []byte) (string, error) {
			adapter, err := upstream.GetAdapter(c.ServerType)
			if err != nil {
				// No adapter registered for test service — return empty auth header.
				return "", nil
			}
			return adapter.AuthHeader(plainCreds)
		},
		UpdateTransport: func(ctx context.Context, id, transport string) error {
			return upstreamStore.UpdateDetectedTransport(ctx, id, transport)
		},
	}

	proxySrv := httptest.NewServer(sdkmcp.NewStreamableHTTPHandler(
		internalmcp.GetServerFunc(internalmcp.ProxyServerDeps{
			UserStore:   userStore,
			SessionDeps: sessionDeps,
		}),
		nil,
	))
	defer proxySrv.Close()

	// Connect MCP client to proxy using the user's proxy token.
	proxyURL := proxySrv.URL + "/mcp/" + user.ProxyToken
	transport := &sdkmcp.StreamableClientTransport{Endpoint: proxyURL}
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "1.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect to proxy: %v", err)
	}
	defer session.Close()

	// List tools and verify they are prefixed.
	result, err := session.ListTools(ctx, &sdkmcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	if len(result.Tools) == 0 {
		t.Fatal("expected at least one tool, got none")
	}
	want := "fakeservice__hello"
	found := false
	for _, tool := range result.Tools {
		if tool.Name == want {
			found = true
			break
		}
	}
	if !found {
		names := make([]string, len(result.Tools))
		for i, tool := range result.Tools {
			names[i] = tool.Name
		}
		t.Errorf("tool %q not found; got %v", want, names)
	}
}
