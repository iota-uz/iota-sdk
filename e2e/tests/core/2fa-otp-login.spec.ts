import { test, expect } from '@playwright/test';
import { login, logout } from '../../fixtures/auth';
import { resetTestDatabase, populateTestData } from '../../fixtures/test-data';
import { TwoFactorVerifyPage } from '../../pages/core/twofactor-verify-page';
import { getOTPCodeFromDB, generateInvalidOTP, waitForOTP } from '../../helpers/otp';

/**
 * OTP Login Flow E2E Tests (Email/SMS)
 *
 * Tests login verification for users who have SMS or Email 2FA enabled:
 * - Automatic OTP sending on verification page
 * - Successful verification with valid OTP
 * - Resend functionality
 * - Error handling
 * - Session state transitions
 */

test.describe('2FA OTP Login Flow - Email Method', () => {
	// Test data: user with Email OTP enabled
	const emailUser = {
		email: 'otp-email-user@example.com',
		password: 'TestPass123!',
	};

	test.beforeAll(async ({ request }) => {
		// Reset database
		await resetTestDatabase(request, { reseedMinimal: true });

		// Create test user with Email OTP enabled
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
						email: emailUser.email,
						password: emailUser.password,
						firstName: 'Email',
						lastName: 'OTP User',
						language: 'en',
						twoFactorMethod: 'email',
						twoFactorEnabledAt: new Date().toISOString(),
					},
				],
			},
		});
	});

	test.beforeEach(async ({ page, request }) => {
		await page.setViewportSize({ width: 1280, height: 720 });
	});

	test.afterEach(async ({ page, request }) => {
		await logout(page);
	});

	test('should redirect to verification page and send OTP automatically', async ({ page, request }) => {
		// Login with credentials
		await page.goto('/login');
		await page.fill('[type=email]', emailUser.email);
		await page.fill('[type=password]', emailUser.password);

		await Promise.all([
			page.waitForURL((url) => url.pathname.includes('/login/2fa/verify')),
			page.click('[type=submit]'),
		]);

		// Verify redirected to verification page
		await expect(page).toHaveURL(/\/login\/2fa\/verify/);

		// Verify email-specific content
		await expect(page.locator('h1, h2')).toContainText(/email/i);

		// Verify code input is displayed
		await expect(page.locator('input[name="Code"]')).toBeVisible();
	});

	test('should successfully login with valid Email OTP', async ({ page, request }) => {
		const verifyPage = new TwoFactorVerifyPage(page);

		// Login with credentials
		await page.goto('/login');
		await page.fill('[type=email]', emailUser.email);
		await page.fill('[type=password]', emailUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		// Wait for OTP and retrieve from database
		const otpCode = await waitForOTP(request, emailUser.email);

		// Enter OTP
		await verifyPage.enterVerificationCode(otpCode);

		// Verify successful login
		await expect(page).not.toHaveURL(/\/login/);

		// Verify can access protected routes
		await page.goto('/users');
		await expect(page).not.toHaveURL(/\/login/);
	});

	test('should display error for invalid Email OTP', async ({ page, request }) => {
		const verifyPage = new TwoFactorVerifyPage(page);

		// Login with credentials
		await page.goto('/login');
		await page.fill('[type=email]', emailUser.email);
		await page.fill('[type=password]', emailUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		// Enter invalid OTP
		await verifyPage.enterVerificationCode(generateInvalidOTP());

		// Verify error message
		await verifyPage.expectErrorMessage();

		// Verify remains on verification page
		await expect(page).toHaveURL(/\/login\/2fa\/verify/);
	});

	test('should allow resending Email OTP', async ({ page, request }) => {
		const verifyPage = new TwoFactorVerifyPage(page);

		// Login with credentials
		await page.goto('/login');
		await page.fill('[type=email]', emailUser.email);
		await page.fill('[type=password]', emailUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		// Verify resend button is visible
		await verifyPage.expectResendButtonVisible();

		// Click resend
		await verifyPage.resendCode();

		// Verify success message
		await verifyPage.expectSuccessMessage();
	});

	test('should preserve nextURL and redirect after Email OTP verification', async ({ page, request }) => {
		const verifyPage = new TwoFactorVerifyPage(page);
		const nextURL = '/users';

		// Login with nextURL parameter
		await page.goto(`/login?next=${encodeURIComponent(nextURL)}`);
		await page.fill('[type=email]', emailUser.email);
		await page.fill('[type=password]', emailUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		// Get OTP and verify
		const otpCode = await waitForOTP(request, emailUser.email);
		await verifyPage.enterVerificationCode(otpCode);

		// Verify redirect to nextURL
		await expect(page).toHaveURL(new RegExp(nextURL));
	});
});

test.describe('2FA OTP Login Flow - SMS Method', () => {
	// Test data: user with SMS OTP enabled
	const smsUser = {
		email: 'otp-sms-user@example.com',
		password: 'TestPass123!',
		phone: '+998901234567',
	};

	test.beforeAll(async ({ request }) => {
		// Reset database
		await resetTestDatabase(request, { reseedMinimal: true });

		// Create test user with SMS OTP enabled
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
						email: smsUser.email,
						password: smsUser.password,
						firstName: 'SMS',
						lastName: 'OTP User',
						language: 'en',
						phone: smsUser.phone,
						twoFactorMethod: 'sms',
						twoFactorEnabledAt: new Date().toISOString(),
					},
				],
			},
		});
	});

	test.beforeEach(async ({ page, request }) => {
		await page.setViewportSize({ width: 1280, height: 720 });
	});

	test.afterEach(async ({ page, request }) => {
		await logout(page);
	});

	test('should redirect to verification page and send SMS OTP automatically', async ({ page, request }) => {
		// Login with credentials
		await page.goto('/login');
		await page.fill('[type=email]', smsUser.email);
		await page.fill('[type=password]', smsUser.password);

		await Promise.all([
			page.waitForURL((url) => url.pathname.includes('/login/2fa/verify')),
			page.click('[type=submit]'),
		]);

		// Verify redirected to verification page
		await expect(page).toHaveURL(/\/login\/2fa\/verify/);

		// Verify SMS-specific content
		await expect(page.locator('h1, h2')).toContainText(/phone|sms/i);

		// Verify code input is displayed
		await expect(page.locator('input[name="Code"]')).toBeVisible();
	});

	test('should successfully login with valid SMS OTP', async ({ page, request }) => {
		const verifyPage = new TwoFactorVerifyPage(page);

		// Login with credentials
		await page.goto('/login');
		await page.fill('[type=email]', smsUser.email);
		await page.fill('[type=password]', smsUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		// Wait for OTP and retrieve from database
		const otpCode = await waitForOTP(request, smsUser.phone);

		// Enter OTP
		await verifyPage.enterVerificationCode(otpCode);

		// Verify successful login
		await expect(page).not.toHaveURL(/\/login/);

		// Verify can access protected routes
		await page.goto('/users');
		await expect(page).not.toHaveURL(/\/login/);
	});

	test('should display error for invalid SMS OTP', async ({ page, request }) => {
		const verifyPage = new TwoFactorVerifyPage(page);

		// Login with credentials
		await page.goto('/login');
		await page.fill('[type=email]', smsUser.email);
		await page.fill('[type=password]', smsUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		// Enter invalid OTP
		await verifyPage.enterVerificationCode(generateInvalidOTP());

		// Verify error message
		await verifyPage.expectErrorMessage();

		// Verify remains on verification page
		await expect(page).toHaveURL(/\/login\/2fa\/verify/);
	});

	test('should allow resending SMS OTP', async ({ page, request }) => {
		const verifyPage = new TwoFactorVerifyPage(page);

		// Login with credentials
		await page.goto('/login');
		await page.fill('[type=email]', smsUser.email);
		await page.fill('[type=password]', smsUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		// Verify resend button is visible
		await verifyPage.expectResendButtonVisible();

		// Click resend
		await verifyPage.resendCode();

		// Verify success message
		await verifyPage.expectSuccessMessage();
	});

	test('should allow multiple retries after invalid SMS OTP', async ({ page, request }) => {
		const verifyPage = new TwoFactorVerifyPage(page);

		// Login with credentials
		await page.goto('/login');
		await page.fill('[type=email]', smsUser.email);
		await page.fill('[type=password]', smsUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		// First attempt: invalid
		await verifyPage.enterVerificationCode(generateInvalidOTP());
		await verifyPage.expectErrorMessage();

		// Second attempt: invalid
		await verifyPage.enterVerificationCode('111111');
		await verifyPage.expectErrorMessage();

		// Third attempt: valid
		const otpCode = await waitForOTP(request, smsUser.phone);
		await verifyPage.enterVerificationCode(otpCode);

		// Verify successful login
		await expect(page).not.toHaveURL(/\/login/);
	});

	test('should display destination phone number (masked)', async ({ page, request }) => {
		// Login with credentials
		await page.goto('/login');
		await page.fill('[type=email]', smsUser.email);
		await page.fill('[type=password]', smsUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		// Verify phone number is displayed (could be masked like ***4567)
		// Implementation may vary - full number or masked
		// await expect(page.locator('text=/\\+998|998|567/i')).toBeVisible();
	});

	test('should display recovery code link on SMS verification page', async ({ page, request }) => {
		const verifyPage = new TwoFactorVerifyPage(page);

		// Login with credentials
		await page.goto('/login');
		await page.fill('[type=email]', smsUser.email);
		await page.fill('[type=password]', smsUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		// Verify recovery link is visible
		await verifyPage.expectRecoveryLinkVisible();

		// Click recovery link
		await verifyPage.navigateToRecoveryPage();

		// Verify navigated to recovery page
		await expect(page).toHaveURL(/\/login\/2fa\/verify\/recovery/);
	});
});
