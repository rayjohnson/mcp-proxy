package sqlite_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	sqstore "github.com/rayjohnson/mcp-proxy/internal/store/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := sqstore.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() }) //nolint:errcheck
	return db
}

// seedUser inserts a minimal user and returns its ID.
func seedUser(t *testing.T, db *sql.DB) string {
	t.Helper()
	ctx := context.Background()
	us := sqstore.NewUserStore(db)
	u, err := us.CreateUser(ctx, "test@example.com", "hash", "developer")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u.ID
}

// seedCatalog inserts a minimal catalog entry and returns its ID.
func seedCatalog(t *testing.T, db *sql.DB) string {
	t.Helper()
	ctx := context.Background()
	cs := sqstore.NewCatalogStore(db)
	e, err := cs.AddCatalogEntry(ctx, "test-server", "", "Test", "", "test@example.com", "none", "stdio", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("seed catalog: %v", err)
	}
	return e.ID
}

// seedUpstream inserts an upstream config and returns its ID.
func seedUpstream(t *testing.T, db *sql.DB, userID string) string {
	t.Helper()
	ctx := context.Background()
	us := sqstore.NewUpstreamStore(db)
	c, err := us.CreateUpstreamConfig(ctx, userID, "test-server", "", "none", nil)
	if err != nil {
		t.Fatalf("seed upstream: %v", err)
	}
	return c.ID
}

// --- ToggleUpstream tests (T008) ---

func TestToggleUpstream_InitialStateTrue(t *testing.T) {
	db := openTestDB(t)
	userID := seedUser(t, db)
	_ = seedUpstream(t, db, userID)
	ctx := context.Background()

	us := sqstore.NewUpstreamStore(db)
	configs, err := us.GetUpstreamConfigsByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("list upstreams: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	if !configs[0].Enabled {
		t.Error("new upstream should default to enabled=true")
	}
}

func TestToggleUpstream_FlipToFalse(t *testing.T) {
	db := openTestDB(t)
	userID := seedUser(t, db)
	upstreamID := seedUpstream(t, db, userID)
	ctx := context.Background()

	us := sqstore.NewUpstreamStore(db)
	newVal, err := us.ToggleUpstream(ctx, upstreamID)
	if err != nil {
		t.Fatalf("toggle: %v", err)
	}
	if newVal {
		t.Error("expected toggle to return false (disabled)")
	}

	configs, err := us.GetUpstreamConfigsByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if configs[0].Enabled {
		t.Error("expected Enabled=false after toggle")
	}
}

func TestToggleUpstream_FlipBackToTrue(t *testing.T) {
	db := openTestDB(t)
	userID := seedUser(t, db)
	upstreamID := seedUpstream(t, db, userID)
	ctx := context.Background()

	us := sqstore.NewUpstreamStore(db)
	_, _ = us.ToggleUpstream(ctx, upstreamID) // false
	newVal, err := us.ToggleUpstream(ctx, upstreamID)
	if err != nil {
		t.Fatalf("toggle: %v", err)
	}
	if !newVal {
		t.Error("expected toggle to return true (re-enabled)")
	}

	configs, err := us.GetUpstreamConfigsByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !configs[0].Enabled {
		t.Error("expected Enabled=true after re-enable")
	}
}

// --- ToggleCatalogEntry / DisabledCatalogIDs tests (T009) ---

func TestToggleCatalogEntry_FirstToggleDisables(t *testing.T) {
	db := openTestDB(t)
	userID := seedUser(t, db)
	catID := seedCatalog(t, db)
	ctx := context.Background()

	ts := sqstore.NewToggleStore(db)
	newVal, err := ts.ToggleCatalogEntry(ctx, userID, catID)
	if err != nil {
		t.Fatalf("toggle: %v", err)
	}
	if newVal {
		t.Error("first toggle should disable (return false)")
	}
}

func TestToggleCatalogEntry_SecondToggleEnables(t *testing.T) {
	db := openTestDB(t)
	userID := seedUser(t, db)
	catID := seedCatalog(t, db)
	ctx := context.Background()

	ts := sqstore.NewToggleStore(db)
	_, _ = ts.ToggleCatalogEntry(ctx, userID, catID) // disable
	newVal, err := ts.ToggleCatalogEntry(ctx, userID, catID)
	if err != nil {
		t.Fatalf("toggle: %v", err)
	}
	if !newVal {
		t.Error("second toggle should re-enable (return true)")
	}
}

