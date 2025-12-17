import { defineConfig, devices } from '@playwright/test';
import { Pool } from 'pg';
import * as dotenv from 'dotenv';
import * as path from 'path';
import * as fs from 'fs';
import { exec as execCallback } from 'child_process';
import { promisify } from 'util';

const exec = promisify(execCallback);

// Smart environment detection
function loadEnvironmentConfig() {
	const envPath = path.join(__dirname, '.env.e2e');

	// Load .env.e2e if it exists (local development)
	if (fs.existsSync(envPath)) {
		dotenv.config({ path: envPath });
	}

	// Extract configuration with smart defaults based on environment
	const isCI = process.env.CI === 'true' || process.env.GITHUB_ACTIONS === 'true';
	const defaultPort = isCI ? 5432 : 5438; // CI uses standard port, local uses custom port

	return {
		DB_USER: process.env.DB_USER || 'postgres',
		DB_PASSWORD: process.env.DB_PASSWORD || 'postgres',
		DB_HOST: process.env.DB_HOST || 'localhost',
		DB_PORT: parseInt(process.env.DB_PORT || String(defaultPort)),
		DB_NAME: process.env.DB_NAME || 'iota_erp_e2e',
		BASE_URL: process.env.BASE_URL || 'http://localhost:3201',
	};
}

// Database reset function
export async function resetDatabase() {
	const config = loadEnvironmentConfig();
	const pool = new Pool({
		connectionString: `postgres://${config.DB_USER}:${config.DB_PASSWORD}@${config.DB_HOST}:${config.DB_PORT}/${config.DB_NAME}`,
	});

	const client = await pool.connect();
	try {
		const res = await client.query(
			"SELECT tablename FROM pg_tables WHERE schemaname = 'public';",
		);
		for (const row of res.rows) {
			await client.query(`TRUNCATE TABLE ${row.tablename} RESTART IDENTITY CASCADE;`);
		}
	} finally {
		client.release();
		await pool.end();
	}
}

// Database seed function
export async function seedDatabase() {
	await exec('cd .. && go run cmd/command/main.go e2e seed');
}

// Get environment info for debugging
export function getEnvironmentInfo() {
	const envConfig = loadEnvironmentConfig();
	return {
		env: process.env.NODE_ENV || 'development',
		isCI: process.env.CI === 'true' || process.env.GITHUB_ACTIONS === 'true',
		dbConfig: {
			host: envConfig.DB_HOST,
			port: envConfig.DB_PORT,
			database: envConfig.DB_NAME,
			user: envConfig.DB_USER,
		},
		baseUrl: envConfig.BASE_URL,
	};
}

// Load environment configuration
const envConfig = loadEnvironmentConfig();

/**
 * See https://playwright.dev/docs/test-configuration.
 */
export default defineConfig({
	testDir: './tests',

	// Maximum time one test can run
	timeout: 60 * 1000,

	// Test execution settings
	fullyParallel: true,
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 2 : 0,
	workers: process.env.CI ? 1 : undefined,

	// Reporter configuration
	reporter: 'html',

	// Shared settings for all projects
	use: {
		// Base URL for navigation
		baseURL: envConfig.BASE_URL,

		// Collect trace when retrying the failed test
		trace: 'on-first-retry',

		// Screenshot on failure
		screenshot: 'only-on-failure',

		// Video on failure
		video: 'retain-on-failure',

		// Timeout settings
		actionTimeout: 15000, // defaultCommandTimeout
		navigationTimeout: 60000, // pageLoadTimeout
	},

	// Configure projects for major browsers
	projects: [
		{
			name: 'chromium',
			use: { ...devices['Desktop Chrome'] },
		},
	],

	// Run your local dev server before starting the tests
	// webServer: {
	//   command: 'npm run start',
	//   url: envConfig.BASE_URL,
	//   reuseExistingServer: !process.env.CI,
	// },
});
