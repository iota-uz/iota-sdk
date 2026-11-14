-- +migrate Up
-- Extension runtime platform (#411): runtime schema, notification recipients/targets, bindings, events, workflows, management.

-- ---- Extension runtime data layer: applied physical schema and per-version field maps.
-- Extension runtime data layer: applied physical schema and per-version field maps.
CREATE SCHEMA IF NOT EXISTS extruntime;

-- Physical state of every entity/overlay table. Column names are immutable;
-- "columns" lists each column with every field key that ever mapped to it.
CREATE TABLE extruntime.entity_schemas (
    package text NOT NULL,
    entity_id text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('entity', 'overlay')),
    schema_name text NOT NULL,
    table_name text NOT NULL,
    base jsonb,
    columns jsonb NOT NULL,
    version text NOT NULL,
    applied_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (package, entity_id),
    UNIQUE (schema_name, table_name)
);

-- What each package version sees: field key -> physical column (plus the
-- renamedFrom aliases) and the declared types. Immutable once written.
CREATE TABLE extruntime.entity_schema_versions (
    package text NOT NULL,
    version text NOT NULL,
    entity_id text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('entity', 'overlay')),
    schema_name text NOT NULL,
    table_name text NOT NULL,
    base jsonb,
    fields jsonb NOT NULL,
    applied_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (package, version, entity_id)
);

CREATE INDEX entity_schema_versions_applied_idx ON extruntime.entity_schema_versions (package, applied_at DESC);

CREATE TABLE extruntime.schema_migrations_history (
    id bigserial PRIMARY KEY,
    package text NOT NULL,
    from_version text NOT NULL DEFAULT '',
    to_version text NOT NULL,
    adopted boolean NOT NULL DEFAULT FALSE,
    statements jsonb NOT NULL DEFAULT '[]'::jsonb,
    renames jsonb NOT NULL DEFAULT '[]'::jsonb,
    applied_at timestamp with time zone NOT NULL DEFAULT now()
);

CREATE INDEX schema_migrations_history_package_idx ON extruntime.schema_migrations_history (package, id DESC);

-- ---- Notification recipients, targets and attempts: separate the notice instance, its recipient (user, business record or direct address) and the concrete channel target/device so external customers without an account are first-class recipients; record every provider attempt; add a push device registry.
CREATE TABLE core.notification_notices (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    event_key text NOT NULL,
    event_version integer NOT NULL DEFAULT 1 CHECK (event_version >= 1),
    snapshot_ref text NOT NULL DEFAULT '',
    event_id text NOT NULL,
    mode text NOT NULL CHECK (mode IN ('rule', 'explicit')),
    subject_entity text NOT NULL DEFAULT '',
    subject_key text NOT NULL DEFAULT '',
    producer_invocation_id text NOT NULL DEFAULT '',
    producer_event_id text NOT NULL DEFAULT '',
    producer_instance_id text NOT NULL DEFAULT '',
    producer_occurrence text NOT NULL DEFAULT '',
    event jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, event_key, event_id),
    CHECK ((subject_entity = '') = (subject_key = ''))
);

CREATE INDEX notification_notices_subject_idx ON core.notification_notices (tenant_id, subject_entity, subject_key)
WHERE
    subject_entity <> '';

INSERT INTO core.notification_notices (id, tenant_id, event_key, event_version, event_id, mode, event, created_at)
SELECT
    gen_random_uuid (),
    d.tenant_id,
    d.event_key,
    max(d.content_version),
    d.event_id,
    CASE WHEN d.event_id LIKE 'addressed:%' THEN
        'explicit'
    ELSE
        'rule'
    END,
    COALESCE((
        SELECT
            x.event
        FROM
            core.notification_dispatches x
        WHERE
            x.tenant_id = d.tenant_id
            AND x.event_key = d.event_key
            AND x.event_id = d.event_id), jsonb_build_object('key', d.event_key, 'id', d.event_id, 'tenant_id', d.tenant_id, 'version', max(d.content_version))),
    min(d.created_at)
FROM
    core.notification_deliveries d
GROUP BY
    d.tenant_id,
    d.event_key,
    d.event_id;

ALTER TABLE core.notification_dispatches
    ADD COLUMN mode text NOT NULL DEFAULT 'rule' CHECK (mode IN ('rule', 'explicit')),
    ADD COLUMN addressees jsonb NOT NULL DEFAULT '[]';

UPDATE
    core.notification_dispatches
SET
    mode = 'explicit'
WHERE
    event_id LIKE 'addressed:%';

