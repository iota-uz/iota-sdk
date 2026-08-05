import { mkdtemp, readFile, readdir, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { execFileSync } from 'node:child_process'

const artifactsDir = path.resolve(process.argv[2] ?? 'artifacts/frontend')
const manifest = JSON.parse(await readFile(path.join(artifactsDir, 'frontend-artifacts.json'), 'utf8'))
if (manifest.package !== '@iota-uz/sdk') throw new Error('preview artifact must contain only @iota-uz/sdk')
const tarball = path.join(artifactsDir, manifest.file)
const consumer = await mkdtemp(path.join(tmpdir(), 'iota-sdk-consumer-'))

function run(command, args) {
  return execFileSync(command, args, { cwd: consumer, encoding: 'utf8', stdio: ['ignore', 'pipe', 'inherit'] })
}

try {
  await writeFile(path.join(consumer, 'package.json'), `${JSON.stringify({
    name: 'sdk-consumer', private: true, type: 'module', packageManager: 'pnpm@9.15.0',
  }, null, 2)}\n`)
  await writeFile(path.join(consumer, '.npmrc'), '@iota-uz:registry=http://127.0.0.1:9/\nfetch-retries=0\n')
  run('pnpm', ['add', '--ignore-scripts', '--strict-peer-dependencies', tarball, 'react@19.2.4', 'react-dom@19.2.4', 'vite@7.3.6'])

  const installedPackage = JSON.parse(await readFile(path.join(consumer, 'node_modules/@iota-uz/sdk/package.json'), 'utf8'))
  if (installedPackage.version !== manifest.packageVersion) throw new Error('consumer installed the wrong package version')
  if (Object.keys(installedPackage.dependencies ?? {}).length !== 0) {
    throw new Error('implementation dependencies leaked into the public package dependency graph')
  }

  await writeFile(path.join(consumer, 'index.html'), '<main id="app"></main><script type="module" src="/main.js"></script>\n')
  await writeFile(path.join(consumer, 'main.js'), [
    "import { ClientHostBoundary, SDK_IDENTITY } from '@iota-uz/sdk/client-host'",
    "document.querySelector('#app').dataset.runtime = ClientHostBoundary.name + SDK_IDENTITY.releaseVersion",
    '',
  ].join('\n'))
  run('pnpm', ['exec', 'vite', 'build'])
  const baseAssets = await readdir(path.join(consumer, 'dist/assets'))
  const baseContents = (await Promise.all(baseAssets.filter((name) => name.endsWith('.js')).map((name) => readFile(path.join(consumer, 'dist/assets', name), 'utf8')))).join('\n')
  if (/echarts|LensDashboard|DashboardPanels/.test(baseContents)) {
    throw new Error('client-host-only bundle pulled Lens implementation code')
  }

  await rm(path.join(consumer, 'dist'), { recursive: true, force: true })
  await writeFile(path.join(consumer, 'main.js'), [
    "import { LensDashboard, CONTRACT_VERSION, SDK_IDENTITY } from '@iota-uz/sdk/lens'",
    "import '@iota-uz/sdk/lens/styles.css'",
    "document.querySelector('#app').dataset.runtime = LensDashboard.name + CONTRACT_VERSION + SDK_IDENTITY.releaseVersion",
    '',
  ].join('\n'))
  run('pnpm', ['exec', 'vite', 'build'])

  const list = run('pnpm', ['list', 'react', 'react-dom', '--depth', 'Infinity', '--json'])
  const graph = JSON.parse(list)
  const serialized = JSON.stringify(graph)
  if ((serialized.match(/react@19\.2\.4/g) ?? []).length === 0) throw new Error('React peer was not resolved')
} finally {
  await rm(consumer, { recursive: true, force: true })
}
