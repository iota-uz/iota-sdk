-- Indexes of native workflows (#1321), built concurrently outside a
-- transaction so runs keep writing meanwhile: the parent tree TerminateTree
-- walks per tenant, and the waiting occurrences a wake reads instead of the
-- whole journal.
--
-- Re-runnable: a failed concurrent build leaves an INVALID index under the
-- name, so each index is dropped before it is built instead of being skipped
-- with IF NOT EXISTS. The old parent index is dropped only after its
-- replacement was built (a failed build stops the migration before that).
-- +migrate Up notransaction
DROP INDEX CONCURRENTLY IF EXISTS extruntime.workflow_instances_tenant_parent_idx;

CREATE INDEX CONCURRENTLY workflow_instances_tenant_parent_idx ON extruntime.workflow_instances (tenant_id, parent_instance_id)
WHERE
    parent_instance_id IS NOT NULL;

DROP INDEX CONCURRENTLY IF EXISTS extruntime.workflow_instances_parent_idx;

DROP INDEX CONCURRENTLY IF EXISTS extruntime.workflow_occurrences_waiting_idx;

CREATE INDEX CONCURRENTLY workflow_occurrences_waiting_idx ON extruntime.workflow_occurrences (instance_id)
WHERE
    status = 'waiting';

-- +migrate Down notransaction
DROP INDEX CONCURRENTLY IF EXISTS extruntime.workflow_occurrences_waiting_idx;

DROP INDEX CONCURRENTLY IF EXISTS extruntime.workflow_instances_parent_idx;

CREATE INDEX CONCURRENTLY workflow_instances_parent_idx ON extruntime.workflow_instances (parent_instance_id)
WHERE
    parent_instance_id IS NOT NULL;

DROP INDEX CONCURRENTLY IF EXISTS extruntime.workflow_instances_tenant_parent_idx;
