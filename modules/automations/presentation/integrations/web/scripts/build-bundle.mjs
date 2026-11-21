// Builds the bundle the automations module embeds (../dist, served at /assets/automations/) and
// writes dist/bundle.json, which the Go side reads into a clienthost.Manifest.
//
//   node scripts/build-bundle.mjs          # rebuild
//   node scripts/build-bundle.mjs --check  # fail when the committed bundle is stale
import { spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { cpSync, existsSync, mkdtempSync, readFileSync, readdirSync, rmSync, statSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const web = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const dist = path.resolve(web, '../dist')
const check = process.argv.includes('--check')
const SENTINEL = '0123456789abcdef0123456789abcdef01234567'

function run(cmd, args) {
  const r = spawnSync(cmd, args, { cwd: web, stdio: 'inherit' })
  if (r.status !== 0) process.exit(r.status ?? 1)
}

function files(dir, base = dir) {
  return readdirSync(dir).flatMap((name) => {
    const p = path.join(dir, name)
    return statSync(p).isDirectory() ? files(p, base) : [path.relative(base, p)]
  }).sort()
}

let previous
if (check && existsSync(dist)) {
  previous = mkdtempSync(path.join(tmpdir(), 'automations-dist-'))
  cpSync(dist, previous, { recursive: true })
}

run(path.join(web, 'node_modules/.bin/tailwindcss'), ['--input', 'bundle/bundle.css', '--output', 'bundle/bundle.generated.css', '--minify'])
run(path.join(web, 'node_modules/.bin/vite'), ['build', '--config', 'vite.bundle.config.ts'])

const manifest = JSON.parse(readFileSync(path.join(dist, '.vite/manifest.json'), 'utf8'))
const entry = manifest['bundle/entry.ts']
if (!entry) throw new Error('bundle entry is missing from the Vite manifest')

// The identity is the hash of what was built (with the sentinel in place), then written in.
const hash = createHash('sha1')
for (const f of files(dist)) if (!f.startsWith('.vite')) hash.update(f).update(readFileSync(path.join(dist, f)))
const commit = hash.digest('hex')
for (const f of files(dist)) {
  if (!f.endsWith('.js')) continue
  const p = path.join(dist, f)
  const text = readFileSync(p, 'utf8')
  if (text.includes(SENTINEL)) writeFileSync(p, text.split(SENTINEL).join(commit))
}
const identity = readFileSync(path.join(web, '../../../../../web/client-host/src/identity.ts'), 'utf8')
const release = /SDK_RELEASE_VERSION = ['"]([^'"]+)['"]/.exec(identity)?.[1]
const protocol = /CLIENT_HOST_PROTOCOL_VERSION = ['"]([^'"]+)['"]/.exec(readFileSync(path.join(web, '../../../../../web/client-host/src/protocol.ts'), 'utf8'))?.[1]
if (!release || !protocol) throw new Error('cannot read the SDK release or protocol version')
rmSync(path.join(dist, '.vite'), { recursive: true, force: true })
writeFileSync(path.join(dist, 'bundle.json'), JSON.stringify({
  sdkCommit: commit,
  sdkReleaseVersion: release,
  protocolVersion: protocol,
  entry: entry.file,
  styles: entry.css ?? [],
}, null, 2) + '\n')

if (check) {
  const now = files(dist)
  const before = previous ? files(previous) : []
  const stale = JSON.stringify(now) !== JSON.stringify(before) ||
    now.some((f) => !readFileSync(path.join(dist, f)).equals(readFileSync(path.join(previous, f))))
  if (previous) rmSync(previous, { recursive: true, force: true })
  if (stale) {
    console.error('modules/automations/presentation/integrations/dist is stale: run `node scripts/build-bundle.mjs` in its web/ and commit the result')
    process.exit(1)
  }
}
console.log(`automations bundle: ${entry.file} (${commit})`)