ALTER TABLE core.notification_deliveries
    DROP CONSTRAINT notification_deliveries_tenant_id_event_key_event_id_user_i_key,
    DROP CONSTRAINT IF EXISTS notification_deliveries_status_check,
    ADD COLUMN notice_id uuid,
    ADD COLUMN recipient_kind text NOT NULL DEFAULT 'user' CHECK (recipient_kind IN ('user', 'record', 'address')),
    ADD COLUMN recipient_ref_entity text,
    ADD COLUMN recipient_ref_key text,
    ADD COLUMN recipient_key text NOT NULL DEFAULT '',
    ADD COLUMN target_key text NOT NULL DEFAULT '',
    ADD COLUMN target_address text NOT NULL DEFAULT '',
    ADD COLUMN target_device_id text NOT NULL DEFAULT '',
    ADD COLUMN content jsonb NOT NULL DEFAULT '{}',
    ADD COLUMN attempt_limit integer NOT NULL DEFAULT 5 CHECK (attempt_limit >= 1),
    ADD COLUMN inbox_notification_id uuid REFERENCES core.notifications (id) ON DELETE SET NULL;

ALTER TABLE core.notification_deliveries RENAME COLUMN user_id TO recipient_user_id;

ALTER TABLE core.notification_deliveries
    ALTER COLUMN recipient_user_id DROP NOT NULL;

ALTER TABLE core.notification_deliveries RENAME COLUMN last_error TO error_code;

UPDATE
    core.notification_deliveries d
SET
    notice_id = n.id,
    recipient_key = 'user:' || d.recipient_user_id,
    error_code = CASE d.error_code
    WHEN '' THEN
        ''
    WHEN 'recipient has no address for channel' THEN
        'no_address'
    WHEN 'attempt limit reached' THEN
        'attempt_limit'
    ELSE
        'provider_failure'
    END
FROM
    core.notification_notices n
WHERE
    n.tenant_id = d.tenant_id
    AND n.event_key = d.event_key
    AND n.event_id = d.event_id;

UPDATE
    core.notification_deliveries d
SET
    inbox_notification_id = i.id
FROM
    core.notifications i
WHERE
    d.channel = 'in_app'
    AND i.tenant_id = d.tenant_id
    AND i.user_id = d.recipient_user_id
    AND i.dedupe_key = d.event_key || ':' || d.event_id;

ALTER TABLE core.notification_deliveries
    ALTER COLUMN notice_id SET NOT NULL,
    ADD CONSTRAINT notification_deliveries_notice_fkey FOREIGN KEY (tenant_id, notice_id) REFERENCES core.notification_notices (tenant_id, id) ON DELETE CASCADE,
    ADD CONSTRAINT notification_deliveries_status_check CHECK (status IN ('unresolved', 'pending', 'sent', 'failed', 'ambiguous', 'no_address', 'skipped')),
    ADD CONSTRAINT notification_deliveries_recipient_check CHECK ((recipient_kind = 'user' AND recipient_user_id IS NOT NULL AND recipient_ref_entity IS NULL AND recipient_ref_key IS NULL) OR (recipient_kind = 'record' AND recipient_user_id IS NULL AND recipient_ref_entity IS NOT NULL AND recipient_ref_key IS NOT NULL) OR (recipient_kind = 'address' AND recipient_user_id IS NULL AND recipient_ref_entity IS NULL AND recipient_ref_key IS NULL AND (target_address <> '' OR status = 'no_address'))),
    ADD CONSTRAINT notification_deliveries_in_app_check CHECK (channel <> 'in_app' OR recipient_kind = 'user'),
    ADD CONSTRAINT notification_deliveries_inbox_check CHECK (inbox_notification_id IS NULL OR channel = 'in_app'),
    ADD CONSTRAINT notification_deliveries_target_key UNIQUE (tenant_id, notice_id, recipient_key, channel, target_key);

CREATE INDEX notification_deliveries_notice_idx ON core.notification_deliveries (tenant_id, event_key, event_id);

DROP INDEX core.notification_deliveries_due_idx;

CREATE INDEX notification_deliveries_due_idx ON core.notification_deliveries (tenant_id, available_at, created_at)
WHERE
    status IN ('pending', 'unresolved')
    AND channel <> 'in_app';

CREATE TABLE core.notification_delivery_attempts (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    delivery_id uuid NOT NULL REFERENCES core.notification_deliveries (id) ON DELETE CASCADE,
    attempt integer NOT NULL CHECK (attempt >= 1),
    started_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at timestamptz,
    outcome text CHECK (outcome IN ('sent', 'failed', 'permanent', 'ambiguous')),
    provider_receipt text NOT NULL DEFAULT '',
    error_code text NOT NULL DEFAULT '',
    PRIMARY KEY (delivery_id, attempt),
    CHECK ((outcome IS NULL) = (finished_at IS NULL))
);

