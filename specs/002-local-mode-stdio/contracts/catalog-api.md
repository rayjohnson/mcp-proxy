# Contract: Admin Catalog API (Extended)

These endpoints extend the existing admin catalog API with new fields for transport type and mode constraints.

---

## GET /api/admin/catalog

Returns all active catalog entries. Unchanged endpoint — response gains new fields.

**Response** `200 OK`:

```json
[
  {
    "id": "abc123",
    "server_type": "github",
    "server_url": "https://api.githubcopilot.com/mcp/",
    "display_name": "GitHub",
    "description": "GitHub Copilot MCP server",
    "added_by": "admin@example.com",
    "active": true,
    "auth_type": "pat",
    "transport": "http",
    "command": null,
    "args": [],
    "env": {}
  },
  {
    "id": "def456",
    "server_type": "filesystem",
    "server_url": "",
    "display_name": "Local Filesystem",
    "description": "Read-only access to ~/docs",
    "added_by": "user@example.com",
    "active": true,
    "auth_type": "none",
    "transport": "stdio",
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-filesystem", "/home/user/docs"],
    "env": {}
  }
]
```

**Notes**:
- `transport` is always present; defaults to `"http"` for existing entries.
- `command`, `args`, `env` are only meaningful when `transport = "stdio"`.
- `encrypted_oauth_secret` is never returned.

---

## POST /api/admin/catalog

Creates a new catalog entry. Extended request body with new optional fields.

**Request body**:

```json
{
  "server_type": "filesystem",
  "display_name": "Local Filesystem",
  "description": "Read-only access to ~/docs",
  "auth_type": "none",
  "transport": "stdio",
  "command": "npx",
  "args": ["-y", "@modelcontextprotocol/server-filesystem", "/home/user/docs"],
  "env": { "NODE_NO_WARNINGS": "1" },
  "server_url": "",
  "oauth_client_id": null,
  "oauth_client_secret": null
}
```

**Field rules**:

| Field | Required | Notes |
|-------|----------|-------|
| `server_type` | Yes | Unique identifier string |
| `display_name` | Yes | |
| `auth_type` | Yes | `"none"`, `"pat"`, `"api_key"`, `"oauth2"` |
| `transport` | No | Defaults to `"http"` |
| `command` | Conditional | Required when `transport = "stdio"` |
| `args` | No | Defaults to `[]` |
| `env` | No | Defaults to `{}` |
| `server_url` | Conditional | Required when `transport = "http"`; ignored for stdio |
| `oauth_client_id` | Conditional | Required when `auth_type = "oauth2"` |
| `oauth_client_secret` | Conditional | Required when `auth_type = "oauth2"` |

**Mode-dependent validation errors**:

*Local mode* — `auth_type = "oauth2"` is rejected:
```json
HTTP 400 Bad Request
{
  "error": "auth_type 'oauth2' requires hosted mode; use 'pat' or 'api_key' in local mode"
}
```

*Hosted mode* — `transport = "stdio"` is rejected:
```json
HTTP 400 Bad Request
{
  "error": "transport 'stdio' is only supported in local mode"
}
```

*stdio entry without command*:
```json
HTTP 400 Bad Request
{
  "error": "command is required when transport is 'stdio'"
}
```

**Success response** `201 Created`:
```json
{
  "id": "def456",
  "server_type": "filesystem",
  "display_name": "Local Filesystem",
  "active": true,
  "auth_type": "none",
  "transport": "stdio",
  "command": "npx",
  "args": ["-y", "@modelcontextprotocol/server-filesystem", "/home/user/docs"],
  "env": {}
}
```

---

## DELETE /api/admin/catalog/{id}

Deactivates a catalog entry. No change from existing behavior.

**Response** `204 No Content` on success.

---

## GET /api/admin/mode

New endpoint. Returns the current deployment mode so AI tools and UIs can discover constraints.

**Response** `200 OK`:

```json
{
  "mode": "local"
}
```

or

```json
{
  "mode": "hosted"
}
```

`mode` is either `"local"` or `"hosted"`.

---

## Config API Contract

`LOCAL_MODE` and `DATA_DIR` are startup environment variables, not runtime API fields. They cannot be changed without restarting the server.
