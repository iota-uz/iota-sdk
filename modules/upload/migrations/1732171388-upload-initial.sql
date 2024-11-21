-- +migrate Up
BEGIN;
CREATE TABLE IF NOT EXISTS uploads
(
    id          VARCHAR(255) PRIMARY KEY,
    url         VARCHAR(1024) NOT NULL DEFAULT '',
    name        VARCHAR(255) NOT NULL DEFAULT '',
    type        VARCHAR(255) NOT NULL DEFAULT '',
    size        INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);
COMMIT;

-- +migrate Down
BEGIN;
DROP TABLE IF EXISTS uploads;
COMMIT;