CREATE INDEX notification_delivery_attempts_tenant_idx ON core.notification_delivery_attempts (tenant_id, delivery_id);

CREATE TABLE core.notification_push_devices (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id integer NOT NULL,
    device_id text NOT NULL CHECK (device_id <> ''),
    token text NOT NULL CHECK (token <> ''),
    platform text NOT NULL CHECK (platform IN ('ios', 'android', 'web')),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at timestamptz,
    CONSTRAINT notification_push_devices_user_tenant_fkey FOREIGN KEY (tenant_id, user_id) REFERENCES users (tenant_id, id) ON DELETE CASCADE,
    PRIMARY KEY (tenant_id, user_id, device_id)
);

CREATE UNIQUE INDEX notification_push_devices_token_idx ON core.notification_push_devices (tenant_id, token)
WHERE
    revoked_at IS NULL;

-- ---- Extension runtime: bindings and invocation records.
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

-- ---- Extension runtime: events (outbox, subscription receipts, trigger owners, schedule runs) and the durable workflow engine (docs/extension-runtime.md §8, §9). Human tasks are assigned to one user or to anyone holding a permission.
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
    assignee_kind text NOT NULL DEFAULT 'user' CHECK (assignee_kind IN ('user', 'permission')),
    assignee_user_id bigint,
    assignee_permission text NOT NULL DEFAULT '',
    decision_schema jsonb,
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'decided', 'expired', 'cancelled')),
    decision jsonb,
    decided_by bigint,
    due_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    decided_at timestamptz,
    UNIQUE (instance_id, name, n),
    CONSTRAINT human_tasks_assignee_check CHECK ((assignee_kind = 'user' AND assignee_user_id IS NOT NULL AND assignee_permission = '') OR (assignee_kind = 'permission' AND assignee_user_id IS NULL AND assignee_permission <> ''))
);

CREATE INDEX human_tasks_assignee_idx ON extruntime.human_tasks (tenant_id, assignee_user_id, status);

CREATE INDEX human_tasks_subject_idx ON extruntime.human_tasks (tenant_id, subject_entity, subject_key);

CREATE INDEX human_tasks_assignee_permission_idx ON extruntime.human_tasks (tenant_id, assignee_permission, status)
WHERE
    assignee_kind = 'permission';

-- ---- Extension runtime management: packages, draft trees, immutable versions, artifacts, reviews, activations, kit snapshots, audit.
-- Extension runtime management: packages, draft trees, immutable versions,
-- artifacts, reviews, activations, trigger owners, kit snapshots, audit.
-- Everything here is application-wide. trigger_owners (per tenant) belongs to the events migration.
CREATE TABLE extruntime.packages (
    namespace text PRIMARY KEY CHECK (namespace ~ '^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$'),
    npm_name text NOT NULL DEFAULT '',
    provenance text NOT NULL CHECK (provenance IN ('git', 'runtime')),
    repo_url text NOT NULL DEFAULT '',
    repo_commit text NOT NULL DEFAULT '',
    -- tree hash of the last imported tree; a draft whose tree still equals it
    -- publishes with git provenance, an edited one with runtime provenance
    imported_tree_hash text NOT NULL DEFAULT '',
    draft_rev bigint NOT NULL DEFAULT 0,
    created_by bigint NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by bigint NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (provenance <> 'git' OR (repo_url <> '' AND repo_commit <> ''))
);

CREATE TABLE extruntime.package_files (
    namespace text NOT NULL REFERENCES extruntime.packages (namespace) ON DELETE CASCADE,
    path text NOT NULL,
    content bytea NOT NULL,
    hash text NOT NULL,
    updated_by bigint NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (namespace, path)
);

