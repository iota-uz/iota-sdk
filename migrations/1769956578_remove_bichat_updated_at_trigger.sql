-- +migrate Up
-- Remove database trigger for bichat_sessions.updated_at
-- Note: Initial migration (1769853356) did not create this trigger/function.
-- This migration is idempotent - safe to run even if trigger doesn't exist.
-- The updated_at field is now managed in the application layer for better control and testability

-- Drop trigger if it exists
DROP TRIGGER IF EXISTS trigger_bichat_sessions_updated_at ON bichat_sessions;

-- Drop function if it exists
DROP FUNCTION IF EXISTS update_bichat_session_updated_at();

-- +migrate Down
-- No action needed - trigger was never created by initial migration
-- The updated_at field remains in the table and is now managed in application layer
-- Initial migration (1769853356_create_bichat_tables.sql) did not include trigger/function
