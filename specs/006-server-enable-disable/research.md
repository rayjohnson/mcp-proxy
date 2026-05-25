# Research: Per-User Server Enable/Disable

## Decision 1: Storage for HTTP upstream toggle state

**Decision**: Add `enabled INTEGER NOT NULL DEFAULT 1` column to the existing `upstream_configs` table.

**Rationale**: HTTP upstreams already have a per-user row in `upstream_configs`. Adding a column is the minimal change — no new table, no join required. The default `1` (enabled) means existing rows are unaffected by the migration.

**Alternatives considered**:
- Separate `upstream_toggles` table — rejected: adds a join to every session open; overkill for a boolean on an already per-user table.

---

## Decision 2: Storage for stdio catalog toggle state

**Decision**: New `server_toggles` table with `(user_id, catalog_id, enabled)`.

**Rationale**: Stdio catalog entries are shared across all users (no per-user row today). A separate toggle table is the only way to store per-user state without duplicating catalog rows. The table mirrors the pattern of `catalog_suggestions` which already handles per-user state for catalog entries.

**Alternatives considered**:
- Add a `disabled_users` JSON column to `default_catalog` — rejected: violates per-row isolation, awkward to query, not cloud-scalable.
- Re-use `upstream_configs` with a synthetic entry for stdio — rejected: conflates two distinct concepts, breaks the invariant that `upstream_configs` rows have credentials.

---

## Decision 3: Toggle API shape

**Decision**: `POST /api/upstreams/{id}/toggle` and `POST /api/catalog/{id}/toggle` — idempotent flip (enabled→disabled, disabled→enabled), returns new state as JSON.

**Rationale**: A single toggle endpoint is simpler than separate enable/disable endpoints and matches the dashboard UX (one click flips the state). The current state is returned so the UI can update without a full page reload.

**Alternatives considered**:
- `PUT /api/upstreams/{id}` with `{"enabled": false}` body — more REST-pure but requires the client to know current state before toggling; more code for the same UX.

---

## Decision 4: Session layer integration

**Decision**: Pass a `ToggleStore` into `SessionDeps`; `OpenSession` filters upstream configs where `enabled = false` before connecting, and `connectStdioEntries` skips catalog entries where the user has a toggle record with `enabled = 0`.

**Rationale**: Filtering at session-open time is the right place — it's the single choke point for all upstream connections. No changes needed in the aggregator or router.

**Alternatives considered**:
- Filter at tool-list time — rejected: tools would still be listed even if the connection was never opened; doesn't satisfy FR-003.
