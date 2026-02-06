-- BiChat module schema source-of-truth for schema collection.
-- Uses schema-qualified table names to match repositories and runtime migrations.

CREATE SCHEMA IF NOT EXISTS bichat;

CREATE TABLE IF NOT EXISTS bichat.sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    user_id bigint NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    title varchar(255) NOT NULL DEFAULT '',
    status varchar(20) NOT NULL DEFAULT 'ACTIVE',
    pinned boolean NOT NULL DEFAULT FALSE,
    parent_session_id uuid REFERENCES bichat.sessions(id) ON DELETE SET NULL,
    pending_question_agent varchar(100),
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW(),
    CONSTRAINT sessions_status_check CHECK (status IN ('ACTIVE', 'ARCHIVED'))
);

CREATE TABLE IF NOT EXISTS bichat.messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id uuid NOT NULL REFERENCES bichat.sessions(id) ON DELETE CASCADE,
    role varchar(20) NOT NULL,
    content text NOT NULL,
    tool_calls jsonb,
    tool_call_id varchar(255),
    citations jsonb,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    CONSTRAINT messages_role_check CHECK (role IN ('user', 'assistant', 'tool', 'system'))
);

CREATE TABLE IF NOT EXISTS bichat.attachments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id uuid NOT NULL REFERENCES bichat.messages(id) ON DELETE CASCADE,
    file_name varchar(255) NOT NULL,
    mime_type varchar(100) NOT NULL,
    size_bytes bigint NOT NULL,
    storage_path text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bichat.checkpoints (
    id varchar(255) PRIMARY KEY,
    thread_id varchar(255) NOT NULL,
    tenant_id uuid NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    user_id bigint NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    agent_name varchar(100) NOT NULL,
    messages jsonb NOT NULL,
    pending_tools jsonb NOT NULL,
    interrupt_type varchar(100) NOT NULL,
    interrupt_data jsonb,
    session_id uuid REFERENCES bichat.sessions(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    expires_at timestamptz NOT NULL DEFAULT NOW() + interval '24 hours'
);

CREATE TABLE IF NOT EXISTS bichat.code_interpreter_outputs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id uuid NOT NULL REFERENCES bichat.messages(id) ON DELETE CASCADE,
    name varchar(255) NOT NULL,
    mime_type varchar(100) NOT NULL,
    url text NOT NULL,
    size_bytes bigint NOT NULL,
    created_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bichat.artifacts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    session_id uuid NOT NULL REFERENCES bichat.sessions(id) ON DELETE CASCADE,
    message_id uuid REFERENCES bichat.messages(id) ON DELETE SET NULL,
    type varchar(50) NOT NULL,
    name varchar(255) NOT NULL,
    description text,
    mime_type varchar(100),
    url text,
    size_bytes bigint DEFAULT 0,
    metadata jsonb DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bichat.learnings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    category text NOT NULL DEFAULT 'sql_error',
    trigger_text text NOT NULL,
    lesson text NOT NULL,
    table_name text,
    sql_patch text,
    used_count integer NOT NULL DEFAULT 0,
    content_hash text GENERATED ALWAYS AS (
        md5(category || ':' || trigger_text || ':' || COALESCE(table_name, ''))
    ) STORED,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    CONSTRAINT learnings_category_check CHECK (
        category IN ('sql_error', 'type_mismatch', 'user_correction', 'business_rule')
    )
);

