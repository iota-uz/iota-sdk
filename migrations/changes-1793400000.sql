-- +migrate Up
-- Extension runtime data layer: applied physical schema and per-version field maps.
CREATE SCHEMA IF NOT EXISTS extruntime;

-- Physical state of every entity/overlay table. Column names are immutable;
-- "columns" lists each column with every field key that ever mapped to it.
CREATE TABLE extruntime.entity_schemas (
    package text NOT NULL,
    entity_id text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('entity', 'overlay')),
    schema_name text NOT NULL,
    table_name text NOT NULL,
    base jsonb,
    columns jsonb NOT NULL,
    version text NOT NULL,
    applied_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (package, entity_id),
    UNIQUE (schema_name, table_name)
);

-- What each package version sees: field key -> physical column (plus the
-- renamedFrom aliases) and the declared types. Immutable once written.
CREATE TABLE extruntime.entity_schema_versions (
    package text NOT NULL,
    version text NOT NULL,
    entity_id text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('entity', 'overlay')),
    schema_name text NOT NULL,
    table_name text NOT NULL,
    base jsonb,
    fields jsonb NOT NULL,
    applied_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (package, version, entity_id)
);

CREATE INDEX entity_schema_versions_applied_idx ON extruntime.entity_schema_versions (package, applied_at DESC);

CREATE TABLE extruntime.schema_migrations_history (
    id bigserial PRIMARY KEY,
    package text NOT NULL,
    from_version text NOT NULL DEFAULT '',
    to_version text NOT NULL,
    adopted boolean NOT NULL DEFAULT FALSE,
    statements jsonb NOT NULL DEFAULT '[]'::jsonb,
    renames jsonb NOT NULL DEFAULT '[]'::jsonb,
    applied_at timestamp with time zone NOT NULL DEFAULT now()
);

CREATE INDEX schema_migrations_history_package_idx ON extruntime.schema_migrations_history (package, id DESC);

-- +migrate Down
DROP TABLE IF EXISTS extruntime.schema_migrations_history;

DROP TABLE IF EXISTS extruntime.entity_schema_versions;

DROP TABLE IF EXISTS extruntime.entity_schemas;
