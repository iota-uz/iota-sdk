import { createHash } from 'node:crypto'
import { cp, mkdir, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { execFileSync } from 'node:child_process'

const root = path.resolve(import.meta.dirname, '..')
const packageRoot = path.join(root, 'web/sdk')
const output = path.resolve(process.argv[2] ?? path.join(root, 'artifacts/frontend'))
const checkoutCommit = execFileSync('git', ['rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8' }).trim()
const expectedCommit = process.env.SDK_SOURCE_SHA?.trim()
if (expectedCommit && expectedCommit !== checkoutCommit) {
  throw new Error(`SDK_SOURCE_SHA ${expectedCommit} does not match checked-out commit ${checkoutCommit}`)
}
const sdkCommit = expectedCommit || checkoutCommit
const canonicalPackage = JSON.parse(await readFile(path.join(packageRoot, 'package.json'), 'utf8'))
const sdkReleaseVersion = canonicalPackage.version
const packageVersion = process.env.FRONTEND_PACKAGE_VERSION?.trim()
  || `0.0.0-sha-${sdkCommit.slice(0, 12)}`

await rm(output, { recursive: true, force: true })
await mkdir(output, { recursive: true })
execFileSync('pnpm', ['--filter', '@iota-uz/sdk', 'build'], {
  cwd: root,
  env: { ...process.env, SDK_SOURCE_SHA: sdkCommit, SDK_RELEASE_VERSION: sdkReleaseVersion },
  stdio: 'inherit',
})

const stagingRoot = await mkdtemp(path.join(tmpdir(), 'iota-sdk-package-'))
try {
  for (const entry of canonicalPackage.files) {
    await cp(path.join(packageRoot, entry), path.join(stagingRoot, entry), { recursive: true })
  }
  const stagedPackage = {
    ...canonicalPackage,
    version: packageVersion,
    scripts: undefined,
    devDependencies: undefined,
  }
  await writeFile(path.join(stagingRoot, 'package.json'), `${JSON.stringify(stagedPackage, null, 2)}\n`)
  const packed = JSON.parse(execFileSync(
    'npm', ['pack', stagingRoot, '--pack-destination', output, '--json'],
    { encoding: 'utf8' },
  ))[0]
  const tarball = path.join(output, packed.filename)
  const manifest = {
    sdkCommit,
    sdkReleaseVersion,
    protocolVersion: '1.0.0',
    package: '@iota-uz/sdk',
    packageVersion,
    file: packed.filename,
    sha256: createHash('sha256').update(await readFile(tarball)).digest('hex'),
  }
  await writeFile(path.join(output, 'frontend-artifacts.json'), `${JSON.stringify(manifest, null, 2)}\n`)
  process.stdout.write(`${JSON.stringify(manifest, null, 2)}\n`)
} finally {
  await rm(stagingRoot, { recursive: true, force: true })
}
