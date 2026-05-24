# Feature Specification: Core MCP Proxy MVP

**Feature Branch**: `001-core-proxy-mvp`

**Created**: 2026-05-23

**Status**: Implemented

**Input**: User description: "basic requirements for the cloud MCP proxy"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Developer Connects AI Tool via Single Proxy Endpoint (Priority: P1)

A developer who currently manages 10–15 separate MCP server configurations across
multiple AI tools signs up for MCP Proxy. They add their upstream MCP servers
(e.g., GitHub, Notion, Linear) and their corresponding credentials once. The system
gives them a single proxy endpoint URL. They paste that URL into their AI tool
(Claude, Gemini, Cursor, etc.) and immediately have access to all their configured
tools — with no additional installation or per-tool setup.

**Why this priority**: This is the entire product. Without this story, there is
nothing else to build.

**Independent Test**: A tester can sign up, configure two upstream servers, receive
an endpoint URL, connect an MCP-compatible AI tool to that URL, and confirm that
tools from both upstream servers appear and are callable — all without any other
story being implemented.

**Acceptance Scenarios**:

1. **Given** a developer has signed up and configured at least one upstream server,
   **When** they connect an MCP-compatible AI tool to their proxy endpoint URL,
   **Then** the AI tool can list and call all tools from all configured upstream
   servers through that single URL.

2. **Given** a developer has configured GitHub and Notion as upstream servers,
   **When** their AI tool requests the list of available tools,
   **Then** the proxy returns the combined tool list from both GitHub and Notion
   with no tools omitted.

3. **Given** a developer's AI tool calls a Notion tool through the proxy,
   **When** the call is made,
   **Then** the result is identical to calling the Notion MCP server directly,
   and the AI tool receives no indication that a proxy was involved.

---

### User Story 2 - Admin Manages Default Server Catalog (Priority: P2)

An administrator configures a default set of upstream MCP servers that the
organization wants available to all developers. New developers who sign up
automatically see these servers pre-listed in their dashboard and can begin
configuring credentials immediately. When the admin adds a new server to the
default catalog, existing developers receive a suggestion in their dashboard to
add it — they can accept (and provide their credentials) or dismiss it. Developers
retain full control: they can remove any default server or add servers that are not
in the default catalog.

**Why this priority**: Without a default catalog, every new developer faces a blank
slate and must know what servers exist. The admin catalog is what transforms the
proxy from a personal tool into an organizational one.

**Independent Test**: An admin can add a server to the default catalog, a new user
signs up and sees it pre-listed, and an existing user sees a suggestion notification
in their dashboard — all independently verifiable.

**Acceptance Scenarios**:

1. **Given** the admin has configured GitHub and Notion as default servers,
   **When** a new developer signs up,
   **Then** GitHub and Notion appear in their dashboard as available servers to
   configure, with no action required beyond adding their own credentials.

2. **Given** a developer has an active account and the admin adds Linear to the
   default catalog,
   **When** the developer next views their dashboard,
   **Then** they see a suggestion to add Linear, which they can accept or dismiss.

3. **Given** a developer dismissed the suggestion to add Linear,
   **When** they view their dashboard subsequently,
   **Then** the Linear suggestion does not reappear.

4. **Given** a developer has removed GitHub from their personal configuration,
   **When** the admin makes no change to GitHub in the default catalog,
   **Then** GitHub does not reappear in the developer's configuration.

---

### User Story 3 - Developer Customizes Their Server Configuration (Priority: P3)

A developer wants to add a new upstream MCP server beyond those in the default
catalog, update credentials for an existing one, or remove a server they no longer
need — without disrupting other connected AI tools.

**Why this priority**: Developers have individual needs that differ from the
organizational defaults. Configuration management is what makes the proxy
personal and useful long-term.

**Independent Test**: A tester with an existing proxy configuration can add a
non-default upstream server and verify its tools appear in the proxy, update
credentials for an existing server and verify connectivity is restored, and remove
a server and verify its tools are no longer listed.

**Acceptance Scenarios**:

1. **Given** a developer has an existing proxy with GitHub configured,
   **When** they add a non-default server with valid credentials,
   **Then** that server's tools appear in the proxy without any existing server
   being affected or any AI tool disconnection occurring.

2. **Given** a developer has an upstream server with expired credentials,
   **When** they update the credentials,
   **Then** the upstream server becomes reachable again within one minute.

3. **Given** a developer removes an upstream server from their configuration,
   **When** their AI tool next lists available tools,
   **Then** the removed server's tools no longer appear, and all remaining
   servers' tools continue to work.

---

### User Story 4 - Upstream Server Failure Is Isolated (Priority: P4)

When one configured upstream MCP server becomes temporarily unavailable or returns
errors, the developer's AI tool continues to work normally with all other configured
servers. The failure is surfaced clearly rather than silently corrupting results.

**Why this priority**: Reliability of the working servers is more important than
perfect availability of any single failing one. A cascading failure would make the
proxy worse than the direct-connection status quo.

**Independent Test**: A tester can simulate one upstream server being unreachable
and verify that the proxy continues to serve tools from all other servers while
reporting the unavailable server's status clearly.

