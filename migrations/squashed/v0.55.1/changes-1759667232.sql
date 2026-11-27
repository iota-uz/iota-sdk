-- +migrate Up

-- Extend users.type constraint to include 'superadmin'
-- Drop the existing constraint
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_type_check;

-- Add new constraint with 'superadmin' type
ALTER TABLE users
    ADD CONSTRAINT users_type_check CHECK (type IN ('system', 'user', 'superadmin'));

-- +migrate Down

-- Revert to previous constraint (removes 'superadmin' support)
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_type_check;

ALTER TABLE users
    ADD CONSTRAINT users_type_check CHECK (type IN ('system', 'user'));
