import { access, cp, lstat, mkdtemp, readFile, readdir, rm } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const packageJSON = JSON.parse(await readFile(path.join(root, 'package.json'), 'utf8'))
const expectedExports = [
  './package.json',
  './testing',
  './runtime',
  './runtime/server',
  './runtime/client',
  './runtime/kit',
  './runtime/testing',
  './identity',
  './client-host',
  './solid',
  './solid-ui',
  './solid-ui/standalone.css',
  './solid-ui/embedded-css',
  './styles.css',
  './chat-ui',
  './chat-ui/icons',
  './chat-ui/styles.css',
  './lens',
  './lens/styles.css',
  './automation-editor',
  './automation-editor/styles.css',
]

if (JSON.stringify(Object.keys(packageJSON.exports).sort()) !== JSON.stringify(expectedExports.sort())) {
  throw new Error(`unexpected public exports: ${Object.keys(packageJSON.exports).join(', ')}`)
}
// The public package carries no dependencies (release contract, see
// scripts/verify-frontend-artifacts.mjs). The build worker and `iota-ext` load
// the native esbuild at run time, so it is an optional peer the host and
// extension packages install themselves.
if (Object.keys(packageJSON.dependencies ?? {}).length !== 0) {
  throw new Error(`the public package must have no dependencies, got ${Object.keys(packageJSON.dependencies).join(', ')}`)
}
if (!packageJSON.peerDependencies?.esbuild || packageJSON.peerDependenciesMeta?.esbuild?.optional !== true) {
  throw new Error('esbuild must be an optional peer dependency')
}
if (packageJSON.peerDependencies?.react || packageJSON.peerDependencies?.['react-dom']) {
  throw new Error('React must not be a public runtime dependency')
}
if (!packageJSON.peerDependencies?.['@axe-core/playwright'] || packageJSON.peerDependenciesMeta?.['@axe-core/playwright']?.optional !== true) {
  throw new Error('@axe-core/playwright must be an optional peer dependency of the testing subpath')
}
if (!packageJSON.peerDependencies?.['solid-js']) {
  throw new Error('Solid must be a peer dependency')
}

// Node workers shipped inside the runtime subpath: the invocation host and the build worker.
for (const file of [
  'dist/runtime/host/worker.mjs',
  'dist/runtime/host/register.mjs',
  'dist/runtime/host/loader.mjs',
  'dist/runtime/host/sdk/index.mjs',
  'dist/runtime/build/worker.mjs',
  'dist/runtime/build/extract.mjs',
  'dist/runtime/build/test-child.mjs',
  'dist/runtime/build/vendor/lib/lib.es2022.d.ts',
  'dist/runtime/build/vendor/node-types/package.json',
  'dist/runtime/build/vendor/node_modules/solid-js/package.json',
  'dist/runtime/build/vendor/node_modules/csstype/package.json',
  'dist/runtime/build/vendor/node_modules/seroval/package.json',
  'dist/runtime/build/vendor/node_modules/seroval-plugins/package.json',
  'dist/runtime/cli/iota-ext.mjs',
  'dist/runtime/catalog/sdk-natives.json',
  'dist/runtime/README.md',
  'dist/runtime/docs/authoring.md',
  'dist/runtime/docs/example/package.json',
]) {
  await access(path.join(root, file))
}

// The author CLI: a `bin` that points at a real, executable, shebanged file.
if (packageJSON.bin?.['iota-ext'] !== './dist/runtime/cli/iota-ext.mjs') {
  throw new Error('the package must expose the iota-ext bin at ./dist/runtime/cli/iota-ext.mjs')
}
{
  const cli = path.join(root, 'dist/runtime/cli/iota-ext.mjs')
  const head = (await readFile(cli, 'utf8')).slice(0, 40)
  if (!head.startsWith('#!/usr/bin/env node')) throw new Error('iota-ext.mjs must start with a node shebang')
  if (((await lstat(cli)).mode & 0o111) === 0) throw new Error('iota-ext.mjs must be executable')
}

