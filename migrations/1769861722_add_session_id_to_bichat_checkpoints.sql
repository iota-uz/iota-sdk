-- +migrate Up
-- Add session_id column to bichat_checkpoints for multi-tenant isolation and session tracking
ALTER TABLE bichat_checkpoints
ADD COLUMN session_id UUID;

-- Add index for session_id lookups
CREATE INDEX idx_bichat_checkpoints_session ON bichat_checkpoints(session_id) WHERE session_id IS NOT NULL;

-- +migrate Down
-- Remove session_id column and index
DROP INDEX IF EXISTS idx_bichat_checkpoints_session;
ALTER TABLE bichat_checkpoints
DROP COLUMN IF EXISTS session_id;
