# AGENTS.md - IOTA SDK Guide

## Overview
DO NOT COMMENT EXECESSIVELY. Instead, write clear and concise code that is self-explanatory.

## Design Philosophy
iota-sdk is a general purpose ERP building engine/solution. When designing anything inside iota-sdk:
- Make it extensible, generalizable, and customizable
- Prefer interfaces over concrete structs at boundaries
- Apply dependency inversion and inject interfaces
- Keep domain and services decoupled from infrastructure details

## Module Architecture

Each module follows a strict **Domain-Driven Design (DDD)** pattern with clear layer separation:

```
modules/{module}/
├── domain/                     # Pure business logic
│   ├── aggregates/{entity}/    # Complex business entities
│   │   ├── {entity}.go         # Entity interface
│   │   ├── {entity}_impl.go    # Entity implementation
│   │   ├── {entity}_events.go  # Domain events
│   │   └── {entity}_repository.go # Repository interface
│   ├── entities/{entity}/      # Simpler domain entities
│   └── value_objects/          # Immutable domain concepts
├── infrastructure/             # External concerns
│   └── persistence/
│       ├── models/models.go    # Database models
│       ├── {entity}_repository.go # Repository implementations
│       ├── {module}_mappers.go # Domain-to-DB mapping
│       ├── schema/{module}-schema.sql # SQL schema
│       └── setup_test.go       # Test utilities
├── services/                   # Business logic orchestration
│   ├── {entity}_service.go     # Service implementation
│   ├── {entity}_service_test.go # Service tests
│   └── setup_test.go           # Test setup
├── presentation/               # UI and API layer
│   ├── controllers/
│   │   ├── {entity}_controller.go # HTTP handlers
│   │   ├── {entity}_controller_test.go # Controller tests
│   │   ├── dtos/{entity}_dto.go # Data transfer objects
│   │   └── setup_test.go       # Test utilities
│   ├── templates/
│   │   ├── pages/{entity}/     # Entity-specific pages
│   │   │   ├── list.templ      # List view
│   │   │   ├── edit.templ      # Edit form
│   │   │   └── new.templ       # Create form
│   │   └── components/         # Reusable UI components
│   ├── viewmodels/             # Presentation models
│   ├── mappers/mappers.go      # Domain-to-presentation mapping
│   └── locales/                # Internationalization
│       ├── en.json             # English translations
│       ├── ru.json             # Russian translations
│       └── uz.json             # Uzbek translations
├── module.go                   # Module registration
├── links.go                    # Navigation items
└── permissions/constants.go    # RBAC permissions
```

## Creating New Entities (Repositories, Services, Controllers)

### 1. Domain Layer
- Create domain entity in `modules/{module}/domain/aggregates/{entity_name}/`
- Define repository interface with CRUD operations and domain events
- Follow existing patterns (see `payment_category` or `expense_category`)

### 2. Infrastructure Layer
- Add database model to `modules/{module}/infrastructure/persistence/models/models.go`
- Create repository implementation in `modules/{module}/infrastructure/persistence/{entity_name}_repository.go`
- Add domain-to-database mappers in `modules/{module}/infrastructure/persistence/{module}_mappers.go`

### 3. Service Layer
- Create service in `modules/{module}/services/{entity_name}_service.go`
- Include event publishing and business logic methods
- Follow constructor pattern: `NewEntityService(repo, eventPublisher)`

### 4. Presentation Layer
- Create DTOs in `modules/{module}/presentation/controllers/dtos/{entity_name}_dto.go`
- Create controller in `modules/{module}/presentation/controllers/{entity_name}_controller.go`
- Create viewmodel in `modules/{module}/presentation/viewmodels/{entity_name}_viewmodel.go`
- Add mapper in `modules/{module}/presentation/mappers/mappers.go`

### 5. Templates (if needed)
- Create templ files in `modules/{module}/presentation/templates/pages/{entity_name}/`
- Common templates: `list.templ`, `edit.templ`, `new.templ`
- Run `templ generate` after creating/modifying .templ files

### 6. Localization
- Add translations to all locale files in `modules/{module}/presentation/locales/`
- Include NavigationLinks, Meta (titles), List, and Single sections

### 7. Registration
- Add navigation item to `modules/{module}/links.go`
- Register service and controller in `modules/{module}/module.go`:
  - Add service to `app.RegisterServices()` call
  - Add controller to `app.RegisterControllers()` call  
  - Add quick links to `app.QuickLinks().Add()` call

### 8. Verification
- Run `go vet ./...` to verify compilation
- Run `templ generate && make css` if templates were modified

## Technology Stack

- **Backend**: Go 1.23.2, IOTA SDK framework, GraphQL
- **Database**: PostgreSQL 13+ (multi-tenant with organization_id)
- **Frontend**: HTMX + Alpine.js + Templ + Tailwind CSS
- **Auth**: Cookie-based sessions with RBAC

## Core Rules

- **Multi-tenant isolation**: Always include `tenant_id` in WHERE clauses
- **Error handling**: Use `pkg/serrors` - `serrors.E(op, err)` pattern
- **HTMX**: Check `htmx.IsHxRequest(r)`, use `htmx.SetTrigger(w, "event", payload)`
- **Never read `*_templ.go` files** - they're generated

## Tool Use
- DO NOT USE `sed` for file manipulation
- Prefer `mcp__bloom__search_code(repo: "iota-uz/iota-sdk")` for semantic search when you do not know exact file names or need to explore the codebase

## Build/Lint/Test Commands

### Code Quality Commands:
- Format Go code and templates: `make fix fmt`
- Organize and format Go imports: `make fix imports`
- Lint code (check unused variables/functions): `make check lint`
- Check translation files: `make check tr`

### Other Commands:
- After changes to css or .templ files: `templ generate && make css`
- After changes to Go code: `go vet ./...`
- **Never run `go build`** - use `go vet` instead
- Run all tests: `make test` or `go test -v ./...`
- Run single test: `go test -v ./path/to/package -run TestName`
- Run specific subtest: `go test -v ./path/to/package -run TestName/SubtestName`
- Apply migrations: `make db migrate up`

## E2E Testing

```bash
make e2e run      # Interactive UI mode
make e2e ci       # Headless CI mode
cd e2e && npx playwright test tests/module/specific.spec.ts  # Single test
```

## Code Style Guidelines
- Use `go fmt` for formatting. Do not indent code manually.
- Use Go v1.23.2 and follow standard Go idioms
- File organization: group related functionality in modules/ or pkg/ directories
- Naming: use camelCase for variables, PascalCase for exported functions/types
- Testing: table-driven tests with descriptive names (TestFunctionName_Scenario), use the `require` and `assert` packages from `github.com/stretchr/testify`
- Error handling: use `pkg/serrors` with pattern `serrors.E(op, err)` for standard error types
- When writing a mapper function, always use utilities from `pkg/mapping` to ensure consistency
- Type safety: use strong typing and avoid interface{} where possible
- Follow existing patterns for database operations with jmoiron/sqlx
- For UI components, follow the existing templ/htmx patterns

## UI Implementation Guidelines

- Use `pkg/htmx` for all UI interactions
- Use existing components from `components/` package before creating new ones

### HTMX Best Practices
- Use `htmx.IsHxRequest(r)` to check if a request is from HTMX
- Use `htmx.SetTrigger(w, "eventName", payload)` for setting HTMX response triggers

