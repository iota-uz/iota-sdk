-- Extension runtime schedules: a slot older than the schedule's missed-slot window is recorded as skipped.
-- +migrate Up
ALTER TABLE extruntime.schedule_runs
    DROP CONSTRAINT IF EXISTS schedule_runs_status_check;

ALTER TABLE extruntime.schedule_runs
    ADD CONSTRAINT schedule_runs_status_check CHECK (status IN ('done', 'failed', 'dead', 'blocked', 'skipped'));

-- +migrate Down
DELETE FROM extruntime.schedule_runs
WHERE status = 'skipped';

ALTER TABLE extruntime.schedule_runs
    DROP CONSTRAINT IF EXISTS schedule_runs_status_check;

ALTER TABLE extruntime.schedule_runs
    ADD CONSTRAINT schedule_runs_status_check CHECK (status IN ('done', 'failed', 'dead', 'blocked'));
