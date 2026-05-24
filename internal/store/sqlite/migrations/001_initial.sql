CREATE TABLE IF NOT EXISTS users (
	id           TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
	email        TEXT NOT NULL UNIQUE,
	password_hash TEXT,
	role         TEXT NOT NULL DEFAULT 'developer',
	proxy_token  TEXT NOT NULL DEFAULT (lower(hex(randomblob(32)))),
	created_at   TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at   TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS default_catalog (
	id                     TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
	server_type            TEXT NOT NULL UNIQUE,
	server_url             TEXT NOT NULL DEFAULT '',
	display_name           TEXT NOT NULL,
	description            TEXT,
	added_by               TEXT NOT NULL,
	active                 INTEGER NOT NULL DEFAULT 1,
	auth_type              TEXT NOT NULL,
	oauth_client_id        TEXT,
	encrypted_oauth_secret BLOB,
	transport              TEXT NOT NULL DEFAULT 'http',
	command                TEXT,
	args                   TEXT NOT NULL DEFAULT '[]',
	env                    TEXT NOT NULL DEFAULT '{}',
	created_at             TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at             TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS upstream_configs (
	id                  TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
	user_id             TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	server_type         TEXT NOT NULL,
	server_url          TEXT NOT NULL DEFAULT '',
	auth_type           TEXT NOT NULL,
	encrypted_creds     BLOB,
	detected_transport  TEXT,
	status              TEXT NOT NULL DEFAULT 'pending',
	status_checked_at   TEXT,
	created_at          TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at          TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(user_id, server_type)
);

CREATE TABLE IF NOT EXISTS oauth2_state (
	id          TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
	user_id     TEXT NOT NULL,
	server_type TEXT NOT NULL,
	state       TEXT NOT NULL UNIQUE,
	expires_at  TEXT NOT NULL,
	created_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS catalog_suggestions (
	id          TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
	user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	catalog_id  TEXT NOT NULL REFERENCES default_catalog(id) ON DELETE CASCADE,
	status      TEXT NOT NULL DEFAULT 'pending',
	resolved_at TEXT,
	created_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(user_id, catalog_id)
);

PRAGMA user_version = 1;
