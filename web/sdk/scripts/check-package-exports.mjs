import { access, readFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const packageJSON = JSON.parse(await readFile(path.join(root, 'package.json'), 'utf8'))
const expectedExports = [
  './package.json',
  './identity',
  './client-host',
  './lens',
  './lens/styles.css',
]

if (JSON.stringify(Object.keys(packageJSON.exports).sort()) !== JSON.stringify(expectedExports.sort())) {
  throw new Error(`unexpected public exports: ${Object.keys(packageJSON.exports).join(', ')}`)
}
if (Object.keys(packageJSON.dependencies ?? {}).length !== 0) {
  throw new Error('the public package must not expose implementation dependencies')
}
if (!packageJSON.peerDependencies?.react || !packageJSON.peerDependencies?.['react-dom']) {
  throw new Error('React and ReactDOM must remain peer dependencies')
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

const identity = JSON.parse(await readFile(path.join(root, 'dist/sdk-identity.json'), 'utf8'))
if (identity.releaseVersion !== packageJSON.version) {
  throw new Error('package version and generated SDK identity differ')
}
if (!/^[0-9a-f]{40}$/.test(identity.sourceCommit)) {
  throw new Error('generated SDK identity does not contain a full commit SHA')
}
