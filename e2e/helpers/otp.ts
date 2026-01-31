/**
 * OTP Helper for E2E Tests
 *
 * Provides utilities for retrieving OTP codes via test API endpoints
 */

import { APIRequestContext } from '@playwright/test';

/**
 * Retrieve OTP code from test API for a specific identifier (phone/email)
 *
 * @param request - Playwright API request context
 * @param identifier - Phone number or email address
 * @returns The OTP code
 */
export async function getOTPCodeFromDB(
	request: APIRequestContext,
	identifier: string
): Promise<string> {
	const response = await request.get('/__test__/otp', {
		params: { identifier },
		failOnStatusCode: false,
	});

	if (!response.ok()) {
		throw new Error(`Failed to get OTP for ${identifier}: ${response.statusText()}`);
	}

	const body = await response.json();
	return body.code;
}

/**
 * Retrieve OTP code for a user by user ID via test API
 *
 * @param request - Playwright API request context
 * @param userId - The user ID
 * @returns The OTP code
 */
export async function getOTPCodeByUserID(
	request: APIRequestContext,
	userId: number
): Promise<string> {
	const response = await request.get(`/__test__/otp/${userId}`, {
		failOnStatusCode: false,
	});

	if (!response.ok()) {
		throw new Error(`Failed to get OTP for user ${userId}: ${response.statusText()}`);
	}

	const body = await response.json();
	return body.code;
}

/**
 * Generate OTP for a user via test API
 *
 * @param request - Playwright API request context
 * @param userId - User ID
 * @param identifier - Phone number or email address
 * @param channel - Delivery channel ('sms' or 'email')
 * @returns The plaintext OTP code
 */
export async function generateOTPForUser(
	request: APIRequestContext,
	userId: number,
	identifier: string,
	channel: 'sms' | 'email'
): Promise<string> {
	const response = await request.post(`/__test__/otp/${userId}`, {
		data: { identifier, channel },
		failOnStatusCode: false,
	});

	if (!response.ok()) {
		throw new Error(`Failed to generate OTP: ${response.statusText()}`);
	}

	const body = await response.json();
	return body.code;
}

/**
 * Generate a fake/invalid OTP code for error testing
 *
 * @returns 6-digit invalid code
 */
export function generateInvalidOTP(): string {
	return '000000';
}

/**
 * Wait for OTP to be sent (with timeout)
 *
 * @param request - Playwright API request context
 * @param identifier - Phone number or email address
 * @param timeoutMs - Maximum wait time in milliseconds
 * @returns The OTP code
 */
export async function waitForOTP(
	request: APIRequestContext,
	identifier: string,
	timeoutMs: number = 10000
): Promise<string> {
	const startTime = Date.now();
	const pollInterval = 500; // Check every 500ms

	while (Date.now() - startTime < timeoutMs) {
		try {
			const code = await getOTPCodeFromDB(request, identifier);
			if (code) {
				return code;
			}
		} catch (error) {
			// OTP not found yet, continue polling
		}

		await new Promise((resolve) => setTimeout(resolve, pollInterval));
	}

	throw new Error(`OTP not received within ${timeoutMs}ms for ${identifier}`);
}
