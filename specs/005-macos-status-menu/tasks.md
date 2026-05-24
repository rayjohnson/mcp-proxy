# Tasks: macOS Status Bar Menu

**Input**: Design documents from `specs/005-macos-status-menu/`

**Platform note**: All `go build` and `go get` commands for this feature require `CGO_ENABLED=1` on a macOS machine.

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1–US4)

---

## Phase 1: Setup

**Purpose**: Add dependencies and scaffold all new files.

- [x] T001 Run `CGO_ENABLED=1 go get github.com/getlantern/systray github.com/webview/webview_go` on macOS to add both libraries to `go.mod` and `go.sum`
- [x] T002 Create `cmd/mcp-proxy-menu/main.go` with package main, import of `internal/menuapp`, and a `main()` that calls `menuapp.Run()`
- [x] T003 [P] Create stub files with package declaration only: `internal/menuapp/app.go`, `internal/menuapp/status.go`, `internal/menuapp/prefs.go`, `internal/menuapp/launchd.go`, `internal/menuapp/config.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Config reading, status polling goroutine, and the base menu skeleton must exist before any user story can be tested.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [x] T004 Implement `ReadConfig() (port string)` in `internal/menuapp/config.go`: read `~/Library/Application Support/mcp-proxy/config.json`, return the `"port"` field string, default to `"9753"` if file is absent or malformed
- [x] T005 Implement `ServiceState` type (`Running`/`Stopped`/`Unknown`) and `StartPoller(port string) <-chan ServiceState` in `internal/menuapp/status.go`: poll `GET http://localhost:{port}/health` every 3 seconds with a 1-second timeout; send state on channel when it changes
- [x] T006 Implement `Run()` in `internal/menuapp/app.go`: call `systray.Run(onReady, onExit)`; in `onReady` read config, start poller, create menu items (status label, separator, Start/Stop, separator, Preferences…, Open Dashboard, separator, Quit); wire Quit item to `systray.Quit()`

**Checkpoint**: `CGO_ENABLED=1 go build ./cmd/mcp-proxy-menu/...` succeeds and the app launches with a placeholder icon and menu on macOS.

---

## Phase 3: User Story 1 — Service Status at a Glance (Priority: P1) 🎯 MVP

**Goal**: Menu bar icon and status label update automatically within 5 seconds of any service state change.

**Independent Test**: Launch `mcp-proxy-menu`. Start and stop the proxy service. Confirm the menu bar icon changes between filled-circle and empty-circle states within 5 seconds each time, without clicking anything.

- [x] T007 [US1] Create two monochrome PNG icon files (16×16 and 32×32): `internal/menuapp/icons/running.png` (filled circle) and `internal/menuapp/icons/stopped.png` (empty circle); add `//go:embed icons/*.png` directive in a new `internal/menuapp/icons.go` file
- [x] T008 [US1] Wire the status poller channel in `app.go`: in a goroutine, `select` on the poller channel and call `systray.SetIcon(runningIcon)` or `systray.SetIcon(stoppedIcon)` plus `systray.SetTooltip("mcp-proxy: Running")` or `"mcp-proxy: Stopped"` based on received state
- [x] T009 [US1] Update the non-clickable status menu item title in the same goroutine: `"● Running"`, `"○ Stopped"`, or `"– Checking…"` for `Unknown` state

**Checkpoint**: US1 complete — status visible at a glance, auto-updates without interaction.

---

## Phase 4: User Story 2 — Start and Stop the Service (Priority: P2)

**Goal**: User can start or stop the proxy from the menu bar; the icon reflects the new state within 5 seconds.

**Independent Test**: With the proxy stopped, click "Start" — confirm the proxy starts and the icon changes to running. With it running, click "Stop" — confirm the proxy stops and the icon changes to stopped. Confirm "Stopping…" / "Starting…" labels appear during transitions.

- [x] T010 [P] [US2] Implement `Start(uid int)` and `Stop(uid int)` in `internal/menuapp/launchd.go`: `Start` runs `launchctl kickstart -k gui/{uid}/io.mcp-proxy.local`; `Stop` runs `launchctl kill TERM gui/{uid}/io.mcp-proxy.local`; both use `os/exec`, log stderr on failure
- [x] T011 [US2] Wire Start/Stop menu item in `app.go`: show "Start" when `Stopped`, "Stop" when `Running`, disabled when `Unknown`; on click, relabel to "Starting…" or "Stopping…" and disable; start a 10-second timeout goroutine that posts a macOS notification via `osascript -e 'display notification "mcp-proxy failed to start." with title "mcp-proxy"'` if the expected state is not reached

**Checkpoint**: US2 complete — service lifecycle controllable from the menu bar.

---

## Phase 5: User Story 3 — Preferences Window (Priority: P3)

