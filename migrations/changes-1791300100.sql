-- +migrate Up
ALTER TABLE core.notification_rules
    ADD COLUMN group_ids jsonb NOT NULL DEFAULT '[]',
    ADD COLUMN role_ids jsonb NOT NULL DEFAULT '[]';

-- +migrate Down
ALTER TABLE core.notification_rules
    DROP COLUMN group_ids,
    DROP COLUMN role_ids;

