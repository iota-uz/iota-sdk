# SHY ELD - Claude Code Orchestrator Configuration

Claude serves as a **pure orchestrator** with general project knowledge, translating business requirements into specific code paths and delegating all implementation work to specialized agents.

## QUICK DECISION TREE

**Task Classification → Agent Selection (Use this first)**

| Task Scope | File Count | Agent Combination | Example Triggers |
|------------|------------|------------------|------------------|
| **Single Read** | 1-3 files | **No agents needed** | Documentation lookup, code reading |
| **Small Fix** | 1-5 files | `debugger` + 1 specialist | Single controller bug, small template fix |
| **Medium Feature** | 6-15 files | 3-4 agents parallel | New form, API endpoint, page updates |
| **Large Feature** | 16-30 files | 5-7 agents parallel | New module, major refactoring |
| **Cross-Module** | 30+ files | 7-10+ agents parallel | Architecture changes, bulk renaming |
| **Pattern Scanning** | Variable | `speed-editor` + specialists | Code analysis, hardcoded values, duplicate patterns |

**Agent Selection Matrix:**
- **Errors/Failures**: Always start with `debugger`
- **Go Code Changes**: Always end with `refactoring-expert`
- **Database Changes**: Always include `database-expert`
- **Template/Translation**: Always include `ui-editor`
- **Bulk Operations**: Use `speed-editor` for preprocessing
- **Pattern Discovery**: Use `speed-editor` for code scanning/analysis
- **Production Changes**: `refactoring-expert` ALWAYS

