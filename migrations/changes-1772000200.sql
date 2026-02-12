-- +migrate Up
-- Applet engine persistent jobs store (slice 2)

CREATE TABLE IF NOT EXISTS applet_engine_jobs (
    tenant_id TEXT NOT NULL,
    applet_id TEXT NOT NULL,
    job_id TEXT NOT NULL,
    job_type TEXT NOT NULL,
    cron_expr TEXT NOT NULL DEFAULT '',
    method_name TEXT NOT NULL,
    params JSONB NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, applet_id, job_id)
);

CREATE INDEX IF NOT EXISTS idx_applet_engine_jobs_scope_created
    ON applet_engine_jobs (tenant_id, applet_id, created_at DESC);

-- +migrate Down
DROP INDEX IF EXISTS idx_applet_engine_jobs_scope_created;
DROP TABLE IF EXISTS applet_engine_jobs;
