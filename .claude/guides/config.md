# Configuration Management Guide

**Environment configuration, build system, docker, and project documentation for IOTA SDK.**

## Overview

Configuration in IOTA SDK includes:
- **Environment files**: `.env`, `.env.example`
- **Docker configs**: `compose*.yml`
- **Build system**: `Makefile`
- **Frontend configs**: `tailwind.config.js`, `tsconfig*.json`
- **Documentation**: `README.md`, `docs/`

## Environment Configuration

### File Structure

```
.env                 # Local development (gitignored)
.env.example         # Template (committed)
e2e/.env.e2e        # E2E testing environment
```

### .env.example Template

```bash
# Core Application
LOG_LEVEL=debug
SESSION_DURATION=720h
DOMAIN=localhost
GO_APP_ENV=dev

# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_NAME=iota_erp
DB_USER=postgres
DB_PASSWORD=postgres

# Authentication
GOOGLE_CLIENT_ID=example-client-id
GOOGLE_CLIENT_SECRET=example-client-secret
GOOGLE_REDIRECT_URL=http://localhost:3000/auth/google/callback

# Payments
STRIPE_SECRET_KEY=sk_test_...
STRIPE_PUBLISHABLE_KEY=pk_test_...
STRIPE_WEBHOOK_SECRET=whsec_...
```

### When to Update Environment Files

**Add new variables when**:
- New service integrations (Stripe, OAuth providers)
- Database configuration changes
- New feature flags
- Port or host changes
- Authentication updates
- Testing environment modifications

**Process**:
1. Add to `.env.example` with example value
2. Add to local `.env` with real value
3. Document in README.md if user-facing
4. Update docker-compose if needed

## Docker Configuration

### File Structure

```
compose.dev.yml      # Development environment
compose.yml          # Production configuration
compose.testing.yml  # E2E testing environment
```

### Development Configuration

**compose.dev.yml**:

```yaml
services:
  app:
    build: .
    ports:
      - "3000:3000"
    environment:
      - DB_HOST=db
      - DB_PORT=5432
    depends_on:
      - db
    volumes:
      - .:/app

  db:
    image: postgres:13
    environment:
      POSTGRES_DB: iota_erp
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

volumes:
  postgres_data:
```

### When to Update Docker Configs

**Update when**:
- New services added (Redis, message queues)
- Database version upgrades
- Service dependency changes
- Port conflicts or network updates
- Volume mount requirements change
- Environment variable propagation needs

### Docker Commands

```bash
# Start all services
make compose up

# Stop all services
make compose down

# Restart services
make compose restart

# View logs
make compose logs

# Build images
make build docker-base
make build docker-prod
```

## Build System (Makefile)

### Core Targets

**Code Quality**:
```makefile
make fix fmt       # Format Go code and templates
make fix imports   # Organize and format imports
make check lint    # Check unused variables/functions
make check tr      # Validate translation consistency
```

**Building**:
```makefile
make build local       # Build for local OS
make build linux       # Build for Linux (production)
make build docker-base # Build Docker base image
make build docker-prod # Build Docker production image
```

**Testing**:
```makefile
make test                   # Run all tests
make test coverage          # Run tests with coverage
make test detailed-coverage # Detailed coverage analysis
make test verbose           # Verbose output
make test failures          # Show only failing tests
```

**Database**:
```makefile
make db migrate up    # Apply migrations
make db migrate down  # Rollback migrations
make db migrate status # Check migration status
make db reset         # Reset development database
```

**Templates & Assets**:
```makefile
make generate       # Generate templ templates
make generate watch # Watch and regenerate templates
make css            # Compile CSS with minification
make css watch      # Watch and recompile CSS
make css dev        # Compile without minification
make css clean      # Clean CSS artifacts
```

### When to Update Makefile

**Add new targets when**:
- New build steps required (CSS frameworks, asset compilation)
- Database migration commands change
- Testing infrastructure updates
- Docker workflow modifications
- Code generation steps added
- New linting or formatting tools

### Makefile Best Practices

**Prevent target conflicts**:

```makefile
# For multi-word commands (e.g., 'make e2e test')
# Add early exit check to prevent conflicts
test:
	@if [ "$(word 1,$(MAKECMDGOALS))" != "test" ]; then \
		exit 0; \
	fi
	# Rest of target logic...

.PHONY: test
```

**Test with dry-run**:

```bash
make target-name --dry-run
```

## Frontend Build Configuration

### Tailwind CSS

**tailwind.config.js**:

```javascript
module.exports = {
  content: [
    "./modules/**/*.{html,js,ts,templ}",
    "../iota-sdk/components/**/*.{html,js,templ}",
  ],
  theme: {
    extend: {
      colors: {
        primary: { /* OKLCH colors */ },
        surface: { /* Layer hierarchy */ },
      },
      fontFamily: {
        sans: ['Gilroy', 'system-ui'],
      },
    },
  },
};
```

