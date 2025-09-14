/// <reference types="cypress" />

describe("Payment Edit Form with Attachments", () => {
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

	it("should create a payment and then edit it with attachment upload", () => {
		cy.login("test@gmail.com", "TestPass123!");

		// Navigate to payments and create a new payment
		cy.visit("/finance/payments");
		cy.url().should("include", "/finance/payments");

		// Click new payment button
		cy.get('a[href="/finance/payments/new"]').filter(":visible").first().click();

		// Fill out the new payment form
		cy.get("[name=Amount]").type("1000.00");
		cy.get("[name=Date]").type("2024-01-15");
		cy.get("[name=AccountingPeriod]").type("2024-01-01");

		// Select account (assuming first option works)
		cy.get("[name=AccountID]").select(1);

		// Select payment category
		cy.get("[name=PaymentCategoryID]").select(1);

		// Add comment
		cy.get("[name=Comment]").type("Test payment for attachment upload");

		// Save the payment
		cy.get("#save-btn").click();

		// Should redirect to payments list
		cy.url().should("include", "/finance/payments");

		// Find and click on the newly created payment to edit it
		cy.get("tbody tr").contains("td", "1000.00").parent("tr").find("td a").first().click();

		// Should be on the edit page
		cy.url().should("include", "/finance/payments/");
		cy.url().should("not.include", "/new");

		// Verify form is populated with existing data
		cy.get("[name=Amount]").should("have.value", "1000.00");
		cy.get("[name=Comment]").should("contain.value", "Test payment for attachment upload");
	});

	it("should upload an attachment and submit it with the form", () => {
		cy.login("test@gmail.com", "TestPass123!");

		// Navigate to payments and find the existing payment
		cy.visit("/finance/payments");
		cy.get("tbody tr").contains("td", "1000.00").parent("tr").find("td a").first().click();

		// Create a test file for upload
		const fileName = "test-receipt.txt";
		const fileContent = "This is a test receipt file for payment attachment.";

		// Upload an attachment
		cy.get('input[type="file"]').selectFile({
			contents: Cypress.Buffer.from(fileContent),
			fileName: fileName,
			mimeType: "text/plain",
		}, { force: true });

		// Wait for file upload to complete
		cy.get('input[type="hidden"][name="Attachments"]', { timeout: 10000 }).should("exist");

		// Modify the payment comment to verify the form submission includes both data and attachments
		cy.get("[name=Comment]").clear().type("Updated payment with attachment");

		// Submit the form
		cy.get("#save-btn").click();

		// Check that the form was submitted successfully and redirected
		cy.url().should("include", "/finance/payments");

		// Go back to the payment to verify the attachment was saved
		cy.get("tbody tr").contains("td", "1000.00").parent("tr").find("td a").first().click();

		// Verify the comment was updated (indicates form submission worked)
		cy.get("[name=Comment]").should("contain.value", "Updated payment with attachment");

		// Verify the attachment is displayed in the existing files section
		cy.get(".attachment-list").should("exist");
		cy.get(".attachment-list").should("contain.text", fileName);
	});

	it("should handle multiple attachments correctly", () => {
		cy.login("test@gmail.com", "TestPass123!");

		// Navigate to payments and find the existing payment
		cy.visit("/finance/payments");
		cy.get("tbody tr").contains("td", "1000.00").parent("tr").find("td a").first().click();

		// Create multiple test files
		const files = [
			{
				fileName: "receipt1.txt",
				content: "First receipt file",
				mimeType: "text/plain"
			},
			{
				fileName: "receipt2.pdf",
				content: "PDF receipt content",
				mimeType: "application/pdf"
			}
		];

		// Upload multiple files
		files.forEach((file, index) => {
			cy.get('input[type="file"]').selectFile({
				contents: Cypress.Buffer.from(file.content),
				fileName: file.fileName,
				mimeType: file.mimeType,
			}, { force: true });

			// Wait for each upload to complete
			cy.get(`input[type="hidden"][name="Attachments"]`, { timeout: 10000 })
				.should("have.length.at.least", index + 1);
		});

		// Modify payment to test form submission
		cy.get("[name=Comment]").clear().type("Payment with multiple attachments");

		// Submit the form
		cy.get("#save-btn").click();

		// Verify successful submission
		cy.url().should("include", "/finance/payments");

		// Verify attachments were saved
		cy.get("tbody tr").contains("td", "1000.00").parent("tr").find("td a").first().click();
		cy.get("[name=Comment]").should("contain.value", "Payment with multiple attachments");

		// Check that multiple attachments are listed
		files.forEach(file => {
			cy.get(".attachment-list").should("contain.text", file.fileName);
		});
	});

	it("should allow deleting attachments", () => {
		cy.login("test@gmail.com", "TestPass123!");

		// Navigate to the payment with attachments
		cy.visit("/finance/payments");
		cy.get("tbody tr").contains("td", "1000.00").parent("tr").find("td a").first().click();

		// Verify attachments exist
		cy.get(".attachment-list").should("exist");
		cy.get(".attachment-list .flex.items-center").should("have.length.at.least", 1);

		// Get initial count of attachments
		cy.get(".attachment-list .flex.items-center").then(($attachments) => {
			const initialCount = $attachments.length;

			// Delete the first attachment
			cy.get(".attachment-list .flex.items-center").first().find("button").click();

			// Confirm deletion in the dialog
			cy.get("[data-cy=confirm]").click();

			// Verify one less attachment
			cy.get(".attachment-list .flex.items-center").should("have.length", initialCount - 1);
		});
	});

	it("should preserve form data when upload fails", () => {
		cy.login("test@gmail.com", "TestPass123!");

		// Navigate to payment edit
		cy.visit("/finance/payments");
		cy.get("tbody tr").contains("td", "1000.00").parent("tr").find("td a").first().click();

		// Fill out some form data
		cy.get("[name=Comment]").clear().type("Important payment data to preserve");
		cy.get("[name=Amount]").clear().type("2500.00");

		// Try to upload a very large file that might fail
		const largeContent = "x".repeat(50 * 1024 * 1024); // 50MB file
		cy.get('input[type="file"]').selectFile({
			contents: Cypress.Buffer.from(largeContent),
			fileName: "huge-file.txt",
			mimeType: "text/plain",
		}, { force: true });

		// Submit the form - might fail due to file size or other constraints
		cy.get("#save-btn").click();

		// If there's an error, verify that form data is preserved
		cy.get("[name=Comment]").should("contain.value", "Important payment data to preserve");
		cy.get("[name=Amount]").should("have.value", "2500.00");
	});
});