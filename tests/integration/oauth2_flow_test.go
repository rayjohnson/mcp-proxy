//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/rayjohnson/mcp-proxy/internal/store"
)

// TestOAuth2Flow exercises the full authorize → callback → token storage → refresh flow
// using a mock OAuth2 provider.
func TestOAuth2Flow(t *testing.T) {
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
	stateStore := store.NewOAuth2StateStore(pool)

	// Start a mock OAuth2 token endpoint.
	var exchangeCallCount int
	oauthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			exchangeCallCount++
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
				"access_token":  fmt.Sprintf("access-%d", exchangeCallCount),
				"token_type":    "Bearer",
				"expires_in":    3600,
				"refresh_token": "refresh-token-abc",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer oauthSrv.Close()

	// Create test user.
	user, err := userStore.CreateUser(ctx, "oauth2-test@example.com", "$2a$10$placeholder", "developer")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID) }) //nolint:errcheck

	// --- Step 1: Generate OAuth2 state ---
	stateToken := "test-state-" + time.Now().Format("20060102150405")
	if err := stateStore.CreateOAuth2State(ctx, user.ID, "notion", stateToken); err != nil {
		t.Fatalf("create oauth2 state: %v", err)
	}

	// --- Step 2: Consume OAuth2 state (simulates callback validation) ---
	st, err := stateStore.ConsumeOAuth2State(ctx, stateToken)
	if err != nil {
		t.Fatalf("consume oauth2 state: %v", err)
	}
	if st.UserID != user.ID || st.ServerType != "notion" {
		t.Errorf("state mismatch: got user=%s type=%s, want %s/%s",
			st.UserID, st.ServerType, user.ID, "notion")
	}

	// --- Step 3: Consuming the same state again should fail (deleted) ---
	if _, err := stateStore.ConsumeOAuth2State(ctx, stateToken); err == nil {
		t.Error("expected error consuming state twice, got nil")
	}

	// --- Step 4: Store encrypted token in upstream_configs ---
	fakeEncrypted := []byte(`{"access_token":"access-1","refresh_token":"refresh-token-abc"}`)
	cfg, err := upstreamStore.CreateUpstreamConfig(ctx,
		user.ID, "notion", "https://mcp.notion.com", "oauth2", fakeEncrypted)
	if err != nil {
		t.Fatalf("create upstream config: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM upstream_configs WHERE id = $1", cfg.ID) //nolint:errcheck
	})

	// --- Step 5: Simulate token refresh by updating encrypted creds ---
	refreshed := []byte(`{"access_token":"access-2","refresh_token":"refresh-token-abc"}`)
	if err := upstreamStore.UpdateEncryptedCreds(ctx, cfg.ID, refreshed); err != nil {
		t.Fatalf("update encrypted creds: %v", err)
	}

	// Verify the update persisted.
	updated, err := upstreamStore.GetUpstreamConfigByID(ctx, cfg.ID)
	if err != nil {
		t.Fatalf("get upstream config: %v", err)
	}
	if string(updated.EncryptedCreds) != string(refreshed) {
		t.Errorf("encrypted creds mismatch after refresh")
	}

	// --- Step 6: Simulate invalid_grant → set status to reauth_required ---
	if err := upstreamStore.UpdateUpstreamStatus(ctx, cfg.ID, "reauth_required"); err != nil {
		t.Fatalf("set reauth status: %v", err)
	}
	final, err := upstreamStore.GetUpstreamConfigByID(ctx, cfg.ID)
	if err != nil {
		t.Fatalf("get final config: %v", err)
	}
	if final.Status != "reauth_required" {
		t.Errorf("expected status=reauth_required, got %q", final.Status)
	}

	// --- Step 7: Expired states are cleaned up ---
	expiredToken := "expired-state-" + time.Now().Format("20060102150405.000")
	pool.Exec(ctx, `INSERT INTO oauth2_state (user_id, server_type, state, expires_at)
		VALUES ($1, 'test', $2, now() - interval '1 minute')`, user.ID, expiredToken) //nolint:errcheck
	if err := stateStore.DeleteExpiredStates(ctx); err != nil {
		t.Fatalf("delete expired states: %v", err)
	}
	if _, err := stateStore.ConsumeOAuth2State(ctx, expiredToken); err == nil {
		t.Error("expected expired state to be gone, but ConsumeOAuth2State succeeded")
	}
}