**Acceptance Scenarios**:

1. **Given** a developer has GitHub and Notion configured and Notion becomes
   unavailable, **When** their AI tool lists available tools,
   **Then** GitHub tools are returned normally and Notion is reported as temporarily
   unavailable rather than the entire request failing.

2. **Given** an upstream server returns an error for a specific tool call,
   **When** the AI tool receives the response,
   **Then** the error is scoped to that tool and other tools remain callable.

---

### Edge Cases

- What happens when two upstream servers expose a tool with the same name? The proxy
  automatically prefixes every tool name with the server identifier using double
  underscores (e.g., `github__search`, `notion__search`), ensuring all tools are
  always visible and unambiguous.
- What happens when the admin removes a server from the default catalog that some
  developers have already configured with their own credentials? Their personal
  configuration is unaffected; only the default suggestion is removed.
- What if a developer accepts a default server suggestion but provides invalid
  credentials?
- What happens when a developer's proxy management session expires while a connected
  AI tool is mid-session with the proxy endpoint?
- What if a developer provides a URL that is not a valid MCP server during upstream
  configuration? The proxy should validate connectivity and MCP compliance before
  saving the configuration.
- What if an OAuth2 refresh token expires while an AI tool is actively mid-session?
  The proxy should gracefully mark that server's tools as unavailable with a
  re-authorization error rather than hanging or returning corrupt results.
