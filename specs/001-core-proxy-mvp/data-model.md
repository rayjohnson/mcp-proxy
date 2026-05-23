# Data Model: Core MCP Proxy MVP

**Branch**: `001-core-proxy-mvp` | **Date**: 2026-05-23

All tables live in Cloud SQL (PostgreSQL 16). Credentials are stored as KMS-encrypted
ciphertext — plaintext values never reach the database.

---

## users

Represents a registered developer or administrator account.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | Generated on creation |
| email | TEXT UNIQUE NOT NULL | Lowercased, indexed |
| password_hash | TEXT | bcrypt hash; NULL if OIDC-only account |
| role | TEXT NOT NULL | `developer` or `admin` |
| proxy_token | TEXT UNIQUE NOT NULL | URL-safe random token; identifies the user's MCP endpoint |
| created_at | TIMESTAMPTZ NOT NULL | |
| updated_at | TIMESTAMPTZ NOT NULL | |

**Constraints**:
- `proxy_token` is generated once at registration and never changes (SC-005: stable
  endpoint URL)
- `role` is enforced at the application layer for admin catalog operations

---

## sessions

Short-lived JWT sessions are stateless and do not require a database table. This
table is reserved for future refresh token tracking if session policy changes.

*(No table for MVP — JWT validation is stateless.)*

---

## upstream_configs

A developer's personal configuration of a single upstream MCP server.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| user_id | UUID NOT NULL → users.id | Cascade delete |
| server_type | TEXT NOT NULL | `github`, `notion`, `linear`, `cloudflare`, `google_cloud` |
| server_url | TEXT NOT NULL | HTTPS URL of the upstream MCP server |
| auth_type | TEXT NOT NULL | `api_key` or `oauth2` |
| encrypted_creds | BYTEA | KMS-encrypted JSON blob; see Credential Payload below |
| status | TEXT NOT NULL | `reachable`, `unreachable`, `credential_error`, `reauth_required` |
| status_checked_at | TIMESTAMPTZ | Last time the proxy verified connectivity |
| created_at | TIMESTAMPTZ NOT NULL | |
| updated_at | TIMESTAMPTZ NOT NULL | |

**Unique constraint**: `(user_id, server_type)` — one config per server type per user.

**Credential Payload** (encrypted JSON, never stored plaintext):

For `api_key`:
```json
{ "key": "<api key value>" }
```

For `oauth2`:
```json
{
  "access_token": "...",
  "refresh_token": "...",
  "token_type": "Bearer",
  "expiry": "2026-05-23T12:00:00Z",
  "scopes": ["..."]
}
```

**State transitions**:
```
(new) → reachable
reachable → unreachable       (connectivity check fails)
reachable → credential_error  (upstream returns 401)
reachable → reauth_required   (OAuth2 refresh fails with invalid_grant)
unreachable → reachable       (connectivity restored)
credential_error → reachable  (credentials updated)
reauth_required → reachable   (developer completes re-authorization)
```

---

## default_catalog

Admin-managed list of upstream MCP servers suggested to all developers.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| server_type | TEXT UNIQUE NOT NULL | Must match a supported server type |
| server_url | TEXT NOT NULL | Default URL for this server type |
| display_name | TEXT NOT NULL | Human-readable name shown in UI |
| description | TEXT | Optional short description |
| added_by | UUID NOT NULL → users.id | Admin who added this entry |
| active | BOOLEAN NOT NULL DEFAULT true | Soft delete; false = removed from catalog |
| created_at | TIMESTAMPTZ NOT NULL | |
| updated_at | TIMESTAMPTZ NOT NULL | |

**No credentials are stored here** — the catalog records only server type and URL.
Each developer provides their own credentials when accepting a catalog entry.

---

## catalog_suggestions

Tracks which catalog entries have been surfaced to which developers, and whether
they were accepted or dismissed.

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| user_id | UUID NOT NULL → users.id | Cascade delete |
| catalog_id | UUID NOT NULL → default_catalog.id | |
| status | TEXT NOT NULL | `pending`, `accepted`, `dismissed` |
| created_at | TIMESTAMPTZ NOT NULL | When suggestion was created (admin added catalog entry) |
| resolved_at | TIMESTAMPTZ | When developer accepted or dismissed |

**Unique constraint**: `(user_id, catalog_id)`.

**Rules**:
- A `pending` suggestion is shown in the developer's dashboard.
- `dismissed` suggestions are never re-shown for the same catalog entry (FR-021).
- When a developer adds an upstream server whose type matches a `pending` suggestion,
  the suggestion transitions to `accepted` automatically.
- Removing a catalog entry (setting `active = false`) does not affect existing
  `accepted` suggestions or developer configs.

---

## oauth2_state

Short-lived CSRF state tokens for OAuth2 authorization flows. Prevents callback
interception (FR-011 edge case).

| Column | Type | Notes |
|--------|------|-------|
| id | UUID PK | |
| user_id | UUID NOT NULL → users.id | |
| server_type | TEXT NOT NULL | Which upstream service this flow is for |
| state | TEXT UNIQUE NOT NULL | Random URL-safe token |
| expires_at | TIMESTAMPTZ NOT NULL | 10 minutes from creation |
| created_at | TIMESTAMPTZ NOT NULL | |

**Cleanup**: Expired rows are deleted on the next OAuth2 callback request for that
user. A background cleanup job is not required for MVP.

---

## Entity Relationships

```
users (1) ──────────────── (many) upstream_configs
  │                                      │
  │                                      └── encrypted_creds (KMS blob)
  │
  └── (many) catalog_suggestions ── (1) default_catalog
  │
  └── (many) oauth2_state
```

The `proxy_token` on `users` is the public-facing identifier in the MCP endpoint
URL: `https://<service>/mcp/<proxy_token>`. It is never the same as the user's `id`.
