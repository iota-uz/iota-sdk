/// <reference types="cypress" />

describe("user auth and registration flow", () => {
	before(() => {
		cy.task("resetDatabase");
		cy.task("seedDatabase");
	});

	afterEach(() => {
		cy.logout();
	});

	it("creates a user and displays changes in users table", () => {
		cy.login("test@gmail.com", "TestPass123!");

		cy.visit("http://localhost:3200/users");
		cy.url().should("eq", "http://localhost:3200/users");

		cy.get('a[href="/users/new"]').filter(":visible").click();
		cy.get("[name=FirstName]").type("Test");
		cy.get("[name=LastName]").type("User");
		cy.get("[name=MiddleName]").type("Mid");
		cy.get("[name=Email]").type("test1@gmail.com");
		cy.get("[name=Phone]").type("+14155551234");
		cy.get("[name=Password]").type("TestPass123!");
		cy.get("[name=Language]").select(2);
		cy.get('select[name="RoleIDs"]')
			.closest('div') // this div directly wraps the select
			.find('button[x-ref="trigger"]')
			.click();
		cy.get("ul[x-ref=list]").should("be.visible");
		cy.get("ul[x-ref=list]").find("li").first().click();
		cy.get("[id=save-btn]").click();
		cy.get("tbody tr").should("have.length", 4); // including the spinner row
		cy.logout();

		cy.login("test1@gmail.com", "TestPass123!");
		cy.visit("http://localhost:3200/users");

		cy.url().should("include", "/users");
		cy.get("tbody tr").should("have.length", 4); // including the spinner row
	});

	it("edits a user and displays changes in users table", () => {
		cy.login("test1@gmail.com", "TestPass123!");

		cy.visit("http://localhost:3200/users");
		cy.url().should("eq", "http://localhost:3200/users");

		cy.get("tbody tr").contains("td", "Test User").parent("tr").find("td a").click();
		cy.url().should("include", "/users/");
		cy.get("[name=FirstName]").clear().type("TestNew");
		cy.get("[name=LastName]").clear().type("UserNew");
		cy.get("[name=MiddleName]").clear().type("MidNew");
		cy.get("[name=Email]").clear().type("test1new@gmail.com");
		cy.get("[name=Phone]").clear().type("+14155559876");
		cy.get("[name=Language]").select(1);
		cy.get("[id=save-btn]").click();

		cy.visit("http://localhost:3200/users");
		cy.get("tbody tr").should("have.length", 4); // including the spinner row
		cy.get("tbody tr").should("contain.text", "TestNew UserNew");

		// Verify phone number persists by checking the edit page
		cy.get("tbody tr").contains("td", "TestNew UserNew").parent("tr").find("td a").click();
		cy.url().should("include", "/users/");
		cy.get("[name=Phone]").should("have.value", "14155559876");

		cy.logout();
		cy.login("test1new@gmail.com", "TestPass123!");
		cy.visit("http://localhost:3200/users");
		cy.url().should("include", "/users");
	});
});

