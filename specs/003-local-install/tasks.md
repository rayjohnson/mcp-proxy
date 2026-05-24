# Tasks: Local Mode Packaging & Distribution

**Input**: Design documents from `specs/003-local-install/`

**Prerequisites**: plan.md ✓, spec.md ✓, research.md ✓, data-model.md ✓, contracts/install-script.md ✓, contracts/release-artifacts.md ✓

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on each other)
- **[Story]**: Which user story this task belongs to (US1–US5)

---

## Phase 1: Setup

**Purpose**: New dependencies, release tooling, Go module updates

- [X] T001 Add `github.com/zalando/go-keyring` to `go.mod`/`go.sum` via `go get github.com/zalando/go-keyring`
- [X] T002 [P] Create `.goreleaser.yml` at repo root — `version: 2`, `goos: [darwin]`, `goarch: [arm64, amd64]`, `CGO_ENABLED=0`, `ldflags: [-s -w -X main.version={{.Version}}]`, `checksum: checksums.txt` per `specs/003-local-install/contracts/release-artifacts.md`
- [X] T003 [P] Create `.github/workflows/release.yml` — triggers on `push: tags: ["v*.*.*"]`, uses `goreleaser/goreleaser-action@v6` with `version: "~> v2"` and `GITHUB_TOKEN` secret per `specs/003-local-install/contracts/release-artifacts.md`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Binary version embedding, SQLite migration runner, and Keychain key loading — required by all user stories.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T004 Add `var version = "dev"` to `cmd/server/main.go`; add `--version` flag that prints `mcp-proxy <version>` and exits; update the `build` target in `Makefile` to inject `-ldflags "-X main.version=$(shell git describe --tags --always --dirty)"` so `make build` produces a versioned binary
- [X] T005 [P] Refactor SQLite schema management in `internal/store/sqlite/db.go`: (1) create `internal/store/sqlite/migrations/001_initial.sql` containing the current `schema` const DDL verbatim (without `IF NOT EXISTS` where unnecessary), (2) replace `applySchema()`/`schema` const with `applyMigrations()` using `//go:embed migrations/*.sql` + `PRAGMA user_version` — each migration file runs in its own transaction and updates `PRAGMA user_version = N` atomically; see `specs/003-local-install/research.md` Decision 4 for implementation sketch
- [X] T006 [P] Update `internal/config/config.go`: when `LOCAL_MODE=true` and `LOCAL_KMS_KEY` env var is empty, read the encryption key from the macOS Keychain using `go-keyring` (`keyring.Get("mcp-proxy", "encryption-key")`); log a clear error if the key is not found in either source; existing `LOCAL_KMS_KEY` env var takes precedence over Keychain (for `make run-local` compatibility)

**Checkpoint**: `go build ./...` succeeds; `./bin/mcp-proxy --version` prints version; SQLite migrations apply on fresh DB; `go test ./...` passes.

---

## Phase 3: User Story 1 — One-Command Install from the Internet (Priority: P1) 🎯 MVP

**Goal**: A developer runs a single curl command and has the proxy installed.

**Independent Test**: On a clean macOS machine, run `curl -fsSL .../install.sh | sh` → binary lands at `/usr/local/bin/mcp-proxy` → `mcp-proxy --version` works → `config.json` written to `~/Library/Application Support/mcp-proxy/`.

