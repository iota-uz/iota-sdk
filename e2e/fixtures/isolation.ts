/**
 * Run-level state of the isolated E2E lifecycle.
 *
 * Global setup builds one sealed, fingerprinted baseline database through the
 * Go testenv-baseline command and records where it lives. Each Playwright
 * worker then asks its own `cmd/testenv` driver for a dedicated database
 * clone and application process (see environment.ts). Credentials never pass
 * through this state: the control token stays in a private 0600 file.
 */
import * as fs from 'fs';
import * as path from 'path';

export type Isolation = 'worker' | 'shared';

export interface IsolationState {
	runId: string;
	database: string;
	databasePrefix: string;
	manifest: { schemaFingerprint: string; baselineFingerprint: string; buildRevision: string };
	tokenFile: string;
	artifacts: string;
	privateDir: string;
}

export const STATE_ENV = 'E2E_TESTENV_STATE';
export const repoRoot = path.resolve(__dirname, '..', '..');

/**
 * `worker` (default, locally and in CI): every worker owns a database clone
 * and application process. `shared`: legacy single server at BASE_URL with a
 * dedicated database; only valid with one worker.
 */
export function isolationMode(): Isolation {
	const value = process.env.E2E_ISOLATION ?? 'worker';
	if (value !== 'worker' && value !== 'shared') {
		throw new Error(`E2E_ISOLATION must be "worker" or "shared", got "${value}"`);
	}
	return value;
}

export function readIsolationState(): IsolationState | undefined {
	const file = process.env[STATE_ENV];
	if (!file) return undefined;
	return JSON.parse(fs.readFileSync(file, 'utf8')) as IsolationState;
}
