# Feature Specification: Local Deployment Mode with stdio MCP Server Support

**Feature Branch**: `002-local-mode-stdio`

**Created**: 2026-05-23

**Status**: Draft

**Input**: User description: "local deployment mode with stdio MCP server support"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Personal Local Proxy (Priority: P1)

A developer runs the proxy on their own laptop. They act as their own admin — there is no separate admin setup, no OAuth2 app registration, and no cloud deployment required. They add MCP servers by PAT or API key and connect their AI tools to `http://localhost:PORT`.

**Why this priority**: This unlocks solo use without any infrastructure. It is the simplest entry point and the foundation all other local-mode stories build on.

**Independent Test**: A developer can install, run, and use the proxy on their laptop with a single GitHub PAT — adding the GitHub MCP server, connecting Claude Code, and listing tools — without creating any cloud resources or registering any OAuth2 application.

**Acceptance Scenarios**:

1. **Given** the binary is running on localhost with no prior configuration, **When** the developer opens the UI for the first time, **Then** they are prompted to create an admin account and are immediately granted admin privileges.
2. **Given** the developer is an admin in local mode, **When** they add a catalog entry with an API key or PAT, **Then** the entry is saved and immediately available to connected AI tools.
3. **Given** the proxy is running on localhost, **When** an AI tool connects to `http://localhost:PORT/mcp/{token}`, **Then** it receives the aggregated MCP tool list for that user.

---

### User Story 2 - stdio MCP Server Bridging (Priority: P2)

A developer has a locally installed MCP server that communicates over stdio (e.g., `npx @modelcontextprotocol/server-filesystem`). They want the proxy to discover and expose it alongside their cloud MCP servers — so their AI tool only needs one URL.

**Why this priority**: stdio servers are the most common MCP distribution format today. Without stdio support, the local proxy cannot replace direct per-tool MCP configuration.

**Independent Test**: A developer can add a stdio-backed catalog entry (specifying a command and arguments), connect an AI tool to the local proxy, and successfully invoke a tool that is served by the stdio process — without any manual tunneling or port configuration.

**Acceptance Scenarios**:

1. **Given** a valid stdio command is registered in the catalog (e.g., `npx @modelcontextprotocol/server-filesystem /home/user/docs`), **When** a user's AI tool calls a tool on that server, **Then** the proxy launches the process, forwards the request over stdio, and returns the response.
2. **Given** a stdio MCP server process is running, **When** the process exits unexpectedly, **Then** the proxy restarts it on the next request and does not permanently mark the server unavailable.
3. **Given** a stdio catalog entry specifies environment variables, **When** the proxy spawns the process, **Then** those variables are present in the child process environment.
4. **Given** a stdio command is not found on PATH, **When** the proxy attempts to start it, **Then** the user sees a clear error message (not a silent failure) and the server is listed as unavailable in the catalog.

---

### User Story 3 - PAT-Only Catalog Constraint in Local Mode (Priority: P3)

When the proxy runs in local mode, OAuth2 app credentials (client ID + secret) cannot be registered in the catalog. The only permitted auth types are API key and Personal Access Token. This matches the reality that local deployments cannot host an OAuth2 redirect endpoint reliably and that users should not be burdened with app registration for personal use.

**Why this priority**: This enforces a clean security boundary. Without it, a user could accidentally configure OAuth2 in a mode where it cannot work, causing confusing failures.

**Independent Test**: A developer running in local mode who attempts to add a catalog entry with `auth_type: oauth2` receives a clear rejection message. A developer running in hosted mode retains full OAuth2 support.

**Acceptance Scenarios**:

1. **Given** the proxy is running in local mode, **When** an admin submits a catalog entry with `auth_type: oauth2`, **Then** the request is rejected with a message explaining that OAuth2 app credentials require hosted mode.
2. **Given** the proxy is running in local mode, **When** an admin submits a catalog entry with `auth_type: pat` or `auth_type: api_key`, **Then** the entry is accepted and saved normally.
3. **Given** the proxy is running in hosted mode, **When** an admin submits a catalog entry with any auth type including `auth_type: oauth2`, **Then** all types are accepted as before.

---

### User Story 4 - Dual-Proxy Setup (Priority: P4)

A developer's company runs a hosted proxy that provides shared cloud MCP servers (e.g., internal Jira, shared GitHub org). The developer also runs a local proxy for personal tools (local filesystem, private repos via PAT, stdio servers). Their AI tool is configured with both URLs and sees a unified tool list.

**Why this priority**: The dual-proxy model is the target end-state for team deployments. It requires no changes to the AI tool after initial setup — the developer simply adds both proxy URLs.

**Independent Test**: An AI tool configured with two MCP proxy URLs (one local, one cloud) returns tools from both. Adding a new catalog entry to either proxy immediately makes its tools available to the AI tool without reconfiguring the tool.

**Acceptance Scenarios**:

