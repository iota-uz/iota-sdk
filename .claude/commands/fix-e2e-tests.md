---
allowed-tools: |
  Bash(cd e2e:*), Bash(make e2e:*), Bash(npm run:*), Bash(pnpm:*),
  Bash(go run cmd/command/main.go e2e:*),
  Read, Edit, MultiEdit, Grep, Glob,
  Bash(docker:*), Bash(lsof:*), Bash(ps:*), Bash(kill:*),
  Task(subagent_type:debugger), Task(subagent_type:ui-editor)
description: Systematically identify and fix broken E2E tests using Cypress-specific debugging workflow
---

# Fix E2E Tests

This command systematically identifies and fixes broken E2E tests using Cypress infrastructure and environment-aware debugging.

## Strategy

### 1. Environment & Service Validation Phase
- **ALWAYS verify E2E database state** with `make e2e reset` before debugging
- **Check port conflicts**: `lsof -i :3201` (E2E server) and `lsof -i :5438` (E2E DB)
- **Validate environment**: Verify `/e2e/.env.e2e` configuration matches test expectations
- **Server status**: Ensure E2E dev server is running on correct port with proper environment
- **Database isolation**: Confirm `iota_erp_e2e` database exists and is separate from `iota_erp`

### 2. Discovery Phase
- Run `make e2e test` to identify failing tests systematically
- Use `cd e2e && npm run cy:run --spec "cypress/e2e/module/specific-test.cy.js"` for focused debugging
- Categorize E2E failures by type:
  - **Database state issues** (stale data, missing seeds, isolation failures)
  - **Timing/race conditions** (Alpine.js initialization, HTMX requests, async operations)
  - **Form submission bugs** (attachment handling, hidden inputs, FormData serialization)
  - **Navigation issues** (routing, redirects, authentication state)
  - **Element interaction failures** (selectors, visibility, Alpine.js component state)
  - **Network/server errors** (API endpoints, server crashes, database connections)

### 3. Analysis Phase
For each failing E2E test:
- **Check Cypress screenshots** in `e2e/cypress/screenshots/` for visual debugging clues
- **Examine console errors** from browser dev tools captured by Cypress
- **Validate test data setup**: Verify `cy.task("resetDatabase")` and `cy.task("seedDatabase")` work
- **Analyze timing issues**: Look for missing `cy.waitForAlpine()` or inadequate waits
- **Review custom commands**: Check `/e2e/cypress/support/commands.js` for command failures
- **Database state**: Verify test isolation and data persistence between test runs

### 4. Systematic Fix Phase (Environment-Aware)

#### Database & Environment Issues:
- **Reset E2E environment**: `make e2e reset` to ensure clean state
- **Verify migrations**: `make e2e migrate` to ensure schema is current
- **Check seeding**: `make e2e seed` to verify test data generation
- **Validate isolation**: Confirm E2E tests use `iota_erp_e2e` database, not `iota_erp`

#### Cypress Test Failures:
- **Use debugger agent first** for systematic error analysis and root cause identification
- **Fix timing issues**: Add proper waits, use `cy.waitForAlpine()` for Alpine.js components
- **Update selectors**: Fix element selectors broken by UI changes
- **Handle async operations**: Ensure proper intercepting and waiting for network requests
- **Form handling**: Fix attachment uploads, hidden inputs, and FormData submission issues

#### Server/Infrastructure Issues:
- **Port conflicts**: Kill conflicting processes on ports 3201/5438
- **Server startup**: Use `make e2e dev` to start E2E development server with hot reload
- **Environment variables**: Validate `.env.e2e` configuration matches Cypress expectations
- **Database connectivity**: Check PostgreSQL connection to E2E database

### 5. Validation Phase
- Run `make e2e reset` to ensure clean starting state
- Run `make e2e test` to execute full E2E suite
- Use `cd e2e && npm run cy:run --spec "cypress/e2e/module/*.cy.js"` for module-specific validation
- Verify no test pollution: Tests should pass in isolation and when run together
- Check screenshot artifacts: Ensure no new visual regressions captured

## E2E-Specific Best Practices

### Database & State Management
- **Always start with `make e2e reset`** for clean database state
- **Use database tasks**: `cy.task("resetDatabase")` and `cy.task("seedDatabase")` in beforeEach
- **Verify isolation**: E2E tests should not affect main development database
- **Clean sessions**: Use `cy.logout()` in afterEach to prevent authentication pollution

### Timing & Alpine.js Integration
- **Wait for Alpine initialization**: Use `cy.waitForAlpine()` after page navigation
- **Handle async operations**: Properly intercept and wait for HTMX/Alpine.js requests
- **Avoid hard waits**: Use `cy.should()` assertions instead of `cy.wait(timeout)`
- **Element visibility**: Ensure elements are visible before interaction attempts

### Form & Component Testing
- **File uploads**: Use custom `uploadFileAndWaitForAttachment()` command consistently
- **Form submission**: Intercept POST requests to verify FormData structure
- **Hidden inputs**: Validate form attribute association and value persistence
- **Alpine.js components**: Test component state, reactivity, and error handling

### Error Handling & Debugging
- **Leverage custom error handling**: Utilize configured uncaught exception handling
- **Screenshot analysis**: Check generated screenshots in failure scenarios
- **Console log review**: Analyze browser console errors captured by Cypress
- **Network debugging**: Use request interception for API call validation

## Agent Selection for E2E Work

### Use `debugger` agent for:
- Unknown E2E test failures requiring systematic investigation
- Server startup issues, database connectivity problems
- Complex timing/race condition analysis
- Network/API endpoint debugging

### Use `ui-editor` agent for:
- Template/component changes breaking E2E selectors
- HTMX/Alpine.js integration issues affecting E2E tests
- Form submission logic fixes (attachment handling, hidden inputs)
- UI component state management problems

### Multi-agent workflows:
- **Complex form bugs**: `debugger` + `ui-editor` (parallel investigation and fix)
- **Database-related failures**: `debugger` + `database-expert` (if schema changes needed)
- **Performance issues**: `debugger` + `go-editor` (server-side optimization)

## Common E2E Failure Patterns

### Database Isolation Issues
- Test pollution between runs → Use `make e2e reset` between debugging sessions
- Wrong database connection → Verify `.env.e2e` configuration
- Missing test data → Ensure `cy.task("seedDatabase")` completes successfully

### Timing/Race Conditions
- Alpine.js not initialized → Add `cy.waitForAlpine()` calls
- HTMX requests not completed → Use proper request interception
- Elements not visible → Use `cy.should('be.visible')` before interaction

### Form Submission Bugs
- Attachments not submitted → Verify hidden input generation and form association
- FormData serialization issues → Check request interception and validation
- Missing CSRF tokens → Ensure proper session and form state

### Infrastructure Problems
- Port conflicts → Use `lsof -i :PORT` to identify and kill conflicting processes
- Server not running → Start with `make e2e dev` for development server
- Database connection failures → Check PostgreSQL service and E2E database existence