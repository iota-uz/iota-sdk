-- +migrate Up

CREATE TABLE IF NOT EXISTS bichat.artifact_provider_files (
    tenant_id uuid NOT NULL REFERENCES public.tenants (id) ON DELETE CASCADE,
    artifact_id uuid NOT NULL REFERENCES bichat.artifacts (id) ON DELETE CASCADE,
    provider varchar(50) NOT NULL,
    provider_file_id varchar(255) NOT NULL,
    source_url text NOT NULL,
    source_size_bytes bigint NOT NULL DEFAULT 0,
    synced_at timestamptz NOT NULL DEFAULT NOW(),
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, artifact_id, provider)
);

CREATE INDEX IF NOT EXISTS idx_artifact_provider_files_provider
    ON bichat.artifact_provider_files (tenant_id, provider, synced_at DESC);

CREATE INDEX IF NOT EXISTS idx_artifact_provider_files_file_id
    ON bichat.artifact_provider_files (provider_file_id);

-- +migrate Down

DROP INDEX IF EXISTS bichat.idx_artifact_provider_files_file_id;
DROP INDEX IF EXISTS bichat.idx_artifact_provider_files_provider;
DROP TABLE IF EXISTS bichat.artifact_provider_files;
