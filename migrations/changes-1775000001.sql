-- Migration: Create OIDC tables for OpenID Connect Provider
-- Date: 2026-01-31
-- Purpose: Establish OIDC database schema with 5 core tables:
--   - oidc_clients: OAuth 2.0/OIDC client registrations (GLOBAL, Super Admin managed)
--   - oidc_auth_requests: Ephemeral authorization requests (5-min TTL)
--   - oidc_refresh_tokens: Long-lived refresh tokens with tenant isolation
--   - oidc_signing_keys: JWK signing keys for ID/access tokens
--   - oidc_revoked_access_tokens: Access-token blacklist for JWT revocation support

-- +migrate Up

CREATE SCHEMA IF NOT EXISTS oidc;

-- OIDC Clients (GLOBAL - no tenant_id, managed by Super Admin)
CREATE TABLE oidc.clients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id VARCHAR(255) NOT NULL UNIQUE,
    client_secret_hash VARCHAR(255),        -- bcrypt, NULL for public clients
    name VARCHAR(255) NOT NULL,
    application_type VARCHAR(50) NOT NULL,  -- web, native, user_agent
    redirect_uris TEXT[] NOT NULL,
    post_logout_redirect_uris TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    grant_types VARCHAR(50)[] DEFAULT ARRAY['authorization_code'],
    response_types VARCHAR(50)[] DEFAULT ARRAY['code'],
    scopes TEXT[] DEFAULT ARRAY['openid','profile','email'],
    auth_method VARCHAR(50) DEFAULT 'client_secret_basic',
    access_token_lifetime INTERVAL DEFAULT '1 hour',
    id_token_lifetime INTERVAL DEFAULT '1 hour',
    refresh_token_lifetime INTERVAL DEFAULT '720 hours',
    require_pkce BOOLEAN DEFAULT true,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT chk_oidc_clients_application_type
        CHECK (application_type IN ('web', 'native', 'user_agent')),
    CONSTRAINT chk_oidc_clients_auth_method
        CHECK (auth_method IN ('client_secret_basic', 'client_secret_post', 'none')),
    CONSTRAINT chk_oidc_clients_access_token_lifetime_positive
        CHECK (access_token_lifetime > INTERVAL '0 seconds'),
    CONSTRAINT chk_oidc_clients_id_token_lifetime_positive
        CHECK (id_token_lifetime > INTERVAL '0 seconds'),
    CONSTRAINT chk_oidc_clients_refresh_token_lifetime_positive
        CHECK (refresh_token_lifetime > INTERVAL '0 seconds')
);

CREATE INDEX idx_oidc_clients_client_id ON oidc.clients(client_id);
CREATE INDEX idx_oidc_clients_is_active ON oidc.clients(is_active);

COMMENT ON TABLE oidc.clients IS 'OAuth 2.0/OIDC client registrations managed by Super Admin';
COMMENT ON COLUMN oidc.clients.client_secret_hash IS 'bcrypt hash of client secret, NULL for public clients (PKCE-only)';
COMMENT ON COLUMN oidc.clients.application_type IS 'RFC 7591 application type: web, native, user_agent';
COMMENT ON COLUMN oidc.clients.auth_method IS 'Token endpoint authentication method: client_secret_basic, client_secret_post, none';
COMMENT ON COLUMN oidc.clients.post_logout_redirect_uris IS 'Allowed post_logout_redirect_uri values for RP-initiated logout';

