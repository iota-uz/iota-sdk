import { test, expect } from '@playwright/test';
import { login, logout } from '../../fixtures/auth';
import { resetTestDatabase, seedScenario } from '../../fixtures/test-data';
import path from 'path';
import fs from 'fs';

test.describe('expense attachments', () => {
	let expenseId: string;

	test.beforeAll(async ({ request }) => {
		// Reset database and seed with comprehensive data
		await resetTestDatabase(request, { reseedMinimal: false });
		await seedScenario(request, 'comprehensive');
	});

	test.beforeEach(async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 720 });
		await login(page, 'test@gmail.com', 'TestPass123!');

		// Create a new expense for testing
		await page.goto('/finance/expenses/new');
		await expect(page).toHaveURL(/\/finance\/expenses\/new$/);

		// Fill in expense form
		await page.locator('[name=Amount]').fill('150.00');
		await page.locator('[name=Date]').fill('2024-01-15');
		await page.locator('[name=AccountingPeriod]').fill('2024-01-01');
		await page.locator('[name=Comment]').fill('Test expense for attachments');

		// Select account (wait for lazy load)
		await page.waitForSelector('select[name="AccountID"]', { timeout: 5000 });
		await page.locator('select[name="AccountID"]').selectOption({ index: 1 });

		// Select category (wait for lazy load)
		await page.waitForSelector('select[name="CategoryID"]', { timeout: 5000 });
		await page.locator('select[name="CategoryID"]').selectOption({ index: 1 });

		// Save and capture the expense ID from URL
		await Promise.all([
			page.waitForURL(/\/finance\/expenses$/),
			page.locator('[id=save-btn]').click()
		]);

		// Navigate to expenses list and get the first expense
		await page.goto('/finance/expenses');
		const firstExpenseLink = page.locator('tbody tr').first().locator('a').first();
		await firstExpenseLink.click();

		// Extract expense ID from URL
		await page.waitForURL(/\/finance\/expenses\/[0-9a-fA-F-]+$/);
		const url = page.url();
		expenseId = url.split('/').pop() || '';
		expect(expenseId).toBeTruthy();
	});

	test.afterEach(async ({ page }) => {
		await logout(page);
	});

	test('uploads a PDF attachment to an expense', async ({ page }) => {
		// Create a test PDF file
		const testFilePath = path.join(__dirname, '../../fixtures/test-files/test-document.pdf');
		const testFileDir = path.dirname(testFilePath);

		if (!fs.existsSync(testFileDir)) {
			fs.mkdirSync(testFileDir, { recursive: true });
		}

		if (!fs.existsSync(testFilePath)) {
			// Create a simple PDF file for testing
			const pdfContent = '%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj 2 0 obj<</Type/Pages/Count 1/Kids[3 0 R]>>endobj 3 0 obj<</Type/Page/MediaBox[0 0 612 792]/Parent 2 0 R/Resources<<>>>>endobj\nxref\n0 4\n0000000000 65535 f\n0000000009 00000 n\n0000000056 00000 n\n0000000115 00000 n\ntrailer<</Size 4/Root 1 0 R>>\nstartxref\n210\n%%EOF';
			fs.writeFileSync(testFilePath, pdfContent);
		}

		// Upload file through the upload input
		const fileInput = page.locator('input[name="Attachments"]');
		await fileInput.setInputFiles(testFilePath);

		// Wait for upload to complete and save
		await page.locator('[id=save-btn]').click();

		// Verify attachment appears in the list
		await expect(page.locator('.attachment-list')).toBeVisible({ timeout: 10000 });
		await expect(page.locator('.attachment-list a').filter({ hasText: /test-document/ })).toBeVisible();
	});

	test('uploads an image attachment to an expense', async ({ page }) => {
		// Create a test image file
		const testFilePath = path.join(__dirname, '../../fixtures/test-files/test-receipt.png');
		const testFileDir = path.dirname(testFilePath);

		if (!fs.existsSync(testFileDir)) {
			fs.mkdirSync(testFileDir, { recursive: true });
		}

		if (!fs.existsSync(testFilePath)) {
			// Create a 1x1 PNG file (minimal valid PNG)
			const pngBuffer = Buffer.from([
				0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
				0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
				0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
				0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
				0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41,
				0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
				0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
				0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
				0x42, 0x60, 0x82
			]);
			fs.writeFileSync(testFilePath, pngBuffer);
		}

		// Upload file
		const fileInput = page.locator('input[name="Attachments"]');
		await fileInput.setInputFiles(testFilePath);

		// Save and verify
		await page.locator('[id=save-btn]').click();

		await expect(page.locator('.attachment-list')).toBeVisible({ timeout: 10000 });
		await expect(page.locator('.attachment-list a').filter({ hasText: /test-receipt/ })).toBeVisible();
	});

	test('uploads multiple attachments to an expense', async ({ page }) => {
		// Create test files
		const testPdfPath = path.join(__dirname, '../../fixtures/test-files/invoice.pdf');
		const testImagePath = path.join(__dirname, '../../fixtures/test-files/receipt.png');
		const testFileDir = path.dirname(testPdfPath);

		if (!fs.existsSync(testFileDir)) {
			fs.mkdirSync(testFileDir, { recursive: true });
		}

		// Create PDF
		if (!fs.existsSync(testPdfPath)) {
			const pdfContent = '%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj 2 0 obj<</Type/Pages/Count 1/Kids[3 0 R]>>endobj 3 0 obj<</Type/Page/MediaBox[0 0 612 792]/Parent 2 0 R/Resources<<>>>>endobj\nxref\n0 4\n0000000000 65535 f\n0000000009 00000 n\n0000000056 00000 n\n0000000115 00000 n\ntrailer<</Size 4/Root 1 0 R>>\nstartxref\n210\n%%EOF';
			fs.writeFileSync(testPdfPath, pdfContent);
		}

		// Create PNG
		if (!fs.existsSync(testImagePath)) {
			const pngBuffer = Buffer.from([
				0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
				0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
				0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
				0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
				0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41,
				0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
				0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
				0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
				0x42, 0x60, 0x82
			]);
			fs.writeFileSync(testImagePath, pngBuffer);
		}

		// Upload multiple files (multiple: true is set in the component)
		const fileInput = page.locator('input[name="Attachments"]');
		await fileInput.setInputFiles([testPdfPath, testImagePath]);

		// Save and verify
		await page.locator('[id=save-btn]').click();

		// Should see both attachments
		await expect(page.locator('.attachment-list')).toBeVisible({ timeout: 10000 });
		await expect(page.locator('.attachment-list a').filter({ hasText: /invoice/ })).toBeVisible();
		await expect(page.locator('.attachment-list a').filter({ hasText: /receipt/ })).toBeVisible();
	});

	test('displays attachment with correct icon based on file type', async ({ page }) => {
		// Create a test PDF
		const testFilePath = path.join(__dirname, '../../fixtures/test-files/report.pdf');
		const testFileDir = path.dirname(testFilePath);

		if (!fs.existsSync(testFileDir)) {
			fs.mkdirSync(testFileDir, { recursive: true });
		}

		if (!fs.existsSync(testFilePath)) {
			const pdfContent = '%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj 2 0 obj<</Type/Pages/Count 1/Kids[3 0 R]>>endobj 3 0 obj<</Type/Page/MediaBox[0 0 612 792]/Parent 2 0 R/Resources<<>>>>endobj\nxref\n0 4\n0000000000 65535 f\n0000000009 00000 n\n0000000056 00000 n\n0000000115 00000 n\ntrailer<</Size 4/Root 1 0 R>>\nstartxref\n210\n%%EOF';
			fs.writeFileSync(testFilePath, pdfContent);
		}

		// Upload PDF
		const fileInput = page.locator('input[name="Attachments"]');
		await fileInput.setInputFiles(testFilePath);

		await page.locator('[id=save-btn]').click();

		// Verify PDF icon is shown (should have red color class for PDF)
		await expect(page.locator('.attachment-list')).toBeVisible({ timeout: 10000 });
		const attachmentItem = page.locator('.attachment-list').first();
		await expect(attachmentItem.locator('svg.text-red-500')).toBeVisible(); // PDF icon should be red
	});

	test('deletes an attachment from an expense', async ({ page }) => {
		// First upload an attachment
		const testFilePath = path.join(__dirname, '../../fixtures/test-files/to-delete.pdf');
		const testFileDir = path.dirname(testFilePath);

		if (!fs.existsSync(testFileDir)) {
			fs.mkdirSync(testFileDir, { recursive: true });
		}

		if (!fs.existsSync(testFilePath)) {
			const pdfContent = '%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj 2 0 obj<</Type/Pages/Count 1/Kids[3 0 R]>>endobj 3 0 obj<</Type/Page/MediaBox[0 0 612 792]/Parent 2 0 R/Resources<<>>>>endobj\nxref\n0 4\n0000000000 65535 f\n0000000009 00000 n\n0000000056 00000 n\n0000000115 00000 n\ntrailer<</Size 4/Root 1 0 R>>\nstartxref\n210\n%%EOF';
			fs.writeFileSync(testFilePath, pdfContent);
		}

		const fileInput = page.locator('input[name="Attachments"]');
		await fileInput.setInputFiles(testFilePath);
		await page.locator('[id=save-btn]').click();

		// Wait for attachment to appear
		await expect(page.locator('.attachment-list')).toBeVisible({ timeout: 10000 });
		await expect(page.locator('.attachment-list a').filter({ hasText: /to-delete/ })).toBeVisible();

		// Find and click delete button
		const deleteButton = page.locator('.attachment-list button').first();
		await expect(deleteButton).toBeVisible();

		// Handle confirmation dialog
		page.on('dialog', async dialog => {
			expect(dialog.type()).toBe('confirm');
			await dialog.accept();
		});

		await deleteButton.click();

		// Verify attachment is removed - the attachment-list should either be hidden or not contain the file
		await expect(page.locator('.attachment-list a').filter({ hasText: /to-delete/ })).not.toBeVisible({ timeout: 10000 });
	});

	test('attachment download link works correctly', async ({ page }) => {
		// Upload an attachment
		const testFilePath = path.join(__dirname, '../../fixtures/test-files/download-test.pdf');
		const testFileDir = path.dirname(testFilePath);

		if (!fs.existsSync(testFileDir)) {
			fs.mkdirSync(testFileDir, { recursive: true });
		}

		if (!fs.existsSync(testFilePath)) {
			const pdfContent = '%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj 2 0 obj<</Type/Pages/Count 1/Kids[3 0 R]>>endobj 3 0 obj<</Type/Page/MediaBox[0 0 612 792]/Parent 2 0 R/Resources<<>>>>endobj\nxref\n0 4\n0000000000 65535 f\n0000000009 00000 n\n0000000056 00000 n\n0000000115 00000 n\ntrailer<</Size 4/Root 1 0 R>>\nstartxref\n210\n%%EOF';
			fs.writeFileSync(testFilePath, pdfContent);
		}

		const fileInput = page.locator('input[name="Attachments"]');
		await fileInput.setInputFiles(testFilePath);
		await page.locator('[id=save-btn]').click();

		await expect(page.locator('.attachment-list')).toBeVisible({ timeout: 10000 });

		// Verify download link has correct attributes
		const downloadLink = page.locator('.attachment-list a').filter({ hasText: /download-test/ }).first();
		await expect(downloadLink).toHaveAttribute('target', '_blank');
		await expect(downloadLink).toHaveAttribute('rel', 'noopener noreferrer');

		// Verify link has a valid href
		const href = await downloadLink.getAttribute('href');
		expect(href).toBeTruthy();
		expect(href).toContain('/uploads/'); // Assuming uploads are served from /uploads/
	});
});
