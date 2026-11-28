-- Migration: Add ON UPDATE CASCADE to permission FK constraints
-- Date: 2026-03-02
-- Purpose: When permission UUIDs are corrected via upsert (SET id = EXCLUDED.id),
--   ON UPDATE CASCADE automatically propagates the new UUID to role_permissions
--   and user_permissions, preventing orphaned or mismatched foreign keys.

-- +migrate Up
ALTER TABLE role_permissions
    DROP CONSTRAINT role_permissions_permission_id_fkey,
    ADD CONSTRAINT role_permissions_permission_id_fkey
        FOREIGN KEY (permission_id) REFERENCES permissions (id)
        ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE user_permissions
    DROP CONSTRAINT user_permissions_permission_id_fkey,
    ADD CONSTRAINT user_permissions_permission_id_fkey
        FOREIGN KEY (permission_id) REFERENCES permissions (id)
        ON DELETE CASCADE ON UPDATE CASCADE;

-- +migrate Down
ALTER TABLE role_permissions
    DROP CONSTRAINT IF EXISTS role_permissions_permission_id_fkey;
ALTER TABLE role_permissions
    ADD CONSTRAINT role_permissions_permission_id_fkey
        FOREIGN KEY (permission_id) REFERENCES permissions (id)
        ON DELETE CASCADE;

ALTER TABLE user_permissions
    DROP CONSTRAINT IF EXISTS user_permissions_permission_id_fkey;
ALTER TABLE user_permissions
    ADD CONSTRAINT user_permissions_permission_id_fkey
        FOREIGN KEY (permission_id) REFERENCES permissions (id)
        ON DELETE CASCADE;
