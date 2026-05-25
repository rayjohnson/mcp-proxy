# Tasks: Per-User Server Enable/Disable

**Input**: Design documents from `specs/006-server-enable-disable/`

**TDD**: Test tasks appear before their implementation tasks in every phase. Write the test, confirm it fails, then implement.

## Format: `[ID] [P?] [Story] Description`

---

## Phase 1: Setup (Schema & Interfaces)

**Purpose**: DB migration and store interface changes that all user stories depend on.

- [ ] T001 Write SQLite migration `internal/store/sqlite/migrations/002_server_toggles.sql` — add `enabled INTEGER NOT NULL DEFAULT 1` to `upstream_configs`; create `server_toggles (id, user_id, catalog_id, enabled, created_at, updated_at)` table with `UNIQUE(user_id, catalog_id)`; set `PRAGMA user_version = 2`
- [ ] T002 Add `Enabled bool` field to `UpstreamConfig` struct in `internal/store/upstream.go` (Postgres) and `internal/store/sqlite/upstream.go` (SQLite)
- [ ] T003 Extend `UpstreamStoreI` with `ToggleUpstream(ctx context.Context, id string) (bool, error)` and add new `ToggleStoreI` interface with `ToggleCatalogEntry`, `DisabledCatalogIDs` methods in `internal/store/interfaces.go`

**Checkpoint**: Interfaces defined, migration file written — store implementations can now be built against these contracts.

---

## Phase 2: Foundational (Store Implementations)

**Purpose**: Implement the store layer for both SQLite and Postgres. Blocks all user story work.

- [ ] T004 Update all SQLite upstream store queries to include the `enabled` column (SELECT, INSERT, RETURNING) and implement `ToggleUpstream` (flip `enabled`, return new value) in `internal/store/sqlite/upstream.go`
- [ ] T005 Implement SQLite `ToggleStore` in `internal/store/sqlite/toggle.go` — `ToggleCatalogEntry` (upsert into `server_toggles`, flip enabled), `DisabledCatalogIDs` (SELECT catalog_id WHERE enabled=0 for user)
- [ ] T006 Add `Enabled` column to all Postgres upstream queries and implement `ToggleUpstream` in `internal/store/upstream.go`
- [ ] T007 Implement Postgres `ToggleStore` in `internal/store/toggle.go` — same interface as SQLite implementation; document the required Postgres migration SQL in a comment at the top of the file

**Checkpoint**: Full store layer complete for both backends — session and handler work can begin.

---

## Phase 3: User Story 1 — Disable a Server (Priority: P1) 🎯 MVP

**Goal**: User can toggle any connected server off from the dashboard; new MCP sessions exclude disabled servers.

**Independent Test**: Connect two servers. Disable one via the dashboard toggle. Start a new MCP session and confirm the disabled server's tools are absent.

### Tests for User Story 1 ⚠️ Write first — confirm they FAIL before implementing

- [ ] T008 [P] [US1] Write SQLite store tests for `ToggleUpstream` (initial state true, flip to false, flip back to true; verify `Enabled` is returned from `GetUpstreamConfigsByUserID`) in `internal/store/sqlite/toggle_test.go`
- [ ] T009 [P] [US1] Write SQLite store tests for `ToggleCatalogEntry` and `DisabledCatalogIDs` (toggle twice, confirm sparse storage, confirm absent entry = enabled) in `internal/store/sqlite/toggle_test.go`
- [ ] T010 [P] [US1] Write handler tests for `POST /api/upstreams/{id}/toggle` (200 with new enabled state, 403 wrong user, 404 not found, 401 unauthenticated) in `internal/handler/toggle_test.go`
- [ ] T011 [P] [US1] Write handler tests for `POST /api/catalog/{id}/toggle` (200 with new enabled state, 404 inactive entry, 401 unauthenticated) in `internal/handler/toggle_test.go`
- [ ] T012 [US1] Write session tests verifying a disabled HTTP upstream is not connected in `OpenSession` and a disabled catalog entry is skipped in `connectStdioEntries` in `internal/mcp/session_test.go`

### Implementation for User Story 1

- [ ] T013 [P] [US1] Implement toggle HTTP handler (`ToggleUpstreamHandler`) and toggle catalog handler (`ToggleCatalogHandler`) in `internal/handler/toggle.go` — verify user owns the upstream (403 if not), call store toggle, return updated state as JSON
- [ ] T014 [US1] Update `SessionDeps` to include `ToggleStore store.ToggleStoreI` and update `OpenSession` to skip configs where `cfg.Enabled == false` in `internal/mcp/session.go`
- [ ] T015 [US1] Update `connectStdioEntries` to call `deps.ToggleStore.DisabledCatalogIDs` and skip disabled entries in `internal/mcp/session.go`
- [ ] T016 [US1] Add `Enabled bool` to `UpstreamView` struct and pass it through in `internal/handler/pages.go`; pass a `ToggleStore` into `SessionDeps` in `cmd/mcp-proxy/main.go`; register `POST /api/upstreams/{id}/toggle` and `POST /api/catalog/{id}/toggle` routes
- [ ] T017 [US1] Add toggle button (fetches POST to toggle endpoint, updates row in-place) and `.server-disabled` visual state (greyed out name + status) to connected server rows in `web/templates/dashboard.html`
- [ ] T018 [US1] Add `.server-disabled` CSS class (muted text, reduced opacity) in `web/static/style.css`

