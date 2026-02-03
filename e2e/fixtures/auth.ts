/**
 * Authentication fixtures for Playwright tests
 */

import { Page } from '@playwright/test';

const LOGIN_ATTEMPTS = 3;
const RETRY_DELAY_MS = 4000;

/**
 * Login via API request then load app. Avoids ERR_EMPTY_RESPONSE on form redirect in CI.
 * Form field names match modules/core login: Email, Password.
 *
 * @param page - Playwright page object
 * @param email - User email
 * @param password - User password
 */
export async function login(page: Page, email: string, password: string) {
	for (let attempt = 1; attempt <= LOGIN_ATTEMPTS; attempt++) {
		try {
			const response = await page.request.post('/login', {
				form: { Email: email, Password: password },
				maxRedirects: 2,
				timeout: 30000,
			});
			if (!response.ok() && response.status() !== 302) {
				throw new Error(`Login failed: ${response.status()} ${await response.text()}`);
			}
			await page.goto('/', { waitUntil: 'domcontentloaded', timeout: 15000 });
			const url = page.url();
			if (url.includes('/login')) {
				throw new Error('Still on login page after POST');
			}
			return;
		} catch (err) {
			const msg = err instanceof Error ? err.message : String(err);
			const isRetryable = attempt < LOGIN_ATTEMPTS && (
				msg.includes('ERR_EMPTY_RESPONSE') ||
				msg.includes('timeout') ||
				msg.includes('Timeout') ||
				msg.includes('Still on login')
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
