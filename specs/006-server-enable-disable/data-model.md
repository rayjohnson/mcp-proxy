# Data Model: Per-User Server Enable/Disable

## Modified Entity: UpstreamConfig

Add `Enabled bool` field (default `true`).

| Field | Type | Notes |
|-------|------|-------|
| ID | string | existing |
| UserID | string | existing |
| ServerType | string | existing |
| ServerURL | string | existing |
| AuthType | string | existing |
| EncryptedCreds | []byte | existing |
| DetectedTransport | *string | existing |
| Status | string | existing |
| StatusCheckedAt | *time.Time | existing |
| **Enabled** | **bool** | **NEW — default true** |

### Migration change (SQLite)

```sql
ALTER TABLE upstream_configs ADD COLUMN enabled INTEGER NOT NULL DEFAULT 1;
```

### Migration change (PostgreSQL)

```sql
ALTER TABLE upstream_configs ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT TRUE;
```

---

## New Entity: ServerToggle

Per-user enabled/disabled state for stdio catalog entries.

| Field | Type | Notes |
|-------|------|-------|
| ID | string | primary key |
| UserID | string | FK → users.id ON DELETE CASCADE |
| CatalogID | string | FK → default_catalog.id ON DELETE CASCADE |
| Enabled | bool | default true |
| CreatedAt | time.Time | |
| UpdatedAt | time.Time | |

**Unique constraint**: `(user_id, catalog_id)`

**Default behaviour**: No row = enabled (absence means enabled). A row is only written on the first toggle to disabled, or explicitly on re-enable. This keeps the table sparse.

### SQL (SQLite)

```sql
CREATE TABLE IF NOT EXISTS server_toggles (
    id         TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    catalog_id TEXT NOT NULL REFERENCES default_catalog(id) ON DELETE CASCADE,
    enabled    INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, catalog_id)
);
```

---

## Store Interface Extensions

### UpstreamStoreI (addition)

```go
ToggleUpstream(ctx context.Context, id string) (bool, error)
// Flips enabled↔disabled for the given upstream config.
// Returns the new enabled state.
```

### ToggleStoreI (new interface)

```go
type ToggleStoreI interface {
    ToggleCatalogEntry(ctx context.Context, userID, catalogID string) (bool, error)
    // Flips enabled state for (userID, catalogID). Returns new enabled state.

    IsDisabledCatalogEntry(ctx context.Context, userID, catalogID string) (bool, error)
    // Returns true if the entry is explicitly disabled for this user.

    DisabledCatalogIDs(ctx context.Context, userID string) ([]string, error)
    // Returns catalog IDs the user has disabled. Used by session layer.
}
```

---

## State Transitions

```
HTTP Upstream:
  enabled=true  --[toggle]--> enabled=false  --[toggle]--> enabled=true

Stdio Catalog Entry (via server_toggles):
  no row (enabled)  --[toggle]--> row with enabled=0  --[toggle]--> row with enabled=1
```

Both transitions are idempotent: toggling twice returns to the original state.
