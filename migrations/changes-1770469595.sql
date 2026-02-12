-- +migrate Up
-- BiChat Learnings & Validated Queries
-- Stores agent-captured insights and proven SQL query patterns

-- ========================================
-- Learnings Table
-- ========================================
CREATE TABLE IF NOT EXISTS bichat.learnings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    category TEXT NOT NULL DEFAULT 'sql_error',
    trigger_text TEXT NOT NULL,
    lesson TEXT NOT NULL,
    table_name TEXT,
    sql_patch TEXT,
    used_count INTEGER NOT NULL DEFAULT 0,
    content_hash TEXT GENERATED ALWAYS AS (
        md5(category || ':' || trigger_text || ':' || COALESCE(table_name, ''))
    ) STORED,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT learnings_category_check CHECK (
        category IN ('sql_error', 'type_mismatch', 'user_correction', 'business_rule')
    )
);

-- ========================================
-- Validated Queries Table
-- ========================================
CREATE TABLE IF NOT EXISTS bichat.validated_queries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,
    question TEXT NOT NULL,
    sql TEXT NOT NULL,
    summary TEXT NOT NULL,
    tables_used TEXT[] NOT NULL DEFAULT '{}',
    data_quality_notes TEXT[] DEFAULT '{}',
    used_count INTEGER NOT NULL DEFAULT 0,
    sql_hash TEXT GENERATED ALWAYS AS (
        md5(sql)
    ) STORED,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ========================================
-- Indexes
-- ========================================
CREATE INDEX IF NOT EXISTS idx_bichat_learnings_tenant ON bichat.learnings(tenant_id);
CREATE INDEX IF NOT EXISTS idx_bichat_learnings_table ON bichat.learnings(tenant_id, table_name);
CREATE INDEX IF NOT EXISTS idx_bichat_learnings_category ON bichat.learnings(tenant_id, category);
CREATE INDEX IF NOT EXISTS idx_bichat_learnings_fts ON bichat.learnings
    USING GIN (to_tsvector('english', trigger_text || ' ' || lesson));

CREATE INDEX IF NOT EXISTS idx_bichat_vq_tenant ON bichat.validated_queries(tenant_id);
CREATE INDEX IF NOT EXISTS idx_bichat_vq_tables ON bichat.validated_queries USING GIN (tables_used);
CREATE INDEX IF NOT EXISTS idx_bichat_vq_fts ON bichat.validated_queries
    USING GIN (to_tsvector('english', question || ' ' || summary));

-- ========================================
-- Deduplication Indexes
-- ========================================
CREATE UNIQUE INDEX IF NOT EXISTS idx_bichat_learnings_dedup ON bichat.learnings(tenant_id, content_hash);
CREATE UNIQUE INDEX IF NOT EXISTS idx_bichat_vq_dedup ON bichat.validated_queries(tenant_id, sql_hash);

-- ========================================
-- Comments
-- ========================================
COMMENT ON TABLE bichat.learnings IS 'Agent-captured learnings from SQL errors, type mismatches, and user corrections';
COMMENT ON TABLE bichat.validated_queries IS 'Proven SQL query patterns that successfully answered user questions';

COMMENT ON COLUMN bichat.learnings.category IS 'Type of learning: sql_error, type_mismatch, user_correction, business_rule';
COMMENT ON COLUMN bichat.learnings.trigger_text IS 'What caused this learning (error message, user input, etc.)';
COMMENT ON COLUMN bichat.learnings.lesson IS 'What to do/avoid next time';
COMMENT ON COLUMN bichat.learnings.table_name IS 'Optional: Related table for schema-specific learnings';
COMMENT ON COLUMN bichat.learnings.sql_patch IS 'Optional: SQL fix or pattern to apply';
COMMENT ON COLUMN bichat.learnings.used_count IS 'Track how often this learning has been retrieved and applied';
COMMENT ON COLUMN bichat.learnings.content_hash IS 'MD5 hash of category:trigger_text:table_name for deduplication';

COMMENT ON COLUMN bichat.validated_queries.question IS 'Original user question this query answered';
COMMENT ON COLUMN bichat.validated_queries.sql IS 'Validated SQL query that successfully answered the question';
COMMENT ON COLUMN bichat.validated_queries.summary IS 'Brief description of what the query does';
COMMENT ON COLUMN bichat.validated_queries.tables_used IS 'Array of table names referenced in the query';
COMMENT ON COLUMN bichat.validated_queries.data_quality_notes IS 'Optional: Known issues or caveats with this query/data';
COMMENT ON COLUMN bichat.validated_queries.used_count IS 'Track how often this query pattern has been reused';
COMMENT ON COLUMN bichat.validated_queries.sql_hash IS 'MD5 hash of SQL for deduplication';

-- ========================================
-- Sessions & Checkpoints (response chain)
-- ========================================
ALTER TABLE bichat.sessions
    ADD COLUMN IF NOT EXISTS llm_previous_response_id varchar(255);

ALTER TABLE bichat.checkpoints
    ADD COLUMN IF NOT EXISTS previous_response_id varchar(255);

-- Persist assistant debug trace for deterministic debug mode rendering.
ALTER TABLE bichat.messages
    ADD COLUMN IF NOT EXISTS debug_trace jsonb;

-- ========================================
-- HITL redesign: move question state from sessions to messages
-- Single source of truth: question_data JSONB column on messages table
-- Replaces the session-level pending_question_agent flag
-- ========================================

-- Add question_data column to messages
ALTER TABLE bichat.messages
    ADD COLUMN IF NOT EXISTS question_data jsonb;

-- Remove redundant session-level HITL flag
ALTER TABLE bichat.sessions
    DROP COLUMN IF EXISTS pending_question_agent;

-- At most one pending question per session (DB-level constraint)
CREATE UNIQUE INDEX IF NOT EXISTS idx_messages_one_pending_question
    ON bichat.messages (session_id)
    WHERE question_data ->> 'status' = 'PENDING';

-- +migrate Down
DROP INDEX IF EXISTS bichat.idx_messages_one_pending_question;

ALTER TABLE bichat.messages
    DROP COLUMN IF EXISTS question_data;

ALTER TABLE bichat.sessions
    ADD COLUMN IF NOT EXISTS pending_question_agent varchar(100);

ALTER TABLE bichat.messages
    DROP COLUMN IF EXISTS debug_trace;

ALTER TABLE bichat.checkpoints
    DROP COLUMN IF EXISTS previous_response_id;

ALTER TABLE bichat.sessions
    DROP COLUMN IF EXISTS llm_previous_response_id;

DROP TABLE IF EXISTS bichat.validated_queries CASCADE;
DROP TABLE IF EXISTS bichat.learnings CASCADE;
