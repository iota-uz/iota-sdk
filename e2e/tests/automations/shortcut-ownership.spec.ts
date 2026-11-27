/**
 * Shortcut ownership: ⌘K in the automation editor opens the editor's add-step palette and only that; the
 * shell's global search keeps ⌘K everywhere else, and stays reachable in the editor from its button.
 */
import { test, expect } from '../../fixtures/test';
import { login } from '../../fixtures/auth';
import { ADMIN, PASSWORD } from './inbox';

const search = (page: import('@playwright/test').Page) => page.locator('[x-data="spotlight"] [role="dialog"]');

test.describe('automations: shortcut ownership', () => {
	test('⌘K opens one dialog: the palette in the editor, global search elsewhere', async ({ page }) => {
		await login(page, ADMIN, PASSWORD);

		await page.goto('/automations/new');
		await expect(page.locator('.ae-canvas')).toBeVisible();
		await expect(page.locator('.ae-editor')).toHaveAttribute('data-claim-shortcuts', /(^| )mod\+k( |$)/);

		// Focus on the canvas, clicked above the empty-state card that covers its middle.
		await page.locator('.ae-canvas').click({ position: { x: 500, y: 40 } });
		await page.keyboard.press('ControlOrMeta+k');
		await expect(page.locator('.ae-palette')).toBeVisible();
		await expect(search(page)).toBeHidden();
		await page.keyboard.press('Escape');
		await expect(page.locator('.ae-palette')).toBeHidden();

		// Nothing focused (the body).
		await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
		await page.keyboard.press('ControlOrMeta+k');
		await expect(page.locator('.ae-palette')).toBeVisible();
		await expect(search(page)).toBeHidden();
		await page.keyboard.press('Escape');

		// Global search is still one click away inside the editor.
		await page.getByTestId('spotlight-trigger').click();
		await expect(search(page)).toBeVisible();
		await expect(page.locator('.ae-palette')).toBeHidden();
		await page.keyboard.press('Escape');

		// Outside the editor ⌘K is the shell's again.
		await page.goto('/automations');
		await page.keyboard.press('ControlOrMeta+k');
		await expect(search(page)).toBeVisible();
	});
});
