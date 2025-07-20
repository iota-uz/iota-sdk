-- +migrate Up
-- Add logo columns to tenants table
ALTER TABLE tenants
    ADD COLUMN logo_id int REFERENCES uploads (id) ON DELETE SET NULL,
    ADD COLUMN logo_compact_id int REFERENCES uploads (id) ON DELETE SET NULL;

-- +migrate Down
-- Remove logo columns from tenants table
ALTER TABLE tenants
    DROP COLUMN IF EXISTS logo_compact_id,
    DROP COLUMN IF EXISTS logo_id;


