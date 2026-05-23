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
)

// TestFailureIsolation verifies that one unreachable upstream does not prevent
// tools from other upstreams from being listed and called.
func TestFailureIsolation(t *testing.T) {
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

	// Start a working upstream server.
	workingSrv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "working", Version: "1.0"}, nil)
	workingSrv.AddTool(&sdkmcp.Tool{
		Name:        "alive",
		Description: "A tool on the working server",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(_ context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "alive"}},
		}, nil
	})
	workingHTTP := httptest.NewServer(sdkmcp.NewStreamableHTTPHandler(
		func(*http.Request) *sdkmcp.Server { return workingSrv },
		nil,
	))
	defer workingHTTP.Close()

	// Start a server then immediately close it to simulate an unreachable upstream.
	deadHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "dead", http.StatusServiceUnavailable)
	}))
	deadURL := deadHTTP.URL
	deadHTTP.Close() // close immediately so connections fail

	user, err := userStore.CreateUser(ctx, "isolation-test@example.com", "$2a$10$placeholder", "developer")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID) }) //nolint:errcheck

	cfgWorking, err := upstreamStore.CreateUpstreamConfig(ctx,
		user.ID, "working", workingHTTP.URL, "api_key", []byte("key"))
	if err != nil {
		t.Fatalf("create working config: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM upstream_configs WHERE id = $1", cfgWorking.ID) //nolint:errcheck
	})

	cfgDead, err := upstreamStore.CreateUpstreamConfig(ctx,
		user.ID, "dead", deadURL, "api_key", []byte("key"))
	if err != nil {
		t.Fatalf("create dead config: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM upstream_configs WHERE id = $1", cfgDead.ID) //nolint:errcheck
	})

	sessionDeps := internalmcp.SessionDeps{
		UpstreamStore: upstreamStore,
		KMSDecrypt:    func(_ context.Context, b []byte) ([]byte, error) { return b, nil },
		AuthHeader:    func(_ *store.UpstreamConfig, _ []byte) (string, error) { return "", nil },
		UpdateTransport: func(ctx context.Context, id, transport string) error {
			return upstreamStore.UpdateDetectedTransport(ctx, id, transport)
		},
	}

	proxyHandler := sdkmcp.NewStreamableHTTPHandler(
		internalmcp.GetServerFunc(internalmcp.ProxyServerDeps{
			UserStore:   userStore,
			SessionDeps: sessionDeps,
		}),
		nil,
	)
	proxySrv := httptest.NewServer(proxyHandler)
	defer proxySrv.Close()

	transport := &sdkmcp.StreamableClientTransport{Endpoint: proxySrv.URL + "/mcp/" + user.ProxyToken}
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "1.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect to proxy: %v", err)
	}
	defer session.Close()

	result, err := session.ListTools(ctx, &sdkmcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	// Working server's tool must appear.
	found := false
	for _, tool := range result.Tools {
		if tool.Name == "working__alive" {
			found = true
		}
		// Dead server's tools must NOT appear.
		if tool.Name == "dead__alive" {
			t.Errorf("dead server tool appeared in tool list")
		}
	}
	if !found {
		names := make([]string, len(result.Tools))
		for i, t := range result.Tools {
			names[i] = t.Name
		}
		t.Errorf("working__alive not found in tools: %v", names)
	}
}
