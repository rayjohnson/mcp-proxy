---
description: "Task list for Core MCP Proxy MVP"
---

# Tasks: Core MCP Proxy MVP

**Input**: `specs/001-core-proxy-mvp/` (spec, plan, research, data-model, contracts)

**Branch**: `001-core-proxy-mvp`

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel with other [P] tasks in the same phase
- **[Story]**: US1–US4 map to the four user stories in spec.md
- All paths are relative to the repository root

---

## Phase 1: Setup

**Purpose**: Scaffolding that every subsequent task depends on.

- [X] T001 Initialize Go module: `go mod init github.com/rayjohnson/mcp-proxy`
- [X] T002 Create project directory skeleton: `cmd/server/`, `internal/`, `web/templates/`, `web/static/`, `migrations/`, `deploy/`, `tests/integration/`, `docs/`
- [X] T003 [P] Vendor HTMX v2: download `htmx.min.js` to `web/static/htmx.min.js`
- [X] T004 [P] Write `deploy/Dockerfile`: multi-stage Go build, non-root user, expose port 8080
- [X] T005 [P] Write `deploy/service.yaml`: Cloud Run service definition (image placeholder, env var references, Cloud SQL connection, min-instances 0)
- [X] T006 [P] Write `internal/config/config.go`: load all runtime config from environment variables (DB DSN, KMS key name, port, OIDC config placeholder)

---

## Phase 2: Foundation

**Purpose**: Core infrastructure that MUST be complete before any user story.

⚠️ **CRITICAL**: No user story work begins until this phase is complete.

- [X] T007 Write `migrations/001_initial_schema.sql`: create all five tables (`users`, `upstream_configs`, `default_catalog`, `catalog_suggestions`, `oauth2_state`) with constraints and indexes per `data-model.md`
- [X] T008 Write `internal/store/db.go`: Cloud SQL connection setup via `pgx/v5` pool; Cloud SQL Auth Proxy socket path from config
- [X] T009 [P] Write `internal/kms/kms.go`: GCP KMS encrypt/decrypt wrapper using `cloud.google.com/go/kms`; exported `Encrypt(plaintext []byte) ([]byte, error)` and `Decrypt(ciphertext []byte) ([]byte, error)`
- [X] T010 [P] Write `internal/store/users.go`: `CreateUser`, `GetUserByEmail`, `GetUserByID`, `GetUserByProxyToken`
- [X] T011 [P] Write `internal/store/upstream.go`: `CreateUpstreamConfig`, `GetUpstreamConfigsByUserID`, `GetUpstreamConfigByID`, `UpdateUpstreamStatus`, `UpdateDetectedTransport`, `UpdateEncryptedCreds`, `DeleteUpstreamConfig`
- [X] T012 [P] Write `internal/store/catalog.go`: `AddCatalogEntry`, `ListActiveCatalogEntries`, `DeactivateCatalogEntry`, `GetCatalogEntryByServerType`
- [X] T013 [P] Write `internal/store/suggestions.go`: `CreateSuggestionForAllUsers`, `ListPendingSuggestionsForUser`, `ResolveSuggestion` (accept/dismiss), `GetSuggestion`
- [X] T014 [P] Write `internal/store/oauth2state.go`: `CreateOAuth2State`, `ConsumeOAuth2State` (fetch + delete in one tx), `DeleteExpiredStates`
- [X] T015 Write `internal/auth/password.go`: bcrypt hash + verify (depends on T010 being defined)
- [X] T016 [P] Write `internal/auth/jwt.go`: sign and verify JWTs; 24h expiry; claims: user ID + role
- [X] T017 [P] Write `internal/auth/idp.go`: `IdentityProvider` interface (`Authenticate(code, state string) (*User, error)`, `AuthURL(state string) string`) for future JumpCloud OIDC plug-in
- [X] T018 Write `internal/handler/middleware.go`: JWT cookie auth middleware; role-check middleware (admin gate); request logging
- [X] T019 Write `cmd/server/main.go`: wire up config, DB pool, KMS client, HTTP router, start server on `$PORT`

**Checkpoint**: Run `go build ./...` — zero errors. All packages compile.

---

## Phase 3: User Story 1 — Single Proxy Endpoint (Priority: P1) 🎯 MVP

**Goal**: An AI tool can connect to the proxy endpoint and call tools from all of
a developer's configured upstream servers through one URL.

**Independent Test**: Register account → add one upstream server (any type) →
connect Claude Desktop → confirm tools appear and are callable.

### Tests for User Story 1

> **Write these tests FIRST — they must FAIL before implementation begins**

