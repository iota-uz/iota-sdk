-- +migrate Up
-- Exclusive provider identities: a provider account that can deliver to one place only (a Telegram bot has
-- one webhook) belongs to one connection of one tenant, or to the platform credential. The row is written
-- in the transaction that saves the connection, under an advisory lock on the identity, so two saves for
-- the same bot cannot both set the provider up.
CREATE TABLE automation.exclusive_identities (
    kind text NOT NULL,
    external text NOT NULL,
    -- the connection that owns the identity; NULL for the platform credential
    connection_id uuid REFERENCES automation.connections (id) ON DELETE CASCADE,
    platform boolean NOT NULL DEFAULT FALSE,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (kind, external),
    CHECK (platform = (connection_id IS NULL))
);

CREATE INDEX exclusive_identities_connection_idx ON automation.exclusive_identities (connection_id);

-- +migrate Down
DROP TABLE IF EXISTS automation.exclusive_identities;
