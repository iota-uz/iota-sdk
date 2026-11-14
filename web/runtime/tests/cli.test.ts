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
    for (const f of ['package.json', 'tsconfig.json', 'src/contracts.ts', 'src/server.ts', 'src/client.tsx', 'src/server.test.ts', 'src/NotesScreen.module.css', 'src/css-modules.d.ts', '.iota/kit/core/index.d.ts', '.iota/kit/kit.json']) {
      expect(existsSync(join(pkg, f)), f).toBe(true)
    }
    const written = JSON.parse(readFileSync(join(pkg, 'package.json'), 'utf8')) as { iota: unknown; dependencies: Record<string, string> }
    expect(written.iota).toMatchObject({ namespace: 'acme.loyalty', contracts: './src/contracts.ts' })
    // The SDK dependency is the exact version of the CLI that ran init, never a range.
    const sdkVersion = (JSON.parse(readFileSync(join(sandbox, 'sdk', 'package.json'), 'utf8')) as { version: string }).version
    expect(written.dependencies['@iota-uz/sdk']).toBe(sdkVersion)
    // The kit sits outside node_modules and the editor finds it through tsconfig paths.
    expect(existsSync(join(pkg, 'node_modules/@iota-kit'))).toBe(false)
    expect(JSON.parse(readFileSync(join(pkg, 'tsconfig.json'), 'utf8')).compilerOptions.paths).toEqual({ '@iota-kit/*': ['./.iota/kit/*'] })
    expect(readFileSync(join(pkg, '.gitignore'), 'utf8')).toBe('node_modules\n.iota/build\n')
    // The native keys are typed from the snapshot, not guessed.
    expect(readFileSync(join(pkg, '.iota/kit/crm/index.js'), 'utf8')).toContain('{ key: "int64" }')

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

    // The scaffolded page styles itself with a kit component, Tailwind utilities and a CSS module; the build ships all of it.
    const screen = readFileSync(join(pkg, 'src/NotesScreen.tsx'), 'utf8')
    expect(screen).toContain('from "@iota-uz/sdk/solid-ui"')
    expect(screen).toContain('class="flex flex-col gap-4 p-6"')
    expect(screen).toContain('from "./NotesScreen.module.css"')
    const clientManifest = JSON.parse(readFileSync(join(pkg, '.iota/build/artifacts/client/manifest.json'), 'utf8')) as { css: string[]; shared: string[] }
    expect(clientManifest.shared).toContain('@iota-uz/sdk/solid-ui')
    expect(clientManifest.css).toHaveLength(2)
    expect(clientManifest.css[0]).toBe('client/tailwind.css')
    const utilities = readFileSync(join(pkg, '.iota/build/artifacts', clientManifest.css[0]!), 'utf8')
    expect(utilities).toContain('.gap-4{')
    expect(utilities).not.toContain('box-sizing') // utilities only: the host page has the base
    expect(readFileSync(join(pkg, '.iota/build/artifacts', clientManifest.css[1]!), 'utf8')).toMatch(/\.NotesScreen_[0-9a-f]{8}_list\b/)

    const tests = iota(pkg, 'test')
    expect(tests.out).toContain('✓ a stored note is listed back')
    expect(tests.out).toContain('1 test: 1 passed')
    expect(tests.status).toBe(0)
  }, 180_000)

  describe('the SDK dependency init writes is the spec the SDK was installed with', () => {
    function written(parent: string, args: string[], cwd = parent): string {
      writeFileSync(join(parent, 'package.json'), JSON.stringify({ name: 'host', dependencies: { '@iota-uz/sdk': args[0] } }))
      const init = iota(cwd, 'init', ...args.slice(1), '--namespace', 'acme.spec', '--no-client', '--no-kit')
      expect(init.err).toBe('')
      expect(init.status).toBe(0)
      const target = join(cwd, args[1] ?? 'pkg')
      return (JSON.parse(readFileSync(join(target, 'package.json'), 'utf8')) as { dependencies: Record<string, string> }).dependencies['@iota-uz/sdk']!
    }

    it('rewrites a file: tarball spec relative to the new package', () => {
      const parent = project('spec-file')
      expect(written(parent, ['file:../x.tgz', 'pkg'])).toBe('file:../../x.tgz')
    })

    it('rewrites a link: directory and a bare relative path, and resolves from a nested working directory', () => {
      const parent = project('spec-link')
      expect(written(parent, ['link:../sdk-checkout', 'pkg'])).toBe('link:../../sdk-checkout')
      expect(written(parent, ['../x.tgz', 'pkg2'])).toBe('file:../../x.tgz')
      mkdirSync(join(parent, 'packages'))
      expect(written(parent, ['file:./vendor/sdk.tgz', 'pkg3'], join(parent, 'packages'))).toBe('file:../../vendor/sdk.tgz')
    })

    it('keeps an absolute path, a git ref and a URL as written', () => {
      const parent = project('spec-keep')
      expect(written(parent, ['file:/opt/sdk.tgz', 'a'])).toBe('file:/opt/sdk.tgz')
      expect(written(parent, ['github:iota-uz/iota-sdk#abc123', 'b'])).toBe('github:iota-uz/iota-sdk#abc123')
      expect(written(parent, ['https://example.com/sdk.tgz', 'c'])).toBe('https://example.com/sdk.tgz')
    })

    it('writes the exact installed version for a registry range or tag', () => {
      const parent = project('spec-registry')
      const sdkVersion = (JSON.parse(readFileSync(join(sandbox, 'sdk', 'package.json'), 'utf8')) as { version: string }).version
      expect(written(parent, ['^0.5.0', 'a'])).toBe(sdkVersion)
      expect(written(parent, ['latest', 'b'])).toBe(sdkVersion)
    })
  })

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
      expect(JSON.parse(json.out)).toMatchObject({ ok: false, diagnostics: expect.arrayContaining([expect.objectContaining({ file: 'src/broken.ts', severity: 'error' })]) })
      // `test` does not run a package that does not compile.
      const t = iota(dir(), 'test')
      expect(t.status).toBe(1)
      expect(t.err).toContain('tests were not run')
    } finally {
      rmSync(file)
    }
  })

  it('refuses a global stylesheet in client code with its location and the two allowed forms', () => {
    const parent = project('css')
    expect(iota(parent, 'init', 'pkg', '--namespace', 'acme.css').status).toBe(0)
    const pkg = join(parent, 'pkg')
    writeFileSync(join(pkg, 'src', 'global.css'), 'body { background: red }\n')
    const screen = join(pkg, 'src', 'NotesScreen.tsx')
    const original = readFileSync(screen, 'utf8')
    writeFileSync(screen, original.replace('import * as c from "./contracts"', 'import * as c from "./contracts"\nimport "./global.css"'))
    const r = iota(pkg, 'check')
    expect(r.status).toBe(1)
    expect(r.err).toMatch(/^src\/NotesScreen\.tsx:5:8 graph\.globalCss /m)
    expect(r.err).toContain('shares one page')
    expect(r.err).toContain('Tailwind')
    expect(r.err).toContain('.module.css')
    // The build reports the same, with no artifacts.
    const built = iota(pkg, 'build')
    expect(built.status).toBe(1)
    expect(existsSync(join(pkg, '.iota/build/artifacts/client/manifest.json'))).toBe(false)
    // Restored, and a module stylesheet without its type declaration explains how to declare it.
    writeFileSync(screen, original)
    rmSync(join(pkg, 'src', 'css-modules.d.ts'))
    const untyped = iota(pkg, 'check')
    expect(untyped.status).toBe(1)
    expect(untyped.err).toMatch(/^src\/NotesScreen\.tsx:\d+:\d+ ts\.2307 /m)
    expect(untyped.err).toContain('declare module "*.module.css"')
  }, 120_000)

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

  it('reports an unknown kit module once, naming the kit command, without knock-on type errors', () => {
    const parent = project('unknownkit')
    expect(iota(parent, 'init', 'pkg', '--namespace', 'acme.unknownkit', '--no-client').status).toBe(0)
    const pkg = join(parent, 'pkg')
    writeFileSync(
      join(pkg, 'src', 'uses.ts'),
      `import { implement } from "@iota-uz/sdk/runtime/server"
import * as billing from "@iota-kit/billing"
import * as c from "./contracts"
export const a = implement(billing.Pay, async (tx, input) => {
  const row = await tx.insert(billing.Invoice, { number: input.number })
  return { invoice: row.ref, total: row.total.toFixed(2), note: c.AddNote.id }
})
export const b = implement(billing.Void, (_tx, input) => input.reason.length)
`,
    )
    for (const command of ['check', 'test'] as const) {
      const r = iota(pkg, command)
      expect(r.status).toBe(1)
      expect(r.err).toContain('src/uses.ts:2:')
      expect(r.err).toContain('graph.unknownKit kit module "@iota-kit/billing" is not part of the developer kit this package builds against (installed: @iota-kit/core, @iota-kit/crm)')
      expect(r.err).toContain('iota-ext kit pull --server')
      expect(r.err.match(/@iota-kit\/billing" is not part/g)).toHaveLength(1)
      expect(r.err).not.toMatch(/ts\.(2307|7006|7031|2339)/)
      expect(r.err).toMatch(/follow-on type errors?/)
    }
    const json = JSON.parse(iota(pkg, 'check', '--json').out) as { diagnostics: { code: string; severity: string }[] }
    expect(json.diagnostics.filter((d) => d.severity === 'error').map((d) => d.code)).toEqual(['graph.unknownKit'])
  })
})

