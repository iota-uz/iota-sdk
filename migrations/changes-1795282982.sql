-- Indexes of native workflows (#1321), built concurrently outside a
-- transaction so runs keep writing meanwhile: the parent tree TerminateTree
-- walks per tenant, and the waiting occurrences a wake reads instead of the
-- whole journal.
-- +migrate Up notransaction
CREATE INDEX CONCURRENTLY IF NOT EXISTS workflow_instances_tenant_parent_idx ON extruntime.workflow_instances (tenant_id, parent_instance_id)
WHERE
    parent_instance_id IS NOT NULL;

DROP INDEX CONCURRENTLY IF EXISTS extruntime.workflow_instances_parent_idx;

CREATE INDEX CONCURRENTLY IF NOT EXISTS workflow_occurrences_waiting_idx ON extruntime.workflow_occurrences (instance_id)
WHERE
    status = 'waiting';

-- +migrate Down notransaction
DROP INDEX CONCURRENTLY IF EXISTS extruntime.workflow_occurrences_waiting_idx;

CREATE INDEX CONCURRENTLY IF NOT EXISTS workflow_instances_parent_idx ON extruntime.workflow_instances (parent_instance_id)
WHERE
    parent_instance_id IS NOT NULL;

DROP INDEX CONCURRENTLY IF EXISTS extruntime.workflow_instances_tenant_parent_idx;
