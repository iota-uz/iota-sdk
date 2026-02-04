# OIDC Module

OpenID Connect (OIDC) authentication and authorization module for IOTA SDK.

## Features

- ✅ **OpenID Connect Core 1.0** compliant
- ✅ **OAuth 2.0** authorization framework
- ✅ **PKCE** (Proof Key for Code Exchange) support
- ✅ **Multi-tenant** architecture
- ✅ **RSA signing keys** with encryption at rest
- ✅ **Refresh tokens** with rotation
- ✅ **Token revocation** support
- ✅ **User info endpoint**
- ✅ **JWKS** (JSON Web Key Set) endpoint
- ✅ **Discovery endpoint** (.well-known/openid-configuration)

## Quick Start

### 1. Run Migrations

```bash
make db migrate up
```

This creates the following tables:
- `oidc_clients` - OAuth2/OIDC client applications
- `oidc_auth_requests` - Authorization requests (temporary storage)
- `oidc_refresh_tokens` - Refresh tokens
- `oidc_signing_keys` - RSA signing keys (encrypted)

### 2. Configure Environment

```bash
# .env
OIDC_ISSUER_URL=https://your-domain.com
OIDC_CRYPTO_KEY=your-secure-random-32-char-minimum-key-here
```

Generate crypto key:
```bash
openssl rand -base64 32
```

### 3. Bootstrap Signing Keys

Start the application (keys will be auto-generated):
```bash
make run
```

### 4. Register a Client

```sql
INSERT INTO oidc_clients (
    client_id,
    name,
    application_type,
    redirect_uris,
    require_pkce
) VALUES (
    'my-client-id',
    'My Application',
    'web',
    ARRAY['http://localhost:3000/callback'],
    true
);
```

Or use the example script:
```bash
psql -d iota_erp -f modules/oidc/examples/client_setup.sql
```

### 5. Test OIDC Flow

Use the provided shell script to test the complete flow:
```bash
bash modules/oidc/examples/curl_flow.sh
```

## Documentation

Comprehensive documentation is available in the main SDK docs:

