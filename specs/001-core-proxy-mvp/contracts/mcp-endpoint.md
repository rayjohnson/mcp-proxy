# Contract: MCP Proxy Endpoint

The MCP proxy endpoint is what AI tools (Claude, Gemini, Cursor, etc.) connect to.
It implements the MCP Streamable HTTP transport (spec version 2025-06-18).

---

## Endpoint

```
/mcp/{proxy_token}
```

`proxy_token` is the stable, opaque token from `users.proxy_token`. It identifies
the developer's account and determines which upstream servers to aggregate.

---

## GET /mcp/{proxy_token}

Opens a server-sent event stream for server-to-client notifications.

**Request headers**:
- `Accept: text/event-stream` (required)
- `Mcp-Session-Id: <session-id>` (optional; omit on first connect)

**Response** (200 OK):
- `Content-Type: text/event-stream`
- `Mcp-Session-Id: <session-id>` (assigned by server on first connect)
- Body: SSE stream of JSON-RPC notification events

**Error responses**:
- `401 Unauthorized` — invalid or unknown `proxy_token`
- `503 Service Unavailable` — all upstream servers unreachable

---

## POST /mcp/{proxy_token}

Sends a JSON-RPC message from the AI tool to the proxy.

**Request headers**:
- `Content-Type: application/json`
- `Mcp-Session-Id: <session-id>` (required after initial handshake)

**Request body**: JSON-RPC 2.0 message (request, notification, or batch)

**Response** (200 OK):
- `Content-Type: application/json` — for single-response messages
- `Content-Type: text/event-stream` — for streaming responses (tool calls with
  streamed output)

**Error responses**:
- `401 Unauthorized` — invalid `proxy_token`
- `400 Bad Request` — malformed JSON-RPC

---

## Proxy Behavior

### Tool list aggregation (`tools/list`)

When the AI tool calls `tools/list`, the proxy:
1. Connects to all of the user's `reachable` upstream servers concurrently.
2. Collects each server's tool list.
3. Prefixes every tool name: `{server_type}__{original_name}`
   (e.g., `github__create_issue`, `notion__search_pages`).
4. Returns the combined list as a single `tools/list` response.

Upstream servers in `unreachable`, `credential_error`, or `reauth_required` status
are excluded from the tool list. Their absence is surfaced in the management UI, not
in the MCP protocol response.

### Tool call routing (`tools/call`)

When the AI tool calls a tool:
1. The proxy parses the `{server_type}__` prefix from the tool name.
2. Routes the call to the matching upstream server, stripping the prefix before
   forwarding.
3. Returns the upstream response unmodified.

If the upstream server is unavailable at call time, the proxy returns a JSON-RPC
error with a descriptive message scoped to that tool.

### Session lifecycle

- A session is established on the first POST request and identified by
  `Mcp-Session-Id`.
- The proxy maintains upstream MCP client connections for the duration of the session.
- Sessions expire after 30 minutes of inactivity.
- The AI tool may send `DELETE /mcp/{proxy_token}` with `Mcp-Session-Id` to
  explicitly terminate a session.

---

## DELETE /mcp/{proxy_token}

Terminates an active session and closes all upstream connections.

**Request headers**:
- `Mcp-Session-Id: <session-id>` (required)

**Response**: `200 OK` with empty body.
