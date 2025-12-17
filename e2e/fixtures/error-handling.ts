/**
 * Error handling configuration for Playwright tests
 *
 * Provides utilities to handle expected errors in page context.
 */

import { Page } from '@playwright/test';

/**
 * Errors that should be ignored during testing
 */
const IGNORED_ERROR_PATTERNS = [
	// Ignore ResizeObserver loop errors
	/ResizeObserver loop/,

	// Ignore Alpine.js initialization errors during testing
	/value is not defined/,
	/Alpine/,
	/Cannot read properties of undefined/,
	/Cannot convert undefined or null to object/,
];

/**
 * Check if an error should be ignored based on patterns
 *
 * @param errorMessage - The error message to check
 * @returns true if error should be ignored
 */
export function shouldIgnoreError(errorMessage: string): boolean {
	return IGNORED_ERROR_PATTERNS.some(pattern => pattern.test(errorMessage));
}

/**
 * Setup error handling for a page
 * Call this in test.beforeEach or at the start of each test
 *
 * @param page - Playwright page object
 */
export async function setupErrorHandling(page: Page) {
	// Listen for console errors and filter out known issues
	page.on('console', msg => {
		if (msg.type() === 'error') {
			const text = msg.text();
			if (shouldIgnoreError(text)) {
				console.warn('Ignored known error during testing:', text);
			}
		}
	});

	// Listen for page errors and handle them
	page.on('pageerror', error => {
		if (shouldIgnoreError(error.message)) {
			console.warn('Ignored known page error during testing:', error.message);
			// Don't throw, just log
		} else {
			// Let other errors fail the test
			throw error;
		}
	});
}

/**
 * Example usage in tests:
 *
 * import { test } from '@playwright/test';
 * import { setupErrorHandling } from '../fixtures/error-handling';
 *
 * test.beforeEach(async ({ page }) => {
 *   await setupErrorHandling(page);
 * });
 */
