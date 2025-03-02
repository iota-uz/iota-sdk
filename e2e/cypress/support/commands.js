Cypress.on("uncaught:exception", (err) => !err.message.includes("ResizeObserver loop"));

Cypress.Commands.add("login", (email, password) => {
	cy.session([email, password], () => {
		cy.visit("http://localhost:3200/login");
		cy.get("[type=email]").type(email);
		cy.get("[type=password]").type(password);
		cy.get("[type=submit]").click();
	});
});

Cypress.Commands.add("logout", () => {
	cy.visit("http://localhost:3200/logout");
});
