-- +migrate Up
CREATE SCHEMA IF NOT EXISTS extruntime;

CREATE TABLE extruntime.bindings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slot_kind text NOT NULL CHECK (slot_kind IN ('command', 'screen', 'shell')),
    target_id text NOT NULL,
    package text NOT NULL,
    version text NOT NULL,
    export text NOT NULL,
    activated_by bigint,
    active_from timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    active_to timestamptz,
    CHECK (active_to IS NULL OR active_to >= active_from)
);

CREATE UNIQUE INDEX bindings_open_slot_idx ON extruntime.bindings (slot_kind, target_id)
WHERE
    active_to IS NULL;

CREATE TABLE extruntime.invocations (
    id uuid PRIMARY KEY,
    parent_invocation_id uuid,
    root_id uuid NOT NULL,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    subject_user_id bigint NOT NULL,
    subject_kind text NOT NULL CHECK (subject_kind IN ('initiator', 'owner')),
    contract text NOT NULL,
    contract_hash text NOT NULL DEFAULT '',
    binding_id uuid,
    binding_package text NOT NULL DEFAULT '',
    binding_version text NOT NULL DEFAULT '',
    binding_export text NOT NULL DEFAULT '',
    trigger_kind text NOT NULL CHECK (trigger_kind IN ('ui', 'mcp', 'endpoint', 'schedule', 'subscription', 'workflow')),
    trigger_ref text NOT NULL DEFAULT '',
    started_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at timestamptz,
    outcome text NOT NULL DEFAULT 'unknown' CHECK (outcome IN ('ok', 'error', 'worker_lost', 'unknown')),
    committed_tx_count integer NOT NULL DEFAULT 0 CHECK (committed_tx_count >= 0),
    partially_committed boolean NOT NULL DEFAULT false,
    result jsonb,
    error jsonb
);

CREATE INDEX invocations_tenant_started_idx ON extruntime.invocations (tenant_id, started_at DESC);

CREATE INDEX invocations_root_idx ON extruntime.invocations (root_id);

CREATE INDEX invocations_contract_started_idx ON extruntime.invocations (contract, started_at DESC);

-- +migrate Down
DROP TABLE IF EXISTS extruntime.invocations;

DROP TABLE IF EXISTS extruntime.bindings;