-- Authorization Requests (ephemeral, 5-min TTL)
CREATE TABLE oidc.auth_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id VARCHAR(255) NOT NULL,
    redirect_uri TEXT NOT NULL,
    scopes TEXT[] NOT NULL,
    state VARCHAR(512),
    nonce VARCHAR(512),
    response_type VARCHAR(50) NOT NULL,
    code_challenge VARCHAR(128),
    code_challenge_method VARCHAR(10),
    user_id BIGINT,                         -- NULL until authenticated
    tenant_id UUID,                         -- Set after auth (from user's tenant)
    auth_time TIMESTAMPTZ,                  -- Set after authentication
    created_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,        -- Default: created_at + 5 min
    code VARCHAR(64) UNIQUE,                -- Cryptographic authorization code (base64url)
    code_used_at TIMESTAMPTZ,               -- NULL until code exchanged (one-time use)
    CONSTRAINT fk_oidc_auth_requests_client_id
        FOREIGN KEY (client_id) REFERENCES oidc.clients(client_id) ON DELETE CASCADE,
    CONSTRAINT fk_oidc_auth_requests_user_id
        FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE SET NULL,
    CONSTRAINT fk_oidc_auth_requests_tenant_id
        FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE SET NULL
);

CREATE INDEX idx_oidc_auth_requests_expires ON oidc.auth_requests(expires_at);
CREATE INDEX idx_oidc_auth_requests_client_id ON oidc.auth_requests(client_id);
CREATE INDEX idx_oidc_auth_requests_user_id ON oidc.auth_requests(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_oidc_auth_requests_code ON oidc.auth_requests(code) WHERE code IS NOT NULL;

COMMENT ON TABLE oidc.auth_requests IS 'Ephemeral authorization requests with 5-minute TTL';
COMMENT ON COLUMN oidc.auth_requests.user_id IS 'NULL until user authenticates, then set to authenticated user ID';
COMMENT ON COLUMN oidc.auth_requests.tenant_id IS 'Set after authentication from user''s tenant for multi-tenant isolation';
COMMENT ON COLUMN oidc.auth_requests.code_challenge IS 'PKCE code challenge for public clients (RFC 7636)';
COMMENT ON COLUMN oidc.auth_requests.code IS 'Cryptographically random authorization code (base64url, 32 bytes)';
COMMENT ON COLUMN oidc.auth_requests.code_used_at IS 'Timestamp when code was exchanged, NULL if unused (one-time use)';

-- Refresh Tokens (for token refresh flow)
CREATE TABLE oidc.refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash VARCHAR(255) NOT NULL UNIQUE, -- SHA-256 for lookup
    client_id VARCHAR(255) NOT NULL,
    user_id BIGINT NOT NULL,
    tenant_id UUID NOT NULL,                 -- For tenant isolation in tokens
    scopes TEXT[] NOT NULL,
    audience TEXT[],                         -- aud claim
    auth_time TIMESTAMPTZ NOT NULL,
    amr TEXT[],                              -- Authentication Methods
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,                  -- Set when token is explicitly revoked
    replaced_by UUID,                        -- Next refresh token in rotation chain
    CONSTRAINT fk_oidc_refresh_tokens_client_id
        FOREIGN KEY (client_id) REFERENCES oidc.clients(client_id) ON DELETE CASCADE,
    CONSTRAINT fk_oidc_refresh_tokens_user_id
        FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE,
    CONSTRAINT fk_oidc_refresh_tokens_tenant_id
        FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE,
    CONSTRAINT fk_oidc_refresh_tokens_replaced_by
        FOREIGN KEY (replaced_by) REFERENCES oidc.refresh_tokens(id) ON DELETE SET NULL
);

CREATE INDEX idx_oidc_refresh_tokens_hash ON oidc.refresh_tokens(token_hash);
CREATE INDEX idx_oidc_refresh_tokens_expires ON oidc.refresh_tokens(expires_at);
CREATE INDEX idx_oidc_refresh_tokens_user_id ON oidc.refresh_tokens(user_id);
CREATE INDEX idx_oidc_refresh_tokens_tenant_id ON oidc.refresh_tokens(tenant_id);
CREATE INDEX idx_oidc_refresh_tokens_user_client_tenant ON oidc.refresh_tokens(user_id, client_id, tenant_id);
CREATE INDEX idx_oidc_refresh_tokens_revoked_at ON oidc.refresh_tokens(revoked_at) WHERE revoked_at IS NOT NULL;
CREATE INDEX idx_oidc_refresh_tokens_replaced_by ON oidc.refresh_tokens(replaced_by) WHERE replaced_by IS NOT NULL;

