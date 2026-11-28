-- +migrate Up
-- Automation connections (#1330): the automation schema, encrypted connector credentials of tenants,
-- tenant grants of opt-in platform credentials and connector usage.
CREATE SCHEMA IF NOT EXISTS automation;

-- Credentials are AES-256-GCM sealed with the keyring key key_id; the AAD is tenant_id|id|kind, so a
-- row moved to another tenant or kind does not decrypt.
CREATE TABLE automation.connections (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    id uuid PRIMARY KEY,
    kind text NOT NULL CHECK (kind <> ''),
    name text NOT NULL CHECK (name ~ '^[a-z][a-z0-9_]{0,39}$'),
    display_name text NOT NULL,
    identity jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL CHECK (status IN ('connected', 'needs_reauth', 'error', 'disabled')),
    status_reason text NOT NULL DEFAULT '',
    key_id text NOT NULL,
    nonce bytea NOT NULL,
    secret bytea NOT NULL,
    inbound jsonb NOT NULL DEFAULT '{}'::jsonb,
    version integer NOT NULL CHECK (version >= 1),
    created_by bigint,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, kind, name)
);

-- Rewrap finds rows sealed with an older key.
CREATE INDEX connections_key_id_idx ON automation.connections (key_id);

CREATE TABLE automation.platform_grants (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    kind text NOT NULL,
    enabled boolean NOT NULL,
    enabled_by bigint,
    enabled_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, kind)
);

-- connection_id is a connection UUID or "platform:<kind>", so platform and tenant usage stay apart.
CREATE TABLE automation.connector_usage (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    kind text NOT NULL,
    connection_id text NOT NULL,
    unit text NOT NULL,
    quantity bigint NOT NULL CHECK (quantity > 0),
    run_id uuid,
    at timestamp with time zone NOT NULL DEFAULT now()
);

CREATE INDEX connector_usage_tenant_kind_at_idx ON automation.connector_usage (tenant_id, kind, at);

-- +migrate Down
DROP TABLE IF EXISTS automation.connector_usage;

DROP TABLE IF EXISTS automation.platform_grants;

DROP TABLE IF EXISTS automation.connections;

DROP SCHEMA IF EXISTS automation;
