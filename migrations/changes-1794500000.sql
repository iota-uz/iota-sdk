-- +migrate Up
-- Extension runtime: events (outbox, subscription receipts, trigger owners,
-- schedule runs) and the durable workflow engine (docs/extension-runtime.md §8, §9).

CREATE TABLE extruntime.outbox_events (
    seq bigserial NOT NULL UNIQUE,
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    event text NOT NULL,
    subject_entity text,
    subject_key text,
    payload jsonb NOT NULL,
    initiator_user_id bigint,
    producer_invocation_id uuid,
    occurred_at timestamptz NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX outbox_events_event_seq_idx ON extruntime.outbox_events (event, seq);

CREATE INDEX outbox_events_occurred_idx ON extruntime.outbox_events (occurred_at);

CREATE INDEX outbox_events_tenant_idx ON extruntime.outbox_events (tenant_id, seq);

CREATE TABLE extruntime.subscription_receipts (
    subscription text NOT NULL,
    event_id uuid NOT NULL REFERENCES extruntime.outbox_events (id) ON DELETE CASCADE,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    status text NOT NULL CHECK (status IN ('pending', 'done', 'failed', 'dead', 'blocked')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_error text NOT NULL DEFAULT '',
    invocation_id uuid,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (subscription, event_id)
);

CREATE INDEX subscription_receipts_retry_idx ON extruntime.subscription_receipts (subscription, next_at)
WHERE
    status IN ('pending', 'failed', 'blocked');

CREATE TABLE extruntime.trigger_owners (
    trigger_id text NOT NULL,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id bigint NOT NULL,
    assigned_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (trigger_id, tenant_id)
);

CREATE TABLE extruntime.schedule_runs (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    schedule text NOT NULL,
    slot timestamptz NOT NULL,
    status text NOT NULL CHECK (status IN ('done', 'failed', 'dead', 'blocked')),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_error text NOT NULL DEFAULT '',
    invocation_id uuid,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, schedule, slot)
);

CREATE TABLE extruntime.workflow_instances (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    workflow text NOT NULL,
    package text NOT NULL,
    version text NOT NULL,
    artifact_hash text NOT NULL DEFAULT '',
    export text NOT NULL,
    input jsonb NOT NULL,
    status text NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'waiting', 'completed', 'failed', 'terminated', 'blocked')),
    result jsonb,
    error jsonb,
    blocked_reason text NOT NULL DEFAULT '',
    blocked_detail text NOT NULL DEFAULT '',
    subject_user_id bigint NOT NULL,
    subject_kind text NOT NULL CHECK (subject_kind IN ('initiator', 'owner')),
    predecessor_id uuid REFERENCES extruntime.workflow_instances (id),
    parent_instance_id uuid REFERENCES extruntime.workflow_instances (id),
    start_invocation_id uuid,
    start_key text,
    wake_at timestamptz,
    lease_owner text,
    lease_until timestamptz,
    lease_epoch bigint NOT NULL DEFAULT 0,
    run_count integer NOT NULL DEFAULT 0,
    failures integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at timestamptz
);

CREATE UNIQUE INDEX workflow_instances_start_key_idx ON extruntime.workflow_instances (tenant_id, start_key)
WHERE
    start_key IS NOT NULL;

CREATE INDEX workflow_instances_ready_idx ON extruntime.workflow_instances (status, wake_at, lease_until)
WHERE
    status IN ('running', 'waiting');

CREATE INDEX workflow_instances_tenant_idx ON extruntime.workflow_instances (tenant_id, created_at DESC);

CREATE INDEX workflow_instances_predecessor_idx ON extruntime.workflow_instances (predecessor_id)
WHERE
    predecessor_id IS NOT NULL;

CREATE INDEX workflow_instances_parent_idx ON extruntime.workflow_instances (parent_instance_id)
WHERE
    parent_instance_id IS NOT NULL;

CREATE TABLE extruntime.workflow_occurrences (
    instance_id uuid NOT NULL REFERENCES extruntime.workflow_instances (id) ON DELETE CASCADE,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name text NOT NULL,
    n integer NOT NULL CHECK (n >= 0),
    kind text NOT NULL CHECK (kind IN ('transaction', 'step', 'sleep', 'waitFor', 'task', 'call', 'start', 'now', 'uuid')),
    status text NOT NULL CHECK (status IN ('started', 'completed', 'failed', 'waiting', 'cancelled')),
    input_hash text NOT NULL DEFAULT '',
    result jsonb,
    error jsonb,
    attempts integer NOT NULL DEFAULT 0,
    wake_at timestamptz,
    invocation_id uuid,
    contract_hash text NOT NULL DEFAULT '',
    meta jsonb,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (instance_id, name, n)
);

CREATE INDEX workflow_occurrences_tenant_idx ON extruntime.workflow_occurrences (tenant_id, instance_id);

CREATE TABLE extruntime.workflow_signals (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    instance_id uuid NOT NULL REFERENCES extruntime.workflow_instances (id) ON DELETE CASCADE,
    name text NOT NULL,
    payload jsonb NOT NULL,
    sender_user_id bigint,
    signal_key text,
    consumed_name text,
    consumed_n integer,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    consumed_at timestamptz
);

CREATE UNIQUE INDEX workflow_signals_key_idx ON extruntime.workflow_signals (tenant_id, instance_id, signal_key)
WHERE
    signal_key IS NOT NULL;

CREATE INDEX workflow_signals_open_idx ON extruntime.workflow_signals (instance_id, name, created_at)
WHERE
    consumed_name IS NULL;

CREATE TABLE extruntime.human_tasks (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    instance_id uuid NOT NULL REFERENCES extruntime.workflow_instances (id) ON DELETE CASCADE,
    name text NOT NULL,
    n integer NOT NULL,
    subject_entity text NOT NULL DEFAULT '',
    subject_key text NOT NULL DEFAULT '',
    assignee_user_id bigint NOT NULL,
    decision_schema jsonb,
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'decided', 'expired', 'cancelled')),
    decision jsonb,
    decided_by bigint,
    due_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    decided_at timestamptz,
    UNIQUE (instance_id, name, n)
);

CREATE INDEX human_tasks_assignee_idx ON extruntime.human_tasks (tenant_id, assignee_user_id, status);

CREATE INDEX human_tasks_subject_idx ON extruntime.human_tasks (tenant_id, subject_entity, subject_key);

-- +migrate Down
DROP TABLE IF EXISTS extruntime.human_tasks;

DROP TABLE IF EXISTS extruntime.workflow_signals;

DROP TABLE IF EXISTS extruntime.workflow_occurrences;

DROP TABLE IF EXISTS extruntime.workflow_instances;

DROP TABLE IF EXISTS extruntime.schedule_runs;

DROP TABLE IF EXISTS extruntime.trigger_owners;

DROP TABLE IF EXISTS extruntime.subscription_receipts;

DROP TABLE IF EXISTS extruntime.outbox_events;
