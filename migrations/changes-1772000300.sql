-- +migrate Up
-- Applet engine jobs schedule metadata (slice 2)

ALTER TABLE applet_engine_jobs
    ADD COLUMN IF NOT EXISTS next_run_at TIMESTAMPTZ;

ALTER TABLE applet_engine_jobs
    ADD COLUMN IF NOT EXISTS last_run_at TIMESTAMPTZ;

ALTER TABLE applet_engine_jobs
    ADD COLUMN IF NOT EXISTS last_status TEXT NOT NULL DEFAULT '';

ALTER TABLE applet_engine_jobs
    ADD COLUMN IF NOT EXISTS last_error TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_applet_engine_jobs_due_runs
    ON applet_engine_jobs (status, next_run_at)
    WHERE job_type = 'scheduled' AND status = 'scheduled';

-- +migrate Down
DROP INDEX IF EXISTS idx_applet_engine_jobs_due_runs;

ALTER TABLE applet_engine_jobs
    DROP COLUMN IF EXISTS last_error;

ALTER TABLE applet_engine_jobs
    DROP COLUMN IF EXISTS last_status;

ALTER TABLE applet_engine_jobs
    DROP COLUMN IF EXISTS last_run_at;

ALTER TABLE applet_engine_jobs
    DROP COLUMN IF EXISTS next_run_at;