- [X] T020 [P] [US1] Write `tests/integration/proxy_roundtrip_test.go`: end-to-end test — register user, insert a test upstream config, connect MCP client to `/mcp/{proxy_token}`, call `tools/list`, assert tools are prefixed correctly
- [X] T021 [P] [US1] Write `tests/integration/transport_detection_test.go`: mock upstream serving SSE; assert proxy connects via SSE fallback and `detected_transport` is set to `sse` in DB

### Implementation for User Story 1

- [X] T022 [US1] Write `internal/mcp/client.go`: MCP upstream client using `go-sdk`; `Connect(serverURL, authHeader string) (Client, error)` — tries `NewStreamableHTTPClientTransport` first, falls back to `NewSSEClientTransport`; updates `detected_transport` in store on success (depends on T011)
- [X] T023 [US1] Write `internal/mcp/session.go`: per-user proxy session — holds map of `server_type → upstream Client`; `OpenSession(userID)` loads user's upstream configs, connects all reachable ones concurrently (depends on T022)
- [X] T024 [US1] Write `internal/mcp/aggregator.go`: `AggregateTolList(sessions) []Tool` — collects tool lists from all upstream clients, prefixes each name `{server_type}__{name}` (depends on T023)
- [X] T025 [US1] Write `internal/mcp/router.go`: `RouteToolCall(toolName, params, sessions)` — parses `{server_type}__` prefix, forwards call to matching upstream client, returns response unmodified (depends on T023)
- [X] T026 [US1] Write `internal/mcp/server.go`: Streamable HTTP MCP server using `go-sdk`; handles `GET /mcp/{proxy_token}` (SSE stream) and `POST /mcp/{proxy_token}` (messages); validates proxy token via store; delegates to session, aggregator, router (depends on T023, T024, T025)
- [X] T027 [US1] Wire proxy endpoint into router in `cmd/server/main.go` at `/mcp/` (depends on T026, T019)

**Checkpoint**: T020 integration test passes. Claude Desktop can list tools from
a configured upstream server through the proxy URL.

---

## Phase 4: User Story 2 — Admin Default Catalog (Priority: P2)

**Goal**: Admin configures default servers; new users see them pre-listed; existing
users receive suggestions when admin adds a new server.

**Independent Test**: Admin adds server to catalog → new user sees it on dashboard →
existing user sees suggestion within 60 seconds.

### Tests for User Story 2

> **Write these tests FIRST**

- [X] T028 [P] [US2] Write `tests/integration/catalog_test.go`: admin adds catalog entry → assert `catalog_suggestions` rows created for all existing users; new user registers → assert pre-listed servers match catalog
- [X] T029 [P] [US2] Write `tests/integration/suggestion_test.go`: developer dismisses suggestion → re-fetch → assert suggestion does not reappear

### Implementation for User Story 2

- [X] T030 [US2] Write `internal/catalog/service.go`: `AddToCatalog(entry)` — inserts into `default_catalog`, then calls `CreateSuggestionForAllUsers`; `RemoveFromCatalog(id)` — soft delete (depends on T012, T013)
- [X] T031 [US2] Write `internal/handler/admin.go`: admin API handlers — `GET /api/admin/catalog`, `POST /api/admin/catalog`, `DELETE /api/admin/catalog/{id}` per `contracts/management-api.md`; protected by admin role middleware (depends on T030, T018)
- [X] T032 [US2] Write `internal/handler/suggestions.go`: `GET /api/suggestions`, `POST /api/suggestions/{id}/dismiss`, `POST /api/suggestions/{id}/accept` (depends on T013)
- [X] T033 [P] [US2] Write `web/templates/admin-catalog.html` + `web/templates/partials/catalog-row.html`: admin catalog management page; HTMX-powered add/remove rows
- [X] T034 [P] [US2] Write `web/templates/partials/suggestions-list.html`: suggestion cards with HTMX dismiss (`hx-post`, `hx-swap="outerHTML"`) and accept actions
- [X] T035 [US2] Integrate pre-listed catalog servers into new-user registration flow in `internal/handler/auth.go` — after account creation, seed dashboard with active catalog entries (depends on T030, T031 handler structure)

**Checkpoint**: T028 and T029 integration tests pass. Admin can manage catalog;
suggestions appear and dismiss correctly.

---

## Phase 5: User Story 3 — Developer Configuration (Priority: P3)

**Goal**: Developer adds, updates credentials for, or removes upstream servers
including OAuth2-based ones — without disrupting the proxy.

**Independent Test**: Add Cloudflare (API key) + Notion (OAuth2 flow) → both tools
appear in proxy → remove Cloudflare → only Notion tools remain.

### Tests for User Story 3

> **Write these tests FIRST**

- [X] T036 [P] [US3] Write `tests/integration/upstream_config_test.go`: add API-key server → tools appear in proxy; remove server → tools gone; update credentials → status returns to reachable
- [X] T037 [P] [US3] Write `tests/integration/oauth2_flow_test.go`: mock OAuth2 provider; exercise full authorize → callback → token storage → token refresh flow

