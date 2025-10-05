/**
 * Authentication fixtures for Playwright tests
 *
 * Migrated from Cypress custom commands (cypress/support/commands.js)
 */

import { Page } from '@playwright/test';

/**
 * Login helper function
 * Migrates Cypress.Commands.add("login")
 *
 * @param page - Playwright page object
 * @param email - User email
 * @param password - User password
 */
export async function login(page: Page, email: string, password: string) {
	await page.goto('/login');
	await page.fill('[type=email]', email);
	await page.fill('[type=password]', password);

	// Wait for navigation BEFORE clicking submit (Playwright best practice)
	// This prevents race conditions where navigation completes before waitForURL is called
	await Promise.all([
		page.waitForURL(url => !url.pathname.includes('/login')),
		page.click('[type=submit]')
	]);
}

/**
 * Logout helper function
 * Migrates Cypress.Commands.add("logout")
 *
 * @param page - Playwright page object
 */
export async function logout(page: Page) {
	await page.goto('/logout');
}

/**
 * Wait for Alpine.js initialization
 * Migrates Cypress.Commands.add("waitForAlpine")
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
