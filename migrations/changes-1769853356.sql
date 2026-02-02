-- +migrate Up
-- BiChat module tables for multi-tenant chat sessions, messages, attachments, and HITL checkpoints
-- Drop legacy tables from previous implementation
DROP TABLE IF EXISTS dialogues CASCADE;

DROP TABLE IF EXISTS prompts CASCADE;

-- Create bichat schema
CREATE SCHEMA IF NOT EXISTS bichat;

-- Sessions table
CREATE TABLE IF NOT EXISTS bichat.sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    tenant_id uuid NOT NULL REFERENCES public.tenants (id) ON DELETE CASCADE,
    user_id bigint NOT NULL REFERENCES public.users (id) ON DELETE CASCADE,
    title varchar(255) NOT NULL DEFAULT '',
    status varchar(20) NOT NULL DEFAULT 'active',
    pinned boolean NOT NULL DEFAULT FALSE,
    parent_session_id uuid REFERENCES bichat.sessions (id) ON DELETE SET NULL,
    pending_question_agent varchar(100),
    created_at timestamp with time zone NOT NULL DEFAULT NOW(),
    updated_at timestamp with time zone NOT NULL DEFAULT NOW(),
    CONSTRAINT sessions_status_check CHECK (status IN ('active', 'archived'))
);

-- Messages table
CREATE TABLE IF NOT EXISTS bichat.messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    session_id uuid NOT NULL REFERENCES bichat.sessions (id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL,
    content text NOT NULL,
    tool_calls jsonb,
    tool_call_id varchar(255),
    citations jsonb,
    created_at timestamp with time zone NOT NULL DEFAULT NOW(),
    CONSTRAINT messages_role_check CHECK (ROLE IN ('user', 'assistant', 'tool', 'system'))
);

-- Attachments table
CREATE TABLE IF NOT EXISTS bichat.attachments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    message_id uuid NOT NULL REFERENCES bichat.messages (id) ON DELETE CASCADE,
    file_name varchar(255) NOT NULL,
    mime_type varchar(100) NOT NULL,
    size_bytes bigint NOT NULL,
    storage_path text NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT NOW()
);

-- Checkpoints table
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
    created_at timestamp with time zone NOT NULL DEFAULT NOW(),
    expires_at timestamp with time zone NOT NULL DEFAULT NOW() + interval '24 hours'
);

-- Indexes for sessions
CREATE INDEX idx_sessions_tenant_user ON bichat.sessions (tenant_id, user_id);

CREATE INDEX idx_sessions_tenant_id ON bichat.sessions (tenant_id, id);

CREATE INDEX idx_sessions_status ON bichat.sessions (status);

CREATE INDEX idx_sessions_created_at ON bichat.sessions (created_at DESC);

CREATE INDEX idx_sessions_pinned ON bichat.sessions (pinned)
WHERE
    pinned = TRUE;

CREATE INDEX idx_sessions_parent ON bichat.sessions (parent_session_id)
WHERE
    parent_session_id IS NOT NULL;

-- Indexes for messages
CREATE INDEX idx_messages_session ON bichat.messages (session_id, created_at DESC);

CREATE INDEX idx_messages_created_at ON bichat.messages (created_at);

CREATE INDEX idx_messages_role ON bichat.messages (ROLE);

CREATE INDEX idx_messages_tool_call ON bichat.messages (tool_call_id)
WHERE
    tool_call_id IS NOT NULL;

-- Indexes for attachments
CREATE INDEX idx_attachments_message ON bichat.attachments (message_id);

CREATE INDEX idx_attachments_created_at ON bichat.attachments (created_at);

-- Indexes for checkpoints
CREATE INDEX idx_checkpoints_thread ON bichat.checkpoints (thread_id);

CREATE INDEX idx_checkpoints_tenant_user ON bichat.checkpoints (tenant_id, user_id);

CREATE INDEX idx_checkpoints_expires ON bichat.checkpoints (expires_at);

CREATE INDEX idx_checkpoints_session ON bichat.checkpoints (session_id)
WHERE
    session_id IS NOT NULL;

-- +migrate StatementBegin
-- Create analytics schema for BiChat query executor
-- This schema contains denormalized views with automatic tenant isolation
CREATE SCHEMA IF NOT EXISTS analytics;

-- Create bichat_agent_role with restricted permissions (SELECT only)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT
        FROM
            pg_catalog.pg_roles
        WHERE
            rolname = 'bichat_agent_role') THEN
    CREATE ROLE bichat_agent_role;
END IF;
EXCEPTION
    WHEN duplicate_object THEN
        NULL;
END
$$;

