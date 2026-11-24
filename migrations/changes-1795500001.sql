-- The index of open ask waits by correlation token (#1336), built concurrently outside a transaction
-- so runs keep writing automation.waits meanwhile. Re-runnable: a failed concurrent build leaves an
-- INVALID index under the name, so it is dropped before it is built.
-- +migrate Up notransaction
DROP INDEX CONCURRENTLY IF EXISTS automation.waits_open_correlation_idx;

CREATE INDEX CONCURRENTLY waits_open_correlation_idx ON automation.waits (correlation)
WHERE
    status = 'open' AND correlation IS NOT NULL;

-- +migrate Down notransaction
DROP INDEX CONCURRENTLY IF EXISTS automation.waits_open_correlation_idx;
