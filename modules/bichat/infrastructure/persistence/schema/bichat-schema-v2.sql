-- BI-Chat Schema v2
-- This schema replaces the legacy dialogues table with a proper sessions/messages structure
-- TODO: This migration will be applied when Phase 1 (Agent Framework) is complete

-- Drop legacy tables (if migrating from v1)
-- DROP TABLE IF EXISTS dialogues CASCADE;
-- DROP TABLE IF EXISTS prompts CASCADE;

-- Sessions table for chat conversations
CREATE TABLE IF NOT EXISTS bichat_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    pinned BOOLEAN NOT NULL DEFAULT false,
    parent_session_id UUID REFERENCES bichat_sessions(id) ON DELETE SET NULL,
    pending_question_agent VARCHAR(100),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT bichat_sessions_status_check CHECK (status IN ('active', 'archived'))
);

-- Messages table for individual messages in sessions
CREATE TABLE IF NOT EXISTS bichat_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES bichat_sessions(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    tool_calls JSONB,
    tool_call_id VARCHAR(255),
    citations JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT bichat_messages_role_check CHECK (role IN ('user', 'assistant', 'tool', 'system'))
);

-- Attachments table for file attachments
CREATE TABLE IF NOT EXISTS bichat_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES bichat_messages(id) ON DELETE CASCADE,
    file_name VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL,
    storage_path TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Checkpoints table for HITL (Human-in-the-Loop) state persistence
CREATE TABLE IF NOT EXISTS bichat_checkpoints (
    id VARCHAR(255) PRIMARY KEY,
    thread_id VARCHAR(255) NOT NULL,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    agent_name VARCHAR(100) NOT NULL,
    messages JSONB NOT NULL,
    pending_tools JSONB NOT NULL,
    interrupt_type VARCHAR(100) NOT NULL,
    interrupt_data JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW() + INTERVAL '24 hours'
);

-- Indexes for sessions
CREATE INDEX IF NOT EXISTS idx_bichat_sessions_tenant_user
    ON bichat_sessions(tenant_id, user_id);

CREATE INDEX IF NOT EXISTS idx_bichat_sessions_status
    ON bichat_sessions(status);

CREATE INDEX IF NOT EXISTS idx_bichat_sessions_created_at
    ON bichat_sessions(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_bichat_sessions_pinned
    ON bichat_sessions(pinned) WHERE pinned = true;

CREATE INDEX IF NOT EXISTS idx_bichat_sessions_parent
    ON bichat_sessions(parent_session_id) WHERE parent_session_id IS NOT NULL;

-- Indexes for messages
CREATE INDEX IF NOT EXISTS idx_bichat_messages_session
    ON bichat_messages(session_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_bichat_messages_role
    ON bichat_messages(role);

CREATE INDEX IF NOT EXISTS idx_bichat_messages_tool_call
    ON bichat_messages(tool_call_id) WHERE tool_call_id IS NOT NULL;

-- Indexes for attachments
CREATE INDEX IF NOT EXISTS idx_bichat_attachments_message
    ON bichat_attachments(message_id);

-- Indexes for checkpoints
CREATE INDEX IF NOT EXISTS idx_bichat_checkpoints_thread
    ON bichat_checkpoints(thread_id);

CREATE INDEX IF NOT EXISTS idx_bichat_checkpoints_tenant_user
    ON bichat_checkpoints(tenant_id, user_id);

CREATE INDEX IF NOT EXISTS idx_bichat_checkpoints_expires
    ON bichat_checkpoints(expires_at);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_bichat_session_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger for sessions updated_at
DROP TRIGGER IF EXISTS trigger_bichat_sessions_updated_at ON bichat_sessions;
CREATE TRIGGER trigger_bichat_sessions_updated_at
    BEFORE UPDATE ON bichat_sessions
    FOR EACH ROW
    EXECUTE FUNCTION update_bichat_session_updated_at();

-- Function to cleanup expired checkpoints
CREATE OR REPLACE FUNCTION cleanup_expired_bichat_checkpoints()
RETURNS void AS $$
BEGIN
    DELETE FROM bichat_checkpoints WHERE expires_at < NOW();
END;
$$ LANGUAGE plpgsql;

-- Comments for documentation
COMMENT ON TABLE bichat_sessions IS 'Chat sessions with multi-tenant support';
COMMENT ON TABLE bichat_messages IS 'Messages within chat sessions';
COMMENT ON TABLE bichat_attachments IS 'File attachments for messages';
COMMENT ON TABLE bichat_checkpoints IS 'HITL checkpoints for resumable execution';

COMMENT ON COLUMN bichat_sessions.pending_question_agent IS 'Agent name waiting for user answer (HITL)';
COMMENT ON COLUMN bichat_messages.tool_calls IS 'JSON array of tool calls made by assistant';
COMMENT ON COLUMN bichat_messages.tool_call_id IS 'Reference to tool call this message responds to';
COMMENT ON COLUMN bichat_messages.citations IS 'JSON array of source citations';
COMMENT ON COLUMN bichat_checkpoints.thread_id IS 'Session or conversation identifier for checkpoint continuity';
COMMENT ON COLUMN bichat_checkpoints.interrupt_data IS 'Handler-specific interrupt data (e.g., questions)';
