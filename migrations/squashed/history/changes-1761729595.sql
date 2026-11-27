-- +migrate Up
ALTER TABLE uploads ADD COLUMN IF NOT EXISTS geopoint POINT NOT NULL DEFAULT '(0, 0)';
-- +migrate Down
ALTER TABLE uploads DROP COLUMN IF EXISTS geopoint;