describe('the kit outside node_modules', () => {
  /** What a package manager does to node_modules: everything it did not install goes, the SDK it did install stays. */
  function npmInstall(pkg: string): void {
    for (const entry of readdirSync(join(pkg, 'node_modules'))) if (entry !== '@iota-uz') rmSync(join(pkg, 'node_modules', entry), { recursive: true, force: true })
  }

  it('survives a reinstall: check, test and the editor typecheck still resolve @iota-kit', () => {
    const parent = project('npm')
    expect(iota(parent, 'init', 'pkg', '--namespace', 'acme.npm', '--no-client').status).toBe(0)
    const pkg = join(parent, 'pkg')
    writeFileSync(join(pkg, 'src', 'uses.ts'), `import * as crm from "@iota-kit/crm"\nexport const x = crm.Client\n`)
    // The package's own node_modules: the installed SDK and node typings, plus a kit that an old CLI generated into it.
    mkdirSync(join(pkg, 'node_modules', '@iota-uz'), { recursive: true })
    symlinkSync(join(sandbox, 'sdk'), join(pkg, 'node_modules', '@iota-uz', 'sdk'), 'dir')
    mkdirSync(join(pkg, 'node_modules', '@iota-kit', 'core'), { recursive: true })
    writeFileSync(join(pkg, 'node_modules', '@iota-kit', 'core', 'index.js'), 'export const stale = true\n')
    npmInstall(pkg)
    expect(existsSync(join(pkg, 'node_modules', '@iota-kit'))).toBe(false)
    const check = iota(pkg, 'check')
    expect(check.err).toBe('')
    expect(check.status).toBe(0)
    expect(iota(pkg, 'test').status).toBe(0)
    expect(iota(pkg, 'kit', 'status').out).toMatch(/ok {2}@iota-kit\/core/)

    // The author's own tsc resolves the kit through tsconfig paths (types from the same dist the package installs).
    mkdirSync(join(pkg, 'node_modules', '@types'), { recursive: true })
    symlinkSync(join(runtimeRoot, 'node_modules', '@types', 'node'), join(pkg, 'node_modules', '@types', 'node'), 'dir')
    const tsc = spawnSync(process.execPath, [join(runtimeRoot, 'node_modules', 'typescript', 'bin', 'tsc'), '-p', 'tsconfig.json', '--noEmit'], { cwd: pkg, encoding: 'utf8' })
    expect(tsc.stdout + tsc.stderr).toBe('')
    expect(tsc.status).toBe(0)
  }, 120_000)

  it('moves a kit left in node_modules/@iota-kit and adds the tsconfig paths entry', () => {
    const parent = project('legacy')
    expect(iota(parent, 'init', 'pkg', '--namespace', 'acme.legacy', '--no-client').status).toBe(0)
    const pkg = join(parent, 'pkg')
    // The layout of earlier versions: kit and record in node_modules, no paths entry, no kit.json.
    mkdirSync(join(pkg, 'node_modules', '@iota-kit'), { recursive: true })
    cpSync(join(pkg, '.iota', 'kit', 'core'), join(pkg, 'node_modules', '@iota-kit', 'core'), { recursive: true })
    cpSync(join(pkg, '.iota', 'kit', 'crm'), join(pkg, 'node_modules', '@iota-kit', 'crm'), { recursive: true })
    const record = JSON.parse(readFileSync(join(pkg, '.iota', 'kit', 'kit.json'), 'utf8')) as { hash: string }
    writeFileSync(join(pkg, 'node_modules', '.iota-kit.json'), JSON.stringify({ hash: record.hash, source: { kind: 'catalog', files: ['sdk'] }, modules: ['@iota-kit/core', '@iota-kit/crm'] }))
    rmSync(join(pkg, '.iota', 'kit'), { recursive: true })
    const tsconfig = JSON.parse(readFileSync(join(pkg, 'tsconfig.json'), 'utf8')) as { compilerOptions: { paths?: unknown } }
    delete tsconfig.compilerOptions.paths
    writeFileSync(join(pkg, 'tsconfig.json'), JSON.stringify(tsconfig, null, 2))

    const before = iota(pkg, 'check')
    expect(before.status, before.err).toBe(0)
    expect(before.err).toContain('npm install')
    expect(before.err).toContain('iota-ext kit generate')
    const status = iota(pkg, 'kit', 'status')
    expect(status.out).toContain('node_modules/@iota-kit')
    expect(status.out).toContain('tsconfig.json has no @iota-kit/* path')

    const moved = iota(pkg, 'kit', 'generate')
    expect(moved.status, moved.err).toBe(0)
    expect(moved.out).toContain('updated tsconfig.json')
    expect(existsSync(join(pkg, 'node_modules', '@iota-kit'))).toBe(false)
    expect(existsSync(join(pkg, 'node_modules', '.iota-kit.json'))).toBe(false)
    expect(existsSync(join(pkg, '.iota', 'kit', 'core', 'index.js'))).toBe(true)
    expect(JSON.parse(readFileSync(join(pkg, 'tsconfig.json'), 'utf8')).compilerOptions.paths).toEqual({ '@iota-kit/*': ['./.iota/kit/*'] })
    const after = iota(pkg, 'check')
    expect(after.err).toBe('')
    expect(after.status).toBe(0)
  }, 120_000)

  describe('status verifies the files, not just the record', () => {
    const fresh = (name: string): string => {
      const parent = project(name)
      expect(iota(parent, 'init', 'pkg', '--namespace', `acme.${name}`, '--no-client').status).toBe(0)
      return join(parent, 'pkg')
    }

    it('is ok for an intact kit and reports no kit when none is installed', () => {
      const pkg = fresh('stok')
      const status = iota(pkg, 'kit', 'status')
      expect(status.status).toBe(0)
      expect(status.out).toMatch(/ok {2}@iota-kit\/core/)
      expect(JSON.parse(iota(pkg, 'kit', 'status', '--json').out)).toMatchObject({ ok: true, installed: true, location: 'kit', editor: 'present', modules: [{ name: '@iota-kit/core', state: 'ok' }, { name: '@iota-kit/crm', state: 'ok' }] })
      rmSync(join(pkg, '.iota'), { recursive: true })
      const none = iota(pkg, 'kit', 'status')
      expect(none.status).toBe(0)
      expect(none.out).toContain('no developer kit installed')
    })

    it('reports a module whose files are gone, and a missing file, with exit 1', () => {
      const pkg = fresh('stgone')
      rmSync(join(pkg, '.iota', 'kit', 'crm'), { recursive: true })
      rmSync(join(pkg, '.iota', 'kit', 'core', 'index.d.ts'))
      const status = iota(pkg, 'kit', 'status')
      expect(status.status).toBe(1)
      expect(status.out).toMatch(/missing {2}@iota-kit\/crm: .*index\.js is missing/)
      expect(status.out).toMatch(/missing {2}@iota-kit\/core: index\.d\.ts is missing/)
      expect(status.out).toContain('the kit is damaged')
      const json = JSON.parse(iota(pkg, 'kit', 'status', '--json').out) as { ok: boolean; modules: { name: string; state: string }[] }
      expect(json.ok).toBe(false)
      expect(json.modules.map((m) => m.state)).toEqual(['missing', 'missing'])
      // check says so too, instead of only complaining about the imports.
      writeFileSync(join(pkg, 'src', 'uses.ts'), `import * as crm from "@iota-kit/crm"\nexport const x = crm.Client\n`)
      const check = iota(pkg, 'check')
      expect(check.status).toBe(1)
      expect(check.err).toContain('the kit does not match its record')
      expect(check.err).toContain('@iota-kit/crm')
      // Regenerating restores it.
      expect(iota(pkg, 'kit', 'generate').status).toBe(0)
      expect(iota(pkg, 'kit', 'status').status).toBe(0)
    })

    it('reports an edited file as modified', () => {
      const pkg = fresh('stmod')
      writeFileSync(join(pkg, '.iota', 'kit', 'core', 'index.js'), 'export const User = 1\n')
      const status = iota(pkg, 'kit', 'status')
      expect(status.status).toBe(1)
      expect(status.out).toMatch(/modified {2}@iota-kit\/core: index\.js differs from the recorded snapshot/)
    })
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
    const js = readFileSync(join(dir, '.iota/kit/billing/index.js'), 'utf8')
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
    // What the server sends: a good kit, or a hostile one.
    let mode: 'good' | 'name' | 'path' = 'good'

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
          const modules = mode === 'name' ? ['@iota-kit/../../escaped'] : ['@iota-kit/granite']
          res.writeHead(200, { 'content-type': 'application/json' }).end(JSON.stringify({ hash: 'abc123', modules }))
        } else if (req.url?.startsWith('/_ext/manage/kit/modules/granite.tgz?hash=abc123') === true) {
          const files: Record<string, string> =
            mode === 'path'
              ? { 'index.js': 'x', 'a\\..\\..\\..\\escaped.js': 'x' }
              : { 'index.js': 'export const Deal = 1\n', 'index.d.ts': 'export const Deal: 1\n', 'package.json': '{"name":"@iota-kit/granite"}' }
          res.writeHead(200, { 'content-type': 'application/gzip' }).end(tarball(files))
        } else {
          res.writeHead(404).end()
        }
      })
      await new Promise<void>((ok) => server.listen(0, '127.0.0.1', ok))
      const address = server.address()
      base = `http://127.0.0.1:${typeof address === 'object' && address !== null ? address.port : 0}`
    })
    afterAll(() => void server.close())

    it('downloads the modules into .iota/kit and records the snapshot hash', async () => {
      const dir = project('pull')
      writeFileSync(join(dir, 'package.json'), '{"name":"x","iota":{"namespace":"acme.p","server":"./s.ts"}}')
      const r = await iotaAsync(dir, { IOTA_TOKEN: 's3cret' }, 'kit', 'pull', '--server', base, '--token-env', 'IOTA_TOKEN')
      expect(r.status, r.err).toBe(0)
      expect(sawAuth).toBe('Bearer s3cret')
      expect(readFileSync(join(dir, '.iota/kit/granite/index.js'), 'utf8')).toBe('export const Deal = 1\n')
      expect(JSON.parse(readFileSync(join(dir, '.iota/kit/kit.json'), 'utf8'))).toMatchObject({ hash: 'abc123', modules: ['@iota-kit/granite'], source: { kind: 'server' } })
      expect(existsSync(join(dir, 'node_modules/@iota-kit'))).toBe(false)
    })

    it('refuses a hostile kit before it deletes or writes anything, keeping the installed kit', async () => {
      const dir = project('pullevil')
      writeFileSync(join(dir, 'package.json'), '{"name":"x"}')
      mode = 'good'
      expect((await iotaAsync(dir, { IOTA_TOKEN: 's3cret' }, 'kit', 'pull', '--server', base, '--token-env', 'IOTA_TOKEN')).status).toBe(0)
      for (const hostile of ['name', 'path'] as const) {
        mode = hostile
        const r = await iotaAsync(dir, { IOTA_TOKEN: 's3cret' }, 'kit', 'pull', '--server', base, '--token-env', 'IOTA_TOKEN')
        expect(r.status, r.err).toBe(3)
        expect(r.err).toContain('the installed kit is unchanged')
        expect(readFileSync(join(dir, '.iota/kit/granite/index.js'), 'utf8')).toBe('export const Deal = 1\n')
        expect(readdirSync(join(dir, '.iota'))).toEqual(['kit'])
        expect(existsSync(join(sandbox, 'escaped'))).toBe(false)
        expect(readdirSync(sandbox).filter((n) => n.includes('escaped'))).toEqual([])
      }
      mode = 'good'
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
    cpSync(example, pkg, { recursive: true, filter: (src) => !src.includes('node_modules') && !src.includes(join('.iota', 'build')) })
    // The committed kit is the one `kit generate` writes now, and is intact.
    const committed = readFileSync(join(pkg, '.iota/kit/kit.json'), 'utf8')
    expect(iota(pkg, 'kit', 'status').status).toBe(0)
    expect(iota(pkg, 'kit', 'generate').status).toBe(0)
    expect(readFileSync(join(pkg, '.iota/kit/kit.json'), 'utf8'), 'the example\'s committed .iota/kit is stale: run `iota-ext kit generate` in docs/extensions/example').toBe(committed)
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

    // Its client screen uses the UI kit, a Tailwind utility and a scoped CSS module, and the build ships all of it.
    const manifest = JSON.parse(readFileSync(join(pkg, '.iota/build/artifacts/client/manifest.json'), 'utf8')) as { css: string[]; shared: string[] }
    expect(manifest.shared).toContain('@iota-uz/sdk/solid-ui')
    expect(manifest.css).toHaveLength(2)
    expect(readFileSync(join(pkg, '.iota/build/artifacts', manifest.css[0]!), 'utf8')).toContain('.p-6{')
    expect(readFileSync(join(pkg, '.iota/build/artifacts', manifest.css[1]!), 'utf8')).toMatch(/\.RewardsScreen_[0-9a-f]{8}_board\b/)
  }, 240_000)

  it('is a package as `iota-ext init` and the guide describe it', () => {
    const pkg = JSON.parse(readFileSync(join(example, 'package.json'), 'utf8')) as { dependencies: Record<string, string>; devDependencies: Record<string, string> }
    const sdkVersion = (JSON.parse(readFileSync(join(repoRoot, 'web', 'sdk', 'package.json'), 'utf8')) as { version: string }).version
    expect(pkg.dependencies['@iota-uz/sdk'], 'the exact SDK version init writes, not a range').toBe(sdkVersion)
    expect(pkg.devDependencies['esbuild']).toBe('^0.25.0')
    // §4: the editor resolves the kit from .iota/kit through tsconfig paths.
    expect((JSON.parse(readFileSync(join(example, 'tsconfig.json'), 'utf8')) as { compilerOptions: { paths: unknown } }).compilerOptions.paths).toEqual({ '@iota-kit/*': ['./.iota/kit/*'] })
    // The kit is committed (§3): nothing ignores it, and the generated build output stays ignored.
    expect(readFileSync(join(example, '.gitignore'), 'utf8')).toBe('node_modules\n.iota/build\n')
    expect(existsSync(join(example, '.iota/kit/kit.json'))).toBe(true)
    const ignored = (path: string) => spawnSync('git', ['check-ignore', '-q', path], { cwd: repoRoot }).status === 0
    expect(ignored('docs/extensions/example/.iota/kit/core/index.js')).toBe(false)
    expect(ignored('docs/extensions/example/.iota/build/manifest.json')).toBe(true)
  })

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
    expect(files).toEqual(['ClientScreen.tsx', 'RedemptionScreen.tsx', 'RewardsScreen.module.css', 'RewardsScreen.tsx', 'client.tsx', 'contracts.ts', 'css-modules.d.ts', 'server.test.ts', 'server.ts'])
    expect(statSync(join(example, 'package.json')).isFile()).toBe(true)
  })
})
