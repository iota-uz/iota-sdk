-- +migrate Up
-- The audit of guarded changes (#1340): one event per change an operation applied, written in the
-- transaction that commits the receipt and delivered to the MCP auditor afterwards. A lost commit
-- response or a stopped process leaves the event here; it is delivered again (idempotently by
-- operation and change index) by the next call about the operation or by the dispatcher.
CREATE TABLE mcp.operation_audit (
    operation_id uuid NOT NULL REFERENCES mcp.operations (id) ON DELETE CASCADE,
    item integer NOT NULL CHECK (item >= 0),
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    subject varchar(255) NOT NULL,
    client_id varchar(255) NOT NULL DEFAULT '',
    tool varchar(255) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    delivered_at timestamptz,
    PRIMARY KEY (operation_id, item)
);

CREATE INDEX operation_audit_undelivered_idx ON mcp.operation_audit (created_at)
WHERE
    delivered_at IS NULL;

-- +migrate Down
DROP TABLE IF EXISTS mcp.operation_audit;
