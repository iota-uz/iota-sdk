# IOTA SDK - Claude Code Orchestrator Configuration

Claude serves as a **pure orchestrator** with general project knowledge, translating business requirements into specific code paths and delegating all implementation work to specialized agents.

## QUICK DECISION TREE

**Task Classification → Agent Selection:**

**Single Read (1-3 files):** No agents needed
- Documentation lookup, code reading

**Small Fix (1-5 files):** `debugger` + 1 specialist
- Single controller bug, small template fix

**Medium Feature (6-15 files):** 3-4 agents parallel
- New form, API endpoint, page updates

**Large Feature (16-30 files):** 5-7 agents parallel
- New module, major refactoring

**Cross-Module (30+ files):** 7-10+ agents parallel
- Architecture changes, bulk renaming

**Agent Selection Matrix:**
- **Errors/Failures**: Always start with `debugger`
- **Go Code Changes**: Always end with `auditor`
- **Database Changes**: Always include `editor`
- **Template/Translation**: Always include `editor`
- **Production Changes**: `auditor` ALWAYS
- **Complex Research/Discovery**: Use `general-purpose` when uncertain about code location or architecture
- **Multi-Step Search Operations**: Use `general-purpose` when simple grep/glob won't suffice

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
- Format Go code and templates: `make fix fmt`
- Organize and format Go imports: `make fix imports`
- Template generation: `templ generate` (or `make generate watch` for watch mode)
- Apply migrations: `make db migrate up` / `make db migrate down`
- After changes to Go code: `go vet ./...`
- DO NOT run `go build`, as it does the same thing as `go vet`

### Testing Commands:
- Run all tests: `make test`
- Run tests with coverage: `make test coverage`
- Run tests in watch mode: `make test watch`
- Run tests with verbose output: `make test verbose`
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
- Format Go code and templates: `make fix fmt`
- Organize and format Go imports: `make fix imports`
- Lint code (check unused variables/functions): `make check lint`
- Check translation files: `make check tr`

### Other Commands:
- Generate dependency graph: `make graph`

### Documentation Commands:
- Serve docs locally: `make docs serve` (Jekyll server with live reload)
- Install docs dependencies: `make docs install` (Ruby/Jekyll deps)

## E2E Testing Commands
Playwright E2E tests use separate `iota_erp_e2e` database (vs `iota_erp` for dev). Config: `/e2e/.env.e2e`, `/e2e/playwright.config.ts`

**IMPORTANT**: For ANY E2E test work (writing, editing, debugging), ALWAYS use the `e2e-tester` agent.

### Commands:
- Setup/reset: `make e2e test|reset|seed|migrate|clean`
- Run tests interactively: `make e2e run` - Open Playwright UI mode for interactive test running
- Run tests in CI mode: `make e2e ci` - Run all tests headless (no UI, serial execution) - ideal for terminal/CI
- Run individual e2e test: `cd e2e && npx playwright test tests/module/specific-test.spec.ts` (for debugging/focused testing)
- Debug mode: `cd e2e && npx playwright test --debug` (Playwright Inspector)
- Generate traces: `cd e2e && npx playwright test --trace on` (for debugging)
- View traces: `cd e2e && npx playwright show-trace trace.zip`

### Structure:
Tests in `/e2e/tests/{module}/`, fixtures in `/e2e/fixtures/`, page objects in `/e2e/pages/`

### When to Use e2e-tester Agent:
- Writing new Playwright test files (.spec.ts)
- Editing existing E2E tests
- Debugging failing Playwright tests
- Creating test fixtures (auth, test-data)
- Building page objects for UI interactions
- Adding test coverage for new features
- Improving test reliability and reducing flakiness
- Testing HTMX interactions and realtime SSE updates

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

**Superadmin Module** (Path: `modules/superadmin/`)
- Dashboard: `/` → `presentation/controllers/dashboard_controller.go` | `services/analytics_service.go` | `infrastructure/persistence/analytics_query_repository.go` | `presentation/templates/pages/dashboard/index.templ`
- Tenants: `/superadmin/tenants` → `presentation/controllers/tenants_controller.go` | `services/tenant_service.go` | `infrastructure/persistence/analytics_query_repository.go` | `presentation/templates/pages/tenants/index.templ`

**Security**: All superadmin routes MUST use `RequireSuperAdmin()` middleware to restrict access to superadmin users only.

### Core Rules
- **Use `// TODO` comments** for unimplemented parts or future enhancements

### Extensibility Patterns for Child Projects

#### PageContextProvider Interface

The `PageContextProvider` interface (`pkg/types/pagecontext.go`) enables child projects to extend page context behavior without modifying SDK code. Child projects can implement custom PageContext types with additional fields and override methods.

