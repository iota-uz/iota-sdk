// Builds the bundle the automations module embeds (../dist, served at /assets/automations/) and
// writes dist/bundle.json, which the Go side reads into a clienthost.Manifest.
//
//   node scripts/build-bundle.mjs          # rebuild
//   node scripts/build-bundle.mjs --check  # fail when the committed bundle is stale
//
// The build always goes to a temporary directory first: the check compares it with ../dist without
// touching it, and a rebuild swaps the finished directory in, so ../dist (read by go:embed) is never
// half-written.
import { spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { existsSync, mkdtempSync, readFileSync, readdirSync, renameSync, rmSync, statSync, writeFileSync } from 'node:fs'
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

// The bundle's utilities sit in the same cascade layer as the application's stylesheet and load after
// it, so an unscoped `.hidden` would beat the layout's `lg:flex` (the sidebar disappeared). They are
// nested under the client route's mount and portals, so they apply to the screen and nothing else.
const SCOPE = ':is(#iota-client-route,#iota-client-route-portals)'
function scopeUtilities(file) {
  const css = readFileSync(file, 'utf8')
  const open = '@layer utilities{'
  const start = css.indexOf(open)
  if (start < 0) throw new Error('no utilities layer in the bundle stylesheet')
  let depth = 1
  let i = start + open.length
  for (; i < css.length && depth > 0; i++) {
    if (css[i] === '{') depth++
    else if (css[i] === '}') depth--
  }
  const end = i - 1 // the brace closing the layer
  const body = css.slice(start + open.length, end)
  let scoped = css.slice(0, start) + open + SCOPE + '{' + body + '}' + css.slice(end)
  // The theme tokens the utilities read are declared on the same mount, not on :root: an application's
  // own tokens (a custom --spacing) stay what they are outside the screen. (theme(reference) would emit
  // none, and the host stylesheet declares only the tokens it uses itself.)
  const root = '@layer theme{:root,:host{'
  if (!scoped.includes(root)) throw new Error('no theme layer in the bundle stylesheet')
  scoped = scoped.replace(root, '@layer theme{' + SCOPE + '{')
  if (/:root|:host/.test(scoped)) throw new Error('the bundle stylesheet still declares global selectors')
  writeFileSync(file, scoped)
}

function files(dir, base = dir) {
  return readdirSync(dir).flatMap((name) => {
    const p = path.join(dir, name)
    return statSync(p).isDirectory() ? files(p, base) : [path.relative(base, p)]
  }).sort()
}

// A sibling of ../dist (same file system, so the swap is a rename), outside the embedded directory.
const out = mkdtempSync(path.resolve(web, '../.dist-build-'))
process.on('exit', () => rmSync(out, { recursive: true, force: true }))

run(path.join(web, 'node_modules/.bin/tailwindcss'), ['--input', 'bundle/bundle.css', '--output', 'bundle/bundle.generated.css', '--minify'])
scopeUtilities(path.join(web, 'bundle/bundle.generated.css'))
run(path.join(web, 'node_modules/.bin/vite'), ['build', '--config', 'vite.bundle.config.ts', '--outDir', out, '--emptyOutDir'])

const manifest = JSON.parse(readFileSync(path.join(out, '.vite/manifest.json'), 'utf8'))
const entry = manifest['bundle/entry.ts']
if (!entry) throw new Error('bundle entry is missing from the Vite manifest')

// The identity is the hash of what was built (with the sentinel in place), then written in.
const hash = createHash('sha1')
for (const f of files(out)) if (!f.startsWith('.vite')) hash.update(f).update(readFileSync(path.join(out, f)))
const commit = hash.digest('hex')
for (const f of files(out)) {
  if (!f.endsWith('.js')) continue
  const p = path.join(out, f)
  const text = readFileSync(p, 'utf8')
  if (text.includes(SENTINEL)) writeFileSync(p, text.split(SENTINEL).join(commit))
}
const protocol = /CLIENT_HOST_PROTOCOL_VERSION = ['"]([^'"]+)['"]/.exec(readFileSync(path.join(web, '../../../../../web/client-host/src/protocol.ts'), 'utf8'))?.[1]
if (!protocol) throw new Error('cannot read the client host protocol version')
if (!files(out).some((f) => f.endsWith('.js') && readFileSync(path.join(out, f), 'utf8').includes('0.0.0-iota-automations-release'))) {
  throw new Error('the release placeholder is missing from the bundle')
}
rmSync(path.join(out, '.vite'), { recursive: true, force: true })
writeFileSync(path.join(out, 'bundle.json'), JSON.stringify({
  sdkCommit: commit,
  // The SDK release is the serving binary's: assets.go writes it over this placeholder.
  releasePlaceholder: '0.0.0-iota-automations-release',
  protocolVersion: protocol,
  entry: entry.file,
  styles: entry.css ?? [],
}, null, 2) + '\n')

if (check) {
  const now = files(out)
  const before = existsSync(dist) ? files(dist) : []
  const stale = JSON.stringify(now) !== JSON.stringify(before) ||
    now.some((f) => !readFileSync(path.join(out, f)).equals(readFileSync(path.join(dist, f))))
  if (stale) {
    console.error('modules/automations/presentation/graphs/dist is stale: run `node scripts/build-bundle.mjs` in its web/ and commit the result')
    process.exit(1)
  }
} else {
  const old = `${dist}.old`
  rmSync(old, { recursive: true, force: true })
  if (existsSync(dist)) renameSync(dist, old)
  renameSync(out, dist)
  rmSync(old, { recursive: true, force: true })
}
console.log(`automations graphs bundle: ${entry.file} (${commit})`)
