-- +migrate Up
ALTER TABLE uploads ADD COLUMN IF NOT EXISTS slug VARCHAR (255);
UPDATE uploads SET slug = hash WHERE slug IS NULL;

ALTER TABLE uploads 
	ALTER COLUMN slug SET NOT NULL,
	ADD CONSTRAINT uploads_tenant_id_slug_unique UNIQUE (tenant_id, slug);

-- +migrate Down
ALTER TABLE uploads 
	DROP CONSTRAINT uploads_tenant_id_slug_unique,
	DROP COLUMN slug;
