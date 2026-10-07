-- +migrate Up
ALTER TABLE core.notifications
    ADD COLUMN level TEXT NOT NULL DEFAULT 'info' CHECK (level IN ('info', 'success', 'warning', 'error'));

ALTER TABLE core.notification_rules
    ADD COLUMN level TEXT NOT NULL DEFAULT '' CHECK (level IN ('', 'info', 'success', 'warning', 'error'));

ALTER TABLE core.notification_rules
    ADD COLUMN recipient_keys JSONB NOT NULL DEFAULT '[]';

ALTER TABLE core.notification_rules
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP;

-- +migrate Down
ALTER TABLE core.notification_rules
    DROP COLUMN created_at,
    DROP COLUMN recipient_keys,
    DROP COLUMN level;

ALTER TABLE core.notifications
    DROP COLUMN level;

