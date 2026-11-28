-- The connection that holds the token set another connection shares (one email sign-in is a connection to
-- send and a connection to receive): kept in the clear beside the encrypted credentials, so deleting or
-- re-linking a connection finds its dependents without decrypting every connection of the tenant. Its
-- index is built concurrently by the next migration.
-- +migrate Up
ALTER TABLE automation.connections
    ADD COLUMN IF NOT EXISTS token_owner uuid;

-- +migrate Down
-- The column stays: the links live in the encrypted credentials (token_from) of the dependents, which a
-- rollback leaves in place, and the column cannot be rebuilt from them in SQL. Dropping it would make a
-- re-upgrade forget which connection owns a shared token set, and the dependent of a deleted owner would
-- lose its tokens. A release that does not know the column ignores it.
SELECT 1;
