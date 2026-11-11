// End-to-end tests of the shipped author CLI: the real dist layout (`<sdk>/dist/runtime/cli/iota-ext.mjs`), a real
// node process per command, no network except a local server for `kit pull`.
import { spawn, spawnSync } from 'node:child_process'
import { cpSync, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, statSync, symlinkSync, writeFileSync } from 'node:fs'
import { createServer, type Server } from 'node:http'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { gzipSync } from 'node:zlib'

import { afterAll, beforeAll, describe, expect, it } from 'vitest'

const here = dirname(fileURLToPath(import.meta.url))
const runtimeRoot = resolve(here, '..')
const repoRoot = resolve(runtimeRoot, '..', '..')

let sandbox = ''
let cli = ''
const env = { ...process.env, NODE_PATH: join(runtimeRoot, 'node_modules'), NO_COLOR: '1' }

function iota(cwd: string, ...args: string[]): { status: number; out: string; err: string } {
  const r = spawnSync(process.execPath, ['--no-warnings', cli, ...args], { cwd, env, encoding: 'utf8', timeout: 120_000 })
  return { status: r.status ?? -1, out: r.stdout, err: r.stderr }
}

/** Async variant for tests that run a server in this process (a synchronous spawn would block it). */
function iotaAsync(cwd: string, extraEnv: Record<string, string>, ...args: string[]): Promise<{ status: number; out: string; err: string }> {
  return new Promise((done) => {
    const child = spawn(process.execPath, ['--no-warnings', cli, ...args], { cwd, env: { ...env, ...extraEnv } })
    let out = ''
    let err = ''
    child.stdout.on('data', (b: Buffer) => (out += b.toString('utf8')))
    child.stderr.on('data', (b: Buffer) => (err += b.toString('utf8')))
    child.on('close', (code) => done({ status: code ?? -1, out, err }))
  })
}

/** A project directory inside the sandbox whose node_modules sees the installed SDK, as after `npm install`. */
function project(name: string): string {
  const dir = join(sandbox, name)
  mkdirSync(join(dir, 'node_modules', '@iota-uz'), { recursive: true })
  symlinkSync(join(sandbox, 'sdk'), join(dir, 'node_modules', '@iota-uz', 'sdk'), 'dir')
  return dir
}

beforeAll(() => {
  sandbox = mkdtempSync(join(tmpdir(), 'iota-ext-cli-'))
  const sdk = join(sandbox, 'sdk')
  mkdirSync(sdk, { recursive: true })
  cpSync(join(repoRoot, 'web', 'sdk', 'package.json'), join(sdk, 'package.json'))
  const build = spawnSync(process.execPath, ['--no-warnings', join(runtimeRoot, 'scripts', 'build-worker.mjs'), '--emit-sdk', join(sdk, 'dist', 'runtime')], {
    cwd: runtimeRoot,
    encoding: 'utf8',
  })
  if (build.status !== 0) throw new Error(`building the dist failed:\n${build.stdout}\n${build.stderr}`)
  cli = join(sdk, 'dist', 'runtime', 'cli', 'iota-ext.mjs')
}, 240_000)

afterAll(() => {
  if (sandbox !== '') rmSync(sandbox, { recursive: true, force: true })
})