-- Grant USAGE on analytics and bichat schemas
GRANT USAGE ON SCHEMA analytics TO bichat_agent_role;

GRANT USAGE ON SCHEMA bichat TO bichat_agent_role;

-- Grant SELECT on all current and future tables/views in analytics schema
GRANT SELECT ON ALL TABLES IN SCHEMA analytics TO bichat_agent_role;

ALTER DEFAULT PRIVILEGES IN SCHEMA analytics GRANT
SELECT
    ON TABLES TO bichat_agent_role;

-- Grant SELECT on all current and future tables in bichat schema
GRANT SELECT ON ALL TABLES IN SCHEMA bichat TO bichat_agent_role;

ALTER DEFAULT PRIVILEGES IN SCHEMA bichat GRANT
SELECT
    ON TABLES TO bichat_agent_role;

-- Revoke all permissions on public schema and other schemas
REVOKE ALL ON SCHEMA public FROM bichat_agent_role;

REVOKE ALL ON ALL TABLES IN SCHEMA public FROM bichat_agent_role;

-- Block access to system schemas
REVOKE ALL ON SCHEMA pg_catalog FROM bichat_agent_role;

REVOKE ALL ON SCHEMA information_schema FROM bichat_agent_role;

-- Create denormalized views for all tenant-scoped tables
-- Public schema tables
CREATE OR REPLACE VIEW analytics.clients AS
SELECT
    *
FROM
    public.clients
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.counterparty AS
SELECT
    *
FROM
    public.counterparty
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.employees AS
SELECT
    *
FROM
    public.employees
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.users AS
SELECT
    *
FROM
    public.users
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.warehouse_units AS
SELECT
    *
FROM
    public.warehouse_units
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.warehouse_positions AS
SELECT
    *
FROM
    public.warehouse_positions
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.warehouse_products AS
SELECT
    *
FROM
    public.warehouse_products
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.warehouse_orders AS
SELECT
    *
FROM
    public.warehouse_orders
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.inventory AS
SELECT
    *
FROM
    public.inventory
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.inventory_checks AS
SELECT
    *
FROM
    public.inventory_checks
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.inventory_check_results AS
SELECT
    *
FROM
    public.inventory_check_results
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.transactions AS
SELECT
    *
FROM
    public.transactions
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.payments AS
SELECT
    *
FROM
    public.payments
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.expenses AS
SELECT
    *
FROM
    public.expenses
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.expense_categories AS
SELECT
    *
FROM
    public.expense_categories
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.payment_categories AS
SELECT
    *
FROM
    public.payment_categories
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.money_accounts AS
SELECT
    *
FROM
    public.money_accounts
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.debts AS
SELECT
    *
FROM
    public.debts
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.projects AS
SELECT
    *
FROM
    public.projects
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.billing_transactions AS
SELECT
    *
FROM
    public.billing_transactions
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.uploads AS
SELECT
    *
FROM
    public.uploads
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.sessions AS
SELECT
    *
FROM
    public.sessions
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.chats AS
SELECT
    *
FROM
    public.chats
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.dialogues AS
SELECT
    *
FROM
    public.dialogues
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.message_templates AS
SELECT
    *
FROM
    public.message_templates
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.action_logs AS
SELECT
    *
FROM
    public.action_logs
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.authentication_logs AS
SELECT
    *
FROM
    public.authentication_logs
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.positions AS
SELECT
    *
FROM
    public.positions
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.roles AS
SELECT
    *
FROM
    public.roles
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.prompts AS
SELECT
    *
FROM
    public.prompts
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.passports AS
SELECT
    *
FROM
    public.passports
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.user_groups AS
SELECT
    *
FROM
    public.user_groups
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.companies AS
SELECT
    *
FROM
    public.companies
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.chat_members AS
SELECT
    *
FROM
    public.chat_members
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.ai_chat_configs AS
SELECT
    *
FROM
    public.ai_chat_configs
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

CREATE OR REPLACE VIEW analytics.permissions AS
SELECT
    *
FROM
    public.permissions
WHERE
    tenant_id = current_setting('app.tenant_id', TRUE)::uuid;

COMMENT ON SCHEMA analytics IS 'Denormalized views for BiChat query executor with automatic tenant isolation using current_setting(''app.tenant_id'', true)::UUID pattern.';

-- +migrate StatementEnd
-- Code interpreter outputs table
CREATE TABLE IF NOT EXISTS bichat.code_interpreter_outputs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    message_id uuid NOT NULL REFERENCES bichat.messages (id) ON DELETE CASCADE,
    name varchar(255) NOT NULL,
    mime_type varchar(100) NOT NULL,
    url text NOT NULL,
    size_bytes bigint NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT NOW()
);