**Why Use PageContextProvider:**
- Add custom fields (tenant branding, feature flags, analytics)
- Override translation logic (custom locales, tenant-specific strings)
- Extend logging/metrics capabilities
- Maintain full backward compatibility with existing SDK code

**Implementation Pattern:**

```go
// In your child project's types package
import "github.com/iota-uz/iota-sdk/pkg/types"

// Step 1: Define your custom context struct
type TenantBrandingData struct {
    PrimaryColor   string
    CompanyLogo    string
    CompanyName    string
}

type CustomPageContext struct {
    // Embed the SDK's PageContextProvider interface
    base types.PageContextProvider

    // Add custom fields
    TenantBranding TenantBrandingData
    FeatureFlags   map[string]bool
    Analytics      AnalyticsConfig
}

// Step 2: Create a constructor
func NewCustomPageContext(
    sdkCtx types.PageContextProvider,
    branding TenantBrandingData,
) *CustomPageContext {
    return &CustomPageContext{
        base:           sdkCtx,
        TenantBranding: branding,
        FeatureFlags:   make(map[string]bool),
    }
}

// Step 3: Override methods as needed
// Override T() to provide tenant-specific translations
func (c *CustomPageContext) T(key string, args ...map[string]interface{}) string {
    // Check for tenant-specific override
    if override := c.lookupTenantTranslation(key); override != "" {
        return override
    }
    // Fall back to SDK default
    return c.base.T(key, args...)
}

// Override TSafe() similarly
func (c *CustomPageContext) TSafe(key string, args ...map[string]interface{}) string {
    if override := c.lookupTenantTranslation(key); override != "" {
        return override
    }
    return c.base.TSafe(key, args...)
}

// Pass through other methods to the base implementation
func (c *CustomPageContext) Namespace(prefix string) types.PageContextProvider {
    newBase := c.base.Namespace(prefix)
    return &CustomPageContext{
        base:           newBase,
        TenantBranding: c.TenantBranding,
        FeatureFlags:   c.FeatureFlags,
    }
}

func (c *CustomPageContext) ToJSLocale() string {
    return c.base.ToJSLocale()
}

func (c *CustomPageContext) GetLocale() language.Tag {
    return c.base.GetLocale()
}

func (c *CustomPageContext) GetURL() *url.URL {
    return c.base.GetURL()
}

func (c *CustomPageContext) GetLocalizer() *i18n.Localizer {
    return c.base.GetLocalizer()
}

// Add custom methods
func (c *CustomPageContext) TenantColor() string {
    return c.TenantBranding.PrimaryColor
}

func (c *CustomPageContext) HasFeature(flag string) bool {
    enabled, exists := c.FeatureFlags[flag]
    return exists && enabled
}

// Step 4: In your middleware, replace SDK context with custom context
func WithCustomPageContext(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Get the SDK-provided base context
        baseCtx := composables.UsePageCtx(r.Context())

        // Fetch tenant branding (from DB or cache)
        branding := fetchTenantBranding(r.Context())

        // Create custom context wrapping the base
        customCtx := NewCustomPageContext(baseCtx, branding)

        // Store custom context in request context
        newCtx := composables.WithPageCtx(r.Context(), customCtx)
        next.ServeHTTP(w, r.WithContext(newCtx))
    })
}
```

**Key Points:**
- Child projects embed `PageContextProvider` interface, not `PageContext` struct
- SDK code continues using `PageContextProvider` interface (not `*PageContext`)
- `WithPageCtx()` accepts `PageContextProvider` for full extensibility
- Override only the methods you need; delegate others to base implementation
- Add `Namespace()` support if you override translation methods
- No changes to SDK code required - fully backward compatible

### Build/Test Commands
```bash
# Code Quality & Testing
go vet ./...                          # After Go changes (prefer over go build)
make test                             # Run all tests (default, use 10-minute timeout for full suite)
make test failures                    # Show only failing tests (JSON format, use 10-minute timeout)
make test coverage                    # Run tests with simple coverage report (Go default, use 10-minute timeout)
make test detailed-coverage           # Run tests with detailed coverage analysis & insights (use 10-minute timeout)
make test verbose                     # Run tests with verbose output (use 10-minute timeout)
go test -v ./path/to/package -run TestSpecificName  # Run individual test by name (for debugging/focused testing)
make check tr                         # Validate translations

# Linting & Code Quality
make lint                             # Run golangci-lint (checks for unused variables/functions)
# Note: Run `make lint` after Go code changes to check for unused code before committing

# CSS Compilation
make css                              # Compile CSS with minification (default)
make css watch                        # Compile and watch for changes
make css dev                          # Compile without minification (debugging)
make css clean                        # Clean CSS build artifacts

# Template Generation
templ generate                          # Generate templ templates (default)
make generate watch                   # Watch and regenerate templ templates on changes

# Migrations & Setup
make migrate up                       # Apply migrations
make migrate down                     # Rollback migrations
```

