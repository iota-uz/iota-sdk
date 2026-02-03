-- BiChat module schema for multi-tenant chat sessions, messages, attachments, and HITL checkpoints
-- This file defines the complete schema structure for the bichat module
-- For migrations, see the migrations/ directory
-- ========================================
-- Tables
-- ========================================
-- Sessions table for chat conversations
CREATE TABLE IF NOT EXISTS bichat_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title varchar(255) NOT NULL DEFAULT '',
    status varchar(20) NOT NULL DEFAULT 'ACTIVE',
    pinned boolean NOT NULL DEFAULT FALSE,
    parent_session_id uuid REFERENCES bichat_sessions (id) ON DELETE SET NULL,
    pending_question_agent varchar(100),
    created_at timestamp with time zone NOT NULL DEFAULT NOW(),
    updated_at timestamp with time zone NOT NULL DEFAULT NOW(),
    CONSTRAINT bichat_sessions_status_check CHECK (status IN ('ACTIVE', 'ARCHIVED'))
);

-- Messages table for individual messages in sessions
CREATE TABLE IF NOT EXISTS bichat_messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    session_id uuid NOT NULL REFERENCES bichat_sessions (id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL,
    content text NOT NULL,
    tool_calls jsonb,
    tool_call_id varchar(255),
    citations jsonb,
    created_at timestamp with time zone NOT NULL DEFAULT NOW(),
    CONSTRAINT bichat_messages_role_check CHECK (ROLE IN ('user', 'assistant', 'tool', 'system'))
);

-- Attachments table for file attachments
CREATE TABLE IF NOT EXISTS bichat_attachments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    message_id uuid NOT NULL REFERENCES bichat_messages (id) ON DELETE CASCADE,
    file_name varchar(255) NOT NULL,
    mime_type varchar(100) NOT NULL,
    size_bytes bigint NOT NULL,
    storage_path text NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT NOW()
);

-- Checkpoints table for HITL (Human-in-the-Loop) state persistence
CREATE TABLE IF NOT EXISTS bichat_checkpoints (
    id varchar(255) PRIMARY KEY,
    thread_id varchar(255) NOT NULL,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    agent_name varchar(100) NOT NULL,
    messages jsonb NOT NULL,
    pending_tools jsonb NOT NULL,
    interrupt_type varchar(100) NOT NULL,
    interrupt_data jsonb,
    created_at timestamp with time zone NOT NULL DEFAULT NOW(),
    expires_at timestamp with time zone NOT NULL DEFAULT NOW() + interval '24 hours'
);

-- ========================================
-- Indexes
-- ========================================
-- Indexes for sessions
CREATE INDEX IF NOT EXISTS idx_bichat_sessions_tenant_user ON bichat_sessions (tenant_id, user_id);

CREATE INDEX IF NOT EXISTS idx_bichat_sessions_tenant_id ON bichat_sessions (tenant_id, id);

CREATE INDEX IF NOT EXISTS idx_bichat_sessions_user_status ON bichat_sessions (user_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_bichat_sessions_status ON bichat_sessions (status);

CREATE INDEX IF NOT EXISTS idx_bichat_sessions_created_at ON bichat_sessions (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_bichat_sessions_status_pinned ON bichat_sessions (status, pinned, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_bichat_sessions_pinned ON bichat_sessions (pinned)
WHERE
    pinned = TRUE;

CREATE INDEX IF NOT EXISTS idx_bichat_sessions_parent ON bichat_sessions (parent_session_id)
WHERE
    parent_session_id IS NOT NULL;

-- Indexes for messages
CREATE INDEX IF NOT EXISTS idx_bichat_messages_session ON bichat_messages (session_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_bichat_messages_created_at ON bichat_messages (created_at);

CREATE INDEX IF NOT EXISTS idx_bichat_messages_role ON bichat_messages (ROLE);

CREATE INDEX IF NOT EXISTS idx_bichat_messages_tool_call ON bichat_messages (tool_call_id)
WHERE
    tool_call_id IS NOT NULL;

-- Indexes for attachments
CREATE INDEX IF NOT EXISTS idx_bichat_attachments_message ON bichat_attachments (message_id);

CREATE INDEX IF NOT EXISTS idx_bichat_attachments_created_at ON bichat_attachments (created_at);

-- Indexes for checkpoints
CREATE INDEX IF NOT EXISTS idx_bichat_checkpoints_thread ON bichat_checkpoints (thread_id);

CREATE INDEX IF NOT EXISTS idx_bichat_checkpoints_tenant_user ON bichat_checkpoints (tenant_id, user_id);

CREATE INDEX IF NOT EXISTS idx_bichat_checkpoints_expires ON bichat_checkpoints (expires_at);

-- ========================================
-- Functions
-- ========================================
-- Function to cleanup expired checkpoints
CREATE OR REPLACE FUNCTION cleanup_expired_bichat_checkpoints ()
    RETURNS void
    AS $$
BEGIN
    DELETE FROM bichat_checkpoints
    WHERE expires_at < NOW();
END;
$$
LANGUAGE plpgsql;

-- ========================================
-- Comments (Documentation)
-- ========================================
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

-- +migrate Down
DROP TABLE IF EXISTS bichat_checkpoints CASCADE;

DROP TABLE IF EXISTS bichat_attachments CASCADE;

DROP TABLE IF EXISTS bichat_messages CASCADE;

DROP TABLE IF EXISTS bichat_sessions CASCADE;

DROP FUNCTION IF EXISTS update_bichat_session_updated_at ();

DROP FUNCTION IF EXISTS cleanup_expired_bichat_checkpoints ();