-- Index for fast lookup by message
CREATE INDEX idx_code_outputs_message ON bichat.code_interpreter_outputs (message_id);

-- Index for created_at ordering
CREATE INDEX idx_code_outputs_created_at ON bichat.code_interpreter_outputs (created_at);

-- Generic artifacts table for extensible artifact storage
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
    created_at timestamp with time zone NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_artifacts_session ON bichat.artifacts(session_id, created_at DESC);
CREATE INDEX idx_artifacts_tenant ON bichat.artifacts(tenant_id);
CREATE INDEX idx_artifacts_type ON bichat.artifacts(type);
CREATE INDEX idx_artifacts_message ON bichat.artifacts(message_id) WHERE message_id IS NOT NULL;

COMMENT ON TABLE bichat.artifacts IS 'Generic artifact storage for session outputs (charts, exports, code outputs, etc.)';
COMMENT ON COLUMN bichat.artifacts.type IS 'Artifact type (code_output, chart, export, etc.) - extensible';
COMMENT ON COLUMN bichat.artifacts.metadata IS 'Type-specific data as JSONB (chart spec, row counts, etc.)';

-- +migrate Down
DROP TABLE IF EXISTS bichat.artifacts;
-- Drop bichat schema (cascades to all tables and indexes)
DROP SCHEMA IF EXISTS bichat CASCADE;

-- +migrate StatementBegin
-- Drop analytics views (explicit drops for clarity, though CASCADE will handle them)
DROP VIEW IF EXISTS analytics.permissions;

DROP VIEW IF EXISTS analytics.ai_chat_configs;

DROP VIEW IF EXISTS analytics.chat_members;

DROP VIEW IF EXISTS analytics.companies;

DROP VIEW IF EXISTS analytics.user_groups;

DROP VIEW IF EXISTS analytics.passports;

DROP VIEW IF EXISTS analytics.prompts;

DROP VIEW IF EXISTS analytics.roles;

DROP VIEW IF EXISTS analytics.positions;

DROP VIEW IF EXISTS analytics.authentication_logs;

DROP VIEW IF EXISTS analytics.action_logs;

DROP VIEW IF EXISTS analytics.message_templates;

DROP VIEW IF EXISTS analytics.dialogues;

DROP VIEW IF EXISTS analytics.chats;

DROP VIEW IF EXISTS analytics.tabs;

DROP VIEW IF EXISTS analytics.sessions;

DROP VIEW IF EXISTS analytics.uploads;

DROP VIEW IF EXISTS analytics.billing_transactions;

DROP VIEW IF EXISTS analytics.projects;

DROP VIEW IF EXISTS analytics.debts;

DROP VIEW IF EXISTS analytics.money_accounts;

DROP VIEW IF EXISTS analytics.payment_categories;

DROP VIEW IF EXISTS analytics.expense_categories;

DROP VIEW IF EXISTS analytics.expenses;

DROP VIEW IF EXISTS analytics.payments;

DROP VIEW IF EXISTS analytics.transactions;

DROP VIEW IF EXISTS analytics.inventory_check_results;

DROP VIEW IF EXISTS analytics.inventory_checks;

DROP VIEW IF EXISTS analytics.inventory;

DROP VIEW IF EXISTS analytics.warehouse_orders;

DROP VIEW IF EXISTS analytics.warehouse_products;

DROP VIEW IF EXISTS analytics.warehouse_positions;

DROP VIEW IF EXISTS analytics.warehouse_units;

DROP VIEW IF EXISTS analytics.users;

DROP VIEW IF EXISTS analytics.employees;

DROP VIEW IF EXISTS analytics.counterparty;

DROP VIEW IF EXISTS analytics.clients;

-- Drop analytics schema and bichat_agent_role
DROP SCHEMA IF EXISTS analytics CASCADE;

DROP ROLE IF EXISTS bichat_agent_role;

-- Restore legacy tables dropped in Up (Down reverses Up exactly)
CREATE TABLE dialogues (
	id         SERIAL8 PRIMARY KEY,
	user_id    INT8 NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	label      VARCHAR(255) NOT NULL,
	messages   JSONB NOT NULL,
	created_at TIMESTAMPTZ DEFAULT now(),
	updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE prompts (
	id          VARCHAR(30) PRIMARY KEY,
	title       VARCHAR(255) NOT NULL,
	description TEXT NOT NULL,
	prompt      TEXT NOT NULL,
	created_at  TIMESTAMPTZ DEFAULT now()
);

-- +migrate StatementEnd