### Implementation for User Story 3

- [X] T038 [US3] Write `internal/upstream/adapter.go`: `UpstreamAdapter` interface — `AuthHeader(decryptedCreds) (string, error)`, `OAuth2Config() *oauth2.Config`; implemented by each service type
- [X] T039 [P] [US3] Write `internal/upstream/github.go`: GitHub adapter (OAuth2 + API key variants)
- [X] T040 [P] [US3] Write `internal/upstream/notion.go`: Notion adapter (OAuth2)
- [X] T041 [P] [US3] Write `internal/upstream/linear.go`: Linear adapter (API key + OAuth2)
- [X] T042 [P] [US3] Write `internal/upstream/cloudflare.go`: Cloudflare adapter (API key)
- [X] T043 [P] [US3] Write `internal/upstream/googlecloud.go`: Google Cloud adapter (OAuth2)
- [X] T044 [US3] Write `internal/oauth2client/service.go`: `StartAuthFlow(userID, serverType) (redirectURL string, error)` — generates state token, stores in `oauth2_state`, returns upstream authorization URL; `HandleCallback(serverType, code, state string) error` — validates state, exchanges code for tokens, encrypts with KMS, stores in `upstream_configs` (depends on T009, T014, T038–T043)
- [X] T045 [US3] Write `internal/handler/upstream.go`: API handlers — `GET /api/upstream`, `POST /api/upstream`, `DELETE /api/upstream/{id}`, `PATCH /api/upstream/{id}/credentials`, `GET /api/upstream/{id}/status` per `contracts/management-api.md` (depends on T011, T044)
- [X] T046 [US3] Write `internal/handler/oauth2.go`: `GET /api/oauth2/authorize/{server_type}` and `GET /api/oauth2/callback/{server_type}` (depends on T044)
- [X] T047 [US3] Write `internal/oauth2client/refresh.go`: `RefreshIfExpired(config *UpstreamConfig) error` — checks token expiry, calls refresh endpoint if within 5-minute window, re-encrypts and stores updated token pair; detects `invalid_grant` and sets status to `reauth_required` (depends on T044)
- [X] T048 [P] [US3] Write `web/templates/dashboard.html`: main developer dashboard — server list, proxy endpoint URL display, suggestions section; HTMX status polling (`hx-get`, `hx-trigger="every 30s"`) on status badges
- [X] T049 [P] [US3] Write `web/templates/partials/server-row.html`: upstream server row with status badge, remove button (`hx-delete`), re-auth button when applicable
- [X] T050 [P] [US3] Write `web/templates/partials/add-server-form.html`: add server inline form — branches on auth type (API key input vs. OAuth2 authorize button)
- [X] T051 [US3] Wire all upstream + OAuth2 handlers into router in `cmd/server/main.go` (depends on T045, T046)

**Checkpoint**: T036 and T037 tests pass. Full add/remove/re-auth flow works in
the management UI.

---

## Phase 6: User Story 4 — Failure Isolation (Priority: P4)

**Goal**: One unavailable upstream does not break the proxy for all others.

**Independent Test**: Simulate Notion unreachable → GitHub tools still callable →
dashboard shows Notion as `unreachable`.

### Tests for User Story 4

> **Write these tests FIRST**

- [X] T052 [US4] Write `tests/integration/failure_isolation_test.go`: configure two upstreams; block one at network level; assert `tools/list` returns the working server's tools; assert blocked server status is `unreachable`

### Implementation for User Story 4

- [X] T053 [US4] Update `internal/mcp/session.go`: catch per-upstream connection errors in `OpenSession`; mark failed upstreams as unavailable in session without failing the whole session (depends on T023)
- [X] T054 [US4] Update `internal/mcp/aggregator.go`: skip upstreams with no live client connection; do not return error if at least one upstream is reachable (depends on T024)
- [X] T055 [US4] Update `internal/mcp/router.go`: return scoped JSON-RPC error if the target upstream is unavailable; do not affect other tools (depends on T025)
- [X] T056 [US4] Write `internal/handler/health.go`: background goroutine that probes each user's upstream configs every 5 minutes; updates `status` and `status_checked_at` in `upstream_configs`; triggers `RefreshIfExpired` for OAuth2 configs (depends on T047, T011)

**Checkpoint**: T052 integration test passes. Proxy survives individual upstream
failures without degrading other servers.

---

## Phase 7: Auth UI + Registration Flow

**Purpose**: End-to-end auth so a real developer can sign up and use the system.