COMMENT ON TABLE oidc.refresh_tokens IS 'Long-lived refresh tokens with tenant isolation for multi-tenant token refresh';
COMMENT ON COLUMN oidc.refresh_tokens.token_hash IS 'SHA-256 hash of refresh token for secure lookup';
COMMENT ON COLUMN oidc.refresh_tokens.amr IS 'Authentication Methods References (e.g., ["pwd", "mfa"])';
COMMENT ON COLUMN oidc.refresh_tokens.audience IS 'Token audience (aud claim) for resource server restrictions';
COMMENT ON COLUMN oidc.refresh_tokens.revoked_at IS 'Timestamp when refresh token was revoked';
COMMENT ON COLUMN oidc.refresh_tokens.replaced_by IS 'Refresh token ID issued as rotation successor';

-- Revoked Access Tokens (JWT blacklist support)
CREATE TABLE oidc.revoked_access_tokens (
    jti VARCHAR(255) PRIMARY KEY,            -- JWT ID claim
    tenant_id UUID REFERENCES public.tenants(id) ON DELETE SET NULL,
    user_id BIGINT REFERENCES public.users(id) ON DELETE SET NULL,
    client_id VARCHAR(255) REFERENCES oidc.clients(client_id) ON DELETE SET NULL,
    revoked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_oidc_revoked_access_tokens_expires ON oidc.revoked_access_tokens(expires_at);
CREATE INDEX idx_oidc_revoked_access_tokens_tenant_user ON oidc.revoked_access_tokens(tenant_id, user_id);
CREATE INDEX idx_oidc_revoked_access_tokens_client_id ON oidc.revoked_access_tokens(client_id);

COMMENT ON TABLE oidc.revoked_access_tokens IS 'Revoked JWT access tokens for server-side blacklist checks';
COMMENT ON COLUMN oidc.revoked_access_tokens.jti IS 'JWT ID (jti) value for revoked access token';

-- Signing Keys (JWKs) - bootstrapped on first start if empty
CREATE TABLE oidc.signing_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_id VARCHAR(255) NOT NULL UNIQUE,     -- kid for JWKS
    algorithm VARCHAR(10) DEFAULT 'RS256',
    private_key BYTEA NOT NULL,              -- AES-encrypted with OIDC_CRYPTO_KEY
    public_key BYTEA NOT NULL,               -- PEM-encoded, public
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ                   -- For key rotation (Phase 3)
);

CREATE INDEX idx_oidc_signing_keys_key_id ON oidc.signing_keys(key_id);
CREATE INDEX idx_oidc_signing_keys_is_active ON oidc.signing_keys(is_active);

COMMENT ON TABLE oidc.signing_keys IS 'JWK signing keys for ID tokens and access tokens, bootstrapped on first start';
COMMENT ON COLUMN oidc.signing_keys.private_key IS 'AES-encrypted private key using OIDC_CRYPTO_KEY environment variable';
COMMENT ON COLUMN oidc.signing_keys.public_key IS 'PEM-encoded public key exposed via /.well-known/jwks.json';
COMMENT ON COLUMN oidc.signing_keys.expires_at IS 'Key expiration for rotation (Phase 3), NULL for active keys';

-- +migrate Down

-- Undo CREATE_TABLE: oidc_revoked_access_tokens
DROP TABLE IF EXISTS oidc.revoked_access_tokens CASCADE;

-- Undo CREATE_TABLE: oidc_signing_keys
DROP TABLE IF EXISTS oidc.signing_keys CASCADE;

-- Undo CREATE_TABLE: oidc_refresh_tokens
DROP TABLE IF EXISTS oidc.refresh_tokens CASCADE;

-- Undo CREATE_TABLE: oidc_auth_requests
DROP TABLE IF EXISTS oidc.auth_requests CASCADE;

-- Undo CREATE_TABLE: oidc_clients
DROP TABLE IF EXISTS oidc.clients CASCADE;

DROP SCHEMA IF EXISTS oidc CASCADE;
