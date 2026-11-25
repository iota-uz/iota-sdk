-- The tenant secrets of automations (the built-in «Secret» connector, #1418). A record keeps the name and
-- the reference to the value plus safe metadata; the value lives behind the storage port
-- (pkg/automation/secrets): secret_values is the sealed adapter's table (AES-256-GCM, the key ring of the
-- connections), an OpenBao KV v2 adapter (#1382) replaces it without touching the records.
-- +migrate Up
CREATE TABLE automation.secret_values (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    path text NOT NULL,
    version integer NOT NULL CHECK (version >= 1),
    key_id text NOT NULL,
    nonce bytea NOT NULL,
    ciphertext bytea NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, path)
);

CREATE INDEX secret_values_key_idx ON automation.secret_values (key_id);

CREATE TABLE automation.secrets (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name text NOT NULL CHECK (name ~ '^[A-Z][A-Z0-9_]{0,63}$'),
    description text NOT NULL DEFAULT '',
    ref_path text NOT NULL,
    version integer NOT NULL DEFAULT 1,
    value_version integer NOT NULL DEFAULT 1,
    created_by bigint NOT NULL DEFAULT 0,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    rotated_at timestamp with time zone NOT NULL DEFAULT now(),
    last_used_at timestamp with time zone,
    UNIQUE (tenant_id, name)
);

-- +migrate Down
DROP TABLE IF EXISTS automation.secrets;

DROP TABLE IF EXISTS automation.secret_values;