- [X] T007 [US1] Create `install.sh` at repo root implementing the core install flow per `specs/003-local-install/contracts/install-script.md`: (1) argument parsing (`--port`, `--data-dir`, `--version`, `--uninstall`, `--purge-data`, `--help`), (2) arch detection via `uname -m` → `arm64` or `amd64`, (3) fetch latest release tag from GitHub API (`https://api.github.com/repos/rayjohnson/mcp-proxy/releases/latest`), (4) compare against `mcp-proxy --version` — exit 0 if already current, (5) download `mcp-proxy_<ver>_darwin_<arch>.tar.gz` + `checksums.txt`, (6) SHA-256 verification (`shasum -a 256 -c`), (7) extract and install binary to `/usr/local/bin/mcp-proxy` (fallback `~/.local/bin/mcp-proxy` if not writable), (8) write `~/Library/Application Support/mcp-proxy/config.json` with port/data-dir/version/keychain fields
- [X] T008 [US1] Run `shellcheck install.sh` and fix all warnings before proceeding (required by global CLAUDE.md)
- [X] T009 [US1] Update `README.md`: add `curl -fsSL https://raw.githubusercontent.com/rayjohnson/mcp-proxy/main/install.sh | sh` as the primary install command in the quickstart section; update any references to port 8080 for local mode to port 9753

**Checkpoint**: `shellcheck install.sh` exits 0; README curl command is accurate; binary installs to correct location.

---

## Phase 4: User Story 2 — Service Runs Automatically on Login (Priority: P2)

**Goal**: After install, the proxy starts on login and restarts on crash with no user action.

**Independent Test**: Run install script → log out and back in → `curl http://localhost:9753/health` returns 200 without launching anything manually.

- [X] T010 [US2] Extend `install.sh` with launchd integration (after the binary install step): (1) create `~/Library/Logs/mcp-proxy/` directory, (2) write `~/Library/LaunchAgents/io.mcp-proxy.local.plist` with `Label: io.mcp-proxy.local`, `ProgramArguments: [<binary>, --port, <port>, --data-dir, <data-dir>]`, explicit `EnvironmentVariables` (`HOME`, `PATH=/usr/local/bin:/usr/bin:/bin`), `KeepAlive: true`, `RunAtLoad: true`, `StandardOutPath`/`StandardErrorPath` to log dir; (3) on upgrade: `launchctl bootout gui/$(id -u) <plist>` before replacing binary, then re-bootstrap; (4) `launchctl bootstrap gui/$(id -u) <plist>`; (5) poll `http://localhost:<port>/health` up to 5 seconds and print success or failure with log path; run `shellcheck install.sh` after changes

**Checkpoint**: `launchctl print gui/$(id -u)/io.mcp-proxy.local` shows the service running; proxy responds at default port.

---

## Phase 5: User Story 3 — Persistent Encryption Key (Priority: P3)

**Goal**: Stored credentials survive service restarts; key is never regenerated on upgrade.

**Independent Test**: Install → add catalog entry with PAT → stop and restart service → PAT is still functional.

- [X] T011 [US3] Extend `install.sh` with Keychain key management (before writing plist): on first install (`security find-generic-password -s mcp-proxy -a encryption-key` returns non-zero), generate key with `openssl rand -hex 32` and store via `security add-generic-password -s mcp-proxy -a encryption-key -w <key>`; on upgrade, skip if key already exists; run `shellcheck install.sh` after changes

**Checkpoint**: After install, `security find-generic-password -s mcp-proxy -a encryption-key` exits 0; after restart, SQLite credentials remain decryptable.

---

## Phase 6: User Story 4 — Non-Conflicting Default Port (Priority: P4)

**Goal**: Default port is 9753, not 8080, so installation doesn't break existing dev workflows.

**Independent Test**: Default install on a machine running something on 8080 → both coexist without error.

- [X] T012 [US4] Change the default LOCAL_MODE port from `8080` to `9753` in `internal/config/config.go` (`PORT` env var default for local mode) and update the `run-local` target in `Makefile` to use `BASE_URL=http://localhost:9753 PORT=9753`
- [X] T013 [US4] Update `specs/002-local-mode-stdio/quickstart.md` to replace all references to port `8080` with `9753` for local mode; update any other docs or comments referencing the old local port

**Checkpoint**: `make run-local` starts on port 9753; `curl http://localhost:9753/health` succeeds.

---

## Phase 7: User Story 5 — Clean Uninstall (Priority: P5)

**Goal**: One command removes the proxy completely; data is preserved by default.

