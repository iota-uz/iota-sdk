-- +migrate Up
-- Add audience column to sessions table for session isolation between applications
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS audience VARCHAR(32) NOT NULL DEFAULT 'granite';

-- Add index for efficient audience filtering
CREATE INDEX IF NOT EXISTS idx_sessions_audience ON sessions(audience);

-- +migrate Down
-- Remove audience-related changes
DROP INDEX IF EXISTS idx_sessions_audience;
ALTER TABLE sessions DROP COLUMN IF EXISTS audience;
