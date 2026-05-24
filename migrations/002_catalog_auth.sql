-- 002_catalog_auth.sql
-- Admin pre-configures each MCP server's URL, auth type, and OAuth credentials
-- so users only need to supply their own key or go through OAuth.

ALTER TABLE default_catalog
  ADD COLUMN auth_type              TEXT NOT NULL DEFAULT 'api_key'
      CHECK (auth_type IN ('api_key', 'oauth2')),
  ADD COLUMN oauth_client_id        TEXT,
  ADD COLUMN encrypted_oauth_secret BYTEA;
