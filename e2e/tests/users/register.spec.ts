import { test, expect } from '@playwright/test';
import { login, logout } from '../../fixtures/auth';
import { resetTestDatabase, seedScenario } from '../../fixtures/test-data';

test.describe('user auth and registration flow', () => {
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
		await page.locator('[name=Phone]').fill('+14155551234');
		await page.locator('[name=Password]').fill('TestPass123!');
		await page.locator('[name=Language]').selectOption({ index: 2 });

		// Handle Alpine.js dropdown for RoleIDs
		const roleSelect = page.locator('select[name="RoleIDs"]');
		const roleContainer = await roleSelect.locator('xpath=ancestor::div[1]');
		await roleContainer.locator('button[x-ref="trigger"]').click();

		// Wait for dropdown to be visible and click first option
		const dropdown = page.locator('ul[x-ref=list]');
		await expect(dropdown).toBeVisible();
		await dropdown.locator('li').first().click();

		// Save the form
		await page.locator('[id=save-btn]').click();

		// Verify user appears in table
		await expect(page.locator('tbody tr')).toHaveCount(4); // including the spinner row

		await logout(page);

		// Login as the newly created user
		await login(page, 'test1@gmail.com', 'TestPass123!');
		await page.goto('/users');

		await expect(page).toHaveURL(/\/users/);
		await expect(page.locator('tbody tr')).toHaveCount(4); // including the spinner row
	});

	test('edits a user and displays changes in users table', async ({ page }) => {
		await login(page, 'test1@gmail.com', 'TestPass123!');

		await page.goto('/users');
		await expect(page).toHaveURL(/\/users/);

		// Find and click the edit link for "Test User"
		const userRow = page.locator('tbody tr').filter({ hasText: 'Test User' });
		await userRow.locator('td a').click();

		await expect(page).toHaveURL(/\/users\/.+/);

		// Edit the user details
		await page.locator('[name=FirstName]').fill('TestNew');
		await page.locator('[name=LastName]').fill('UserNew');
		await page.locator('[name=MiddleName]').fill('MidNew');
		await page.locator('[name=Email]').fill('test1new@gmail.com');
		await page.locator('[name=Phone]').fill('+14155559876');
		await page.locator('[name=Language]').selectOption({ index: 1 });
		await page.locator('[id=save-btn]').click();

		// Verify changes in the users list
		await page.goto('/users');
		await expect(page.locator('tbody tr')).toHaveCount(4); // including the spinner row
		await expect(page.locator('tbody tr')).toContainText('TestNew UserNew');

		// Verify phone number persists by checking the edit page
		const updatedUserRow = page.locator('tbody tr').filter({ hasText: 'TestNew UserNew' });
		await updatedUserRow.locator('td a').click();
		await expect(page).toHaveURL(/\/users\/.+/);
		await expect(page.locator('[name=Phone]')).toHaveValue('14155559876');

		await logout(page);

		// Login with the updated email
		await login(page, 'test1new@gmail.com', 'TestPass123!');
		await page.goto('/users');
		await expect(page).toHaveURL(/\/users/);
	});

	test('newly created user should see tabs in the sidebar', async ({ page }) => {
		await login(page, 'test1@gmail.com', 'TestPass123!');
		await page.goto('/');
		await expect(page).not.toHaveURL(/\/login/);

		// Check that the sidebar contains at least one tab/link
		const sidebarItems = page.locator('#sidebar-navigation li');
		await expect(sidebarItems).toHaveCount(expect.any(Number));
		const count = await sidebarItems.count();
		expect(count).toBeGreaterThanOrEqual(1);
	});
});
