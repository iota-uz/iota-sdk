---
allowed-tools: |
  Bash(cd e2e:*), Bash(make e2e:*), Bash(npm run:*), Bash(pnpm:*), Bash(npx playwright:*),
  Bash(go run cmd/command/main.go e2e:*),
  Read, Edit, MultiEdit, Grep, Glob,
  Bash(docker:*), Bash(lsof:*), Bash(ps:*), Bash(kill:*),
  Task
description: Systematically identify and fix broken E2E tests using Playwright debugging workflow
---

# Fix E2E Tests

This command systematically identifies and fixes broken E2E tests using Playwright infrastructure and environment-aware debugging.

## Strategy

### 1. Environment & Service Validation Phase
- **ALWAYS verify E2E database state** with `make e2e reset` before debugging
- **Check port conflicts**: `lsof -i :3201` (E2E server) and `lsof -i :5438` (E2E DB)
- **Validate environment**: Verify `/e2e/.env.e2e` configuration matches test expectations
- **Server status**: Ensure E2E dev server is running on correct port with proper environment
- **Database isolation**: Confirm `iota_erp_e2e` database exists and is separate from `iota_erp`

### 2. Discovery Phase
- Run `make e2e test` to identify failing tests systematically
- Use `cd e2e && npx playwright test tests/module/specific-test.spec.ts` for focused debugging
- Use `cd e2e && npx playwright test --ui` for interactive debugging
- Categorize E2E failures by type:
  - **Database state issues** (stale data, missing seeds, isolation failures)
  - **Timing/race conditions** (Alpine.js initialization, HTMX requests, async operations)
  - **Form submission bugs** (attachment handling, hidden inputs, FormData serialization)
  - **Navigation issues** (routing, redirects, authentication state)
  - **Element interaction failures** (selectors, visibility, Alpine.js component state)
  - **Network/server errors** (API endpoints, server crashes, database connections)

### 3. Analysis Phase
For each failing E2E test:
- **Check Playwright screenshots** in `e2e/test-results/` for visual debugging clues
- **Examine console errors** from browser dev tools captured by Playwright
- **Validate test data setup**: Verify database reset and seeding fixtures work
- **Analyze timing issues**: Look for missing waits or inadequate auto-waiting
- **Review fixtures**: Check `/e2e/fixtures/` for fixture failures
- **Database state**: Verify test isolation and data persistence between test runs

### 4. Systematic Fix Phase (Environment-Aware)

#### Database & Environment Issues:
- **Reset E2E environment**: `make e2e reset` to ensure clean state
- **Verify migrations**: `make e2e migrate` to ensure schema is current
- **Check seeding**: `make e2e seed` to verify test data generation
- **Validate isolation**: Confirm E2E tests use `iota_erp_e2e` database, not `iota_erp`

#### Playwright Test Failures:
- **Use debugger agent first** for systematic error analysis and root cause identification
- **Fix timing issues**: Add proper waits, Playwright has auto-waiting but may need explicit waits for Alpine.js
- **Update selectors**: Fix element selectors broken by UI changes, prefer data-testid attributes
- **Handle async operations**: Ensure proper request interception and waiting for network requests
- **Form handling**: Fix attachment uploads, hidden inputs, and FormData submission issues

#### Server/Infrastructure Issues:
- **Port conflicts**: Kill conflicting processes on ports 3201/5438
- **Server startup**: Use `make e2e dev` to start E2E development server with hot reload
- **Environment variables**: Validate `.env.e2e` configuration matches test expectations
- **Database connectivity**: Check PostgreSQL connection to E2E database

### 5. Validation Phase
- Run `make e2e reset` to ensure clean starting state
- Run `make e2e test` to execute full E2E suite
- Use `cd e2e && npx playwright test tests/module/` for module-specific validation
- Verify no test pollution: Tests should pass in isolation and when run together
- Check screenshot artifacts: Ensure no new visual regressions captured in test-results/

## E2E-Specific Best Practices

### Database & State Management
- **Always start with `make e2e reset`** for clean database state
- **Use database fixtures**: Utilize fixtures in `beforeEach` for database setup
- **Verify isolation**: E2E tests should not affect main development database
- **Clean sessions**: Clear cookies/storage in afterEach to prevent authentication pollution

### Timing & Alpine.js Integration
- **Wait for Alpine initialization**: Use explicit waits after page navigation if needed
- **Handle async operations**: Properly intercept and wait for HTMX/Alpine.js requests
- **Auto-waiting**: Playwright auto-waits for elements, but may need explicit waits for dynamic content
- **Element visibility**: Use `waitFor()` for visibility checks before interactions

### Form & Component Testing
- **File uploads**: Use Playwright's file upload methods with fixtures
- **Form submission**: Intercept POST requests to verify FormData structure
- **Hidden inputs**: Validate form attribute association and value persistence
- **Alpine.js components**: Test component state, reactivity, and error handling

### Error Handling & Debugging
- **Leverage error handling**: Check browser console for errors via page.on('console')
- **Screenshot analysis**: Check generated screenshots in test-results/ folder
- **Console log review**: Analyze browser console errors captured by Playwright
- **Network debugging**: Use route interception for API call validation

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
- Missing test data → Ensure database fixtures complete successfully

### Timing/Race Conditions
- Alpine.js not initialized → Add explicit waits for Alpine.js initialization
- HTMX requests not completed → Use proper route interception with Playwright
- Elements not visible → Use `waitFor({ state: 'visible' })` before interaction

### Form Submission Bugs
- Attachments not submitted → Verify hidden input generation and form association
- FormData serialization issues → Check route interception and validation
- Missing CSRF tokens → Ensure proper session and form state

### Infrastructure Problems
- Port conflicts → Use `lsof -i :PORT` to identify and kill conflicting processes
- Server not running → Start with `make e2e dev` for development server
- Database connection failures → Check PostgreSQL service and E2E database existence