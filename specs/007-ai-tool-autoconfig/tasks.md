# Tasks: AI Tool Auto-Configuration

**Input**: Design documents from `specs/007-ai-tool-autoconfig/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/api.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the `internal/aitools` package skeleton and wire it into the existing handler/router infrastructure.

- [x] T001 Create `internal/aitools/registry.go` — define `AITool` struct (`ID`, `DisplayName`, `Status`, `ErrorMessage`), `ToolStatus` type constants, and `Configurer` interface with `Detect() AITool` and `Configure(proxyURL string) error` methods
- [x] T00X Create `internal/handler/aitools.go` — define `AIToolsHandler` struct with `localMode bool` and slice of `Configurer`, and stub `StatusAPI` (GET /api/tools) and `ConfigureAPI` (POST /api/tools/{id}/configure) methods returning 503 if not local mode
- [x] T00X Wire `AIToolsHandler` into `cmd/mcp-proxy/main.go` — register GET `/api/tools` and POST `/api/tools/{id}/configure` routes on the existing `mux`, gated by `cfg.LocalMode`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared helpers used by all tool implementations.

**⚠️ CRITICAL**: Tool implementations (Phase 3+) depend on this phase being complete.

- [x] T00X Add `proxyURLFromRequest(r *http.Request) string` helper in `internal/handler/aitools.go` — derives `http://host:port` from the incoming request's Host header
- [x] T00X Add `atomicWriteJSON(path string, data any) error` helper in `internal/aitools/registry.go` — marshals data to a temp file in the same directory, then `os.Rename` to target path; returns error without touching the original on any failure

**Checkpoint**: Helpers complete — tool-specific implementations can now proceed in parallel.

---

## Phase 3: User Story 1 — One-Click Configure Claude Desktop (Priority: P1) 🎯 MVP

**Goal**: Detect Claude Desktop installation/config status and write the proxy entry into `claude_desktop_config.json` in one click.

**Independent Test**: With Claude Desktop installed but not configured, call `POST /api/tools/claude-desktop/configure`. Verify `~/Library/Application Support/Claude/claude_desktop_config.json` now contains `mcpServers.mcp-proxy` with `command: npx` and the correct proxy URL. Call `GET /api/tools` and confirm status is `configured`.

### Implementation for User Story 1

- [x] T00X [P] [US1] Create `internal/aitools/claude_desktop.go` — `ClaudeDesktopTool` struct implementing `Configurer`; `Detect()` checks `/Applications/Claude.app` exists and reads `~/Library/Application Support/Claude/claude_desktop_config.json` to determine status
- [x] T00X [US1] Implement `ClaudeDesktopTool.Configure(proxyURL string)` in `internal/aitools/claude_desktop.go` — read existing JSON (or empty object if missing), merge `mcpServers["mcp-proxy"]` entry `{"command":"npx","args":["-y","mcp-remote","<proxyURL>/mcp","--allow-http"]}`, write atomically via T005 helper
- [x] T00X [US1] Register `ClaudeDesktopTool` in `internal/handler/aitools.go` — add to the `Configurer` slice passed to `NewAIToolsHandler`
- [x] T00X [US1] Implement `AIToolsHandler.StatusAPI` in `internal/handler/aitools.go` — call `Detect()` on each tool, return JSON array matching contract in `contracts/api.md`
- [x] T0XX [US1] Implement `AIToolsHandler.ConfigureAPI` in `internal/handler/aitools.go` — look up tool by ID, return 404 if not installed, call `Configure(proxyURL)`, return updated status or 500 with original config unchanged on error

**Checkpoint**: Claude Desktop detection and one-click configure fully functional. `GET /api/tools` and `POST /api/tools/claude-desktop/configure` work end-to-end.

---

## Phase 4: User Story 2 — Configure Gemini CLI (Priority: P2)

**Goal**: Detect Gemini CLI installation/config status and register the proxy via `gemini mcp add`.

**Independent Test**: With Gemini CLI installed but not configured, call `POST /api/tools/gemini-cli/configure`. Run `gemini mcp list` and confirm `mcp-proxy` appears. Call `GET /api/tools` and confirm status is `configured`.

### Implementation for User Story 2

- [x] T0XX [P] [US2] Create `internal/aitools/gemini_cli.go` — `GeminiCLITool` struct implementing `Configurer`; `Detect()` resolves `gemini` binary in PATH using `exec.LookPath`, then runs `gemini mcp list` with a 5s timeout to check if `mcp-proxy` is already registered; returns `not_installed` / `unconfigured` / `configured`
- [x] T0XX [US2] Implement `GeminiCLITool.Configure(proxyURL string)` in `internal/aitools/gemini_cli.go` — run `gemini mcp add mcp-proxy <proxyURL>/mcp` as a subprocess with 10s timeout; on non-zero exit return the stderr as the error message
- [x] T0XX [US2] Register `GeminiCLITool` in `internal/handler/aitools.go` — add to the `Configurer` slice alongside `ClaudeDesktopTool`

