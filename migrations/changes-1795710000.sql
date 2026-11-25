-- The platform credential's reservation of a provider account (a Telegram bot) is a lease (#1439): every instance of
-- the application that runs with the credential renews it, and it is released only once no instance has renewed it
-- for the lease's length. During a rolling update that changes the platform bot, the instances that still run with
-- the previous bot keep its account reserved, so no tenant can connect it (and set its webhook) while they serve it,
-- and a new instance's start does not release it under them. A row written before this column (by an instance of
-- the previous release, which does not renew) holds for ten minutes from its write. Connection rows ignore it.
-- +migrate Up
ALTER TABLE automation.exclusive_identities
    ADD COLUMN lease_until timestamp with time zone NOT NULL DEFAULT now() + interval '10 minutes';

-- A platform reservation whose lease is live is never deleted, whatever deletes it: an instance of the previous
-- release (which, at its start, deletes the platform's reservations of every other account, and all of them when it
-- runs without a platform credential) cannot release the account an instance of this release still serves. An
-- operator's release of all of them (connectors.ReleasePlatformIdentities) sets automation.release_platform.
-- +migrate StatementBegin
CREATE FUNCTION automation.keep_live_platform_leases() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.platform AND OLD.lease_until > clock_timestamp()
        AND coalesce(current_setting('automation.release_platform', true), '') <> 'on' THEN
        RETURN NULL;
    END IF;
    RETURN OLD;
END;
$$;
-- +migrate StatementEnd

CREATE TRIGGER exclusive_identities_keep_live_platform_leases
    BEFORE DELETE ON automation.exclusive_identities
    FOR EACH ROW EXECUTE FUNCTION automation.keep_live_platform_leases();

-- The accounts that stopped being the platform's: a platform setWebhook the provider applied late (from an instance
-- that crashed during the call) may point such an account back to the platform route after a tenant's connection took
-- it. The platform route recognises the stray updates of these accounts and points their webhook back to the
-- connection (engine.PlatformStrayReceiver). Kept a day.
CREATE TABLE automation.retired_platform_identities (
    kind text NOT NULL,
    external text NOT NULL,
    retired_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (kind, external)
);

-- +migrate StatementBegin
CREATE FUNCTION automation.retire_platform_identity() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.platform THEN
        INSERT INTO automation.retired_platform_identities (kind, external) VALUES (OLD.kind, OLD.external)
        ON CONFLICT (kind, external) DO UPDATE SET retired_at = now();
    END IF;
    RETURN OLD;
END;
$$;
-- +migrate StatementEnd

CREATE TRIGGER exclusive_identities_retire_platform_identity
    AFTER DELETE ON automation.exclusive_identities
    FOR EACH ROW EXECUTE FUNCTION automation.retire_platform_identity();

-- +migrate Down
DROP TRIGGER IF EXISTS exclusive_identities_retire_platform_identity ON automation.exclusive_identities;
DROP FUNCTION IF EXISTS automation.retire_platform_identity();
DROP TABLE IF EXISTS automation.retired_platform_identities;
DROP TRIGGER IF EXISTS exclusive_identities_keep_live_platform_leases ON automation.exclusive_identities;
DROP FUNCTION IF EXISTS automation.keep_live_platform_leases();
ALTER TABLE automation.exclusive_identities
    DROP COLUMN IF EXISTS lease_until;
