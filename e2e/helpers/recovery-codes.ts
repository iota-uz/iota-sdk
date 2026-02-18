/**
 * Recovery Codes Helper for E2E Tests
 *
 * Provides utilities for generating and storing recovery codes for testing.
 * Recovery codes are generated in plaintext, hashed with bcrypt, and stored in the database.
 * The plaintext versions are returned for use in tests.
 */

import { Pool } from 'pg';
import * as bcrypt from 'bcryptjs';

/**
 * Database configuration for E2E tests
 */
function getDBConfig() {
	const isCI = process.env.CI === 'true' || process.env.GITHUB_ACTIONS === 'true';
	const defaultPort = isCI ? 5432 : 5438;

	return {
		user: process.env.DB_USER || 'postgres',
		password: process.env.DB_PASSWORD || 'postgres',
		host: process.env.DB_HOST || (isCI ? 'postgres' : 'localhost'),
		port: parseInt(process.env.DB_PORT || String(defaultPort)),
		database: process.env.DB_NAME || 'iota_erp_e2e',
	};
}

/**
 * Generate a single recovery code in the format XXXX-XXXX-XXXX
 * Uses base32 alphabet (matching backend implementation)
 *
 * @returns Recovery code in format XXXX-XXXX-XXXX
 */
export function generateRecoveryCode(): string {
	// Base32 alphabet (excluding 0, 1, 8, 9 to avoid confusion with letters)
	const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
	const segments: string[] = [];

	// Generate 3 segments of 4 characters each
	for (let i = 0; i < 3; i++) {
		let segment = '';
		for (let j = 0; j < 4; j++) {
			const randomIndex = Math.floor(Math.random() * chars.length);
			segment += chars[randomIndex];
		}
		segments.push(segment);
	}

	return segments.join('-');
}

/**
 * Normalize a recovery code for hashing (remove dashes, uppercase)
 *
 * @param code - Recovery code with or without dashes
 * @returns Normalized code (no dashes, uppercase)
 */
function normalizeRecoveryCode(code: string): string {
	return code.replace(/-/g, '').toUpperCase();
}

/**
 * Create test recovery codes for a user
 * Generates plaintext codes, hashes them with bcrypt, stores in database, and returns plaintexts
 *
 * @param userID - User ID to create recovery codes for
 * @param count - Number of recovery codes to generate (default: 10)
 * @returns Array of plaintext recovery codes
 */
export async function createTestRecoveryCodesForUser(
	userID: number,
	count: number = 10
): Promise<string[]> {
	const config = getDBConfig();
	const pool = new Pool(config);

	try {
		const client = await pool.connect();
		try {
			// Get tenant ID for the user
			const userResult = await client.query('SELECT tenant_id FROM users WHERE id = $1', [
				userID,
			]);

			if (userResult.rows.length === 0) {
				throw new Error(`User not found: ${userID}`);
			}

			const tenantID = userResult.rows[0].tenant_id;
			const codes: string[] = [];

			// Generate and store recovery codes
			for (let i = 0; i < count; i++) {
				// Generate plaintext code
				const plaintext = generateRecoveryCode();

				// Normalize (remove dashes, uppercase) before hashing
				const normalized = normalizeRecoveryCode(plaintext);

				// Hash with bcrypt (cost factor 10, matching backend)
				const hash = await bcrypt.hash(normalized, 10);

				// Insert into database
				await client.query(
					'INSERT INTO recovery_codes (user_id, code_hash, tenant_id) VALUES ($1, $2, $3)',
					[userID, hash, tenantID]
				);

				// Store plaintext for return
				codes.push(plaintext);
			}

			return codes;
		} finally {
			client.release();
		}
	} finally {
		await pool.end();
	}
}

/**
 * Get count of unused recovery codes for a user
 *
 * @param userID - User ID
 * @returns Count of unused recovery codes
 */
export async function getUnusedRecoveryCodeCount(userID: number): Promise<number> {
	const config = getDBConfig();
	const pool = new Pool(config);

	try {
		const client = await pool.connect();
		try {
			const result = await client.query(
				`SELECT COUNT(*) as count FROM recovery_codes
				 WHERE user_id = $1 AND used_at IS NULL`,
				[userID]
			);

			return parseInt(result.rows[0].count);
		} finally {
			client.release();
		}
	} finally {
		await pool.end();
	}
}

/**
 * Delete all recovery codes for a user
 *
 * @param userID - User ID
 */
export async function deleteAllRecoveryCodesForUser(userID: number): Promise<void> {
	const config = getDBConfig();
	const pool = new Pool(config);

	try {
		const client = await pool.connect();
		try {
			await client.query('DELETE FROM recovery_codes WHERE user_id = $1', [userID]);
		} finally {
			client.release();
		}
	} finally {
		await pool.end();
	}
}
