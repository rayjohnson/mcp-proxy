//go:build integration

package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	internalmcp "github.com/rayjohnson/mcp-proxy/internal/mcp"
	"github.com/rayjohnson/mcp-proxy/internal/store"
)

// TestTransportDetection starts a fake upstream that only speaks SSE (not
// Streamable HTTP), runs OpenSession, and asserts that detected_transport is
// set to "sse" in the database.
func TestTransportDetection(t *testing.T) {
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

	// Start a fake upstream that only serves SSE (rejecting Streamable HTTP POSTs).
	fakeUpstream := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "sse-only", Version: "1.0"}, nil)
	sseHandler := sdkmcp.NewSSEHandler(func(*http.Request) *sdkmcp.Server { return fakeUpstream }, nil)
	upstreamSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Reject POST so Streamable HTTP transport falls back to SSE.
		if r.Method == http.MethodPost {
			http.Error(w, "not supported", http.StatusMethodNotAllowed)
			return
		}
		sseHandler.ServeHTTP(w, r)
	}))
	defer upstreamSrv.Close()

	// Create test user.
	user, err := userStore.CreateUser(ctx, "transport-test@example.com", "$2a$10$placeholder", "developer")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID) //nolint:errcheck
	})

	// Insert upstream config with no cached transport.
	cfg, err := upstreamStore.CreateUpstreamConfig(ctx,
		user.ID, "sseonly", upstreamSrv.URL, "api_key",
		[]byte("plaintext-stub"),
	)
	if err != nil {
		t.Fatalf("create upstream config: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM upstream_configs WHERE id = $1", cfg.ID) //nolint:errcheck
	})

	var detectedTransport string
	sessionDeps := internalmcp.SessionDeps{
		UpstreamStore: upstreamStore,
		KMSDecrypt:    func(_ context.Context, ciphertext []byte) ([]byte, error) { return ciphertext, nil },
		AuthHeader:    func(_ *store.UpstreamConfig, _ []byte) (string, error) { return "", nil },
		UpdateTransport: func(ctx context.Context, id, transport string) error {
			detectedTransport = transport
			return upstreamStore.UpdateDetectedTransport(ctx, id, transport)
		},
	}

	ps, err := internalmcp.OpenSession(ctx, user.ID, sessionDeps)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}
	defer ps.Close()

	if detectedTransport != internalmcp.TransportSSE {
		t.Errorf("detected_transport = %q, want %q", detectedTransport, internalmcp.TransportSSE)
	}

	// Confirm it's persisted in the DB.
	updated, err := upstreamStore.GetUpstreamConfigByID(ctx, cfg.ID)
	if err != nil {
		t.Fatalf("get upstream config: %v", err)
	}
	if updated.DetectedTransport == nil || *updated.DetectedTransport != internalmcp.TransportSSE {
		var got string
		if updated.DetectedTransport != nil {
			got = *updated.DetectedTransport
		}
		t.Errorf("DB detected_transport = %q, want %q", got, internalmcp.TransportSSE)
	}
}
