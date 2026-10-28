-- +migrate Up
CREATE SCHEMA IF NOT EXISTS mcp;

CREATE TABLE mcp.plans (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    subject varchar(255) NOT NULL,
    client_id varchar(255) NOT NULL DEFAULT '',
    hash varchar(64) NOT NULL,
    reason text NOT NULL DEFAULT '',
    changes jsonb NOT NULL,
    items jsonb NOT NULL,
    warnings jsonb NOT NULL DEFAULT '[]',
    approval_required boolean NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX plans_tenant_subject_idx ON mcp.plans (tenant_id, subject, created_at DESC);

CREATE TABLE mcp.approvals (
    ref_hash varchar(64) PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    plan_id uuid NOT NULL REFERENCES mcp.plans (id) ON DELETE CASCADE,
    plan_hash varchar(64) NOT NULL,
    subject varchar(255) NOT NULL,
    approved_by varchar(255) NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX approvals_plan_idx ON mcp.approvals (tenant_id, plan_id);

CREATE TABLE mcp.operations (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    subject varchar(255) NOT NULL,
    idempotency_key varchar(255) NOT NULL,
    plan_id uuid NOT NULL REFERENCES mcp.plans (id),
    plan_hash varchar(64) NOT NULL,
    status varchar(16) NOT NULL CHECK (status IN ('pending', 'succeeded', 'failed')),
    external_effects varchar(16) NOT NULL DEFAULT 'none' CHECK (external_effects IN ('none', 'pending', 'succeeded', 'failed')),
    receipt jsonb,
    error jsonb,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, subject, idempotency_key)
);

CREATE TABLE mcp.operation_effects (
    id uuid PRIMARY KEY,
    operation_id uuid NOT NULL REFERENCES mcp.operations (id) ON DELETE CASCADE,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    kind varchar(255) NOT NULL,
    payload jsonb NOT NULL,
    status varchar(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'succeeded', 'failed')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX operation_effects_operation_idx ON mcp.operation_effects (operation_id);

CREATE INDEX operation_effects_pending_idx ON mcp.operation_effects (created_at)
WHERE
    status = 'pending';

-- +migrate Down
DROP TABLE IF EXISTS mcp.operation_effects;

DROP TABLE IF EXISTS mcp.operations;

DROP TABLE IF EXISTS mcp.approvals;

DROP TABLE IF EXISTS mcp.plans;

DROP SCHEMA IF EXISTS mcp;

