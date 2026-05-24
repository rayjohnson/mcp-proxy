# MCP Tool Contracts: Proxy Management Tools

All five tools are registered on the proxy's own MCP endpoint. They are scoped to the user identified by the proxy token in the session URL. No additional authentication is required.

---

## `proxy_list_catalog`

Lists all available upstream MCP servers the user could connect to.

**Input schema**: *(no parameters)*

**Output** (text, JSON array):
```json
[
  {
    "id": "<uuid>",
    "display_name": "GitHub",
    "description": "GitHub source control and CI tools",
    "server_type": "github",
    "auth_type": "api_key",
    "requires_oauth": false
  },
  {
    "id": "<uuid>",
    "display_name": "Google Cloud",
    "description": null,
    "server_type": "googlecloud",
    "auth_type": "oauth2",
    "requires_oauth": true
  }
]
```

**Error cases**: none expected (returns empty array if catalog is empty).

---

## `proxy_list_upstreams`

Lists the calling user's currently connected upstream MCP servers.

**Input schema**: *(no parameters)*

**Output** (text, JSON array):
```json
[
  {
    "id": "<uuid>",
    "display_name": "GitHub",
    "server_type": "github",
    "auth_type": "api_key",
    "status": "active",
    "detected_transport": "http"
  }
]
```

**Error cases**: returns empty array if no upstreams connected.

---

## `proxy_connect_upstream`

Connects the user to an upstream server using an API key or PAT.

**Input schema**:
```json
{
  "catalog_id": "<uuid>",
  "api_key": "<string>"
}
```

| Parameter | Required | Description |
|-----------|----------|-------------|
| catalog_id | yes | ID from `proxy_list_catalog` |
| api_key | yes | User's API key or personal access token for the service |

**Output on success** (text, JSON object):
```json
{
  "id": "<uuid>",
  "server_type": "github",
  "status": "active"
}
```

**Error cases**:
- `catalog_id` not found → `"Unknown catalog entry: <id>"`
- `api_key` empty or blank → `"api_key must not be empty"`
- `auth_type` is `"oauth2"` → `"This server requires OAuth2. Complete the connection at /dashboard"`
- `auth_type` is `"none"` or `"stdio"` → `"This server requires no credentials and is connected automatically"`
- Internal encrypt failure → `"Internal error: could not secure credentials"`

---

## `proxy_disconnect_upstream`

Removes a connected upstream for the calling user.

**Input schema**:
```json
{
  "upstream_id": "<uuid>"
}
```

| Parameter | Required | Description |
|-----------|----------|-------------|
| upstream_id | yes | ID from `proxy_list_upstreams` |

**Output on success** (text):
```
Disconnected successfully.
```

**Error cases**:
- `upstream_id` not found or belongs to a different user → `"Upstream not found"`
- Internal delete failure → `"Internal error: could not remove upstream"`

---

## `proxy_update_credentials`

Replaces the stored API key for an existing upstream connection.

**Input schema**:
```json
{
  "upstream_id": "<uuid>",
  "api_key": "<string>"
}
```

| Parameter | Required | Description |
|-----------|----------|-------------|
| upstream_id | yes | ID from `proxy_list_upstreams` |
| api_key | yes | New API key or PAT |

**Output on success** (text):
```
Credentials updated. Status reset to active.
```

**Error cases**:
- `upstream_id` not found or belongs to a different user → `"Upstream not found"`
- `api_key` empty or blank → `"api_key must not be empty"`
- Internal encrypt failure → `"Internal error: could not secure credentials"`