- **[OIDC Overview](https://iota-sdk.dev/oidc)** - Module overview and quick start
- **[API Reference](https://iota-sdk.dev/oidc/api)** - Complete API reference for all OIDC endpoints
- **[Configuration](https://iota-sdk.dev/oidc/configuration)** - Environment variables, token lifetimes, security best practices
- **[Migration Guide](https://iota-sdk.dev/oidc/migration)** - Step-by-step guide to enable OIDC on existing installation

## Examples

Example code and scripts are available in the `examples/` directory:

- **[client_setup.sql](examples/client_setup.sql)** - SQL examples for creating OIDC clients
- **[curl_flow.sh](examples/curl_flow.sh)** - Complete OIDC authorization code flow with curl

## Architecture

```
modules/oidc/
├── domain/
│   └── entities/
│       ├── client/          # OAuth2 client entity
│       ├── authrequest/     # Authorization request entity
│       └── token/           # Refresh token entity
├── services/
│   └── oidc_service.go      # Business logic layer
├── infrastructure/
│   ├── persistence/         # Repository implementations
│   └── oidc/
│       ├── keys.go          # RSA key management
│       └── storage.go       # OP storage implementation
├── presentation/
│   └── controllers/
│       └── oidc_controller.go  # HTTP handlers
├── docs/                    # Documentation
└── examples/                # Example code and scripts
```

## Testing

The module includes comprehensive test coverage:

### Repository Tests
- `infrastructure/persistence/client_repository_test.go`
- `infrastructure/persistence/authrequest_repository_test.go`
- `infrastructure/persistence/token_repository_test.go`

Tests cover:
- CRUD operations
- Uniqueness constraints
- Expiration cleanup
- Error cases
- Pagination and filtering

### Entity Tests
- `domain/entities/client/client_test.go`
- `domain/entities/authrequest/authrequest_test.go`
- `domain/entities/token/token_test.go`

Tests cover:
- Business logic validation
- Immutable setters
- Functional options
- Expiration checks

### Service Tests
- `services/oidc_service_test.go`

Tests cover:
- CompleteAuthRequest (success and expired)
- GetAuthRequest (valid and invalid)
- Edge cases and error handling

### Keys Management Tests
- `infrastructure/oidc/keys_test.go`

Tests cover:
- Key bootstrap
- Key retrieval and decryption
- AES encryption/decryption roundtrip
- Concurrent key access
- Error cases

### Run Tests

```bash
# Run all OIDC tests
go test ./modules/oidc/... -v

# Run specific test suite
go test ./modules/oidc/infrastructure/persistence -v
go test ./modules/oidc/domain/entities/client -v
go test ./modules/oidc/services -v

# Run with coverage
go test ./modules/oidc/... -cover

# Run specific test
go test ./modules/oidc/infrastructure/persistence -run TestClientRepository_Create -v
```

## Supported Flows

### Authorization Code Flow

Standard OAuth 2.0 authorization code flow with optional PKCE:

1. Client initiates authorization request
2. User authenticates and consents
3. Authorization code is issued
4. Client exchanges code for tokens
5. Client uses access token to access resources

### Refresh Token Flow

Obtain new access tokens using refresh tokens:

1. Client sends refresh token to token endpoint
2. Server validates refresh token
3. New access token is issued
4. Optional: New refresh token is issued (rotation)

### Client Credentials Flow

For machine-to-machine communication (future implementation).

## Endpoints

- `GET /.well-known/openid-configuration` - Discovery endpoint
- `GET /oidc/authorize` - Authorization endpoint
- `POST /oidc/token` - Token endpoint
- `GET /oidc/userinfo` - User info endpoint
- `GET /oidc/keys` - JWKS endpoint
- `POST /oidc/revoke` - Token revocation endpoint

See [API Reference](https://iota-sdk.dev/oidc/api) for detailed endpoint documentation.

## Security

### Key Management

- RSA signing keys are generated automatically on first run
- Private keys are encrypted using AES-256-GCM before storage
- Crypto key is derived from `OIDC_CRYPTO_KEY` environment variable using SHA-256
- Keys are stored in the `oidc_signing_keys` table

### Token Security

- Access tokens are short-lived (default: 1 hour)
- Refresh tokens are long-lived (default: 30 days)
- Authorization codes expire quickly (default: 5 minutes)
- All tokens are JWTs signed with RS256
- Refresh tokens are hashed (SHA-256) before storage

### Client Security

- Client secrets are hashed using bcrypt
- PKCE is required for public clients
- Redirect URIs must exactly match (no wildcards)
- HTTPS required in production

See [Configuration Guide](https://iota-sdk.dev/oidc/configuration) for security best practices.

## Multi-Tenant Support

The OIDC module fully supports IOTA SDK's multi-tenant architecture:

- Clients are global (shared across tenants)
- Auth requests are scoped to tenant after user authentication
- Refresh tokens are scoped to user + tenant + client combination
- Token claims include `tenant_id`
- User info returns data for authenticated tenant only

## Configuration

### Token Lifetimes

Configure token lifetimes via environment variables:

```bash
OIDC_ACCESS_TOKEN_LIFETIME=3600          # 1 hour (seconds)
OIDC_ID_TOKEN_LIFETIME=3600              # 1 hour (seconds)
OIDC_REFRESH_TOKEN_LIFETIME=2592000      # 30 days (seconds)
OIDC_AUTH_CODE_LIFETIME=300              # 5 minutes (seconds)
```

Or per-client via database:

```sql
UPDATE oidc_clients
SET
    access_token_lifetime = INTERVAL '30 minutes',
    id_token_lifetime = INTERVAL '30 minutes',
    refresh_token_lifetime = INTERVAL '7 days'
WHERE client_id = 'my-client-id';
```

### PKCE

PKCE (Proof Key for Code Exchange) is recommended for all clients and required for public clients:

```sql
-- Require PKCE for client
UPDATE oidc_clients
SET require_pkce = true
WHERE client_id = 'my-client-id';
```

See [Configuration Guide](https://iota-sdk.dev/oidc/configuration) for complete configuration guide.

## Troubleshooting

### Signing Keys Not Generated

```bash
# Check if keys exist
psql -d iota_erp -c "SELECT COUNT(*) FROM oidc_signing_keys;"

# If count is 0, restart the application
make restart
```

### Token Decryption Fails

Verify `OIDC_CRYPTO_KEY` environment variable matches the key used during key generation.

### Invalid Redirect URI

Redirect URI must exactly match the registered URI (case-sensitive, including trailing slashes).

```sql
-- Verify redirect URIs
SELECT client_id, redirect_uris FROM oidc_clients WHERE client_id = 'my-client';
```

See [Configuration Guide](https://iota-sdk.dev/oidc/configuration#troubleshooting) for more troubleshooting tips.

## Development

### Project Structure

The OIDC module follows IOTA SDK's DDD architecture:

- **Domain Layer**: Pure business logic (entities, value objects, interfaces)
- **Service Layer**: Application logic and orchestration
- **Infrastructure Layer**: Data persistence and external integrations
- **Presentation Layer**: HTTP controllers and templates

### Adding New Features

1. Define domain entities in `domain/entities/`
2. Add repository interfaces to domain layer
3. Implement repositories in `infrastructure/persistence/`
4. Add business logic in `services/`
5. Create HTTP handlers in `presentation/controllers/`
6. Write tests for all layers
7. Update documentation

### Testing Guidelines

- Use ITF (IOTA Test Framework) for all tests
- Include `t.Parallel()` for test isolation
- Test happy path, error cases, edge cases, and permissions
- Aim for >80% test coverage on critical paths
- Follow existing test patterns in `*_test.go` files

## License

This module is part of IOTA SDK and follows the same license.

## Support

For issues, questions, or contributions:

1. Check the documentation in `docs/`
2. Review test files for usage examples
3. Open an issue on GitHub
4. Contact IOTA SDK support team

## Contributing

Contributions are welcome! Please:

1. Follow the existing code structure and patterns
2. Write comprehensive tests for new features
3. Update documentation
4. Follow Go best practices and SDK conventions
5. Ensure all tests pass before submitting PR

## Changelog

### v1.0.0 (Initial Release)

- ✅ OpenID Connect Core 1.0 implementation
- ✅ Authorization code flow with PKCE
- ✅ Refresh token support
- ✅ Token revocation
- ✅ Multi-tenant support
- ✅ RSA key management with encryption
- ✅ Comprehensive test suite
- ✅ Complete documentation

## Roadmap

Future enhancements:

- [ ] Client credentials grant type
- [ ] Device authorization grant
- [ ] JWT bearer token grant
- [ ] Dynamic client registration
- [ ] Key rotation automation
- [ ] Admin UI for client management
- [ ] Enhanced monitoring and metrics
- [ ] Rate limiting per client
- [ ] Advanced claims customization
