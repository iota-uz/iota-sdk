/**
 * Page Object for Two-Factor Authentication Setup Flow
 */

import { Page, expect } from '@playwright/test';
import { extractSecretFromOTPAuthURL, generateTOTPCode } from '../../helpers/totp';

export class TwoFactorSetupPage {
	constructor(private page: Page) {}

	/**
	 * Navigate to 2FA setup page
	 *
	 * @param nextURL - Optional next URL parameter
	 */
	async navigateTo(nextURL: string = '/') {
		const url = `/login/2fa/setup${nextURL ? `?next=${encodeURIComponent(nextURL)}` : ''}`;
		await this.page.goto(url);
		await expect(this.page).toHaveURL(new RegExp('/login/2fa/setup'));
	}

	/**
	 * Select 2FA method from method choice page
	 *
	 * @param method - The 2FA method ('totp', 'sms', or 'email')
	 */
	async selectMethod(method: 'totp' | 'sms' | 'email') {
		// The radios are intentionally visually hidden (sr-only), so force check is required.
		await this.page.locator(`input[name="Method"][value="${method}"]`).check({ force: true });

		// Submit the form
		await this.page.click('button[type="submit"]');

		// Wait for navigation after form submission
		if (method === 'totp') {
			await expect(this.page).toHaveURL(/\/login\/2fa\/setup\/totp/);
		} else {
			await expect(this.page).toHaveURL(/\/login\/2fa\/setup\/otp/);
		}
	}

	/**
	 * Get QR code data URL from TOTP setup page
	 *
	 * @returns The QR code data URL
	 */
	async getQRCodeURL(): Promise<string> {
		const qrImage = this.page.locator('img[alt="QR Code"]');
		await expect(qrImage).toBeVisible();

		const src = await qrImage.getAttribute('src');
		if (!src) {
			throw new Error('QR code image has no src attribute');
		}

		return src;
	}

	/**
	 * Extract TOTP secret from QR code
	 * Since we can't decode QR from data URL easily, we extract from page attributes
	 *
	 * @returns The TOTP secret
	 */
	async extractTOTPSecret(): Promise<string> {
		const otpAuthInput = this.page.locator('input[name="OTPAuthURL"]');
		if (await otpAuthInput.count()) {
			const otpAuthURL = await otpAuthInput.first().inputValue();
			if (otpAuthURL) {
				return extractSecretFromOTPAuthURL(otpAuthURL);
			}
		}

		// Get the QR code data URL
		const qrCodeURL = await this.getQRCodeURL();

		// In a real implementation, we'd decode the QR code
		// For testing, we can extract the otpauth URL from page context or API
		// Here we'll use a workaround: generate the secret from visible URL params

		// Alternative: Look for otpauth:// URL in page source or attributes
		const pageContent = await this.page.content();

		// Try to extract otpauth:// URL pattern
		const otpauthMatch = pageContent.match(/otpauth:\/\/totp\/[^"'\s]+/);

		if (otpauthMatch) {
			const otpauthURL = otpauthMatch[0].replace(/&amp;/g, '&');
			return extractSecretFromOTPAuthURL(otpauthURL);
		}

		// If not found in HTML, we need another approach
		// For E2E tests, we might need to add a data attribute with the secret
		throw new Error('Could not extract TOTP secret from page');
	}

	/**
	 * Enter TOTP verification code
	 *
	 * @param code - The 6-digit TOTP code
	 */
	async enterTOTPCode(code: string) {
		await this.page.fill('input[name="Code"]', code);
		await this.page.click('button[type="submit"]');
	}

	/**
	 * Complete TOTP setup with auto-generated code
	 * Extracts secret and generates valid code automatically
	 */
	async completeTOTPSetup(): Promise<string[]> {
		const secret = await this.extractTOTPSecret();
		const code = generateTOTPCode(secret);

		await this.enterTOTPCode(code);

		// Wait for navigation or recovery codes page
		await this.page.waitForURL(/\/login\/2fa\/setup|\//, { timeout: 10000 });

		// Extract recovery codes if displayed
		return await this.getRecoveryCodes();
	}

	/**
	 * Enter OTP verification code
	 *
	 * @param code - The 6-digit OTP code
	 */
	async enterOTPCode(code: string) {
		await this.page.fill('input[name="Code"]', code);
		await this.page.click('button[type="submit"]');
	}

	/**
	 * Complete OTP setup with provided code
	 *
	 * @param code - The OTP code received via SMS/Email
	 */
	async completeOTPSetup(code: string) {
		await this.enterOTPCode(code);

		// Wait for success redirect
		await this.page.waitForURL(/\//, { timeout: 10000 });
	}

	/**
	 * Get recovery codes from setup complete page
	 *
	 * @returns Array of recovery codes
	 */
	async getRecoveryCodes(): Promise<string[]> {
		// Wait for recovery codes to be visible
		const recoveryCodesContainer = this.page.locator('.recovery-code, [data-recovery-code], code, pre');

		const count = await recoveryCodesContainer.count();

		if (count === 0) {
			// Recovery codes might not be displayed for OTP methods
			return [];
		}

		const codes: string[] = [];

		for (let i = 0; i < count; i++) {
			const text = await recoveryCodesContainer.nth(i).textContent();
			if (text && text.trim()) {
				codes.push(text.trim());
			}
		}

		return codes;
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
	 * Click "Resend Code" button (for OTP methods)
	 */
	async resendCode() {
		await this.page.click('button:has-text("Resend"), button[type="submit"]:has-text("Resend")');

		// Wait for success message
		await this.expectSuccessMessage();
	}

	/**
	 * Verify QR code is displayed
	 */
	async expectQRCodeVisible() {
		const qrImage = this.page.locator('img[alt="QR Code"]');
		await expect(qrImage).toBeVisible();
	}
}
