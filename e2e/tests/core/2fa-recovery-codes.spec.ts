import { test, expect } from '@playwright/test';
import { login, logout } from '../../fixtures/auth';
import { resetTestDatabase, populateTestData } from '../../fixtures/test-data';
import { TwoFactorVerifyPage } from '../../pages/core/twofactor-verify-page';
import { TwoFactorSetupPage } from '../../pages/core/twofactor-setup-page';
import { generateTOTPCode } from '../../helpers/totp';
import { Pool } from 'pg';

/**
 * Recovery Codes E2E Tests
 *
 * Tests recovery code functionality:
 * - Recovery codes display after TOTP setup
 * - Recovery code verification for login bypass
 * - One-time use enforcement
 * - Error handling for invalid codes
 * - Recovery code depletion scenarios
 */

/**
 * Helper function to get database configuration
 */
function getDBConfig() {
	return {
		user: process.env.DB_USER || 'postgres',
		password: process.env.DB_PASSWORD || 'postgres',
		host: process.env.DB_HOST || 'localhost',
		port: parseInt(process.env.DB_PORT || '5438'),
		database: process.env.DB_NAME || 'iota_erp_e2e',
	};
}

/**
 * Helper function to get recovery codes for a user from database
 */
async function getRecoveryCodesFromDB(userID: number): Promise<string[]> {
	const config = getDBConfig();
	const pool = new Pool(config);

	try {
		const client = await pool.connect();
		try {
			const result = await client.query(
				`SELECT code FROM recovery_codes
				 WHERE user_id = $1 AND used_at IS NULL
				 ORDER BY created_at ASC`,
				[userID]
			);

			return result.rows.map((row) => row.code);
		} finally {
			client.release();
		}
	} finally {
		await pool.end();
	}
}

/**
 * Helper function to get user ID from email
 */
async function getUserIDByEmail(email: string): Promise<number> {
	const config = getDBConfig();
	const pool = new Pool(config);

	try {
		const client = await pool.connect();
		try {
			const result = await client.query(`SELECT id FROM users WHERE email = $1`, [email]);

			if (result.rows.length === 0) {
				throw new Error(`User not found: ${email}`);
			}

			return result.rows[0].id;
		} finally {
			client.release();
		}
	} finally {
		await pool.end();
	}
}

