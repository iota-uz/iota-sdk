// The `iota-ext` commands. Every command returns its exit code:
//   0 success   1 the package has errors (diagnostics, failing tests)   2 usage   3 environment (worker, network, files)
import { existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, relative, resolve } from 'node:path'

import type { BuildResponse, Diagnostic, KitResponse, KitSourcePayload, TestResponse } from '../build/protocol.ts'
import { UsageError, parseArgs } from './args.ts'
import { InvalidKitError, KIT_DIR, KIT_PATHS_KEY, KIT_PATHS_VALUE, editorPaths, ensureEditorPaths, inspectKit, installKit, kitNamespace, loadKit, type KitReport } from './kitfs.ts'
import { formatDiagnostics, formatTests, plural, style, type Style } from './report.ts'
import { EnvironmentError, locateDistribution, runWorker, type Distribution } from './runtime.ts'
import { defaultPackageName, scaffold, validateNamespace } from './scaffold.ts'
import { untar } from './tar.ts'

export interface Io {
  readonly cwd: string
  readonly env: NodeJS.ProcessEnv
  readonly color: boolean
  out(text: string): void
  err(text: string): void
}

interface Context {
  readonly io: Io
  readonly dist: Distribution
  readonly s: Style
}

const hasErrors = (list: readonly Diagnostic[]): boolean => list.some((d) => d.severity === 'error')
const json = (io: Io, value: unknown): void => io.out(JSON.stringify(value, null, 2))

function packageDir(ctx: Context, dir: string | undefined): string {
  const abs = resolve(ctx.io.cwd, dir ?? '.')
  if (!existsSync(join(abs, 'package.json'))) throw new UsageError(`no package.json in ${abs}: run iota-ext from a package directory, or pass --dir`)
  return abs
}

const KIT_COMMANDS = '`iota-ext kit generate` (the SDK natives core and crm, offline), `iota-ext kit generate --catalog <file>` or `iota-ext kit pull --server <url>`'

/** What is wrong with the installed kit, one sentence; undefined when it is intact. */
function kitDamage(report: KitReport): string | undefined {
  const broken = report.modules.filter((m) => m.state !== 'ok')
  if (broken.length === 0) return undefined
  const detail = broken.map((m) => `${m.name} (${m.problems.join('; ')})`).join(', ')
  return `the kit does not match its record: ${detail}. Restore it with ${KIT_COMMANDS}; \`iota-ext kit status\` lists the details.`
}

