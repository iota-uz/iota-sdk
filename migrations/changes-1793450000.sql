-- Migration: Notification recipients, targets and attempts
-- Date: 2026-11-11
-- Purpose: Separate the notice instance, its recipient (user, business record or direct address) and the concrete channel target/device so external customers without an account are first-class recipients; record every provider attempt; add a push device registry.

-- +migrate Up
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

-- +migrate Down
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