// The SDK's native catalog snapshot the offline kit is generated from.
{
  const snapshot = JSON.parse(await readFile(path.join(root, 'dist/runtime/catalog/sdk-natives.json'), 'utf8'))
  if (snapshot.schema !== 'iota.catalog/1' || !Array.isArray(snapshot.sources)) throw new Error('sdk-natives.json is not a catalog snapshot')
  const ids = new Set(snapshot.sources.flatMap((s) => s.resources.map((r) => r.id)))
  for (const id of ['core.user', 'crm.client', 'crm.commands.saveClient', 'crm.operations.upsertClient']) {
    if (!ids.has(id)) throw new Error(`sdk-natives.json lacks the native resource ${id}`)
  }
  for (const s of snapshot.sources) {
    for (const r of s.resources) {
      if (r.kind === 'entity' && !['int64', 'uuid', 'string'].includes(r.keyType)) throw new Error(`native entity ${r.id} has no keyType`)
    }
  }
}

// The vendored tree must hold real files: a symlink survives neither npm pack nor another machine.
{
  const walk = async (dir) => {
    for (const entry of await readdir(dir, { withFileTypes: true })) {
      const file = path.join(dir, entry.name)
      if (entry.isSymbolicLink()) throw new Error(`dist contains a symbolic link: ${path.relative(root, file)}`)
      if (entry.isDirectory()) await walk(file)
    }
  }
  await walk(path.join(root, 'dist/runtime/build/vendor'))
}

// Test fixtures never ship: the browser pages and fake extension assets of the runtime host live in
// web/runtime/test/browser, outside the published tree, and unit tests and their helpers are not emitted.
// (The authoring guide's example package under dist/runtime/docs and the vendored toolchain are data, not fixtures.)
{
  const fixtureLike = [
    [/(^|\/)(browser|_ext)$/, 'a browser fixture directory'],
    [/\.(spec|test)\.[cm]?[jt]sx?$/, 'a test file'],
    [/(^|\/)(playwright|vite|vitest)\.config\.[cm]?[jt]s$/, 'a test runner config'],
    [/(^|\/)client-host\/fixtures\.(d\.ts|[cm]?[jt]s)$/, 'the client host test fixtures'],
    [/\.html$/, 'an HTML fixture page'],
  ]
  const leaked = []
  const walk = async (dir) => {
    for (const entry of await readdir(dir, { withFileTypes: true })) {
      const file = path.join(dir, entry.name)
      const rel = path.relative(path.join(root, 'dist/runtime'), file).split(path.sep).join('/')
      if (rel === 'docs' || rel === 'build/vendor') continue
      const hit = fixtureLike.find(([pattern]) => pattern.test(rel))
      if (hit) leaked.push(`${rel} (${hit[1]})`)
      if (entry.isDirectory()) await walk(file)
    }
  }
  await walk(path.join(root, 'dist/runtime'))
  if (leaked.length > 0) throw new Error(`test fixtures are in the published runtime package:\n  ${leaked.join('\n  ')}`)
  // The browser entry of the runtime host and the slot / cell island host are what ships.
  for (const file of ['dist/runtime/client-host/entry.js', 'dist/runtime/client-host/islands.js', 'dist/runtime/client-host/slots.js']) {
    await access(path.join(root, file))
  }
}

const targets = []
for (const value of Object.values(packageJSON.exports)) {
  if (typeof value === 'string') {
    targets.push(value)
    continue
  }
  targets.push(...Object.values(value))
}
for (const target of targets) {
  if (target.includes('*')) continue
  await access(path.resolve(root, target))
}

const solidUIStyles = await readFile(path.join(root, 'dist/solid-ui/standalone.css'), 'utf8')
if (solidUIStyles.includes('/assets/fonts/')) {
  throw new Error('Solid UI standalone CSS must use package-relative font URLs')
}
for (const font of [
  'Inter.var.woff2',
  'Gilroy/Gilroy-Regular.woff2',
  'Gilroy/Gilroy-Medium.woff2',
  'Gilroy/Gilroy-Semibold.woff2',
]) {
  await access(path.join(root, 'dist/solid-ui/fonts', font))
}

