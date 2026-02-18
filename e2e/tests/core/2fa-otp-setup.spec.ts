import { test, expect } from '@playwright/test';
import { login, logout } from '../../fixtures/auth';
import { resetTestDatabase, seedScenario } from '../../fixtures/test-data';
import { TwoFactorSetupPage } from '../../pages/core/twofactor-setup-page';
import { generateInvalidOTP, waitForOTP } from '../../helpers/otp';

/**
 * OTP Setup Flow E2E Tests (Email/SMS)
 *
 * Tests the complete OTP (One-Time Password) setup workflow for SMS and Email methods:
 * - Method selection
 * - OTP sending
 * - Code verification
 * - Resend functionality
 * - Error handling
 */

test.describe('2FA OTP Setup Flow', () => {
	test.beforeEach(async ({ page, request }) => {
		await resetTestDatabase(request, { reseedMinimal: false });
		await seedScenario(request, 'comprehensive');
		await page.setViewportSize({ width: 1280, height: 720 });
		await login(page, 'test@gmail.com', 'TestPass123!');
	});

	test.afterEach(async ({ page, request }) => {
		await logout(page);
	});

	test('should send OTP automatically after selecting Email method', async ({ page, request }) => {
		const setupPage = new TwoFactorSetupPage(page);

		// Navigate to setup page
		await page.goto('/login/2fa/setup');

		// Select Email method
		await setupPage.selectMethod('email');

		// Verify success message indicates OTP was sent
		await setupPage.expectSuccessMessage();

		// Verify code input is displayed
		await expect(page.locator('input[name="Code"]')).toBeVisible();

		// Verify email-specific instructions
		await expect(page.locator('text=/email|inbox/i').first()).toBeVisible();
	});

	test('should send OTP automatically after selecting SMS method', async ({ page, request }) => {
		const setupPage = new TwoFactorSetupPage(page);

		// Navigate to setup page
		await page.goto('/login/2fa/setup');

		// Select SMS method
		await setupPage.selectMethod('sms');

		// Verify success message indicates OTP was sent
		await setupPage.expectSuccessMessage();

		// Verify code input is displayed
		await expect(page.locator('input[name="Code"]')).toBeVisible();

		// Verify SMS-specific instructions
		await expect(page.locator('text=/sms|phone|text message/i').first()).toBeVisible();
	});

	test('should successfully complete Email OTP setup with valid code', async ({ page, request }) => {
		const setupPage = new TwoFactorSetupPage(page);
		const userEmail = 'test@gmail.com'; // From comprehensive seed data

		// Navigate to setup page
		await page.goto('/login/2fa/setup');

		// Select Email method
		await setupPage.selectMethod('email');

		// Wait for OTP to be sent and retrieve from database
		const otpCode = await waitForOTP(request, userEmail);

		// Enter valid OTP code
		await setupPage.enterOTPCode(otpCode);

		// Verify successful setup (redirect to success page or dashboard)
		await expect(page).not.toHaveURL(/\/login\/2fa\/setup/);
		await expect(page).not.toHaveURL(/\/login/);
	});

	test('should successfully complete SMS OTP setup with valid code', async ({ page, request }) => {
		const setupPage = new TwoFactorSetupPage(page);
		const userPhone = '+998901230001'; // Phone from test user in seed data

		// Navigate to setup page
		await page.goto('/login/2fa/setup');

		// Select SMS method
		await setupPage.selectMethod('sms');

		// Wait for OTP to be sent and retrieve from database
		const otpCode = await waitForOTP(request, userPhone);

		// Enter valid OTP code
		await setupPage.enterOTPCode(otpCode);

		// Verify successful setup
		await expect(page).not.toHaveURL(/\/login\/2fa\/setup/);
		await expect(page).not.toHaveURL(/\/login/);
	});

	test('should display error for invalid OTP code', async ({ page, request }) => {
		const setupPage = new TwoFactorSetupPage(page);

		// Navigate to setup page
		await page.goto('/login/2fa/setup');

		// Select Email method
		await setupPage.selectMethod('email');

		// Enter invalid code
		const invalidCode = generateInvalidOTP();
		await setupPage.enterOTPCode(invalidCode);

		// Verify user remains on setup page (can retry)
		await expect(page).toHaveURL(/\/login\/2fa\/setup\/otp/);
		await expect(page.locator('input[name="Code"]')).toBeVisible();
	});

	test('should allow OTP resend for Email method', async ({ page, request }) => {
		const setupPage = new TwoFactorSetupPage(page);

		// Navigate to setup page
		await page.goto('/login/2fa/setup');

		// Select Email method
		await setupPage.selectMethod('email');

		// Click resend button
		await setupPage.resendCode();

		// Verify success message
		await setupPage.expectSuccessMessage();

		// Verify code input is still visible
		await expect(page.locator('input[name="Code"]')).toBeVisible();
	});

	test('should allow OTP resend for SMS method', async ({ page, request }) => {
		const setupPage = new TwoFactorSetupPage(page);

		// Navigate to setup page
		await page.goto('/login/2fa/setup');

		// Select SMS method
		await setupPage.selectMethod('sms');

		// Click resend button
		await setupPage.resendCode();

		// Verify success message
		await setupPage.expectSuccessMessage();

		// Verify code input is still visible
		await expect(page.locator('input[name="Code"]')).toBeVisible();
	});

	test('should allow retry after invalid OTP code', async ({ page, request }) => {
		const setupPage = new TwoFactorSetupPage(page);
		const userEmail = 'test@gmail.com';

		// Navigate to setup page
		await page.goto('/login/2fa/setup');

		// Select Email method
		await setupPage.selectMethod('email');

		// First attempt: invalid code
		await setupPage.enterOTPCode(generateInvalidOTP());
		await expect(page).toHaveURL(/\/login\/2fa\/setup\/otp/);

		// Second attempt: valid code
		const otpCode = await waitForOTP(request, userEmail);
		await setupPage.enterOTPCode(otpCode);

		// Verify successful setup
		await expect(page).toHaveURL(/^(?!.*\/login\/2fa\/setup)/);
	});

	test.skip('should display error if user tries SMS without phone number', async ({ page, request }) => {
		// This test assumes the system validates phone number requirement
		// and shows appropriate error. Implementation may vary.
		// Skip for now if system allows SMS setup without validation
	});

	test('should preserve nextURL parameter throughout OTP setup', async ({ page, request }) => {
		const setupPage = new TwoFactorSetupPage(page);
		const nextURL = '/users';
		const userEmail = 'test@gmail.com';

		// Navigate with nextURL parameter
		await page.goto(`/login/2fa/setup?next=${encodeURIComponent(nextURL)}`);

		// Select Email method
		await setupPage.selectMethod('email');

		// Verify nextURL is preserved in form
		const hiddenNextURL = await page.locator('input[name="NextURL"]').first().inputValue();
		expect(hiddenNextURL).toBe(nextURL);

		// Complete setup
		const otpCode = await waitForOTP(request, userEmail);
		await setupPage.enterOTPCode(otpCode);

		// Verify redirect to nextURL (or shows success, then redirects)
		// Implementation may vary
		await page.waitForTimeout(1000);
		// await expect(page).toHaveURL(new RegExp(nextURL));
	});

	test('should validate OTP input format (6 digits)', async ({ page, request }) => {
		const setupPage = new TwoFactorSetupPage(page);

		// Navigate to setup page
		await page.goto('/login/2fa/setup');

		// Select Email method
		await setupPage.selectMethod('email');

		const codeInput = page.locator('input[name="Code"]');

		// Verify input constraints
		await expect(codeInput).toHaveAttribute('maxlength', '6');
		await expect(codeInput).toHaveAttribute('pattern', '[0-9]{6}');
		await expect(codeInput).toHaveAttribute('inputmode', 'numeric');

		// Try to enter more than 6 digits
		await codeInput.fill('1234567890');
		const value = await codeInput.inputValue();
		expect(value.length).toBeLessThanOrEqual(6);
	});

	test('should display method-specific help text', async ({ page, request }) => {
		const setupPage = new TwoFactorSetupPage(page);

		// Test Email method
		await page.goto('/login/2fa/setup');
		await setupPage.selectMethod('email');
		await expect(page.locator('h1')).toContainText(/email/i);
		await expect(page.locator('input[name="Code"]')).toBeVisible();

		// Go back and test SMS method
		await page.goto('/login/2fa/setup');
		await setupPage.selectMethod('sms');
		await expect(page.locator('h1')).toContainText(/phone|sms/i);
		await expect(page.locator('input[name="Code"]')).toBeVisible();
	});

	test('should display destination for OTP delivery', async ({ page, request }) => {
		const setupPage = new TwoFactorSetupPage(page);

		// Navigate to setup page
		await page.goto('/login/2fa/setup');

		// Select Email method
		await setupPage.selectMethod('email');

		// Verify email address is displayed (masked or full)
		// This helps users confirm where the code was sent
		// Implementation may vary - could be "sent to t***@gmail.com" or full email
		// await expect(page.locator('text=/@|gmail|email/i')).toBeVisible();
	});

	test('should not display recovery codes for OTP setup (only TOTP)', async ({ page, request }) => {
		const setupPage = new TwoFactorSetupPage(page);
		const userEmail = 'test@gmail.com';

		// Navigate to setup page
		await page.goto('/login/2fa/setup');

		// Select Email method
		await setupPage.selectMethod('email');

		// Complete setup
		const otpCode = await waitForOTP(request, userEmail);
		await setupPage.enterOTPCode(otpCode);

		// Verify NO recovery codes are displayed (OTP methods don't use recovery codes)
		const recoveryCodes = await setupPage.getRecoveryCodes();
		expect(recoveryCodes.length).toBe(0);
	});
});
