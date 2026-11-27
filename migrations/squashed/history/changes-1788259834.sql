-- Allow encrypted authorization codes produced by the OIDC provider.
-- +migrate Up
ALTER TABLE oidc.auth_requests
    ALTER COLUMN code TYPE TEXT;

-- +migrate Down
ALTER TABLE oidc.auth_requests
    ALTER COLUMN code TYPE VARCHAR(64);
