-- Remove tenant_id from permissions table to make permissions system-wide
-- +migrate Up

-- Drop the index first
DROP INDEX IF EXISTS permissions_tenant_id_idx;

-- Drop the unique constraint that includes tenant_id
ALTER TABLE permissions DROP CONSTRAINT IF EXISTS permissions_tenant_id_name_key;

-- Remove the tenant_id column
ALTER TABLE permissions DROP COLUMN IF EXISTS tenant_id;

-- Add new unique constraint on name only
ALTER TABLE permissions ADD CONSTRAINT permissions_name_key UNIQUE (name);

-- +migrate Down

-- Re-add tenant_id column
ALTER TABLE permissions ADD COLUMN tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE;

-- Remove the single-column unique constraint
ALTER TABLE permissions DROP CONSTRAINT IF EXISTS permissions_name_key;

-- Re-add the composite unique constraint
ALTER TABLE permissions ADD CONSTRAINT permissions_tenant_id_name_key UNIQUE (tenant_id, name);

-- Re-create the index
CREATE INDEX permissions_tenant_id_idx ON permissions (tenant_id);