## AGENT ORCHESTRATION WORKFLOWS

**Claude's Default Operating Mode: Balanced Orchestration**

Use agents for complex, multi-file, or unclear-scope tasks. Handle simple, well-defined work (1-3 files, <50 lines, clear requirements) directly with Read/Edit/Bash tools. Don't use agents for typo fixes, single-line changes, reading files, or running commands.

### Multi-Agent Workflow Matrix

**Workflow Types:**

**Feature Development:** Use `editor` agent
- Optional: `auditor` (always after editor)
- When: New features, enhancements, major functionality

**Bug Resolution:** `debugger` → `editor` + `auditor`
- When: Bug fixes, error resolution, system failures

**Performance Issues:** `debugger` + `editor` + `auditor`
- When: Slow queries, high latency, resource usage

**UI/Template Changes:** Use `editor` agent
- When: UI updates, forms, frontend functionality

**Database Changes:** `editor` + `auditor`
- Note: editor handles all layers
- When: Schema changes, migrations, query optimization

**Cross-Module Work:** Multiple `editor` + `auditor`
- When: Architecture changes, large refactoring

**Config Management:** Use `editor` agent
- Handles: CLAUDE.md updates, env files, docs, agent defs

**E2E Test Work:** Use `e2e-tester` agent
- Optional: `debugger` (for debugging failing tests)
- When: Playwright tests, fixtures, page objects

**Research & Discovery:** Use `general-purpose` agent
- Feeds findings to implementation agents
- When: Codebase exploration, pattern analysis, architecture understanding, complex searches

**Agent Launch Rules:**
- **Always parallel**: Launch required agents simultaneously in single message
- **Always sequential**: `debugger` first for bugs, `auditor` last for Go changes
- **Scale by scope**: 1-3 agents (small), 4-6 agents (medium), 7-10+ agents (large)
- **Agent threshold**: Use agents only when task requires 4+ files, 50+ lines, or investigation

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
2. Analysis: 15 errors in core, 20 in finance, 10 in warehouse
3. Launch 3 editor agents with specific scope:
   → editor (1): Fix 15 core module type errors
   → editor (2): Fix 20 finance module type errors
   → editor (3): Fix 10 warehouse module type errors
```

**Example: Template Updates Across Pages**
```
1. Run: find . -name "*.templ" (discovers 28 template files)
2. Analysis: 12 in core, 10 in finance, 6 in warehouse
3. Launch balanced editor agents:
   → editor (1): Update 12 core templates
   → editor (2): Update 10 finance templates
   → editor (3): Update 6 warehouse templates
```

**Example: Test Coverage Gaps**
```
1. Run: find . -name "*_test.go" + coverage analysis
2. Analysis: Missing tests in 8 services, 12 controllers, 5 repositories
3. Launch editor agents with balanced scope:
   → editor (1): Services (8 files) - implement missing tests
   → editor (2): Controllers (12 files) - implement missing tests
   → editor (3): Repositories (5 files) - implement missing tests
```


##### 4. Assessment Tools for Orchestrators

**Type Errors:**
- Analysis: `go vet ./...`, `go build ./...`
- Distribution: Split by module/package

**Template Work:**
- Analysis: `find . -name "*.templ"`
- Distribution: Split by functional area

**Translation Missing:**
- Analysis: `make check tr`, `grep -r "missing"`
- Distribution: Split by language files

**Test Coverage:**
- Analysis: `go test -cover ./...`, find tests
- Distribution: Split by layer/domain

**Performance Issues:**
- Analysis: `go test -bench ./...`, profiling
- Distribution: Split by service/component

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
5. **Final Review Phase**: `auditor` reviews all changes

#### Scaling Triggers
- **Large scope discovered**: Add more agents of same type with balanced loads
- **Cross-layer changes**: Add specialized agents for each layer
- **Multiple modules**: Launch multiple instances with module-specific scope
- **Complex integrations**: Include coordination agents with clear responsibilities

### Practical Workflow Examples

**Example 1: Bug Investigation & Fix**
```
1. Investigation: debugger (investigate error, identify root cause)
2. Implementation: editor (fix the bug across affected files)
3. Quality review: auditor (review changes, ensure code quality)

Syntax: debugger && editor && auditor
```

**Example 2: New Feature Development**
```
1. Planning (optional): general-purpose for architecture research if needed
2. Implementation: editor (implement domain, service, repository, controller, templates)
3. Quality review: auditor (ensure production-ready code)

