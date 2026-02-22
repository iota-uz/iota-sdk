-- BiChat module schema source-of-truth for schema collection.
-- Uses schema-qualified table names to match repositories and runtime migrations.
CREATE SCHEMA IF NOT EXISTS bichat;

CREATE TABLE IF NOT EXISTS bichat.sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    tenant_id uuid NOT NULL REFERENCES public.tenants (id) ON DELETE CASCADE,
    user_id bigint NOT NULL REFERENCES public.users (id) ON DELETE CASCADE,
    title varchar(255) NOT NULL DEFAULT '',
    status varchar(20) NOT NULL DEFAULT 'ACTIVE',
    pinned boolean NOT NULL DEFAULT FALSE,
    parent_session_id uuid REFERENCES bichat.sessions (id) ON DELETE SET NULL,
    llm_previous_response_id varchar(255),
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW(),
    CONSTRAINT sessions_status_check CHECK (status IN ('ACTIVE', 'ARCHIVED'))
);

CREATE TABLE IF NOT EXISTS bichat.messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    session_id uuid NOT NULL REFERENCES bichat.sessions (id) ON DELETE CASCADE,
    role varchar(20) NOT NULL,
    content text NOT NULL,
    tool_calls jsonb,
    tool_call_id varchar(255),
    citations jsonb,
    debug_trace jsonb,
    question_data jsonb,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    CONSTRAINT messages_role_check CHECK (ROLE IN ('user', 'assistant', 'tool', 'system'))
);

CREATE TABLE IF NOT EXISTS bichat.checkpoints (
    id varchar(255) PRIMARY KEY,
    thread_id varchar(255) NOT NULL,
    tenant_id uuid NOT NULL REFERENCES public.tenants (id) ON DELETE CASCADE,
    user_id bigint NOT NULL REFERENCES public.users (id) ON DELETE CASCADE,
    agent_name varchar(100) NOT NULL,
    messages jsonb NOT NULL,
    pending_tools jsonb NOT NULL,
    interrupt_type varchar(100) NOT NULL,
    interrupt_data jsonb,
    session_id uuid REFERENCES bichat.sessions (id) ON DELETE SET NULL,
    previous_response_id varchar(255),
    created_at timestamptz NOT NULL DEFAULT NOW(),
    expires_at timestamptz NOT NULL DEFAULT NOW() + interval '24 hours'
);

CREATE TABLE IF NOT EXISTS bichat.artifacts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    tenant_id uuid NOT NULL REFERENCES public.tenants (id) ON DELETE CASCADE,
    session_id uuid NOT NULL REFERENCES bichat.sessions (id) ON DELETE CASCADE,
    message_id uuid REFERENCES bichat.messages (id) ON DELETE SET NULL,
    upload_id bigint REFERENCES public.uploads (id) ON DELETE RESTRICT,
    type varchar(50) NOT NULL,
    name varchar(255) NOT NULL,
    description text,
    mime_type varchar(100),
    url text,
    size_bytes bigint DEFAULT 0,
    metadata jsonb DEFAULT '{}',
    status varchar(32) NOT NULL DEFAULT 'available',
    idempotency_key varchar(255),
    created_at timestamptz NOT NULL DEFAULT NOW(),
    CONSTRAINT artifacts_status_check CHECK (status IN ('pending_upload', 'available', 'failed', 'deleted')),
    CONSTRAINT artifacts_attachment_requires_upload CHECK (type <> 'attachment' OR upload_id IS NOT NULL)
);

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

CREATE TABLE IF NOT EXISTS bichat.learnings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    tenant_id uuid NOT NULL REFERENCES public.tenants (id) ON DELETE CASCADE,
    category text NOT NULL DEFAULT 'sql_error',
    trigger_text text NOT NULL,
    lesson text NOT NULL,
    table_name text,
    sql_patch text,
    used_count integer NOT NULL DEFAULT 0,
    content_hash text GENERATED ALWAYS AS (md5(category || ':' || trigger_text || ':' || COALESCE(table_name, ''))) STORED,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    CONSTRAINT learnings_category_check CHECK (category IN ('sql_error', 'type_mismatch', 'user_correction', 'business_rule'))
);

CREATE TABLE IF NOT EXISTS bichat.validated_queries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    tenant_id uuid NOT NULL REFERENCES public.tenants (id) ON DELETE CASCADE,
    question text NOT NULL,
    sql text NOT NULL,
    summary text NOT NULL,
    tables_used text[] NOT NULL DEFAULT '{}',
    data_quality_notes text[] DEFAULT '{}',
    used_count integer NOT NULL DEFAULT 0,
    sql_hash text GENERATED ALWAYS AS (md5(sql)) STORED,
    created_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sessions_tenant_user ON bichat.sessions (tenant_id, user_id);

CREATE INDEX IF NOT EXISTS idx_sessions_tenant_id ON bichat.sessions (tenant_id, id);