- [X] T057 [P] Write `internal/handler/auth.go`: `POST /api/auth/register`, `POST /api/auth/login`, `POST /api/auth/logout` per `contracts/management-api.md`; sets JWT cookie on login/register (depends on T015, T016, T010)
- [X] T058 [P] Write `web/templates/login.html`: login form with HTMX form submission and inline error display
- [X] T059 [P] Write `web/templates/register.html`: registration form
- [X] T060 [P] Write `web/templates/layout.html`: base layout with nav, HTMX script include, CSS link
- [X] T061 Write `GET /api/proxy/endpoint` handler in `internal/handler/auth.go` — returns stable proxy endpoint URL for current user
- [X] T062 Wire auth handlers and page routes (`GET /login`, `GET /register`, `GET /dashboard`) into router (depends on T057–T061)

---

## Phase 8: Polish & Hardening

- [X] T063 [P] Credential log audit: grep all log statements across `internal/` for any path that could emit credential values; fix any found
- [X] T064 [P] Run `go vet ./...` and `staticcheck ./...`; fix all findings
- [X] T065 [P] Write `deploy/Dockerfile` health check: `HEALTHCHECK CMD curl -f http://localhost:8080/healthz || exit 1`; add `GET /healthz` handler
- [X] T066 [P] Add `web/static/style.css`: minimal stylesheet for management UI (functional, not polished)

### Phase 9: Features Added During Implementation (FR-023 – FR-026)

- [X] T068 [P] Add `migrations/002_catalog_auth.sql`: extend `default_catalog` with `auth_type`, `oauth_client_id`, `encrypted_oauth_secret`; update `CatalogStore` and `catalog.Service`
- [X] T069 [P] Extend upstream adapter interface: `OAuth2Config(clientID, clientSecret, redirectURL)` — credentials now come from catalog, not hardcoded; update all 5 adapters
- [X] T070 [P] Update `oauth2client.Service`: add `catalogStore` dependency; `oauthConfig()` helper looks up catalog entry, decrypts secret, builds `*oauth2.Config`
- [X] T071 [P] Redesign dashboard: catalog cards with per-server Connect button; OAuth2 → authorize flow, api_key → `/connect/{server_type}` key-entry form; add `connect.html` template
- [X] T072 [P] Add admin user management: `GET /admin/users`, `POST /admin/users/{id}/role`; `admin-users.html` template; `ListAllUsers` and `UpdateUserRole` on `UserStore`; prevent self-demotion
- [X] T073 [P] First-user admin bootstrap: `CountUsers()` on `UserStore`; registration assigns `admin` role when count is 0, `developer` otherwise
- [X] T074 [P] Fix `AuthMiddleware`: redirect browser requests to `/login` instead of returning plain-text 401; JSON `{"error":"unauthorized"}` for `/api/` paths
- [X] T075 [P] Fix PG16 compatibility: replace `encode(...,'base64url')` (PG17-only) with URL-safe base64 workaround in `001_initial_schema.sql`
- [X] T076 [P] Add `ErrDuplicateEmail` sentinel in `UserStore`; registration handler distinguishes duplicate from other DB errors
- [X] T077 [P] Add admin JSON API: `GET/POST /api/admin/catalog`, `DELETE /api/admin/catalog/{id}`; JSON errors on all `/api/` admin paths
- [X] T078 [P] Add AI tool setup guide to dashboard: collapsible config snippets for Claude Code, Claude Desktop, Cursor, VS Code, Windsurf; pre-filled with user's proxy URL
- [X] T079 [P] Add handler e2e tests (`auth_e2e_test.go`, `handler_test.go`): full register/login/dashboard/cookie/redirect flows using fake stores and `httptest.Server`
- [ ] T067 Run quickstart.md end-to-end validation against a staging Cloud Run deployment; fix any failures

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately
- **Phase 2 (Foundation)**: Depends on Phase 1 — BLOCKS all user stories
- **Phase 3–6 (User Stories)**: All depend on Phase 2 completion
  - Can proceed in priority order (P1 → P2 → P3 → P4) sequentially
  - Or in parallel if staffed (P1 is prerequisite for meaningful integration testing)
- **Phase 7 (Auth UI)**: Can run in parallel with Phase 3–6 after Phase 2
- **Phase 8 (Polish)**: Depends on all prior phases

### Key Within-Phase Dependencies

- T007 (schema) → T008 (DB pool) → T010–T014 (store functions)
- T009 (KMS) → T044 (OAuth2 service) → T045–T046 (handlers)
- T022 (MCP client) → T023 (session) → T024, T025 → T026 (MCP server)
- T038 (adapter interface) → T039–T043 (per-service adapters) → T044

### Parallel Opportunities

- T003–T006 all parallelizable within Phase 1
- T009–T014 all parallelizable within Phase 2 (after T007 + T008)
- T039–T043 (upstream adapters) fully parallel
- T033–T034, T048–T050 (templates) parallel within their phases
- T057–T060 (auth UI) all parallel within Phase 7
