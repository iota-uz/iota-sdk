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

-- Move attachments to core upload linkage for artifacts.
DELETE FROM bichat.artifacts;

ALTER TABLE IF EXISTS bichat.artifacts
    ADD COLUMN IF NOT EXISTS upload_id bigint REFERENCES public.uploads (id) ON DELETE RESTRICT;

ALTER TABLE IF EXISTS bichat.artifacts
    DROP CONSTRAINT IF EXISTS artifacts_attachment_requires_upload;

ALTER TABLE IF EXISTS bichat.artifacts
    DROP CONSTRAINT IF EXISTS artifacts_attachment_requires_upload_chk;

ALTER TABLE IF EXISTS bichat.artifacts
    ADD CONSTRAINT artifacts_attachment_requires_upload
    CHECK (type <> 'attachment' OR upload_id IS NOT NULL);

CREATE INDEX IF NOT EXISTS idx_artifacts_upload ON bichat.artifacts (upload_id)
    WHERE upload_id IS NOT NULL;

DROP TABLE IF EXISTS bichat.attachments;

-- Drop legacy code interpreter outputs table.
DROP TABLE IF EXISTS bichat.code_interpreter_outputs;

-- Make artifact rows safer for async delivery and deduping.
ALTER TABLE IF EXISTS bichat.artifacts
    ADD COLUMN IF NOT EXISTS status varchar(32) NOT NULL DEFAULT 'available';

ALTER TABLE IF EXISTS bichat.artifacts
    ADD COLUMN IF NOT EXISTS idempotency_key varchar(255);

ALTER TABLE IF EXISTS bichat.artifacts
    DROP CONSTRAINT IF EXISTS artifacts_status_check;

ALTER TABLE IF EXISTS bichat.artifacts
    ADD CONSTRAINT artifacts_status_check CHECK (status IN ('pending_upload', 'available', 'failed', 'deleted'));

CREATE UNIQUE INDEX IF NOT EXISTS idx_artifacts_idempotency
    ON bichat.artifacts(tenant_id, session_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- +migrate Down
CREATE TABLE IF NOT EXISTS bichat.attachments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    message_id uuid NOT NULL REFERENCES bichat.messages (id) ON DELETE CASCADE,
    file_name varchar(255) NOT NULL,
    mime_type varchar(100) NOT NULL,
    size_bytes bigint NOT NULL,
    storage_path text NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_attachments_message ON bichat.attachments (message_id);
CREATE INDEX IF NOT EXISTS idx_attachments_created_at ON bichat.attachments (created_at);

ALTER TABLE IF EXISTS bichat.artifacts
    DROP CONSTRAINT IF EXISTS artifacts_attachment_requires_upload;

ALTER TABLE IF EXISTS bichat.artifacts
    DROP CONSTRAINT IF EXISTS artifacts_attachment_requires_upload_chk;

DROP INDEX IF EXISTS bichat.idx_artifacts_upload;

ALTER TABLE IF EXISTS bichat.artifacts
    DROP COLUMN IF EXISTS upload_id;

DROP INDEX IF EXISTS bichat.idx_artifacts_idempotency;

ALTER TABLE IF EXISTS bichat.artifacts
    DROP CONSTRAINT IF EXISTS artifacts_status_check;

ALTER TABLE IF EXISTS bichat.artifacts
    DROP COLUMN IF EXISTS idempotency_key;

ALTER TABLE IF EXISTS bichat.artifacts
    DROP COLUMN IF EXISTS status;

DROP INDEX IF EXISTS bichat.idx_artifact_provider_files_file_id;
DROP INDEX IF EXISTS bichat.idx_artifact_provider_files_provider;
DROP TABLE IF EXISTS bichat.artifact_provider_files;

CREATE TABLE IF NOT EXISTS bichat.code_interpreter_outputs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    message_id uuid NOT NULL REFERENCES bichat.messages (id) ON DELETE CASCADE,
    name varchar(255) NOT NULL,
    mime_type varchar(100) NOT NULL,
    url text NOT NULL,
    size_bytes bigint NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_code_outputs_message ON bichat.code_interpreter_outputs (message_id);

CREATE INDEX idx_code_outputs_created_at ON bichat.code_interpreter_outputs (created_at);
