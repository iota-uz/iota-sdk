-- +migrate Up
-- BiChat module tables for multi-tenant chat sessions, messages, attachments, and HITL checkpoints

-- Sessions table
CREATE TABLE IF NOT EXISTS bichat_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    pinned BOOLEAN NOT NULL DEFAULT false,
    parent_session_id UUID REFERENCES bichat_sessions(id) ON DELETE SET NULL,
    pending_question_agent VARCHAR(100),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT bichat_sessions_status_check CHECK (status IN ('active', 'archived'))
);

-- Messages table
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

-- Attachments table
CREATE TABLE IF NOT EXISTS bichat_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES bichat_messages(id) ON DELETE CASCADE,
    file_name VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL,
    storage_path TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Checkpoints table
CREATE TABLE IF NOT EXISTS bichat_checkpoints (
    id VARCHAR(255) PRIMARY KEY,
    thread_id VARCHAR(255) NOT NULL,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    agent_name VARCHAR(100) NOT NULL,
    messages JSONB NOT NULL,
    pending_tools JSONB NOT NULL,
    interrupt_type VARCHAR(100) NOT NULL,
    interrupt_data JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW() + INTERVAL '24 hours'
);

-- Indexes for sessions
CREATE INDEX idx_bichat_sessions_tenant_user ON bichat_sessions(tenant_id, user_id);
CREATE INDEX idx_bichat_sessions_tenant_id ON bichat_sessions(tenant_id, id);
CREATE INDEX idx_bichat_sessions_status ON bichat_sessions(status);
CREATE INDEX idx_bichat_sessions_created_at ON bichat_sessions(created_at DESC);
CREATE INDEX idx_bichat_sessions_pinned ON bichat_sessions(pinned) WHERE pinned = true;
CREATE INDEX idx_bichat_sessions_parent ON bichat_sessions(parent_session_id) WHERE parent_session_id IS NOT NULL;

-- Indexes for messages
CREATE INDEX idx_bichat_messages_session ON bichat_messages(session_id, created_at DESC);
CREATE INDEX idx_bichat_messages_created_at ON bichat_messages(created_at);
CREATE INDEX idx_bichat_messages_role ON bichat_messages(role);
CREATE INDEX idx_bichat_messages_tool_call ON bichat_messages(tool_call_id) WHERE tool_call_id IS NOT NULL;

-- Indexes for attachments
CREATE INDEX idx_bichat_attachments_message ON bichat_attachments(message_id);
CREATE INDEX idx_bichat_attachments_created_at ON bichat_attachments(created_at);

-- Indexes for checkpoints
CREATE INDEX idx_bichat_checkpoints_thread ON bichat_checkpoints(thread_id);
CREATE INDEX idx_bichat_checkpoints_tenant_user ON bichat_checkpoints(tenant_id, user_id);
CREATE INDEX idx_bichat_checkpoints_expires ON bichat_checkpoints(expires_at);

-- Add session_id column to bichat_checkpoints for multi-tenant isolation and session tracking
ALTER TABLE bichat_checkpoints
ADD COLUMN session_id UUID;

-- Add index for session_id lookups
CREATE INDEX idx_bichat_checkpoints_session ON bichat_checkpoints(session_id) WHERE session_id IS NOT NULL;

-- +migrate StatementBegin
-- Create analytics schema for BiChat query executor
-- This schema contains denormalized views with automatic tenant isolation
CREATE SCHEMA IF NOT EXISTS analytics;

-- Create bichat_agent_role with restricted permissions (SELECT only)
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'bichat_agent_role') THEN
        CREATE ROLE bichat_agent_role;
    END IF;
END
$$;

-- Grant USAGE on analytics schema only
GRANT USAGE ON SCHEMA analytics TO bichat_agent_role;

-- Grant SELECT on all current and future tables/views in analytics schema
GRANT SELECT ON ALL TABLES IN SCHEMA analytics TO bichat_agent_role;
ALTER DEFAULT PRIVILEGES IN SCHEMA analytics GRANT SELECT ON TABLES TO bichat_agent_role;

-- Revoke all permissions on public schema and other schemas
REVOKE ALL ON SCHEMA public FROM bichat_agent_role;
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM bichat_agent_role;

-- Block access to system schemas
REVOKE ALL ON SCHEMA pg_catalog FROM bichat_agent_role;
REVOKE ALL ON SCHEMA information_schema FROM bichat_agent_role;

-- NOTE: Child projects should create their own denormalized views in the analytics schema
-- Example pattern for child projects:
--
-- CREATE OR REPLACE VIEW analytics.payments_summary AS
-- SELECT
--     id,
--     tenant_id,
--     amount,
--     status,
--     created_at
-- FROM public.payments
-- WHERE tenant_id = current_setting('app.tenant_id', true)::UUID;
--
-- CREATE OR REPLACE VIEW analytics.clients_overview AS
-- SELECT
--     id,
--     tenant_id,
--     name,
--     email,
--     total_orders
-- FROM public.clients
-- WHERE tenant_id = current_setting('app.tenant_id', true)::UUID;

-- Placeholder structure for child projects to extend
COMMENT ON SCHEMA analytics IS 'Denormalized views for BiChat query executor. Child projects should create tenant-isolated views here using current_setting(''app.tenant_id'', true)::UUID pattern.';
-- +migrate StatementEnd

-- +migrate Down
-- +migrate StatementBegin
-- Drop analytics schema and bichat_agent_role
DROP SCHEMA IF EXISTS analytics CASCADE;
DROP ROLE IF EXISTS bichat_agent_role;
-- +migrate StatementEnd

-- Remove session_id column and index
DROP INDEX IF EXISTS idx_bichat_checkpoints_session;
ALTER TABLE bichat_checkpoints
DROP COLUMN IF EXISTS session_id;

DROP TABLE IF EXISTS bichat_checkpoints CASCADE;
DROP TABLE IF EXISTS bichat_attachments CASCADE;
DROP TABLE IF EXISTS bichat_messages CASCADE;
DROP TABLE IF EXISTS bichat_sessions CASCADE;
