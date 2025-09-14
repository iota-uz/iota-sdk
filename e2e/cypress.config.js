const util = require("node:util");
const cp = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");
const { defineConfig } = require("cypress");
const { Pool } = require("pg");

const exec = util.promisify(cp.exec);

// Smart environment detection
function loadEnvironmentConfig() {
	const envPath = path.join(__dirname, '.env.e2e');

	// Load .env.e2e if it exists (local development)
	if (fs.existsSync(envPath)) {
		require('dotenv').config({ path: envPath });
	}

	// Extract configuration with smart defaults based on environment
	const isCI = process.env.CI === 'true' || process.env.GITHUB_ACTIONS === 'true';
	const defaultPort = isCI ? 5432 : 5438; // CI uses standard port, local uses custom port

	return {
		DB_USER: process.env.DB_USER || "postgres",
		DB_PASSWORD: process.env.DB_PASSWORD || "postgres",
		DB_HOST: process.env.DB_HOST || "localhost",
		DB_PORT: parseInt(process.env.DB_PORT) || defaultPort,
		DB_NAME: process.env.DB_NAME || "iota_erp_e2e",
	};
}

async function resetDatabase() {
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
	return null;
}

async function seedDatabase() {
	await exec("cd .. && go run cmd/command/main.go e2e seed");
	return null;
}

module.exports = defineConfig({
	e2e: {
		defaultCommandTimeout: 15000,
		requestTimeout: 20000,
		responseTimeout: 20000,
		pageLoadTimeout: 60000,
		setupNodeEvents(on, config) {
			// Load environment configuration for logging
			const envConfig = loadEnvironmentConfig();

			// Set base URL from environment configuration
			if (envConfig.CYPRESS_BASE_URL) {
				config.baseUrl = envConfig.CYPRESS_BASE_URL;
				console.log(`Setting baseUrl from .env.e2e: ${config.baseUrl}`);
			} else if (process.env.CYPRESS_BASE_URL) {
				config.baseUrl = process.env.CYPRESS_BASE_URL;
				console.log(`Setting baseUrl from process.env: ${config.baseUrl}`);
			} else {
				console.log('No CYPRESS_BASE_URL found in environment');
				console.log('envConfig keys:', Object.keys(envConfig));
			}

			on("task", {
				resetDatabase,
				seedDatabase,
				// Add a task to get environment info for debugging
				getEnvironmentInfo() {
					return {
						env: process.env.NODE_ENV || 'development',
						isCI: process.env.CI === 'true' || process.env.GITHUB_ACTIONS === 'true',
						dbConfig: {
							host: envConfig.DB_HOST,
							port: envConfig.DB_PORT,
							database: envConfig.DB_NAME,
							user: envConfig.DB_USER,
						},
					};
				},
			});

			// Return the modified configuration
			return config;
		},
	},
});
