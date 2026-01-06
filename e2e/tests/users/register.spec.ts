import { test, expect } from '@playwright/test';
import { login, logout } from '../../fixtures/auth';
import { resetTestDatabase, seedScenario } from '../../fixtures/test-data';

test.describe('user auth and registration flow', () => {
	// Tests MUST run serially - each test depends on data created by previous tests
	test.describe.configure({ mode: 'serial' });

	// Reset database once for entire suite - tests are dependent
	test.beforeAll(async ({ request }) => {
		// Reset database and seed with comprehensive data including users and roles
		await resetTestDatabase(request, { reseedMinimal: false });
		await seedScenario(request, 'comprehensive');
	});

	test.beforeEach(async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 720 });
	});

	test.afterEach(async ({ page }) => {
		await logout(page);
	});

	test('creates a user and displays changes in users table', async ({ page }) => {
		await login(page, 'test@gmail.com', 'TestPass123!');

		await page.goto('/users');
		await expect(page).toHaveURL(/\/users$/);

		// Click the "New User" link
		await page.locator('a[href="/users/new"]').filter({ hasText: /.+/ }).first().click();

		// Fill in the form
		await page.locator('[name=FirstName]').fill('Test');
		await page.locator('[name=LastName]').fill('User');
		await page.locator('[name=MiddleName]').fill('Mid');
		await page.locator('[name=Email]').fill('test1@gmail.com');
		await page.locator('[name=Phone]').fill('+998901234567');
		await page.locator('[name=Password]').fill('TestPass123!');
		await page.locator('[name=Language]').selectOption({ index: 2 });

		// Handle Alpine.js dropdown for RoleIDs
		// Find the combobox container and click the dropdown indicator (caret down icon)
		const roleCombobox = page.locator('select[name="RoleIDs"]').locator('..');
		await roleCombobox.locator('svg.cursor-pointer').click(); // Click the CaretDown icon with cursor-pointer class

		// Wait for dropdown to be visible and click first option (scope to role combobox)
		const roleDropdown = roleCombobox.locator('ul[x-ref=list]');
		await expect(roleDropdown).toBeVisible();
		await roleDropdown.locator('li').first().click();

		// Save the form
		await page.locator('[id=save-btn]').click();

		// Wait for redirect after save
		await page.waitForURL(/\/users$/);

		// Verify user appears in table (comprehensive seed creates 3 users + 1 new = 4 total)
		await expect(page.locator('tbody tr')).toHaveCount(4);

		await logout(page);

		// Login as the newly created user
		await login(page, 'test1@gmail.com', 'TestPass123!');
		await page.goto('/users');

		await expect(page).toHaveURL(/\/users/);
		await expect(page.locator('tbody tr')).toHaveCount(4);
	});

	test('edits a user and displays changes in users table', async ({ page }) => {
		// Login as admin user (not the newly created user from test 1)
		await login(page, 'test@gmail.com', 'TestPass123!');

		await page.goto('/users');
		await expect(page).toHaveURL(/\/users/);

		// Find and click the edit link for the user created in test 1 (use email for unambiguous selection)
		const userRow = page.locator('tbody tr').filter({ hasText: 'test1@gmail.com' });
		await userRow.locator('td a').click();

		await expect(page).toHaveURL(/\/users\/.+/);

		// Edit the user details
		await page.locator('[name=FirstName]').fill('TestNew');
		await page.locator('[name=LastName]').fill('UserNew');
		await page.locator('[name=MiddleName]').fill('MidNew');
		await page.locator('[name=Email]').fill('test1new@gmail.com');
		await page.locator('[name=Phone]').fill('+998909876543');
		await page.locator('[name=Language]').selectOption({ index: 1 });
		await page.locator('[id=save-btn]').click();

		// Wait for redirect after save
		await page.waitForURL(/\/users$/);

		// Verify changes in the users list (still 4 users total)
		await expect(page.locator('tbody tr')).toHaveCount(4);
		await expect(page.locator('tbody tr').filter({ hasText: 'TestNew UserNew' })).toBeVisible();

		// Verify phone number persists by checking the edit page
		const updatedUserRow = page.locator('tbody tr').filter({ hasText: 'TestNew UserNew' });
		await updatedUserRow.locator('td a').click();
		await expect(page).toHaveURL(/\/users\/.+/);
		await expect(page.locator('[name=Phone]')).toHaveValue('998909876543');

		await logout(page);

		// Login with the updated email
		await login(page, 'test1new@gmail.com', 'TestPass123!');
		await page.goto('/users');
		await expect(page).toHaveURL(/\/users/);
	});

	test('newly created user should see tabs in the sidebar', async ({ page }) => {
		// Login with the updated email from test 2 (test1@gmail.com was changed to test1new@gmail.com)
		await login(page, 'test1new@gmail.com', 'TestPass123!');
		await page.goto('/');
		await expect(page).not.toHaveURL(/\/login/);

		// Check that the sidebar contains at least one tab/link
		const sidebarItems = page.locator('#sidebar-navigation li');
		const count = await sidebarItems.count();
		expect(count).toBeGreaterThanOrEqual(1);
	});
});
