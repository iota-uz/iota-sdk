-- +migrate Up
-- Extension runtime management: packages, draft trees, immutable versions,
-- artifacts, reviews, activations, trigger owners, kit snapshots, audit.
-- Everything here is application-wide. trigger_owners (per tenant) belongs to the events migration.
CREATE SCHEMA IF NOT EXISTS extruntime;

CREATE TABLE extruntime.packages (
    namespace text PRIMARY KEY CHECK (namespace ~ '^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$'),
    npm_name text NOT NULL DEFAULT '',
    provenance text NOT NULL CHECK (provenance IN ('git', 'runtime')),
    repo_url text NOT NULL DEFAULT '',
    repo_commit text NOT NULL DEFAULT '',
    -- tree hash of the last imported tree; a draft whose tree still equals it
    -- publishes with git provenance, an edited one with runtime provenance
    imported_tree_hash text NOT NULL DEFAULT '',
    draft_rev bigint NOT NULL DEFAULT 0,
    created_by bigint NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by bigint NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (provenance <> 'git' OR (repo_url <> '' AND repo_commit <> ''))
);

CREATE TABLE extruntime.package_files (
    namespace text NOT NULL REFERENCES extruntime.packages (namespace) ON DELETE CASCADE,
    path text NOT NULL,
    content bytea NOT NULL,
    hash text NOT NULL,
    updated_by bigint NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (namespace, path)
);

-- Content-addressed blobs: tar archives of source trees and of built
-- artifact trees. Immutable.
CREATE TABLE extruntime.artifacts (
    hash text PRIMARY KEY,
    kind text NOT NULL CHECK (kind IN ('source', 'build')),
    content bytea NOT NULL,
    size bigint NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Developer kit snapshots (contract resolution snapshots). Immutable.
CREATE TABLE extruntime.catalog_snapshots (
    hash text PRIMARY KEY,
    content bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE extruntime.versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    namespace text NOT NULL REFERENCES extruntime.packages (namespace),
    version text NOT NULL CHECK (version <> ''),
    source_hash text NOT NULL,
    tree_hash text NOT NULL,
    source_archive text NOT NULL REFERENCES extruntime.artifacts (hash),
    artifact_hash text NOT NULL REFERENCES extruntime.artifacts (hash),
    lockfile_name text NOT NULL DEFAULT '',
    lockfile_hash text NOT NULL DEFAULT '',
    manifest jsonb NOT NULL,
    kit_hash text NOT NULL REFERENCES extruntime.catalog_snapshots (hash),
    provenance text NOT NULL CHECK (provenance IN ('git', 'runtime')),
    repo_url text NOT NULL DEFAULT '',
    repo_commit text NOT NULL DEFAULT '',
    diagnostics jsonb NOT NULL DEFAULT '[]'::jsonb,
    -- contract hashes of the foreign contracts the version implements, as built
    foreign_contracts jsonb NOT NULL DEFAULT '{}'::jsonb,
    author_id bigint NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (namespace, version)
);

CREATE INDEX versions_namespace_created_idx ON extruntime.versions (namespace, created_at DESC);

-- Reviews are append-only and bound to the exact hashes that were reviewed.
CREATE TABLE extruntime.version_reviews (
    id bigserial PRIMARY KEY,
    version_id uuid NOT NULL REFERENCES extruntime.versions (id),
    source_hash text NOT NULL,
    artifact_hash text NOT NULL,
    reviewer_id bigint NOT NULL,
    approve boolean NOT NULL,
    comment text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX version_reviews_version_idx ON extruntime.version_reviews (version_id, id DESC);

-- +migrate StatementBegin
CREATE FUNCTION extruntime.forbid_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'extruntime.% rows are immutable', TG_TABLE_NAME USING ERRCODE = 'restrict_violation';
END;
$$;
-- +migrate StatementEnd

CREATE TRIGGER versions_immutable BEFORE UPDATE OR DELETE ON extruntime.versions
FOR EACH ROW EXECUTE FUNCTION extruntime.forbid_mutation();

CREATE TRIGGER artifacts_immutable BEFORE UPDATE OR DELETE ON extruntime.artifacts
FOR EACH ROW EXECUTE FUNCTION extruntime.forbid_mutation();

CREATE TRIGGER catalog_snapshots_immutable BEFORE UPDATE OR DELETE ON extruntime.catalog_snapshots
FOR EACH ROW EXECUTE FUNCTION extruntime.forbid_mutation();

CREATE TRIGGER version_reviews_immutable BEFORE UPDATE OR DELETE ON extruntime.version_reviews
FOR EACH ROW EXECUTE FUNCTION extruntime.forbid_mutation();

-- Activation history. Application-wide; at most one active row per package.
CREATE TABLE extruntime.activations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    namespace text NOT NULL REFERENCES extruntime.packages (namespace),
    version_id uuid NOT NULL REFERENCES extruntime.versions (id),
    state text NOT NULL CHECK (state IN ('active', 'disabled', 'purged')),
    activated_by bigint NOT NULL,
    activated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ended_by bigint,
    ended_at timestamptz,
    -- DDL statements applied by this activation (empty when the schema did not change)
    schema_steps jsonb NOT NULL DEFAULT '[]'::jsonb
);

CREATE UNIQUE INDEX activations_one_active_idx ON extruntime.activations (namespace) WHERE state = 'active';

CREATE INDEX activations_namespace_idx ON extruntime.activations (namespace, activated_at DESC);

-- What a replacement binding displaced, so deactivating the replacement can
-- reinstate the previous implementation of the slot.
CREATE TABLE extruntime.binding_displaced (
    binding_id uuid PRIMARY KEY REFERENCES extruntime.bindings (id) ON DELETE CASCADE,
    displaced_binding_id uuid NOT NULL REFERENCES extruntime.bindings (id) ON DELETE CASCADE
);

CREATE TABLE extruntime.audit_log (
    id bigserial PRIMARY KEY,
    at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    actor_user_id bigint NOT NULL,
    credential text NOT NULL DEFAULT '',
    action text NOT NULL,
    namespace text NOT NULL DEFAULT '',
    version text NOT NULL DEFAULT '',
    target text NOT NULL DEFAULT '',
    source_hash text NOT NULL DEFAULT '',
    artifact_hash text NOT NULL DEFAULT '',
    outcome text NOT NULL CHECK (outcome IN ('ok', 'failed', 'denied')),
    detail jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX audit_log_at_idx ON extruntime.audit_log (at DESC, id DESC);

CREATE INDEX audit_log_namespace_idx ON extruntime.audit_log (namespace, id DESC);

-- +migrate Down
DROP TABLE IF EXISTS extruntime.audit_log;

DROP TABLE IF EXISTS extruntime.binding_displaced;

DROP TABLE IF EXISTS extruntime.activations;

DROP TABLE IF EXISTS extruntime.version_reviews;

DROP TABLE IF EXISTS extruntime.versions;

DROP TABLE IF EXISTS extruntime.catalog_snapshots;

DROP TABLE IF EXISTS extruntime.artifacts;

DROP TABLE IF EXISTS extruntime.package_files;

DROP TABLE IF EXISTS extruntime.packages;

DROP FUNCTION IF EXISTS extruntime.forbid_mutation();
