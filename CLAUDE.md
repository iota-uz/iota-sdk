## Rules

-   DO NOT COMMENT EXECESSIVELY. Instead, write clear and concise code that is self-explanatory.
-   DO NOT USE `sed` for file manipulation
-   Use `pkg/htmx` for all UI interactions
-   Use existing components from `components/` package before creating new ones
-   After changes to `.css` or `.templ` files: `make css`
-   NEVER read `*_templ.go` files, they contain little useful information since they are generated by `templ generate` command from `.templ` files
-   Do not indent code manually.
-   Error handling: use `pkg/serrors` for standard error types
-   When writing a mapper function, always use utilities from `pkg/mapping` to ensure consistency
-   PREFER `mcp__bloom__search_code` for semantic search over manual file searching when you don't know exact file names or when exploring the codebase to understand functionality

## DevHub MCP Tools

DevHub is a development environment orchestrator that manages all development services (database, server, CSS builds, etc.) configured in `devhub.yml`. When DevHub is running, use these MCP tools to monitor and control the development environment:

### Available MCP Tools:
1. **list_services** - Check status of all development services
   - Shows postgres, server, templ, css, tunnel, etc. configured in devhub.yml
   - Use this to verify all required services are running before starting work

2. **get_logs** - Retrieve logs from a specific service
   - Args: `service` (required), `lines` (optional, default: 50), `offset` (optional, default: 0)
   - Examples: 
     - `get_logs("server")` for latest Air hot-reload logs
     - `get_logs("postgres", lines=100)` for last 100 DB logs
     - `get_logs("server", lines=50, offset=100)` to see older logs
   - Essential for debugging build errors, database issues, or server crashes

3. **service_control** - Start, stop, or restart development services
   - Args: `service` (required), `action` (required: "start", "stop", "restart")
   - Examples: 
     - `service_control("server", "restart")` if Air gets stuck or after fixing a panic
     - `service_control("postgres", "stop")` to free up resources
     - `service_control("templ", "restart")` if templ watcher stops working

4. **health_check** - Get detailed health status of a service
   - Args: `service` (required)
   - Returns status, health, uptime, CPU/memory usage
   - Use to check if postgres is ready, server is healthy, etc.

5. **search_logs** - Search for patterns in service logs
   - Args: `service` (required), `pattern` (required), `context_lines` (optional, default: 2), `max_results` (optional, default: 50)
   - Examples:
     - `search_logs("server", "panic")` to find panics in server logs
     - `search_logs("templ", "error")` to find template compilation errors
     - `search_logs("server", "404", context_lines=5)` to debug missing routes with more context
   - Case-insensitive search with context lines before/after matches

### Common DevHub Workflows:
- **Templ compilation errors**: Use `get_logs("templ")` to see syntax errors in .templ files
- **Server runtime errors/panics**: Use `search_logs("server", "panic")` to quickly find panic stack traces
- **Server not responding**: Use `health_check("server")` to check if it crashed, then `get_logs("server")` for the error
- **Database connection issues**: Use `health_check("postgres")` to verify it's ready
- **Before running tests**: Use `list_services` to ensure postgres and server are healthy
- **After fixing a panic**: Use `service_control("server", "restart")` to restart the server
- **Debugging template issues**: Check `get_logs("templ")` for compilation errors, then `get_logs("server")` for runtime template errors
- **Finding specific errors**: Use `search_logs("server", "error", context_lines=5)` to find all errors with context
- **Debugging 404s**: Use `search_logs("server", "404")` to find missing route errors
- **Reviewing older logs**: Use `get_logs("server", lines=100, offset=200)` to see logs from earlier in the session

## Build/Lint/Test Commands
- Format code and remove unused imports: `make fmt`
- Apply migrations: `make migrate up`
- After changes to .templ files: `templ generate`
- After changes to Go code: `go vet ./...` 
- Do NOT run `go build`, as it does the same thing as `go vet`
- Run all tests: `make test` or `go test -v ./...`
- Run single test: `go test -v ./path/to/package -run TestName`
- Run specific subtest: `go test -v ./path/to/package -run TestName/SubtestName`
- Linting translation files: `make check-tr`
- Linting code: `make lint`

## Code Style Guidelines
- Use Go v1.23.2 and follow standard Go idioms
- Naming: use camelCase for variables, PascalCase for exported functions/types
- Testing: table-driven tests with descriptive names (TestFunctionName_Scenario), use the `require` and `assert` packages from `github.com/stretchr/testify`
- Type safety: use strong typing and avoid `interface{}/any` where possible
- Follow existing patterns for database operations with `jmoiron/sqlx`
- For UI components, follow the existing templ/htmx patterns

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
│   ├── entities/{entity}/
│   └── value_objects/          # Immutable domain concepts
├── infrastructure/             # External concerns
│   └── persistence/
│       ├── models/models.go    # Database models
│       ├── {entity}_repository.go # Repository implementations
│       ├── {module}_mappers.go # Domain-to-DB/DB-to-Domain mapping
│       ├── schema/{module}-schema.sql # SQL schema
│       └── setup_test.go
├── services/                   # Business logic orchestration
│   ├── {entity}_service.go
│   ├── {entity}_service_test.go
│   └── setup_test.go
├── presentation/
│   ├── controllers/
│   │   ├── {entity}_controller.go
│   │   ├── {entity}_controller_test.go
│   │   ├── dtos/{entity}_dto.go
│   │   └── setup_test.go
│   ├── templates/
│   │   ├── pages/{entity}/
│   │   │   ├── list.templ
│   │   │   ├── edit.templ
│   │   │   └── new.templ
│   │   └── components/         # Reusable UI components
│   ├── viewmodels/             # Presentation models
│   ├── mappers/mappers.go      # Domain-to-presentation mapping
│   └── locales/
│       ├── en.json
│       ├── ru.json
│       └── uz.json
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
- Run `templ generate` after creating/modifying `.templ` files

### 6. Localization
- Add translations to all locale files in `modules/{module}/presentation/locales/`
- Include NavigationLinks, Meta (titles), List, and Single sections

### 7. Registration
- Add navigation item to `modules/{module}/links.go`
- Register service and controller in `modules/{module}/module.go`:
  - Add service to `app.RegisterServices()` call
  - Add controller to `app.RegisterControllers()` call  
  - Add quick links to `app.QuickLinks().Add()` call

