-- Migration: OIDC Phase 2 feature-proofing and integrity hardening
-- Purpose:
--   - Align user_id types with public.users(id) (BIGINT)
--   - Add relational integrity for OIDC core tables
--   - Add post-logout redirect URI support for clients
--   - Add refresh token lineage metadata for rotation/replay tracking
--   - Add access token revocation storage for JWT blacklist support
--   - Add defensive CHECK constraints and performance indexes

-- +migrate Up

-- Align user ID types with public.users(id)
ALTER TABLE oidc.auth_requests
    ALTER COLUMN user_id TYPE BIGINT USING user_id::BIGINT;

ALTER TABLE oidc.refresh_tokens
    ALTER COLUMN user_id TYPE BIGINT USING user_id::BIGINT;

-- Future-proof client metadata for RP-initiated logout
ALTER TABLE oidc.clients
    ADD COLUMN IF NOT EXISTS post_logout_redirect_uris TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[];

COMMENT ON COLUMN oidc.clients.post_logout_redirect_uris IS 'Allowed post_logout_redirect_uri values for RP-initiated logout';

-- Refresh token lineage for rotation/replay handling
ALTER TABLE oidc.refresh_tokens
    ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS replaced_by UUID;

-- Relational integrity constraints
ALTER TABLE oidc.auth_requests
    ADD CONSTRAINT fk_oidc_auth_requests_client_id
        FOREIGN KEY (client_id) REFERENCES oidc.clients(client_id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_oidc_auth_requests_user_id
        FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE SET NULL,
    ADD CONSTRAINT fk_oidc_auth_requests_tenant_id
        FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE SET NULL;

ALTER TABLE oidc.refresh_tokens
    ADD CONSTRAINT fk_oidc_refresh_tokens_client_id
        FOREIGN KEY (client_id) REFERENCES oidc.clients(client_id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_oidc_refresh_tokens_user_id
        FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_oidc_refresh_tokens_tenant_id
        FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_oidc_refresh_tokens_replaced_by
        FOREIGN KEY (replaced_by) REFERENCES oidc.refresh_tokens(id) ON DELETE SET NULL;

-- Defensive client constraints
ALTER TABLE oidc.clients
    ADD CONSTRAINT chk_oidc_clients_application_type
        CHECK (application_type IN ('web', 'native', 'user_agent')),
    ADD CONSTRAINT chk_oidc_clients_auth_method
        CHECK (auth_method IN ('client_secret_basic', 'client_secret_post', 'none')),
    ADD CONSTRAINT chk_oidc_clients_access_token_lifetime_positive
        CHECK (access_token_lifetime > INTERVAL '0 seconds'),
    ADD CONSTRAINT chk_oidc_clients_id_token_lifetime_positive
        CHECK (id_token_lifetime > INTERVAL '0 seconds'),
    ADD CONSTRAINT chk_oidc_clients_refresh_token_lifetime_positive
        CHECK (refresh_token_lifetime > INTERVAL '0 seconds');

-- Access-token blacklist support (JWT revocation)
CREATE TABLE IF NOT EXISTS oidc.revoked_access_tokens (
    jti VARCHAR(255) PRIMARY KEY,
    tenant_id UUID REFERENCES public.tenants(id) ON DELETE SET NULL,
    user_id BIGINT REFERENCES public.users(id) ON DELETE SET NULL,
    client_id VARCHAR(255) REFERENCES oidc.clients(client_id) ON DELETE SET NULL,
    revoked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_oidc_revoked_access_tokens_expires
    ON oidc.revoked_access_tokens(expires_at);

CREATE INDEX IF NOT EXISTS idx_oidc_revoked_access_tokens_tenant_user
    ON oidc.revoked_access_tokens(tenant_id, user_id);

CREATE INDEX IF NOT EXISTS idx_oidc_revoked_access_tokens_client_id
    ON oidc.revoked_access_tokens(client_id);

-- Query path optimization for session termination and token lifecycle jobs
CREATE INDEX IF NOT EXISTS idx_oidc_refresh_tokens_user_client_tenant
    ON oidc.refresh_tokens(user_id, client_id, tenant_id);

CREATE INDEX IF NOT EXISTS idx_oidc_refresh_tokens_revoked_at
    ON oidc.refresh_tokens(revoked_at) WHERE revoked_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_oidc_refresh_tokens_replaced_by
    ON oidc.refresh_tokens(replaced_by) WHERE replaced_by IS NOT NULL;

-- +migrate Down

DROP INDEX IF EXISTS idx_oidc_refresh_tokens_replaced_by;
DROP INDEX IF EXISTS idx_oidc_refresh_tokens_revoked_at;
DROP INDEX IF EXISTS idx_oidc_refresh_tokens_user_client_tenant;

DROP INDEX IF EXISTS idx_oidc_revoked_access_tokens_client_id;
DROP INDEX IF EXISTS idx_oidc_revoked_access_tokens_tenant_user;
DROP INDEX IF EXISTS idx_oidc_revoked_access_tokens_expires;
DROP TABLE IF EXISTS oidc.revoked_access_tokens;

ALTER TABLE oidc.clients
    DROP CONSTRAINT IF EXISTS chk_oidc_clients_refresh_token_lifetime_positive,
    DROP CONSTRAINT IF EXISTS chk_oidc_clients_id_token_lifetime_positive,
    DROP CONSTRAINT IF EXISTS chk_oidc_clients_access_token_lifetime_positive,
    DROP CONSTRAINT IF EXISTS chk_oidc_clients_auth_method,
    DROP CONSTRAINT IF EXISTS chk_oidc_clients_application_type;

ALTER TABLE oidc.refresh_tokens
    DROP CONSTRAINT IF EXISTS fk_oidc_refresh_tokens_replaced_by,
    DROP CONSTRAINT IF EXISTS fk_oidc_refresh_tokens_tenant_id,
    DROP CONSTRAINT IF EXISTS fk_oidc_refresh_tokens_user_id,
    DROP CONSTRAINT IF EXISTS fk_oidc_refresh_tokens_client_id;

ALTER TABLE oidc.auth_requests
    DROP CONSTRAINT IF EXISTS fk_oidc_auth_requests_tenant_id,
    DROP CONSTRAINT IF EXISTS fk_oidc_auth_requests_user_id,
    DROP CONSTRAINT IF EXISTS fk_oidc_auth_requests_client_id;

ALTER TABLE oidc.refresh_tokens
    DROP COLUMN IF EXISTS replaced_by,
    DROP COLUMN IF EXISTS revoked_at;

ALTER TABLE oidc.clients
    DROP COLUMN IF EXISTS post_logout_redirect_uris;

ALTER TABLE oidc.refresh_tokens
    ALTER COLUMN user_id TYPE INTEGER USING user_id::INTEGER;

ALTER TABLE oidc.auth_requests
    ALTER COLUMN user_id TYPE INTEGER USING user_id::INTEGER;
