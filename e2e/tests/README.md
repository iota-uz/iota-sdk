# Playwright Test Directory

This directory contains all E2E tests for the IOTA SDK platform, organized by module.

## Directory Structure

```
tests/
├── users/              # User management and authentication tests
│   ├── register.spec.ts
│   └── realtime.spec.ts
├── employees/          # Employee management tests
│   └── employees.spec.ts
└── README.md           # This file
```

## Test Organization

Tests are organized by business module, matching the application's module structure:
- `users/` - User registration, authentication, profile management
- `employees/` - Employee CRUD operations, assignments
- Future modules: `finance/`, `warehouse/`, `crm/`, etc.

## Writing Tests

### Basic Test Template

```typescript
import { test, expect } from '@playwright/test';
import { login, resetDB, seedDB, setupErrorHandling } from '../../fixtures';

test.describe('Feature Name', () => {
  test.beforeEach(async ({ page }) => {
    // Setup error handling for Alpine.js and ResizeObserver
    await setupErrorHandling(page);

    // Reset and seed database
    await resetDB();
    await seedDB();

    // Login if needed
    await login(page, 'user@example.com', 'password');
  });

  test('should perform action successfully', async ({ page }) => {
    await page.goto('/feature-path');

    // Your test logic here
    await page.fill('input[name="field"]', 'value');
    await page.click('button[type="submit"]');

    // Assertions
    await expect(page.locator('.success-message')).toBeVisible();
  });
});
```

### Available Fixtures

Import fixtures from `../../fixtures` (or appropriate relative path):

```typescript
// Database operations
import { resetDB, seedDB, getEnvironmentInfo } from '../../fixtures';

// Authentication
import { login, logout, waitForAlpine } from '../../fixtures';

// Error handling (recommended for all tests)
import { setupErrorHandling } from '../../fixtures';

// File upload
import { uploadFileAndWaitForAttachment } from '../../fixtures';

// Test data (requires request context)
import {
  resetTestDatabase,
  populateTestData,
  seedScenario,
  TestDataBuilders
} from '../../fixtures';
```

## Running Tests

### Run All Tests
```bash
npm test
```

### Run Specific Module
```bash
npm run test:users
npm run test:employees
```

### Run Specific Test File
```bash
npx playwright test tests/users/register.spec.ts
```

### Run Tests in UI Mode (Interactive)
```bash
npm run test:ui
```

### Run Tests in Headed Mode (See Browser)
```bash
npm run test:headed
```

### Run Tests in Debug Mode
```bash
npm run test:debug
```

### Run Specific Test by Name
```bash
npx playwright test --grep "should register successfully"
```

## Test Best Practices

### 1. Use Descriptive Test Names
```typescript
// Good
test('should display validation error when email is invalid', async ({ page }) => {

// Avoid
test('test1', async ({ page }) => {
```

### 2. Always Setup Error Handling
```typescript
test.beforeEach(async ({ page }) => {
  await setupErrorHandling(page);
});
```

### 3. Reset Database Before Each Test
```typescript
test.beforeEach(async ({ page }) => {
  await resetDB();
  await seedDB();
});
```

### 4. Use Fixtures for Common Operations
```typescript
// Use fixture
await login(page, email, password);

// Instead of repeating login logic
```

### 5. Wait for Alpine.js Initialization
```typescript
// After navigation to pages with Alpine.js
await waitForAlpine(page);
```

### 6. Use Proper Locators
```typescript
// Good - Specific and stable
page.locator('[data-test="submit-button"]')
page.getByRole('button', { name: 'Submit' })
page.getByLabel('Email address')

// Avoid - Fragile
page.locator('button').nth(2)
page.locator('.btn-primary')
```

## Common Patterns

### Form Submission
```typescript
await page.fill('input[name="email"]', 'user@example.com');
await page.fill('input[name="password"]', 'password123');
await page.click('button[type="submit"]');
await expect(page).toHaveURL('/dashboard');
```

### Table Interaction
```typescript
const rows = page.locator('table tbody tr');
await expect(rows).toHaveCount(5);
await rows.first().click();
```

### File Upload
```typescript
await uploadFileAndWaitForAttachment(
  page,
  'file content',
  'document.txt',
  'text/plain'
);
```

### API Calls in Tests
```typescript
test('API endpoint test', async ({ request }) => {
  const response = await request.post('/api/users', {
    data: { name: 'John Doe' }
  });
  expect(response.ok()).toBeTruthy();
});
```

## Troubleshooting

### Database Connection Issues
If tests fail with database connection errors:
1. Check `.env.e2e` has correct `DB_PORT` (5432 for CI, 5438 for local)
2. Verify database is running: `docker ps`
3. Check database name matches `DB_NAME=iota_erp_e2e`

### Tests Timeout
If tests are timing out:
1. Check if Go server is running on correct port (3201)
2. Increase timeout for specific test: `test.setTimeout(120000)`
3. Use `test.slow()` to triple timeout for slow operations

### Alpine.js Errors
If you see Alpine.js errors in console:
1. Add `setupErrorHandling(page)` in `beforeEach`
2. Use `waitForAlpine(page)` after navigation

### Element Not Found
If locators fail to find elements:
1. Check element is visible: `await expect(locator).toBeVisible()`
2. Wait for element: `await page.waitForSelector('selector')`
3. Use debug mode: `npm run test:debug`

## Debugging Tips

### Pause Test Execution
```typescript
await page.pause(); // Opens Playwright Inspector
```

### Take Screenshot
```typescript
await page.screenshot({ path: 'debug.png' });
```

### View Console Output
```typescript
page.on('console', msg => console.log(msg.text()));
```

### Slow Down Execution
```typescript
test.slow(); // Triples the timeout
```

## Related Documentation

- Parent directory: `../README.md` - Complete E2E testing guide
- Fixtures: `../fixtures/` - Shared test utilities
- Configuration: `../playwright.config.ts` - Test configuration
- Environment: `../.env.e2e` - Environment variables
