/**
 * Database fixtures for Playwright tests
 *
 * This module provides utilities for database operations in tests,
 * matching the functionality of Cypress tasks.
 */

import { resetDatabase, seedDatabase, getEnvironmentInfo } from '../playwright.config';

/**
 * Reset the database by truncating all tables
 * Use this in beforeEach hooks to ensure a clean state
 */
export async function resetDB() {
	await resetDatabase();
}

/**
 * Seed the database with test data
 * Use this after resetDB to populate initial test data
 */
export async function seedDB() {
	await seedDatabase();
}

/**
 * Get environment configuration info for debugging
 */
export function getEnvInfo() {
	return getEnvironmentInfo();
}

/**
 * Example usage in tests:
 *
 * import { test, expect } from '@playwright/test';
 * import { resetDB, seedDB } from '../fixtures/database';
 *
 * test.beforeEach(async () => {
 *   await resetDB();
 *   await seedDB();
 * });
 *
 * test('should display users', async ({ page }) => {
 *   await page.goto('/users');
 *   // ... test code
 * });
 */
