import { test, expect, type Page } from '@playwright/test';
import { login, logout, waitForAlpine } from '../../fixtures/auth';
import { resetTestDatabase, seedScenario } from '../../fixtures/test-data';

/**
 * E2E coverage for the wide-dataset scaffold-table primitives (iota-sdk#799),
 * dogfooded on the Expense Categories list:
 *   - WithTruncate()  on the "description" column
 *   - WithPriority(2) on the "created_at" column (hidden below the md breakpoint)
 *   - WithDrawer()    keyboard navigation (arrow keys + Enter)
 *   - horizontal scroll affordance overlays
 *
 * The list lives at /finance/expense-categories. We seed a category through the
 * create drawer so at least one real data row exists regardless of scenario.
 */

const BASE = '/finance/expense-categories';

async function createCategory(page: Page, name: string, description: string) {
  await page.goto(BASE);
  await page.locator('[hx-get$="/new/drawer"]').first().click();
  // Drawer form fields are plain inputs named name/description.
  await page.locator('input[name="name"]').fill(name);
  const desc = page.locator('textarea[name="description"], input[name="description"]').first();
  await desc.fill(description);
  await Promise.all([
    page.waitForResponse((r) => r.url().includes(BASE) && r.request().method() === 'POST'),
    page.getByRole('button', { name: /save|add|create/i }).first().click(),
  ]);
  await page.goto(BASE);
  await waitForAlpine(page);
}

test.describe('Wide-dataset scaffold table (expense categories)', () => {
  test.describe.configure({ mode: 'serial' });

  const longDescription =
    'This is an intentionally very long expense category description that should overflow its truncated column width and therefore trigger the overflow tooltip behaviour.';

  test.beforeAll(async ({ request }) => {
    await resetTestDatabase(request, { reseedMinimal: false });
    await seedScenario(request, 'comprehensive');
  });

  test.beforeEach(async ({ page }) => {
    await login(page, 'test@gmail.com', 'TestPass123!');
  });

  test.afterEach(async ({ page }) => {
    await logout(page);
  });

  test('truncate column wraps cell content in a clamped, tooltip-bound container', async ({
    page,
  }) => {
    await createCategory(page, 'Truncate Cat', longDescription);

    const descCell = page.locator('tbody tr[data-row-drawer] td[data-col="description"]').first();
    await expect(descCell).toBeVisible();

    // Truncated cells wrap their content in a max-width truncate div bound to
    // the cellTruncate Alpine component.
    const clamp = descCell.locator('div.truncate[x-data="cellTruncate"]');
    await expect(clamp).toHaveCount(1);
    const maxWidth = await clamp.evaluate((el) => getComputedStyle(el).maxWidth);
    expect(maxWidth).not.toBe('none');
  });

  test('priority column is hidden on narrow viewports, visible on wide', async ({
    page,
  }) => {
    await page.goto(BASE);
    await waitForAlpine(page);

    const createdHeader = page.locator('thead th[data-col="created_at"]').first();
    await expect(createdHeader).toHaveAttribute('data-col-priority', '2');

    // Desktop: visible.
    await page.setViewportSize({ width: 1280, height: 800 });
    await expect(createdHeader).toBeVisible();

    // Below the md breakpoint (768px): the max-md:hidden class hides it.
    await page.setViewportSize({ width: 640, height: 800 });
    await expect(createdHeader).toBeHidden();
  });

  test('drawer rows are keyboard-navigable and Enter opens the drawer', async ({
    page,
  }) => {
    await createCategory(page, 'Keyboard Cat', 'Short');
    await page.setViewportSize({ width: 1280, height: 800 });

    const rows = page.locator('tbody tr[data-row-drawer]');
    await expect(rows.first()).toBeVisible();

    // Rows expose the accessibility/keyboard hooks.
    await expect(rows.first()).toHaveAttribute('tabindex', '0');
    await expect(rows.first()).toHaveAttribute('role', 'button');

    // Focus first row, ArrowDown moves focus to the next drawer row (if any).
    await rows.first().focus();
    const count = await rows.count();
    if (count > 1) {
      await page.keyboard.press('ArrowDown');
      const secondFocused = await rows.nth(1).evaluate((el) => el === document.activeElement);
      expect(secondFocused).toBe(true);
      await page.keyboard.press('ArrowUp');
    }

    // Enter triggers the row's hx-get into #view-drawer.
    await rows.first().focus();
    await Promise.all([
      page.waitForResponse((r) => r.url().includes('/drawer')),
      page.keyboard.press('Enter'),
    ]);
    await expect(page.locator('#view-drawer')).not.toBeEmpty();
  });

  test('horizontal scroll affordance overlays exist in the table wrapper', async ({
    page,
  }) => {
    await page.goto(BASE);
    await waitForAlpine(page);

    // Gradient overlays are rendered (visibility is driven by scroll state).
    const overlays = page.locator(
      '#sortable-table-container [class*="bg-gradient-to-r"], #sortable-table-container [class*="bg-gradient-to-l"]',
    );
    await expect(overlays.first()).toHaveCount(1);
  });
});
