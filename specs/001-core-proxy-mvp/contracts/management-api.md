# Contract: Management API

The management API is consumed by the server-rendered management web UI. All
endpoints require authentication unless marked public. Responses are JSON.

Base path: `/api`

---

## Authentication

### POST /api/auth/register *(public)*

Create a new developer account.

**Request body**:
```json
{ "email": "dev@example.com", "password": "..." }
```

**Response** (201 Created):
```json
{ "proxy_endpoint": "https://<host>/mcp/<proxy_token>" }
```
Sets `Set-Cookie: session=<jwt>; HttpOnly; Secure; SameSite=Lax`

**Errors**: `409 Conflict` (email taken), `422 Unprocessable` (validation)

---

### POST /api/auth/login *(public)*

Sign in with email and password.

**Request body**:
```json
{ "email": "dev@example.com", "password": "..." }
```

**Response** (200 OK): Sets session cookie. Body: `{ "role": "developer" }`

**Errors**: `401 Unauthorized` (invalid credentials)

---

### POST /api/auth/logout

Invalidates the session (clears cookie).

**Response**: `204 No Content`

---

## Proxy Endpoint

### GET /api/proxy/endpoint

Returns the current user's MCP proxy endpoint URL.

**Response** (200 OK):
```json
{ "endpoint_url": "https://<host>/mcp/<proxy_token>" }
```

---

## Upstream Server Management

### GET /api/upstream

List the current user's configured upstream servers.

**Response** (200 OK):
```json
{
  "servers": [
    {
      "id": "uuid",
      "server_type": "github",
      "display_name": "GitHub",
      "server_url": "https://api.githubcopilot.com/mcp/",
      "auth_type": "oauth2",
      "status": "reachable",
      "status_checked_at": "2026-05-23T10:00:00Z"
    }
  ]
}
```

---

### POST /api/upstream

Add an upstream server with an API key.

**Request body**:
```json
{
  "server_type": "cloudflare",
  "server_url": "https://mcp.cloudflare.com/sse",
  "auth_type": "api_key",
  "api_key": "<key>"
}
```

**Response** (201 Created): Server object (without credentials).

**Errors**: `409 Conflict` (server type already configured), `422 Unprocessable`

---

### DELETE /api/upstream/{id}

Remove an upstream server and revoke any stored credentials.

**Response**: `204 No Content`

**Errors**: `404 Not Found`

---

### PATCH /api/upstream/{id}/credentials

Update the API key for an existing API-key-based server.

**Request body**:
```json
{ "api_key": "<new key>" }
```

**Response** (200 OK): Updated server object.

---

### GET /api/upstream/{id}/status

Trigger an on-demand connectivity check for one upstream server.

**Response** (200 OK):
```json
{ "status": "reachable", "checked_at": "2026-05-23T10:01:00Z" }
```

---

## OAuth2 Upstream Authorization

### GET /api/oauth2/authorize/{server_type}

Initiates an OAuth2 browser authorization flow for the given upstream server type.
Redirects the browser to the upstream service's authorization URL.

**Path param**: `server_type` — e.g., `notion`, `github`, `google_cloud`

**Response**: `302 Redirect` to upstream OAuth2 authorization URL

Sets a short-lived `oauth2_state` cookie for CSRF validation.

---

### GET /api/oauth2/callback/{server_type} *(public — called by upstream service)*

Receives the authorization code after the developer approves access.
Exchanges the code for tokens, encrypts and stores them, then redirects to
the management dashboard.

**Query params**: `code`, `state`

**Response**: `302 Redirect` to `/dashboard?connected={server_type}`

**Errors**:
- `400 Bad Request` — missing or invalid state (CSRF)
- `502 Bad Gateway` — upstream token exchange failed

---

## Catalog Suggestions

### GET /api/suggestions

List pending catalog suggestions for the current user.

**Response** (200 OK):
```json
{
  "suggestions": [
    {
      "id": "uuid",
      "server_type": "linear",
      "display_name": "Linear",
      "description": "Issue tracking",
      "server_url": "https://mcp.linear.app/sse"
    }
  ]
}
```

---

### POST /api/suggestions/{id}/dismiss

Permanently dismiss a catalog suggestion.

**Response**: `204 No Content`

---

### POST /api/suggestions/{id}/accept

Accept a suggestion — redirects to the appropriate add-server or OAuth2 flow.

**Response** (200 OK):
```json
{
  "next_action": "oauth2_authorize",
  "authorize_url": "/api/oauth2/authorize/linear"
}
```
or
```json
{
  "next_action": "enter_api_key",
  "server_type": "cloudflare"
}
```

---

## Admin: Default Catalog

All `/api/admin/*` endpoints require `role = admin`.

### GET /api/admin/catalog

List all default catalog entries (active and inactive).

**Response** (200 OK):
```json
{
  "entries": [
    {
      "id": "uuid",
      "server_type": "github",
      "display_name": "GitHub",
      "server_url": "https://api.githubcopilot.com/mcp/",
      "active": true,
      "created_at": "2026-05-23T00:00:00Z"
    }
  ]
}
```

---

### POST /api/admin/catalog

Add a new server to the default catalog. Creates `pending` suggestions for all
existing developer accounts.

**Request body**:
```json
{
  "server_type": "linear",
  "server_url": "https://mcp.linear.app/sse",
  "display_name": "Linear",
  "description": "Issue tracking and project management"
}
```

**Response** (201 Created): Catalog entry object.

**Errors**: `409 Conflict` (server type already in active catalog)

---

### DELETE /api/admin/catalog/{id}

Soft-delete a catalog entry (sets `active = false`). Does not affect existing
developer configurations.

**Response**: `204 No Content`
