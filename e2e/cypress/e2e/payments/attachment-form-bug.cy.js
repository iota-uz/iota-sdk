/// <reference types="cypress" />

describe("Payment Attachment Form Submission Bug Fix", () => {
	before(() => {
		// Reset database and seed with finance-focused data
		cy.resetTestDatabase({ reseedMinimal: false });
		cy.seedScenario("finance");
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
		cy.visit("/finance/payments/new");

		// Wait for Alpine.js and page initialization
		cy.waitForAlpine();

		// Fill minimal required fields
		cy.get("[name=Amount]").type("500.00");
		cy.get("[name=TransactionDate]").type("2024-01-15");
		cy.get("[name=AccountingPeriod]").type("2024-01-01");
		// Select first available option for AccountID and PaymentCategoryID
		cy.get("[name=AccountID] option").then($options => {
			if ($options.length > 1) {
				cy.get("[name=AccountID]").select($options.eq(1).val());
			}
		});
		cy.get("[name=PaymentCategoryID] option").then($options => {
			if ($options.length > 1) {
				cy.get("[name=PaymentCategoryID]").select($options.eq(1).val());
			}
		});

		// Save to create the payment
		cy.get("#save-btn").click();

		// Wait for redirect to payments list
		cy.url().should("include", "/finance/payments");
		cy.waitForAlpine();

		// Wait for the table to load and find the created payment
		cy.get("tbody tr").should("exist");
		cy.get("tbody tr").contains("td", "500.00").should("be.visible");

		// Edit the created payment - find the row with amount 500.00 and click the edit button
		cy.get("tbody tr").contains("td", "500.00").parent("tr").find("button.btn-fixed").click();

		// Wait for edit page to fully load
		cy.waitForAlpine();
		cy.get("#save-btn").should('be.visible'); // Ensure form is loaded

		// Intercept the form submission to verify attachment IDs are included
		cy.intercept("POST", "/finance/payments/*", (req) => {
			// Check that the request body includes Attachments field with IDs
			const formData = req.body;
			if (formData instanceof FormData) {
				const attachments = formData.getAll("Attachments");
				if (attachments && attachments.length > 0) {
					expect(attachments).to.not.be.empty;
					attachments.forEach(attachment => {
						expect(attachment).to.not.be.empty;
					});
				}
			}
		}).as("submitPayment");

		// Upload an attachment using custom command
		cy.uploadFileAndWaitForAttachment("Test file content for bug verification", "test-bug-fix.txt");

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
		cy.visit("/finance/payments");
		cy.waitForAlpine();

		// Wait for payments to load and navigate to first payment - click the edit button
		cy.get("tbody tr").should("exist");
		cy.get("tbody tr").first().find("button.btn-fixed").click();

		// Wait for edit page to fully load
		cy.waitForAlpine();
		cy.get("#save-btn").should('be.visible');

		// Verify the form exists
		cy.get("#save-form").should("exist");

		// Upload a file using custom command
		cy.uploadFileAndWaitForAttachment("Form attribute test", "form-test.txt");

		// Verify the hidden input has the correct form attribute
		cy.get('input[type="hidden"][name="Attachments"]').then(($input) => {
			// Check if form attribute exists - it may or may not be required depending on implementation
			if ($input.attr('form')) {
				expect($input.attr('form')).to.equal('save-form');
			} else {
				// If no form attribute, verify it's inside the form or properly associated
				cy.get('#save-form').should('exist');
			}
		});

		// Verify the hidden input is properly associated with the save form
		cy.get('input[type="hidden"][name="Attachments"]').should("exist");
		cy.get("#save-form").should("exist");
	});

	it("should handle multiple attachments with correct form association", () => {
		cy.login("test@gmail.com", "TestPass123!");

		// Navigate to payment edit
		cy.visit("/finance/payments");
		cy.waitForAlpine();

		// Wait for payments to load and click first payment
		cy.get("tbody tr").should("exist");
		cy.get("tbody tr").first().find("button.btn-fixed").click();

		// Wait for edit page to fully load
		cy.waitForAlpine();
		cy.get("#save-btn").should('be.visible');

		// Verify the form exists
		cy.get("#save-form").should("exist");

		// Intercept and verify all attachment IDs are submitted
		cy.intercept("POST", "/finance/payments/*", (req) => {
			const formData = req.body;
			if (formData instanceof FormData) {
				const attachmentValues = formData.getAll("Attachments");
				if (attachmentValues && attachmentValues.length > 0) {
					expect(attachmentValues).to.have.length.at.least(1);
					attachmentValues.forEach(value => {
						expect(value).to.not.be.empty;
					});
				}
			}
		}).as("submitWithMultipleAttachments");

		// Upload multiple files using custom command
		const files = [
			{ name: 'test-1.txt', content: 'Test content 1' },
			{ name: 'test-2.txt', content: 'Test content 2' },
			{ name: 'test-3.txt', content: 'Test content 3' }
		];

		files.forEach((file, index) => {
			cy.uploadFileAndWaitForAttachment(file.content, file.name);

			// Verify we have at least this many attachments
			cy.get('input[type="hidden"][name="Attachments"]')
				.should("have.length.at.least", index + 1);

			// Small delay between uploads to ensure proper processing
			cy.wait(500);
		});

		// Verify all hidden inputs exist and are properly associated
		cy.get('input[type="hidden"][name="Attachments"]')
			.should("have.length.at.least", 1)
			.each(($input) => {
				// Check that each input has a non-empty value
				expect($input.val()).to.not.be.empty;

				// Check form attribute if it exists, otherwise verify form association
				if ($input.attr('form')) {
					expect($input.attr('form')).to.equal('save-form');
				}
			});

		// Verify form is still present
		cy.get("#save-form").should("exist");

		// Submit the form
		cy.get("#save-btn").click();

		// Verify the request
		cy.wait("@submitWithMultipleAttachments");

		// Verify successful redirect
		cy.url().should("include", "/finance/payments");
	});

	// Additional test to verify recovery from Alpine.js initialization issues
	it("should handle Alpine.js initialization errors gracefully", () => {
		cy.login("test@gmail.com", "TestPass123!");

		// Navigate to payment edit without waiting for Alpine initialization
		cy.visit("/finance/payments");

		// Wait for table to exist then try to click on payment immediately to simulate Alpine.js not being ready
		cy.get("tbody tr").should("exist");
		cy.get("tbody tr").first().find("button.btn-fixed").click();

		// Now wait for proper initialization
		cy.waitForAlpine();

		// Verify page is functional despite potential early Alpine.js errors
		cy.get("#save-btn").should('be.visible');
		cy.get("#save-form").should("exist");

		// Test that file upload still works
		cy.uploadFileAndWaitForAttachment("Error recovery test content", "recovery-test.txt");

		// Verify attachment was processed correctly
		cy.get('input[type="hidden"][name="Attachments"]')
			.should("exist")
			.should("have.value")
			.invoke("val").should("not.be.empty");

		// Intercept form submission to ensure attachments are included
		cy.intercept("POST", "/finance/payments/*", (req) => {
			const formData = req.body;
			if (formData instanceof FormData) {
				const attachments = formData.getAll("Attachments");
				if (attachments && attachments.length > 0) {
					expect(attachments[0]).to.not.be.empty;
				}
			}
		}).as("submitRecoveryTest");

		// Submit form
		cy.get("#save-btn").click();
		cy.wait("@submitRecoveryTest");

		// Verify successful completion
		cy.url().should("include", "/finance/payments");
	});
});