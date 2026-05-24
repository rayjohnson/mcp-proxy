# Data Model: Local Deployment Mode with stdio MCP Server Support

## Schema Changes

### `default_catalog` (extended)

New columns added to the existing table (both Postgres migration and SQLite DDL):

| Column | Type | Default | Nullable | Description |
|--------|------|---------|----------|-------------|
| `transport` | TEXT | `'http'` | NOT NULL | Transport protocol: `http` or `stdio` |
| `command` | TEXT | NULL | YES | Executable path/name for stdio servers (e.g., `npx`) |
| `args` | JSONB / TEXT | `'[]'` | NOT NULL | JSON array of command arguments |
| `env` | JSONB / TEXT | `'{}'` | NOT NULL | JSON object of environment variable overrides |

**Notes**:
- `transport = 'http'` is the default; all existing rows remain valid unchanged.
- `transport = 'stdio'` requires `command` to be non-null.
- `args` and `env` are JSONB in Postgres, TEXT (JSON-encoded) in SQLite.
- `oauth_client_id` and `encrypted_oauth_secret` remain optional for backward compat; they are unused for `transport = 'stdio'` entries.

### Updated `CatalogEntry` Go Struct

```go
type CatalogEntry struct {
    ID                   string
    ServerType           string
    ServerURL            string      // empty string for stdio entries
    DisplayName          string
    Description          *string
    AddedBy              string
    Active               bool
    AuthType             string      // "api_key", "pat", "oauth2"
    OAuthClientID        *string
    EncryptedOAuthSecret []byte
    Transport            string      // "http" (default) or "stdio"
    Command              *string     // stdio only
    Args                 []string    // stdio only, default []
    Env                  map[string]string // stdio only, default {}
}
```

---

## New Interfaces

### `internal/store/interfaces.go`

Interfaces extracted from existing concrete types. All existing store methods are preserved exactly; no signatures change.

```go
type UserStore interface {
    CreateUser(ctx, email, passwordHash, role string) (*User, error)
    GetUserByEmail(ctx, email string) (*User, error)
    GetUserByID(ctx, id string) (*User, error)
    GetUserByProxyToken(ctx, token string) (*User, error)
    ListAllUsers(ctx) ([]*User, error)
    CountUsers(ctx) (int, error)
    UpdateUserRole(ctx, id, role string) error
}

type CatalogStore interface {
    AddCatalogEntry(ctx, serverType, serverURL, displayName, description, addedBy,
        authType, transport string, command *string, args []string,
        env map[string]string, oauthClientID *string, encryptedOAuthSecret []byte) (*CatalogEntry, error)
    ListActiveCatalogEntries(ctx) ([]*CatalogEntry, error)
    GetCatalogEntryByServerType(ctx, serverType string) (*CatalogEntry, error)
    GetCatalogEntryByID(ctx, id string) (*CatalogEntry, error)
    DeactivateCatalogEntry(ctx, id string) error
}

type UpstreamStore interface {
    CreateUpstreamConfig(ctx, userID, serverType, serverURL, authType string, encryptedCreds []byte) (*UpstreamConfig, error)
    GetUpstreamConfigsByUserID(ctx, userID string) ([]*UpstreamConfig, error)
    GetUpstreamConfigByID(ctx, id string) (*UpstreamConfig, error)
    UpdateUpstreamStatus(ctx, id, status string) error
    UpdateDetectedTransport(ctx, id, transport string) error
    UpdateEncryptedCreds(ctx, id string, encryptedCreds []byte) error
    DeleteUpstreamConfig(ctx, id string) error
}

type OAuth2StateStore interface {
    CreateState(ctx, userID, serverType, codeVerifier string) (string, error)
    ConsumeState(ctx, state string) (*OAuth2State, error)
}

type SuggestionStore interface {
    RecordSuggestionSearch(ctx, userID, query string, resultCount int) error
    ListRecentSuggestionSearches(ctx, userID string, limit int) ([]*SuggestionSearch, error)
}
```

---

## Configuration Extensions

### `internal/config/Config` (extended)

```go
type Config struct {
    // existing fields ...
    Port       string
    DBDSN      string
    KMSKeyName string
    BaseURL    string

    // New fields
    LocalMode bool   // true when LOCAL_MODE=true
    DataDir   string // directory for local SQLite file; defaults to "."
}
```

**Validation changes in `Load()`**:
- `DB_DSN` is no longer required when `LOCAL_MODE=true`; defaults to `file:{DataDir}/mcp-proxy.db`
- `KMS_KEY_NAME` and `BASE_URL` remain required in all modes

---

## Database File Layout (local mode)

```
{DataDir}/
└── mcp-proxy.db    # SQLite database; created on first run
```

Default `DataDir` is the current working directory (`.`). Override with `DATA_DIR=/path/to/dir`.

---

## Postgres Migration

```sql
-- Migration: add stdio/transport columns to default_catalog
ALTER TABLE default_catalog
    ADD COLUMN transport TEXT NOT NULL DEFAULT 'http',
    ADD COLUMN command TEXT,
    ADD COLUMN args JSONB NOT NULL DEFAULT '[]',
    ADD COLUMN env JSONB NOT NULL DEFAULT '{}';
```

## SQLite Schema (initial, created fresh on first run)

The full DDL matches the Postgres schema with SQLite-compatible types:
- `UUID` → `TEXT`
- `JSONB` → `TEXT` (JSON-encoded)
- `now()` → `CURRENT_TIMESTAMP`
- `$N` placeholders → `?`
- `RETURNING` clause supported in SQLite ≥ 3.35 (go-sqlite3 bundles 3.45+)