function kitHint(dir: string, diagnostics: readonly Diagnostic[]): string | undefined {
  if (!diagnostics.some((d) => /@iota-kit\//.test(d.message) || d.code === 'graph.unknownKit')) return undefined
  const kit = loadKit(dir)
  const damage = kitDamage(inspectKit(dir))
  if (damage !== undefined) return damage
  if (kit.installed) {
    return `kit modules installed (${KIT_DIR}): ${Object.keys(kit.modules).join(', ')}. A module missing here comes from another package or an application: use \`iota-ext kit pull --server <url>\` or \`iota-ext kit generate --catalog <file>\`.`
  }
  return `no developer kit is installed: run ${KIT_COMMANDS}. The kit lives in ${KIT_DIR} (package managers never touch it).`
}

/** Said whenever a command reads a kit from the old node_modules location, where a package manager deletes it. */
function legacyNote(ctx: Context, dir: string): void {
  if (loadKit(dir).location === 'legacy') {
    ctx.io.err(`${ctx.s.dim('note:')} the kit is installed under node_modules/@iota-kit, which \`npm install\` deletes. Run \`iota-ext kit generate\` (or \`kit pull\`) once to move it to ${KIT_DIR}.`)
  }
}

function printDiagnostics(ctx: Context, dir: string, diagnostics: readonly Diagnostic[]): void {
  if (diagnostics.length > 0) ctx.io.err(formatDiagnostics(diagnostics, ctx.s))
  const hint = kitHint(dir, diagnostics)
  if (hint !== undefined) ctx.io.err(`${ctx.s.dim('hint:')} ${hint}`)
}

// ---------------------------------------------------------------- check | build

async function runPackageOp(ctx: Context, op: 'check' | 'build', dir: string, outDir: string): Promise<BuildResponse> {
  legacyNote(ctx, dir)
  const kit = loadKit(dir)
  return runWorker<BuildResponse>(ctx.dist, { op, dir, outDir, kit: { hash: kit.hash, modules: kit.modules } }, ctx.io.env)
}

async function withScratch<T>(body: (dir: string) => Promise<T>): Promise<T> {
  const scratch = mkdtempSync(join(tmpdir(), 'iota-ext-'))
  try {
    return await body(scratch)
  } finally {
    rmSync(scratch, { recursive: true, force: true })
  }
}

async function check(ctx: Context, argv: readonly string[]): Promise<number> {
  const args = parseArgs(argv, { values: ['dir'], booleans: ['json'] })
  const dir = packageDir(ctx, args.value('dir'))
  const res = await withScratch((scratch) => runPackageOp(ctx, 'check', dir, scratch))
  const ok = !hasErrors(res.diagnostics)
  if (args.flag('json')) {
    json(ctx.io, { ok, diagnostics: res.diagnostics, manifest: res.manifest })
    return ok ? 0 : 1
  }
  printDiagnostics(ctx, dir, res.diagnostics)
  if (!ok) {
    ctx.io.err(ctx.s.red(`check failed: ${plural(res.diagnostics.filter((d) => d.severity === 'error').length, 'error')}`))
    return 1
  }
  ctx.io.out(`${ctx.s.green('check passed')}: ${res.manifest?.namespace ?? ''} declares ${plural(res.manifest?.resources.length ?? 0, 'resource')}`)
  return 0
}

async function build(ctx: Context, argv: readonly string[]): Promise<number> {
  const args = parseArgs(argv, { values: ['dir', 'out'], booleans: ['json'] })
  const dir = packageDir(ctx, args.value('dir'))
  const out = resolve(ctx.io.cwd, args.value('out') ?? join(dir, '.iota', 'build'))
  const rel = relative(out, dir)
  if (rel === '' || (!rel.startsWith('..') && !rel.startsWith('/'))) throw new UsageError(`--out ${out} contains the package; choose a separate directory`)
  rmSync(join(out, 'artifacts'), { recursive: true, force: true })
  mkdirSync(out, { recursive: true })
  const res = await runPackageOp(ctx, 'build', dir, out)
  const ok = !hasErrors(res.diagnostics) && res.manifest !== undefined
  if (ok) writeFileSync(join(out, 'manifest.json'), JSON.stringify(res.manifest, null, 2) + '\n')
  if (args.flag('json')) {
    json(ctx.io, { ok, out, diagnostics: res.diagnostics, artifacts: res.artifacts, manifest: res.manifest })
    return ok ? 0 : 1
  }
  printDiagnostics(ctx, dir, res.diagnostics)
  if (!ok) {
    ctx.io.err(ctx.s.red(`build failed: ${plural(res.diagnostics.filter((d) => d.severity === 'error').length, 'error')}`))
    return 1
  }
  ctx.io.out(`${ctx.s.green('build passed')}: ${res.manifest?.namespace ?? ''} (${plural(res.manifest?.resources.length ?? 0, 'resource')})`)
  for (const a of res.artifacts) ctx.io.out(`  ${join(relative(ctx.io.cwd, out) || '.', 'artifacts', a)}`)
  ctx.io.out(`  ${join(relative(ctx.io.cwd, out) || '.', 'manifest.json')}`)
  return 0
}

// ---------------------------------------------------------------- test

async function test(ctx: Context, argv: readonly string[]): Promise<number> {
  const args = parseArgs(argv, { values: ['dir'], booleans: ['json'] })
  const dir = packageDir(ctx, args.value('dir'))
  legacyNote(ctx, dir)
  const kit = loadKit(dir)
  const res = await withScratch((scratch) =>
    runWorker<TestResponse>(ctx.dist, { op: 'test', dir, outDir: scratch, kit: { hash: kit.hash, modules: kit.modules }, typecheck: true, filter: args.positionals }, ctx.io.env),
  )
  const failed = res.cases.filter((c) => !c.passed).length
  const ok = !hasErrors(res.diagnostics) && failed === 0 && res.cases.length > 0
  if (args.flag('json')) {
    json(ctx.io, { ok, diagnostics: res.diagnostics, cases: res.cases, passed: res.cases.length - failed, failed })
    return ok ? 0 : 1
  }
  if (hasErrors(res.diagnostics)) {
    printDiagnostics(ctx, dir, res.diagnostics)
    ctx.io.err(ctx.s.red('tests were not run: the package does not compile'))
    return 1
  }
  if (res.cases.length === 0) {
    ctx.io.err(`no tests found: add *.test.ts files that register cases with test() from "@iota-uz/sdk/runtime/testing"${args.positionals.length > 0 ? ` and match ${args.positionals.join(', ')}` : ''}`)
    return 1
  }
  ctx.io.out(formatTests(res.cases, ctx.s))
  const summary = `${plural(res.cases.length, 'test')}: ${res.cases.length - failed} passed${failed > 0 ? `, ${failed} failed` : ''}`
  ctx.io.out(failed > 0 ? ctx.s.red(summary) : ctx.s.green(summary))
  return ok ? 0 : 1
}

// ---------------------------------------------------------------- kit

interface CatalogFile {
  schema?: string
  sources?: { namespace: string; resources?: KitSourcePayload['resources']; lists?: string[]; contractsModule?: string }[]
}

function readCatalog(ctx: Context, spec: string): { label: string; sources: KitSourcePayload[] } {
  const file = spec === 'sdk' ? ctx.dist.sdkCatalog : resolve(ctx.io.cwd, spec)
  let doc: CatalogFile
  try {
    doc = JSON.parse(readFileSync(file, 'utf8')) as CatalogFile
  } catch (err) {
    throw new EnvironmentError(`cannot read catalog snapshot ${file}: ${err instanceof Error ? err.message : String(err)}`)
  }
  if (doc.schema !== 'iota.catalog/1' || !Array.isArray(doc.sources)) {
    throw new EnvironmentError(`${file} is not a catalog snapshot (schema "iota.catalog/1"); export one with GET /_ext/manage/catalog/natives or the application's catalog command`)
  }
  return {
    label: spec,
    sources: doc.sources.map((s) => ({
      namespace: s.namespace,
      resources: s.resources ?? [],
      ...(s.lists === undefined ? {} : { lists: s.lists }),
      ...(s.contractsModule === undefined ? {} : { contractsModule: s.contractsModule }),
    })),
  }
}

function pathsSnippet(): string {
  return `"compilerOptions": { "paths": { "${KIT_PATHS_KEY}": ["${KIT_PATHS_VALUE}"] } }`
}

/** The editor (tsc, the language server) resolves the kit through tsconfig `paths`; say when that is not set up. */
function reportEditor(ctx: Context, editor: ReturnType<typeof ensureEditorPaths>): void {
  if (editor === 'added') ctx.io.out(`updated tsconfig.json: ${pathsSnippet()} (the editor resolves ${KIT_PATHS_KEY} there)`)
  else if (editor === 'manual') ctx.io.err(`${ctx.s.dim('note:')} add ${pathsSnippet()} to tsconfig.json so the editor resolves ${KIT_PATHS_KEY} (iota-ext itself does not need it)`)
}

async function kitGenerate(ctx: Context, argv: readonly string[]): Promise<number> {
  const args = parseArgs(argv, { values: ['dir'], lists: ['catalog'], booleans: ['json'] })
  const dir = packageDir(ctx, args.value('dir'))
  const specs = args.list('catalog').length > 0 ? args.list('catalog') : ['sdk']
  const owner = new Map<string, string>()
  const sources: KitSourcePayload[] = []
  for (const spec of specs) {
    const cat = readCatalog(ctx, spec)
    for (const s of cat.sources) {
      const prev = owner.get(s.namespace)
      if (prev !== undefined) {
        throw new UsageError(`namespace "${s.namespace}" is in both ${prev} and ${cat.label}: an application snapshot already contains the SDK natives, so pass only that file`)
      }
      owner.set(s.namespace, cat.label)
      sources.push(s)
    }
  }
  const res = await runWorker<KitResponse>(ctx.dist, { op: 'kit', sources }, ctx.io.env)
  if (hasErrors(res.diagnostics)) {
    if (args.flag('json')) json(ctx.io, { ok: false, diagnostics: res.diagnostics })
    else ctx.io.err(formatDiagnostics(res.diagnostics, ctx.s))
    return 1
  }
  const record = installKit(dir, res.hash, res.modules, { kind: 'catalog', files: [...specs] })
  const editor = ensureEditorPaths(dir)
  if (args.flag('json')) json(ctx.io, { ok: true, hash: record.hash, modules: record.modules, dir: KIT_DIR, editor })
  else {
    ctx.io.out(`${ctx.s.green('kit generated')}: ${record.modules.join(', ')} (snapshot ${record.hash.slice(0, 12)}, in ${KIT_DIR})`)
    reportEditor(ctx, editor)
  }
  return 0
}

function managementBase(server: string): string {
  const trimmed = server.replace(/\/+$/, '')
  return trimmed.endsWith('/_ext/manage') ? trimmed : `${trimmed}/_ext/manage`
}

async function kitPull(ctx: Context, argv: readonly string[]): Promise<number> {
  const args = parseArgs(argv, { values: ['dir', 'server', 'token-env', 'cookie-env'], booleans: ['json'] })
  const dir = packageDir(ctx, args.value('dir'))
  const server = args.value('server')
  if (server === undefined) throw new UsageError('kit pull needs --server <url> (the application that runs the extension runtime)')
  const headers: Record<string, string> = { Accept: 'application/json' }
  const tokenEnv = args.value('token-env')
  if (tokenEnv !== undefined) {
    const token = ctx.io.env[tokenEnv]
    if (token === undefined || token === '') throw new UsageError(`environment variable ${tokenEnv} is empty: export the API token there (it is never taken from the command line)`)
    headers.Authorization = `Bearer ${token}`
  }
  const cookieEnv = args.value('cookie-env')
  if (cookieEnv !== undefined) {
    const cookie = ctx.io.env[cookieEnv]
    if (cookie === undefined || cookie === '') throw new UsageError(`environment variable ${cookieEnv} is empty: export the session cookie header there`)
    headers.Cookie = cookie
  }
  const base = managementBase(server)
  const get = async (url: string): Promise<Response> => {
    let res: Response
    try {
      res = await fetch(url, { headers })
    } catch (err) {
      throw new EnvironmentError(`cannot reach ${url}: ${err instanceof Error ? err.message : String(err)}`)
    }
    if (!res.ok) {
      let detail = ''
      try {
        const body = (await res.json()) as { error?: { code?: string; message?: string } }
        detail = body.error === undefined ? '' : `: ${body.error.code ?? ''} ${body.error.message ?? ''}`.trimEnd()
      } catch {
        detail = ''
      }
      throw new EnvironmentError(`${url} answered ${res.status}${detail}${res.status === 401 || res.status === 403 ? ' (pass --token-env <VAR> with an API token, or --cookie-env <VAR>; the user needs the extensions read permission)' : ''}`)
    }
    return res
  }
  const index = (await (await get(`${base}/kit`)).json()) as { hash: string; modules: string[] }
  const modules: Record<string, Record<string, Uint8Array>> = {}
  let record: ReturnType<typeof installKit>
  try {
    // The server is not trusted with names or paths: they are validated before anything is requested or written.
    if (typeof index.hash !== 'string' || !Array.isArray(index.modules)) throw new InvalidKitError('the kit index has no hash or module list')
    const namespaces = index.modules.map((name) => kitNamespace(name))
    for (const [i, name] of index.modules.entries()) {
      const archive = new Uint8Array(await (await get(`${base}/kit/modules/${namespaces[i]}.tgz?hash=${encodeURIComponent(index.hash)}`)).arrayBuffer())
      modules[name] = Object.fromEntries(untar(archive).map((e) => [e.path, e.data]))
    }
    record = installKit(dir, index.hash, modules, { kind: 'server', url: server })
  } catch (err) {
    if (err instanceof InvalidKitError) throw new EnvironmentError(`${server} sent a kit that cannot be installed (the installed kit is unchanged): ${err.message}`)
    throw err
  }
  const editor = ensureEditorPaths(dir)
  if (args.flag('json')) json(ctx.io, { ok: true, hash: record.hash, modules: record.modules, dir: KIT_DIR, editor })
  else {
    ctx.io.out(`${ctx.s.green('kit pulled')}: ${record.modules.join(', ')} (snapshot ${record.hash.slice(0, 12)}, in ${KIT_DIR})`)
    reportEditor(ctx, editor)
  }
  return 0
}

function kitStatus(ctx: Context, argv: readonly string[]): number {
  const args = parseArgs(argv, { values: ['dir'], booleans: ['json'] })
  const dir = packageDir(ctx, args.value('dir'))
  const report = inspectKit(dir)
  const editor = editorPaths(dir)
  const record = report.record
  const damaged = report.location !== 'none' && !report.ok
  if (args.flag('json')) {
    json(ctx.io, {
      ok: !damaged,
      installed: report.location !== 'none',
      dir: KIT_DIR,
      location: report.location,
      hash: record?.hash ?? null,
      source: record?.source ?? null,
      modules: report.modules,
      editor,
    })
    return damaged ? 1 : 0
  }
  if (report.location === 'none') {
    ctx.io.out(`no developer kit installed: run ${KIT_COMMANDS}`)
    return 0
  }
  ctx.io.out(`snapshot ${record?.hash ?? 'unknown'} (${report.location === 'legacy' ? 'node_modules/@iota-kit' : KIT_DIR})`)
  for (const m of report.modules) ctx.io.out(`  ${m.state === 'ok' ? ctx.s.green('ok') : ctx.s.red(m.state)}  ${m.name}${m.problems.length > 0 ? `: ${m.problems.join('; ')}` : ''}`)
  if (report.location === 'legacy') ctx.io.out(`the kit is under node_modules, where \`npm install\` deletes it: run \`iota-ext kit generate\` (or \`kit pull\`) to move it to ${KIT_DIR}`)
  else if (damaged) ctx.io.out(`the kit is damaged: restore it with ${KIT_COMMANDS}`)
  if (editor === 'manual') ctx.io.out(`tsconfig.json has no ${KIT_PATHS_KEY} path, so the editor cannot resolve the kit: add ${pathsSnippet()}`)
  return damaged ? 1 : 0
}

async function kit(ctx: Context, argv: readonly string[]): Promise<number> {
  const [sub, ...rest] = argv
  if (sub === 'generate') return kitGenerate(ctx, rest)
  if (sub === 'pull') return kitPull(ctx, rest)
  if (sub === 'status') return kitStatus(ctx, rest)
  throw new UsageError('usage: iota-ext kit <generate|pull|status>')
}

// ---------------------------------------------------------------- init

async function init(ctx: Context, argv: readonly string[]): Promise<number> {
  const args = parseArgs(argv, { values: ['namespace', 'name'], booleans: ['no-client', 'no-kit', 'force'] })
  const target = args.positionals[0]
  if (target === undefined) throw new UsageError('usage: iota-ext init <dir> --namespace <owner.module> [--name <npm name>] [--no-client] [--no-kit]')
  const namespace = args.value('namespace')
  if (namespace === undefined) throw new UsageError('init needs --namespace <owner.module>, for example --namespace acme.loyalty')
  const invalid = validateNamespace(namespace)
  if (invalid !== undefined) throw new UsageError(invalid)
  const dir = resolve(ctx.io.cwd, target)
  if (existsSync(dir) && readdirSync(dir).length > 0 && !args.flag('force')) throw new UsageError(`${dir} is not empty (use --force to write the scaffold anyway)`)
  const files = scaffold({ namespace, name: args.value('name') ?? defaultPackageName(namespace), sdkVersion: ctx.dist.sdkVersion, client: !args.flag('no-client') })
  for (const [path, text] of Object.entries(files)) {
    const file = join(dir, path)
    mkdirSync(dirname(file), { recursive: true })
    writeFileSync(file, text)
  }
  ctx.io.out(`created ${plural(Object.keys(files).length, 'file')} in ${relative(ctx.io.cwd, dir) || '.'}`)
  if (!args.flag('no-kit')) {
    const code = await kitGenerate(ctx, ['--dir', dir])
    if (code !== 0) return code
  }
  const here = relative(ctx.io.cwd, dir)
  ctx.io.out(
    [
      '',
      'next:',
      ...(here === '' ? [] : [`  cd ${here}`]),
      '  npm install        # editor support (typescript, @types/node, solid-js) and the lockfile publishing needs; the kit in .iota/kit is untouched by it',
      '  npx iota-ext check',
      '  npx iota-ext test',
    ].join('\n'),
  )
  return 0
}

// ---------------------------------------------------------------- dispatch

export const HELP = `iota-ext — author tooling for extension packages

Usage: iota-ext <command> [options]

  init <dir> --namespace <owner.module> [--name <npm>] [--no-client] [--no-kit] [--force]
                          scaffold a package (contracts, server, client, a passing test) and its offline kit
  kit generate [--catalog <file|sdk>]...
                          render @iota-kit/* into .iota/kit from catalog snapshots, offline (default: the SDK natives core, crm)
  kit pull --server <url> [--token-env VAR] [--cookie-env VAR]
                          download the live developer kit into .iota/kit from the application's /_ext/manage API
  kit status              verify the installed kit (modules, files, hashes, tsconfig paths); exit 1 when it is damaged
  check                   typecheck, validate the graphs and the manifest (writes nothing)
  build [--out <dir>]     check, then write artifacts and manifest.json (default .iota/build)
  test [filter...]        typecheck, then run *.test.ts with the synthetic test host

Options for every command except init: --dir <package dir> (default: the current directory), --json (machine output).
Diagnostics print as "file:line:col code message".
Exit codes: 0 ok, 1 package errors or failing tests, 2 usage, 3 environment (network, worker, files).
`

export async function run(argv: readonly string[], io: Io, moduleURL: string): Promise<number> {
  const [command, ...rest] = argv
  const dist = locateDistribution(moduleURL)
  const ctx: Context = { io, dist, s: style(io.color) }
  try {
    switch (command) {
      case undefined:
      case 'help':
      case '--help':
      case '-h':
        io.out(HELP)
        return command === undefined ? 2 : 0
      case '--version':
      case 'version':
        io.out(dist.sdkVersion)
        return 0
      case 'init':
        return await init(ctx, rest)
      case 'kit':
        return await kit(ctx, rest)
      case 'check':
        return await check(ctx, rest)
      case 'build':
        return await build(ctx, rest)
      case 'test':
        return await test(ctx, rest)
      default:
        throw new UsageError(`unknown command "${command}"`)
    }
  } catch (err) {
    if (err instanceof UsageError) {
      io.err(`iota-ext: ${err.message}\nrun "iota-ext help" for usage`)
      return 2
    }
    if (err instanceof EnvironmentError) {
      io.err(`iota-ext: ${err.message}`)
      return 3
    }
    throw err
  }
}

