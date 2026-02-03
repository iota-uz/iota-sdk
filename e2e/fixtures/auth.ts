/**
 * Authentication fixtures for Playwright tests
 */

import { Page } from '@playwright/test';

const LOGIN_ATTEMPTS = 3;
const LOGIN_NAV_TIMEOUT_MS = 60000;
const RETRY_DELAY_MS = 4000;

/**
 * Login helper function. Retries on network errors (e.g. ERR_EMPTY_RESPONSE in CI).
 *
 * @param page - Playwright page object
 * @param email - User email
 * @param password - User password
 */
export async function login(page: Page, email: string, password: string) {
	for (let attempt = 1; attempt <= LOGIN_ATTEMPTS; attempt++) {
		try {
			await page.goto('/login', { waitUntil: 'domcontentloaded', timeout: 30000 });
			await page.waitForSelector('[type=submit]', { state: 'visible', timeout: 10000 });
			await page.fill('[type=email]', email);
			await page.fill('[type=password]', password);

			// Wait for navigation after submit (Playwright: start wait, then trigger)
			await Promise.all([
				page.waitForURL(url => !url.pathname.includes('/login'), { timeout: LOGIN_NAV_TIMEOUT_MS }),
				page.click('[type=submit]')
			]);
			return;
		} catch (err) {
			const msg = err instanceof Error ? err.message : String(err);
			const isRetryable = attempt < LOGIN_ATTEMPTS && (
				msg.includes('ERR_EMPTY_RESPONSE') ||
				msg.includes('timeout') ||
				msg.includes('Timeout')
			);
			if (!isRetryable) throw err;
			await new Promise(r => setTimeout(r, RETRY_DELAY_MS));
		}
	}
}

/**
 * Logout helper function
 *
 * @param page - Playwright page object
 */
export async function logout(page: Page) {
	await page.goto('/logout');
}

/**
 * Wait for Alpine.js initialization
 *
 * @param page - Playwright page object
 * @param timeout - Maximum wait time in ms (default: 5000)
 */
export async function waitForAlpine(page: Page, timeout: number = 5000) {
	// Wait for Alpine.js to be available on window
	await page.waitForFunction(
		() => {
			const win = window as any;
			return win.Alpine && win.Alpine.version;
		},
		{ timeout }
	).catch(() => {
		// Don't fail if Alpine isn't available, just continue
		console.warn('Alpine.js not detected within timeout, continuing anyway');
	});

	// Wait for body to be visible
	await page.waitForSelector('body', { state: 'visible' });

	// Allow time for initialization
	await page.waitForTimeout(1000);
}
