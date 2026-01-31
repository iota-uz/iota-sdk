-- OIDC Client Setup Example
-- This script demonstrates how to manually create OIDC clients

-- Example 1: Web Application (Confidential Client)
INSERT INTO oidc_clients (
    client_id,
    client_secret_hash,
    name,
    application_type,
    redirect_uris,
    grant_types,
    response_types,
    scopes,
    auth_method,
    access_token_lifetime,
    id_token_lifetime,
    refresh_token_lifetime,
    require_pkce,
    is_active
) VALUES (
    'webapp-client',
    -- Secret: 'my-secret-123' (hash with bcrypt before using in production)
    '$2a$10$rEMZV8NbZ.8qGz5vZ7KqXOZGqZ5Z7vZ7KqXOZGqZ5Z7vZ7KqXOZGq',
    'My Web Application',
    'web',
    ARRAY['http://localhost:3000/callback', 'https://app.example.com/callback'],
    ARRAY['authorization_code', 'refresh_token'],
    ARRAY['code'],
    ARRAY['openid', 'profile', 'email', 'offline_access'],
    'client_secret_basic',
    INTERVAL '1 hour',     -- Access token lifetime
    INTERVAL '1 hour',     -- ID token lifetime
    INTERVAL '30 days',    -- Refresh token lifetime
    false,                 -- PKCE optional for confidential clients
    true                   -- Active
)
ON CONFLICT (client_id) DO NOTHING;

-- Example 2: Single Page Application (Public Client)
INSERT INTO oidc_clients (
    client_id,
    client_secret_hash,
    name,
    application_type,
    redirect_uris,
    grant_types,
    response_types,
    scopes,
    auth_method,
    access_token_lifetime,
    id_token_lifetime,
    refresh_token_lifetime,
    require_pkce,
    is_active
) VALUES (
    'spa-client',
    NULL,  -- No client secret for public clients
    'My SPA Application',
    'spa',
    ARRAY['http://localhost:3000/callback'],
    ARRAY['authorization_code', 'refresh_token'],
    ARRAY['code'],
    ARRAY['openid', 'profile', 'email', 'offline_access'],
    'none',  -- No authentication for public clients
    INTERVAL '30 minutes', -- Shorter lifetime for public clients
    INTERVAL '30 minutes',
    INTERVAL '7 days',     -- Shorter refresh token lifetime
    true,                  -- PKCE REQUIRED for public clients
    true
)
ON CONFLICT (client_id) DO NOTHING;

-- Example 3: Mobile Application (Public Client)
INSERT INTO oidc_clients (
    client_id,
    client_secret_hash,
    name,
    application_type,
    redirect_uris,
    grant_types,
    response_types,
    scopes,
    auth_method,
    access_token_lifetime,
    id_token_lifetime,
    refresh_token_lifetime,
    require_pkce,
    is_active
) VALUES (
    'mobile-app',
    NULL,
    'My Mobile App',
    'native',
    ARRAY['com.example.app://callback'],  -- Custom URI scheme
    ARRAY['authorization_code', 'refresh_token'],
    ARRAY['code'],
    ARRAY['openid', 'profile', 'email', 'offline_access'],
    'none',
    INTERVAL '1 hour',
    INTERVAL '1 hour',
    INTERVAL '90 days',  -- Longer for mobile apps
    true,                -- PKCE REQUIRED
    true
)
ON CONFLICT (client_id) DO NOTHING;

-- Example 4: Backend Service (Client Credentials)
INSERT INTO oidc_clients (
    client_id,
    client_secret_hash,
    name,
    application_type,
    redirect_uris,
    grant_types,
    response_types,
    scopes,
    auth_method,
    access_token_lifetime,
    id_token_lifetime,
    refresh_token_lifetime,
    require_pkce,
    is_active
) VALUES (
    'backend-service',
    '$2a$10$rEMZV8NbZ.8qGz5vZ7KqXOZGqZ5Z7vZ7KqXOZGqZ5Z7vZ7KqXOZGq',
    'Backend API Service',
    'service',
    ARRAY[],  -- No redirect URIs for client credentials
    ARRAY['client_credentials'],
    ARRAY[],  -- No response types for client credentials
    ARRAY['api:read', 'api:write'],  -- Custom scopes
    'client_secret_post',
    INTERVAL '2 hours',
    INTERVAL '0',        -- No ID token for client credentials
    INTERVAL '0',        -- No refresh token for client credentials
    false,
    true
)
ON CONFLICT (client_id) DO NOTHING;

-- Verify clients were created
SELECT
    client_id,
    name,
    application_type,
    array_length(redirect_uris, 1) as redirect_uri_count,
    grant_types,
    require_pkce,
    is_active
FROM oidc_clients
WHERE client_id IN ('webapp-client', 'spa-client', 'mobile-app', 'backend-service')
ORDER BY client_id;

-- Example: Update existing client
UPDATE oidc_clients
SET
    redirect_uris = ARRAY['https://app.example.com/callback', 'https://app.example.com/callback2'],
    scopes = ARRAY['openid', 'profile', 'email', 'offline_access', 'custom:read'],
    updated_at = NOW()
WHERE client_id = 'webapp-client';

-- Example: Deactivate client
UPDATE oidc_clients
SET is_active = false, updated_at = NOW()
WHERE client_id = 'old-client';

-- Example: Delete client (and cascade delete refresh tokens)
-- DELETE FROM oidc_clients WHERE client_id = 'client-to-delete';
