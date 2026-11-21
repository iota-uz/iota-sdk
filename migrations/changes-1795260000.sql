-- Extension runtime schedules: a slot older than the schedule's missed-slot window is recorded as skipped, and the accounted past is remembered per schedule and tenant.
-- +migrate Up
ALTER TABLE extruntime.schedule_runs
    DROP CONSTRAINT IF EXISTS schedule_runs_status_check;

-- Broader than the constraint it replaces, so existing rows satisfy it; it is validated by the next migration, outside this lock.
ALTER TABLE extruntime.schedule_runs
    ADD CONSTRAINT schedule_runs_status_check CHECK (status IN ('done', 'failed', 'dead', 'blocked', 'skipped')) NOT VALID;

-- A counter every change of a receipt increments: a late writer proves nobody recorded a result since it claimed the slot.
ALTER TABLE extruntime.schedule_runs
    ADD COLUMN generation bigint NOT NULL DEFAULT 0;

-- Every slot of the schedule at or before accounted_through has a schedule_runs receipt (run or skipped).
CREATE TABLE extruntime.schedule_accounting (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    schedule text NOT NULL,
    accounted_through timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, schedule)
);

-- +migrate Down
DROP TABLE IF EXISTS extruntime.schedule_accounting;

ALTER TABLE extruntime.schedule_runs
    DROP COLUMN IF EXISTS generation;

DELETE FROM extruntime.schedule_runs
WHERE status = 'skipped';

ALTER TABLE extruntime.schedule_runs
    DROP CONSTRAINT IF EXISTS schedule_runs_status_check;

ALTER TABLE extruntime.schedule_runs
    ADD CONSTRAINT schedule_runs_status_check CHECK (status IN ('done', 'failed', 'dead', 'blocked'));
