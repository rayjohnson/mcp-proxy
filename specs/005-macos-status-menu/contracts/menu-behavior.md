# Behavior Contract: macOS Status Bar Menu

## Menu Structure

```
[icon]  ← status icon in menu bar
  ─────────────────────
  ● Running             ← status label (non-clickable)
  ─────────────────────
  Stop                  ← context-sensitive (shows "Start" when stopped)
  ─────────────────────
  Preferences…
  Open Dashboard
  ─────────────────────
  Quit
```

When the service status is `Unknown` (e.g., first poll not yet complete):
- Status label shows "Checking…"
- Start/Stop item is disabled

---

## Icon States

| Service State | Icon | Description |
|---------------|------|-------------|
| Running | Filled circle (●) | Template image, white in dark menu bar |
| Stopped | Empty circle (○) | Template image, dimmed |
| Unknown | Dash (–) | Shown briefly at startup |

Icons are monochrome template images so macOS applies the correct tint automatically.

---

## Status Polling Contract

- Polls `GET http://localhost:{port}/health` every **3 seconds**
- HTTP client timeout: **1 second**
- HTTP 200 response → `Running`
- Connection refused, timeout, or any non-200 → `Stopped`
- First poll completes within 3 seconds of app launch; icon transitions from `Unknown` immediately after

---

## Start Action Contract

Precondition: Service state is `Stopped`.

1. Menu item is relabeled "Starting…" and disabled.
2. Run: `launchctl kickstart -k gui/{uid}/io.mcp-proxy.local`
3. Poll resumes immediately; icon updates when the health check returns 200.
4. If still stopped after 10 seconds, show a macOS notification: "mcp-proxy failed to start."

---

## Stop Action Contract

Precondition: Service state is `Running`.

1. Menu item is relabeled "Stopping…" and disabled.
2. Run: `launchctl kill TERM gui/{uid}/io.mcp-proxy.local`
3. Poll resumes immediately; icon updates when the health check stops returning 200.

---

## Preferences Window Contract

- Opens a WebView window titled "mcp-proxy — Preferences"
- Initial URL: `http://localhost:{port}/dashboard`
- Window size: 960 × 700 (resizable)
- If the service is stopped when Preferences is opened: WebView displays an error page; no special handling needed beyond what the browser/WebKit provides
- Only one Preferences window may be open at a time; subsequent clicks bring the existing window to front

---

## Open Dashboard Contract

- Runs: `open http://localhost:{port}/dashboard`
- Opens in the user's default browser
- If the service is stopped: macOS will show a "connection refused" error in the browser — no special handling by the menu app

---

## Quit Contract

- Exits the `mcp-proxy-menu` process
- Does **not** send any signal to the `mcp-proxy` service
- The proxy continues running after the menu app exits

---

## Install Integration Contract

`install.sh` changes:
1. Downloads `mcp-proxy-menu` binary from the same GitHub Release as the main binary
2. Installs to the same directory as `mcp-proxy`
3. Writes `~/Library/LaunchAgents/io.mcp-proxy.menu.plist`
4. Bootstraps `io.mcp-proxy.menu` into `gui/{uid}` domain
5. On uninstall: boots out and removes both plists and both binaries
