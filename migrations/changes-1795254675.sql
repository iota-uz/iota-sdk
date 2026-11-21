-- Workflow contracts with a Go implementation (#1321): native instances and
-- the title of a task a native workflow opens. The indexes are built
-- concurrently in changes-1795282982.sql.
-- +migrate Up
ALTER TABLE extruntime.workflow_instances
    ADD COLUMN native boolean NOT NULL DEFAULT FALSE;

ALTER TABLE extruntime.human_tasks
    ADD COLUMN title jsonb;

-- +migrate Down
ALTER TABLE extruntime.human_tasks
    DROP COLUMN IF EXISTS title;

ALTER TABLE extruntime.workflow_instances
    DROP COLUMN IF EXISTS native;
