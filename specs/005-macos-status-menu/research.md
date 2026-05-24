# Research: macOS Status Bar Menu

## Decision 1: Separate binary vs subcommand

**Decision**: Separate binary `mcp-proxy-menu` at `cmd/mcp-proxy-menu/main.go`.

**Rationale**: The status menu requires CGO on macOS (for the systray and WebView libraries). The main `mcp-proxy` binary is intentionally CGO-free for cross-compilation on Linux CI. Separating the menu app into its own binary preserves the main binary's build constraints and keeps the deployment footprint distinct.

**Alternatives considered**: `mcp-proxy menu` subcommand — rejected because enabling CGO on the main binary would require a macOS CI runner for all releases, not just the menu artifact.

---

## Decision 2: System tray library

**Decision**: `github.com/getlantern/systray` v1.

**Rationale**: The most widely adopted pure-API Go systray library (11k+ stars). Wraps Cocoa/AppKit via CGO on macOS. Supports custom icons, menu items, separators, and dynamic label updates. Well-maintained as of 2026.

**Alternatives considered**:
- `fyne-io/systray` (a fork of getlantern's) — functionally equivalent; getlantern is the upstream
- `progrium/macdriver` — lower-level Obj-C bridge; too complex for this use case

---

## Decision 3: Preferences window

**Decision**: `github.com/webview/webview_go` rendering the existing web dashboard at `http://localhost:{port}/dashboard`. The window is opened in a call to `webview.New(false)` with a fixed 900×650 initial size.

**Rationale**: The systray binary already requires CGO on macOS; WebView (which wraps WebKit, built into macOS) adds no new system dependencies. Rendering the existing dashboard means zero duplicate UI code — all upstream management features (including future ones) are available in the Preferences window automatically.

**Alternatives considered**:
- Open browser tab instead — rejected because the user explicitly asked for a Preferences *window*, not a browser tab; a separate window has a cleaner UX (no browser chrome, no tab management)
- Native Go UI toolkit (Fyne) — rejected per Simplicity First; would require rebuilding the full upstream management UI in Go

---

## Decision 4: Service status polling

**Decision**: Poll `GET http://localhost:{port}/health` every 3 seconds with a 1-second HTTP timeout. Running = HTTP 200 response. Stopped = connection refused or timeout.

**Rationale**: The existing `HealthHandler` already returns 200 OK with no auth required. Polling is the simplest reliable approach. 3-second polling meets the 5-second update SLA from the spec with margin.

**Alternatives considered**:
- `launchctl list io.mcp-proxy.local` — parses JSON/text output; more fragile than an HTTP probe
- FSEvents on the PID file — requires PID file infrastructure that doesn't currently exist

---

## Decision 5: Start and stop mechanism

**Decision**:
- Start: `launchctl kickstart -k gui/{uid}/io.mcp-proxy.local`
- Stop: `launchctl kill TERM gui/{uid}/io.mcp-proxy.local`

**Rationale**: The service is registered as a LaunchAgent during install with label `io.mcp-proxy.local` (from `install.sh`). `kickstart -k` starts it (killing any stuck instance first). `kill TERM` sends a clean shutdown. These are the macOS 13+ approved commands, consistent with the existing install script.

**Alternatives considered**: `launchctl start/stop` — deprecated on macOS 13+; rejected to stay consistent with the install script's `bootstrap/bootout` approach.

---

## Decision 6: Port and config discovery

**Decision**: Read `~/Library/Application Support/mcp-proxy/config.json` at startup. Fall back to port `9753` if the file is absent or malformed.

**Rationale**: The `install.sh` writes this file with `"port"` and `"data_dir"` fields. Reading it gives the menu app the same port as the running service without hardcoding.

---

## Decision 7: Menu app startup (LaunchAgent)

**Decision**: `install.sh` is updated to write a second LaunchAgent plist for `mcp-proxy-menu` with label `io.mcp-proxy.menu`, referencing the installed `mcp-proxy-menu` binary. It is bootstrapped alongside the main service.

**Rationale**: Consistent with the existing install approach. Adding a second LaunchAgent is a two-line addition to `install.sh` (write plist + bootstrap). No new install mechanism needed.

---

## Decision 8: CI build for the menu binary

**Decision**: The menu binary is built by a separate job in `.github/workflows/release.yml` that runs on a `macos-latest` runner with CGO enabled. It produces `mcp-proxy-menu_darwin_arm64` and `mcp-proxy-menu_darwin_amd64` artifacts uploaded to the same GitHub Release.

**Rationale**: CGO binaries cannot be cross-compiled on Linux. macOS-hosted GitHub Actions runners support both arm64 (Apple Silicon) and amd64 (Rosetta). GoReleaser is not used for the menu binary — a simple `go build -ldflags=... -o mcp-proxy-menu` step suffices.

**Alternatives considered**: macOS Docker cross-compilation with osxcross — impractical; WebKit headers are not available in that environment.
