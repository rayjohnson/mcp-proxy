# Tasks: Proxy Management MCP Tools

**Input**: Design documents from `specs/004-proxy-mgmt-tools/`

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)

---

## Phase 1: Setup

**Purpose**: Scaffold the new file so later phases can be written independently.

- [x] T001 Create `internal/mcp/mgmt.go` with package declaration, `ManagementDeps` struct (UpstreamStore, CatalogStore, KMSEncrypt func), and empty `registerManagementTools(server, ps, deps)` function signature

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Wire user identity and management deps into the session and server layers. Must be complete before any tool handler can be tested.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [x] T002 Add `UserID string` field to `ProxySession` struct and populate it from the `userID` parameter in `OpenSession` in `internal/mcp/session.go`
- [x] T003 Add `ManagementDeps` field to `ProxyServerDeps` struct, thread it into `buildMCPServer`, and call `registerManagementTools(server, ps, deps.ManagementDeps)` after the upstream tool loop in `internal/mcp/server.go`
- [x] T004 Wire `ManagementDeps{UpstreamStore: upstreamStore, CatalogStore: catalogStore, KMSEncrypt: kmsClient.Encrypt}` into the `ProxyServerDeps` literal in `cmd/mcp-proxy/main.go`

**Checkpoint**: `go build ./...` succeeds with stub `registerManagementTools` — foundation ready.

---

## Phase 3: User Story 1 — Discover and Connect an Upstream (Priority: P1) 🎯 MVP

**Goal**: User can list catalog entries and connect a new api_key/PAT upstream from any MCP client.

**Independent Test**: Call `proxy_list_catalog` — confirm catalog entries appear with `requires_oauth` flag. Then call `proxy_connect_upstream` with a valid catalog ID and API key — confirm a new upstream ID is returned. Then call with an OAuth2 catalog ID — confirm descriptive error message.

- [x] T005 [P] [US1] Implement `proxy_list_catalog` handler in `internal/mcp/mgmt.go`: call `deps.CatalogStore.ListActiveCatalogEntries`, return JSON array with `id`, `display_name`, `description`, `server_type`, `auth_type`, `requires_oauth` (derived from `auth_type == "oauth2"`); strip all OAuth credential fields
- [x] T006 [P] [US1] Implement `proxy_connect_upstream` handler in `internal/mcp/mgmt.go`: validate `catalog_id` and `api_key` are non-empty, look up entry, return error for `oauth2`/`stdio`/`none` auth types, call `deps.KMSEncrypt`, call `deps.UpstreamStore.CreateUpstreamConfig`, return `{id, server_type, status}`
- [x] T007 [US1] Register `proxy_list_catalog` and `proxy_connect_upstream` via `server.AddTool` in `registerManagementTools` in `internal/mcp/mgmt.go`

**Checkpoint**: US1 fully functional — users can discover and connect api_key upstreams via MCP.

---

## Phase 4: User Story 2 — List and Remove Connected Upstreams (Priority: P2)

**Goal**: User can see all their connected upstreams with status and disconnect any of them.

**Independent Test**: Call `proxy_list_upstreams` — confirm list shows display names (joined from catalog), server type, status, and no credential fields. Call `proxy_disconnect_upstream` with a valid upstream ID — confirm it's removed. Call with an unknown ID — confirm "Upstream not found" error.

- [x] T008 [P] [US2] Implement `proxy_list_upstreams` handler in `internal/mcp/mgmt.go`: call `deps.UpstreamStore.GetUpstreamConfigsByUserID(ps.UserID)`, call `deps.CatalogStore.ListActiveCatalogEntries` to build a `map[serverType]displayName`, return JSON array with `id`, `display_name`, `server_type`, `auth_type`, `status`, `detected_transport`; never include `encrypted_creds` or `user_id`
- [x] T009 [P] [US2] Implement `proxy_disconnect_upstream` handler in `internal/mcp/mgmt.go`: look up config by ID, verify `cfg.UserID == ps.UserID`, call `deps.UpstreamStore.DeleteUpstreamConfig`, return success text or ownership error
- [x] T010 [US2] Register `proxy_list_upstreams` and `proxy_disconnect_upstream` via `server.AddTool` in `registerManagementTools` in `internal/mcp/mgmt.go`

**Checkpoint**: US2 fully functional — users can list and remove upstreams via MCP.

---

## Phase 5: User Story 3 — Update Credentials (Priority: P3)

**Goal**: User can rotate an API key for an existing upstream without disconnecting and reconnecting.

**Independent Test**: Call `proxy_update_credentials` with a valid upstream ID and new API key — confirm success and upstream status resets to active. Call with an upstream ID belonging to a different user — confirm "Upstream not found" error.

- [x] T011 [US3] Implement `proxy_update_credentials` handler in `internal/mcp/mgmt.go`: look up config by ID, verify `cfg.UserID == ps.UserID`, validate `api_key` non-empty, call `deps.KMSEncrypt`, call `deps.UpstreamStore.UpdateEncryptedCreds` then `UpdateUpstreamStatus(..., "active")`, return success text
- [x] T012 [US3] Register `proxy_update_credentials` via `server.AddTool` in `registerManagementTools` in `internal/mcp/mgmt.go`

**Checkpoint**: All 3 user stories complete — all 5 management tools functional.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [x] T013 Run `go build ./...` from repo root and fix any compilation errors
- [x] T014 Run `go test ./internal/mcp/...` and verify all existing proxy tests still pass
- [x] T015 [P] Run `golangci-lint run --fix ./...` and fix all remaining warnings

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on T001 — blocks all user stories
- **US1 (Phase 3)**: Depends on T001–T004
- **US2 (Phase 4)**: Depends on T001–T004; T008 and T009 can run in parallel with US1
- **US3 (Phase 5)**: Depends on T001–T004
- **Polish (Phase 6)**: Depends on all story phases complete

### Parallel Opportunities

- T005 and T006 (both US1 handlers) can be written in parallel — different functions in the same file, no shared state
- T008 and T009 (US2 handlers) can be written in parallel
- US2 (Phase 4) can begin in parallel with US1 (Phase 3) once Foundational is complete
- US3 (Phase 5) can begin in parallel with US1 and US2

---

## Implementation Strategy

### MVP (User Story 1 Only)

1. Complete Phase 1 (T001)
2. Complete Phase 2 (T002–T004) — verify `go build ./...` passes
3. Complete Phase 3 (T005–T007)
4. **Validate**: Connect Claude Code to the proxy, call `proxy_list_catalog` and `proxy_connect_upstream`

### Full Delivery

Add Phase 4 → Phase 5 → Phase 6 in sequence, validating at each checkpoint.

---

## Notes

- All 5 tools must appear in the MCP `list_tools` response alongside proxied upstream tools
- `encrypted_creds` must never appear in any tool response — check every JSON marshal path
- Ownership check pattern is the same in T009 and T011: look up by ID, compare `cfg.UserID == ps.UserID`, return "Upstream not found" on mismatch (do not distinguish "not found" from "wrong owner")
