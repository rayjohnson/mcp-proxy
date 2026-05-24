# Data Model: macOS Status Bar Menu

No database schema changes. The menu app is a read-only client of the running proxy — it reads `config.json` on disk and communicates with the proxy via its existing HTTP interface.

## Local Configuration

### InstallConfig (read from `~/Library/Application Support/mcp-proxy/config.json`)

| Field | Type | Notes |
|-------|------|-------|
| port | string | Port the proxy listens on (default `"9753"`) |
| data_dir | string | Path to the data directory |
| version | string | Installed binary version |
| keychain.service | string | Keychain service name (not used by menu app) |
| keychain.account | string | Keychain account name (not used by menu app) |

The menu app only reads `port` from this file.

---

## In-Process State (menu app only)

### ServiceState

Represents the current known state of the proxy service. Managed by the polling goroutine; read by the UI goroutine.

| Field | Type | Values |
|-------|------|--------|
| Status | enum | `Running`, `Stopped`, `Unknown` |
| LastChecked | time.Time | When the last poll completed |

`Unknown` is the initial state before the first poll completes and the state used if polling itself errors unexpectedly.

---

## LaunchAgent Entities

### `io.mcp-proxy.local` (existing)
The main proxy service LaunchAgent. The menu app sends `kickstart` / `kill TERM` to this label.

### `io.mcp-proxy.menu` (new)
LaunchAgent for the `mcp-proxy-menu` binary itself. Added by `install.sh`.

| Plist Key | Value |
|-----------|-------|
| Label | `io.mcp-proxy.menu` |
| ProgramArguments | `["/path/to/mcp-proxy-menu"]` |
| RunAtLoad | `true` |
| KeepAlive | `false` (user controls via Quit in the menu) |

---

## No New API Surface

The Preferences window loads the existing `/dashboard` page from the running proxy. No new API endpoints are introduced by this feature. All upstream management operations use the routes already implemented (list upstreams, connect, disconnect, etc.).
