# Research: Proxy Management MCP Tools

## Decision 1: Where management tools are registered

**Decision**: Registered in `buildMCPServer` alongside proxied upstream tools, in a new file `internal/mcp/mgmt.go`.

**Rationale**: The MCP SDK's `server.AddTool` call is the only integration point. Co-location with the existing tool registration loop is the simplest approach and avoids a separate server instance or router.

**Alternatives considered**: A separate MCP endpoint for management — rejected because it requires the client to configure two endpoints and breaks the "single proxy endpoint" value proposition.

---

## Decision 2: User identity inside tool handlers

**Decision**: Add `UserID string` field to `ProxySession`. It is set once in `OpenSession` from the `userID` argument already passed there.

**Rationale**: Tool handlers need the user's ID to scope store queries. `ProxySession` is the per-session context passed to every tool handler; storing it there avoids threading it through call arguments.

**Alternatives considered**: Re-extracting from the HTTP request context inside the tool handler — rejected because MCP tool callbacks receive only `context.Context` and `*CallToolRequest`, not the originating HTTP request.

---

## Decision 3: KMS encrypt access for `proxy_connect_upstream`

**Decision**: Add `KMSEncrypt func(ctx, []byte) ([]byte, error)` to a new `ManagementDeps` struct inside `ProxyServerDeps`. The existing `SessionDeps.KMSDecrypt` is a decrypt-only func; encrypt is a separate operation on the `*kms.Client`.

**Rationale**: The KMS client already exists and is injected at the HTTP handler layer. Passing the encrypt func (not the full `*kms.Client`) keeps the dependency minimal and consistent with how decrypt is already passed in `SessionDeps`.

**Alternatives considered**: Passing the full `*kms.Client` — rejected per Simplicity First; the tool only needs one method.

---

## Decision 4: Display names in `proxy_list_upstreams`

**Decision**: Load all active catalog entries at list time, build a `map[serverType]displayName`, and use it to annotate each upstream config in the response.

**Rationale**: `UpstreamConfig` does not store a display name — it stores `ServerType`, which is the join key to the catalog. The catalog list is small (O(10) entries) so a full scan on each list call is trivially cheap.

**Alternatives considered**: Denormalizing display name into `UpstreamConfig` — rejected; adds a schema change and creates a sync problem if the catalog entry's display name is updated.

---

## Decision 5: OAuth2 connect path

**Decision**: `proxy_connect_upstream` returns a descriptive error (not a panic) when called with a catalog entry whose `AuthType` is `"oauth2"`. The error message includes the dashboard URL path `/dashboard`.

**Rationale**: OAuth2 requires a browser redirect; there is no way to complete the flow inside an MCP tool call. The error must be informative enough that the user knows what to do next.

**Alternatives considered**: Silently ignoring OAuth entries in `proxy_list_catalog` — rejected; hiding available servers is more confusing than explaining the limitation at connect time.

---

## Decision 6: Duplicate upstream connections

**Decision**: Allowed. `proxy_connect_upstream` always creates a new `UpstreamConfig` row without checking for duplicates with the same `ServerType` for the same user.

**Rationale**: This mirrors the existing behavior of the web UI's Connect handler. Users may have multiple API keys for the same service, or may wish to connect multiple accounts.

**Alternatives considered**: Reject duplicates — would require a new store query and introduces a behavior difference between the tool and the web UI with no clear benefit.