**Checkpoint**: Disable flow fully functional — toggle button appears, disabling a server removes it from the next MCP session.

---

## Phase 4: User Story 2 — Re-enable a Server (Priority: P2)

**Goal**: A previously disabled server can be re-enabled; subsequent sessions include it without re-entering credentials.

**Independent Test**: Disable a server, start a session (tools absent), re-enable, start another session (tools present).

### Tests for User Story 2 ⚠️ Write first — confirm they FAIL before implementing

- [ ] T019 [P] [US2] Write store test for round-trip toggle (disable → re-enable → confirm `Enabled=true` returned from list query) in `internal/store/sqlite/toggle_test.go`
- [ ] T020 [P] [US2] Write session test verifying a re-enabled upstream IS connected in a fresh `OpenSession` call in `internal/mcp/session_test.go`

### Implementation for User Story 2

- [ ] T021 [US2] Update dashboard toggle button label: show "Disable" when `Enabled=true`, "Enable" when `Enabled=false`; apply `.server-disabled` class only when disabled in `web/templates/dashboard.html`

**Checkpoint**: Full enable/disable round-trip works from the dashboard with correct button labels and visual states.

---

## Phase 5: User Story 3 — Persistence Across Sessions (Priority: P3)

**Goal**: Disabled state survives user logout, session expiry, and proxy restarts.

**Independent Test**: Disable a server, restart the proxy process, log in again, start a new MCP session — disabled server's tools are still absent.

### Tests for User Story 3 ⚠️ Write first — confirm they FAIL before implementing

- [ ] T022 [US3] Write store test verifying that `DisabledCatalogIDs` returns the same disabled set after the SQLite DB is closed and reopened (using a file-backed DB in `t.TempDir()`) in `internal/store/sqlite/toggle_test.go`
- [ ] T023 [US3] Write store test verifying that `GetUpstreamConfigsByUserID` returns `Enabled=false` for a toggled-off upstream after a fresh DB connection in `internal/store/sqlite/toggle_test.go`

### Implementation for User Story 3

No additional implementation required — persistence is provided by the DB writes in Phase 3. These tests verify that property holds.

**Checkpoint**: All three user stories fully verified — toggle state is durable across process restarts.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [ ] T024 Run `make lint` and fix any issues across all new files
- [ ] T025 Run `make cover` and verify coverage threshold is met; adjust `COVER_THRESHOLD` in `Makefile` if new tests bring coverage up

---

## Additional: AI Tool Enhancements (spec 007 follow-up)

**Context**: Both `gemini` and `claude` are installed at `/opt/homebrew/bin/` but the launchd service runs with a restricted PATH (`/usr/local/bin:/usr/bin:/bin`), so `exec.LookPath` misses them. Also, Claude Code CLI should be a configurable tool alongside Claude Desktop and Gemini CLI.

- [ ] T026 Fix binary lookup in `GeminiCLITool.lookupGemini()` to probe common install locations when `exec.LookPath` fails: `/opt/homebrew/bin/gemini`, `/usr/local/bin/gemini`, `~/.local/bin/gemini` in `internal/aitools/gemini_cli.go`; add test covering the fallback path in `internal/aitools/aitools_test.go`
- [ ] T027 Add `ClaudeCodeTool` implementing `Configurer` in `internal/aitools/claude_code.go` — detect via probing `/opt/homebrew/bin/claude`, `/usr/local/bin/claude`, `~/.local/bin/claude` (same pattern as T026); detect configured state by running `claude mcp list` and checking for the proxy URL; configure via `claude mcp add --transport http mcp-proxy <url>`; add to registry in `cmd/mcp-proxy/main.go`; write tests in `internal/aitools/aitools_test.go`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 (interfaces must be defined first)
- **Phase 3 (US1)**: Depends on Phase 2 completion — BLOCKS on store layer
- **Phase 4 (US2)**: Depends on Phase 3 (toggle mechanism must exist to re-enable)
- **Phase 5 (US3)**: Depends on Phase 3 (persistence is a property of the Phase 3 DB writes)
- **Phase 6 (Polish)**: Depends on all story phases

### Within Each Story: TDD Order

1. Write tests → confirm they **FAIL** (no implementation yet)
2. Implement → tests pass
3. Checkpoint before next story

### Parallel Opportunities

- T008, T009, T010, T011 — all test files, different paths, can be written in parallel
- T013, T014, T015 — can overlap once interfaces are defined
- T004, T005 (SQLite store) can run in parallel with T006, T007 (Postgres store)

---

## Implementation Strategy

### MVP (User Story 1 only)

1. Complete Phase 1 + Phase 2 (schema + store)
2. Complete Phase 3 tests → implementation
3. **Validate**: Toggle a server off, start a new session, confirm tools absent
4. **Ship** — US2 and US3 are incremental improvements on a working foundation

### Incremental Delivery

1. Phase 1 + 2 → store layer done
2. Phase 3 → disable works end-to-end
3. Phase 4 → re-enable UX polished
4. Phase 5 → persistence verified