1. **Given** an AI tool has two MCP proxy endpoints configured, **When** it requests the tool list, **Then** it receives tools from both proxies as if they were one server.
2. **Given** a catalog entry is added to the local proxy, **When** the AI tool next queries tools, **Then** the new tool appears without any change to the AI tool's configuration.
3. **Given** the local proxy is offline, **When** the AI tool queries the hosted proxy, **Then** only the hosted proxy's tools are returned — the local proxy's unavailability does not affect the hosted proxy.

---

### Edge Cases

- What happens when a stdio process takes more than a few seconds to start? The proxy should apply a configurable startup timeout and surface an error.
- How does the system handle concurrent requests to the same stdio server? Requests must be serialized per-server (stdio is single-channel) or the server must be multiplexed safely.
- What if a local proxy is started while another instance is already running on the same port? The new process should fail with a clear "address already in use" message, not silently corrupt state.
- What happens when a user transitions from local mode to hosted mode with existing stdio entries? stdio entries should remain stored but be flagged as "local only" — they will not be visible to users connecting via the hosted proxy.
- How does local mode handle the first-user bootstrap? The first user to reach the UI on a fresh local instance becomes admin automatically (same as hosted mode).

## Requirements *(mandatory)*

### Functional Requirements

**Local Mode**

- **FR-001**: The proxy MUST support a local deployment mode, activated by a startup flag or environment variable, that does not require a cloud database, external KMS, or OAuth2 app registration.
- **FR-002**: In local mode, the first user to create an account MUST automatically receive admin privileges (same bootstrap behavior as hosted mode).
- **FR-003**: In local mode, the admin UI MUST be accessible without setting a separate `ADMIN_SECRET` — the admin is authenticated as any logged-in admin user.
- **FR-004**: The running mode (local vs hosted) MUST be reflected in the UI so the user knows which mode they are in.

**stdio MCP Server Support**

- **FR-005**: Catalog entries MUST support a `transport` field with values `http` (default, existing behavior) and `stdio` (new).
- **FR-006**: For `stdio` catalog entries, admins MUST be able to specify: the executable command, command-line arguments, and optional environment variable overrides.
- **FR-007**: The proxy MUST launch stdio MCP servers as child processes on demand and communicate with them using the MCP stdio transport protocol.
- **FR-008**: The proxy MUST restart a crashed stdio server process on the next incoming request, up to a configurable retry limit.
- **FR-009**: stdio servers MUST only be available in local mode — the hosted proxy MUST reject catalog entries with `transport: stdio`.
- **FR-010**: When a stdio server fails to start (command not found, permission denied, non-zero exit on startup), the proxy MUST mark it unavailable and return a descriptive error to admin diagnostics.

**Auth Type Constraints**

- **FR-011**: In local mode, the catalog MUST reject entries with `auth_type: oauth2` (full OAuth2 app flow) and return a clear error message.
- **FR-012**: In local mode, `auth_type: pat` and `auth_type: api_key` MUST remain fully supported.
- **FR-013**: Hosted mode MUST continue to support all existing auth types without restriction.

**Dual-Proxy Model**

- **FR-014**: An AI tool MUST be able to configure multiple MCP proxy endpoints and receive a merged, deduplicated tool list.
- **FR-015**: Tool name conflicts between proxies MUST be resolved by prefixing with a proxy-specific namespace (consistent with existing `serverType__toolName` convention).
- **FR-016**: The setup guide on the dashboard MUST be updated to show how to add both a local and a hosted proxy endpoint to each supported AI tool.

### Key Entities

- **Deployment Mode**: A runtime attribute of the proxy instance — either `local` or `hosted`. Determines which catalog entry types and auth types are permitted.
- **Catalog Entry (extended)**: Gains a `transport` field (`http` | `stdio`) and, for stdio entries, `command`, `args`, and `env` fields.
- **stdio Server Process**: A managed child process corresponding to a stdio catalog entry. Has lifecycle state (starting, running, crashed, unavailable).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer with no cloud accounts can download the binary, run it, and have a functioning personal MCP proxy with at least one tool available — in under 5 minutes.
- **SC-002**: A stdio MCP server registered in the catalog responds to tool invocations within 2 seconds of process startup (excluding tool execution time).
- **SC-003**: When the local proxy is offline, the hosted proxy continues to serve requests with no degradation.
- **SC-004**: 100% of catalog entry types that are invalid for the current mode (stdio on hosted, oauth2 on local) are rejected at write time with a human-readable error.
- **SC-005**: An AI tool configured with two proxy endpoints lists tools from both without requiring any tool-side changes beyond adding the second URL.

## Assumptions

- stdio MCP servers follow the standard MCP stdio transport (newline-delimited JSON over stdin/stdout).
- The local proxy uses a local SQLite database (or equivalent embedded store) rather than Postgres — eliminating the Docker/Postgres dependency for local mode.
- The local KMS key (already implemented as `LOCAL_KMS_KEY`) is sufficient for local mode encryption; no external KMS is needed.
- stdio support is scoped to local mode only in this iteration; remote stdio bridging (via SSH or tunnel) is out of scope.
- The hosted proxy need not be aware of local proxy instances; coordination between a local and hosted proxy is the AI tool's responsibility.
- Users running local mode are assumed to be on a trusted personal machine; network-level auth (firewall, VPN) is outside scope.
