# API Contracts: AI Tool Auto-Configuration

All endpoints require an authenticated session (existing cookie/session auth). These endpoints are only available when the proxy runs in local mode (`LOCAL_MODE=true`).

---

## GET /api/tools

Returns the list of supported AI tools and their current installation/configuration status.

**Request**: No body. No query parameters.

**Response 200**:

```json
[
  {
    "id": "claude-desktop",
    "display_name": "Claude Desktop",
    "status": "unconfigured"
  },
  {
    "id": "gemini-cli",
    "display_name": "Gemini CLI",
    "status": "configured"
  }
]
```

**Response body fields**:

| Field | Type | Values |
|-------|------|--------|
| `id` | string | Stable tool identifier |
| `display_name` | string | Human-readable name |
| `status` | string | `not_installed` \| `unconfigured` \| `configured` \| `error` |
| `error_message` | string (optional) | Only present when `status` is `"error"` |

**Response 503** (non-local mode):

```json
{"error": "auto-configuration is only available in local mode"}
```

---

## POST /api/tools/{id}/configure

Writes the proxy's MCP endpoint into the specified tool's config. Idempotent — safe to call multiple times.

**Path parameter**: `id` — tool identifier matching one of the IDs returned by `GET /api/tools`.

**Request**: No body required. The proxy URL is derived from the request's `Host` header.

**Response 200** (success):

```json
{
  "id": "claude-desktop",
  "display_name": "Claude Desktop",
  "status": "configured"
}
```

**Response 400** (unknown tool ID):

```json
{"error": "unknown tool: <id>"}
```

**Response 404** (tool not installed):

```json
{"error": "claude-desktop is not installed"}
```

**Response 500** (write failure — original config unchanged):

```json
{"error": "failed to write config: <human-readable reason>"}
```

**Response 503** (non-local mode):

```json
{"error": "auto-configuration is only available in local mode"}
```

---

## Dashboard Integration

The dashboard page (`GET /dashboard`) renders the AI Tools section via the `ai-tools.html` partial. The partial calls `GET /api/tools` on load and renders a row per tool. The Configure button calls `POST /api/tools/{id}/configure` and updates the row status in place without a full page reload.

Tools with `status: "not_installed"` render a greyed-out row with no Configure button.