**File-Type Mandates:**
- **.templ or .json files**: `ui-editor` ONLY
- **Database work**: `database-expert` ONLY
- **Deployments**: `railway-ops` ONLY
- **migrations/*.sql**: `database-expert` ONLY
- **Configuration files**: `config-manager` ONLY
- **CLAUDE.md updates**: `config-manager` ONLY
- **Documentation maintenance**: `config-manager` ONLY

## PROJECT OVERVIEW

### Business Overview
IOTA SDK is a multi-tenant business management platform providing modular solutions for financial management, CRM, warehouse operations, project management, and HR functionality.

### Core Business Domains
- **Financial Management**: Payments, expenses, debts, transactions, counterparties, accounts
- **Customer Relations**: Client management, communication, message templates
- **Warehouse Operations**: Inventory, products, orders, positions, units
- **Project Management**: Project tracking, stage management, task coordination
- **Human Resources**: Employee management, organizational structure
- **Billing & Subscriptions**: Payment processing, subscription management

### Technology Stack
- **Backend**: Go 1.23.2, IOTA SDK framework, GraphQL
- **Database**: PostgreSQL 13+ (multi-tenant with organization_id)
- **Frontend**: HTMX + Alpine.js + Templ + Tailwind CSS
- **Auth**: Cookie-based sessions with RBAC
- **Payments**: Stripe subscriptions

## Build/Lint/Test Commands
- Format code and remove unused imports: `make check fmt`
- Template generation: `make generate` (or `make generate watch` for watch mode)
- Apply migrations: `make db migrate up` / `make db migrate down`
- After changes to Go code: `go vet ./...`
- DO NOT run `go build`, as it does the same thing as `go vet`

### Testing Commands:
- Run all tests: `make test`
- Run tests with coverage: `make test coverage`
- Run tests in watch mode: `make test watch`
- Run tests with verbose output: `make test verbose`
- Run specific package tests: `make test package ./path/to/package`
- Run individual test by name: `go test -v ./path/to/package -run TestSpecificName` (for debugging/focused testing)
- Run tests in Docker: `make test docker`
- Generate coverage report: `make test report`
- Check coverage score: `make test score`

### CSS Commands:
- Compile CSS: `make css`
- Compile CSS in development mode: `make css dev`
- Watch CSS changes: `make css watch`
- Clean CSS artifacts: `make css clean`

### Docker Compose Commands:
- Start all services: `make compose up`
- Stop all services: `make compose down`
- Restart services: `make compose restart`
- View logs: `make compose logs`

### Build Commands:
- Build for local OS: `make build local`
- Build for Linux (production): `make build linux`
- Build Docker base image: `make build docker-base`
- Build Docker production image: `make build docker-prod`

### Code Quality Commands:
- Format code and remove unused imports: `make check fmt`
- Lint code (check unused variables/functions): `make check lint`
- Check translation files: `make check tr`

### Other Commands:
- Generate dependency graph: `make graph`
- Generate documentation: `make docs`

## E2E Testing Commands
Cypress E2E tests use separate `iota_erp_e2e` database (vs `iota_erp` for dev). Config: `/e2e/.env.e2e`, `/e2e/cypress.config.js`

### Two-Step E2E Testing Workflow:
1. **Start E2E development server** (with Air hot reload): `make e2e dev`
2. **Run tests in separate terminal**: `make e2e test` or `cd e2e && npm run test:headed`

### Commands:
- Setup/reset: `make e2e test|reset|seed|migrate|clean`
- Development server: `make e2e dev` - Start server with Air hot reload connected to e2e database on port 3201
- Run tests: `make e2e test|run` - Execute Cypress tests against running e2e server
- Run individual e2e test: `cd e2e && npm run cy:run --spec "cypress/e2e/module/specific-test.cy.js"` (for debugging/focused testing)

### Workflow Description:
The e2e testing process is now separated into two distinct phases:
- **Development Phase**: `make e2e dev` starts the server with Air for automatic code reloading during development
- **Testing Phase**: `make e2e test` runs the Cypress test suite against the running e2e development server
- This separation allows for better development experience with hot reload while maintaining clean test execution

### Structure:
Tests in `/e2e/cypress/e2e/{module}/`, commands in `/e2e/cypress/support/commands.js`, fixtures in `/e2e/cypress/fixtures/`

### Environment Branches
- **Production**: `main` branch
- **Staging**: `staging` branch

## CODE ORGANIZATION

### File Path Patterns
```
modules/{module}/
├── presentation/controllers/{page}_controller.go           # HTTP handlers
├── services/{domain}_service.go                            # Business logic
├── infrastructure/persistence/{entity}_repository.go       # Data access
├── presentation/templates/pages/{page}/                    # UI templates
└── presentation/locales/{lang}.json                        # Translations
```

## BUSINESS → CODE MAPPING

### Core Module Mapping

**Core Module** (Path: `modules/core/`)
- Dashboard: `/` → `presentation/controllers/dashboard_controller.go` | `services/dashboard_service.go` | `presentation/templates/pages/dashboard/index.templ`
- Users: `/users` → `presentation/controllers/user_controller.go` | `services/user_service.go` | `infrastructure/persistence/user_repository.go` | `presentation/templates/pages/users/index.templ`
- Roles: `/roles` → `presentation/controllers/roles_controller.go` | `services/roles_service.go` | `infrastructure/persistence/roles_repository.go` | `presentation/templates/pages/roles/index.templ`
- Groups: `/groups` → `presentation/controllers/group_controller.go` | `services/group_service.go` | `infrastructure/persistence/group_repository.go` | `presentation/templates/pages/groups/index.templ`
- Settings: `/settings` → `presentation/controllers/settings_controller.go` | `services/settings_service.go` | `infrastructure/persistence/settings_repository.go` | `presentation/templates/pages/settings/index.templ`
- Account: `/account` → `presentation/controllers/account_controller.go` | `services/account_service.go` | `infrastructure/persistence/account_repository.go` | `presentation/templates/pages/account/index.templ`

**Finance Module** (Path: `modules/finance/`)
- Financial Overview: `/finance` → `presentation/controllers/financial_overview_controller.go` | `services/financial_service.go` | `infrastructure/persistence/financial_repository.go` | `presentation/templates/pages/financial_overview/index.templ`
- Payments: `/finance/payments` → `presentation/controllers/payment_controller.go` | `services/payment_service.go` | `infrastructure/persistence/payment_repository.go` | `presentation/templates/pages/payments/index.templ`
- Expenses: `/finance/expenses` → `presentation/controllers/expense_controller.go` | `services/expense_service.go` | `infrastructure/persistence/expense_repository.go` | `presentation/templates/pages/expenses/index.templ`
- Debts: `/finance/debts` → `presentation/controllers/debt_controller.go` | `services/debt_service.go` | `infrastructure/persistence/debt_repository.go` | `presentation/templates/pages/debts/index.templ`
- Transactions: `/finance/transactions` → `presentation/controllers/transaction_controller.go` | `services/transaction_service.go` | `infrastructure/persistence/transaction_repository.go` | `presentation/templates/pages/transactions/index.templ`
- Counterparties: `/finance/counterparties` → `presentation/controllers/counterparties_controller.go` | `services/counterparties_service.go` | `infrastructure/persistence/counterparties_repository.go` | `presentation/templates/pages/counterparties/index.templ`
- Money Accounts: `/finance/accounts` → `presentation/controllers/money_account_controller.go` | `services/money_account_service.go` | `infrastructure/persistence/money_account_repository.go` | `presentation/templates/pages/moneyaccounts/index.templ`
- Reports: `/finance/reports` → `presentation/controllers/financial_report_controller.go` | `services/report_service.go` | `infrastructure/persistence/report_repository.go` | `presentation/templates/pages/reports/index.templ`

**CRM Module** (Path: `modules/crm/`)
- Clients: `/crm/clients` → `presentation/controllers/client_controller.go` | `services/client_service.go` | `infrastructure/persistence/client_repository.go` | `presentation/templates/pages/clients/index.templ`
- Chats: `/crm/chats` → `presentation/controllers/chat_controller.go` | `services/chat_service.go` | `infrastructure/persistence/chat_repository.go` | `presentation/templates/pages/chats/index.templ`
- Message Templates: `/crm/message-templates` → `presentation/controllers/message_template_controller.go` | `services/message_template_service.go` | `infrastructure/persistence/message_template_repository.go` | `presentation/templates/pages/message-templates/index.templ`

**Warehouse Module** (Path: `modules/warehouse/`)
- Inventory: `/warehouse/inventory` → `presentation/controllers/inventory_controller.go` | `services/inventory_service.go` | `infrastructure/persistence/inventory_repository.go` | `presentation/templates/pages/inventory/index.templ`
- Products: `/warehouse/products` → `presentation/controllers/product_controller.go` | `services/product_service.go` | `infrastructure/persistence/product_repository.go` | `presentation/templates/pages/products/index.templ`
- Orders: `/warehouse/orders` → `presentation/controllers/order_controller.go` | `services/order_service.go` | `infrastructure/persistence/order_repository.go` | `presentation/templates/pages/orders/index.templ`
- Positions: `/warehouse/positions` → `presentation/controllers/position_controller.go` | `services/position_service.go` | `infrastructure/persistence/position_repository.go` | `presentation/templates/pages/positions/index.templ`
- Units: `/warehouse/units` → `presentation/controllers/unit_controller.go` | `services/unit_service.go` | `infrastructure/persistence/unit_repository.go` | `presentation/templates/pages/units/index.templ`

**Projects Module** (Path: `modules/projects/`)
- Projects: `/projects` → `presentation/controllers/project_controller.go` | `services/project_service.go` | `infrastructure/persistence/project_repository.go` | `presentation/templates/pages/projects/index.templ`
- Project Stages: `/projects/stages` → `presentation/controllers/project_stage_controller.go` | `services/project_stage_service.go` | `infrastructure/persistence/project_stage_repository.go` | `presentation/templates/pages/project_stages/index.templ`

**HRM Module** (Path: `modules/hrm/`)
- Employees: `/hrm/employees` → `presentation/controllers/employee_controller.go` | `services/employee_service.go` | `infrastructure/persistence/employee_repository.go` | `presentation/templates/pages/employees/index.templ`

**Billing Module** (Path: `modules/billing/`)
- Billing Dashboard: `/billing` → `presentation/controllers/billing_controller.go` | `services/billing_service.go` | `infrastructure/persistence/billing_repository.go` | `presentation/templates/pages/billing/index.templ`

**Website Module** (Path: `modules/website/`)
- Public Pages: `/` → `presentation/controllers/website_controller.go` | `services/website_service.go` | `infrastructure/persistence/website_repository.go` | `presentation/templates/pages/website/index.templ`

### Core Rules
- **Use `// TODO` comments** for unimplemented parts or future enhancements

### Build/Test Commands
```bash
# Code Quality & Testing
go vet ./...                          # After Go changes (prefer over go build)
make test                             # Run all tests (default, use 10-minute timeout for full suite)
make test failures                    # Show only failing tests (JSON format, use 10-minute timeout)
make test coverage                    # Run tests with simple coverage report (Go default, use 10-minute timeout)
make test detailed-coverage           # Run tests with detailed coverage analysis & insights (use 10-minute timeout)
make test verbose                     # Run tests with verbose output (use 10-minute timeout)
make test package ./path/to/package   # Test specific package
go test -v ./path/to/package -run TestSpecificName  # Run individual test by name (for debugging/focused testing)
make check-tr                         # Validate translations

# Linting & Code Quality
make lint                             # Run golangci-lint (checks for unused variables/functions)
# Note: Run `make lint` after Go code changes to check for unused code before committing

# CSS Compilation
make css                              # Compile CSS with minification (default)
make css watch                        # Compile and watch for changes
make css dev                          # Compile without minification (debugging)
make css clean                        # Clean CSS build artifacts

# Template Generation
make generate                         # Generate templ templates (default)
make generate watch                   # Watch and regenerate templ templates on changes

# Migrations & Setup
make migrate up                       # Apply migrations
make migrate down                     # Rollback migrations
```

## AGENT ORCHESTRATION WORKFLOWS

**Claude's Default Operating Mode: Multi-Agent Parallel Execution**

Multi-agent workflows are the **standard approach** for all non-trivial development tasks. Single agents are the exception, not the rule.

### Multi-Agent Workflow Matrix

| Workflow Type           | Required Agents                                        | Optional Agents                                                                 | When to Use                                     |
|-------------------------|--------------------------------------------------------|---------------------------------------------------------------------------------|-------------------------------------------------|
| **Feature Development** | `go-editor` + `ui-editor` + `test-editor`              | `database-expert` (data changes), `refactoring-expert` (always after go-editor) | New features, enhancements, major functionality |
| **Bug Resolution**      | `debugger` → `go-editor` + `refactoring-expert`        | `test-editor` (regression tests), `ui-editor` (UI bugs)                         | Bug fixes, error resolution, system failures    |
| **Performance Issues**  | `debugger` + `go-editor` + `refactoring-expert`        | `database-expert` (query optimization)                                          | Slow queries, high latency, resource usage      |
| **UI/Template Changes** | `ui-editor`                                            | `go-editor` (controller changes), `test-editor` (coverage)                      | UI updates, forms, frontend functionality       |
| **Database Changes**    | `database-expert` + `go-editor` + `refactoring-expert` | `test-editor` (data validation tests)                                           | Schema changes, migrations, query optimization  |
| **Cross-Module Work**   | Multiple `go-editor` + `refactoring-expert`            | `database-expert`, `ui-editor`, `test-editor`                                   | Architecture changes, large refactoring         |
| **Bulk Operations**     | `speed-editor` → specialist agents                     | `refactoring-expert` (review)                                                   | Mass renaming, pattern standardization          |
| **Pattern Discovery**   | `speed-editor` → analysis agents                       | `go-editor`, `refactoring-expert`                                               | Code scanning, hardcoded values, anti-patterns  |
| **Config Management**   | `config-manager`                                       | None (handles all config concerns)                                              | CLAUDE.md updates, env files, docs, agent defs  |

**Agent Launch Rules:**
- **Always parallel**: Launch required agents simultaneously in single message
- **Always sequential**: `debugger` first for bugs, `refactoring-expert` last for Go changes
- **Scale by scope**: 1-3 agents (small), 4-6 agents (medium), 7-10+ agents (large)

### Workflow Execution Patterns

#### Orchestrator Analysis & Work Distribution (Critical Phase)

**BEFORE launching any agents, Claude must:**

##### 1. Scope Analysis Phase
```bash
# Always analyze full scope first using available tools:
go vet ./...                    # Identify all type errors, issues
grep -r "TODO" --include="*.go" # Find incomplete work
find . -name "*.templ" | wc -l  # Count template files
find . -name "*_test.go" | wc -l # Assess test coverage needs
```

##### 2. Work Distribution Strategy
- **Count total issues/files** before delegation
- **Divide work evenly** between multiple agents of same type
- **Assign specific scope** to each agent (files, modules, error ranges)

##### 3. Balanced Delegation Patterns

**Example: Type Errors Across Codebase**
```
1. Run: go vet ./... (discovers 45 type errors across 3 modules)
2. Analysis: 15 errors in logistics, 20 in finance, 10 in safety
3. Launch 3 go-editor agents with specific scope:
   → go-editor (1): Fix 15 logistics module type errors  
   → go-editor (2): Fix 20 finance module type errors
   → go-editor (3): Fix 10 safety module type errors
```

**Example: Template Updates Across Pages**
```
1. Run: find . -name "*.templ" (discovers 28 template files)
2. Analysis: 12 in logistics, 10 in finance, 6 in safety
3. Launch balanced ui-editor agents:
   → ui-editor (1): Update 12 logistics templates
   → ui-editor (2): Update 10 finance templates  
   → ui-editor (3): Update 6 safety templates
```

**Example: Test Coverage Gaps**
```
1. Run: find . -name "*_test.go" + coverage analysis
2. Analysis: Missing tests in 8 services, 12 controllers, 5 repositories
3. Launch test-editor agents with balanced scope:
   → test-editor (1): Services (8 files)
   → test-editor (2): Controllers (12 files)
   → test-editor (3): Repositories (5 files)
```

**Example: Bulk Renaming with Speed-Editor**
```
1. Run: grep -r "oldFunctionName" --include="*.go" (discovers 89 occurrences)
2. Analysis: 45 in logistics, 30 in finance, 14 in safety modules
3. Launch speed-editor for mechanical renaming:
   → speed-editor: Rename oldFunctionName → newFunctionName across all 89 files
4. Follow up with go-editor for any logic adjustments needed
```

**Example: Pattern Discovery with Speed-Editor**
```
1. Request: "Find all hardcoded string values that could be enum constants"
2. Launch speed-editor for pattern scanning:
   → speed-editor: Scan for hardcoded strings, identify enum candidates
3. Analysis: Found 34 hardcoded status strings, 12 repeated error messages
4. Follow up with go-editor to create enums and refactor usage
```

##### 4. Assessment Tools for Orchestrators

| Task Type | Analysis Commands | Distribution Strategy |
|-----------|------------------|----------------------|
| **Type Errors** | `go vet ./...`, `go build ./...` | Split by module/package |
| **Template Work** | `find . -name "*.templ"` | Split by functional area |
| **Translation Missing** | `make check-tr`, `grep -r "missing"` | Split by language files |
| **Test Coverage** | `go test -cover ./...`, find tests | Split by layer/domain |
| **Performance Issues** | `go test -bench ./...`, profiling | Split by service/component |
| **Bulk Operations** | `grep -r "pattern"`, `find . -name "*"` | Use speed-editor for mechanical work |
| **Pattern Discovery** | Complex grep patterns, code analysis | Use speed-editor for scanning, then specialist agents |

#### Parallel Agent Launch (After Analysis)
- **Always analyze scope FIRST** using assessment tools
- **Launch agents with specific, balanced scope** in single message
- **Scale UP agent usage** based on analysis results:
    - **1-5 files**: 1-2 agents maximum
    - **6-15 files**: 3-4 agents (optimal)
    - **16-30 files**: 5-7 agents (high efficiency)
    - **31+ files**: 8-10 agents (maximum capacity)
- **Performance threshold**: >10 agents degrades coordination efficiency

#### Agent Handoff Protocol
1. **Pre-Analysis Phase**: Claude assesses full scope using tools
2. **Balanced Distribution Phase**: Work divided evenly between agents
3. **Independent Work Phase**: Agents work in parallel on assigned scope
4. **Integration Points**: Outputs from one agent feed others as needed
5. **Final Review Phase**: `refactoring-expert` reviews all changes

#### Scaling Triggers
- **Large scope discovered**: Add more agents of same type with balanced loads
- **Cross-layer changes**: Add specialized agents for each layer
- **Multiple modules**: Launch multiple instances with module-specific scope
- **Complex integrations**: Include coordination agents with clear responsibilities

### Agent Collaboration Matrix

| Primary Agent | Provides Input To | Receives Input From | Parallel Partners |
|---------------|-------------------|---------------------|-------------------|
| **debugger** | `go-editor`, `database-expert` | Error logs, user reports | None (investigation first) |
| **go-editor** | `test-editor`, `refactoring-expert` | `debugger`, `database-expert` | `ui-editor`, `database-expert`, `speed-editor` |
| **database-expert** | `go-editor`, `refactoring-expert` | Business requirements | `go-editor`, `ui-editor` |
| **ui-editor** | `go-editor`, `test-editor` | Controller changes | `go-editor`, `test-editor`, `speed-editor` |
| **test-editor** | `refactoring-expert` | `go-editor`, `ui-editor` | `go-editor`, `ui-editor` |
| **speed-editor** | Other agents (bulk work) | Task specifications | `go-editor`, `ui-editor` |
| **config-manager** | Agent coordination, documentation | Project requirements | None (configuration coordination) |
| **refactoring-expert** | Final output | All other agents | None (final review) |

### Single Agent Exceptions

**Use single agents ONLY for:**
- **Simple read-only queries**: Documentation lookups, code reading
- **Emergency hotfixes**: Time-critical production issues (but follow up with multi-agent review)
- **Single-file documentation updates**: README changes, comment additions
- **Configuration tweaks**: Small settings adjustments

**Never use single agents for:**
- Cross-layer changes (controller + template + service)
- Data schema modifications
- Feature development
- Bug fixes with unknown scope
- Performance optimization

### Anti-Patterns to Avoid

**❌ Agent Misuse:**
- Using `ui-editor` for Go logic changes
- Using `go-editor` for database schema modifications
- Using `speed-editor` for complex business logic
- Using `test-editor` without existing implementation

**❌ Workflow Mistakes:**
- Launching agents sequentially when parallel is possible
- Single agent for multi-layer changes
- Skipping `debugger` for unknown issues
- Missing `refactoring-expert` after Go changes

### Speed-Editor Agent

**Purpose**: High-speed bulk mechanical operations and pattern discovery (renaming, find-replace, code scanning, cleanup tasks)

**Use for**: Bulk renaming, simple find-replace, adding/removing trivial elements, mechanical pattern changes, large-scale refactoring prep, pattern scanning, hardcoded value discovery, code smell identification

**Don't use for**: Complex logic, architecture decisions, new features, debugging, database work, template logic

**Characteristics**: Maximum speed, parallel partner for bulk operations, preprocessing role before intelligent agents, high-speed pattern scanner for code analysis

#### Business Context Translation
**Business Request → Multi-Agent Orchestration**

| Business Context                            | Standard Multi-Agent Launch                                                                       |
|---------------------------------------------|---------------------------------------------------------------------------------------------------|
| "Fix dashboard bug"                         | `debugger` && (`go-editor` & `ui-editor` & `test-editor` & `refactoring-expert`)                  |
| "Add new driver form"                       | (`go-editor` & `database-expert` & `ui-editor`) && `test-editor` && `refactoring-expert`          |
| "Optimize accounting performance"           | `debugger` && (`database-expert` & `go-editor`) && `refactoring-expert`                           |
| "Update finance module"                     | (Multiple `go-editor` & `database-expert` & `ui-editor`) && `test-editor` && `refactoring-expert` |
| "Rename functions across codebase"          | `speed-editor` && `go-editor` && `refactoring-expert`                                             |
| "Standardize import patterns"               | `speed-editor` && `refactoring-expert`                                                            |
| "Find hardcoded strings for enum constants" | `speed-editor` && `go-editor` && `refactoring-expert`                                             |
| "Scan for duplicate code patterns"          | `speed-editor` && `go-editor` && `refactoring-expert`                                             |
| "Update CLAUDE.md with new agent"           | `config-manager`                                                                                  |
| "Fix environment configuration issues"      | `config-manager`                                                                                  |
| "Add new documentation section"             | `config-manager`                                                                                  |
| "Deploy to staging"                         | `railway-ops`                                                                                     |

**Agent Execution Syntax:**
- `&` = Parallel execution (agents run simultaneously)
- `&&` = Sequential execution (wait for completion before next step)
- `Multiple agent` = Launch several instances of same agent type with divided scope

## Special instructions for plan mode
When in plan mode, your plan should always include the following:
- A clear decision on whether to use single-agent or multi-agent workflow, with justification.
- If multi-agent, specify the exact agents to be used, their roles, and how they will collaborate.