-- +migrate Up
-- The audit of guarded changes (#1340): one event per change an operation applied, written in the
-- transaction that commits the receipt and delivered to the MCP auditor afterwards. A lost commit
-- response or a stopped process leaves the event here; it is delivered again (idempotently by
-- operation and change index) by the next call about the operation or by the dispatcher.
CREATE TABLE mcp.operation_audit (
    operation_id uuid NOT NULL REFERENCES mcp.operations (id) ON DELETE CASCADE,
    item integer NOT NULL CHECK (item >= 0),
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    -- text: a client id is what the token names, of any length
    subject text NOT NULL,
    client_id text NOT NULL DEFAULT '',
    request_id text NOT NULL DEFAULT '',
    tool text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- a delivery the auditor did not confirm is retried after a backoff; the others go on meanwhile
    attempts integer NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    delivered_at timestamptz,
    PRIMARY KEY (operation_id, item)
);

CREATE INDEX operation_audit_undelivered_idx ON mcp.operation_audit (next_attempt_at)
WHERE
    delivered_at IS NULL;

-- +migrate Down
DROP TABLE IF EXISTS mcp.operation_audit;
