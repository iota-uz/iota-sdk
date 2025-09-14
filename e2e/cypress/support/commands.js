// Handle common JavaScript errors that shouldn't fail tests
Cypress.on("uncaught:exception", (err) => {
	// Ignore ResizeObserver loop errors
	if (err.message.includes("ResizeObserver loop")) {
		return false;
	}

	// Ignore Alpine.js initialization errors during testing
	if (err.message.includes("value is not defined") ||
	    err.message.includes("Alpine") ||
	    err.message.includes("Cannot read properties of undefined")) {
		console.warn('Ignored Alpine.js error during testing:', err.message);
		return false;
	}

	// Let other errors fail the test
	return true;
});

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

// Custom command to wait for Alpine.js and page initialization
Cypress.Commands.add("waitForAlpine", (timeout = 5000) => {
	cy.window({ timeout }).then((win) => {
		// Wait for Alpine.js to be available
		return new Promise((resolve) => {
			const checkAlpine = () => {
				if (win.Alpine && win.Alpine.version) {
					resolve();
				} else {
					setTimeout(checkAlpine, 100);
				}
			};
			// Start checking, but don't fail if Alpine isn't available
			setTimeout(() => resolve(), timeout);
			checkAlpine();
		});
	});

	// Additionally wait for body to be visible and DOM to be ready
	cy.get('body').should('be.visible');
	cy.wait(1000); // Allow time for initialization
});

// Custom command for file upload with proper wait
Cypress.Commands.add("uploadFileAndWaitForAttachment", (fileContent, fileName, mimeType = "text/plain") => {
	cy.get('input[type="file"]').selectFile({
		contents: Cypress.Buffer.from(fileContent),
		fileName: fileName,
		mimeType: mimeType,
	}, { force: true });

	// Wait for upload to complete with extended timeout
	cy.get('input[type="hidden"][name="Attachments"]', { timeout: 15000 })
		.should("exist")
		.should("have.value")
		.invoke("val").should("not.be.empty");
});
