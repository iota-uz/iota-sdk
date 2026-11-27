import { execFileSync } from 'child_process';
import { randomBytes, randomUUID } from 'crypto';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { STATE_ENV, isolationMode, repoRoot, type IsolationState } from './fixtures/isolation';

function revision(): string {
	if (process.env.E2E_BUILD_REVISION) return process.env.E2E_BUILD_REVISION;
	const head = execFileSync('git', ['rev-parse', 'HEAD'], { cwd: repoRoot, encoding: 'utf8' }).trim();
	const dirty = execFileSync('git', ['status', '--porcelain'], { cwd: repoRoot, encoding: 'utf8' }).trim() !== '';
	return dirty ? `${head}-dirty-${randomUUID()}` : head;
}

function buildBinaries(directory: string): NonNullable<IsolationState['binaries']> {
	const suffix = process.platform === 'win32' ? '.exe' : '';
	const binaries = { testenv: path.join(directory, `testenv${suffix}`), server: path.join(directory, `server${suffix}`) };
	const started = Date.now();
	const build = (args: string[]) => execFileSync('go', ['build', ...args], { cwd: repoRoot, env: process.env, stdio: ['ignore', 'inherit', 'inherit'] });
	build(['-o', binaries.testenv, './cmd/testenv']);
	build(['-tags', 'dev', '-o', binaries.server, './cmd/server']);
	console.log(`[e2e] testenv driver and server built in ${Date.now() - started}ms`);
	return binaries;
}

/**
 * Builds the run's sealed baseline once (migrations + E2E seed, owned by Go)
 * and returns a teardown that drops it after every worker environment stopped.
 * Nothing here truncates or resets an existing database.
 */
export default async function globalSetup(): Promise<(() => Promise<void>) | void> {
	if (isolationMode() !== 'worker') return;

	const runId = randomUUID();
	const short = runId.replace(/-/g, '').slice(0, 10);
	const privateDir = fs.mkdtempSync(path.join(os.tmpdir(), 'iota-e2e-'));
	fs.chmodSync(privateDir, 0o700);

	// Cleanup is registered before anything is built, so a failed build or
	// baseline leaves neither the private directory (token, binaries) nor a
	// sealed baseline behind. Playwright skips the returned teardown when a
	// run is interrupted (SIGINT, fatal error), so this also runs
	// synchronously when the runner exits. Worker environments clean
	// themselves up: their driver stops its process group and drops its
	// clone on stdin EOF.
	let baseline: string | undefined;
	let cleaned = false;
	const cleanup = () => {
		if (cleaned) return;
		cleaned = true;
		try {
			if (baseline) {
				execFileSync('go', ['run', './cmd/testenv-baseline', '--drop', baseline], {
					cwd: repoRoot,
					env: process.env,
					stdio: ['ignore', 'inherit', 'inherit'],
				});
			}
		} finally {
			fs.rmSync(privateDir, { recursive: true, force: true });
			delete process.env[STATE_ENV];
		}
	};
	process.once('exit', () => {
		try {
			cleanup();
		} catch (error) {
			console.error(`[e2e] failed to drop baseline ${baseline}:`, error);
		}
	});

	try {
		const started = Date.now();
		const output = execFileSync('go', ['run', './cmd/testenv-baseline', '--name', `e2e_base_${short}`, '--revision', revision()], {
			cwd: repoRoot,
			env: process.env,
			encoding: 'utf8',
			stdio: ['ignore', 'pipe', 'inherit'],
			maxBuffer: 16 * 1024 * 1024,
		});
		const built = JSON.parse(output.trim().split('\n').pop() ?? '{}');
		baseline = built.database;
		console.log(`[e2e] sealed baseline ${built.database} built in ${Date.now() - started}ms`);

		const tokenFile = path.join(privateDir, 'control-token');
		fs.writeFileSync(tokenFile, randomBytes(32).toString('hex'), { mode: 0o600 });

		// Every worker starts the driver and the application process. Compile both
		// once here: a cold `go run` of the server in each worker at once outlasts
		// the driver's start deadline and fails the first test of every worker.
		const binaries = buildBinaries(path.join(privateDir, 'bin'));

		const state: IsolationState = {
			runId,
			database: built.database,
			databasePrefix: `e2e_${short}_`,
			manifest: built.manifest,
			tokenFile,
			artifacts: path.join(__dirname, 'test-results', 'environments', runId),
			privateDir,
			binaries,
		};
		const stateFile = path.join(privateDir, 'state.json');
		fs.writeFileSync(stateFile, JSON.stringify(state), { mode: 0o600 });
		process.env[STATE_ENV] = stateFile;
	} catch (error) {
		cleanup();
		throw error;
	}
	return async () => cleanup();
}