-- Content-addressed blobs: tar archives of source trees and of built
-- artifact trees. Immutable.
CREATE TABLE extruntime.artifacts (
    hash text PRIMARY KEY,
    kind text NOT NULL CHECK (kind IN ('source', 'build')),
    content bytea NOT NULL,
    size bigint NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Developer kit snapshots (contract resolution snapshots). Immutable.
CREATE TABLE extruntime.catalog_snapshots (
    hash text PRIMARY KEY,
    content bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE extruntime.versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    namespace text NOT NULL REFERENCES extruntime.packages (namespace),
    version text NOT NULL CHECK (version <> ''),
    source_hash text NOT NULL,
    tree_hash text NOT NULL,
    source_archive text NOT NULL REFERENCES extruntime.artifacts (hash),
    artifact_hash text NOT NULL REFERENCES extruntime.artifacts (hash),
    lockfile_name text NOT NULL DEFAULT '',
    lockfile_hash text NOT NULL DEFAULT '',
    manifest jsonb NOT NULL,
    kit_hash text NOT NULL REFERENCES extruntime.catalog_snapshots (hash),
    provenance text NOT NULL CHECK (provenance IN ('git', 'runtime')),
    repo_url text NOT NULL DEFAULT '',
    repo_commit text NOT NULL DEFAULT '',
    diagnostics jsonb NOT NULL DEFAULT '[]'::jsonb,
    -- contract hashes of the foreign contracts the version implements, as built
    foreign_contracts jsonb NOT NULL DEFAULT '{}'::jsonb,
    author_id bigint NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (namespace, version)
);

CREATE INDEX versions_namespace_created_idx ON extruntime.versions (namespace, created_at DESC);

-- Reviews are append-only and bound to the exact hashes that were reviewed.
CREATE TABLE extruntime.version_reviews (
    id bigserial PRIMARY KEY,
    version_id uuid NOT NULL REFERENCES extruntime.versions (id),
    source_hash text NOT NULL,
    artifact_hash text NOT NULL,
    reviewer_id bigint NOT NULL,
    approve boolean NOT NULL,
    comment text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX version_reviews_version_idx ON extruntime.version_reviews (version_id, id DESC);

-- +migrate StatementBegin
CREATE FUNCTION extruntime.forbid_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'extruntime.% rows are immutable', TG_TABLE_NAME USING ERRCODE = 'restrict_violation';
END;
$$;
-- +migrate StatementEnd

CREATE TRIGGER versions_immutable BEFORE UPDATE OR DELETE ON extruntime.versions
FOR EACH ROW EXECUTE FUNCTION extruntime.forbid_mutation();

CREATE TRIGGER artifacts_immutable BEFORE UPDATE OR DELETE ON extruntime.artifacts
FOR EACH ROW EXECUTE FUNCTION extruntime.forbid_mutation();

CREATE TRIGGER catalog_snapshots_immutable BEFORE UPDATE OR DELETE ON extruntime.catalog_snapshots
FOR EACH ROW EXECUTE FUNCTION extruntime.forbid_mutation();

CREATE TRIGGER version_reviews_immutable BEFORE UPDATE OR DELETE ON extruntime.version_reviews
FOR EACH ROW EXECUTE FUNCTION extruntime.forbid_mutation();

-- Activation history. Application-wide; at most one active row per package.
CREATE TABLE extruntime.activations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    namespace text NOT NULL REFERENCES extruntime.packages (namespace),
    version_id uuid NOT NULL REFERENCES extruntime.versions (id),
    state text NOT NULL CHECK (state IN ('active', 'disabled', 'purged')),
    activated_by bigint NOT NULL,
    activated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ended_by bigint,
    ended_at timestamptz,
    -- DDL statements applied by this activation (empty when the schema did not change)
    schema_steps jsonb NOT NULL DEFAULT '[]'::jsonb
);

CREATE UNIQUE INDEX activations_one_active_idx ON extruntime.activations (namespace) WHERE state = 'active';

CREATE INDEX activations_namespace_idx ON extruntime.activations (namespace, activated_at DESC);

-- What a replacement binding displaced, so deactivating the replacement can
-- reinstate the previous implementation of the slot.
CREATE TABLE extruntime.binding_displaced (
    binding_id uuid PRIMARY KEY REFERENCES extruntime.bindings (id) ON DELETE CASCADE,
    displaced_binding_id uuid NOT NULL REFERENCES extruntime.bindings (id) ON DELETE CASCADE
);

-- Position in the event outbox from which a subscription receives events:
-- set when the subscription is first activated, never replayed backwards.
CREATE TABLE extruntime.subscription_watermarks (
    subscription text PRIMARY KEY,
    package text NOT NULL REFERENCES extruntime.packages (namespace),
    after_seq bigint NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX subscription_watermarks_package_idx ON extruntime.subscription_watermarks (package);

CREATE TABLE extruntime.audit_log (
    id bigserial PRIMARY KEY,
    at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    actor_user_id bigint NOT NULL,
    credential text NOT NULL DEFAULT '',
    action text NOT NULL,
    namespace text NOT NULL DEFAULT '',
    version text NOT NULL DEFAULT '',
    target text NOT NULL DEFAULT '',
    source_hash text NOT NULL DEFAULT '',
    artifact_hash text NOT NULL DEFAULT '',
    outcome text NOT NULL CHECK (outcome IN ('ok', 'failed', 'denied')),
    detail jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX audit_log_at_idx ON extruntime.audit_log (at DESC, id DESC);

CREATE INDEX audit_log_namespace_idx ON extruntime.audit_log (namespace, id DESC);

-- +migrate Down
DROP TABLE IF EXISTS extruntime.audit_log;

DROP TABLE IF EXISTS extruntime.subscription_watermarks;

DROP TABLE IF EXISTS extruntime.binding_displaced;

DROP TABLE IF EXISTS extruntime.activations;

DROP TABLE IF EXISTS extruntime.version_reviews;

DROP TABLE IF EXISTS extruntime.versions;

DROP TABLE IF EXISTS extruntime.catalog_snapshots;

DROP TABLE IF EXISTS extruntime.artifacts;

DROP TABLE IF EXISTS extruntime.package_files;

DROP TABLE IF EXISTS extruntime.packages;

DROP FUNCTION IF EXISTS extruntime.forbid_mutation();

DROP TABLE IF EXISTS extruntime.human_tasks;

DROP TABLE IF EXISTS extruntime.workflow_signals;

DROP TABLE IF EXISTS extruntime.workflow_occurrences;

DROP TABLE IF EXISTS extruntime.workflow_instances;

DROP TABLE IF EXISTS extruntime.schedule_runs;

DROP TABLE IF EXISTS extruntime.trigger_owners;

DROP TABLE IF EXISTS extruntime.subscription_receipts;

DROP TABLE IF EXISTS extruntime.outbox_events;

DROP TABLE IF EXISTS extruntime.invocations;

DROP TABLE IF EXISTS extruntime.bindings;

DROP TABLE IF EXISTS core.notification_push_devices;

DROP TABLE IF EXISTS core.notification_delivery_attempts;

DROP INDEX IF EXISTS core.notification_deliveries_due_idx;

DROP INDEX IF EXISTS core.notification_deliveries_notice_idx;

-- Only user-recipient, single-target deliveries fit the previous model.
DELETE FROM core.notification_deliveries
WHERE recipient_kind <> 'user'
    OR target_key <> '';

DELETE FROM core.notification_deliveries
WHERE status IN ('unresolved', 'ambiguous', 'skipped');

ALTER TABLE core.notification_deliveries
    DROP CONSTRAINT notification_deliveries_target_key,
    DROP CONSTRAINT notification_deliveries_inbox_check,
    DROP CONSTRAINT notification_deliveries_in_app_check,
    DROP CONSTRAINT notification_deliveries_recipient_check,
    DROP CONSTRAINT notification_deliveries_status_check,
    DROP CONSTRAINT notification_deliveries_notice_fkey;

ALTER TABLE core.notification_deliveries RENAME COLUMN error_code TO last_error;

ALTER TABLE core.notification_deliveries RENAME COLUMN recipient_user_id TO user_id;

ALTER TABLE core.notification_deliveries
    ALTER COLUMN user_id SET NOT NULL,
    DROP COLUMN inbox_notification_id,
    DROP COLUMN attempt_limit,
    DROP COLUMN content,
    DROP COLUMN target_device_id,
    DROP COLUMN target_address,
    DROP COLUMN target_key,
    DROP COLUMN recipient_key,
    DROP COLUMN recipient_ref_key,
    DROP COLUMN recipient_ref_entity,
    DROP COLUMN recipient_kind,
    DROP COLUMN notice_id,
    ADD CONSTRAINT notification_deliveries_status_check CHECK (status IN ('pending', 'sent', 'failed', 'no_address')),
    ADD CONSTRAINT notification_deliveries_tenant_id_event_key_event_id_user_i_key UNIQUE (tenant_id, event_key, event_id, user_id, channel);

CREATE INDEX notification_deliveries_due_idx ON core.notification_deliveries (tenant_id, available_at, created_at)
WHERE
    status = 'pending'
    AND channel <> 'in_app';

ALTER TABLE core.notification_dispatches
    DROP COLUMN addressees,
    DROP COLUMN mode;

DROP TABLE IF EXISTS core.notification_notices;

DROP TABLE IF EXISTS extruntime.schema_migrations_history;

DROP TABLE IF EXISTS extruntime.entity_schema_versions;

DROP TABLE IF EXISTS extruntime.entity_schemas;
