-- +migrate Up
-- HITL redesign: move question state from sessions to messages
-- Single source of truth: question_data JSONB column on messages table
-- Replaces the session-level pending_question_agent flag

-- Add question_data column to messages
ALTER TABLE bichat.messages
    ADD COLUMN IF NOT EXISTS question_data jsonb;

-- Remove redundant session-level HITL flag
ALTER TABLE bichat.sessions
    DROP COLUMN IF EXISTS pending_question_agent;

-- At most one pending question per session (DB-level constraint)
CREATE UNIQUE INDEX IF NOT EXISTS idx_messages_one_pending_question
    ON bichat.messages (session_id)
    WHERE question_data ->> 'status' = 'pending';

-- +migrate Down
DROP INDEX IF EXISTS bichat.idx_messages_one_pending_question;

ALTER TABLE bichat.messages
    DROP COLUMN IF EXISTS question_data;

ALTER TABLE bichat.sessions
    ADD COLUMN IF NOT EXISTS pending_question_agent varchar(100);
