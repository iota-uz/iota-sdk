/// <reference types="cypress" />

describe("employees CRUD operations", () => {
	before(() => {
		// Reset database and seed with comprehensive data for employee management
		cy.resetTestDatabase({ reseedMinimal: false });
		cy.seedScenario("comprehensive");
	});

	beforeEach(() => {
		cy.viewport(1280, 720);
	});

	afterEach(() => {
		cy.logout();
	});

	it("displays employees list page", () => {
		cy.login("test@gmail.com", "TestPass123!");

		cy.visit("/hrm/employees");
		cy.url().should("eq", Cypress.config().baseUrl + "/hrm/employees");

		// Check page title and main elements
		cy.get("h1").should("contain.text", "Employees");
		cy.get('a[href="/hrm/employees/new"]').should("be.visible");

		// Check search and filter form
		cy.get('form input[name="name"]').should("be.visible");
		cy.get('form select[name="limit"]').should("be.visible");
	});
});
