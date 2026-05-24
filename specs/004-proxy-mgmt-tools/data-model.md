# Data Model: Proxy Management MCP Tools

No new database schema is introduced by this feature. All tool handlers operate on existing tables via existing store interfaces.

## Entities Used

### CatalogEntry (read-only from tool perspective)

Source: `store.CatalogEntry` / `default_catalog` table.

Fields exposed by `proxy_list_catalog`:

| Field | Type | Notes |
|-------|------|-------|
| id | string (UUID) | Opaque identifier; passed to `proxy_connect_upstream` |
| display_name | string | Human-readable name shown to the user |
| description | string? | Optional description of the server |
| server_type | string | Internal type key (e.g. `"github"`, `"linear"`) |
| auth_type | string | `"api_key"`, `"pat"`, `"oauth2"`, `"none"` |
| requires_oauth | bool | Derived: `auth_type == "oauth2"` |

Fields **never** exposed:
- `oauth_client_id`, `encrypted_oauth_secret` — admin-only secrets
- `server_url` — not needed by the user at connect time
- `added_by` — internal

---

### UpstreamConfig (user-scoped read/write)

Source: `store.UpstreamConfig` / `upstream_configs` table.

Fields exposed by `proxy_list_upstreams`:

| Field | Type | Notes |
|-------|------|-------|
| id | string (UUID) | Opaque identifier; passed to `proxy_disconnect_upstream` / `proxy_update_credentials` |
| display_name | string | Joined from catalog at list time via `server_type` key |
| server_type | string | Internal type key |
| auth_type | string | `"api_key"` or `"pat"` |
| status | string | `"active"`, `"unreachable"`, `"reauth_required"` |
| detected_transport | string? | `"http"` or `"sse"` if probed; null if not yet determined |

Fields **never** exposed:
- `encrypted_creds` — credential material; never returned in any response
- `user_id` — implicit from session; not returned

---

## Dependency on spec 004 changes

### `ProxySession` (modified)

Add `UserID string` field, populated in `OpenSession`.

### `ProxyServerDeps` (modified)

Add `ManagementDeps` nested struct:

```
ManagementDeps {
    UpstreamStore store.UpstreamStoreI
    CatalogStore  store.CatalogStoreI
    KMSEncrypt    func(ctx context.Context, plaintext []byte) (ciphertext []byte, err error)
}
```

Note: `UpstreamStore` and `CatalogStore` are already present in `SessionDeps`; the `ManagementDeps` fields are the same instances, just surfaced at a level where tool-handler constructors can access them without going through the session.
