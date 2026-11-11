// Starts what the specs run against: the runtime host bundle, then the Go server of the e2e package
// (TestBrowserServer) on a fixed local port, waiting until it has activated the fixture packages and written
// its state file. The returned teardown stops the server and removes the database it created.
import { spawn, spawnSync, type ChildProcess } from 'node:child_process'
import { existsSync, mkdtempSync, readFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const repo = resolve(here, '../../../..')
const e2eDir = resolve(here, '..')
const SERVER_START_TIMEOUT_MS = 8 * 60_000

function run(command: string, args: string[], cwd: string): void {
  const result = spawnSync(command, args, { cwd, stdio: 'inherit', env: process.env })
  if (result.status !== 0) throw new Error(`${command} ${args.join(' ')} failed (${result.status ?? result.signal})`)
}

export default async function globalSetup(): Promise<() => Promise<void>> {
  // Development loop: attach to a server already running (state file of a TestBrowserServer you started).
  if (process.env.IOTA_EXT_BROWSER_ATTACH) {
    process.env.IOTA_EXT_BROWSER_STATE = process.env.IOTA_EXT_BROWSER_ATTACH
    return async () => {}
  }
  const port = process.env.IOTA_EXT_BROWSER_PORT ?? '4290'
  const work = mkdtempSync(join(tmpdir(), 'iota-ext-browser-'))
  const state = join(work, 'state.json')
  const hostJS = join(work, 'runtime-host.js')
  const testBinary = join(work, 'e2e.test')

  run(process.execPath, [join(here, 'build-host.mjs'), hostJS], repo)
  // A compiled test binary instead of `go test`: the server is a long-running test, and stopping it must reach it.
  run('go', ['test', '-c', '-o', testBinary, '.'], e2eDir)

  const server: ChildProcess = spawn(testBinary, ['-test.run', '^TestBrowserServer$', '-test.timeout', '0', '-test.v'], {
    cwd: e2eDir,
    // The server stops when its stdin closes, so it never outlives the runner.
    stdio: ['pipe', 'inherit', 'inherit'],
    env: { ...process.env, IOTA_EXT_BROWSER_STATE: state, IOTA_EXT_BROWSER_HOST_JS: hostJS, IOTA_EXT_BROWSER_PORT: port },
  })
  const exited = new Promise<number | null>((resolveExit) => server.once('exit', (code) => resolveExit(code)))
  let dead = false
  void exited.then(() => { dead = true })

  const deadline = Date.now() + SERVER_START_TIMEOUT_MS
  while (!existsSync(state)) {
    if (dead) throw new Error('the e2e application server exited before it was ready (see its output above)')
    if (Date.now() > deadline) {
      server.kill('SIGTERM')
      throw new Error(`the e2e application server was not ready after ${SERVER_START_TIMEOUT_MS / 1000}s`)
    }
    await new Promise((r) => setTimeout(r, 250))
  }
  process.env.IOTA_EXT_BROWSER_STATE = state
  const { baseURL } = JSON.parse(readFileSync(state, 'utf8')) as { baseURL: string }
  console.log(`extension runtime browser suite: application at ${baseURL}`)

  return async () => {
    server.stdin?.end()
    const stopped = await Promise.race([exited, new Promise<'timeout'>((r) => setTimeout(() => r('timeout'), 30_000))])
    if (stopped === 'timeout') server.kill('SIGKILL')
    rmSync(work, { recursive: true, force: true })
  }
}