**Independent Test**: Install → run `install.sh --uninstall` → binary gone, service not registered, no login-time start; `install.sh --uninstall --purge-data` also removes data dir and Keychain entry.

- [X] T014 [US5] Extend `install.sh` with `--uninstall` flow: `launchctl bootout gui/$(id -u) <plist>` (ignore error if not loaded), `rm -f <plist>`, `rm -f <binary>`; with `--purge-data`: also `rm -rf "~/Library/Application Support/mcp-proxy"`, `rm -rf ~/Library/Logs/mcp-proxy`, `security delete-generic-password -s mcp-proxy -a encryption-key`; print confirmation of what was removed; run `shellcheck install.sh` after changes

**Checkpoint**: After `--uninstall`, `launchctl print gui/$(id -u)/io.mcp-proxy.local` returns error; no binary at install path.

---

## Phase 8: Polish & Cross-Cutting Concerns

- [X] T015 Run final `shellcheck install.sh` across the complete script and fix any remaining warnings
- [X] T016 [P] Run `make lint` and fix any issues (`make lint-fix` for auto-fixable items); `errcheck` will flag unhandled `keyring.Get` errors — handle them with proper error propagation in `internal/config/config.go`
- [X] T017 [P] Run `go test ./...` and verify all existing tests still pass
- [ ] T018 Manual smoke test: follow `specs/003-local-install/quickstart.md` end-to-end — curl install, browser open, register, add catalog entry, confirm proxy token works, upgrade by running install again, verify data preserved, uninstall cleanly

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately; T002 and T003 are parallel
- **Foundational (Phase 2)**: Depends on T001 (go-keyring in go.mod); T005 and T006 are parallel after T004
- **US1 (Phase 3)**: Depends on T004 (--version flag needed for version comparison in install script)
- **US2 (Phase 4)**: Depends on T007–T009 (extends install.sh)
- **US3 (Phase 5)**: Depends on T010 (extends install.sh); also depends on T006 (Go binary reads key)
- **US4 (Phase 6)**: Depends on Foundational — parallel with US2/US3
- **US5 (Phase 7)**: Depends on T007 (extends install.sh) — parallel with US2/US3/US4
- **Polish (Phase 8)**: Depends on all phases complete

### install.sh Build Order

```
T007 (core: download/verify/install/config.json)
  → T010 (add: plist + launchctl)
    → T011 (add: Keychain key generation)
      → T014 (add: --uninstall/--purge-data)
        → T015 (final shellcheck pass)
```

### Parallel Opportunities

- T002 and T003 (release tooling files — different files)
- T005 and T006 (different packages — sqlite/db.go vs config.go)
- T012 and T013 (config.go port change vs docs update)
- T016 and T017 (lint vs test — independent)

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001–T003)
2. Complete Phase 2: Foundational (T004–T006) — binary version + migrations + Keychain config
3. Complete Phase 3: US1 (T007–T009) — working install script, binary download + install
4. **STOP and VALIDATE**: Binary installs on macOS; `mcp-proxy --version` works
5. Add US2 (T010): launchd registration
6. Add US3 (T011): Keychain key persistence
7. Add US4 (T012–T013): port change
8. Add US5 (T014): uninstall
9. Polish (T015–T018)

### Notes

- `install.sh` must pass `shellcheck` at every stage (required by global CLAUDE.md — run after each task that modifies it)
- The `go-keyring` library shells out to the macOS `security` CLI; it will not work on Linux CI runners — guard with build tags or runtime `LOCAL_MODE` check
- GoReleaser cross-compiles on Linux runners with `CGO_ENABLED=0`; the pure-Go binary (no CGO in modernc.org/sqlite) supports this
- The `001_initial.sql` migration must be idempotent-safe: on a brand-new DB, `PRAGMA user_version` is 0, so the migration runs; on an existing DB opened before migration support was added, `user_version` is also 0 — the initial migration DDL must use `CREATE TABLE IF NOT EXISTS` to handle this case safely
