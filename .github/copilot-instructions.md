# IOTA SDK Multi-Tenant Business Management Platform

This document provides context and guidelines for GitHub Copilot when working on this codebase.

## Project Overview

Multi-tenant business management platform built on the IOTA SDK framework. Provides modular solutions for financial management, CRM, warehouse operations, project management, and HR functionality.

### Core Business Domains

- **Financial Management**: Payments, expenses, debts, transactions, counterparties, money accounts
- **Customer Relations**: Client management, communication, message templates
- **Warehouse Operations**: Inventory, products, orders, positions, units
- **Project Management**: Project tracking, stage management, task coordination
- **Human Resources**: Employee management, organizational structure
- **Billing & Subscriptions**: Payment processing, subscription management, Stripe integration

## Technology Stack

- **Backend**: Go 1.24.10, IOTA SDK framework, GraphQL
- **Database**: PostgreSQL 13+ (multi-tenant with organization_id)
- **Frontend**: HTMX + Alpine.js + Templ + Tailwind CSS
- **Auth**: Cookie-based sessions with RBAC
- **Payments**: Stripe subscriptions

## Architecture

### Design Principles

- **DDD (Domain-Driven Design)**: Strict layer separation between domain, service, repository, and presentation
- **Multi-tenant**: All data is isolated via `organization_id` - every query MUST include tenant filtering
- **HTMX-First**: Server-side rendering with HTMX + Alpine.js + Templ
- **Dual Servers**:
  - **Main** (`cmd/server/main.go`): Multi-tenant platform, all modules
  - **SuperAdmin** (`cmd/superadmin/main.go`): Admin-only server (port 4000)

### Code Organization

```
modules/{module}/
  domain/
    aggregates/         # Domain entities
    */repository.go     # Repository interfaces
  presentation/
    controllers/{page}_controller.go
    viewmodels/{entity}_viewmodel.go
    templates/pages/{page}/
    locales/{lang}.toml
  services/{domain}_service.go
  infrastructure/persistence/{entity}_repository.go
```

### Environment Branches

- **Production**: `main` branch
- **Staging**: `staging` branch

## Verification Commands

Run these commands to verify code quality:

```bash
# Code Quality
go vet ./...                          # After Go changes (prefer over go build)
make check lint                       # Run golangci-lint
make check deadcode                   # Detect unreachable/unused code

# Testing (Note: Full test suite is SLOW - use sparingly)
go test -v ./path/to/package -run TestName  # Single test by name (fast)
make test                             # Run all tests
make test coverage                    # Run tests with coverage report

# Template & CSS
make generate                         # Generate templ templates
make css                              # Compile CSS with minification

# Migrations
make db migrate up                    # Apply migrations
make db migrate down                  # Rollback migrations

# Translations
make check tr                         # Validate translations

# E2E Tests
make e2e ci                           # Run Playwright E2E tests (headless)
make e2e run                          # Run Playwright E2E tests (UI mode)
```

## Critical Code Rules

### Multi-Tenant Isolation (CRITICAL)

Every database query MUST include tenant filtering:

```go
// CORRECT - Always include organization_id
SELECT * FROM payments WHERE organization_id = $1 AND id = $2

// WRONG - Security vulnerability
SELECT * FROM payments WHERE id = $1
```

Use `composables.UseTenantID(ctx)` to get the current tenant ID.

### Error Handling

Always wrap errors with operation context:

```go
const op = serrors.Op("service.PaymentService.GetByID")
if err != nil {
    return nil, serrors.E(op, err)
}
```

### Dependency Injection

Use `di.H` pattern for all service dependencies:

```go
func NewController(h di.H) *Controller {
    return &Controller{
        paymentService: di.Get[PaymentService](h),
    }
}
```

### Repository Pattern

- Interfaces in domain layer (`modules/*/domain/*/repository.go`)
- Implementations in infrastructure (`modules/*/infrastructure/persistence/*`)
- Use `composables.UseTx(ctx)` for transactions
- Use parameterized queries ($1, $2) - never string concatenation

### Controllers

- Auth middleware via `middleware.Authorize()`
- Form parsing: `composables.UseForm[DTO](r)`
- Query params: `composables.UseQuery[DTO](r)`
- HTMX helpers: Use `pkg/htmx` package functions

### Templates (Templ)

- Use `templ.URL()` for dynamic URLs
- Never use `@templ.Raw()` with user input (XSS risk)
- Include CSRF tokens in all forms
- Use CamelCase for form field names (e.g., `FirstName`, not `first_name`)

### Translations

- Update all 3 locale files: `en.toml`, `ru.toml`, `uz.toml`
- Avoid TOML reserved keywords (OTHER, ID, DESCRIPTION)
- Validate with `make check tr`

### Migrations

See `.claude/guides/backend/migrations.md` for comprehensive patterns. Key rules:
- Include both Up and Down sections (Down must reverse Up)
- Use StatementBegin/StatementEnd for PL/pgSQL functions/triggers
- Always include `organization_id` with `ON DELETE CASCADE` for multi-tenant tables
- Test reversibility: Up→Down→Up cycle

## Documentation

For detailed implementation patterns, see these guide files:
- `.claude/guides/backend/domain-service.md` - DDD, services, entities
- `.claude/guides/backend/repository.md` - Repository patterns, query optimization
- `.claude/guides/backend/testing.md` - ITF testing framework
- `.claude/guides/backend/presentation.md` - Controllers, ViewModels, templates
- `.claude/guides/backend/migrations.md` - SQL migration patterns
- `.claude/guides/backend/i18n.md` - Translation management
- `.claude/guides/backend/routing.md` - Module registration, DI
