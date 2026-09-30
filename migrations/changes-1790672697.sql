-- +migrate Up
-- Validate separately so the table scan does not hold the ALTER TABLE lock of changes-1790672696.sql.
ALTER TABLE users VALIDATE CONSTRAINT users_status_check;

-- +migrate Down
ALTER TABLE users
DROP CONSTRAINT IF EXISTS users_status_check;

ALTER TABLE users
ADD CONSTRAINT users_status_check CHECK (status IN ('active', 'pending_onboarding')) NOT VALID;
