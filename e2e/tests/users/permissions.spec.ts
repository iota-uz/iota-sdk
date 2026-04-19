import { test, expect } from '@playwright/test';
import { login, logout, waitForAlpine } from '../../fixtures/auth';
import { resetTestDatabase, seedScenario } from '../../fixtures/test-data';

test.describe('user direct permission editing', () => {
	test.beforeAll(async ({ request }) => {
		await resetTestDatabase(request, { reseedMinimal: false });
		await seedScenario(request, 'comprehensive');
	});

	test.beforeEach(async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 720 });
	});

	test.afterEach(async ({ page }) => {
		await logout(page);
	});

	test('permission toggles are submitted with the save form and persist after edit', async ({ page }) => {
		await login(page, 'test@gmail.com', 'TestPass123!');

		await page.goto('/users/new');
		await expect(page).toHaveURL(/\/users\/new$/);

		await page.locator('[name=FirstName]').fill('Permission');
		await page.locator('[name=LastName]').fill('Target');
		await page.locator('[name=MiddleName]').fill('Regression');
		await page.locator('[name=Email]').fill('permissions-target@example.com');
		await page.locator('[name=Phone]').fill('+998901112233');
		await page.locator('[name=Password]').fill('TestPass123!');
		await page.locator('[name=Language]').selectOption({ value: 'en' });
		await page.locator('#save-btn').click();

		await page.waitForURL(/\/users$/);

		const createdUserRow = page.locator('tbody tr').filter({ hasText: 'Permission Target' });
		await expect(createdUserRow).toBeVisible();

		await createdUserRow.locator('td a[href$="/edit"]').click();
		await expect(page).toHaveURL(/\/users\/\d+\/edit$/);

		await page.getByRole('button', { name: /permissions/i }).click();
		await waitForAlpine(page);
		await expect(page.locator('input[type="checkbox"][name^="Permissions["]').first()).toBeAttached();

		const selectedPermissions = await page.evaluate(() => {
			const form = document.getElementById('save-form');
			if (!(form instanceof HTMLFormElement)) {
				throw new Error('save-form was not rendered');
			}

			const inputs = Array.from(
				document.querySelectorAll<HTMLInputElement>('input[type="checkbox"][name^="Permissions["]'),
			).filter((input) => !input.checked);

			if (inputs.length < 2) {
				throw new Error('expected at least two unchecked permission toggles');
			}

			const selected = inputs.slice(0, 2).map((input) => {
				input.checked = true;
				input.dispatchEvent(new Event('change', { bubbles: true }));
				return input.name;
			});

			const submitted = Array.from(new FormData(form).keys()).filter((key) => key.startsWith('Permissions['));

			return { selected, submitted };
		});

		expect(selectedPermissions.submitted).toEqual(expect.arrayContaining(selectedPermissions.selected));

		await page.locator('#save-btn').click();
		await page.waitForURL(/\/users$/);

		const updatedUserRow = page.locator('tbody tr').filter({ hasText: 'Permission Target' });
		await expect(updatedUserRow).toBeVisible();
		await updatedUserRow.locator('td a[href$="/edit"]').click();
		await expect(page).toHaveURL(/\/users\/\d+\/edit$/);

		await page.getByRole('button', { name: /permissions/i }).click();
		await waitForAlpine(page);

		for (const permissionName of selectedPermissions.selected) {
			await expect(page.locator(`input[name="${permissionName}"]`)).toBeChecked();
		}
	});
});
