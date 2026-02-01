/**
 * TOTP Helper for E2E Tests
 *
 * Provides utilities for generating TOTP codes from secrets
 */

import { authenticator } from 'otplib';

/**
 * Extract TOTP secret from otpauth:// URL
 *
 * @param otpauthURL - The otpauth:// URL from QR code
 * @returns The secret key
 */
export function extractSecretFromOTPAuthURL(otpauthURL: string): string {
	try {
		const url = new URL(otpauthURL);
		const secret = url.searchParams.get('secret');

		if (!secret) {
			throw new Error('No secret found in otpauth URL');
		}

		return secret;
	} catch (error) {
		throw new Error(`Failed to extract secret from OTP auth URL: ${error}`);
	}
}

/**
 * Generate valid TOTP code from secret
 *
 * @param secret - The TOTP secret key
 * @returns 6-digit TOTP code
 */
export function generateTOTPCode(secret: string): string {
	try {
		const code = authenticator.generate(secret);
		return code;
	} catch (error) {
		throw new Error(`Failed to generate TOTP code: ${error}`);
	}
}

/**
 * Extract secret from data URL containing QR code
 * Parses the otpauth:// URL from the QR code image
 *
 * Note: This requires QR code decoding, which is complex.
 * For testing, we can extract from page attributes instead.
 *
 * @param dataURL - data:image/png;base64,... URL
 * @returns The secret key
 */
export function extractSecretFromDataURL(dataURL: string): string {
	// This would require QR code decoding library
	// For now, we'll use a different approach in tests
	throw new Error('Not implemented - use attribute extraction instead');
}

/**
 * Generate an invalid TOTP code (for error testing)
 *
 * @returns 6-digit invalid code
 */
export function generateInvalidTOTPCode(): string {
	return '000000';
}

/**
 * Verify TOTP code validity
 *
 * @param secret - The TOTP secret
 * @param code - The code to verify
 * @returns true if code is valid
 */
export function verifyTOTPCode(secret: string, code: string): boolean {
	try {
		return authenticator.verify({ token: code, secret });
	} catch (error) {
		return false;
	}
}
