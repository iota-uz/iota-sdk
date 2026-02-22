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

-- Canonical observability graph storage (trace/session/generation/span/event).
CREATE TABLE IF NOT EXISTS bichat.traces (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    tenant_id uuid NOT NULL REFERENCES public.tenants (id) ON DELETE CASCADE,
    session_id uuid NOT NULL REFERENCES bichat.sessions (id) ON DELETE CASCADE,
    message_id uuid REFERENCES bichat.messages (id) ON DELETE SET NULL,
    external_trace_id text NOT NULL,
    trace_url text,
    status varchar(24) NOT NULL DEFAULT 'completed',
    generation_ms bigint NOT NULL DEFAULT 0,
    thinking text,
    observation_reason text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW(),
    CONSTRAINT bichat_traces_status_check CHECK (status IN ('running', 'completed', 'error', 'interrupted'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bichat_traces_external_id
    ON bichat.traces (tenant_id, external_trace_id);

CREATE INDEX IF NOT EXISTS idx_bichat_traces_session
    ON bichat.traces (tenant_id, session_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_bichat_traces_message
    ON bichat.traces (message_id)
    WHERE message_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS bichat.generations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    tenant_id uuid NOT NULL REFERENCES public.tenants (id) ON DELETE CASCADE,
    trace_ref_id uuid NOT NULL REFERENCES bichat.traces (id) ON DELETE CASCADE,
    external_generation_id text NOT NULL,
    request_id text,
    model varchar(255),
    provider varchar(100),
    finish_reason varchar(64),
    prompt_tokens integer NOT NULL DEFAULT 0,
    completion_tokens integer NOT NULL DEFAULT 0,
    total_tokens integer NOT NULL DEFAULT 0,
    cached_tokens integer NOT NULL DEFAULT 0,
    cost numeric(18, 8) NOT NULL DEFAULT 0,
    latency_ms bigint NOT NULL DEFAULT 0,
    input_text text,
    output_text text,
    thinking text,
    observation_reason text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bichat_generations_external_id
    ON bichat.generations (tenant_id, trace_ref_id, external_generation_id);

CREATE INDEX IF NOT EXISTS idx_bichat_generations_request
    ON bichat.generations (tenant_id, request_id)
    WHERE request_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS bichat.spans (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    tenant_id uuid NOT NULL REFERENCES public.tenants (id) ON DELETE CASCADE,
    trace_ref_id uuid NOT NULL REFERENCES bichat.traces (id) ON DELETE CASCADE,
    external_span_id text NOT NULL,
    parent_external_span_id text,
    generation_external_id text,
    name varchar(255) NOT NULL,
    type varchar(64) NOT NULL DEFAULT 'span',
    status varchar(32) NOT NULL DEFAULT 'success',
    level varchar(16),
    call_id text,
    tool_name varchar(255),
    input_text text,
    output_text text,
    error_text text,
    duration_ms bigint NOT NULL DEFAULT 0,
    attributes jsonb NOT NULL DEFAULT '{}'::jsonb,
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bichat_spans_external_id
    ON bichat.spans (tenant_id, trace_ref_id, external_span_id);

CREATE INDEX IF NOT EXISTS idx_bichat_spans_call_id
    ON bichat.spans (tenant_id, call_id)
    WHERE call_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS bichat.events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    tenant_id uuid NOT NULL REFERENCES public.tenants (id) ON DELETE CASCADE,
    trace_ref_id uuid NOT NULL REFERENCES bichat.traces (id) ON DELETE CASCADE,
    external_event_id text NOT NULL,
    name varchar(255) NOT NULL,
    type varchar(64) NOT NULL DEFAULT 'event',
    level varchar(16),
    message text,
    reason text,
    span_external_id text,
    generation_external_id text,
    attributes jsonb NOT NULL DEFAULT '{}'::jsonb,
    timestamp timestamptz,
    created_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bichat_events_external_id
    ON bichat.events (tenant_id, trace_ref_id, external_event_id);

CREATE INDEX IF NOT EXISTS idx_bichat_events_trace_timestamp
    ON bichat.events (tenant_id, trace_ref_id, created_at DESC);

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

DROP INDEX IF EXISTS bichat.idx_bichat_events_trace_timestamp;
DROP INDEX IF EXISTS bichat.idx_bichat_events_external_id;
DROP TABLE IF EXISTS bichat.events;

DROP INDEX IF EXISTS bichat.idx_bichat_spans_call_id;
DROP INDEX IF EXISTS bichat.idx_bichat_spans_external_id;
DROP TABLE IF EXISTS bichat.spans;

DROP INDEX IF EXISTS bichat.idx_bichat_generations_request;
DROP INDEX IF EXISTS bichat.idx_bichat_generations_external_id;
DROP TABLE IF EXISTS bichat.generations;

DROP INDEX IF EXISTS bichat.idx_bichat_traces_message;
DROP INDEX IF EXISTS bichat.idx_bichat_traces_session;
DROP INDEX IF EXISTS bichat.idx_bichat_traces_external_id;
DROP TABLE IF EXISTS bichat.traces;
