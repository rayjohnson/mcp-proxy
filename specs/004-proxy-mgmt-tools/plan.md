# Implementation Plan: Proxy Management MCP Tools

**Branch**: `007-proxy-mgmt-tools` | **Date**: 2026-05-24 | **Spec**: [spec.md](spec.md)

## Summary

Add five MCP management tools (`proxy_list_catalog`, `proxy_list_upstreams`, `proxy_connect_upstream`, `proxy_disconnect_upstream`, `proxy_update_credentials`) to the proxy's own MCP server endpoint. Tools are scoped to the authenticated user's session and operate entirely through existing store interfaces — no schema changes are required.

## Technical Context

**Language/Version**: Go 1.26 (existing)

**Primary Dependencies**:
- `github.com/modelcontextprotocol/go-sdk/mcp` — existing MCP SDK; `server.AddTool` is the integration point
- `github.com/rayjohnson/mcp-proxy/internal/store` — existing store interfaces; no new methods needed
- `github.com/rayjohnson/mcp-proxy/internal/kms` — existing KMS client; expose `Encrypt` alongside existing `Decrypt`

**Storage**: No schema changes. Uses existing `upstream_configs` and `default_catalog` tables.

**Testing**: `go test ./...` (existing); new unit tests in `internal/mcp/`; existing integration tests in `tests/integration/`

**Target Platform**: Same as existing binary (Linux/macOS, cloud and local mode)

**Project Type**: Addition to existing background service

**Performance Goals**: Tool responses complete in under 500ms; catalog/upstream list calls are O(10) rows

**Constraints**: Credential material (`encrypted_creds`) must never appear in tool responses; tools must pass ownership checks before any mutation

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Simplicity First | ✅ PASS | Registers tools in the existing `buildMCPServer` call. New file `mgmt.go` holds all handler logic. No new abstractions, no new dependencies. |
| II. Dual-Deployment | ✅ PASS | Management tools work in both local and cloud mode — they use the same store interface, not mode-specific code paths. |
| III. MCP Protocol Fidelity | ✅ PASS | Tools are registered with `server.AddTool` using standard MCP SDK types. No protocol deviations. |
| IV. Security by Design | ✅ PASS | `proxy_connect_upstream` encrypts the API key before storage. `proxy_list_upstreams` strips `encrypted_creds`. Ownership verified before all mutations. |

## Project Structure

### Documentation (this feature)

```text
specs/004-proxy-mgmt-tools/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── contracts/
│   └── mcp-tools.md     # Tool schemas and error contracts
└── tasks.md             # Phase 2 output (/speckit-tasks)
```

### Source Code Changes

```text
# New files
internal/mcp/mgmt.go           # Five management tool handler functions + ManagementDeps wiring

# Modified files
internal/mcp/session.go        # Add UserID string field to ProxySession; populate in OpenSession
internal/mcp/server.go         # Add ManagementDeps to ProxyServerDeps; register 5 tools in buildMCPServer
cmd/mcp-proxy/main.go          # Wire kms.Client.Encrypt into ManagementDeps at startup
```

## Key Design Decisions

### ProxySession carries UserID
`ProxySession` gains a `UserID string` field set in `OpenSession`. Tool handlers receive `*ProxySession` as a closure variable and read `ps.UserID` to scope all store queries. This avoids threading the user ID through the MCP SDK callback signature.

### ManagementDeps nested in ProxyServerDeps
```go
type ManagementDeps struct {
    UpstreamStore store.UpstreamStoreI
    CatalogStore  store.CatalogStoreI
    KMSEncrypt    func(ctx context.Context, plaintext []byte) ([]byte, error)
}
```
These are the same store instances already wired into `SessionDeps`; they're re-exposed here so `buildMCPServer` can hand them to tool handler closures directly.

### Display name join at list time
`proxy_list_upstreams` calls `CatalogStore.ListActiveCatalogEntries`, builds a `map[serverType]displayName`, and annotates each `UpstreamConfig` in the response. The catalog is O(10) entries; no caching needed.

### OAuth2 guard in `proxy_connect_upstream`
If `entry.AuthType == "oauth2"`, the tool returns a text error directing the user to `/dashboard`. No upstream record is created.

### Tool naming convention
All five tools are prefixed `proxy_` to distinguish them from proxied upstream tools in the combined `list_tools` response. This is the only namespacing; no nested tool groups are introduced.
