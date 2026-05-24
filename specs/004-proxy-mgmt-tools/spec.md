# Feature Specification: Proxy Management MCP Tools

**Feature Branch**: `007-proxy-mgmt-tools`

**Created**: 2026-05-24

**Status**: Draft

**Input**: User description: "Add MCP management tools to the proxy's own MCP server endpoint, so that Claude Code (or any MCP client) can list, add, and remove upstream MCP servers through the proxy itself — without needing to use the web admin UI."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Discover and Connect an Upstream (Priority: P1)

A user connected to the proxy via their MCP client wants to connect a new upstream MCP server. They ask the proxy (via MCP tool) to show them what servers are available in the catalog, choose one, provide their API key, and the proxy registers the connection so it becomes available in their next session.

**Why this priority**: This is the primary workflow — without being able to add upstreams via MCP, the web UI remains the only option. Delivering this alone provides immediate value.

**Independent Test**: Can be fully tested by calling `proxy_list_catalog` (sees available servers), then `proxy_connect_upstream` with a catalog ID and API key (returns the new upstream ID), then verifying the connection is available on next list.

**Acceptance Scenarios**:

1. **Given** a user is connected to the proxy via their MCP token, **When** they call `proxy_list_catalog`, **Then** they receive a list of all available server entries including id, display name, description, auth type, and whether OAuth is required.
2. **Given** a catalog entry exists with `auth_type: api_key` or `pat`, **When** the user calls `proxy_connect_upstream` with the catalog entry id and their API key, **Then** the upstream is saved, encrypted at rest, and the tool returns the new upstream's id.
3. **Given** a catalog entry requires OAuth2, **When** the user calls `proxy_connect_upstream` with that catalog id, **Then** the tool returns an error message explaining that OAuth2 connections must be completed via the web dashboard, and includes the dashboard URL.

---

### User Story 2 - List and Remove Connected Upstreams (Priority: P2)

A user wants to see which upstream MCP servers they currently have connected and remove one that is no longer needed — all without leaving their MCP client.

**Why this priority**: Visibility and cleanup are essential operational tasks; they complement the connect flow and close the management loop within the MCP interface.

**Independent Test**: Can be fully tested by calling `proxy_list_upstreams` (returns connected upstreams with status), then `proxy_disconnect_upstream` with an upstream id (removes it), then verifying it no longer appears in the list.

**Acceptance Scenarios**:

1. **Given** a user has one or more connected upstreams, **When** they call `proxy_list_upstreams`, **Then** they receive a list including id, display name, server type, auth type, connection status, and detected transport — without any credential data.
2. **Given** a user calls `proxy_list_upstreams` with no connected upstreams, **Then** they receive an empty list (not an error).
3. **Given** a valid upstream id that belongs to the calling user, **When** they call `proxy_disconnect_upstream` with that id, **Then** the upstream is removed and the tool confirms success.
4. **Given** an upstream id that does not belong to the calling user (or does not exist), **When** they call `proxy_disconnect_upstream`, **Then** the tool returns an error and the upstream is not affected.

---

### User Story 3 - Update Credentials for a Connected Upstream (Priority: P3)

A user's API key for a connected upstream has been rotated. They want to update the stored credential without disconnecting and reconnecting.

**Why this priority**: Credential rotation is a common maintenance task; handling it in-place avoids losing the upstream's id and any associated state.

**Independent Test**: Can be fully tested by calling `proxy_update_credentials` with an upstream id and new API key, then verifying the upstream status returns to active.

**Acceptance Scenarios**:

1. **Given** an existing connected upstream with `auth_type: api_key` or `pat`, **When** the user calls `proxy_update_credentials` with a new API key, **Then** the stored credential is replaced (encrypted) and the upstream status is reset to active.
2. **Given** an upstream id that does not belong to the calling user, **When** `proxy_update_credentials` is called, **Then** the tool returns an error and no credential is changed.

---

### Edge Cases

- What happens when the catalog is empty? `proxy_list_catalog` returns an empty list.
- What if the user passes an empty or blank API key to `proxy_connect_upstream`? The tool returns a validation error without creating a record.
- What if the same catalog entry is connected twice by the same user? The second connect call should succeed and create a second record (duplicate connections are allowed — the user may have multiple API keys for the same service).
- What if `proxy_disconnect_upstream` is called on an upstream that is currently in use in this session? The upstream is removed from storage; existing in-memory session state continues until the next reconnect.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The proxy MUST expose the following MCP tools on its own endpoint: `proxy_list_catalog`, `proxy_list_upstreams`, `proxy_connect_upstream`, `proxy_disconnect_upstream`, `proxy_update_credentials`.
- **FR-002**: All management tools MUST operate in the scope of the authenticated user identified by the proxy token in the current MCP session — a user cannot read or modify another user's upstream configurations.
- **FR-003**: `proxy_list_catalog` MUST return all active catalog entries visible to the user, including: id, display name, description, server type, auth type, and a flag indicating whether OAuth is required.
- **FR-004**: `proxy_list_catalog` MUST NOT return OAuth client secrets or any other admin-only fields.
- **FR-005**: `proxy_list_upstreams` MUST return the calling user's connected upstreams including: id, display name (from catalog), server type, auth type, connection status, and detected transport. It MUST NOT return encrypted credentials or raw credential material.
- **FR-006**: `proxy_connect_upstream` MUST accept a catalog entry id and an API key string, validate both are non-empty, encrypt the credential using the existing KMS, and persist the upstream config.
- **FR-007**: `proxy_connect_upstream` MUST return a clear error (not a panic or internal error) when called with a catalog entry that requires OAuth2, directing the user to the web dashboard.
- **FR-008**: `proxy_disconnect_upstream` MUST verify that the specified upstream id belongs to the calling user before deleting it.
- **FR-009**: `proxy_update_credentials` MUST verify upstream ownership before updating, encrypt the new credential, and reset the upstream status to active.
- **FR-010**: Management tools MUST be registered alongside proxied upstream tools in the same MCP server instance — they are visible in the same tool list as all other aggregated tools.

### Key Entities

- **CatalogEntry**: Admin-defined description of an available MCP server — server type, URL, display name, description, auth type, OAuth config. Read-only from the user's perspective.
- **UpstreamConfig**: A user's active connection to a catalog entry — stores encrypted credentials, connection status, and detected transport. Created and deleted by the user via management tools.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can discover, connect, verify, and disconnect an upstream MCP server entirely within their MCP client in under 2 minutes, with no browser UI required (for api_key/PAT auth types).
- **SC-002**: Calling any management tool with an invalid or unauthorized input returns a descriptive error message; no tool call results in an unhandled panic or an opaque "internal error" response.
- **SC-003**: Credential material (API keys) is never returned in any tool response — `proxy_list_upstreams` and `proxy_update_credentials` responses contain zero raw credential fields.
- **SC-004**: All five management tools appear in the tool list returned by the proxy's MCP `list_tools` response alongside proxied upstream tools.

## Assumptions

- The calling user's identity is established by the proxy token already embedded in the MCP session URL — no separate authentication step is needed for the management tools.
- OAuth2-based upstream connections remain out of scope for this feature; the browser redirect flow cannot be replicated inside an MCP tool call. Users doing OAuth must use the web dashboard.
- The catalog is managed by the proxy admin (not the end user) — management tools provide read access to the catalog but no write access.
- Adding a "connect via stdio command" upstream is out of scope for this feature; stdio upstreams are pre-configured by the admin in the catalog.
- Display names for connected upstreams are resolved by joining against the catalog at list time; the UpstreamConfig itself does not store a display name.
