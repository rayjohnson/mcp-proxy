//go:build integration

package integration

import (
	"context"
	"os"
	"testing"

	"github.com/rayjohnson/mcp-proxy/internal/store"
)

// TestCatalogSuggestions verifies that adding a catalog entry creates pending
// suggestions for all existing developer users, and that a newly registered
// user can list the active catalog entries.
func TestCatalogSuggestions(t *testing.T) {
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

	// Create an admin user to serve as the catalog entry author.
	admin, err := userStore.CreateUser(ctx, "admin-catalog@example.com", "$2a$10$placeholder", "admin")
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM users WHERE id = $1", admin.ID) }) //nolint:errcheck

	// Create an existing developer user who should receive the suggestion.
	dev, err := userStore.CreateUser(ctx, "dev-catalog@example.com", "$2a$10$placeholder", "developer")
	if err != nil {
		t.Fatalf("create dev: %v", err)
	}
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM users WHERE id = $1", dev.ID) }) //nolint:errcheck

	// Admin adds a catalog entry.
	entry, err := catalogStore.AddCatalogEntry(ctx,
		"github", "https://mcp.github.com", "GitHub MCP", "GitHub tools", admin.ID)
	if err != nil {
		t.Fatalf("add catalog entry: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM default_catalog WHERE id = $1", entry.ID) //nolint:errcheck
	})

	// Create suggestions for all existing developers.
	if err := suggestionStore.CreateSuggestionForAllUsers(ctx, entry.ID); err != nil {
		t.Fatalf("create suggestions: %v", err)
	}

	// Assert the existing developer has a pending suggestion.
	suggestions, err := suggestionStore.ListPendingSuggestionsForUser(ctx, dev.ID)
	if err != nil {
		t.Fatalf("list suggestions: %v", err)
	}
	found := false
	for _, s := range suggestions {
		if s.CatalogID == entry.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected suggestion for catalog entry %q, got %v", entry.ID, suggestions)
	}

	// Assert re-creating suggestions is idempotent (ON CONFLICT DO NOTHING).
	if err := suggestionStore.CreateSuggestionForAllUsers(ctx, entry.ID); err != nil {
		t.Fatalf("idempotent create suggestions: %v", err)
	}
	suggestions2, _ := suggestionStore.ListPendingSuggestionsForUser(ctx, dev.ID)
	if len(suggestions2) != len(suggestions) {
		t.Errorf("expected %d suggestions after idempotent call, got %d", len(suggestions), len(suggestions2))
	}

	// Assert a newly registered user can list active catalog entries.
	newDev, err := userStore.CreateUser(ctx, "newdev-catalog@example.com", "$2a$10$placeholder", "developer")
	if err != nil {
		t.Fatalf("create new dev: %v", err)
	}
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM users WHERE id = $1", newDev.ID) }) //nolint:errcheck

	catalog, err := catalogStore.ListActiveCatalogEntries(ctx)
	if err != nil {
		t.Fatalf("list catalog: %v", err)
	}
	foundInCatalog := false
	for _, e := range catalog {
		if e.ID == entry.ID {
			foundInCatalog = true
			break
		}
	}
	if !foundInCatalog {
		t.Errorf("expected catalog entry %q to be visible to new user", entry.ID)
	}
}
