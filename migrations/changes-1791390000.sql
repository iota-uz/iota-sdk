-- +migrate Up
ALTER TABLE core.notifications
    ADD COLUMN level TEXT NOT NULL DEFAULT 'info' CHECK (level IN ('info', 'success', 'warning', 'error'));

ALTER TABLE core.notification_rules
    ADD COLUMN level TEXT NOT NULL DEFAULT '' CHECK (level IN ('', 'info', 'success', 'warning', 'error'));

ALTER TABLE core.notification_rules
    ADD COLUMN recipient_keys JSONB NOT NULL DEFAULT '[]';

ALTER TABLE core.notification_rules
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP;

CREATE TABLE core.notification_dispatches (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    event_key text NOT NULL,
    event_id text NOT NULL,
    event jsonb NOT NULL,
    rule jsonb NOT NULL,
    recipient_ids jsonb NOT NULL,
    cursor integer NOT NULL DEFAULT 0 CHECK (CURSOR >= 0),
    delivered integer NOT NULL DEFAULT 0 CHECK (delivered >= 0),
    attempts integer NOT NULL DEFAULT 0,
    last_error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    available_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at timestamptz,
    UNIQUE (tenant_id, event_key, event_id)
);

CREATE INDEX notification_dispatches_ready_idx ON core.notification_dispatches (tenant_id, available_at, created_at)
WHERE
    completed_at IS NULL;

-- +migrate Down
DROP TABLE core.notification_dispatches;

ALTER TABLE core.notification_rules
    DROP COLUMN created_at,
    DROP COLUMN recipient_keys,
    DROP COLUMN level;

ALTER TABLE core.notifications
    DROP COLUMN level;

