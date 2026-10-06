-- +migrate Up
INSERT INTO permissions (id, name, resource, action, modifier, description)
    VALUES ('c196d78b-c653-4783-af32-87d7a823440b', 'NotificationRules.Read', 'notification_rules', 'read', 'all', ''),
    ('3a72dc29-2048-421b-807b-4307b7080868', 'NotificationRules.Manage', 'notification_rules', 'update', 'all', '')
ON CONFLICT
    DO NOTHING;

CREATE TABLE core.notifications (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id integer NOT NULL,
    event_key varchar(200) NOT NULL DEFAULT '',
    title varchar(300) NOT NULL,
    body text NOT NULL DEFAULT '',
    action_url varchar(2000) NOT NULL DEFAULT '',
    dedupe_key varchar(300),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    read_at timestamptz,
    CONSTRAINT notifications_user_tenant_fkey FOREIGN KEY (tenant_id, user_id) REFERENCES users (tenant_id, id) ON DELETE CASCADE,
    UNIQUE (tenant_id, user_id, dedupe_key)
);

CREATE INDEX notifications_recipient_idx ON core.notifications (tenant_id, user_id, created_at DESC, id DESC);

CREATE INDEX notifications_unread_idx ON core.notifications (tenant_id, user_id)
WHERE
    read_at IS NULL;

CREATE TABLE core.notification_rules (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    event_key text NOT NULL,
    enabled boolean NOT NULL DEFAULT FALSE,
    user_ids jsonb NOT NULL DEFAULT '[]',
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, event_key)
);

-- +migrate Down
DROP TABLE IF EXISTS core.notification_rules;

DROP TABLE IF EXISTS core.notifications;

DELETE FROM permissions
WHERE id IN ('c196d78b-c653-4783-af32-87d7a823440b', '3a72dc29-2048-421b-807b-4307b7080868');

