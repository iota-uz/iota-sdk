// Test data management commands for IOTA SDK E2E tests
// These commands interact with the testkit module endpoints

/**
 * Reset the test database, optionally reseeding with minimal data
 * @param {Object} options - Reset options
 * @param {boolean} options.reseedMinimal - Whether to reseed minimal data after reset
 */
Cypress.Commands.add("resetTestDatabase", (options = {}) => {
	const defaultOptions = {
		reseedMinimal: true,
	};

	const requestOptions = { ...defaultOptions, ...options };

	return cy.request({
		method: 'POST',
		url: '/__test__/reset',
		body: requestOptions,
		failOnStatusCode: false
	}).then((response) => {
		if (response.status !== 200) {
			throw new Error(`Database reset failed: ${response.body.error || response.statusText}`);
		}
		cy.log('Database reset successfully', response.body);
		return response.body;
	});
});

/**
 * Populate test data using JSON specification
 * @param {Object} dataSpec - The data specification object
 */
Cypress.Commands.add("populateTestData", (dataSpec) => {
	return cy.request({
		method: 'POST',
		url: '/__test__/populate',
		body: dataSpec,
		failOnStatusCode: false
	}).then((response) => {
		if (response.status !== 200) {
			throw new Error(`Data population failed: ${response.body.error || response.statusText}`);
		}
		cy.log('Test data populated successfully', response.body);
		return response.body.data;
	});
});

/**
 * Seed a predefined scenario
 * @param {string} scenarioName - Name of the scenario to seed
 */
Cypress.Commands.add("seedScenario", (scenarioName = "minimal") => {
	return cy.request({
		method: 'POST',
		url: '/__test__/seed',
		body: { scenario: scenarioName },
		failOnStatusCode: false
	}).then((response) => {
		if (response.status !== 200) {
			throw new Error(`Scenario seeding failed: ${response.body.error || response.statusText}`);
		}
		cy.log(`Scenario '${scenarioName}' seeded successfully`, response.body);
		return response.body;
	});
});

/**
 * Get list of available scenarios
 */
Cypress.Commands.add("getAvailableScenarios", () => {
	return cy.request({
		method: 'GET',
		url: '/__test__/seed',
		failOnStatusCode: false
	}).then((response) => {
		if (response.status !== 200) {
			throw new Error(`Failed to get scenarios: ${response.statusText}`);
		}
		return response.body.scenarios;
	});
});

/**
 * Check test endpoints health
 */
Cypress.Commands.add("checkTestEndpointsHealth", () => {
	return cy.request({
		method: 'GET',
		url: '/__test__/health',
		failOnStatusCode: false
	}).then((response) => {
		if (response.status !== 200) {
			throw new Error(`Test endpoints health check failed: ${response.statusText}`);
		}
		return response.body;
	});
});

// Data builder helpers for common test scenarios
export const TestDataBuilders = {
	/**
	 * Create a minimal user specification
	 */
	createUser: (overrides = {}) => ({
		email: "test@example.com",
		password: "TestPass123!",
		firstName: "Test",
		lastName: "User",
		language: "en",
		...overrides
	}),

	/**
	 * Create a money account specification
	 */
	createMoneyAccount: (overrides = {}) => ({
		name: "Test Account",
		currency: "USD",
		balance: 1000.00,
		type: "cash",
		...overrides
	}),

	/**
	 * Create a payment specification
	 */
	createPayment: (overrides = {}) => ({
		amount: 100.00,
		date: new Date().toISOString().split('T')[0],
		accountRef: "@moneyAccounts.testAccount",
		categoryRef: "@paymentCategories.testCategory",
		comment: "Test payment",
		...overrides
	}),

	/**
	 * Create a complete populate request with basic financial data
	 */
	createFinanceScenario: (overrides = {}) => ({
		version: "1.0",
		tenant: {
			id: "00000000-0000-0000-0000-000000000001",
			name: "Test Tenant",
			domain: "test.localhost"
		},
		data: {
			users: [
				TestDataBuilders.createUser({ _ref: "testUser" })
			],
			finance: {
				moneyAccounts: [
					TestDataBuilders.createMoneyAccount({ _ref: "testAccount" })
				],
				paymentCategories: [
					{
						name: "Test Category",
						type: "income",
						_ref: "testCategory"
					}
				],
				payments: [
					TestDataBuilders.createPayment()
				]
			}
		},
		options: {
			clearExisting: false,
			returnIds: true,
			validateReferences: true,
			stopOnError: true
		},
		...overrides
	})
};