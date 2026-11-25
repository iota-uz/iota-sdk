-- The platform credential's reservation of a provider account (a Telegram bot) is a lease (#1439): every instance of
-- the application that runs with the credential renews it, and it is released only once no instance has renewed it
-- for the lease's length. During a rolling update that changes the platform bot, the instances that still run with
-- the previous bot keep its account reserved, so no tenant can connect it (and set its webhook) while they serve it,
-- and a new instance's start does not release it under them. A row written before this column (by an instance of
-- the previous release, which does not renew) holds for ten minutes from its write. Connection rows ignore it.
-- +migrate Up
ALTER TABLE automation.exclusive_identities
    ADD COLUMN lease_until timestamp with time zone NOT NULL DEFAULT now() + interval '10 minutes';

-- +migrate Down
ALTER TABLE automation.exclusive_identities
    DROP COLUMN IF EXISTS lease_until;
