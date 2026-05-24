# Tasks: Local Deployment Mode with stdio MCP Server Support

**Input**: Design documents from `specs/002-local-mode-stdio/`

**Prerequisites**: plan.md ✓, spec.md ✓, research.md ✓, data-model.md ✓, contracts/catalog-api.md ✓

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on each other)
- **[Story]**: Which user story this task belongs to (US1–US4)

---

## Phase 1: Setup

**Purpose**: Constitution amendment and new dependency

- [X] T001 Amend constitution Principle II in `.specify/memory/constitution.md` to recognize local deployment as a supported target alongside cloud-hosted (removes "cloud-only" constraint; version bump 1.0.0 → 1.1.0)
- [X] T002 Add `modernc.org/sqlite` to `go.mod` / `go.sum` via `go get modernc.org/sqlite`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Store interface layer, SQLite backend, config extensions, and catalog schema changes that every user story depends on.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T003 Create `internal/store/interfaces.go` — define `UserStore`, `CatalogStore`, `UpstreamStore`, `OAuth2StateStore`, `SuggestionStore` interfaces matching all existing method signatures (see data-model.md)
- [X] T004 Extend `internal/store/catalog.go` — add `Transport`, `Command`, `Args`, `Env` fields to `CatalogEntry` struct; update `AddCatalogEntry` signature and all scan helpers; update `catalogCols` constant
- [X] T005 Verify existing Postgres store types satisfy the new interfaces (compile check; no logic changes expected for `users.go`, `upstream.go`, `oauth2state.go`, `suggestions.go`)
- [X] T006 Create `internal/store/sqlite/db.go` — open/create SQLite DB at configured path using `modernc.org/sqlite` + `database/sql`; run full DDL schema on first connection (all tables: `users`, `default_catalog`, `upstream_configs`, `oauth2_states`, `suggestion_searches`)
- [X] T007 [P] Create `internal/store/sqlite/users.go` — SQLite `UserStore` implementation (all methods from interface; use `?` placeholders, `CURRENT_TIMESTAMP`, `database/sql`)
- [X] T008 [P] Create `internal/store/sqlite/catalog.go` — SQLite `CatalogStore` implementation (includes `transport`, `command`, `args`, `env` columns; JSON encode/decode `args` and `env` fields as TEXT)
- [X] T009 [P] Create `internal/store/sqlite/upstream.go` — SQLite `UpstreamStore` implementation
- [X] T010 [P] Create `internal/store/sqlite/oauth2state.go` — SQLite `OAuth2StateStore` implementation
- [X] T011 [P] Create `internal/store/sqlite/suggestions.go` — SQLite `SuggestionStore` implementation
- [X] T012 Update `internal/config/config.go` — add `LocalMode bool` (from `LOCAL_MODE` env), `DataDir string` (from `DATA_DIR` env, default `.`); make `DB_DSN` optional when `LocalMode=true` (auto-default to `file:{DataDir}/mcp-proxy.db`)
- [X] T013 Create `db/migrations/0004_add_stdio_columns.sql` — Postgres migration adding `transport TEXT NOT NULL DEFAULT 'http'`, `command TEXT`, `args JSONB NOT NULL DEFAULT '[]'`, `env JSONB NOT NULL DEFAULT '{}'` to `default_catalog`
- [X] T014 Update `cmd/server/main.go` — store factory: if `cfg.LocalMode`, open SQLite and construct sqlite store types; otherwise construct existing Postgres store types; wire both paths into the existing handler constructors using the new interfaces

**Checkpoint**: `go build ./...` succeeds with both Postgres and SQLite paths wired; `LOCAL_MODE=true go run ./cmd/server` starts without error.

---

## Phase 3: User Story 1 — Personal Local Proxy (Priority: P1) 🎯 MVP

**Goal**: Developer runs proxy on laptop with zero external services; acts as own admin; PAT/API key catalog.

