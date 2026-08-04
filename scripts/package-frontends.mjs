import { createHash } from 'node:crypto'
import { cpSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { execFileSync } from 'node:child_process'

const root = path.resolve(import.meta.dirname, '..')
const output = path.resolve(process.argv[2] ?? path.join(root, 'artifacts', 'frontend'))
const commit = execFileSync('git', ['rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8' }).trim()
const version = `0.0.0-sha-${commit.slice(0, 12)}`
const packages = [
  { dir: 'web/client-host', build: 'build', output: 'dist' },
  { dir: 'web/lens', build: 'build:package', output: 'package-dist' },
]

mkdirSync(output, { recursive: true })
const stagingRoot = mkdtempSync(path.join(tmpdir(), 'iota-sdk-frontends-'))
const artifacts = []
try {
  for (const spec of packages) {
    const source = path.join(root, spec.dir)
    execFileSync('pnpm', ['run', spec.build], { cwd: source, stdio: 'inherit' })
    const packageJSON = JSON.parse(readFileSync(path.join(source, 'package.json'), 'utf8'))
    packageJSON.version = version
    packageJSON.dependencies = Object.fromEntries(Object.entries(packageJSON.dependencies ?? {}).map(([name, value]) => [
      name, name === '@iota-uz/client-host' ? version : value,
    ]))
    const staging = path.join(stagingRoot, packageJSON.name.replaceAll('/', '-'))
    mkdirSync(staging, { recursive: true })
    cpSync(path.join(source, spec.output), path.join(staging, spec.output), { recursive: true })
    writeFileSync(path.join(staging, 'package.json'), `${JSON.stringify(packageJSON, null, 2)}\n`)
    const packed = JSON.parse(execFileSync('npm', ['pack', staging, '--pack-destination', output, '--json'], { encoding: 'utf8' }))[0]
    const tarball = path.join(output, packed.filename)
    artifacts.push({ package: packageJSON.name, version, file: packed.filename, sha256: createHash('sha256').update(readFileSync(tarball)).digest('hex') })
  }
  const manifest = { sdkCommit: commit, protocolVersion: '1.0.0', artifacts }
  writeFileSync(path.join(output, 'frontend-artifacts.json'), `${JSON.stringify(manifest, null, 2)}\n`)
  process.stdout.write(`${JSON.stringify(manifest, null, 2)}\n`)
} finally {
  rmSync(stagingRoot, { recursive: true, force: true })
}