CREATE TABLE IF NOT EXISTS bichat.validated_queries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    question text NOT NULL,
    sql text NOT NULL,
    summary text NOT NULL,
    tables_used text[] NOT NULL DEFAULT '{}',
    data_quality_notes text[] DEFAULT '{}',
    used_count integer NOT NULL DEFAULT 0,
    sql_hash text GENERATED ALWAYS AS (md5(sql)) STORED,
    created_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sessions_tenant_user ON bichat.sessions(tenant_id, user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_tenant_id ON bichat.sessions(tenant_id, id);
CREATE INDEX IF NOT EXISTS idx_sessions_user_status ON bichat.sessions(user_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_status ON bichat.sessions(status);
CREATE INDEX IF NOT EXISTS idx_sessions_created_at ON bichat.sessions(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_status_pinned ON bichat.sessions(status, pinned, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_pinned ON bichat.sessions(pinned) WHERE pinned = TRUE;
CREATE INDEX IF NOT EXISTS idx_sessions_parent ON bichat.sessions(parent_session_id) WHERE parent_session_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_messages_session ON bichat.messages(session_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON bichat.messages(created_at);
CREATE INDEX IF NOT EXISTS idx_messages_role ON bichat.messages(role);
CREATE INDEX IF NOT EXISTS idx_messages_tool_call ON bichat.messages(tool_call_id) WHERE tool_call_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_attachments_message ON bichat.attachments(message_id);
CREATE INDEX IF NOT EXISTS idx_attachments_created_at ON bichat.attachments(created_at);

CREATE INDEX IF NOT EXISTS idx_checkpoints_thread ON bichat.checkpoints(thread_id);
CREATE INDEX IF NOT EXISTS idx_checkpoints_tenant_user ON bichat.checkpoints(tenant_id, user_id);
CREATE INDEX IF NOT EXISTS idx_checkpoints_expires ON bichat.checkpoints(expires_at);
CREATE INDEX IF NOT EXISTS idx_checkpoints_session ON bichat.checkpoints(session_id) WHERE session_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_code_outputs_message ON bichat.code_interpreter_outputs(message_id);
CREATE INDEX IF NOT EXISTS idx_code_outputs_created_at ON bichat.code_interpreter_outputs(created_at);

CREATE INDEX IF NOT EXISTS idx_artifacts_session ON bichat.artifacts(session_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_artifacts_tenant ON bichat.artifacts(tenant_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_type ON bichat.artifacts(type);
CREATE INDEX IF NOT EXISTS idx_artifacts_message ON bichat.artifacts(message_id) WHERE message_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_bichat_learnings_tenant ON bichat.learnings(tenant_id);
CREATE INDEX IF NOT EXISTS idx_bichat_learnings_table ON bichat.learnings(tenant_id, table_name);
CREATE INDEX IF NOT EXISTS idx_bichat_learnings_category ON bichat.learnings(tenant_id, category);
CREATE INDEX IF NOT EXISTS idx_bichat_learnings_fts ON bichat.learnings
    USING GIN (to_tsvector('english', trigger_text || ' ' || lesson));
CREATE UNIQUE INDEX IF NOT EXISTS idx_bichat_learnings_dedup ON bichat.learnings(tenant_id, content_hash);

CREATE INDEX IF NOT EXISTS idx_bichat_vq_tenant ON bichat.validated_queries(tenant_id);
CREATE INDEX IF NOT EXISTS idx_bichat_vq_tables ON bichat.validated_queries USING GIN (tables_used);
CREATE INDEX IF NOT EXISTS idx_bichat_vq_fts ON bichat.validated_queries
    USING GIN (to_tsvector('english', question || ' ' || summary));
CREATE UNIQUE INDEX IF NOT EXISTS idx_bichat_vq_dedup ON bichat.validated_queries(tenant_id, sql_hash);

COMMENT ON TABLE bichat.sessions IS 'Chat sessions with multi-tenant support';
COMMENT ON TABLE bichat.messages IS 'Messages within chat sessions';
COMMENT ON TABLE bichat.attachments IS 'File attachments for messages';
COMMENT ON TABLE bichat.checkpoints IS 'HITL checkpoints for resumable execution';
COMMENT ON TABLE bichat.code_interpreter_outputs IS 'Output artifacts generated by code interpreter';
COMMENT ON TABLE bichat.artifacts IS 'Generic artifact storage for session outputs';
COMMENT ON TABLE bichat.learnings IS 'Agent-captured learnings from SQL errors and user corrections';
COMMENT ON TABLE bichat.validated_queries IS 'Proven SQL query patterns that answered prior questions';
