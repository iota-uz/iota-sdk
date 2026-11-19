import { defineConfig, devices } from '@playwright/test';
import * as dotenv from 'dotenv';
import * as path from 'path';
import * as fs from 'fs';
import { isolationMode } from './fixtures/isolation';

const isCI = process.env.CI === 'true' || process.env.GITHUB_ACTIONS === 'true';

// Smart environment detection
function loadEnvironmentConfig() {
	const envPath = path.join(__dirname, '.env.e2e');

	// Load .env.e2e only for local development. CI provides its own service
	// ports and credentials, which must not be overwritten by local defaults.
	if (!isCI && fs.existsSync(envPath)) {
		dotenv.config({ path: envPath });
	}

	// Extract configuration with smart defaults based on environment
	const defaultPort = isCI ? 5432 : 5438; // CI uses standard port, local uses custom port

	return {
		DB_USER: process.env.DB_USER || 'postgres',
		DB_PASSWORD: process.env.DB_PASSWORD || 'postgres',
		DB_HOST: process.env.DB_HOST || 'localhost',
		DB_PORT: parseInt(process.env.DB_PORT || String(defaultPort), 10),
		DB_NAME: process.env.DB_NAME || 'iota_erp_e2e',
		BASE_URL: process.env.BASE_URL || 'http://localhost:3201',
	};
}

// Get environment info for debugging
export function getEnvironmentInfo() {
	const envConfig = loadEnvironmentConfig();
	return {
		env: process.env.NODE_ENV || 'development',
		isCI,
		isolation: isolationMode(),
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

	// Builds the run's sealed baseline database; workers clone it.
	globalSetup: './global-setup.ts',

	// Test execution settings. Under worker isolation every worker owns its
	// database and server, so files run in parallel. Tests inside a file stay
	// in order because many suites prepare state in beforeAll. The legacy
	// shared mode targets one dedicated server and must use a single worker.
	fullyParallel: false,
	forbidOnly: isCI,
	retries: isCI ? 2 : 0,
	workers: isolationMode() === 'shared' ? 1 : isCI ? Number(process.env.E2E_WORKERS ?? 3) : undefined,

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

		// Timeout settings (higher in CI to reduce flakiness on slower runners)
		actionTimeout: isCI ? 25000 : 15000,
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
