/**
 * Direct SQL access for legacy specs.
 *
 * Supported data setup goes through the application's test endpoints
 * (`/__test__/reset`, `/__test__/populate`, scenario controls) on the
 * worker-owned server. The previous TypeScript truncate/seed helpers were
 * removed: schema and baseline lifecycle are Go-owned (cmd/testenv-baseline).
 *
 * `withDatabase` remains for the few specs that still prepare rows with SQL.
 * Under worker isolation it connects to the database owned by this worker's
 * environment (named by the run's database prefix and environment ID), never
 * to a shared database. Replacing these calls with typed scenarios is tracked
 * separately.
 */
import { Pool, PoolClient } from 'pg';
import { getEnvironmentInfo } from '../playwright.config';
import { currentWorkerEnvironment } from './environment';
import { isolationMode } from './isolation';

/**
 * Get environment configuration info for debugging
 */
export function getEnvInfo() {
	return getEnvironmentInfo();
}

function workerDatabase(): string {
	const environment = currentWorkerEnvironment();
	if (environment) {
		return environment.databasePrefix + environment.environmentId.replace(/-/g, '');
	}
	if (isolationMode() === 'worker') {
		throw new Error('withDatabase requires the worker environment fixture; import `test` from fixtures/test');
	}
	return getEnvironmentInfo().dbConfig.database;
}

export async function withDatabase<T>(callback: (client: PoolClient) => Promise<T>): Promise<T> {
	const env = getEnvironmentInfo().dbConfig;
	const pool = new Pool({
		user: env.user,
		host: env.host,
		port: env.port,
		database: workerDatabase(),
		password: process.env.DB_PASSWORD || 'postgres',
	});
	const client = await pool.connect();
	try {
		return await callback(client);
	} finally {
		client.release();
		await pool.end();
	}
}
