import { test, expect } from '@playwright/test';
import { login, logout } from '../../fixtures/auth';
import { resetTestDatabase, seedScenario } from '../../fixtures/test-data';

test.describe('user realtime behavior', () => {
	test.beforeAll(async ({ request }) => {
		// Reset database and seed with comprehensive data for user management tests
		await resetTestDatabase(request, { reseedMinimal: false });
		await seedScenario(request, 'comprehensive');
	});

	test.beforeEach(async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 720 });
	});

	test.afterEach(async ({ page }) => {
		await logout(page);
	});

	test('updates user table in realtime when a user is created, edited, and deleted', async ({ page, request }) => {
		await login(page, 'test@gmail.com', 'TestPass123!');
		await page.goto('/users');
		await expect(page).toHaveURL(/\/users$/);

		// Get initial row count
		const initialRows = page.locator('tbody tr');
		const initialRowCount = await initialRows.count();

		// This simulates adding a user through a different session
		await request.post('/users', {
			form: {
				FirstName: 'Realtime',
				LastName: 'Test',
				MiddleName: 'Mid',
				Email: 'realtime@gmail.com',
				Phone: '+14155551234',
				Password: 'TestPass123!',
				Language: 'en',
				RoleIDs: '1',
			}
		});

		// Verify user was added in realtime
		await expect(page.locator('tbody tr')).toHaveCount(initialRowCount + 1);
		await expect(page.locator('tbody tr').filter({ hasText: 'Realtime Test' })).toBeVisible();

		// Get the user ID from the href attribute of the edit link
		const realtimeUserRow = page.locator('tbody tr').filter({ hasText: 'Realtime Test' });
		const editLink = realtimeUserRow.locator('td a');
		const href = await editLink.getAttribute('href');
		const userId = href!.split('/').pop();

		// Edit the user through a direct request (staying on the users page)
		await request.post(`/users/${userId}`, {
			form: {
				FirstName: 'RealtimeUpdated',
				LastName: 'TestUpdated',
				MiddleName: 'Mid',
				Email: 'realtime@gmail.com',
				Phone: '+14155559876',
				Language: 'en',
				RoleIDs: '1',
			}
		});

		// Verify user was updated in the table without refreshing
		await expect(page.locator('tbody tr').filter({ hasText: 'RealtimeUpdated TestUpdated' })).toBeVisible();
		await expect(page.locator('tbody tr').filter({ hasText: 'Realtime Test' })).toHaveCount(0);
		await expect(page.locator('tbody tr')).toHaveCount(initialRowCount + 1);

		// Delete the user through a direct request
		await request.delete(`/users/${userId}`);

		// Verify user was removed from the table without refreshing
		await expect(page.locator('tbody tr').filter({ hasText: 'RealtimeUpdated TestUpdated' })).toHaveCount(0);
		await expect(page.locator('tbody tr')).toHaveCount(initialRowCount);
	});
});
