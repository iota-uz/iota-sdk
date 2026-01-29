-- +migrate Up
ALTER TABLE uploads ADD COLUMN IF NOT EXISTS source VARCHAR(64) NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_uploads_tenant_source ON uploads(tenant_id, source);

-- +migrate Down
DROP INDEX IF EXISTS idx_uploads_tenant_source;
ALTER TABLE uploads DROP COLUMN IF EXISTS source;
