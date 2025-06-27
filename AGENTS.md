# CLAUDE.md - IOTA SDK Guide

## Overview
DO NOT COMMENT EXECESSIVELY. Instead, write clear and concise code that is self-explanatory.

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

## Tool use
- DO NOT USE `sed` for file manipulation

## Build/Lint/Test Commands
- After changes to css or .templ files: `templ generate && make css`
- After changes to Go code: `go vet ./...` (Do NOT run `go build` as it is not needed)
- Run all tests: `make test` or `go test -v ./...` 
- Run single test: `go test -v ./path/to/package -run TestName`
- Run specific subtest: `go test -v ./path/to/package -run TestName/SubtestName`
- Linting translation files: `make check-tr`
- Apply migrations: `make migrate up`

## Code Style Guidelines
- Use `go fmt` for formatting. Do not indent code manually.
- Use Go v1.23.2 and follow standard Go idioms
- File organization: group related functionality in modules/ or pkg/ directories
- Naming: use camelCase for variables, PascalCase for exported functions/types
- Testing: table-driven tests with descriptive names (TestFunctionName_Scenario), use the `require` and `assert` packages from `github.com/stretchr/testify`
- Error handling: use pkg/serrors for standard error types
- Type safety: use strong typing and avoid interface{} where possible
- Follow existing patterns for database operations with jmoiron/sqlx
- For UI components, follow the existing templ/htmx patterns
- NEVER read *_templ.go files, they contain no useful information since they are generated by templ generate (make generate) command from .templ files

## UI Implementation Guidelines

### HTMX Best Practices
- Use `htmx.IsHxRequest(r)` to check if a request is from HTMX
- Use `htmx.SetTrigger(w, "eventName", payload)` for setting HTMX response triggers

