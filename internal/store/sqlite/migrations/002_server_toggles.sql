ALTER TABLE upstream_configs ADD COLUMN enabled INTEGER NOT NULL DEFAULT 1;

CREATE TABLE IF NOT EXISTS server_toggles (
	id         TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
	user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	catalog_id TEXT NOT NULL REFERENCES default_catalog(id) ON DELETE CASCADE,
	enabled    INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(user_id, catalog_id)
);

PRAGMA user_version = 2;
