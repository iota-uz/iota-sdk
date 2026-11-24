-- The connection that holds the token set another connection shares (one email sign-in is a connection to
-- send and a connection to receive): kept in the clear beside the encrypted credentials, so deleting or
-- re-linking a connection finds its dependents without decrypting every connection of the tenant.
-- +migrate Up
ALTER TABLE automation.connections
    ADD COLUMN token_owner uuid;

CREATE INDEX connections_token_owner_idx ON automation.connections (tenant_id, token_owner)
WHERE
    token_owner IS NOT NULL;

-- +migrate Down
DROP INDEX IF EXISTS automation.connections_token_owner_idx;

ALTER TABLE automation.connections
    DROP COLUMN IF EXISTS token_owner;