**Independent Test**: `LOCAL_MODE=true KMS_KEY_NAME=local LOCAL_KMS_KEY=$(openssl rand -hex 32) BASE_URL=http://localhost:8080 go run ./cmd/server` → browser to `http://localhost:8080` → register as first user → automatically granted admin → add a GitHub PAT catalog entry → retrieve proxy token → `curl http://localhost:8080/health` returns 200.

- [X] T015 [US1] Update `web/templates/dashboard.html` — add local/hosted mode badge near the page header; template receives `LocalMode bool` from `pages.go` dashboard data struct
- [X] T016 [US1] Update `internal/handler/pages.go` — pass `LocalMode` from config into the `DashboardData` struct rendered to `dashboard.html`
- [X] T017 [US1] Update `Makefile` — add `run-local` target: `LOCAL_MODE=true KMS_KEY_NAME=local BASE_URL=http://localhost:8080 go run ./cmd/server` (no Docker, no db-up dependency); update `.env.local` target to include `LOCAL_MODE=true`

**Checkpoint**: `make run-local` starts server; first registration is admin; catalog add with PAT works; dashboard shows "Local Mode" badge.

---

## Phase 4: User Story 2 — stdio MCP Server Bridging (Priority: P2)

**Goal**: A stdio catalog entry spawns a child process on demand; AI tool calls route through it transparently.

**Independent Test**: Register a catalog entry with `transport=stdio`, `command=npx`, `args=["-y","@modelcontextprotocol/server-filesystem","/tmp"]`; connect an AI tool to the proxy token URL; list tools → filesystem tools appear; call `read_file` on an existing file → response returned correctly.

- [X] T018 [US2] Create `internal/mcp/stdio.go` — `ConnectStdio(ctx, entry *store.CatalogEntry) (*UpstreamClient, error)` function that builds `exec.Command(entry.Command, entry.Args...)` with `entry.Env` merged into the process environment, wraps it in `mcp.CommandTransport`, connects and returns an `UpstreamClient`
- [X] T019 [US2] Update `internal/mcp/aggregator.go` — in the section that connects to upstream servers, branch on `entry.Transport`: call `ConnectStdio` for `"stdio"` entries, existing `Connect` for `"http"` entries
- [X] T020 [US2] Update `internal/upstream/registry.go` (or the adapter lookup path) — for stdio-type entries, bypass the `Adapter.AuthHeader()` call (stdio servers have no HTTP auth header); ensure the nil-adapter case is handled cleanly without a panic
- [X] T021 [US2] Update `internal/handler/upstream.go` — for stdio catalog entries, the user's credential prompt (PAT entry page) should be skipped or show a "no credentials needed" message since stdio auth type is `"none"`

**Checkpoint**: After T018–T021, running `make run-local` and registering a filesystem stdio server allows an MCP client to list its tools via the proxy.

---

## Phase 5: User Story 3 — PAT-Only Catalog Constraint in Local Mode (Priority: P3)

**Goal**: `auth_type=oauth2` rejected in local mode; `transport=stdio` rejected in hosted mode; clear error messages.

**Independent Test**: In local mode, POST `/api/admin/catalog` with `auth_type=oauth2` → `400` with message containing "local mode". In hosted mode, POST with `transport=stdio` → `400` with message containing "hosted mode".

- [X] T022 [US3] Update `internal/handler/admin.go` — `AddCatalogEntryAPI`: add mode-aware validation: (1) if `cfg.LocalMode && req.AuthType == "oauth2"` → 400 error; (2) if `!cfg.LocalMode && req.Transport == "stdio"` → 400 error; (3) if `req.Transport == "stdio" && req.Command == ""` → 400 error; pass `LocalMode` into handler via constructor
- [X] T023 [US3] Add `ModeHandler` to `internal/handler/admin.go` — `GET /api/admin/mode` returns `{"mode":"local"}` or `{"mode":"hosted"}` based on `cfg.LocalMode`
- [X] T024 [US3] Register `GET /api/admin/mode` route in `cmd/server/main.go` — no admin middleware required (public endpoint)

