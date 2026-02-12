-- +migrate Up
-- Applet engine files metadata store (slice 2)

CREATE TABLE IF NOT EXISTS applet_engine_files (
    tenant_id TEXT NOT NULL,
    applet_id TEXT NOT NULL,
    file_id TEXT NOT NULL,
    file_name TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT '',
    size_bytes INT NOT NULL DEFAULT 0,
    storage_path TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, applet_id, file_id)
);

CREATE INDEX IF NOT EXISTS idx_applet_engine_files_scope_created
    ON applet_engine_files (tenant_id, applet_id, created_at DESC);

-- +migrate Down
DROP INDEX IF EXISTS idx_applet_engine_files_scope_created;
DROP TABLE IF EXISTS applet_engine_files;
