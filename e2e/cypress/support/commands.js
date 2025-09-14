Cypress.on("uncaught:exception", (err) => !err.message.includes("ResizeObserver loop"));

Cypress.Commands.add("login", (email, password) => {
	cy.session([email, password], () => {
		cy.visit("/login");
		cy.get("[type=email]").type(email);
		cy.get("[type=password]").type(password);
		cy.get("[type=submit]").click();
		cy.url().should("not.include", "/login");
	});
});

Cypress.Commands.add("logout", () => {
	cy.visit("/logout");
});
