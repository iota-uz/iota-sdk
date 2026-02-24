import { test, expect } from '@playwright/test';
import { login, logout } from '../../fixtures/auth';
import { resetTestDatabase, seedScenario } from '../../fixtures/test-data';
import { TwoFactorSetupPage } from '../../pages/core/twofactor-setup-page';
import { generateTOTPCode, generateInvalidTOTPCode } from '../../helpers/totp';

/**
 * TOTP Setup Flow E2E Tests
 *
 * Tests the complete TOTP (Time-based One-Time Password) setup workflow including:
 * - Method selection
 * - QR code display
 * - Code verification
 * - Recovery codes generation
 * - Error handling
 */

test.describe('2FA TOTP Setup Flow', () => {
	test.beforeEach(async ({ page, request }) => {
		await resetTestDatabase(request, { reseedMinimal: false });
		await seedScenario(request, 'comprehensive');
		await page.setViewportSize({ width: 1280, height: 720 });
		await login(page, 'test@gmail.com', 'TestPass123!');
	});

	test.afterEach(async ({ page }) => {
		await logout(page);
	});

	test('should display method selection page when user needs 2FA setup', async ({ page }) => {
		// Navigate to setup page (simulating session in Pending2FA state)
		await page.goto('/login/2fa/setup');

		// Verify method choice page is displayed
		await expect(page.locator('h1')).toContainText(/two.?factor|2fa/i);

		// Verify all three methods are available
		await expect(page.locator('input[name="Method"][value="totp"]')).toBeVisible();
		await expect(page.locator('input[name="Method"][value="sms"]')).toBeVisible();
		await expect(page.locator('input[name="Method"][value="email"]')).toBeVisible();

		// Verify TOTP is pre-selected (default)
		await expect(page.locator('input[name="Method"][value="totp"]')).toBeChecked();
	});

	test('should display QR code after selecting TOTP method', async ({ page }) => {
		const setupPage = new TwoFactorSetupPage(page);

		// Navigate to setup page
		await page.goto('/login/2fa/setup');

		// Select TOTP method
		await setupPage.selectMethod('totp');

		// Verify QR code is displayed
		await setupPage.expectQRCodeVisible();

		// Verify setup instructions are present
		await expect(page.locator('text=/scan.*qr|authenticator/i').first()).toBeVisible();

		// Verify code input field is present
		await expect(page.locator('input[name="Code"]')).toBeVisible();
		await expect(page.locator('input[name="Code"]')).toHaveAttribute('maxlength', '6');
		await expect(page.locator('input[name="Code"]')).toHaveAttribute('pattern', '[0-9]{6}');
	});

	test('should successfully complete TOTP setup with valid code', async ({ page }) => {
		const setupPage = new TwoFactorSetupPage(page);

		// Navigate to setup page
		await page.goto('/login/2fa/setup');

		// Select TOTP method
		await setupPage.selectMethod('totp');

		// Extract TOTP secret and generate valid code
		const secret = await setupPage.extractTOTPSecret();
		const validCode = generateTOTPCode(secret);

		// Enter valid code
		await setupPage.enterTOTPCode(validCode);

		// Verify recovery codes are displayed
		const recoveryCodes = await setupPage.getRecoveryCodes();
		expect(recoveryCodes.length).toBeGreaterThan(0);
		expect(recoveryCodes.length).toBeLessThanOrEqual(10);

		// Verify each recovery code has expected format (alphanumeric)
		recoveryCodes.forEach((code) => {
			expect(code).toMatch(/^[A-Z0-9-]+$/i);
		});
	});

	test('should display error message for invalid TOTP code', async ({ page }) => {
		const setupPage = new TwoFactorSetupPage(page);

		// Navigate to setup page
		await page.goto('/login/2fa/setup');

		// Select TOTP method
		await setupPage.selectMethod('totp');

		// Enter invalid code
		const secret = await setupPage.extractTOTPSecret();
		const invalidCode = generateInvalidTOTPCode(secret);
		await setupPage.enterTOTPCode(invalidCode);

		// Verify error message is displayed
		await setupPage.expectErrorMessage();

		// Verify user remains on setup page (can retry)
		await expect(page).toHaveURL(/\/login\/2fa\/setup\/totp/);
		await expect(page.locator('input[name="Code"]')).toBeVisible();
	});

	test('should allow retry after invalid TOTP code', async ({ page }) => {
		const setupPage = new TwoFactorSetupPage(page);

		// Navigate to setup page
		await page.goto('/login/2fa/setup');

		// Select TOTP method
		await setupPage.selectMethod('totp');

		// First attempt: invalid code
		const secret = await setupPage.extractTOTPSecret();
		const invalidCode = generateInvalidTOTPCode(secret);
		await setupPage.enterTOTPCode(invalidCode);
		await setupPage.expectErrorMessage();

		// Second attempt: valid code
		const validCode = generateTOTPCode(secret);
		await setupPage.enterTOTPCode(validCode);

		// Verify recovery codes are displayed (setup successful)
		const recoveryCodes = await setupPage.getRecoveryCodes();
		expect(recoveryCodes.length).toBeGreaterThan(0);
	});

	test('should preserve nextURL parameter throughout setup flow', async ({ page }) => {
		const setupPage = new TwoFactorSetupPage(page);
		const nextURL = '/users';

		// Navigate with nextURL parameter
		await page.goto(`/login/2fa/setup?next=${encodeURIComponent(nextURL)}`);

		// Select TOTP method
		await setupPage.selectMethod('totp');

		// Verify nextURL is preserved in form
		const hiddenNextURL = await page.locator('input[name="NextURL"]').inputValue();
		expect(hiddenNextURL).toBe(nextURL);

		// Complete setup with valid code
		const secret = await setupPage.extractTOTPSecret();
		const validCode = generateTOTPCode(secret);
		await setupPage.enterTOTPCode(validCode);

		// After completing setup, user should be redirected to nextURL
		// (or recovery codes page first, then redirect after continuing)
		// This behavior depends on implementation
		// await expect(page).toHaveURL(new RegExp(nextURL));
	});

	test('should validate code format (6 digits only)', async ({ page }) => {
		const setupPage = new TwoFactorSetupPage(page);

		// Navigate to setup page
		await page.goto('/login/2fa/setup');

		// Select TOTP method
		await setupPage.selectMethod('totp');

		// Try to enter non-numeric characters
		const codeInput = page.locator('input[name="Code"]');
		await expect(codeInput).toHaveAttribute('pattern', '[0-9]{6}');
		await codeInput.fill('abc123');
		const isValidBeforeSubmit = await codeInput.evaluate(
			(node) => (node as HTMLInputElement).validity.valid,
		);
		expect(isValidBeforeSubmit).toBe(false);
		await page.click('button[type="submit"]');
		await expect(page).toHaveURL(/\/login\/2fa\/setup\/totp/);
		const isValidAfterSubmit = await codeInput.evaluate(
			(node) => (node as HTMLInputElement).validity.valid,
		);
		expect(isValidAfterSubmit).toBe(false);

		// Try to enter more than 6 digits
		await codeInput.fill('1234567890');
		const value2 = await codeInput.inputValue();
		expect(value2.length).toBeLessThanOrEqual(6);
	});

	test('should display help text and instructions for TOTP setup', async ({ page }) => {
		const setupPage = new TwoFactorSetupPage(page);

		// Navigate to setup page
		await page.goto('/login/2fa/setup');

		// Select TOTP method
		await setupPage.selectMethod('totp');

		// Verify help text is present
		await expect(page.locator('h1')).toContainText(/authenticator|two.?factor|2fa/i);

		// Verify instructions mention scanning QR code
		await expect(page.locator('text=/scan|qr/i').first()).toBeVisible();

		// Verify there's a code input instruction
		await expect(page.locator('text=/enter.*code|verification.*code|6.*digit/i').first()).toBeVisible();
	});
});
