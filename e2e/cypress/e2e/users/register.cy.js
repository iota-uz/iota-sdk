/// <reference types="cypress" />

Cypress.on("uncaught:exception", (err) => !err.message.includes("ResizeObserver loop"));

const login = (email, password) => {
	cy.session([email, password], () => {
		cy.visit("http://localhost:3200/login");
		cy.get("[type=email]").type(email);
		cy.get("[type=password]").type(password);
		cy.get("[type=submit]").click();
	});
};

describe("user auth and registration flow", () => {
	afterEach(() => {
		cy.visit("http://localhost:3200/logout");
	});

	it("creates a user and displays changes in users table", () => {
		login("test@gmail.com", "TestPass123!");
		cy.visit("http://localhost:3200/users");
		cy.get('a[href="/users/new"]').filter(':visible').click();
		cy.get("[name=FirstName]").type("Test");
		cy.get("[name=LastName]").type("User");
		cy.get("[name=MiddleName]").type("Mid");
		cy.get("[name=Email]").type("test1@gmail.com");
		cy.get("[name=Password]").type("TestPass123!");
		cy.get("[name=UILanguage]").select(2);
		cy.get("[x-ref=trigger]").click();
		cy.get("ul[x-ref=list]").should("be.visible");
		cy.get("ul[x-ref=list]")
			.find("li")
			.first()
			.click();
		cy.get("[id=save-btn]").click();
		cy.visit("http://localhost:3200/users");
		cy.get("tbody tr").should("have.length", 2);
		cy.visit("http://localhost:3200/logout");

		login("test1@gmail.com", "TestPass123!");
		cy.visit("http://localhost:3200/users");

		cy.url().should("include", "/users");
		cy.get("tbody tr").should("have.length", 2);
	});
});