**When to update**:
- Design system changes (colors, fonts, spacing)
- New UI components or patterns
- Content paths change (new modules, components)

### TypeScript Configuration

**tsconfig.json**:

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "commonjs",
    "strict": true,
    "esModuleInterop": true
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules"]
}
```

**When to update**:
- TypeScript compilation requirements change
- New source directories added
- Compiler options need adjustment

## Documentation

### README.md

**Primary entry point** for the project:

```markdown
# IOTA SDK

Business management platform for multi-tenant applications.

## Quick Start

\`\`\`bash
# Clone repository
git clone https://github.com/iota-uz/iota-sdk

# Install dependencies
make deps

# Start development environment
make compose up

# Run migrations
make db migrate up

# Start application
make run
\`\`\`

## Documentation

- [Architecture](docs/ARCHITECTURE.md)
- [Contributing](docs/CONTRIBUTING.md)
- [API Documentation](docs/API.md)
```

**When to update**:
- Setup/installation process changes
- New features or modules added
- Deployment procedures update
- Technology stack updates

### docs/ Directory

```
docs/
├── ARCHITECTURE.md    # System architecture
├── CONTRIBUTING.md    # Contribution guidelines
├── API.md            # API documentation
├── DEPLOYMENT.md     # Deployment procedures
└── LLMS.md          # Auto-generated code docs
```

**Generate LLMS.md**:

```bash
make docs
```

**When to update**:
- After major code structure changes
- New modules or packages added
- Public API changes
- Before releases or major deployments

### Package Documentation

**pkg/*/README.md**:

```markdown
# Package Name

Brief description of package purpose.

## Usage

\`\`\`go
import "github.com/iota-uz/iota-sdk/pkg/packagename"

// Example usage
result := packagename.Function()
\`\`\`

## API Reference

See [pkg.go.dev](https://pkg.go.dev/github.com/iota-uz/iota-sdk/pkg/packagename)
```

## Configuration Validation

### Consistency Checks

**Before committing**:

```bash
# Check environment configuration alignment
diff .env .env.example | grep "^<" # Should only show sensitive values

# Validate docker-compose
docker compose -f compose.dev.yml config

# Test Makefile targets
make target-name --dry-run

# Verify documentation builds
make docs
```

### Security Validation

**Never commit**:
- Secrets in `.env.example`
- Real credentials in any config
- Internal URLs in public docs
- Sensitive architecture details

**Ensure**:
- `.env` in `.gitignore`
- Secure defaults for all services
- Access controls documented
- Example configs use placeholders

## Common Configuration Issues

### Environment Configuration Drift

**Problem**: `.env.example` doesn't match actual requirements

**Solution**:
1. Compare `.env` and `.env.example`
2. Update `.env.example` with new variables
3. Document new variables in README
4. Notify team of changes

### Makefile Target Conflicts

**Problem**: `make e2e test` runs both `e2e` and `test` targets

**Solution**:
```makefile
e2e:
	@if [ "$(word 1,$(MAKECMDGOALS))" != "e2e" ]; then \
		exit 0; \
	fi
	# e2e logic

.PHONY: e2e
```

### Docker Service Dependency Errors

**Problem**: Service starts before dependencies ready

**Solution**:
```yaml
services:
  app:
    depends_on:
      db:
        condition: service_healthy

  db:
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 5
```

## Best Practices

### Environment Files

- [ ] Never commit `.env`
- [ ] Keep `.env.example` up to date
- [ ] Use example values (not real credentials)
- [ ] Document all variables
- [ ] Separate configs for dev/test/prod

### Docker Configuration

- [ ] Use specific image versions
- [ ] Document service dependencies
- [ ] Use named volumes for data
- [ ] Expose only necessary ports
- [ ] Set resource limits for production

### Build System

- [ ] Keep Makefile targets simple
- [ ] Add help target for documentation
- [ ] Test targets with `--dry-run`
- [ ] Prevent multi-target conflicts
- [ ] Use `.PHONY` for all non-file targets

### Documentation

- [ ] Keep README up to date
- [ ] Auto-generate where possible
- [ ] Version documentation with code
- [ ] Include examples and quick starts
- [ ] Document breaking changes

## Integration Points

- **Backend**: Uses `.env` for database, auth, payments
- **Frontend**: Uses Tailwind config for styling
- **E2E**: Uses `e2e/.env.e2e` for test database
- **Deployment**: Uses docker-compose for services
- **CI/CD**: Uses Makefile targets for automation

## Quick Reference

### Common Commands

```bash
# Environment
cp .env.example .env
vi .env

# Docker
make compose up
make compose logs

# Build
make fix fmt
make fix imports
make build local

# Database
make db migrate up
make db reset

# Templates
make generate watch

# CSS
make css watch

# Tests
make test
make test coverage

# Documentation
make docs
```

### Validation Commands

```bash
# Check environment sync
make check env

# Validate docker config
docker compose -f compose.dev.yml config

# Test Makefile
make help

# Check translations
make check tr
```
