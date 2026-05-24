# Research: Local Mode Packaging & Distribution

## Decision 1: Default Port

**Decision**: Port **9753**

**Rationale**: Unassigned by IANA, not used by any common developer tool (confirmed against: Node/Rails 3000, Phoenix 4000, Flask 5000, Django 8000, general dev 8080, Jupyter 8888, PHP-FPM/SonarQube 9000, Prometheus 9090, Elasticsearch 9200). The descending digit pattern (9-7-5-3) makes it memorable and easy to type.

**Alternatives considered**: 9876 (also clean, slightly more common in hobby projects), 9119 (palindrome-adjacent, also available). 9753 chosen as the strongest combination of memorability and availability.

---

## Decision 2: macOS Keychain Library

**Decision**: `github.com/zalando/go-keyring`

**Rationale**: Actively maintained (v0.2.8, March 2026), no CGO required (shells out to the macOS `security` CLI internally), and the API is trivially simple for a single get/set use case:
```go
keyring.Set("mcp-proxy", "encryption-key", hexKey)
hexKey, _ := keyring.Get("mcp-proxy", "encryption-key")
```
Keeps the binary statically linkable (no CGO), consistent with the existing `modernc.org/sqlite` choice.

**Alternatives considered**:
- `github.com/99designs/keyring`: Last released December 2022, requires CGO on macOS — both disqualifying.
- Raw `security` CLI subprocess: Identical behavior to `go-keyring` under the hood but requires manual subprocess error handling. No benefit over the library.

---

## Decision 3: Release Pipeline

**Decision**: GoReleaser with `goreleaser/goreleaser-action@v6`

**Rationale**: GoReleaser produces multi-arch macOS binaries, `checksums.txt`, and uploads all assets to GitHub Releases in a single declarative config file. The alternative (manual GitHub Actions matrix) would require ~60 lines of bespoke workflow YAML with equivalent responsibility. GoReleaser's `{{.Version}}` template variable also handles `ldflags` version injection cleanly.

Key config (`CGO_ENABLED=0` required — the GitHub Actions runner is Linux; the binary is pure Go so cross-compilation works without a macOS SDK):

```yaml
version: 2
builds:
  - env: [CGO_ENABLED=0]
    goos: [darwin]
    goarch: [arm64, amd64]
    ldflags: [-s -w -X main.version={{.Version}}]
checksum:
  name_template: "checksums.txt"
```

**Alternatives considered**: Manual matrix build — viable but more YAML to maintain forever with no upside.

---

## Decision 4: SQLite Schema Migrations

**Decision**: `PRAGMA user_version` + `go:embed` SQL files, zero new dependencies

**Rationale**: SQLite's built-in `PRAGMA user_version` integer is the SQLite-recommended approach for application-managed schema versioning. Each migration runs in its own transaction with `PRAGMA user_version` updated atomically — no partial migrations possible. `go:embed` bakes `.sql` files directly into the binary so no runtime file I/O is needed.

Migration runner sketch:
```go
//go:embed migrations/*.sql
var migrationsFS embed.FS

func applyMigrations(ctx context.Context, db *sql.DB) error {
    var version int
    db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version)
    // for each NNN_name.sql where N > version: exec in tx, update PRAGMA user_version = N
}
```

`001_initial.sql` contains the current schema verbatim (extracted from the existing `schema` const in `db.go`). Future migrations add numbered files.

**Alternatives considered**: `golang-migrate/migrate` — 15+ transitive dependencies for a single-user app with no concurrency concerns. Violates Principle I. Rejected.

---

## Decision 5: launchd Integration

**Decision**: `~/Library/LaunchAgents/io.mcp-proxy.local.plist` with `bootstrap`/`bootout` commands

**Rationale**: LaunchAgents in `~/Library/LaunchAgents/` run in the user's GUI session (the `gui/$(id -u)` domain), which is required for Keychain access. `KeepAlive: true` + `RunAtLoad: true` provides auto-start and crash recovery.

**macOS 13+ launchctl API** (old `load`/`unload` is deprecated):
```bash
# Install
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/io.mcp-proxy.local.plist

# Remove
launchctl bootout gui/$(id -u) ~/Library/LaunchAgents/io.mcp-proxy.local.plist
```

**Critical gotchas**:
- LaunchAgents do not inherit the shell environment — `HOME` and `PATH` must be set explicitly in `EnvironmentVariables`.
- The log directory (`~/Library/Logs/mcp-proxy/`) must exist before the agent starts; install script must create it.
- Port and data-dir passed as `ProgramArguments` flags, not environment variables, so they are visible in the plist and easy to inspect/change.
- After editing the plist, `bootout` + `bootstrap` is required — `kickstart` only restarts the process, not the configuration.

**Alternatives considered**: Shell wrapper script that sources a config file — adds complexity for no gain given config is embedded in the plist.