**Goal**: Clicking Preferences opens a WebView window showing the proxy dashboard for upstream management. Only one window can be open at a time.

**Independent Test**: With the proxy running, click "Preferences…" — confirm a native-looking window opens at `http://localhost:{port}/dashboard`. Click Preferences again — confirm focus moves to the existing window rather than opening a second one. Close the window and click again — confirm a new window opens.

**Dependency**: Requires spec 004 (proxy management tools) to be deployed for full upstream management functionality; the window opens and shows the dashboard regardless.

- [x] T012 [US3] Implement `internal/menuapp/prefs.go`: package-level `var wv webview.WebView`; `Open(port string)` function: if `wv != nil` call `wv.Run()` to bring it front, else create `webview.New(false)` with title "mcp-proxy — Preferences", size 960×700, navigate to `http://localhost:{port}/dashboard`, set `wv = nil` in the destroy callback
- [x] T013 [US3] Wire "Preferences…" menu item click in `app.go` to call `prefs.Open(port)` in a new goroutine; disable the menu item when `ServiceState` is `Stopped` and re-enable when `Running`

**Checkpoint**: US3 complete — users can manage upstreams from a dedicated window without opening a browser.

---

## Phase 6: User Story 4 — Open Dashboard in Browser (Priority: P4)

**Goal**: "Open Dashboard" opens the proxy web UI in the user's default browser.

**Independent Test**: Click "Open Dashboard" — confirm the default browser opens `http://localhost:{port}/dashboard`.

- [x] T014 [US4] Wire "Open Dashboard" menu item click in `app.go` to run `exec.Command("open", "http://localhost:"+port+"/dashboard").Start()`

**Checkpoint**: US4 complete — full browser dashboard accessible from the menu bar.

---

## Phase 7: Install Integration

**Purpose**: The menu app ships and registers as a LaunchAgent via the existing install script.

- [x] T015 Update `install.sh`: add download of `mcp-proxy-menu` binary from the GitHub Release (same archive as `mcp-proxy`); install to same directory; write `~/Library/LaunchAgents/io.mcp-proxy.menu.plist` with `Label=io.mcp-proxy.menu`, `RunAtLoad=true`, `KeepAlive=false`; bootstrap it; on uninstall, bootout and remove plist and binary
- [x] T016 Add macOS CGO build job to `.github/workflows/release.yml`: runs on `macos-latest`, `CGO_ENABLED=1`, builds `cmd/mcp-proxy-menu` for `arm64` and `amd64`, uploads both binaries as Release assets alongside the main binary

---

## Phase 8: Polish & Cross-Cutting Concerns

- [x] T017 Build `CGO_ENABLED=1 GOOS=darwin go build ./cmd/mcp-proxy-menu/...` and fix all compile errors
- [x] T018 [P] Run `shellcheck install.sh` and fix all warnings per the project's bash script standard
- [x] T019 [P] Run `golangci-lint run --fix ./internal/menuapp/... ./cmd/mcp-proxy-menu/...` and fix all remaining warnings

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on T001–T003; blocks all user stories
- **US1 (Phase 3)**: Depends on T001–T006
- **US2 (Phase 4)**: Depends on T001–T006; can run in parallel with US1 once Foundational is done
- **US3 (Phase 5)**: Depends on T001–T006; T012 can run in parallel with US1/US2
- **US4 (Phase 6)**: Depends on T006 only (menu item already exists); T014 is one line
- **Install (Phase 7)**: Independent of US1–US4; can run in parallel with any story phase
- **Polish (Phase 8)**: Depends on all phases complete

### Parallel Opportunities

- T003 (stub files) can run in parallel with T001/T002
- T010 (`launchd.go`) can run in parallel with T007–T009 (US1)
- T012 (`prefs.go`) can run in parallel with T007–T011 (US1/US2)
- T015 (install.sh) and T016 (CI) can run at any time independently

---

## Implementation Strategy

### MVP (User Story 1 + 2 — Status and Control)

1. Complete Phase 1 (T001–T003)
2. Complete Phase 2 (T004–T006)
3. Complete Phase 3 (T007–T009) — status icon
4. Complete Phase 4 (T010–T011) — start/stop
5. **Validate**: App shows correct icon, start/stop works from the menu

### Full Delivery

Add US3 (Preferences) → US4 (Open Dashboard) → Install integration → Polish.

---

## Notes

- The `getlantern/systray` run loop must stay on the main thread — all goroutines spawned from `onReady`
- `webview.WebView` is not thread-safe; `prefs.Open` must be called from the goroutine that created it, or use `webview.Dispatch`
- The menu app must never call `os.Exit` or signal the proxy process — Quit only exits the menu app itself
- Icon files are small (< 1KB each); `//go:embed` is the correct mechanism to bundle them in the binary
