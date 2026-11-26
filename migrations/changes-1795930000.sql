-- Migration: Open notification channel set, per-tenant channel connections and user channel links
-- Date: 2026-11-25
-- Purpose: Connectors register notification channels (telegram first, #1349): the channel CHECK becomes a
-- format check and the application validates the channel against its registry. A connector-backed channel
-- sends through the connection a tenant chose for it; a linked channel (telegram) reaches a user at the
-- address they linked through a one-use deep link token.

-- +migrate Up
ALTER TABLE core.notification_deliveries
    DROP CONSTRAINT IF EXISTS notification_deliveries_channel_check;

ALTER TABLE core.notification_deliveries
    ADD CONSTRAINT notification_deliveries_channel_check CHECK (channel ~ '^[a-z][a-z0-9_]{1,31}$');

ALTER TABLE core.notification_channel_preferences
    DROP CONSTRAINT IF EXISTS notification_channel_preferences_channel_check;

ALTER TABLE core.notification_channel_preferences
    ADD CONSTRAINT notification_channel_preferences_channel_check CHECK (channel ~ '^[a-z][a-z0-9_]{1,31}$');

-- The connection a connector-backed channel sends through for a tenant: a connection UUID or
-- "platform:<kind>". No row: the platform credential enabled for the tenant, if any.
CREATE TABLE core.notification_channel_connections (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    channel text NOT NULL CHECK (channel ~ '^[a-z][a-z0-9_]{1,31}$'),
    connection_id text NOT NULL CHECK (connection_id <> '' AND length(connection_id) <= 100),
    updated_by integer,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, channel)
);

-- The external account a user linked for a channel (telegram: the chat id of the private chat with the
-- bot). One link per (tenant, user, channel); an address belongs to one user of a tenant per connection.
-- revoked_reason is set when the provider reported the address gone: the row then only tells the user why
-- the link ended.
CREATE TABLE core.notification_channel_links (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id integer NOT NULL,
    channel text NOT NULL CHECK (channel ~ '^[a-z][a-z0-9_]{1,31}$'),
    connection_id text NOT NULL CHECK (connection_id <> ''),
    address text NOT NULL CHECK (address <> '' AND length(address) <= 200),
    display text NOT NULL DEFAULT '' CHECK (length(display) <= 200),
    linked_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_reason text NOT NULL DEFAULT '' CHECK (revoked_reason ~ '^[a-z_]{0,32}$'),
    revoked_at timestamptz,
    CONSTRAINT notification_channel_links_user_tenant_fkey FOREIGN KEY (tenant_id, user_id) REFERENCES users (tenant_id, id) ON DELETE CASCADE,
    PRIMARY KEY (tenant_id, user_id, channel)
);

CREATE UNIQUE INDEX notification_channel_links_address_idx ON core.notification_channel_links (tenant_id, channel, connection_id, address)
WHERE
    revoked_reason = '';

-- One-use link tokens: globally unique by hash (the token itself is never stored), bound to the tenant,
-- the user and the connection the link is made through. used_update_key names the provider update that
-- used the token, so a redelivery of that update gets the same answer.
CREATE TABLE core.notification_channel_link_tokens (
    token_hash text PRIMARY KEY CHECK (token_hash ~ '^[0-9a-f]{64}$'),
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id integer NOT NULL,
    channel text NOT NULL CHECK (channel ~ '^[a-z][a-z0-9_]{1,31}$'),
    connection_id text NOT NULL CHECK (connection_id <> ''),
    language text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    used_update_key text NOT NULL DEFAULT '',
    used_address text NOT NULL DEFAULT '',
    used_display text NOT NULL DEFAULT '',
    CONSTRAINT notification_channel_link_tokens_user_tenant_fkey FOREIGN KEY (tenant_id, user_id) REFERENCES users (tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX notification_channel_link_tokens_user_idx ON core.notification_channel_link_tokens (tenant_id, user_id, channel, created_at DESC);

CREATE INDEX notification_channel_link_tokens_update_idx ON core.notification_channel_link_tokens (used_update_key)
WHERE
    used_update_key <> '';

-- +migrate Down
DROP TABLE IF EXISTS core.notification_channel_link_tokens;

DROP TABLE IF EXISTS core.notification_channel_links;

DROP TABLE IF EXISTS core.notification_channel_connections;

ALTER TABLE core.notification_channel_preferences
    DROP CONSTRAINT IF EXISTS notification_channel_preferences_channel_check;

-- Fails when a row holds a channel other than the built-in four: such rows must be removed first.
ALTER TABLE core.notification_channel_preferences
    ADD CONSTRAINT notification_channel_preferences_channel_check CHECK (channel IN ('in_app', 'email', 'sms', 'push'));

ALTER TABLE core.notification_deliveries
    DROP CONSTRAINT IF EXISTS notification_deliveries_channel_check;

ALTER TABLE core.notification_deliveries
    ADD CONSTRAINT notification_deliveries_channel_check CHECK (channel IN ('in_app', 'email', 'sms', 'push'));
