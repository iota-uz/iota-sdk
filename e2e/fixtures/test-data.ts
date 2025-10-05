/**
 * Test data management fixtures for Playwright tests
 *
 * Migrated from Cypress custom commands (cypress/support/test-data-commands.js)
 * These interact with the testkit module endpoints
 */

import { APIRequestContext } from '@playwright/test';

/**
 * Options for database reset
 */
interface ResetOptions {
	reseedMinimal?: boolean;
}

/**
 * Reset the test database
 * Migrates Cypress.Commands.add("resetTestDatabase")
 *
 * @param request - Playwright API request context
 * @param options - Reset options
 */
export async function resetTestDatabase(
	request: APIRequestContext,
	options: ResetOptions = { reseedMinimal: true }
) {
	const response = await request.post('/__test__/reset', {
		data: options,
		failOnStatusCode: false,
	});

	if (!response.ok()) {
		const body = await response.json().catch(() => ({ error: 'Unknown error' }));
		throw new Error(`Database reset failed: ${body.error || response.statusText()}`);
	}

	const body = await response.json();
	console.log('Database reset successfully', body);
	return body;
}

/**
 * Populate test data using JSON specification
 * Migrates Cypress.Commands.add("populateTestData")
 *
 * @param request - Playwright API request context
 * @param dataSpec - The data specification object
 */
export async function populateTestData(
	request: APIRequestContext,
	dataSpec: any
) {
	const response = await request.post('/__test__/populate', {
		data: dataSpec,
		failOnStatusCode: false,
	});

	if (!response.ok()) {
		const body = await response.json().catch(() => ({ error: 'Unknown error' }));
		throw new Error(`Data population failed: ${body.error || response.statusText()}`);
	}

	const body = await response.json();
	console.log('Test data populated successfully', body);
	return body.data;
}

/**
 * Seed a predefined scenario
 * Migrates Cypress.Commands.add("seedScenario")
 *
 * @param request - Playwright API request context
 * @param scenarioName - Name of the scenario to seed (default: 'minimal')
 */
export async function seedScenario(
	request: APIRequestContext,
	scenarioName: string = 'minimal'
) {
	const response = await request.post('/__test__/seed', {
		data: { scenario: scenarioName },
		failOnStatusCode: false,
	});

	if (!response.ok()) {
		const body = await response.json().catch(() => ({ error: 'Unknown error' }));
		throw new Error(`Scenario seeding failed: ${body.error || response.statusText()}`);
	}

	const body = await response.json();
	console.log(`Scenario '${scenarioName}' seeded successfully`, body);
	return body;
}

/**
 * Get list of available scenarios
 * Migrates Cypress.Commands.add("getAvailableScenarios")
 *
 * @param request - Playwright API request context
 */
export async function getAvailableScenarios(request: APIRequestContext) {
	const response = await request.get('/__test__/seed', {
		failOnStatusCode: false,
	});

	if (!response.ok()) {
		throw new Error(`Failed to get scenarios: ${response.statusText()}`);
	}

	const body = await response.json();
	return body.scenarios;
}

/**
 * Check test endpoints health
 * Migrates Cypress.Commands.add("checkTestEndpointsHealth")
 *
 * @param request - Playwright API request context
 */
export async function checkTestEndpointsHealth(request: APIRequestContext) {
	const response = await request.get('/__test__/health', {
		failOnStatusCode: false,
	});

	if (!response.ok()) {
		throw new Error(`Test endpoints health check failed: ${response.statusText()}`);
	}

	return await response.json();
}

/**
 * Data builder helpers for common test scenarios
 * Migrated from TestDataBuilders export
 */
export const TestDataBuilders = {
	/**
	 * Create a minimal user specification
	 */
	createUser: (overrides: any = {}) => ({
		email: 'test@example.com',
		password: 'TestPass123!',
		firstName: 'Test',
		lastName: 'User',
		language: 'en',
		...overrides,
	}),

	/**
	 * Create a money account specification
	 */
	createMoneyAccount: (overrides: any = {}) => ({
		name: 'Test Account',
		currency: 'USD',
		balance: 1000.0,
		type: 'cash',
		...overrides,
	}),

	/**
	 * Create a payment specification
	 */
	createPayment: (overrides: any = {}) => ({
		amount: 100.0,
		date: new Date().toISOString().split('T')[0],
		accountRef: '@moneyAccounts.testAccount',
		categoryRef: '@paymentCategories.testCategory',
		comment: 'Test payment',
		...overrides,
	}),

	/**
	 * Create a complete populate request with basic financial data
	 */
	createFinanceScenario: (overrides: any = {}) => ({
		version: '1.0',
		tenant: {
			id: '00000000-0000-0000-0000-000000000001',
			name: 'Test Tenant',
			domain: 'test.localhost',
		},
		data: {
			users: [TestDataBuilders.createUser({ _ref: 'testUser' })],
			finance: {
				moneyAccounts: [TestDataBuilders.createMoneyAccount({ _ref: 'testAccount' })],
				paymentCategories: [
					{
						name: 'Test Category',
						type: 'income',
						_ref: 'testCategory',
					},
				],
				payments: [TestDataBuilders.createPayment()],
			},
		},
		options: {
			clearExisting: false,
			returnIds: true,
			validateReferences: true,
			stopOnError: true,
		},
		...overrides,
	}),
};
