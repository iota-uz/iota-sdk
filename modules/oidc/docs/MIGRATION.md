# OIDC Migration Guide

This guide explains how to enable and configure OIDC on an existing IOTA SDK installation.

## Prerequisites

- IOTA SDK installed and running
- PostgreSQL 13+ database
- Go 1.23.2 or higher
- Access to database migration tools

---

## Step 1: Run Database Migrations

The OIDC module requires several database tables. Run migrations to create them:

```bash
# Apply all migrations (includes OIDC tables)
make db migrate up

# Or apply specific OIDC migration
sql-migrate up -limit=1 migrations/XXXXXX_create_oidc_tables.sql
```

**Migrationfiles created:**
- `oidc_clients` - OAuth2/OIDC client applications
- `oidc_auth_requests` - Authorization requests (temporary)
- `oidc_refresh_tokens` - Refresh tokens
- `oidc_signing_keys` - RSA signing keys (encrypted)

**Verify migrations:**
```bash
psql -d iota_erp -c "\dt oidc_*"
```

Expected output:
```
               List of relations
 Schema |        Name          | Type  |  Owner
--------+----------------------+-------+----------
 public | oidc_auth_requests   | table | postgres
 public | oidc_clients         | table | postgres
 public | oidc_refresh_tokens  | table | postgres
 public | oidc_signing_keys    | table | postgres
```

---

## Step 2: Configure Environment Variables

Add OIDC configuration to your `.env` file:

```bash
# Required: Crypto key for encrypting signing keys
OIDC_CRYPTO_KEY=your-secure-random-32-char-minimum-key-here

# Required: Issuer URL (must match your public domain)
OIDC_ISSUER=https://your-domain.com

# Optional: Token lifetimes (defaults shown)
OIDC_ACCESS_TOKEN_LIFETIME=3600          # 1 hour
OIDC_ID_TOKEN_LIFETIME=3600              # 1 hour
OIDC_REFRESH_TOKEN_LIFETIME=2592000      # 30 days
```

**Generate crypto key:**
```bash
openssl rand -base64 32
```

---

## Step 3: Bootstrap Signing Keys

The OIDC module uses RSA keys to sign tokens. Bootstrap the initial keypair:

```bash
# Start your application (keys will be auto-generated on first run)
make run

# Or manually trigger key generation
psql -d iota_erp -c "SELECT COUNT(*) FROM oidc_signing_keys;"
# If count is 0, restart the application to trigger bootstrap
```

**Verify keys were created:**
```bash
psql -d iota_erp -c "SELECT key_id, algorithm, is_active, created_at FROM oidc_signing_keys;"
```

Expected output:
```
              key_id               | algorithm | is_active |       created_at
-----------------------------------+-----------+-----------+-------------------------
 550e8400-e29b-41d4-a716-446655440000 | RS256     | t         | 2026-01-31 12:00:00
```

---

## Step 4: Register Your First Client

Create an OAuth2/OIDC client for your application:

### Option 1: Using SQL

```sql
INSERT INTO oidc_clients (
    client_id,
    name,
    application_type,
    redirect_uris,
    grant_types,
    response_types,
    scopes,
    require_pkce,
    is_active
) VALUES (
    'my-first-client',                                    -- Unique client ID
    'My Application',                                      -- Display name
    'web',                                                -- 'web' or 'spa'
    ARRAY['http://localhost:3000/callback'],              -- Allowed redirect URIs
    ARRAY['authorization_code', 'refresh_token'],         -- Grant types
    ARRAY['code'],                                        -- Response types
    ARRAY['openid', 'profile', 'email', 'offline_access'], -- Allowed scopes
    true,                                                 -- Require PKCE (recommended)
    true                                                  -- Active
);
```

### Option 2: Using Go Code

```go
package main

import (
    "context"
    "github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/client"
    "github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/persistence"
)

func main() {
    ctx := context.Background()

    // Create client repository
    clientRepo := persistence.NewClientRepository()

    // Create new client
    newClient := client.New(
        "my-first-client",
        "My Application",
        "web",
        []string{"http://localhost:3000/callback"},
        client.WithGrantTypes([]string{"authorization_code", "refresh_token"}),
        client.WithScopes([]string{"openid", "profile", "email", "offline_access"}),
        client.WithRequirePKCE(true),
    )

    // Save to database
    createdClient, err := clientRepo.Create(ctx, newClient)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Client created: %s\n", createdClient.ClientID())
}
```

### Option 3: Using Example Script

```bash
# See modules/oidc/examples/client_setup.sql
psql -d iota_erp -f modules/oidc/examples/client_setup.sql
```

---

## Step 5: Test OIDC Flow

Test the complete OIDC authorization code flow:

### Manual Testing

1. **Start Authorization Flow:**

Visit in your browser:
```
http://localhost:8080/oidc/authorize?client_id=my-first-client&redirect_uri=http://localhost:3000/callback&response_type=code&scope=openid%20profile%20email&state=random-state-123
```

2. **Login as User:**

You'll be redirected to the login page. Login with your credentials.

3. **Get Authorization Code:**

After successful login, you'll be redirected to:
```
http://localhost:3000/callback?code=AUTH_CODE_HERE&state=random-state-123
```

4. **Exchange Code for Tokens:**

```bash
curl -X POST http://localhost:8080/oidc/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=authorization_code" \
  -d "client_id=my-first-client" \
  -d "code=AUTH_CODE_HERE" \
  -d "redirect_uri=http://localhost:3000/callback"
```

Expected response:
```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
  "id_token": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
  "scope": "openid profile email"
}
```

5. **Get User Info:**

```bash
curl -X GET http://localhost:8080/oidc/userinfo \
  -H "Authorization: Bearer ACCESS_TOKEN_HERE"
```

