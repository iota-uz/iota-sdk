-- +migrate Up
-- Applet engine persistent document store (slice 2)

CREATE TABLE IF NOT EXISTS applet_engine_documents (
    tenant_id TEXT NOT NULL,
    applet_id TEXT NOT NULL,
    table_name TEXT NOT NULL,
    document_id TEXT NOT NULL,
    value JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, applet_id, document_id)
);

CREATE INDEX IF NOT EXISTS idx_applet_engine_documents_table
    ON applet_engine_documents (tenant_id, applet_id, table_name, updated_at DESC);

-- +migrate Down
DROP INDEX IF EXISTS idx_applet_engine_documents_table;
DROP TABLE IF EXISTS applet_engine_documents;
