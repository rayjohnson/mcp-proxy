//go:build integration

package integration

import (
	"context"
	"os"
	"testing"

	"github.com/rayjohnson/mcp-proxy/internal/store"
)

// TestSuggestionDismiss verifies that a dismissed suggestion is not returned
// by ListPendingSuggestionsForUser on subsequent fetches.
func TestSuggestionDismiss(t *testing.T) {
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
	catalogStore := store.NewCatalogStore(pool)
	suggestionStore := store.NewSuggestionStore(pool)

	admin, err := userStore.CreateUser(ctx, "admin-suggest@example.com", "$2a$10$placeholder", "admin")
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM users WHERE id = $1", admin.ID) }) //nolint:errcheck

	dev, err := userStore.CreateUser(ctx, "dev-suggest@example.com", "$2a$10$placeholder", "developer")
	if err != nil {
		t.Fatalf("create dev: %v", err)
	}
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM users WHERE id = $1", dev.ID) }) //nolint:errcheck

	entry, err := catalogStore.AddCatalogEntry(ctx,
		"notion-suggest", "https://mcp.notion.com", "Notion MCP", "Notion tools", admin.ID)
	if err != nil {
		t.Fatalf("add catalog entry: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM default_catalog WHERE id = $1", entry.ID) //nolint:errcheck
	})

	if err := suggestionStore.CreateSuggestionForAllUsers(ctx, entry.ID); err != nil {
		t.Fatalf("create suggestions: %v", err)
	}

	// List pending suggestions — should contain one entry.
	suggestions, err := suggestionStore.ListPendingSuggestionsForUser(ctx, dev.ID)
	if err != nil {
		t.Fatalf("list suggestions: %v", err)
	}

	var suggestionID string
	for _, s := range suggestions {
		if s.CatalogID == entry.ID {
			suggestionID = s.ID
			break
		}
	}
	if suggestionID == "" {
		t.Fatalf("suggestion for catalog entry %q not found", entry.ID)
	}

	// Dismiss the suggestion.
	if err := suggestionStore.ResolveSuggestion(ctx, suggestionID, dev.ID, "dismissed"); err != nil {
		t.Fatalf("dismiss suggestion: %v", err)
	}

	// Re-fetch — dismissed suggestion must not appear.
	suggestions2, err := suggestionStore.ListPendingSuggestionsForUser(ctx, dev.ID)
	if err != nil {
		t.Fatalf("re-list suggestions: %v", err)
	}
	for _, s := range suggestions2 {
		if s.ID == suggestionID {
			t.Errorf("dismissed suggestion %q still appears in pending list", suggestionID)
		}
	}

	// Attempting to dismiss again must return an error (already resolved).
	err = suggestionStore.ResolveSuggestion(ctx, suggestionID, dev.ID, "dismissed")
	if err == nil {
		t.Error("expected error when dismissing an already-resolved suggestion, got nil")
	}
}
