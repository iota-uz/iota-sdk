---
name: E2E Testing
description: >-
  Run, debug, and fix Playwright E2E tests locally. Use when writing new E2E tests,
  debugging failing tests, or iterating on test fixes before pushing to CI.
---

## Prerequisites

### 1. E2E Database

The E2E tests use a separate `iota_erp_e2e` database. Create it if missing:

```bash
# Check DB connection params from .env (DB_HOST, DB_PORT, DB_PASSWORD)
grep '^DB_' .env

# Create the database (adjust host/port from .env)
PGPASSWORD=postgres psql -U postgres -h localhost -p 5438 -c "CREATE DATABASE iota_erp_e2e;"

# Run migrations
just e2e migrate up
```

### 2. Start the E2E Server

The server must run with test endpoints enabled:

```bash
PORT=3201 ORIGIN='http://localhost:3201' DB_NAME=iota_erp_e2e ENABLE_TEST_ENDPOINTS=true \
  go run -tags e2e ./cmd/server/main.go
```

Or use `just e2e dev` (uses `air` hot-reload, same env vars).

Verify it's running:

```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:3201/login
# Should return 200
```

### 3. Stop the Server When Done

```bash
kill $(lsof -ti:3201) 2>/dev/null
```

## Running Tests

```bash
# Run a specific test file
cd e2e && npx playwright test tests/roles/roles.spec.ts --reporter=line

# Run a specific test by line number
cd e2e && npx playwright test tests/roles/roles.spec.ts:26 --reporter=line

# Run all E2E tests (headless)
just e2e ci

# Run with Playwright UI (interactive debugging)
just e2e run
```

## Debug Loop

When fixing a failing E2E test, follow this loop:

1. **Reproduce locally first** — never iterate via CI pushes
2. **Run the specific failing test** by line number for fast feedback
3. **Check screenshots** in `e2e/test-results/` after failures
4. **Check video recordings** (`.webm`) for timing/animation issues
5. **Read the error context** file in test-results for DOM snapshots
6. **Fix and re-run** until the specific test passes
7. **Run the full test file** to catch regressions in serial tests
8. **Then push to CI**

```bash
# Quick iteration: run just the failing test
cd e2e && npx playwright test tests/module/test.spec.ts:LINE --reporter=line

# After fixing, run the full suite
cd e2e && npx playwright test tests/module/test.spec.ts --reporter=line

# View failure screenshot
# (path shown in test output, use Read tool to view)
```

## Common Patterns

### Dialog Confirmation Buttons

The `<dialog>` top-layer positioning confuses Playwright's `elementFromPoint()` hit testing
in headless Chromium. The bottom action bar (sticky footer) intercepts pointer events even
though the dialog is visually on top.

**Workaround**: After verifying the dialog is visible, trigger the htmx form submit directly
instead of clicking the confirm button:

```typescript
// Open the confirmation dialog
await page.locator('[data-test-id="delete-btn"]').click();

// Wait for dialog to appear
const dialog = page.locator('[data-test-id="delete-confirmation-dialog"]');
await expect(dialog).toBeVisible();

// Trigger htmx delete directly (bypasses dialog hit-test issue)
await page.evaluate(() => {
  const form = document.getElementById('delete-form') as HTMLFormElement;
  (window as any).htmx.trigger(form, 'submit');
});

await page.waitForURL(/\/expected-path$/);
```

### Selectors

Prefer `data-test-id` attributes over text-based or structural selectors:

```typescript
// Good
page.locator('[data-test-id="save-role-btn"]')
page.locator('[data-test-id="dialog-confirm-btn"]')

// Fragile
page.locator('button').filter({ hasText: /Save/i })
page.getByRole('button', { name: /confirm/i })
```

### Waiting for Alpine.js

```typescript
import { waitForAlpine } from '../../fixtures/auth';
await waitForAlpine(page);
```

### Database Reset and Seeding

Tests use `beforeAll` to reset and seed:

```typescript
import { resetTestDatabase, seedScenario } from '../../fixtures/test-data';

test.beforeAll(async ({ request }) => {
  await resetTestDatabase(request, { reseedMinimal: false });
  await seedScenario(request, 'comprehensive');
});
```

## Test Structure

```
e2e/
├── fixtures/
│   ├── auth.ts          # login, logout, waitForAlpine helpers
│   └── test-data.ts     # resetTestDatabase, seedScenario
├── tests/
│   ├── roles/           # Role management tests
│   ├── users/           # User management tests
│   └── ...
├── playwright.config.ts # Playwright config (1280x720 viewport)
└── test-results/        # Screenshots, videos, traces from failures
```

## Key Config

- **Viewport**: 1280x720 (Desktop Chrome)
- **Test timeout**: 60s
- **Retries in CI**: 2 (none locally by default)
- **Workers**: 1 (serial execution for data-dependent tests)
- **Base URL**: `http://localhost:3201`
