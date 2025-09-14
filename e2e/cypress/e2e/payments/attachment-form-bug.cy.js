/// <reference types="cypress" />

describe("Payment Attachment Form Submission Bug Fix", () => {
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

	it("should include attachment IDs in form submission", () => {
		cy.login("test@gmail.com", "TestPass123!");

		// Navigate to payments and create a new payment first
		cy.visit("http://localhost:3200/finance/payments/new");

		// Fill minimal required fields
		cy.get("[name=Amount]").type("500.00");
		cy.get("[name=Date]").type("2024-01-15");
		cy.get("[name=AccountingPeriod]").type("2024-01-01");
		cy.get("[name=AccountID]").select(1);
		cy.get("[name=PaymentCategoryID]").select(1);

		// Save to create the payment
		cy.get("#save-btn").click();
		cy.url().should("include", "/finance/payments");

		// Edit the created payment
		cy.get("tbody tr").contains("td", "500.00").parent("tr").find("td a").first().click();

		// Upload an attachment
		cy.get('input[type="file"]').selectFile({
			contents: Cypress.Buffer.from("Test file content for bug verification"),
			fileName: "test-bug-fix.txt",
			mimeType: "text/plain",
		}, { force: true });

		// Wait for upload to complete and verify hidden input exists
		cy.get('input[type="hidden"][name="Attachments"]', { timeout: 10000 }).should("exist");

		// Intercept the form submission to verify attachment IDs are included
		cy.intercept("POST", "/finance/payments/*", (req) => {
			// Check that the request body includes Attachments field with IDs
			expect(req.body).to.include("Attachments");
			expect(req.body.get("Attachments")).to.not.be.empty;
		}).as("submitPayment");

		// Submit the form
		cy.get("#save-btn").click();

		// Verify the intercepted request was made
		cy.wait("@submitPayment");

		// Verify successful redirect
		cy.url().should("include", "/finance/payments");
	});

	it("should verify hidden inputs have correct form attribute", () => {
		cy.login("test@gmail.com", "TestPass123!");

		// Navigate to an existing payment edit page
		cy.visit("http://localhost:3200/finance/payments");
		cy.get("tbody tr").first().find("td a").first().click();

		// Upload a file
		cy.get('input[type="file"]').selectFile({
			contents: Cypress.Buffer.from("Form attribute test"),
			fileName: "form-test.txt",
			mimeType: "text/plain",
		}, { force: true });

		// Verify the hidden input has the correct form attribute
		cy.get('input[type="hidden"][name="Attachments"]', { timeout: 10000 })
			.should("exist")
			.should("have.attr", "form", "save-form");

		// Verify the hidden input is outside the form but associated with it
		cy.get("#save-form").should("exist");
		cy.get('input[type="hidden"][name="Attachments"][form="save-form"]').should("exist");
	});

	it("should handle multiple attachments with correct form association", () => {
		cy.login("test@gmail.com", "TestPass123!");

		// Navigate to payment edit
		cy.visit("http://localhost:3200/finance/payments");
		cy.get("tbody tr").first().find("td a").first().click();

		// Upload multiple files
		for (let i = 1; i <= 3; i++) {
			cy.get('input[type="file"]').selectFile({
				contents: Cypress.Buffer.from(`Test content ${i}`),
				fileName: `test-${i}.txt`,
				mimeType: "text/plain",
			}, { force: true });

			// Wait for this upload to be processed
			cy.get('input[type="hidden"][name="Attachments"]', { timeout: 10000 })
				.should("have.length.at.least", i);
		}

		// Verify all hidden inputs have the form attribute
		cy.get('input[type="hidden"][name="Attachments"]')
			.should("have.length", 3)
			.each(($input) => {
				cy.wrap($input).should("have.attr", "form", "save-form");
			});

		// Intercept and verify all attachment IDs are submitted
		cy.intercept("POST", "/finance/payments/*", (req) => {
			const attachmentValues = req.body.getAll("Attachments");
			expect(attachmentValues).to.have.length(3);
			attachmentValues.forEach(value => {
				expect(value).to.not.be.empty;
			});
		}).as("submitWithMultipleAttachments");

		// Submit the form
		cy.get("#save-btn").click();

		// Verify the request
		cy.wait("@submitWithMultipleAttachments");
	});
});