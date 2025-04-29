-- +migrate Up

-- 1. Add nullable 'type' column with VARCHAR(50)
ALTER TABLE roles ADD COLUMN type VARCHAR(50);
ALTER TABLE user_groups ADD COLUMN type VARCHAR(50);
ALTER TABLE users ADD COLUMN type VARCHAR(50);

-- 2. Set default value 'user' for all existing records
UPDATE roles SET type = 'user' WHERE type IS NULL;
UPDATE user_groups SET type = 'user' WHERE type IS NULL;
UPDATE users SET type = 'user' WHERE type IS NULL;

-- 3. Set NOT NULL and add CHECK constraint
ALTER TABLE roles
    ALTER COLUMN type SET NOT NULL,
    ADD CONSTRAINT roles_type_check CHECK (type IN ('system', 'user'));

ALTER TABLE user_groups
    ALTER COLUMN type SET NOT NULL,
    ADD CONSTRAINT user_groups_type_check CHECK (type IN ('system', 'user'));

ALTER TABLE users
    ALTER COLUMN type SET NOT NULL,
    ADD CONSTRAINT users_type_check CHECK (type IN ('system', 'user'));

-- +migrate Down

-- Drop 'type' column if it exists
ALTER TABLE users DROP COLUMN IF EXISTS type;
ALTER TABLE user_groups DROP COLUMN IF EXISTS type;
ALTER TABLE roles DROP COLUMN IF EXISTS type;
