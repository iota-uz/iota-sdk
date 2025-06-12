/// <reference types="cypress" />

describe("employees CRUD operations", () => {
	before(() => {
		cy.task("resetDatabase");
		cy.task("seedDatabase");
	});

	beforeEach(() => {
		cy.viewport(1280, 720);
	});

	afterEach(() => {
		cy.logout();
	});

	it("displays employees list page", () => {
		cy.login("test@gmail.com", "TestPass123!");

		cy.visit("http://localhost:3200/hrm/employees");
		cy.url().should("eq", "http://localhost:3200/hrm/employees");

		// Check page title and main elements
		cy.get("h1").should("contain.text", "Employees");
		cy.get('a[href="/hrm/employees/new"]').should("be.visible");
		
		// Check search and filter form
		cy.get('form input[name="name"]').should("be.visible");
		cy.get('form select[name="limit"]').should("be.visible");
	});

	it("creates a new employee and displays in employees table", () => {
		cy.login("test@gmail.com", "TestPass123!");

		cy.visit("http://localhost:3200/hrm/employees");
		cy.url().should("eq", "http://localhost:3200/hrm/employees");

		// Click new employee button
		cy.get('a[href="/hrm/employees/new"]').filter(":visible").click();
		cy.url().should("include", "/hrm/employees/new");

		// Fill out the form - Public tab
		cy.get("[name=FirstName]").type("John");
		cy.get("[name=LastName]").type("Doe");
		cy.get("[name=MiddleName]").type("William");
		cy.get("[name=Email]").type("john.doe@company.com");
		cy.get("[name=Phone]").type("+1234567890");
		cy.get("[name=BirthDate]").type("1990-01-15");
		cy.get("[name=HireDate]").type("2024-01-01");

		// Switch to Private tab
		cy.get('button[aria-controls="private"]').click();
		cy.get("[name=Salary]").type("50000");
		cy.get("[name=Tin]").type("123456789");

		// Submit form
		cy.get("[id=save-btn]").click();

		// Should redirect to employees list
		cy.url().should("eq", "http://localhost:3200/hrm/employees");
		
		// Verify employee appears in table
		cy.get("tbody tr").should("contain.text", "John Doe");
		cy.get("tbody tr").should("contain.text", "john.doe@company.com");
	});

	it("edits an existing employee and displays changes", () => {
		cy.login("test@gmail.com", "TestPass123!");

		cy.visit("http://localhost:3200/hrm/employees");
		cy.url().should("eq", "http://localhost:3200/hrm/employees");

		// Click edit button for John Doe
		cy.get("tbody tr").contains("td", "John Doe").parent("tr").find("td a").click();
		cy.url().should("include", "/hrm/employees/");

		// Edit the employee
		cy.get("[name=FirstName]").clear().type("Johnny");
		cy.get("[name=LastName]").clear().type("Smith");
		cy.get("[name=Email]").clear().type("johnny.smith@company.com");
		cy.get("[name=Phone]").clear().type("+9876543210");

		// Switch to Private tab and edit salary
		cy.get('button[aria-controls="private"]').click();
		cy.get("[name=Salary]").clear().type("60000");

		// Save changes
		cy.get("[id=save-btn]").click();

		// Should redirect to employees list
		cy.url().should("eq", "http://localhost:3200/hrm/employees");

		// Verify changes appear in table
		cy.get("tbody tr").should("contain.text", "Johnny Smith");
		cy.get("tbody tr").should("contain.text", "johnny.smith@company.com");
		cy.get("tbody tr").should("not.contain.text", "John Doe");

		// Verify changes persist by checking edit page
		cy.get("tbody tr").contains("td", "Johnny Smith").parent("tr").find("td a").click();
		cy.get("[name=FirstName]").should("have.value", "Johnny");
		cy.get("[name=LastName]").should("have.value", "Smith");
		cy.get("[name=Email]").should("have.value", "johnny.smith@company.com");
		cy.get("[name=Phone]").should("have.value", "9876543210");
		
		// Check private tab
		cy.get('button[aria-controls="private"]').click();
		cy.get("[name=Salary]").should("have.value", "60000");
	});

	it("handles form validation errors", () => {
		cy.login("test@gmail.com", "TestPass123!");

		cy.visit("http://localhost:3200/hrm/employees/new");

		// Try to submit empty form
		cy.get("[id=save-btn]").click();

		// Should show validation errors
		cy.get(".error").should("be.visible");
		cy.url().should("include", "/hrm/employees/new");

		// Fill required fields
		cy.get("[name=FirstName]").type("Jane");
		cy.get("[name=LastName]").type("Doe");
		cy.get("[name=Email]").type("invalid-email");

		// Submit with invalid email
		cy.get("[id=save-btn]").click();

		// Should show email validation error
		cy.get(".error").should("be.visible");
		cy.url().should("include", "/hrm/employees/new");

		// Fix email and submit
		cy.get("[name=Email]").clear().type("jane.doe@company.com");
		cy.get("[id=save-btn]").click();

		// Should succeed and redirect
		cy.url().should("eq", "http://localhost:3200/hrm/employees");
		cy.get("tbody tr").should("contain.text", "Jane Doe");
	});

	it("deletes an employee", () => {
		cy.login("test@gmail.com", "TestPass123!");

		cy.visit("http://localhost:3200/hrm/employees");
		
		// Click edit button for Jane Doe
		cy.get("tbody tr").contains("td", "Jane Doe").parent("tr").find("td a").click();
		cy.url().should("include", "/hrm/employees/");

		// Click delete button
		cy.get("[id=delete-employee-btn]").click();

		// Confirm deletion in dialog
		cy.get("dialog").should("be.visible");
		cy.get("dialog button").contains("Delete").click();

		// Should redirect to employees list
		cy.url().should("eq", "http://localhost:3200/hrm/employees");

		// Verify employee is removed from table
		cy.get("tbody tr").should("not.contain.text", "Jane Doe");
	});

	it("searches employees by name", () => {
		cy.login("test@gmail.com", "TestPass123!");

		cy.visit("http://localhost:3200/hrm/employees");

		// Type in search box
		cy.get('input[name="name"]').type("Johnny");

		// Wait for HTMX request to complete
		cy.wait(1000);

		// Should show only matching employees
		cy.get("tbody tr").should("contain.text", "Johnny Smith");
		cy.get("tbody tr").should("not.contain.text", "John Doe");

		// Clear search
		cy.get('input[name="name"]').clear();
		cy.wait(1000);

		// Should show all employees again
		cy.get("tbody tr").should("have.length.at.least", 1);
	});

	it("filters employees by page limit", () => {
		cy.login("test@gmail.com", "TestPass123!");

		cy.visit("http://localhost:3200/hrm/employees");

		// Change page limit
		cy.get('select[name="limit"]').select("15");

		// Wait for HTMX request to complete
		cy.wait(1000);

		// Should update the table (exact assertions depend on test data)
		cy.get("tbody tr").should("exist");
	});

	it("handles TIN validation errors", () => {
		cy.login("test@gmail.com", "TestPass123!");

		cy.visit("http://localhost:3200/hrm/employees/new");

		// Fill basic info
		cy.get("[name=FirstName]").type("Test");
		cy.get("[name=LastName]").type("Employee");
		cy.get("[name=Email]").type("test.employee@company.com");

		// Switch to Private tab and enter invalid TIN
		cy.get('button[aria-controls="private"]').click();
		cy.get("[name=Tin]").type("invalid-tin");

		// Submit form
		cy.get("[id=save-btn]").click();

		// Should show TIN validation error
		cy.get(".error").should("be.visible");
		cy.url().should("include", "/hrm/employees/new");
	});

	it("navigates between public and private tabs", () => {
		cy.login("test@gmail.com", "TestPass123!");

		cy.visit("http://localhost:3200/hrm/employees/new");

		// Should start on public tab
		cy.get('button[aria-controls="public"]').should("have.attr", "aria-selected", "true");
		cy.get("[name=FirstName]").should("be.visible");

		// Switch to private tab
		cy.get('button[aria-controls="private"]').click();
		cy.get('button[aria-controls="private"]').should("have.attr", "aria-selected", "true");
		cy.get("[name=Salary]").should("be.visible");

		// Switch back to public tab
		cy.get('button[aria-controls="public"]').click();
		cy.get('button[aria-controls="public"]').should("have.attr", "aria-selected", "true");
		cy.get("[name=FirstName]").should("be.visible");
	});
});