describe('iota-ext init → check → build → test', () => {
  it('scaffolds a package that checks, builds and passes its test offline', () => {
    const dir = project('fresh')
    const init = iota(dir, 'init', 'pkg', '--namespace', 'acme.loyalty')
    expect(init.err).toBe('')
    expect(init.status).toBe(0)
    const pkg = join(dir, 'pkg')
    expect(init.out).toContain('kit generated: @iota-kit/core, @iota-kit/crm')
    for (const f of ['package.json', 'tsconfig.json', 'src/contracts.ts', 'src/server.ts', 'src/client.tsx', 'src/server.test.ts', 'node_modules/@iota-kit/core/index.d.ts']) {
      expect(existsSync(join(pkg, f)), f).toBe(true)
    }
    expect(JSON.parse(readFileSync(join(pkg, 'package.json'), 'utf8')).iota).toMatchObject({ namespace: 'acme.loyalty', contracts: './src/contracts.ts' })
    // The native keys are typed from the snapshot, not guessed.
    expect(readFileSync(join(pkg, 'node_modules/@iota-kit/crm/index.js'), 'utf8')).toContain('{ key: "int64" }')

    const check = iota(pkg, 'check')
    expect(check.err).toBe('')
    expect(check.out).toContain('check passed: acme.loyalty declares')
    expect(check.status).toBe(0)

    const built = iota(pkg, 'build')
    expect(built.status, built.err).toBe(0)
    for (const f of ['.iota/build/manifest.json', '.iota/build/artifacts/contracts.mjs', '.iota/build/artifacts/server.mjs', '.iota/build/artifacts/client/manifest.json']) {
      expect(existsSync(join(pkg, f)), f).toBe(true)
    }
    const manifest = JSON.parse(readFileSync(join(pkg, '.iota/build/manifest.json'), 'utf8')) as { namespace: string; dependencies?: Record<string, string> }
    expect(manifest.namespace).toBe('acme.loyalty')

    const tests = iota(pkg, 'test')
    expect(tests.out).toContain('✓ a stored note is listed back')
    expect(tests.out).toContain('1 test: 1 passed')
    expect(tests.status).toBe(0)
  }, 180_000)

  it('refuses a bad namespace and a non-empty directory with exit code 2', () => {
    const dir = project('usage')
    expect(iota(dir, 'init', 'x', '--namespace', 'Acme').status).toBe(2)
    expect(iota(dir, 'init', 'x', '--namespace', 'acme').err).toContain('one segment')
    mkdirSync(join(dir, 'busy'))
    writeFileSync(join(dir, 'busy', 'f'), 'x')
    const busy = iota(dir, 'init', 'busy', '--namespace', 'acme.x')
    expect(busy.status).toBe(2)
    expect(busy.err).toContain('not empty')
    expect(iota(dir, 'check').status).toBe(2)
    expect(iota(dir, 'frobnicate').status).toBe(2)
  })
})

describe('diagnostics and failing tests are located in the author source', () => {
  const dir = (): string => join(sandbox, 'diag', 'pkg')

  beforeAll(() => {
    const parent = project('diag')
    expect(iota(parent, 'init', 'pkg', '--namespace', 'acme.diag', '--no-client').status).toBe(0)
  })

  it('prints type errors as file:line:col code message and exits 1', () => {
    const file = join(dir(), 'src', 'broken.ts')
    writeFileSync(
      file,
      `import * as crm from "@iota-kit/crm"
import type { Tx } from "@iota-uz/sdk/runtime/server"
import * as c from "./contracts"
declare const tx: Tx
export const a = tx.insert(crm.Client, { firstName: "x" })
export const b = tx.start(c.Note as never, {})
`,
    )
    try {
      const r = iota(dir(), 'check')
      expect(r.status).toBe(1)
      expect(r.err).toMatch(/^src\/broken\.ts:5:\d+ ts\.2345 /m)
      expect(r.err).toContain("'crm.client' belongs to the platform or another package")
      expect(r.err).toContain('published operation (tx.call)')
      expect(r.err).toContain('check failed: ')
      const json = iota(dir(), 'check', '--json')
      expect(json.status).toBe(1)
      expect(JSON.parse(json.out)).toMatchObject({ ok: false, diagnostics: [{ file: 'src/broken.ts', severity: 'error' }] })
      // `test` does not run a package that does not compile.
      const t = iota(dir(), 'test')
      expect(t.status).toBe(1)
      expect(t.err).toContain('tests were not run')
    } finally {
      rmSync(file)
    }
  })

  it('explains tx.start without a key', () => {
    const file = join(dir(), 'src', 'start.ts')
    writeFileSync(
      file,
      `import { workflow, t } from "@iota-uz/sdk/runtime"
import type { Tx } from "@iota-uz/sdk/runtime/server"
const W = workflow("acme.diag.w", { input: t.object({}) })
declare const tx: Tx
export const x = tx.start(W, {})
`,
    )
    try {
      const r = iota(dir(), 'check')
      expect(r.status).toBe(1)
      expect(r.err).toContain('hint: tx.start needs a start key')
    } finally {
      rmSync(file)
    }
  })

  it('reports a failing assertion at the .ts line, with a readable diff and no temp bundle paths', () => {
    const file = join(dir(), 'src', 'fail.test.ts')
    writeFileSync(
      file,
      `import { assert, test } from "@iota-uz/sdk/runtime/testing"

test("deep equal fails", () => {
  assert.deepEqual({ a: [1, 2] }, { a: [1, 3] })
})

test("a thrown error", () => {
  throw new Error("boom")
})
`,
    )
    try {
      const r = iota(dir(), 'test')
      expect(r.status).toBe(1)
      expect(r.out).toContain('✗ deep equal fails')
      expect(r.out).toContain('values differ at a[1]')
      expect(r.out).toMatch(/at src\/fail\.test\.ts:4:\d+/)
      expect(r.out).toMatch(/at src\/fail\.test\.ts:8:\d+/)
      expect(r.out).toContain('2 failed')
      expect(r.out + r.err).not.toMatch(/iota-ext-|test-\d+\.mjs|dist\/runtime/)

      const j = iota(dir(), 'test', '--json')
      const doc = JSON.parse(j.out) as { ok: boolean; failed: number; cases: { name: string; passed: boolean; location?: { file: string; line: number } }[] }
      expect(doc.ok).toBe(false)
      expect(doc.cases.find((c) => c.name === 'deep equal fails')?.location).toMatchObject({ file: 'src/fail.test.ts', line: 4 })

      // A filter runs only matching files.
      const only = iota(dir(), 'test', 'server.test')
      expect(only.status).toBe(0)
      expect(only.out).not.toContain('deep equal fails')
    } finally {
      rmSync(file)
    }
  })

  it('names the kit commands when the kit is missing', () => {
    const parent = project('nokit')
    expect(iota(parent, 'init', 'pkg', '--namespace', 'acme.nokit', '--no-kit', '--no-client').status).toBe(0)
    const pkg = join(parent, 'pkg')
    writeFileSync(join(pkg, 'src', 'uses.ts'), `import * as crm from "@iota-kit/crm"\nexport const x = crm.Client\n`)
    const r = iota(pkg, 'check')
    expect(r.status).toBe(1)
    expect(r.err).toContain('no developer kit is installed')
    expect(r.err).toContain('iota-ext kit generate')
  })
})

