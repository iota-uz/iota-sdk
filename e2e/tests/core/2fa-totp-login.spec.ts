import { test, expect } from '@playwright/test';
import { login, logout } from '../../fixtures/auth';
import { resetTestDatabase, populateTestData } from '../../fixtures/test-data';
import { TwoFactorVerifyPage } from '../../pages/core/twofactor-verify-page';
import { generateTOTPCode, generateInvalidTOTPCode } from '../../helpers/totp';

/**
 * TOTP Login Flow E2E Tests
 *
 * Tests login verification for users who already have TOTP enabled:
 * - Redirect to verification after login
 * - Successful verification with valid code
 * - Error handling for invalid codes
 * - Recovery code access
 * - Session state transitions
 */

test.describe('2FA TOTP Login Flow', () => {
	// Test data: user with TOTP already enabled
	const testUser = {
		email: 'totp-user@example.com',
		password: 'TestPass123!',
		totpSecret: 'JBSWY3DPEHPK3PXP', // Base32 encoded test secret
	};

	test.beforeAll(async ({ request }) => {
		// Reset database
		await resetTestDatabase(request, { reseedMinimal: true });

		// Create test user with TOTP enabled
		await populateTestData(request, {
			version: '1.0',
			tenant: {
				id: '00000000-0000-0000-0000-000000000001',
				name: 'Test Tenant',
				domain: 'test.localhost',
			},
			data: {
				users: [
					{
						email: testUser.email,
						password: testUser.password,
						firstName: 'TOTP',
						lastName: 'User',
						language: 'en',
						twoFactorMethod: 'totp',
						totpSecretEncrypted: testUser.totpSecret, // In real system, this would be encrypted
						twoFactorEnabledAt: new Date().toISOString(),
					},
				],
			},
		});
	});

	test.beforeEach(async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 720 });
	});

	test.afterEach(async ({ page }) => {
		await logout(page);
	});

	test('should redirect to verification page after login for TOTP user', async ({ page }) => {
		// Attempt login with credentials
		await page.goto('/login');
		await page.fill('[type=email]', testUser.email);
		await page.fill('[type=password]', testUser.password);

		// Submit login form
		await Promise.all([
			page.waitForURL((url) => url.pathname.includes('/login/2fa/verify')),
			page.click('[type=submit]'),
		]);

		// Verify redirected to 2FA verification page
		await expect(page).toHaveURL(/\/login\/2fa\/verify/);

		// Verify verification form is displayed
		await expect(page.locator('input[name="Code"]')).toBeVisible();
		await expect(page.locator('input[name="Code"]')).toHaveAttribute('maxlength', '6');
	});

	test('should successfully login with valid TOTP code', async ({ page }) => {
		const verifyPage = new TwoFactorVerifyPage(page);

		// Login with credentials
		await page.goto('/login');
		await page.fill('[type=email]', testUser.email);
		await page.fill('[type=password]', testUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		// Generate valid TOTP code
		const validCode = generateTOTPCode(testUser.totpSecret);

		// Enter code and verify
		await verifyPage.enterVerificationCode(validCode);

		// Verify successful login (redirect to home/dashboard)
		await expect(page).toHaveURL(/^(?!.*\/login)/); // Not on login page
		await expect(page).not.toHaveURL(/\/login/);

		// Verify session is active (user can access protected routes)
		await page.goto('/users');
		await expect(page).not.toHaveURL(/\/login/); // Should not redirect to login
	});

	test('should display error for invalid TOTP code', async ({ page }) => {
		const verifyPage = new TwoFactorVerifyPage(page);

		// Login with credentials
		await page.goto('/login');
		await page.fill('[type=email]', testUser.email);
		await page.fill('[type=password]', testUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		// Enter invalid code
		const invalidCode = generateInvalidTOTPCode(testUser.totpSecret);
		await verifyPage.enterVerificationCode(invalidCode);

		// Verify error message is displayed
		await verifyPage.expectErrorMessage();

		// Verify user remains on verification page
		await expect(page).toHaveURL(/\/login\/2fa\/verify/);
		await expect(page.locator('input[name="Code"]')).toBeVisible();
	});

	test('should allow multiple retry attempts after invalid code', async ({ page }) => {
		const verifyPage = new TwoFactorVerifyPage(page);

		// Login with credentials
		await page.goto('/login');
		await page.fill('[type=email]', testUser.email);
		await page.fill('[type=password]', testUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		// First attempt: invalid code
		await verifyPage.enterVerificationCode(generateInvalidTOTPCode(testUser.totpSecret));
		await verifyPage.expectErrorMessage();

		// Second attempt: another invalid code
		await verifyPage.enterVerificationCode('111111');
		await verifyPage.expectErrorMessage();

		// Third attempt: valid code
		const validCode = generateTOTPCode(testUser.totpSecret);
		await verifyPage.enterVerificationCode(validCode);

		// Verify successful login
		await expect(page).not.toHaveURL(/\/login/);
	});

	test('should preserve nextURL parameter and redirect after verification', async ({ page }) => {
		const verifyPage = new TwoFactorVerifyPage(page);
		const nextURL = '/users';

		// Login with credentials and nextURL parameter
		await page.goto(`/login?next=${encodeURIComponent(nextURL)}`);
		await page.fill('[type=email]', testUser.email);
		await page.fill('[type=password]', testUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		// Verify nextURL is preserved
		await expect(page).toHaveURL(new RegExp(`next=${encodeURIComponent(nextURL)}`));

		// Enter valid code
		const validCode = generateTOTPCode(testUser.totpSecret);
		await verifyPage.enterVerificationCode(validCode);

		// Verify redirect to nextURL after successful verification
		await expect(page).toHaveURL(new RegExp(nextURL));
	});

	test('should display recovery code link on verification page', async ({ page }) => {
		const verifyPage = new TwoFactorVerifyPage(page);

		// Login with credentials
		await page.goto('/login');
		await page.fill('[type=email]', testUser.email);
		await page.fill('[type=password]', testUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		// Verify recovery code link is visible
		await verifyPage.expectRecoveryLinkVisible();

		// Click recovery link
		await verifyPage.navigateToRecoveryPage();

		// Verify navigated to recovery page
		await expect(page).toHaveURL(/\/login\/2fa\/verify\/recovery/);
		await expect(page.locator('input[name="Code"]')).toBeVisible();
	});

	test('should not display resend button for TOTP method', async ({ page }) => {
		// Login with credentials
		await page.goto('/login');
		await page.fill('[type=email]', testUser.email);
		await page.fill('[type=password]', testUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		// Verify resend button is NOT present (TOTP doesn't need resend)
		const resendButton = page.locator('button:has-text("Resend")');
		await expect(resendButton).not.toBeVisible();
	});

	test('should display TOTP-specific instructions', async ({ page }) => {
		// Login with credentials
		await page.goto('/login');
		await page.fill('[type=email]', testUser.email);
		await page.fill('[type=password]', testUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		// Verify TOTP-specific text is present
		await expect(page.locator('text=/authenticator/i')).toBeVisible();

		// Verify heading mentions entering code
		await expect(page.locator('h1, h2')).toContainText(/code/i);
	});

	test('should validate code input format', async ({ page }) => {
		// Login with credentials
		await page.goto('/login');
		await page.fill('[type=email]', testUser.email);
		await page.fill('[type=password]', testUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		const codeInput = page.locator('input[name="Code"]');

		// Verify input constraints
		await expect(codeInput).toHaveAttribute('maxlength', '6');
		await expect(codeInput).toHaveAttribute('pattern', '[0-9]{6}');
		await expect(codeInput).toHaveAttribute('inputmode', 'numeric');

		// Verify placeholder
		const placeholder = await codeInput.getAttribute('placeholder');
		expect(placeholder).toMatch(/0{6}|code/i);
	});

	test('should autofocus verification code input', async ({ page }) => {
		// Login with credentials
		await page.goto('/login');
		await page.fill('[type=email]', testUser.email);
		await page.fill('[type=password]', testUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		// Verify code input has autofocus
		const codeInput = page.locator('input[name="Code"]');
		await expect(codeInput).toHaveAttribute('autofocus');

		// Verify input is focused
		await expect(codeInput).toBeFocused();
	});
});