### Automated Testing

Use the provided shell script:

```bash
# See modules/oidc/examples/curl_flow.sh
bash modules/oidc/examples/curl_flow.sh
```

---

## Step 6: Update Your Application

### Frontend Integration

**Example (JavaScript):**

```javascript
// Authorization
const authorizeUrl = new URL('http://localhost:8080/oidc/authorize');
authorizeUrl.searchParams.set('client_id', 'my-first-client');
authorizeUrl.searchParams.set('redirect_uri', 'http://localhost:3000/callback');
authorizeUrl.searchParams.set('response_type', 'code');
authorizeUrl.searchParams.set('scope', 'openid profile email');
authorizeUrl.searchParams.set('state', generateRandomState());

window.location.href = authorizeUrl.toString();

// Handle callback
const urlParams = new URLSearchParams(window.location.search);
const code = urlParams.get('code');
const state = urlParams.get('state');

// Exchange code for tokens (should be done server-side)
fetch('http://localhost:8080/oidc/token', {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams({
        grant_type: 'authorization_code',
        client_id: 'my-first-client',
        code: code,
        redirect_uri: 'http://localhost:3000/callback'
    })
});
```

### Backend Integration

**Example (Go):**

```go
package main

import (
    "net/http"
    "github.com/gorilla/sessions"
)

func handleCallback(w http.ResponseWriter, r *http.Request) {
    code := r.URL.Query().Get("code")
    state := r.URL.Query().Get("state")

    // Verify state matches session

    // Exchange code for tokens
    tokens, err := exchangeCodeForTokens(code)
    if err != nil {
        http.Error(w, "Token exchange failed", http.StatusInternalServerError)
        return
    }

    // Store tokens in session
    session.Values["access_token"] = tokens.AccessToken
    session.Values["refresh_token"] = tokens.RefreshToken
    session.Save(r, w)

    http.Redirect(w, r, "/dashboard", http.StatusFound)
}
```

---

## Step 7: Production Deployment

Before deploying to production:

### 1. Update Environment

```bash
# Production .env
OIDC_ISSUER=https://your-production-domain.com
OIDC_CRYPTO_KEY=<new-secure-key-from-secret-manager>

# Use HTTPS redirect URIs only
```

### 2. Update Client Redirect URIs

```sql
UPDATE oidc_clients
SET redirect_uris = ARRAY['https://app.your-domain.com/callback']
WHERE client_id = 'my-first-client';
```

### 3. Enable Rate Limiting

```bash
# In .env
OIDC_RATE_LIMIT_ENABLED=true
OIDC_RATE_LIMIT_AUTHORIZE=100
OIDC_RATE_LIMIT_TOKEN=50
OIDC_RATE_LIMIT_USERINFO=200
```

### 4. Configure CORS

```bash
OIDC_ALLOWED_ORIGINS=https://app.your-domain.com,https://admin.your-domain.com
```

### 5. Setup Monitoring

Monitor these metrics:
- Failed authentication attempts
- Token refresh failures
- Invalid client errors
- High-frequency requests

### 6. Backup Signing Keys

```bash
# Backup encrypted keys
pg_dump -d iota_erp -t oidc_signing_keys > oidc_keys_backup.sql

# Store backup securely
aws s3 cp oidc_keys_backup.sql s3://your-backup-bucket/oidc/
```

---

## Rollback Procedure

If you need to rollback the OIDC migration:

### 1. Disable OIDC Routes

Comment out OIDC routes in your router configuration.

### 2. Rollback Database

```bash
# Rollback OIDC migration
sql-migrate down -limit=1 migrations/XXXXXX_create_oidc_tables.sql
```

### 3. Remove Environment Variables

Remove OIDC-related variables from `.env`.

### 4. Restart Application

```bash
make restart
```

---

## Common Migration Issues

### Issue: Signing Keys Not Generated

**Symptom:** `SELECT COUNT(*) FROM oidc_signing_keys` returns 0

**Solution:**
```bash
# Manually trigger bootstrap
go run scripts/bootstrap_oidc_keys.go

# Or restart application
make restart
```

### Issue: Client Creation Fails

**Symptom:** `pq: duplicate key value violates unique constraint "oidc_clients_client_id_key"`

**Solution:**
```sql
-- Check if client already exists
SELECT client_id, name FROM oidc_clients WHERE client_id = 'my-first-client';

-- Delete existing client if needed
DELETE FROM oidc_clients WHERE client_id = 'my-first-client';
```

### Issue: Invalid Redirect URI

**Symptom:** `invalid_redirect_uri` error during authorization

**Solution:**
```sql
-- Verify redirect URIs
SELECT client_id, redirect_uris FROM oidc_clients WHERE client_id = 'my-first-client';

-- Update redirect URIs (exact match required)
UPDATE oidc_clients
SET redirect_uris = ARRAY['http://localhost:3000/callback']
WHERE client_id = 'my-first-client';
```

### Issue: Token Decryption Fails

**Symptom:** `failed to decrypt private key`

**Solution:**
- Verify `OIDC_CRYPTO_KEY` matches the key used during key generation
- Check environment variable is loaded correctly
- Re-bootstrap keys with correct crypto key

---

## Next Steps

After successful migration:

1. Review [API Documentation](API.md) for endpoint details
2. Review [Configuration Guide](CONFIGURATION.md) for advanced settings
3. Implement token refresh in your application
4. Setup monitoring and alerting
5. Conduct security audit
6. Train your team on OIDC flows

---

## Support

For issues or questions:

1. Check [Troubleshooting](CONFIGURATION.md#troubleshooting) section
2. Review [test files](../infrastructure/persistence/*_test.go) for examples
3. Open an issue on GitHub
4. Contact IOTA SDK support team