Syntax: editor && auditor
Or with research: general-purpose && editor && auditor
```

**Example 3: Database Schema Changes**
```
1. Migration: editor (create migration, update domain entities)
2. Data layer: editor (update repositories for new schema)
3. Quality review: auditor (verify migration safety, code quality)

Syntax: editor (migrations + domain) && editor (repositories) && auditor
Or single editor: editor && auditor
```

**Example 4: UI/Template Updates**
```
1. Templates: editor (update .templ files, controllers, ViewModels)
2. Translations: editor (update en.toml, ru.toml, uz.toml)
3. Quality review: auditor (verify HTMX patterns, security)

Syntax: editor && auditor
```

**Example 5: Performance Optimization**
```
1. Investigation: debugger (identify bottlenecks, analyze queries)
2. Optimization: editor (optimize queries, add indexes, improve algorithms)
3. Quality review: auditor (ensure no regressions)

Syntax: debugger && editor && auditor
```

**Example 6: E2E Test Development**
```
1. Test creation: e2e-tester (write Playwright tests, fixtures, page objects)
2. Debug failures (if needed): debugger && e2e-tester

Syntax: e2e-tester
Or with debugging: debugger && e2e-tester
```

**Example 7: Multi-Module Changes (Parallel Execution)**
```
1. Parallel implementation:
   - editor (finance module changes)
   - editor (warehouse module changes)
   - editor (CRM module changes)
2. Sequential review: auditor (review all changes)

Syntax: (editor & editor & editor) && auditor
```

**Example 8: Complex Research & Implementation**
```
1. Research: general-purpose (explore codebase, find patterns, analyze architecture)
2. Implementation: editor (apply findings, implement solution)
3. Quality review: auditor (ensure consistency with codebase)

Syntax: general-purpose && editor && auditor
```

### When NOT to Use Agents

**Handle directly (no agents):**
- Simple file reading, typo fixes, single-line changes
- Running commands (go vet, make test, git status)
- 1-3 file edits with clear requirements (<50 lines total)
- Adding comments, updating docs, config tweaks

**Use agents for:**
- 4+ files or 50+ lines of changes
- Cross-layer changes (controller + service + repository + template)
- Unknown scope requiring investigation
- Feature development, schema changes, refactoring
- Production changes requiring quality review

### Anti-Patterns to Avoid

**❌ Agent Misuse:**
- Using `editor` for E2E test work (use `e2e-tester` instead)
- Using `e2e-tester` for Go code changes (use `editor` instead)
- Splitting related changes across multiple uncoordinated `editor` agents

**❌ Workflow Mistakes:**
- Launching agents sequentially when parallel is possible
- Skipping `debugger` for unknown issues
- Missing `auditor` after Go code changes
- Using direct tools instead of `editor` for complex multi-file changes

### Business Context Translation

**Business Request → Multi-Agent Orchestration Examples:**

**"Fix dashboard bug"**
→ `debugger` && (`editor` & `auditor`)

**"Add new payment form"**
→ `editor` && `auditor`

**"Optimize accounting performance"**
→ `debugger` && `editor` && `auditor`

**"Update finance module"**
→ (Multiple `editor`) && `auditor`

**"Update CLAUDE.md with new agent"**
→ `editor`

**"Fix environment configuration issues"**
→ `editor`

**"Add new documentation section"**
→ `editor`

**"Write E2E tests for user registration"**
→ `e2e-tester`

**"Debug failing Playwright test"**
→ `debugger` && `e2e-tester`

**"Add page objects for vehicles module"**
→ `e2e-tester`

**"How is authentication implemented?"**
→ `general-purpose` (research) → findings inform implementation agents

**"Find all places where payment is processed"**
→ `general-purpose` (multi-step search) → guide `editor` to relevant files

**"Understand the multi-tenant architecture"**
→ `general-purpose` (architecture exploration) → inform feature design

**"Where should I add new RBAC check?"**
→ `general-purpose` (pattern analysis) → guide `editor` to correct location

**"What's the standard error handling pattern?"**
→ `general-purpose` (codebase patterns) → inform implementation approach

**Agent Execution Syntax:**
- `&` = Parallel execution (agents run simultaneously)
- `&&` = Sequential execution (wait for completion before next step)
- `Multiple agent` = Launch several instances of same agent type with divided scope

## Special instructions for plan mode
When in plan mode, your plan should always include the following:
- A clear decision on whether to use single-agent or multi-agent workflow, with justification.
- If multi-agent, specify the exact agents to be used, their roles, and how they will collaborate.