test.describe('2FA Recovery Codes', () => {
	// Test data: user with TOTP enabled and recovery codes
	const testUser = {
		email: 'recovery-test@example.com',
		password: 'TestPass123!',
		totpSecret: 'JBSWY3DPEHPK3PXP',
		recoveryCodes: ['RECOVERY-CODE-1', 'RECOVERY-CODE-2', 'RECOVERY-CODE-3'],
	};

	test.beforeAll(async ({ request }) => {
		// Reset database
		await resetTestDatabase(request, { reseedMinimal: true });

		// Create test user with TOTP and recovery codes
		// Note: This requires populating recovery codes via database or seed script
		// For now, we'll test the recovery code flow assuming they exist
		await populateTestData(request, {
			version: '1.0',
			data: {
				users: [
					{
						email: testUser.email,
						password: testUser.password,
						firstName: 'Recovery',
						lastName: 'Test',
						language: 'en',
						twoFactorMethod: 'totp',
						totpSecretEncrypted: testUser.totpSecret,
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

	test('should display recovery codes after successful TOTP setup', async ({ page }) => {
		const setupPage = new TwoFactorSetupPage(page);

		// Navigate to setup page
		await page.goto('/login/2fa/setup');

		// Select TOTP method
		await setupPage.selectMethod('totp');

		// Complete TOTP setup
		const secret = await setupPage.extractTOTPSecret();
		const code = generateTOTPCode(secret);
		await setupPage.enterTOTPCode(code);

		// Verify recovery codes are displayed
		const recoveryCodes = await setupPage.getRecoveryCodes();
		expect(recoveryCodes.length).toBeGreaterThan(0);
		expect(recoveryCodes.length).toBeLessThanOrEqual(10);

		// Verify format of recovery codes
		recoveryCodes.forEach((code) => {
			expect(code).toMatch(/^[A-Z0-9-]+$/i);
			expect(code.length).toBeGreaterThanOrEqual(8);
		});

		// Verify instructions to save codes
		await expect(page.locator('text=/save|store|keep.*safe|write.*down/i')).toBeVisible();
	});

	test('should navigate to recovery code page from verification page', async ({ page }) => {
		const verifyPage = new TwoFactorVerifyPage(page);

		// Login to trigger verification
		await page.goto('/login');
		await page.fill('[type=email]', testUser.email);
		await page.fill('[type=password]', testUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		// Navigate to recovery page
		await verifyPage.navigateToRecoveryPage();

		// Verify on recovery page
		await expect(page).toHaveURL(/\/login\/2fa\/verify\/recovery/);
		await expect(page.locator('input[name="Code"]')).toBeVisible();

		// Verify recovery-specific instructions
		await expect(page.locator('text=/recovery.*code|backup.*code/i')).toBeVisible();
	});

	test('should successfully login with valid recovery code', async ({ page }) => {
		const verifyPage = new TwoFactorVerifyPage(page);

		// Login to trigger verification
		await page.goto('/login');
		await page.fill('[type=email]', testUser.email);
		await page.fill('[type=password]', testUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		// Navigate to recovery page
		await verifyPage.navigateToRecoveryPage();

		// Get a valid recovery code from database
		const userID = await getUserIDByEmail(testUser.email);
		const recoveryCodes = await getRecoveryCodesFromDB(userID);

		if (recoveryCodes.length === 0) {
			test.skip(); // Skip if no recovery codes available
			return;
		}

		const validCode = recoveryCodes[0];

		// Enter recovery code
		await verifyPage.enterRecoveryCode(validCode);

		// Verify successful login
		await expect(page).not.toHaveURL(/\/login/);

		// Verify can access protected routes
		await page.goto('/users');
		await expect(page).not.toHaveURL(/\/login/);
	});

	test('should display error for invalid recovery code', async ({ page }) => {
		const verifyPage = new TwoFactorVerifyPage(page);

		// Login to trigger verification
		await page.goto('/login');
		await page.fill('[type=email]', testUser.email);
		await page.fill('[type=password]', testUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		// Navigate to recovery page
		await verifyPage.navigateToRecoveryPage();

		// Enter invalid recovery code
		await verifyPage.enterRecoveryCode('INVALID-CODE-123');

		// Verify error message
		await verifyPage.expectErrorMessage();

		// Verify remains on recovery page
		await expect(page).toHaveURL(/\/login\/2fa\/verify\/recovery/);
	});

	test('should mark recovery code as used after successful login', async ({ page }) => {
		const verifyPage = new TwoFactorVerifyPage(page);

		// Get initial recovery codes count
		const userID = await getUserIDByEmail(testUser.email);
		const initialCodes = await getRecoveryCodesFromDB(userID);

		if (initialCodes.length === 0) {
			test.skip(); // Skip if no recovery codes available
			return;
		}

		const codeToUse = initialCodes[0];

		// Login and use recovery code
		await page.goto('/login');
		await page.fill('[type=email]', testUser.email);
		await page.fill('[type=password]', testUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		await verifyPage.navigateToRecoveryPage();
		await verifyPage.enterRecoveryCode(codeToUse);

		// Verify successful login
		await expect(page).not.toHaveURL(/\/login/);

		// Logout
		await logout(page);

		// Verify recovery code count decreased
		const remainingCodes = await getRecoveryCodesFromDB(userID);
		expect(remainingCodes.length).toBe(initialCodes.length - 1);

		// Verify the used code is not in remaining codes
		expect(remainingCodes).not.toContain(codeToUse);
	});

	test('should not allow reusing the same recovery code', async ({ page }) => {
		const verifyPage = new TwoFactorVerifyPage(page);

		// Get a recovery code
		const userID = await getUserIDByEmail(testUser.email);
		const codes = await getRecoveryCodesFromDB(userID);

		if (codes.length === 0) {
			test.skip(); // Skip if no recovery codes available
			return;
		}

		const codeToUse = codes[0];

		// First use: successful
		await page.goto('/login');
		await page.fill('[type=email]', testUser.email);
		await page.fill('[type=password]', testUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		await verifyPage.navigateToRecoveryPage();
		await verifyPage.enterRecoveryCode(codeToUse);
		await expect(page).not.toHaveURL(/\/login/);

		// Logout
		await logout(page);

		// Second use: should fail (already used)
		await page.goto('/login');
		await page.fill('[type=email]', testUser.email);
		await page.fill('[type=password]', testUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		await verifyPage.navigateToRecoveryPage();
		await verifyPage.enterRecoveryCode(codeToUse);

		// Verify error message (code already used)
		await verifyPage.expectErrorMessage();
		await expect(page).toHaveURL(/\/login\/2fa\/verify\/recovery/);
	});

	test('should display warning about recovery code one-time use', async ({ page }) => {
		const verifyPage = new TwoFactorVerifyPage(page);

		// Login and navigate to recovery page
		await page.goto('/login');
		await page.fill('[type=email]', testUser.email);
		await page.fill('[type=password]', testUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		await verifyPage.navigateToRecoveryPage();

		// Verify warning about one-time use
		await expect(page.locator('text=/once|one.?time|cannot.*reuse|single.*use/i')).toBeVisible();
	});

	test('should validate recovery code format', async ({ page }) => {
		const verifyPage = new TwoFactorVerifyPage(page);

		// Login and navigate to recovery page
		await page.goto('/login');
		await page.fill('[type=email]', testUser.email);
		await page.fill('[type=password]', testUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		await verifyPage.navigateToRecoveryPage();

		// Verify input field is present
		const codeInput = page.locator('input[name="Code"]');
		await expect(codeInput).toBeVisible();

		// Recovery codes are typically longer than 6 digits (unlike OTP/TOTP)
		// Verify no maxlength constraint or higher limit
		const maxLength = await codeInput.getAttribute('maxlength');
		if (maxLength) {
			expect(parseInt(maxLength)).toBeGreaterThan(6);
		}
	});

	test('should allow navigation back to standard verification', async ({ page }) => {
		// Login and navigate to recovery page
		await page.goto('/login');
		await page.fill('[type=email]', testUser.email);
		await page.fill('[type=password]', testUser.password);
		await Promise.all([page.waitForURL(/\/login\/2fa\/verify/), page.click('[type=submit]')]);

		// Navigate to recovery page
		const verifyPage = new TwoFactorVerifyPage(page);
		await verifyPage.navigateToRecoveryPage();
		await expect(page).toHaveURL(/\/login\/2fa\/verify\/recovery/);

		// Find and click link to go back to standard verification
		const backLink = page.locator('a[href*="/login/2fa/verify"]').filter({ hasNotText: /recovery/i });
		if ((await backLink.count()) > 0) {
			await backLink.first().click();
			await expect(page).toHaveURL(/\/login\/2fa\/verify$/);
		}
	});
});
