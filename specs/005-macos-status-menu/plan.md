# Implementation Plan: macOS Status Bar Menu

**Branch**: `008-macos-status-menu` | **Date**: 2026-05-24 | **Spec**: [spec.md](spec.md)

## Summary

A separate `mcp-proxy-menu` binary provides a macOS status bar icon showing whether the proxy is running or stopped, start/stop controls via `launchctl`, a Preferences window (WebKit WebView rendering the existing `/dashboard`), and a direct "Open Dashboard" shortcut. The menu binary is CGO-enabled and macOS-only; it does not affect the main `mcp-proxy` binary's CGO-free build.

## Technical Context

**Language/Version**: Go 1.26 (existing), CGO enabled for this binary only

**Primary Dependencies**:
- `github.com/getlantern/systray` — macOS system tray; wraps Cocoa via CGO on macOS
- `github.com/webview/webview_go` — WebKit WebView wrapper; renders the existing web dashboard as the Preferences window; uses macOS-native WebKit (no new system dependencies)

**Storage**: Read-only access to `~/Library/Application Support/mcp-proxy/config.json` for port discovery

**Testing**: Manual smoke tests on macOS; no automated test suite for the UI layer. Health-poller and config-reader can be unit tested without CGO.

**Target Platform**: macOS 13+ (Ventura), arm64 + amd64

**Project Type**: macOS status bar app (separate binary, LaunchAgent)

**Performance Goals**: Icon reflects service reality within 5 seconds; app launch-to-first-poll under 3 seconds

**Constraints**: CGO required; macOS-only binary; no database access; communicates with proxy only via HTTP

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Simplicity First | ✅ PASS | Two new dependencies solve two concrete problems (systray, webview). Preferences window renders the existing dashboard — zero duplicate UI. Start/stop is two `os/exec` calls. |
| II. Dual-Deployment | ✅ PASS | Feature is local-mode only, clearly scoped. Main binary's CGO-free build and cloud deployment are unaffected. |
| III. MCP Protocol Fidelity | ✅ PASS | Does not touch the MCP proxy layer. |
| IV. Security by Design | ✅ PASS | App reads no credentials; opens the existing authenticated dashboard. No new credential surfaces introduced. |

## Project Structure

### Documentation (this feature)

```text
specs/005-macos-status-menu/
├── plan.md                        # This file
├── research.md                    # Phase 0 output
├── data-model.md                  # Phase 1 output
├── contracts/
│   └── menu-behavior.md           # Menu structure, icon states, polling, window contracts
└── tasks.md                       # Phase 2 output (/speckit-tasks)
```

### Source Code Changes

```text
# New files
cmd/mcp-proxy-menu/main.go              # Binary entry point; starts systray run loop
internal/menuapp/app.go                 # systray.Run callback; builds menu items, wires up actions
internal/menuapp/status.go              # Status poller: polls /health every 3s, updates icon/label
internal/menuapp/prefs.go               # Preferences window: webview.New + LoadURL
internal/menuapp/launchd.go             # Start: launchctl kickstart; Stop: launchctl kill TERM
internal/menuapp/config.go              # Reads ~/Library/Application Support/mcp-proxy/config.json

# Modified files
install.sh                              # Download + install mcp-proxy-menu; write io.mcp-proxy.menu plist
.github/workflows/release.yml           # Add macOS CGO build job for mcp-proxy-menu
```

## Key Design Decisions

### CGO separation
`cmd/mcp-proxy-menu` is built only on `macos-latest` GitHub Actions runners with `CGO_ENABLED=1`. The main `mcp-proxy` binary CI job is unchanged. GoReleaser is not used for the menu binary; a plain `go build` step in a dedicated workflow job produces the two macOS arch artifacts.

### Preferences = existing dashboard in WebView
`prefs.go` opens a single `webview.Window` at `http://localhost:{port}/dashboard`. No upstream management logic is duplicated in the menu app. Future dashboard features appear in Preferences automatically.

### Single-instance Preferences window
`prefs.go` maintains a package-level pointer to the open window. If non-nil when the user clicks Preferences, the existing window is brought to the foreground instead of opening a new one.

### Status poller lifecycle
`status.go` starts a goroutine at app launch that sends `ServiceState` updates on a channel. `app.go` reads the channel and updates the systray icon, tooltip, and start/stop menu item label accordingly.

### Start/stop with 10-second failure notification
After issuing `kickstart`, `launchd.go` monitors the status channel for a `Running` state. If 10 seconds elapse without `Running`, a macOS notification is posted via `osascript` (no additional dependency needed).
