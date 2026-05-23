-- 001_initial_schema.sql

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT UNIQUE NOT NULL,
    password_hash TEXT,
    role          TEXT NOT NULL DEFAULT 'developer' CHECK (role IN ('developer', 'admin')),
    proxy_token   TEXT UNIQUE NOT NULL DEFAULT encode(gen_random_bytes(32), 'base64url'),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_users_email ON users (email);
CREATE INDEX idx_users_proxy_token ON users (proxy_token);

CREATE TABLE upstream_configs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    server_type         TEXT NOT NULL,
    server_url          TEXT NOT NULL,
    auth_type           TEXT NOT NULL CHECK (auth_type IN ('api_key', 'oauth2')),
    encrypted_creds     BYTEA,
    detected_transport  TEXT CHECK (detected_transport IN ('streamable_http', 'sse')),
    status              TEXT NOT NULL DEFAULT 'unreachable'
                            CHECK (status IN ('reachable', 'unreachable', 'credential_error', 'reauth_required')),
    status_checked_at   TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, server_type)
);

CREATE INDEX idx_upstream_configs_user_id ON upstream_configs (user_id);

CREATE TABLE default_catalog (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_type  TEXT UNIQUE NOT NULL,
    server_url   TEXT NOT NULL,
    display_name TEXT NOT NULL,
    description  TEXT,
    added_by     UUID NOT NULL REFERENCES users(id),
    active       BOOLEAN NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE catalog_suggestions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    catalog_id  UUID NOT NULL REFERENCES default_catalog(id),
    status      TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'accepted', 'dismissed')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ,
    UNIQUE (user_id, catalog_id)
);

CREATE INDEX idx_catalog_suggestions_user_id ON catalog_suggestions (user_id, status);

CREATE TABLE oauth2_state (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    server_type TEXT NOT NULL,
    state       TEXT UNIQUE NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_oauth2_state_state ON oauth2_state (state);