- What if a developer revokes the proxy's access from the upstream service's own
  settings (e.g., disconnecting the app in Notion's settings)? The proxy detects the
  next failed refresh and surfaces the re-authorization required status.
- What if the OAuth2 authorization callback is intercepted or the state parameter
  is tampered with during the browser flow?

## Requirements *(mandatory)*

### Functional Requirements

**Authentication**

- **FR-001**: A developer MUST be able to create an account and sign in using an
  email address and password or a standard OAuth2 identity provider (e.g.,
  Google, GitHub).
- **FR-002**: The authentication layer MUST be designed to support pluggable identity
  providers so that JumpCloud SSO can be added as a sign-in option without
  requiring changes to the rest of the application.
- **FR-003**: When JumpCloud SSO is not yet configured, the system MUST fall back
  gracefully to email/password authentication; JumpCloud SSO is not required for
  the initial release.

**Proxy Core**

- **FR-004**: The system MUST generate a unique, stable proxy endpoint URL for each
  developer account upon registration.
- **FR-005**: An MCP-compatible AI tool MUST be able to connect to the proxy endpoint
  using the standard MCP protocol with no custom client modifications.
- **FR-006**: The proxy MUST present a unified tool list to connected AI tools,
  aggregating all tools from all configured and reachable upstream servers.
- **FR-007**: The proxy MUST route each incoming tool call to the correct upstream
  server and return the upstream server's response unmodified to the AI tool.
- **FR-008**: The proxy MUST prefix every tool name with the upstream server
  identifier and a double-underscore separator (e.g., `github__search`,
  `notion__search`), ensuring all tools are uniquely named regardless of conflicts.

**Upstream Server Management**

- **FR-009**: The system MUST support the following upstream server types at launch:
  GitHub, Notion, Linear, Cloudflare, Google Cloud.
- **FR-010**: A developer MUST be able to connect an API-key-based upstream server by
  entering a static key or token in the management UI; the proxy stores it encrypted
  and the developer never sees it again after submission.
- **FR-011**: A developer MUST be able to connect an OAuth2-based upstream server by
  completing a browser-based authorization flow initiated from the proxy management
  UI. The developer MUST NOT be required to manually copy or paste tokens; the proxy
  handles the full redirect, callback, and token exchange automatically.
- **FR-012**: The proxy MUST automatically refresh OAuth2 access tokens using the
  stored refresh token before they expire, transparently and without any developer
  action.
- **FR-013**: When an OAuth2 refresh token is revoked or has permanently expired, the
  proxy MUST surface a "re-authorization required" status for that server and prompt
  the developer to repeat the browser authorization flow. The proxy MUST NOT silently
  fail or expose the revoked token.
- **FR-014**: A developer MUST be able to view the current status of each configured
  upstream server. Statuses MUST distinguish between: reachable, unreachable
  (network/server error), credential error (bad API key), and re-authorization
  required (OAuth2 token revoked or expired).
- **FR-015**: A developer MUST be able to remove any upstream server from their
  personal configuration at any time, including servers from the default catalog.
  Removal MUST revoke any stored OAuth2 tokens for that server.
- **FR-016**: Developer credentials for upstream servers — whether API keys or OAuth2
  token pairs — MUST be stored encrypted and MUST NOT appear in logs, error messages,
  or any user-visible interface after initial entry.

**Admin Default Catalog**

- **FR-017**: An administrator role MUST exist with the ability to manage the default
  server catalog without affecting individual developer configurations or credentials.
- **FR-018**: An administrator MUST be able to add an upstream MCP server to the
  default catalog, specifying the server type, URL, and authentication type
  (`api_key` or `oauth2`). For OAuth2 servers, the admin MAY optionally pre-configure
  the organization's OAuth2 app client ID and client secret so that developers only
  need to authorize access — not register their own OAuth2 application.
- **FR-019**: When a new developer signs up, all servers currently in the default
  catalog MUST be pre-listed in their dashboard, ready for credential configuration
  or OAuth2 authorization.
- **FR-020**: When an administrator adds a server to the default catalog, all existing
  developer accounts MUST receive a suggestion in their management dashboard to add
  that server.
- **FR-021**: A developer MUST be able to dismiss a default catalog suggestion; once
  dismissed, it MUST NOT reappear for that developer.
- **FR-022**: An administrator MUST be able to remove a server from the default
  catalog; this MUST NOT affect developers who have already configured that server.
- **FR-023**: An administrator MUST be able to promote any developer account to admin
  and demote any admin account to developer. An administrator MUST NOT be able to
  remove their own admin role.
- **FR-024**: The first user to register on a fresh deployment is automatically
  assigned the admin role. Subsequent users are assigned the developer role by default.
- **FR-025**: The management UI MUST provide inline setup instructions for connecting
  the proxy endpoint to common AI tools, including at minimum Claude Code, Claude
  Desktop, Cursor, and VS Code. Instructions MUST be pre-filled with the developer's
  actual proxy endpoint URL.
- **FR-026**: The proxy management interface MUST expose a JSON API for admin catalog
  operations (`GET`, `POST`, `DELETE` on `/api/admin/catalog`) to allow programmatic
  catalog management (e.g., by an AI assistant).

### Key Entities

- **Developer Account**: A registered user; owns one proxy endpoint, zero or more
  personal upstream server configurations, and a set of dismissed catalog suggestions.
- **Administrator Account**: A privileged user who manages the default server catalog;
  cannot access developer credentials or proxy endpoint traffic.
- **Proxy Endpoint**: The unique URL assigned to a developer account; all AI tool
  connections are made through this URL.
- **Upstream Server Configuration**: Links a developer account to a cloud MCP server.
  Holds the server URL, type, current status, and one of two credential types:
  an encrypted static API key, or an encrypted OAuth2 token pair (access token +
  refresh token) obtained via browser authorization flow.
- **Default Server Catalog**: The admin-maintained list of upstream servers
  pre-suggested to all developers; contains server type, URL, and auth type. For
  OAuth2 servers, optionally holds the organization's OAuth2 app credentials
  (encrypted) so developers do not need to register their own OAuth2 applications.
- **Catalog Suggestion**: A notification to an existing developer that a new default
  server is available; can be accepted (leading to credential/OAuth2 setup) or
  permanently dismissed.
- **Tool**: An MCP-defined capability exposed by an upstream server and proxied
  transparently to AI tools.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer can complete the full onboarding flow — from account
  creation to a working proxy endpoint usable in an AI tool — in under 10 minutes.
- **SC-002**: 100% of tool calls made through the proxy return results equivalent to
  calling the upstream server directly, with no behavioral differences observable
  by the AI tool.
- **SC-003**: When one upstream server is unavailable, 100% of tool calls to all
  other configured upstream servers continue to succeed.
- **SC-004**: Changes to upstream server configuration (add, update credentials,
  remove) are reflected in the proxy within 60 seconds.
- **SC-005**: The proxy endpoint URL is stable; developers never need to update their
  AI tool configuration after adding or removing upstream servers.
- **SC-006**: When an admin adds a server to the default catalog, all existing
  developers see the suggestion in their dashboard within 5 minutes.

## Assumptions

- Upstream MCP servers are cloud-hosted and reachable over HTTPS; locally-running
  MCP servers are out of scope.
- Each developer account has exactly one proxy endpoint in the MVP; multi-endpoint
  or team/organization sharing is deferred.
- The proxy management interface is a web application with a supplementary JSON API
  for admin catalog operations to enable programmatic and AI-assisted management.
- Billing, usage limits, and rate limiting are out of scope for this initial spec.
- Upstream servers use one of two auth models: static API key or OAuth2 authorization
  code flow with refresh tokens. The proxy handles both; developers never manually
  manage tokens for OAuth2 servers.
- OAuth2 access token refresh is handled automatically and silently by the proxy.
  If refresh fails, the developer is notified via a dashboard status change.
- The proxy registers as an OAuth2 application with each supported upstream service
  (Notion, Google, etc.) at the organizational level; per-user OAuth2 flows run
  through those registered applications.
- AI tools connect to the proxy endpoint using MCP over HTTP/SSE (the standard
  transport for cloud-hosted MCP servers).
- The MCP protocol version targeted is the current stable version at implementation
  time; backward compatibility with older client versions is not required for MVP.
- JumpCloud SSO is the planned long-term identity provider but is not available at
  initial launch. The auth system must support adding JumpCloud as a provider later
  without rearchitecting sign-in or session handling.
- The default catalog specifies which servers are available and their URLs; it does
  not include shared credentials. Each developer provides their own credentials.
- Deployment platform is not yet finalized. Leading candidates are Google Cloud Run
  (favoring Go) and Cloudflare Workers (favoring TypeScript). The final decision
  will be made during the planning phase.