describe('kit', () => {
  it('generates from an application snapshot file and refuses two files that declare one namespace', () => {
    const dir = project('kitgen')
    writeFileSync(join(dir, 'package.json'), '{"name":"x","iota":{"namespace":"acme.k","server":"./s.ts"}}')
    const snapshot = {
      schema: 'iota.catalog/1',
      sources: [
        {
          namespace: 'billing',
          resources: [{ id: 'billing.invoice', kind: 'entity', keyType: 'uuid', fields: [{ id: 'number', type: { kind: 'string' } }] }],
        },
      ],
    }
    writeFileSync(join(dir, 'app.json'), JSON.stringify(snapshot))
    const r = iota(dir, 'kit', 'generate', '--catalog', 'app.json')
    expect(r.status, r.err).toBe(0)
    const js = readFileSync(join(dir, 'node_modules/@iota-kit/billing/index.js'), 'utf8')
    expect(js).toContain('nativeEntity("billing.invoice"')
    expect(js).toContain('{ key: "uuid" }')
    expect(iota(dir, 'kit', 'status', '--json').out).toContain('billing')

    writeFileSync(join(dir, 'dup.json'), JSON.stringify(snapshot))
    const dup = iota(dir, 'kit', 'generate', '--catalog', 'app.json', '--catalog', 'dup.json')
    expect(dup.status).toBe(2)
    expect(dup.err).toContain('is in both')
    expect(iota(dir, 'kit', 'generate', '--catalog', 'nope.json').status).toBe(3)
  })

  describe('pull', () => {
    let server: Server
    let base = ''
    let sawAuth: string | undefined

    function tarball(files: Record<string, string>): Buffer {
      const blocks: Buffer[] = []
      for (const [path, text] of Object.entries(files)) {
        const body = Buffer.from(text)
        const header = Buffer.alloc(512)
        header.write(`package/${path}`, 0, 'utf8')
        header.write('0000644\0', 100)
        header.write('0000000\0', 108)
        header.write('0000000\0', 116)
        header.write(`${body.length.toString(8).padStart(11, '0')}\0`, 124)
        header.write('00000000000\0', 136)
        header.write('        ', 148)
        header.write('0', 156)
        header.write('ustar\0', 257)
        header.write('00', 263)
        let sum = 0
        for (const b of header) sum += b
        header.write(`${sum.toString(8).padStart(6, '0')}\0 `, 148)
        blocks.push(header, body, Buffer.alloc((512 - (body.length % 512)) % 512))
      }
      blocks.push(Buffer.alloc(1024))
      return gzipSync(Buffer.concat(blocks))
    }

    beforeAll(async () => {
      server = createServer((req, res) => {
        sawAuth = req.headers.authorization
        if (req.headers.authorization !== 'Bearer s3cret') {
          res.writeHead(401, { 'content-type': 'application/json' }).end(JSON.stringify({ error: { code: 'unauthenticated', message: 'sign in' } }))
          return
        }
        if (req.url === '/_ext/manage/kit') {
          res.writeHead(200, { 'content-type': 'application/json' }).end(JSON.stringify({ hash: 'abc123', modules: ['@iota-kit/granite'] }))
        } else if (req.url?.startsWith('/_ext/manage/kit/modules/granite.tgz?hash=abc123') === true) {
          res.writeHead(200, { 'content-type': 'application/gzip' }).end(
            tarball({ 'index.js': 'export const Deal = 1\n', 'index.d.ts': 'export const Deal: 1\n', 'package.json': '{"name":"@iota-kit/granite"}' }),
          )
        } else {
          res.writeHead(404).end()
        }
      })
      await new Promise<void>((ok) => server.listen(0, '127.0.0.1', ok))
      const address = server.address()
      base = `http://127.0.0.1:${typeof address === 'object' && address !== null ? address.port : 0}`
    })
    afterAll(() => void server.close())

    it('downloads the modules into node_modules/@iota-kit and records the snapshot hash', async () => {
      const dir = project('pull')
      writeFileSync(join(dir, 'package.json'), '{"name":"x","iota":{"namespace":"acme.p","server":"./s.ts"}}')
      const r = await iotaAsync(dir, { IOTA_TOKEN: 's3cret' }, 'kit', 'pull', '--server', base, '--token-env', 'IOTA_TOKEN')
      expect(r.status, r.err).toBe(0)
      expect(sawAuth).toBe('Bearer s3cret')
      expect(readFileSync(join(dir, 'node_modules/@iota-kit/granite/index.js'), 'utf8')).toBe('export const Deal = 1\n')
      expect(JSON.parse(readFileSync(join(dir, 'node_modules/.iota-kit.json'), 'utf8'))).toMatchObject({ hash: 'abc123', modules: ['@iota-kit/granite'], source: { kind: 'server' } })
    })

    it('reports an authentication failure as an environment error (3), never taking the token from argv', async () => {
      const dir = project('pullauth')
      writeFileSync(join(dir, 'package.json'), '{"name":"x"}')
      const denied = await iotaAsync(dir, { IOTA_TOKEN: 'wrong' }, 'kit', 'pull', '--server', base, '--token-env', 'IOTA_TOKEN')
      expect(denied.status).toBe(3)
      expect(denied.err).toContain('401')
      expect(denied.err).toContain('--token-env')
      expect((await iotaAsync(dir, {}, 'kit', 'pull', '--server', base, '--token', 'x')).status).toBe(2)
      expect((await iotaAsync(dir, {}, 'kit', 'pull')).status).toBe(2)
    })
  })
})

