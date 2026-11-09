-- Migration: Notification channels and per-channel deliveries
-- Date: 2026-11-09
-- Purpose: Let event rules select delivery channels and persist the outcome of every event/recipient/channel.

-- +migrate Up
ALTER TABLE core.notification_rules
    ADD COLUMN channels jsonb NOT NULL DEFAULT '["in_app"]';

CREATE TABLE core.notification_deliveries (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    event_key text NOT NULL,
    event_id text NOT NULL,
    content_version integer NOT NULL DEFAULT 1,
    user_id integer NOT NULL,
    channel text NOT NULL CHECK (channel IN ('in_app', 'email', 'sms', 'push')),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed', 'no_address')),
    language text NOT NULL DEFAULT 'en',
    level text NOT NULL DEFAULT 'info' CHECK (level IN ('info', 'success', 'warning', 'error')),
    title varchar(300) NOT NULL,
    body text NOT NULL DEFAULT '',
    action_url varchar(2000) NOT NULL DEFAULT '',
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error text NOT NULL DEFAULT '',
    provider_receipt text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    available_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    delivered_at timestamptz,
    CONSTRAINT notification_deliveries_user_tenant_fkey FOREIGN KEY (tenant_id, user_id) REFERENCES users (tenant_id, id) ON DELETE CASCADE,
    UNIQUE (tenant_id, event_key, event_id, user_id, channel)
);

CREATE INDEX notification_deliveries_due_idx ON core.notification_deliveries (tenant_id, available_at, created_at)
WHERE
    status = 'pending'
    AND channel <> 'in_app';

-- +migrate Down
DROP TABLE IF EXISTS core.notification_deliveries;

ALTER TABLE core.notification_rules
    DROP COLUMN IF EXISTS channels;
