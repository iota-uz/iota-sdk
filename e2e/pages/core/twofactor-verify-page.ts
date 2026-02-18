/**
 * Page Object for Two-Factor Authentication Verification Flow
 */

import { Page, expect } from '@playwright/test';
import { generateTOTPCode } from '../../helpers/totp';

export class TwoFactorVerifyPage {
	constructor(private page: Page) {}

	/**
	 * Navigate to 2FA verification page
	 *
	 * @param nextURL - Optional next URL parameter
	 */
	async navigateTo(nextURL: string = '/') {
		const url = `/login/2fa/verify${nextURL ? `?next=${encodeURIComponent(nextURL)}` : ''}`;
		await this.page.goto(url);
		await expect(this.page).toHaveURL(new RegExp('/login/2fa/verify'));
	}

	/**
	 * Enter verification code (TOTP or OTP)
	 *
	 * @param code - The 6-digit verification code
	 */
	async enterVerificationCode(code: string) {
		await this.page.fill('input[name="Code"]', code);
		await this.page.click('button[type="submit"]');
	}

	/**
	 * Complete TOTP verification with provided secret
	 *
	 * @param secret - The TOTP secret key
	 */
	async verifyWithTOTP(secret: string) {
		const code = generateTOTPCode(secret);
		await this.enterVerificationCode(code);

		// Wait for successful redirect
		await this.page.waitForURL(/\//, { timeout: 10000 });
	}

	/**
	 * Complete OTP verification with provided code
	 *
	 * @param code - The OTP code received via SMS/Email
	 */
	async verifyWithOTP(code: string) {
		await this.enterVerificationCode(code);

		// Wait for successful redirect
		await this.page.waitForURL(/\//, { timeout: 10000 });
	}

	/**
	 * Navigate to recovery code page
	 */
	async navigateToRecoveryPage() {
		const recoveryAction = this.page.locator(
			'a[href*="/login/2fa/verify/recovery"], a:has-text("recovery"), button:has-text("recovery"), [href*="recovery"]'
		);

		if ((await recoveryAction.count()) > 0) {
			await recoveryAction.first().click();
		} else {
			await this.page.goto('/login/2fa/verify/recovery');
		}

		// Wait for navigation to recovery page
		await expect(this.page).toHaveURL(/\/login\/2fa\/verify\/recovery/);
	}

	/**
	 * Enter recovery code
	 *
	 * @param code - The recovery code
	 */
	async enterRecoveryCode(code: string) {
		await this.page.fill('input[name="Code"]', code);
		await this.page.click('button[type="submit"]');
	}

	/**
	 * Complete verification with recovery code
	 *
	 * @param code - The recovery code
	 */
	async verifyWithRecoveryCode(code: string) {
		await this.enterRecoveryCode(code);

		// Wait for successful redirect
		await this.page.waitForURL(/\//, { timeout: 10000 });
	}

	/**
	 * Click "Resend Code" button (for OTP methods)
	 */
	async resendCode() {
		const resendButton = this.page.locator('button:has-text("Resend"), button[type="submit"]:has-text("Resend")');
		await expect(resendButton).toBeVisible();
		await resendButton.click();

		// Wait for success message
		await this.expectSuccessMessage();
	}

	/**
	 * Verify error message is displayed
	 *
	 * @param expectedError - Expected error message text (partial match)
	 */
	async expectErrorMessage(expectedError?: string) {
		const errorLocator = this.page.locator('[data-flash="error"], .error-message, .bg-red-100, .text-red-600');
		await expect(errorLocator.first()).toBeVisible({ timeout: 5000 });

		if (expectedError) {
			await expect(errorLocator.first()).toContainText(expectedError);
		}
	}

	/**
	 * Verify success message is displayed
	 *
	 * @param expectedMessage - Expected success message text (partial match)
	 */
	async expectSuccessMessage(expectedMessage?: string) {
		const successLocator = this.page.locator(
			'[data-flash="success"], .success-message, .bg-green-100, .text-green-600'
		);
		await expect(successLocator.first()).toBeVisible({ timeout: 5000 });

		if (expectedMessage) {
			await expect(successLocator.first()).toContainText(expectedMessage);
		}
	}

	/**
	 * Verify verification page displays correct method type
	 *
	 * @param method - Expected method type ('totp', 'sms', 'email')
	 */
	async expectMethodType(method: 'totp' | 'sms' | 'email') {
		const heading = this.page.locator('h1, h2').first();
		await expect(heading).toBeVisible();

		// Verify method-specific heading text
		if (method === 'totp') {
			await expect(heading).toContainText(/authenticator|code/i);
		} else if (method === 'sms') {
			await expect(heading).toContainText(/phone|sms/i);
		} else if (method === 'email') {
			await expect(heading).toContainText(/email/i);
		}
	}

	/**
	 * Verify resend button is visible (SMS/Email only)
	 */
	async expectResendButtonVisible() {
		const resendButton = this.page.locator('button:has-text("Resend"), button[type="submit"]:has-text("Resend")');
		await expect(resendButton).toBeVisible();
	}

	/**
	 * Verify recovery link is visible
	 */
	async expectRecoveryLinkVisible() {
		const recoveryAction = this.page.locator(
			'a[href*="/login/2fa/verify/recovery"], a:has-text("recovery"), button:has-text("recovery"), [href*="recovery"]'
		);
		await expect(recoveryAction.first()).toBeVisible();
	}
}