describe('the author guide', () => {
  const example = join(repoRoot, 'docs', 'extensions', 'example')

  it('has an example package that checks, builds and passes its tests', () => {
    const dir = project('guide')
    const pkg = join(dir, 'example')
    cpSync(example, pkg, { recursive: true, filter: (src) => !src.includes('node_modules') && !src.includes('.iota') })
    expect(iota(pkg, 'kit', 'generate').status).toBe(0)
    mkdirSync(join(pkg, 'node_modules', '@iota-uz'), { recursive: true })
    symlinkSync(join(sandbox, 'sdk'), join(pkg, 'node_modules', '@iota-uz', 'sdk'), 'dir')

    const check = iota(pkg, 'check')
    expect(check.err).toBe('')
    expect(check.status).toBe(0)
    const built = iota(pkg, 'build')
    expect(built.status, built.err).toBe(0)
    const tests = iota(pkg, 'test')
    expect(tests.out).toMatch(/\d+ tests: \d+ passed$/m)
    expect(tests.out).not.toContain('✗')
    expect(tests.status, tests.out).toBe(0)
  }, 240_000)

  it('quotes the example files verbatim', () => {
    // Every fenced block that starts with a `// example/<file>` marker must consist of lines that occur in that file.
    const guide = readFileSync(join(repoRoot, 'docs', 'extensions', 'authoring.md'), 'utf8')
    const blocks = [...guide.matchAll(/```(?:ts|tsx)\n\/\/ example\/(\S+)\n([\s\S]*?)```/g)]
    expect(blocks.length).toBeGreaterThan(10)
    for (const [, file = '', body = ''] of blocks) {
      const source = readFileSync(join(example, file), 'utf8')
      const have = new Set(source.split('\n').map((l) => l.trim()))
      for (const line of body.split('\n')) {
        const t = line.trim()
        if (t === '' || t === '// …') continue
        expect(have.has(t), `${file}: "${t}" is not in the example`).toBe(true)
      }
    }
  })

  it('ships no stale files in the example', () => {
    const files = readdirSync(join(example, 'src')).sort()
    expect(files).toEqual(['RedemptionScreen.tsx', 'RewardsScreen.tsx', 'client.tsx', 'contracts.ts', 'server.test.ts', 'server.ts'])
    expect(statSync(join(example, 'package.json')).isFile()).toBe(true)
  })
})
