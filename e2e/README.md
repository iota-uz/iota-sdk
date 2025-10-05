# IOTA SDK E2E Testing with Playwright

This directory contains end-to-end tests for the IOTA SDK platform using Playwright, a modern, reliable testing framework.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Running Tests](#running-tests)
- [Directory Structure](#directory-structure)
- [Environment Configuration](#environment-configuration)
- [Database Setup](#database-setup)
- [Writing Tests](#writing-tests)
- [Fixtures Documentation](#fixtures-documentation)
- [Troubleshooting](#troubleshooting)
- [CI/CD Integration](#cicd-integration)

## Overview

The E2E testing suite validates the complete IOTA SDK platform functionality including:
- User authentication and registration
- Employee management
- Financial operations
- Warehouse management
- CRM functionality
- Multi-tenant isolation

**Key Features:**
- TypeScript support for type safety
- Isolated test database (`iota_erp_e2e`)
- Reusable fixtures for common operations
- Smart environment detection (local vs CI)
- Comprehensive error handling
- Fast parallel execution

## Prerequisites

Before running E2E tests, ensure you have:

1. **Node.js** (v18 or higher)
   ```bash
   node --version
   ```

2. **pnpm** (recommended) or npm
   ```bash
   pnpm --version
   ```

3. **PostgreSQL** database running
   - Local: Port 5438 (via Docker)
   - CI: Port 5432
   ```bash
   docker ps  # Verify database is running
   ```

4. **Go server** running on port 3201
   ```bash
   # From project root
   go run cmd/app/main.go
   ```

## Installation

Navigate to the e2e directory and install dependencies:

```bash
cd e2e
pnpm install

# Or with npm
npm install
```

This will install:
- `@playwright/test` - Core testing framework
- `typescript` - TypeScript support
- `@types/node` - Node.js type definitions
- `@types/pg` - PostgreSQL type definitions
- `dotenv` - Environment variable management
- `pg` - PostgreSQL client

## Quick Start

1. **Setup environment file:**
   ```bash
   # .env.e2e already exists with correct configuration
   cat .env.e2e
   ```

2. **Start the test database:**
   ```bash
   # From project root
   make compose up
   ```

3. **Start the Go server:**
   ```bash
   # From project root, in another terminal
   go run cmd/app/main.go
   ```

4. **Run all tests:**
   ```bash
   pnpm test
   ```

5. **Run tests in UI mode (interactive):**
   ```bash
   pnpm run test:ui
   ```

## Running Tests

### Basic Commands

```bash
# Run all tests
pnpm test

# Run tests in interactive UI mode
pnpm run test:ui

# Run tests in headed mode (see browser)
pnpm run test:headed

# Run tests in debug mode
pnpm run test:debug
```

### Module-Specific Tests

```bash
# Run user tests only
pnpm run test:users

# Run employee tests only
pnpm run test:employees
```

### Advanced Usage

```bash
# Run specific test file
npx playwright test tests/users/register.spec.ts

# Run tests matching a pattern
npx playwright test --grep "registration"

# Run tests with specific tag
npx playwright test --grep @smoke

# Run tests in specific browser
npx playwright test --project=chromium

# Generate HTML report
npx playwright show-report
```

### Makefile Integration

From the project root, you can use:

```bash
# Run E2E tests
make e2e test

# Reset E2E database
make e2e reset

# Seed E2E database
make e2e seed

# Run migrations on E2E database
make e2e migrate

# Clean E2E test artifacts
make e2e clean
```

## Directory Structure

```
e2e/
├── .env.e2e                    # Environment configuration
├── package.json                # Dependencies and scripts
├── pnpm-lock.yaml             # Lock file
├── playwright.config.ts        # Playwright configuration
├── tsconfig.json               # TypeScript configuration
├── README.md                   # This file
│
├── fixtures/                   # Shared test utilities
│   ├── index.ts               # Centralized exports
│   ├── auth.ts                # Authentication helpers
│   ├── database.ts            # Database operations
│   ├── error-handling.ts      # Error suppression
│   ├── file-upload.ts         # File upload utilities
│   └── test-data.ts           # Test data builders
│
└── tests/                      # Test files (organized by module)
    ├── README.md              # Test documentation
    ├── users/
    │   ├── register.spec.ts
    │   └── realtime.spec.ts
    └── employees/
        └── employees.spec.ts
```

## Environment Configuration

### .env.e2e File

The `.env.e2e` file contains environment-specific configuration:

```bash
# Database Configuration (E2E-specific database)
DB_HOST=localhost
DB_PORT=5438                    # Local: 5438, CI: 5432
DB_NAME=iota_erp_e2e           # Separate from dev database
DB_USER=postgres
DB_PASSWORD=postgres

# Server Configuration
BASE_URL=http://localhost:3201  # E2E server URL
PORT=3201
SERVER_HOST=localhost

# Environment
NODE_ENV=test
LOG_LEVEL=debug
```

### Smart Environment Detection

The Playwright configuration automatically detects the environment:
- **Local Development**: Uses port 5438 for database
- **CI Environment**: Uses port 5432 for database

This is configured in `playwright.config.ts`:
```typescript
const isCI = process.env.CI === 'true' || process.env.GITHUB_ACTIONS === 'true';
const defaultPort = isCI ? 5432 : 5438;
```

## Database Setup

### Separate Test Database

E2E tests use a dedicated database `iota_erp_e2e` to avoid conflicts with development data.

### Database Operations

```bash
# Reset database (truncate all tables)
make e2e reset

# Seed database with test data
make e2e seed

# Run migrations
make e2e migrate up
make e2e migrate down
```

### In Tests

```typescript
import { resetDB, seedDB } from '../fixtures';

test.beforeEach(async () => {
  await resetDB();   // Clean slate for each test
  await seedDB();    // Seed with minimal data
});
```

## Writing Tests

### Basic Test Structure

```typescript
import { test, expect } from '@playwright/test';
import { login, resetDB, seedDB, setupErrorHandling } from '../fixtures';

test.describe('User Registration', () => {
  test.beforeEach(async ({ page }) => {
    // Setup error handling (recommended for all tests)
    await setupErrorHandling(page);

    // Reset and seed database
    await resetDB();
    await seedDB();
  });

  test('should register new user successfully', async ({ page }) => {
    // Navigate to registration page
    await page.goto('/register');

    // Fill form
    await page.fill('input[name="email"]', 'newuser@example.com');
    await page.fill('input[name="password"]', 'SecurePass123!');
    await page.fill('input[name="name"]', 'John Doe');

    // Submit
    await page.click('button[type="submit"]');

    // Assert success
    await expect(page).toHaveURL('/dashboard');
    await expect(page.locator('.welcome-message')).toContainText('John Doe');
  });

  test('should show validation error for invalid email', async ({ page }) => {
    await page.goto('/register');

    await page.fill('input[name="email"]', 'invalid-email');
    await page.click('button[type="submit"]');

    await expect(page.locator('.error-message')).toContainText('valid email');
  });
});
```

### Test Best Practices

1. **Descriptive Names**: Use clear, descriptive test names
2. **Isolation**: Each test should be independent
3. **Cleanup**: Always reset database before tests
4. **Error Handling**: Setup error handling in `beforeEach`
5. **Stable Locators**: Use data attributes or roles instead of classes
6. **Assertions**: Use appropriate matchers (`toBeVisible`, `toHaveText`, etc.)

## Fixtures Documentation

Fixtures are reusable utility functions that encapsulate common operations.

### Database Fixtures

```typescript
import { resetDB, seedDB, getEnvironmentInfo } from '../fixtures';

// Reset database (truncate all tables)
await resetDB();

// Seed database with test data
await seedDB();

// Get environment info for debugging
const info = getEnvironmentInfo();
console.log(info);
```

**File**: `fixtures/database.ts`

### Authentication Fixtures

```typescript
import { login, logout, waitForAlpine } from '../fixtures';

// Login user
await login(page, 'user@example.com', 'password123');

// Logout user
await logout(page);

// Wait for Alpine.js to initialize
await waitForAlpine(page);
```

**File**: `fixtures/auth.ts`

**Functions:**
- `login(page, email, password)` - Authenticates user and verifies redirect
- `logout(page)` - Logs out current user
- `waitForAlpine(page)` - Waits for Alpine.js initialization

### File Upload Fixtures

```typescript
import { uploadFileAndWaitForAttachment } from '../fixtures';

// Upload file and wait for processing
await uploadFileAndWaitForAttachment(
  page,
  'File content here',
  'document.txt',
  'text/plain'
);
```

**File**: `fixtures/file-upload.ts`

**Function:**
- `uploadFileAndWaitForAttachment(page, content, filename, mimeType)` - Creates file buffer, uploads, and waits for attachment processing

### Test Data Fixtures

```typescript
import {
  resetTestDatabase,
  populateTestData,
  seedScenario,
  getAvailableScenarios,
  checkTestEndpointsHealth,
  TestDataBuilders
} from '../fixtures';

// Reset via API
await resetTestDatabase(request, { reseedMinimal: true });

// Populate specific test data
await populateTestData(request, {
  users: [{ email: 'test@example.com', role: 'admin' }],
  employees: [{ name: 'John Doe', position: 'Developer' }]
});

// Seed predefined scenario
await seedScenario(request, 'minimal');

// Check available scenarios
const scenarios = await getAvailableScenarios(request);

// Health check
const health = await checkTestEndpointsHealth(request);

// Build test data
const user = TestDataBuilders.createUser({
  email: 'custom@example.com',
  role: 'admin'
});
```

**File**: `fixtures/test-data.ts`

**Note**: These functions require the `request` context from Playwright test fixtures.

### Error Handling Fixtures

```typescript
import { setupErrorHandling, shouldIgnoreError } from '../fixtures';

// Setup error handling (call in beforeEach)
test.beforeEach(async ({ page }) => {
  await setupErrorHandling(page);
});

// Check if error should be ignored
const ignore = shouldIgnoreError('ResizeObserver loop error');
```

**File**: `fixtures/error-handling.ts`

**Ignored Errors:**
- ResizeObserver loop errors
- Alpine.js initialization errors
- Common JavaScript errors during testing

### Centralized Exports

All fixtures are re-exported from `fixtures/index.ts` for convenience:

```typescript
import {
  // Database
  resetDB,
  seedDB,
  getEnvironmentInfo,

  // Auth
  login,
  logout,
  waitForAlpine,

  // File upload
  uploadFileAndWaitForAttachment,

  // Test data
  resetTestDatabase,
  populateTestData,
  seedScenario,
  TestDataBuilders,

  // Error handling
  setupErrorHandling,
  shouldIgnoreError
} from '../fixtures';
```

## Troubleshooting

### Database Connection Errors

**Problem**: Tests fail with "Cannot connect to database"

**Solutions:**
1. Verify database is running: `docker ps`
2. Check port in `.env.e2e` matches your setup (5438 for local)
3. Verify database name is `iota_erp_e2e`
4. Test connection manually:
   ```bash
   psql -h localhost -p 5438 -U postgres -d iota_erp_e2e
   ```

### Server Not Running

**Problem**: Tests fail with "net::ERR_CONNECTION_REFUSED"

**Solutions:**
1. Start Go server: `go run cmd/app/main.go`
2. Verify server is on port 3201: `lsof -i :3201`
3. Check `BASE_URL` in `.env.e2e` matches server URL

### Tests Timeout

**Problem**: Tests exceed timeout limits

**Solutions:**
1. Increase timeout in specific test:
   ```typescript
   test('slow test', async ({ page }) => {
     test.setTimeout(120000); // 2 minutes
   });
   ```
2. Use `test.slow()` to triple timeout:
   ```typescript
   test('complex operation', async ({ page }) => {
     test.slow();
     // Test logic...
   });
   ```
3. Check if server is responding slowly

### Alpine.js Errors

**Problem**: Console shows Alpine.js errors

**Solutions:**
1. Add error handling in `beforeEach`:
   ```typescript
   test.beforeEach(async ({ page }) => {
     await setupErrorHandling(page);
   });
   ```
2. Wait for Alpine.js initialization:
   ```typescript
   await waitForAlpine(page);
   ```

### Element Not Found

**Problem**: Locators fail to find elements

**Solutions:**
1. Add explicit wait:
   ```typescript
   await page.waitForSelector('.element');
   ```
2. Check if element is visible:
   ```typescript
   await expect(page.locator('.element')).toBeVisible();
   ```
3. Use Playwright Inspector:
   ```typescript
   await page.pause(); // Opens inspector
   ```
4. Take screenshot for debugging:
   ```typescript
   await page.screenshot({ path: 'debug.png' });
   ```

### TypeScript Errors

**Problem**: Import errors or type errors

**Solutions:**
1. Reinstall dependencies: `pnpm install`
2. Verify `tsconfig.json` exists
3. Check import paths are correct (relative paths)
4. Restart TypeScript server in your editor

## CI/CD Integration

### GitHub Actions

The E2E tests are configured to run in CI with automatic environment detection:

```yaml
# .github/workflows/e2e.yml
- name: Run E2E Tests
  run: |
    cd e2e
    pnpm install
    pnpm test
  env:
    CI: true
    DB_PORT: 5432  # CI uses standard PostgreSQL port
```

### Environment Detection

The `playwright.config.ts` automatically detects CI environment:
- Sets database port to 5432 in CI
- Enables retries (2 attempts) in CI
- Uses single worker in CI for stability
- Captures screenshots and videos on failure

### Test Reports

After CI runs, Playwright generates an HTML report:
```bash
npx playwright show-report
```

## Additional Resources

### Documentation

- **Playwright Official**: https://playwright.dev/docs/intro
- **Test API Reference**: https://playwright.dev/docs/api/class-test
- **Best Practices**: https://playwright.dev/docs/best-practices
- **Debugging Guide**: https://playwright.dev/docs/debug

### Project Files

- **Test Documentation**: `tests/README.md`
- **Configuration**: `playwright.config.ts`
- **Environment**: `.env.e2e`
- **Fixtures**: `fixtures/*.ts`

### Support

For issues or questions:
1. Check this documentation
2. Review existing tests for examples
3. Use debug mode: `pnpm run test:debug`
4. Check Playwright documentation
5. Contact the development team

## Migration Notes

This E2E suite was migrated from Cypress to Playwright for:
- Better TypeScript support
- Faster execution
- More reliable testing
- Modern API and features
- Better debugging tools

All Cypress custom commands have been converted to TypeScript fixtures with improved type safety and error handling.
