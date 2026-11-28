-- +migrate Up
ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS phone VARCHAR(255);

ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS email VARCHAR(255);

-- +migrate Down
ALTER TABLE tenants
    DROP COLUMN IF EXISTS email;

ALTER TABLE tenants
    DROP COLUMN IF EXISTS phone;


