-- Workflow contracts with a Go implementation (#1321): native instances, the
-- parent tree TerminateTree walks per tenant, the title of a task a native
-- workflow opens, and the waiting occurrences a wake reads.
-- +migrate Up
ALTER TABLE extruntime.workflow_instances
    ADD COLUMN native boolean NOT NULL DEFAULT FALSE;

DROP INDEX IF EXISTS extruntime.workflow_instances_parent_idx;

CREATE INDEX workflow_instances_parent_idx ON extruntime.workflow_instances (tenant_id, parent_instance_id)
WHERE
    parent_instance_id IS NOT NULL;

ALTER TABLE extruntime.human_tasks
    ADD COLUMN title jsonb;

-- A wake reads the waiting occurrences of one instance, not its whole journal.
CREATE INDEX workflow_occurrences_waiting_idx ON extruntime.workflow_occurrences (instance_id)
WHERE
    status = 'waiting';

-- +migrate Down
DROP INDEX IF EXISTS extruntime.workflow_occurrences_waiting_idx;

ALTER TABLE extruntime.human_tasks
    DROP COLUMN IF EXISTS title;

DROP INDEX IF EXISTS extruntime.workflow_instances_parent_idx;

CREATE INDEX workflow_instances_parent_idx ON extruntime.workflow_instances (parent_instance_id)
WHERE
    parent_instance_id IS NOT NULL;

ALTER TABLE extruntime.workflow_instances
    DROP COLUMN IF EXISTS native;
