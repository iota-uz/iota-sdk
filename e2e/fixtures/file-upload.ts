/**
 * File upload fixtures for Playwright tests
 *
 * Migrated from Cypress custom commands (cypress/support/commands.js)
 */

import { Page } from '@playwright/test';

/**
 * Upload a file and wait for attachment processing
 * Migrates Cypress.Commands.add("uploadFileAndWaitForAttachment")
 *
 * @param page - Playwright page object
 * @param fileContent - File content as string
 * @param fileName - Name of the file
 * @param mimeType - MIME type (default: 'text/plain')
 */
export async function uploadFileAndWaitForAttachment(
	page: Page,
	fileContent: string,
	fileName: string,
	mimeType: string = 'text/plain'
) {
	// Create file buffer
	const buffer = Buffer.from(fileContent);

	// Upload file
	const fileInput = page.locator('input[type="file"]');
	await fileInput.setInputFiles({
		name: fileName,
		mimeType: mimeType,
		buffer: buffer,
	});

	// Wait for upload to complete (hidden input should have value)
	const attachmentInput = page.locator('input[type="hidden"][name="Attachments"]');
	await attachmentInput.waitFor({ state: 'attached', timeout: 15000 });

	// Verify it has a non-empty value
	const value = await attachmentInput.inputValue();
	if (!value) {
		throw new Error('File upload did not complete - Attachments input is empty');
	}
}
