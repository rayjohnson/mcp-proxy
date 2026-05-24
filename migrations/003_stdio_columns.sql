-- Add stdio transport support columns to default_catalog.
ALTER TABLE default_catalog
    ADD COLUMN transport TEXT NOT NULL DEFAULT 'http',
    ADD COLUMN command TEXT,
    ADD COLUMN args JSONB NOT NULL DEFAULT '[]',
    ADD COLUMN env JSONB NOT NULL DEFAULT '{}';
