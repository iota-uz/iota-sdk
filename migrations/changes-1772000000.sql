-- +migrate Up
-- Applet engine slice-2 storage schema (squashed)

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

CREATE TABLE IF NOT EXISTS applet_engine_jobs (
    tenant_id TEXT NOT NULL,
    applet_id TEXT NOT NULL,
    job_id TEXT NOT NULL,
    job_type TEXT NOT NULL,
    cron_expr TEXT NOT NULL DEFAULT '',
    method_name TEXT NOT NULL,
    params JSONB NOT NULL,
    status TEXT NOT NULL,
    next_run_at TIMESTAMPTZ,
    last_run_at TIMESTAMPTZ,
    last_status TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, applet_id, job_id)
);

CREATE INDEX IF NOT EXISTS idx_applet_engine_jobs_scope_created
    ON applet_engine_jobs (tenant_id, applet_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_applet_engine_jobs_due_runs
    ON applet_engine_jobs (status, next_run_at)
    WHERE job_type = 'scheduled' AND status = 'scheduled';

CREATE TABLE IF NOT EXISTS applet_engine_secrets (
    applet_id TEXT NOT NULL,
    secret_name TEXT NOT NULL,
    cipher_text TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (applet_id, secret_name)
);

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

DROP TABLE IF EXISTS applet_engine_secrets;

DROP INDEX IF EXISTS idx_applet_engine_jobs_due_runs;
DROP INDEX IF EXISTS idx_applet_engine_jobs_scope_created;
DROP TABLE IF EXISTS applet_engine_jobs;

DROP INDEX IF EXISTS idx_applet_engine_documents_table;
DROP TABLE IF EXISTS applet_engine_documents;
