-- +migrate Up
-- Add blocking columns to users table
ALTER TABLE users
ADD COLUMN is_blocked BOOLEAN NOT NULL DEFAULT FALSE,
ADD COLUMN block_reason TEXT,
ADD COLUMN blocked_at TIMESTAMP WITH TIME ZONE,
ADD COLUMN blocked_by INTEGER REFERENCES users(id) ON DELETE SET NULL;

-- Add indexes for better query performance
CREATE INDEX idx_users_is_blocked ON users(is_blocked) WHERE is_blocked = TRUE;
CREATE INDEX idx_users_blocked_by ON users(blocked_by);

-- Add constraint: block_reason must be between 3 and 1024 characters if provided
ALTER TABLE users
ADD CONSTRAINT check_block_reason_length
CHECK (block_reason IS NULL OR (LENGTH(block_reason) >= 3 AND LENGTH(block_reason) <= 1024));

-- Add constraint: ensure block fields are consistent
-- If is_blocked = FALSE, all other fields must be NULL
-- If is_blocked = TRUE, block_reason, blocked_at, and blocked_by must NOT be NULL
ALTER TABLE users
ADD CONSTRAINT check_block_fields_consistency
CHECK (
    (is_blocked = FALSE AND block_reason IS NULL AND blocked_at IS NULL AND blocked_by IS NULL)
    OR
    (is_blocked = TRUE AND block_reason IS NOT NULL AND blocked_at IS NOT NULL AND blocked_by IS NOT NULL)
);

-- +migrate Down
-- Remove constraints
ALTER TABLE users DROP CONSTRAINT IF EXISTS check_block_fields_consistency;
ALTER TABLE users DROP CONSTRAINT IF EXISTS check_block_reason_length;

-- Remove indexes
DROP INDEX IF EXISTS idx_users_blocked_by;
DROP INDEX IF EXISTS idx_users_is_blocked;

-- Remove columns
ALTER TABLE users DROP COLUMN IF EXISTS blocked_by;
ALTER TABLE users DROP COLUMN IF EXISTS blocked_at;
ALTER TABLE users DROP COLUMN IF EXISTS block_reason;
ALTER TABLE users DROP COLUMN IF EXISTS is_blocked;
