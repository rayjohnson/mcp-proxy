# Research: Local Deployment Mode with stdio MCP Server Support

## MCP stdio Transport in go-sdk

**Decision**: Use `mcp.CommandTransport` from `github.com/modelcontextprotocol/go-sdk/mcp`

**Rationale**: The go-sdk v1.6.1 provides `CommandTransport` specifically for connecting as a client to a subprocess-based MCP server. It wraps `os/exec.Cmd`, manages the subprocess lifecycle (start, stdin/stdout piping, graceful shutdown via SIGTERM then SIGKILL), and handles JSON-RPC 2.0 framing over stdio automatically.

```go
transport := &mcp.CommandTransport{Command: exec.Command("npx", "@modelcontextprotocol/server-filesystem", "/path")}
client := mcp.NewClient(&mcp.Implementation{Name: "mcp-proxy", Version: "1.0"}, nil)
session, err := client.Connect(ctx, transport, nil)
```

**Alternatives considered**:
- `mcp.IOTransport{Reader, Writer}` — lower-level; requires manual subprocess management and pipe wiring. Rejected: more code, same result.
- Custom stdin/stdout JSON-RPC implementation — Rejected: unnecessary given SDK support.

---

## Embedded Database for Local Mode

**Decision**: `modernc.org/sqlite` (pure-Go SQLite driver, CGO-free)

**Rationale**: Local mode requires a zero-dependency embedded database. `modernc.org/sqlite` is a pure-Go port of SQLite — no CGO, no system library installation, single binary distribution. Sufficient for single-user local use. The `database/sql` interface it implements is simple and works alongside the existing pgx-based Postgres store.

**Alternatives considered**:
- `mattn/go-sqlite3` — requires CGO and a C compiler at build time. Rejected: complicates single-binary distribution and cross-compilation.
- BoltDB / BadgerDB — key-value stores, not relational. Rejected: would require rewriting all SQL queries; more migration risk.
- Keep Postgres for local mode (via Docker) — rejected: contradicts the spec goal of zero-infrastructure local mode.

---

## Storage Abstraction Strategy

**Decision**: Define Go interfaces in `internal/store/` that the existing PostgreSQL stores implement; add parallel SQLite implementations.

**Rationale**: The existing store types (`UserStore`, `CatalogStore`, `UpstreamStore`, `OAuth2StateStore`) all use `pgxpool.Pool` directly. To support SQLite without duplicating handler logic, we extract the method signatures into interfaces. The existing concrete types implement those interfaces unchanged. New SQLite types in `internal/store/sqlite/` implement the same interfaces using `database/sql`.

**Query compatibility note**: SQLite uses `?` placeholders and `CURRENT_TIMESTAMP` instead of Postgres's `$N` and `now()`. Queries must be written separately per backend. The SQL is otherwise identical (standard DDL, no Postgres-specific extensions used).

**Alternatives considered**:
- `pgx/stdlib` adapter to use `database/sql` for Postgres too — would unify the driver interface but requires migrating all Postgres queries from pgx-style to database/sql style. Rejected: large, risky refactor of working code.
- Single interface + query builder (sqlc, squirrel) — adds new tooling dependency. Rejected: YAGNI.

---

## Local Mode Detection

**Decision**: `LOCAL_MODE=true` environment variable; also detectable via `--local` CLI flag

**Rationale**: Matches the existing pattern (`KMS_KEY_NAME=local` activates the local KMS). A single env var makes Docker/systemd/launchd deployment straightforward with no config file format to define.

**Side effects of LOCAL_MODE=true**:
- `DB_DSN` becomes optional; defaults to `file:./mcp-proxy.db` (SQLite, relative to CWD)
- `auth_type=oauth2` is rejected in catalog write operations
- `transport=stdio` is permitted in catalog entries
- The dashboard UI shows a "local mode" badge

---

## Process Lifecycle for stdio Servers

**Decision**: Reconnect-on-demand with per-server mutex; no persistent subprocess pool

**Rationale**: `mcp.CommandTransport` starts the subprocess during `client.Connect()`. Rather than maintaining a long-running pool of child processes, we connect on demand per incoming MCP session and let the SDK's context cancellation handle teardown. A per-catalog-entry mutex prevents concurrent spawning of the same stdio server.

**Restart strategy**: If `Connect()` fails (process died), retry up to 3 times with 1-second backoff. After 3 failures, mark the catalog entry `status=unavailable` and return an error to the client.

**Alternatives considered**:
- Persistent subprocess pool with health checking — Rejected: more complex, and stdio servers have no standard health-check mechanism.
- One subprocess per user session — Rejected: too many processes for popular servers; stdio servers are typically single-instance.

---

## Constitution Alignment

**Finding**: Principle II ("Cloud-Native by Default") conflicts with this feature. The principle states "Local execution... [is] not valid architectural choices." This feature is explicitly local.

**Resolution**: Principle II must be amended in the same PR as this feature. The amendment changes the scope from "cloud-only" to "dual-deployment: cloud-hosted and local." The security and simplicity principles (I, III, IV) remain unchanged. See `plan.md` Complexity Tracking for the gate justification.
