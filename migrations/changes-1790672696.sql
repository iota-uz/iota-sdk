-- +migrate Up
-- Onboarding state and temporary credentials. Existing users stay active.
ALTER TABLE users
ADD COLUMN status VARCHAR(32) NOT NULL DEFAULT 'active',
ADD COLUMN password_expires_at TIMESTAMP WITH TIME ZONE,
ADD COLUMN failed_password_attempts INTEGER NOT NULL DEFAULT 0;

ALTER TABLE users
ADD CONSTRAINT users_status_check CHECK (status IN ('active', 'pending_onboarding')) NOT VALID;

-- +migrate Down
ALTER TABLE users
DROP CONSTRAINT IF EXISTS users_status_check;

ALTER TABLE users
DROP COLUMN IF EXISTS failed_password_attempts,
DROP COLUMN IF EXISTS password_expires_at,
DROP COLUMN IF EXISTS status;
