import { test, expect } from '@playwright/test';

/**
 * Login page theme toggle E2E tests.
 *
 * Covers two defects fixed together:
 * - #994: choosing "System" applied a literal `system` class to <html>;
 *   Tailwind's class strategy only reacts to `.dark`, so the app stayed light
 *   regardless of the OS preference (and stopped following OS changes).
 * - #1082: the Desktop/Sun/Moon icons used the v3 `peer-checked:group-[]`
 *   variant that Tailwind v4 never emitted, so they stayed at scale 0 — the
 *   toggle rendered as an empty circle.
 *
 * The login page needs no authenticated session, so no fixtures beyond the
 * running server are required.
 */

const THEME_STORAGE_KEY = 'iota-theme';

test.describe('Login page theme toggle', () => {
	test.beforeEach(async ({ page }) => {
		await page.goto('/login');
		await page.evaluate((key) => window.localStorage.removeItem(key), THEME_STORAGE_KEY);
		await page.reload();
	});

	test('resolves "system" against the OS color scheme', async ({ page }) => {
		await page.emulateMedia({ colorScheme: 'dark' });
		await page.reload();

		await expect(page.locator('html')).toHaveClass(/(^|\s)dark($|\s)/);
		await expect(page.locator('html')).not.toHaveClass(/system/);

		await page.emulateMedia({ colorScheme: 'light' });
		await expect(page.locator('html')).toHaveClass(/(^|\s)light($|\s)/);
	});

	test('follows OS theme changes live while "system" is selected', async ({ page }) => {
		await page.emulateMedia({ colorScheme: 'light' });
		await page.reload();
		await expect(page.locator('html')).toHaveClass(/(^|\s)light($|\s)/);

		// emulateMedia fires the prefers-color-scheme change event; with
		// "system" active the listener must re-resolve without a reload.
		await page.emulateMedia({ colorScheme: 'dark' });
		await expect(page.locator('html')).toHaveClass(/(^|\s)dark($|\s)/);
	});

	test('stores the raw choice and applies the resolved class', async ({ page }) => {
		await page.emulateMedia({ colorScheme: 'dark' });

		await page.locator('#theme-dark').check();
		await expect(page.locator('html')).toHaveClass(/(^|\s)dark($|\s)/);
		expect(await page.evaluate((key) => window.localStorage.getItem(key), THEME_STORAGE_KEY)).toBe('dark');

		await page.locator('#theme-light').check();
		await expect(page.locator('html')).toHaveClass(/(^|\s)light($|\s)/);
		expect(await page.evaluate((key) => window.localStorage.getItem(key), THEME_STORAGE_KEY)).toBe('light');
	});

	test('shows the icon of the selected option at full scale', async ({ page }) => {
		const icon = page.locator('label[for="theme-light"] .iota-theme-icon');
		await expect(icon).toBeVisible();

		// The pre-fix bug: Tailwind v4 never emitted the peer-checked:group-[]
		// scale variant, leaving the icon at scale(0) — an empty toggle. The
		// icon must now render with a non-zero box.
		const box = await icon.boundingBox();
		expect(box).not.toBeNull();
		expect(box!.width).toBeGreaterThan(0);
		expect(box!.height).toBeGreaterThan(0);
	});
});