const identity = JSON.parse(await readFile(path.join(root, 'dist/sdk-identity.json'), 'utf8'))
if (identity.releaseVersion !== packageJSON.version) {
  throw new Error('package version and generated SDK identity differ')
}
if (!/^[0-9a-f]{40}$/.test(identity.sourceCommit)) {
  throw new Error('generated SDK identity does not contain a full commit SHA')
}

// The testing subpath loads without the optional axe peer, and its helper names
// the package to install. A copy outside every node_modules tree proves it.
{
  const isolated = await mkdtemp(path.join(os.tmpdir(), 'iota-sdk-testing-'))
  try {
    await cp(path.join(root, 'dist/testing'), isolated, { recursive: true })
    await cp(path.join(root, 'package.json'), path.join(isolated, 'package.json'))
    const testing = await import(pathToFileURL(path.join(isolated, 'index.js')).href)
    if (typeof testing.expectNoA11yViolations !== 'function') {
      throw new Error('@iota-uz/sdk/testing must export expectNoA11yViolations')
    }
    // The helper also looks for the peer from the working directory (the test
    // project); run it from the isolated copy so no workspace install is found.
    const previous = process.cwd()
    process.chdir(isolated)
    const failure = await testing.expectNoA11yViolations({}).then(() => undefined, (error) => error)
    process.chdir(previous)
    if (!(failure instanceof testing.A11yDependencyError) || !failure.message.includes('@axe-core/playwright')) {
      throw new Error(`expectNoA11yViolations without @axe-core/playwright must name the missing package, got: ${failure?.message ?? 'success'}`)
    }
  } finally {
    await rm(isolated, { recursive: true, force: true })
  }
}

// The automation editor subpath: server-safe to import next to solid-ui (SSR smoke), and its canvas
// (@xyflow/system, d3) sits in a lazy chunk the entry does not import statically.
{
  const { renderToString } = await import('solid-js/web')
  const solidUI = await import('@iota-uz/sdk/solid-ui')
  const editor = await import('@iota-uz/sdk/automation-editor')
  const graph = { name: 'Smoke', run_as: 'owner', triggers: [{ id: 'user_created', event: 'core.user.created.v1' }], nodes: [], edges: [] }
  const catalog = { revision: '1', events: [{ key: 'core.user.created.v1', title: 'User created', group: 'Users', subject: 'core.user', payload: {}, trigger_params: null }], nodes: [], context_schema: {} }
  const html = renderToString(() => [
    solidUI.Button({ children: 'SSR smoke' }),
    editor.Outline({ graph, catalog, locale: 'en' }),
    editor.Editor({ graph, catalog, layout: {}, version: 0, locale: 'en', enabled: false, api: {}, save: async () => ({ errors: [] }), reload: async () => ({ graph, version: 0, layout: {} }) }),
  ])
  if (!html.includes('SSR smoke') || !html.includes('User created') || !html.includes('ae-skeleton')) {
    throw new Error('solid-ui and the automation editor did not render together on the server')
  }
  const entry = await readFile(path.join(root, 'dist/automation-editor/index.js'), 'utf8')
  const staticImports = [...entry.matchAll(/from\s*["']\.\/([^"']+)["']/g)].map((m) => m[1])
  for (const file of staticImports) {
    const text = await readFile(path.join(root, 'dist/automation-editor', file), 'utf8')
    if (/XYPanZoom|zoomIdentity/.test(text)) throw new Error(`@iota-uz/sdk/automation-editor statically imports the canvas chunk (${file})`)
  }
  const types = await readFile(path.join(root, 'dist/automation-editor/index.d.ts'), 'utf8')
  if (/@xyflow/.test(types)) throw new Error('@xyflow/system types leak into the automation editor declarations')
}