CREATE INDEX IF NOT EXISTS idx_sessions_user_status ON bichat.sessions (user_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_sessions_status ON bichat.sessions (status);

CREATE INDEX IF NOT EXISTS idx_sessions_created_at ON bichat.sessions (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_sessions_status_pinned ON bichat.sessions (status, pinned, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_sessions_pinned ON bichat.sessions (pinned)
WHERE
    pinned = TRUE;

CREATE INDEX IF NOT EXISTS idx_sessions_parent ON bichat.sessions (parent_session_id)
WHERE
    parent_session_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_messages_session ON bichat.messages (session_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_messages_created_at ON bichat.messages (created_at);

CREATE INDEX IF NOT EXISTS idx_messages_role ON bichat.messages (ROLE);

CREATE INDEX IF NOT EXISTS idx_messages_tool_call ON bichat.messages (tool_call_id)
WHERE
    tool_call_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_messages_one_pending_question ON bichat.messages (session_id)
WHERE
    question_data ->> 'status' = 'PENDING';

CREATE INDEX IF NOT EXISTS idx_checkpoints_thread ON bichat.checkpoints (thread_id);

CREATE INDEX IF NOT EXISTS idx_checkpoints_tenant_user ON bichat.checkpoints (tenant_id, user_id);

CREATE INDEX IF NOT EXISTS idx_checkpoints_expires ON bichat.checkpoints (expires_at);

CREATE INDEX IF NOT EXISTS idx_checkpoints_session ON bichat.checkpoints (session_id)
WHERE
    session_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_artifacts_session ON bichat.artifacts (session_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_artifacts_tenant ON bichat.artifacts (tenant_id);

CREATE INDEX IF NOT EXISTS idx_artifacts_type ON bichat.artifacts (type);

CREATE INDEX IF NOT EXISTS idx_artifacts_message ON bichat.artifacts (message_id)
WHERE
    message_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_artifacts_upload ON bichat.artifacts (upload_id)
WHERE
    upload_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_artifacts_idempotency ON bichat.artifacts (tenant_id, session_id, idempotency_key)
WHERE
    idempotency_key IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_artifact_provider_files_provider ON bichat.artifact_provider_files (tenant_id, provider, synced_at DESC);

CREATE INDEX IF NOT EXISTS idx_artifact_provider_files_file_id ON bichat.artifact_provider_files (provider_file_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bichat_traces_external_id ON bichat.traces (tenant_id, external_trace_id);

CREATE INDEX IF NOT EXISTS idx_bichat_traces_session ON bichat.traces (tenant_id, session_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_bichat_traces_message ON bichat.traces (message_id)
WHERE
    message_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_bichat_generations_external_id ON bichat.generations (tenant_id, trace_ref_id, external_generation_id);

CREATE INDEX IF NOT EXISTS idx_bichat_generations_request ON bichat.generations (tenant_id, request_id)
WHERE
    request_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_bichat_spans_external_id ON bichat.spans (tenant_id, trace_ref_id, external_span_id);

CREATE INDEX IF NOT EXISTS idx_bichat_spans_call_id ON bichat.spans (tenant_id, call_id)
WHERE
    call_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_bichat_events_external_id ON bichat.events (tenant_id, trace_ref_id, external_event_id);

CREATE INDEX IF NOT EXISTS idx_bichat_events_trace_timestamp ON bichat.events (tenant_id, trace_ref_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_bichat_learnings_tenant ON bichat.learnings (tenant_id);

CREATE INDEX IF NOT EXISTS idx_bichat_learnings_table ON bichat.learnings (tenant_id, table_name);

CREATE INDEX IF NOT EXISTS idx_bichat_learnings_category ON bichat.learnings (tenant_id, category);

CREATE INDEX IF NOT EXISTS idx_bichat_learnings_fts ON bichat.learnings USING GIN (to_tsvector('english', trigger_text || ' ' || lesson));

CREATE UNIQUE INDEX IF NOT EXISTS idx_bichat_learnings_dedup ON bichat.learnings (tenant_id, content_hash);

CREATE INDEX IF NOT EXISTS idx_bichat_vq_tenant ON bichat.validated_queries (tenant_id);

CREATE INDEX IF NOT EXISTS idx_bichat_vq_tables ON bichat.validated_queries USING GIN (tables_used);

CREATE INDEX IF NOT EXISTS idx_bichat_vq_fts ON bichat.validated_queries USING GIN (to_tsvector('english', question || ' ' || summary));

CREATE UNIQUE INDEX IF NOT EXISTS idx_bichat_vq_dedup ON bichat.validated_queries (tenant_id, sql_hash);

COMMENT ON TABLE bichat.sessions IS 'Chat sessions with multi-tenant support';

COMMENT ON TABLE bichat.messages IS 'Messages within chat sessions';

COMMENT ON TABLE bichat.checkpoints IS 'HITL checkpoints for resumable execution';

COMMENT ON TABLE bichat.artifacts IS 'Generic artifact storage for session outputs';

COMMENT ON TABLE bichat.artifact_provider_files IS 'Sync map between local artifacts and provider file IDs for tool runtimes';

COMMENT ON TABLE bichat.traces IS 'Normalized run-scoped trace root for each assistant turn';

COMMENT ON TABLE bichat.generations IS 'LLM generation attempts linked to a trace';

COMMENT ON TABLE bichat.spans IS 'Operational spans (tools/context/agent) linked to a trace';

COMMENT ON TABLE bichat.events IS 'Point-in-time observability events linked to a trace';

COMMENT ON TABLE bichat.learnings IS 'Agent-captured learnings from SQL errors and user corrections';

COMMENT ON TABLE bichat.validated_queries IS 'Proven SQL query patterns that answered prior questions';

