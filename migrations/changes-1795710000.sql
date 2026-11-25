-- The tenant secrets of automations (the built-in «Secret» connector, #1418). A record keeps the name and
-- the reference to the value plus safe metadata; the value lives behind the storage port
-- (pkg/automation/secrets): secret_values is the sealed adapter's table (AES-256-GCM, the key ring of the
-- connections), an OpenBao KV v2 adapter (#1382) replaces it without touching the records.
-- Values are versioned (KV v2): a rotation adds a version and the record publishes it by value_version, so
-- a rotation that fails between the two writes leaves readers on the published version. A record is staged
-- (value_version 0: the name is reserved, nothing reads it) before its first value is written, so recovery
-- tells a published secret from an orphan by its row. A deleted record is a tombstone (deleted_at) until its
-- value is gone, so a failed delete of the value is retried. No transaction is held open while a value is
-- written: the value store may share the records' pool.
-- +migrate Up
CREATE TABLE automation.secret_values (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    path text NOT NULL,
    version integer NOT NULL CHECK (version >= 1),
    key_id text NOT NULL,
    nonce bytea NOT NULL,
    ciphertext bytea NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, path, version)
);

CREATE INDEX secret_values_key_idx ON automation.secret_values (key_id);

CREATE TABLE automation.secrets (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name text NOT NULL CHECK (name ~ '^[A-Z][A-Z0-9_]{0,63}$'),
    description text NOT NULL DEFAULT '',
    ref_path text NOT NULL,
    version integer NOT NULL DEFAULT 1,
    value_version integer NOT NULL DEFAULT 0 CHECK (value_version >= 0),
    created_by bigint NOT NULL DEFAULT 0,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    rotated_at timestamp with time zone NOT NULL DEFAULT now(),
    last_used_at timestamp with time zone,
    deleted_at timestamp with time zone
);

-- A name is unique among the live secrets of a tenant; tombstones keep theirs until purged.
CREATE UNIQUE INDEX secrets_tenant_name_idx ON automation.secrets (tenant_id, name) WHERE deleted_at IS NULL;

CREATE INDEX secrets_deleted_idx ON automation.secrets (tenant_id) WHERE deleted_at IS NOT NULL;

-- +migrate Down
DROP TABLE IF EXISTS automation.secrets;

DROP TABLE IF EXISTS automation.secret_values;