**Checkpoint**: Unit-testable via direct handler call; integration test via curl in both modes.

---

## Phase 6: User Story 4 — Dual-Proxy Setup (Priority: P4)

**Goal**: Dashboard setup guide shows how to configure an AI tool with both a local and a hosted proxy URL.

**Independent Test**: Visit `http://localhost:8080` dashboard → "Connect Your AI Tools" section → Claude Code snippet includes two `claude mcp add` commands (one for local, one for company proxy with placeholder).

- [X] T025 [US4] Update `web/templates/dashboard.html` setup guide — add a "Dual-Proxy Setup" subsection to the Claude Code collapsible showing both `claude mcp add` commands; local proxy URL uses `{{.ProxyURL}}`; company proxy URL uses a `{{/* COMPANY_URL_PLACEHOLDER */}}` comment with explanatory note
- [X] T026 [US4] Update `web/static/style.css` if needed to style the dual-proxy subsection consistently with existing `.client-setup` elements

**Checkpoint**: Dashboard renders without template errors in both local and hosted mode.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T027 [P] Update `README.md` — add "Local Mode" section documenting `make run-local`, environment variables (`LOCAL_MODE`, `DATA_DIR`), and a link to `specs/002-local-mode-stdio/quickstart.md`
- [X] T028 Run `make lint` and fix any issues (run `make lint-fix` for auto-fixable items)
- [X] T029 Run `make test` and verify all existing tests still pass
- [ ] T030 Manual smoke test against `quickstart.md` — follow all 7 steps end-to-end in local mode; confirm stdio filesystem server is callable

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — BLOCKS all user stories
- **US1 (Phase 3)**: Depends on Foundational — first deliverable (MVP)
- **US2 (Phase 4)**: Depends on Foundational + T004 (catalog schema with Transport field)
- **US3 (Phase 5)**: Depends on Foundational + T022 handler changes (needs LocalMode in config, T012)
- **US4 (Phase 6)**: Depends on US1 (dashboard template established)
- **Polish (Phase 7)**: Depends on all phases complete

### Within Foundational Phase

```
T003 → T004 → T005 (serial — interfaces first, then catalog, then verify)
T006 (SQLite db.go) must precede T007–T011
T007, T008, T009, T010, T011 can run in parallel (different files)
T012 (config) can run in parallel with T006–T011
T013 (migration SQL) can run in parallel with everything
T014 (main.go wiring) depends on T003–T013 all complete
```

### Parallel Opportunities

- T007, T008, T009, T010, T011 — all SQLite store implementations (different files)
- T013 (migration) — pure SQL file, no code dependencies
- T018 (stdio.go) and T022 (handler validation) — different packages
- T027 (README) — docs only, no code conflicts

---

## Parallel Example: Foundational SQLite Stores

```
# After T006 (db.go) is done, launch all store implementations together:
Task T007: internal/store/sqlite/users.go
Task T008: internal/store/sqlite/catalog.go
Task T009: internal/store/sqlite/upstream.go
Task T010: internal/store/sqlite/oauth2state.go
Task T011: internal/store/sqlite/suggestions.go
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001–T002)
2. Complete Phase 2: Foundational (T003–T014) — hardest phase
3. Complete Phase 3: US1 (T015–T017)
4. **STOP and VALIDATE**: `make run-local` works end-to-end; first user is admin; PAT catalog entry works
5. Demo local proxy to stakeholders

### Incremental Delivery

1. Setup + Foundational → binary can boot in both modes
2. US1 → personal local proxy MVP
3. US2 → add stdio server support
4. US3 → enforce mode constraints in catalog API
5. US4 → improve setup guide for dual-proxy users
6. Polish → lint, tests, docs

---

## Notes

- No new tests are scoped in this feature — existing `go test ./...` suite is the safety net
- The SQLite implementations must be validated with a real SQLite file, not just compile-checked
- The stdio subprocess test (T030) requires `npx` + Node.js installed locally
- Principle II constitution amendment (T001) must be committed before the PR is opened
