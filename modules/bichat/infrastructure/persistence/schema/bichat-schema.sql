-- BiChat module schema for multi-tenant chat sessions, messages, attachments, and HITL checkpoints
-- This file defines the complete schema structure for the bichat module
-- For migrations, see the migrations/ directory

CREATE SCHEMA IF NOT EXISTS bichat;

-- ========================================
-- Tables
-- ========================================

-- Sessions table for chat conversations
CREATE TABLE IF NOT EXISTS bichat.sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    pinned BOOLEAN NOT NULL DEFAULT false,
    parent_session_id UUID REFERENCES bichat.sessions(id) ON DELETE SET NULL,
    pending_question_agent VARCHAR(100),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT sessions_status_check CHECK (status IN ('active', 'archived'))
);

-- Messages table for individual messages in sessions
CREATE TABLE IF NOT EXISTS bichat.messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES bichat.sessions(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    tool_calls JSONB,
    tool_call_id VARCHAR(255),
    citations JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT messages_role_check CHECK (role IN ('user', 'assistant', 'tool', 'system'))
);

-- Attachments table for file attachments
CREATE TABLE IF NOT EXISTS bichat.attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES bichat.messages(id) ON DELETE CASCADE,
    file_name VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL,
    storage_path TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Checkpoints table for HITL (Human-in-the-Loop) state persistence
CREATE TABLE IF NOT EXISTS bichat.checkpoints (
    id VARCHAR(255) PRIMARY KEY,
    thread_id VARCHAR(255) NOT NULL,
    tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    agent_name VARCHAR(100) NOT NULL,
    messages JSONB NOT NULL,
    pending_tools JSONB NOT NULL,
    interrupt_type VARCHAR(100) NOT NULL,
    interrupt_data JSONB,
    session_id UUID REFERENCES bichat.sessions(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW() + INTERVAL '24 hours'
);

-- Code interpreter outputs table
CREATE TABLE IF NOT EXISTS bichat.code_interpreter_outputs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES bichat.messages(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    url TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Artifacts table (generic tool outputs: charts, exports, code outputs)
CREATE TABLE IF NOT EXISTS bichat.artifacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    session_id UUID NOT NULL REFERENCES bichat.sessions(id) ON DELETE CASCADE,
    message_id UUID REFERENCES bichat.messages(id) ON DELETE SET NULL,
    type VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    mime_type VARCHAR(100),
    url TEXT,
    size_bytes BIGINT DEFAULT 0,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- ========================================
-- Indexes
-- ========================================

CREATE INDEX IF NOT EXISTS idx_sessions_tenant_user ON bichat.sessions(tenant_id, user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_tenant_id ON bichat.sessions(tenant_id, id);
CREATE INDEX IF NOT EXISTS idx_sessions_status ON bichat.sessions(status);
CREATE INDEX IF NOT EXISTS idx_sessions_created_at ON bichat.sessions(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_pinned ON bichat.sessions(pinned) WHERE pinned = true;
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

-- ========================================
-- Functions
-- ========================================

CREATE OR REPLACE FUNCTION cleanup_expired_bichat_checkpoints()
RETURNS void AS $$
BEGIN
    DELETE FROM bichat.checkpoints WHERE expires_at < NOW();
END;
$$ LANGUAGE plpgsql;

-- ========================================
-- Comments (Documentation)
-- ========================================

COMMENT ON TABLE bichat.sessions IS 'Chat sessions with multi-tenant support';
COMMENT ON TABLE bichat.messages IS 'Messages within chat sessions';
COMMENT ON TABLE bichat.attachments IS 'File attachments for messages';
COMMENT ON TABLE bichat.checkpoints IS 'HITL checkpoints for resumable execution';

COMMENT ON COLUMN bichat.sessions.pending_question_agent IS 'Agent name waiting for user answer (HITL)';
COMMENT ON COLUMN bichat.messages.tool_calls IS 'JSON array of tool calls made by assistant';
COMMENT ON COLUMN bichat.messages.tool_call_id IS 'Reference to tool call this message responds to';
COMMENT ON COLUMN bichat.messages.citations IS 'JSON array of source citations';
COMMENT ON COLUMN bichat.checkpoints.thread_id IS 'Session or conversation identifier for checkpoint continuity';
COMMENT ON COLUMN bichat.checkpoints.interrupt_data IS 'Handler-specific interrupt data (e.g., questions)';
COMMENT ON TABLE bichat.artifacts IS 'Generic artifact storage for session outputs (charts, exports, code outputs, etc.)';
COMMENT ON COLUMN bichat.artifacts.type IS 'Artifact type (code_output, chart, export, etc.) - extensible';
COMMENT ON COLUMN bichat.artifacts.metadata IS 'Type-specific data as JSONB (chart spec, row counts, etc.)';

-- +migrate Down
DROP FUNCTION IF EXISTS cleanup_expired_bichat_checkpoints();
DROP TABLE IF EXISTS bichat.artifacts CASCADE;
DROP TABLE IF EXISTS bichat.code_interpreter_outputs CASCADE;
DROP TABLE IF EXISTS bichat.checkpoints CASCADE;
DROP TABLE IF EXISTS bichat.attachments CASCADE;
DROP TABLE IF EXISTS bichat.messages CASCADE;
DROP TABLE IF EXISTS bichat.sessions CASCADE;
DROP SCHEMA IF EXISTS bichat CASCADE;
