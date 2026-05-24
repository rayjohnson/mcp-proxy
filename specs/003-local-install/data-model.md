# Data Model: Local Mode Packaging & Distribution

## New Entities

### InstallConfig (file: `~/Library/Application Support/mcp-proxy/config.json`)

Persists install-time decisions so the service can be reconfigured and upgraded without re-running the full install.

| Field | Type | Description |
|-------|------|-------------|
| `version` | string | Installed binary version (e.g. `v1.2.3`) |
| `port` | int | Listening port (default: 9753) |
| `data_dir` | string | Absolute path to data directory |
| `keychain_service` | string | Keychain service name for encryption key lookup (stable: `"mcp-proxy"`) |
| `keychain_account` | string | Keychain account name (stable: `"encryption-key"`) |

Validation: `port` must be 1024–65535. `data_dir` must be an absolute path. `version` must be a valid semver string.

---

### SchemaVersion (SQLite `PRAGMA user_version`)

Tracks the applied migration level of the SQLite database. No separate table — SQLite provides this as a built-in integer pragma.

| State | Value |
|-------|-------|
| Fresh database | 0 |
| After `001_initial.sql` | 1 |
| After each future migration | N (incremented per file) |

The migration runner reads `PRAGMA user_version` on startup and applies all numbered `.sql` files with a higher sequence number, each in its own transaction.

---

### LaunchAgent descriptor (file: `~/Library/LaunchAgents/io.mcp-proxy.local.plist`)

The macOS launchd configuration for the background service. Written by the install script; not a runtime entity managed by the proxy itself.

| Key | Value |
|-----|-------|
| `Label` | `io.mcp-proxy.local` |
| `ProgramArguments` | `[<binary-path>, --port, <port>, --data-dir, <data-dir>]` |
| `EnvironmentVariables` | `HOME`, `PATH` (explicitly set — launchd does not inherit shell env) |
| `KeepAlive` | `true` (restart on crash) |
| `RunAtLoad` | `true` (start on login) |
| `StandardOutPath` | `~/Library/Logs/mcp-proxy/stdout.log` |
| `StandardErrorPath` | `~/Library/Logs/mcp-proxy/stderr.log` |

---

## Modified Entities

### Binary Version (embedded at build time)

A `version` variable in `main` package is injected at build time via `ldflags`:

```
-X main.version=v1.2.3
```

The proxy exposes this via `--version` flag. The install script reads it with `mcp-proxy --version` to compare against the latest GitHub Release tag before downloading.

---

## Migration File Layout

```
internal/store/sqlite/
├── db.go                  # applyMigrations() — replaces applySchema()
└── migrations/
    ├── 001_initial.sql    # Current schema extracted from db.go schema const
    └── 002_*.sql          # Future: added per release as schema changes
```

Each migration file is named `NNN_description.sql` where `NNN` is a zero-padded integer matching the target `PRAGMA user_version`. Files are sorted lexicographically, which gives correct application order.
