/**
 * Worker-owned application environments.
 *
 * Each Playwright worker starts its own `cmd/testenv` driver, which clones the
 * run's sealed baseline into a database it owns and starts a dedicated
 * application process on a free port. The worker's `baseURL` (and therefore
 * `page` and `request`) points at that process, so `/__test__/reset` and every
 * other mutation only touch the worker's own database, uploads and session
 * store. A worker restarted after a failure (including a retry) gets a new
 * worker index, a new slot and a fresh physical environment; the previous one
 * is stopped when its worker exits.
 *
 * The fixture only receives the descriptor (base URL, fingerprints, artifact
 * directory). It never opens SQL connections and the descriptor carries no
 * credentials.
 */
import { test as base } from '@playwright/test';
import { createProcessEnvironmentDriver, type DriverDescriptor } from '../../web/testing/src/driver';
import { acquireEnvironmentLease } from '../../web/testing/src/lease';
import { readIsolationState, repoRoot, type IsolationState } from './isolation';

export type WorkerEnvironment = DriverDescriptor & { databasePrefix: string };

let current: WorkerEnvironment | undefined;

/** The environment owned by this worker process, if isolation is enabled. */
export function currentWorkerEnvironment(): WorkerEnvironment | undefined {
	return current;
}

const REQUIRED_CAPABILITIES = ['database.clone', 'process.dedicated'];

function startDriver(state: IsolationState, workerIndex: number) {
	// Global setup compiles both binaries once; `go run` stays the fallback for
	// a state file written without them.
	const driver = state.binaries ? [state.binaries.testenv] : ['go', 'run', './cmd/testenv'];
	const server = state.binaries ? [state.binaries.server] : ['go', 'run', '-tags', 'dev', './cmd/server'];
	const args = [
		...driver.slice(1),
		'--template', state.database,
		'--revision', state.manifest.buildRevision,
		'--schema-fingerprint', state.manifest.schemaFingerprint,
		'--baseline-fingerprint', state.manifest.baselineFingerprint,
		'--artifacts', state.artifacts,
		'--directory', repoRoot,
		'--database-prefix', state.databasePrefix,
		'--sdk-erp',
		'--control-token-file', state.tokenFile,
		'--',
		...server,
	];
	return createProcessEnvironmentDriver({
		command: driver[0],
		args,
		cwd: repoRoot,
		// Every worker runs its own Go server next to a browser. Bound the Go
		// scheduler so N workers do not each start one OS thread per host CPU
		// (on many-core hosts with a thread cgroup limit that exhausts
		// pthread_create before the suite finishes).
		env: { ...process.env, GOMAXPROCS: process.env.E2E_SERVER_GOMAXPROCS ?? process.env.GOMAXPROCS ?? '4' },
		// Without prebuilt binaries a cold `go run` dominates the first startup.
		timeoutMs: 600_000,
		spec: () => ({
			runId: state.runId,
			slot: `worker-${workerIndex}`,
			isolation: 'worker',
			schemaFingerprint: state.manifest.schemaFingerprint,
			baselineFingerprint: state.manifest.baselineFingerprint,
			requiredCapabilities: REQUIRED_CAPABILITIES,
		}),
	});
}

export const test = base.extend<object, { workerEnvironment: WorkerEnvironment | undefined }>({
	workerEnvironment: [
		async ({}, use, workerInfo) => {
			const state = readIsolationState();
			if (!state) {
				await use(undefined);
				return;
			}
			const driver = startDriver(state, workerInfo.workerIndex);
			const started = Date.now();
			const lease = await acquireEnvironmentLease(driver.lifecycle, workerInfo.workerIndex, { closeDriver: driver.close });
			console.log(`[e2e] worker ${workerInfo.workerIndex} environment ${lease.environment.environmentId} ready in ${Date.now() - started}ms`);
			current = { ...lease.environment, databasePrefix: state.databasePrefix };
			try {
				await use(current);
			} finally {
				current = undefined;
				await lease.close();
			}
		},
		{ scope: 'worker', timeout: 600_000 },
	],
	baseURL: async ({ workerEnvironment, baseURL }, use) => {
		await use(workerEnvironment?.baseURL ?? baseURL);
	},
});
