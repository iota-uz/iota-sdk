-- The index of the dependents of a token set by their owner, built concurrently outside a transaction so
-- the connections keep being read and written meanwhile. Re-runnable: a failed concurrent build leaves an
-- INVALID index under the name, so it is dropped before it is built.
-- +migrate Up notransaction
DROP INDEX CONCURRENTLY IF EXISTS automation.connections_token_owner_idx;

CREATE INDEX CONCURRENTLY connections_token_owner_idx ON automation.connections (tenant_id, token_owner)
WHERE
    token_owner IS NOT NULL;

-- +migrate Down notransaction
DROP INDEX CONCURRENTLY IF EXISTS automation.connections_token_owner_idx;