**Checkpoint**: Both Claude Desktop and Gemini CLI fully operational. `GET /api/tools` returns both; each can be configured independently.

---

## Phase 5: User Story 3 — Dashboard Status View (Priority: P3)

**Goal**: Display the AI Tools section in the dashboard with live status and Configure buttons.

**Independent Test**: With both tools in various states, open the dashboard. Confirm each tool row shows the correct status badge. Confirm the Configure button triggers the API and updates the row without a page reload. Confirm `not_installed` rows have no button.

### Implementation for User Story 3

- [x] T0XX [P] [US3] Create `templates/partials/ai-tools.html` — renders a card section "Connect Your AI Tools"; iterates over tools via JS fetch; shows status badge (`Configured ✓`, `Not configured`, `Not installed`); shows Configure button only when status is `unconfigured`; includes a loading/error state
- [x] T0XX [P] [US3] Create `static/js/ai-tools.js` — on DOMContentLoaded fetches `GET /api/tools`, renders rows, attaches click handler to Configure buttons that calls `POST /api/tools/{id}/configure` and updates the row status in place
- [x] T0XX [US3] Register `ai-tools.html` partial in `internal/handler/pages.go` — add to the `partials` glob list so it's parsed with the dashboard template
- [x] T0XX [US3] Add AI Tools section to `templates/dashboard.html` — include the `ai-tools.html` partial and a `<script src="/static/js/ai-tools.js">` tag; section only renders when `localMode` is true
- [x] T0XX [US3] Serve `static/js/ai-tools.js` via the existing static file handler in `cmd/mcp-proxy/main.go`

**Checkpoint**: Dashboard shows live AI tool status. Configure button works. Status refreshes without page reload.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Error handling, edge cases, and cleanup.

- [x] T0XX [P] Add `//nolint:gosec` annotation to `exec.Command` calls in `internal/aitools/gemini_cli.go` with comment explaining the args are constructed from validated internal values
- [x] T0XX [P] Handle missing config dir gracefully in `internal/aitools/claude_desktop.go` — `os.MkdirAll` the `~/Library/Application Support/Claude/` directory before writing if it doesn't exist
- [x] T0XX Add `error_message` field to the JSON response in `StatusAPI` when a tool's status is `error`, matching the contract in `contracts/api.md`
- [x] T0XX [P] Run `go vet ./...` and `golangci-lint run ./...` — fix any issues in new files

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 completion — blocks all tool implementations
- **US1 (Phase 3)**: Depends on Phase 2 — no dependency on US2/US3
- **US2 (Phase 4)**: Depends on Phase 2 — no dependency on US1/US3
- **US3 (Phase 5)**: Depends on Phase 3 (needs working API endpoints)
- **Polish (Phase 6)**: Depends on all story phases

### User Story Dependencies

- **US1**: Can start after Phase 2 — independent of US2/US3
- **US2**: Can start after Phase 2 — independent of US1/US3; T011-T012 can run in parallel with US1
- **US3**: Depends on US1 API being complete; T014 and T015 can be written in parallel with US2

### Parallel Opportunities

- T006 (claude_desktop.go) and T011 (gemini_cli.go) can run in parallel — different files
- T014 (ai-tools.html) and T015 (ai-tools.js) can run in parallel — different files
- T019, T020, T022 in Phase 6 can run in parallel

---

## Implementation Strategy

### MVP (User Story 1 Only)

1. Complete Phase 1: Setup (T001–T003)
2. Complete Phase 2: Foundational (T004–T005)
3. Complete Phase 3: Claude Desktop (T006–T010)
4. **STOP and VALIDATE**: `GET /api/tools` returns Claude Desktop status; `POST /api/tools/claude-desktop/configure` writes config atomically
5. Ship — Gemini CLI and dashboard UI can follow incrementally

### Incremental Delivery

1. Setup + Foundational → skeleton wired in
2. US1 (Claude Desktop) → API functional for most common tool
3. US2 (Gemini CLI) → API covers both supported tools
4. US3 (Dashboard UI) → visual one-click flow complete
5. Polish → production-quality error handling

---

## Notes

- All new files in `internal/aitools/` are plain Go — no `//go:build darwin` constraint needed (file I/O and subprocesses work on any platform; macOS-specific paths simply won't match on Linux/Windows)
- The `atomicWriteJSON` helper (T005) is the safest approach for Claude Desktop config writes — preserves the original file on any failure
- Gemini CLI subprocess calls must set a timeout to avoid hanging the HTTP request
- The dashboard JS section is only loaded when `localMode` is true, keeping the hosted-mode UI clean
