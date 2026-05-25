# API Contracts: Per-User Server Enable/Disable

## POST /api/upstreams/{id}/toggle

Flips the enabled state of an HTTP upstream config for the authenticated user.

### Authorization
Requires valid session cookie. Returns 401 if unauthenticated. Returns 403 if the upstream does not belong to the authenticated user.

### Path Parameters
| Parameter | Type | Description |
|-----------|------|-------------|
| id | string | Upstream config ID |

### Request Body
None.

### Response — 200 OK
Returns the updated upstream config with the new enabled state.

```json
{
  "id": "abc123",
  "server_type": "github",
  "display_name": "GitHub",
  "status": "active",
  "enabled": false
}
```

### Error Responses
| Status | Condition |
|--------|-----------|
| 401 | Not authenticated |
| 403 | Upstream belongs to a different user |
| 404 | Upstream ID not found |
| 500 | Database error |

---

## POST /api/catalog/{id}/toggle

Flips the per-user enabled state of a stdio catalog entry for the authenticated user.

### Authorization
Requires valid session cookie. Returns 401 if unauthenticated.

### Path Parameters
| Parameter | Type | Description |
|-----------|------|-------------|
| id | string | Catalog entry ID |

### Request Body
None.

### Response — 200 OK
Returns the new enabled state for this user.

```json
{
  "catalog_id": "xyz789",
  "enabled": false
}
```

### Error Responses
| Status | Condition |
|--------|-----------|
| 401 | Not authenticated |
| 404 | Catalog entry not found or not active |
| 500 | Database error |

---

## Dashboard Representation

Both toggle APIs affect the dashboard's visual state:

- **Enabled server**: shown normally with existing status badge
- **Disabled server**: name and status rendered with `.server-disabled` CSS class; toggle button shows "Enable" instead of "Disable"

The dashboard passes `Enabled bool` on each server row entry so the template can render the toggle button and disabled style without a separate API call.
