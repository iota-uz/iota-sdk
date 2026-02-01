# OIDC Configuration Guide

This guide explains how to configure the OIDC module in IOTA SDK.

## Table of Contents

- [Environment Variables](#environment-variables)
- [Crypto Key Generation](#crypto-key-generation)
- [Token Lifetime Configuration](#token-lifetime-configuration)
- [Multi-Tenant Considerations](#multi-tenant-considerations)
- [Security Best Practices](#security-best-practices)

---

## Environment Variables

### Required Variables

Add these variables to your `.env` file:

```bash
# OIDC Issuer URL (must match your domain)
OIDC_ISSUER=https://your-domain.com

# Crypto key for encrypting signing keys (32+ characters recommended)
OIDC_CRYPTO_KEY=your-very-secure-random-key-here-32-chars-minimum

# Database connection (already configured for SDK)
DATABASE_URL=postgresql://user:password@localhost:5432/dbname
```

### Optional Variables

```bash
# Token lifetimes (defaults shown)
OIDC_ACCESS_TOKEN_LIFETIME=3600          # 1 hour in seconds
OIDC_ID_TOKEN_LIFETIME=3600              # 1 hour in seconds
OIDC_REFRESH_TOKEN_LIFETIME=2592000      # 30 days in seconds
OIDC_AUTH_CODE_LIFETIME=300              # 5 minutes in seconds

# PKCE enforcement
OIDC_REQUIRE_PKCE=true                   # Require PKCE for all clients

# CORS settings
OIDC_ALLOWED_ORIGINS=http://localhost:3000,https://app.example.com

# Rate limiting
OIDC_RATE_LIMIT_ENABLED=true
OIDC_RATE_LIMIT_REQUESTS=100
OIDC_RATE_LIMIT_WINDOW=900               # 15 minutes in seconds
```

---

## Crypto Key Generation

The `OIDC_CRYPTO_KEY` is used to encrypt RSA private keys before storing them in the database.

### Generate a Secure Crypto Key

**Option 1: Using OpenSSL**
```bash
openssl rand -base64 32
```

**Option 2: Using Python**
```python
import secrets
print(secrets.token_urlsafe(32))
```

**Option 3: Using Node.js**
```javascript
crypto.randomBytes(32).toString('base64')
```

### Important Security Notes

1. **Never commit the crypto key to version control**
2. **Use different keys for development, staging, and production**
3. **Store the key in a secure secret management system** (AWS Secrets Manager, HashiCorp Vault, etc.)
4. **Rotate the key periodically** (see Key Rotation section below)

### Key Rotation

When rotating the crypto key:

1. Generate a new crypto key
2. Decrypt existing signing keys with old crypto key
3. Re-encrypt with new crypto key
4. Update `OIDC_CRYPTO_KEY` in environment
5. Restart the application

**Migration script example:**
```bash
#!/bin/bash

OLD_KEY="old-crypto-key"
NEW_KEY="new-crypto-key"

# Export signing keys
psql -d iota_erp -c "COPY (SELECT id, key_id, algorithm, private_key, public_key FROM oidc_signing_keys WHERE is_active = true) TO '/tmp/keys_backup.csv' CSV HEADER;"

# Run migration (implement this in your application)
./migrate-crypto-key --old-key="$OLD_KEY" --new-key="$NEW_KEY"

# Verify
./verify-keys --crypto-key="$NEW_KEY"
```

---

## Token Lifetime Configuration

### Default Token Lifetimes

| Token Type | Default Lifetime | Recommended Range |
|------------|------------------|-------------------|
| Access Token | 1 hour | 15 minutes - 1 hour |
| ID Token | 1 hour | 15 minutes - 1 hour |
| Refresh Token | 30 days | 7 days - 90 days |
| Authorization Code | 5 minutes | 1 minute - 10 minutes |

### Per-Client Token Lifetimes

You can configure custom lifetimes for specific clients:

```sql
-- Set custom token lifetimes for a client
UPDATE oidc_clients
SET
    access_token_lifetime = INTERVAL '30 minutes',
    id_token_lifetime = INTERVAL '30 minutes',
    refresh_token_lifetime = INTERVAL '7 days'
WHERE client_id = 'my-client-id';
```

### Lifetime Tuning Guidelines

**Short-lived Access Tokens (15-30 minutes):**
- ✅ Better security (reduced window for token theft)
- ✅ Encourages token refresh
- ❌ More frequent token refreshes
- ❌ Higher load on token endpoint

**Long-lived Access Tokens (2-24 hours):**
- ✅ Fewer token refreshes
- ✅ Better performance
- ❌ Longer vulnerability window
- ❌ Not recommended for sensitive applications

**Refresh Token Lifetime:**
- Mobile apps: 30-90 days
- Web apps: 7-30 days
- Backend services: No refresh tokens (use client credentials)

---

## Multi-Tenant Considerations

### Tenant Isolation

OIDC module respects multi-tenant architecture:

1. **Clients are global** (not tenant-specific)
2. **Auth requests include tenant_id** after user authentication
3. **Refresh tokens are scoped to user + tenant + client**
4. **User info returns data for authenticated tenant only**

### Per-Tenant Configuration

Configure OIDC settings per tenant:

```sql
-- Create tenant-specific settings table
CREATE TABLE tenant_oidc_settings (
    tenant_id UUID PRIMARY KEY REFERENCES tenants(id),
    issuer_override TEXT,
    logo_url TEXT,
    terms_url TEXT,
    privacy_url TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Set custom issuer for tenant
INSERT INTO tenant_oidc_settings (tenant_id, issuer_override)
VALUES ('tenant-uuid', 'https://tenant.example.com');
```

### Multi-Tenant Flow

```
┌─────────────┐
│   Client    │
│ (Global)    │
└──────┬──────┘
       │
       │ 1. Authorization Request
       ▼
┌─────────────┐
│   User      │
│  Login      │────── 2. User selects tenant
└──────┬──────┘
       │
       │ 3. Authenticate
       ▼
┌─────────────┐
│  Auth Code  │────── 4. Code includes tenant_id
└──────┬──────┘
       │
       │ 5. Token Exchange
       ▼
┌─────────────┐
│   Tokens    │────── 6. Tokens scoped to tenant
└─────────────┘
```

---

## Security Best Practices

### 1. Client Configuration

**Public Clients (SPAs, Mobile Apps):**
```sql
INSERT INTO oidc_clients (client_id, name, application_type, redirect_uris, require_pkce)
VALUES (
    'spa-client',
    'My SPA Application',
    'spa',
    ARRAY['http://localhost:3000/callback'],
    true  -- ALWAYS true for public clients
);
```

**Confidential Clients (Backend Services):**
```sql
INSERT INTO oidc_clients (
    client_id,
    name,
    application_type,
    redirect_uris,
    client_secret_hash,
    require_pkce
)
VALUES (
    'backend-client',
    'My Backend Service',
    'web',
    ARRAY['https://api.example.com/callback'],
    '$2a$10$hashed_secret_here',  -- Use bcrypt
    false  -- Optional for confidential clients
);
```

### 2. Redirect URI Validation

**Strict Validation Rules:**
- ✅ Exact match required (no wildcards)
- ✅ HTTPS required in production
- ✅ Localhost allowed only in development
- ❌ Do not use `http://` in production
- ❌ Do not use wildcard subdomains

**Example:**
```sql
-- ✅ Good: Exact URLs
redirect_uris = ARRAY['https://app.example.com/callback']

-- ❌ Bad: Wildcards (not supported)
redirect_uris = ARRAY['https://*.example.com/callback']

-- ✅ Development only
redirect_uris = ARRAY['http://localhost:3000/callback']
```

### 3. Scope Configuration

**Recommended Scopes:**

| Scope | Description | Required? |
|-------|-------------|-----------|
| `openid` | OpenID Connect authentication | Yes (always) |
| `profile` | User profile info (name, picture) | Recommended |
| `email` | User email address | Recommended |
| `offline_access` | Refresh token | Optional |

**Custom Scopes:**
```sql
-- Add custom scopes for specific clients
UPDATE oidc_clients
SET scopes = ARRAY['openid', 'profile', 'email', 'custom:read', 'custom:write']
WHERE client_id = 'my-client';
```

### 4. Rate Limiting

Protect endpoints from abuse:

```bash
# In .env
OIDC_RATE_LIMIT_ENABLED=true

# Per-endpoint limits
OIDC_RATE_LIMIT_AUTHORIZE=100    # per 15 min per IP
OIDC_RATE_LIMIT_TOKEN=50         # per 15 min per client
OIDC_RATE_LIMIT_USERINFO=200     # per 15 min per token
```

### 5. CORS Configuration

Configure CORS for browser-based clients:

```bash
# Allow specific origins
OIDC_ALLOWED_ORIGINS=https://app.example.com,https://admin.example.com

# DO NOT use wildcard in production
OIDC_ALLOWED_ORIGINS=*  # ❌ INSECURE
```

### 6. Token Storage

**Client-side storage guidelines:**

| Storage Type | Access Tokens | Refresh Tokens | ID Tokens |
|-------------|---------------|----------------|-----------|
| Memory | ✅ Best | ❌ No | ✅ OK |
| LocalStorage | ❌ XSS risk | ❌ Never | ⚠️ Low risk |
| SessionStorage | ⚠️ OK | ❌ Never | ⚠️ OK |
| HttpOnly Cookie | ✅ Best | ✅ Best | ✅ OK |
| Secure Cookie | ✅ Best | ✅ Best | ✅ OK |

**Recommended approach:**
```javascript
// Store tokens in httpOnly, secure cookies (backend sets them)
// OR use BFF pattern with session cookies
```

### 7. Monitoring and Logging

**Key metrics to monitor:**
- Failed authentication attempts
- Token refresh failures
- Invalid client credentials
- Unusual geographic locations
- High-frequency token requests

**Logging example:**
```go
log.Info("OIDC authorization success",
    "client_id", clientID,
    "user_id", userID,
    "tenant_id", tenantID,
    "scopes", scopes,
    "ip", clientIP,
)

log.Warn("OIDC authentication failed",
    "client_id", clientID,
    "reason", "invalid_credentials",
    "ip", clientIP,
    "attempt_count", attemptCount,
)
```

### 8. Client Secret Management

**Generate strong client secrets:**
```bash
# 64-character random secret
openssl rand -hex 32
```

**Hash secrets before storage:**
```go
import "golang.org/x/crypto/bcrypt"

hashedSecret, _ := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
```

**Rotate secrets periodically:**
1. Generate new secret
2. Update client with new secret
3. Update all applications using the client
4. Verify all applications work with new secret
5. Revoke old secret after grace period

---

## Production Checklist

Before deploying to production:

- [ ] `OIDC_CRYPTO_KEY` is secure and stored in secret manager
- [ ] All redirect URIs use HTTPS (no http://)
- [ ] PKCE is required for public clients
- [ ] Token lifetimes are configured appropriately
- [ ] Rate limiting is enabled
- [ ] CORS is configured with specific origins (no wildcard)
- [ ] Client secrets are hashed with bcrypt
- [ ] Monitoring and alerting are configured
- [ ] Backup and recovery procedures are documented
- [ ] Key rotation procedure is documented
- [ ] Security audit has been performed

---

## Troubleshooting

### Common Issues

**Issue: "invalid_client" error**
- Check client_id and client_secret are correct
- Verify client exists in database
- Ensure client is active (`is_active = true`)

**Issue: "invalid_redirect_uri" error**
- Redirect URI must exactly match registered URI
- Check for trailing slashes
- Verify protocol (http vs https)

**Issue: "invalid_grant" error**
- Authorization code may have expired (5 min default)
- Code may have already been used (one-time use)
- Verify code_verifier matches code_challenge (PKCE)

**Issue: Signing key errors**
- Bootstrap keys: `make db migrate up`
- Verify OIDC_CRYPTO_KEY is set correctly
- Check database connectivity

**Issue: Token refresh fails**
- Verify refresh token has not expired
- Check client_id matches original authorization
- Ensure refresh token hasn't been revoked

---

## Additional Resources

- [OAuth 2.0 RFC 6749](https://datatracker.ietf.org/doc/html/rfc6749)
- [OpenID Connect Core 1.0](https://openid.net/specs/openid-connect-core-1_0.html)
- [PKCE RFC 7636](https://datatracker.ietf.org/doc/html/rfc7636)
- [OAuth 2.0 Security Best Practices](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-security-topics)
