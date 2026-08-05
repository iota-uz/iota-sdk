import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { execFileSync } from 'node:child_process'

const artifactsDir = path.resolve(process.argv[2] ?? 'artifacts/frontend')
const manifest = JSON.parse(readFileSync(path.join(artifactsDir, 'frontend-artifacts.json'), 'utf8'))
const artifact = (name) => {
  const entry = manifest.artifacts.find((candidate) => candidate.package === name)
  if (!entry) throw new Error(`missing ${name} artifact`)
  return { ...entry, path: path.join(artifactsDir, entry.file) }
}
const clientHost = artifact('@iota-uz/client-host')
const lens = artifact('@iota-uz/lens-web')
if (clientHost.version !== lens.version) throw new Error('frontend sibling versions must match')

const consumer = mkdtempSync(path.join(tmpdir(), 'iota-sdk-frontend-consumer-'))
try {
  writeFileSync(path.join(consumer, 'package.json'), `${JSON.stringify({
    name: 'artifact-consumer', private: true, packageManager: 'pnpm@9.15.0',
  }, null, 2)}\n`)
  // A transitive @iota-uz lookup must fail immediately. Both private packages
  // have to resolve solely from the two sibling tarballs passed to pnpm.
  writeFileSync(path.join(consumer, '.npmrc'), '@iota-uz:registry=http://127.0.0.1:9/\nfetch-retries=0\n')
  execFileSync('pnpm', [
    'add', '--ignore-scripts', '--strict-peer-dependencies',
    clientHost.path, lens.path, 'react@19.2.4', 'react-dom@19.2.4', 'vite@7.2.7',
  ], { cwd: consumer, stdio: 'inherit' })

  const installed = (name) => JSON.parse(readFileSync(path.join(consumer, 'node_modules', ...name.split('/'), 'package.json'), 'utf8'))
  const installedHost = installed('@iota-uz/client-host')
  const installedLens = installed('@iota-uz/lens-web')
  if (installedHost.version !== clientHost.version || installedLens.version !== lens.version) {
    throw new Error('clean consumer installed a frontend package from the wrong SDK commit')
  }
  if (installedLens.dependencies?.['@iota-uz/client-host']) {
    throw new Error('Lens must not resolve client-host transitively from a registry')
  }
  if (installedLens.peerDependencies?.['@iota-uz/client-host'] !== clientHost.version) {
    throw new Error('Lens must require the exact sibling client-host version')
  }

  // Metadata is insufficient: an export can point at a file the tarball never
  // shipped. Bundle both public entry points exactly as a Granite consumer does
  // so Rollup has to resolve the React runtime and the physical stylesheet.
  writeFileSync(path.join(consumer, 'index.html'), '<main id="app"></main><script type="module" src="/main.js"></script>\n')
  writeFileSync(path.join(consumer, 'main.js'), [
    "import { LensDashboard } from '@iota-uz/lens-web'",
    "import '@iota-uz/lens-web/styles.css'",
    "document.querySelector('#app').dataset.runtime = LensDashboard.name",
    '',
  ].join('\n'))
  execFileSync('pnpm', ['exec', 'vite', 'build'], { cwd: consumer, stdio: 'inherit' })
} finally {
  rmSync(consumer, { recursive: true, force: true })
}
