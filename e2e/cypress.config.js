const util = require("node:util");
const cp = require("node:child_process");
const { defineConfig } = require("cypress");
const { Pool } = require("pg");

const exec = util.promisify(cp.exec);
const { env } = process;
const [DB_USER, DB_PASSWORD, DB_HOST, DB_PORT, DB_NAME] = [
	env.DB_USER ?? "postgres",
	env.DB_PASSWORD ?? "postgres",
	env.DB_HOST ?? "localhost",
	env.DB_PORT ?? 5432,
	env.DB_NAME ?? "iota_erp",
];
const pool = new Pool({
	connectionString: `postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}`,
});

async function resetDatabase() {
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
	}
	return null;
}

async function seedDatabase() {
	await exec("cd .. && go run cmd/seed/main.go");
	return null;
}

module.exports = defineConfig({
	e2e: {
		defaultCommandTimeout: 15000,
		requestTimeout: 20000,
		responseTimeout: 20000,
		pageLoadTimeout: 60000,
		setupNodeEvents(on, config) {
			on("task", {
				resetDatabase,
				seedDatabase,
			});
		},
	},
});
