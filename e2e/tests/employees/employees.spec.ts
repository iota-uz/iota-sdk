import { test, expect } from '@playwright/test';
import { login, logout } from '../../fixtures/auth';
import { resetTestDatabase, seedScenario } from '../../fixtures/test-data';

test.describe('employees CRUD operations', () => {
	test.beforeAll(async ({ request }) => {
		// Reset database and seed with comprehensive data for employee management
		await resetTestDatabase(request, { reseedMinimal: false });
		await seedScenario(request, 'comprehensive');
	});

	test.beforeEach(async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 720 });
	});

	test.afterEach(async ({ page }) => {
		await logout(page);
	});

	test('displays employees list page', async ({ page }) => {
		await login(page, 'test@gmail.com', 'TestPass123!');

		await page.goto('/hrm/employees');
		await expect(page).toHaveURL(/\/hrm\/employees$/);

		// Check page title and main elements
		await expect(page.locator('h1')).toContainText('Employees');
		await expect(page.locator('a[href="/hrm/employees/new"]')).toBeVisible();

		// Check search and filter form
		await expect(page.locator('form input[name="name"]')).toBeVisible();
		await expect(page.locator('form select[name="limit"]')).toBeVisible();
	});
});
