-- Validate the schedule_runs status check in its own transaction: validation takes a lock that does not block reads or writes.
-- +migrate Up
ALTER TABLE extruntime.schedule_runs VALIDATE CONSTRAINT schedule_runs_status_check;

-- +migrate Down
SELECT 1;
