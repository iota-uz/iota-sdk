/**
 * OTP Helper for E2E Tests
 *
 * Provides utilities for retrieving OTP codes from database or test API
 */

import { APIRequestContext } from '@playwright/test';
import { Pool } from 'pg';

/**
 * Database configuration for E2E tests
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
 * Retrieve OTP code from database for a specific destination (phone/email)
 *
 * @param destination - Phone number or email address
 * @returns The OTP code
 */
export async function getOTPCodeFromDB(destination: string): Promise<string> {
	const config = getDBConfig();
	const pool = new Pool(config);

	try {
		const client = await pool.connect();
		try {
			// Query the OTP table for the most recent code sent to this destination
			const result = await client.query(
				`SELECT code FROM otps
				 WHERE destination = $1
				 AND expires_at > NOW()
				 ORDER BY created_at DESC
				 LIMIT 1`,
				[destination]
			);

			if (result.rows.length === 0) {
				throw new Error(`No OTP found for destination: ${destination}`);
			}

			return result.rows[0].code;
		} finally {
			client.release();
		}
	} finally {
		await pool.end();
	}
}

/**
 * Retrieve OTP code for a user by user ID
 *
 * @param userId - The user ID
 * @returns The OTP code
 */
export async function getOTPCodeByUserID(userId: number): Promise<string> {
	const config = getDBConfig();
	const pool = new Pool(config);

	try {
		const client = await pool.connect();
		try {
			// Query the OTP table for the most recent code for this user
			const result = await client.query(
				`SELECT code FROM otps
				 WHERE user_id = $1
				 AND expires_at > NOW()
				 ORDER BY created_at DESC
				 LIMIT 1`,
				[userId]
			);

			if (result.rows.length === 0) {
				throw new Error(`No OTP found for user ID: ${userId}`);
			}

			return result.rows[0].code;
		} finally {
			client.release();
		}
	} finally {
		await pool.end();
	}
}

/**
 * Retrieve OTP code via test API (if available)
 *
 * @param request - Playwright API request context
 * @param destination - Phone number or email address
 * @returns The OTP code
 */
export async function getOTPCodeViaAPI(
	request: APIRequestContext,
	destination: string
): Promise<string> {
	const response = await request.get('/__test__/otp', {
		params: { destination },
		failOnStatusCode: false,
	});

	if (!response.ok()) {
		throw new Error(`Failed to get OTP via API: ${response.statusText()}`);
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
 * @param destination - Phone number or email address
 * @param timeoutMs - Maximum wait time in milliseconds
 * @returns The OTP code
 */
export async function waitForOTP(
	destination: string,
	timeoutMs: number = 10000
): Promise<string> {
	const startTime = Date.now();
	const pollInterval = 500; // Check every 500ms

	while (Date.now() - startTime < timeoutMs) {
		try {
			const code = await getOTPCodeFromDB(destination);
			if (code) {
				return code;
			}
		} catch (error) {
			// OTP not found yet, continue polling
		}

		await new Promise((resolve) => setTimeout(resolve, pollInterval));
	}

	throw new Error(`OTP not received within ${timeoutMs}ms for ${destination}`);
}