func TestDisabledCatalogIDs_SparseStorage(t *testing.T) {
	db := openTestDB(t)
	userID := seedUser(t, db)
	catID := seedCatalog(t, db)
	ctx := context.Background()

	ts := sqstore.NewToggleStore(db)

	// Before any toggle: no rows in server_toggles, entry is implicitly enabled.
	disabled, err := ts.DisabledCatalogIDs(ctx, userID)
	if err != nil {
		t.Fatalf("DisabledCatalogIDs: %v", err)
	}
	if len(disabled) != 0 {
		t.Error("expected empty disabled set before any toggle")
	}

	// After disabling:
	_, _ = ts.ToggleCatalogEntry(ctx, userID, catID)
	disabled, err = ts.DisabledCatalogIDs(ctx, userID)
	if err != nil {
		t.Fatalf("DisabledCatalogIDs: %v", err)
	}
	if _, ok := disabled[catID]; !ok {
		t.Error("catalog entry should be in disabled set after toggle")
	}

	// After re-enabling: row still exists but enabled=1, so not in disabled set.
	_, _ = ts.ToggleCatalogEntry(ctx, userID, catID)
	disabled, err = ts.DisabledCatalogIDs(ctx, userID)
	if err != nil {
		t.Fatalf("DisabledCatalogIDs: %v", err)
	}
	if len(disabled) != 0 {
		t.Error("expected empty disabled set after re-enable")
	}
}

func TestDisabledCatalogIDs_AbsentEntryIsEnabled(t *testing.T) {
	db := openTestDB(t)
	userID := seedUser(t, db)
	catID := seedCatalog(t, db)
	ctx := context.Background()

	ts := sqstore.NewToggleStore(db)
	disabled, err := ts.DisabledCatalogIDs(ctx, userID)
	if err != nil {
		t.Fatalf("DisabledCatalogIDs: %v", err)
	}
	if _, ok := disabled[catID]; ok {
		t.Error("catalog entry with no toggle row should be considered enabled")
	}
}

// --- Persistence tests (T022, T023) ---

func TestDisabledCatalogIDs_PersistsAfterReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "persist.db")
	ctx := context.Background()

	db1, err := sqstore.Open(ctx, path)
	if err != nil {
		t.Fatalf("open db1: %v", err)
	}
	us1 := sqstore.NewUserStore(db1)
	u, err := us1.CreateUser(ctx, "p@example.com", "h", "developer")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	cs1 := sqstore.NewCatalogStore(db1)
	e, err := cs1.AddCatalogEntry(ctx, "persist-server", "", "Persist", "", u.ID, "none", "stdio", nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("add catalog: %v", err)
	}
	ts1 := sqstore.NewToggleStore(db1)
	_, _ = ts1.ToggleCatalogEntry(ctx, u.ID, e.ID)
	db1.Close() //nolint:errcheck

	db2, err := sqstore.Open(ctx, path)
	if err != nil {
		t.Fatalf("open db2: %v", err)
	}
	defer db2.Close() //nolint:errcheck

	ts2 := sqstore.NewToggleStore(db2)
	disabled, err := ts2.DisabledCatalogIDs(ctx, u.ID)
	if err != nil {
		t.Fatalf("DisabledCatalogIDs: %v", err)
	}
	if _, ok := disabled[e.ID]; !ok {
		t.Error("disabled state should persist after DB close/reopen")
	}
}

func TestToggleUpstream_PersistsAfterReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "persist2.db")
	ctx := context.Background()

	db1, err := sqstore.Open(ctx, path)
	if err != nil {
		t.Fatalf("open db1: %v", err)
	}
	us1 := sqstore.NewUserStore(db1)
	u, err := us1.CreateUser(ctx, "p2@example.com", "h", "developer")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	upstreamStore1 := sqstore.NewUpstreamStore(db1)
	c, err := upstreamStore1.CreateUpstreamConfig(ctx, u.ID, "srv", "", "none", nil)
	if err != nil {
		t.Fatalf("create upstream: %v", err)
	}
	_, _ = upstreamStore1.ToggleUpstream(ctx, c.ID)
	db1.Close() //nolint:errcheck

	db2, err := sqstore.Open(ctx, path)
	if err != nil {
		t.Fatalf("open db2: %v", err)
	}
	defer db2.Close() //nolint:errcheck

	upstreamStore2 := sqstore.NewUpstreamStore(db2)
	configs, err := upstreamStore2.GetUpstreamConfigsByUserID(ctx, u.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	if configs[0].Enabled {
		t.Error("disabled state should persist after DB close/reopen")
	}
}
