-- Migration: Tenant-qualified unique key on user_groups
-- Purpose: Add a UNIQUE (tenant_id, id) constraint to user_groups so downstream
--   modules can declare tenant-qualified composite foreign keys
--   (e.g. FOREIGN KEY (tenant_id, group_id) REFERENCES user_groups (tenant_id, id)),
--   preventing cross-tenant references at the database level. id is already the
--   primary key, so the composite is always satisfiable for existing rows.

-- +migrate Up
ALTER TABLE user_groups
    ADD CONSTRAINT user_groups_tenant_id_id_key UNIQUE (tenant_id, id);

-- +migrate Down
ALTER TABLE user_groups
    DROP CONSTRAINT IF EXISTS user_groups_tenant_id_id_key;
