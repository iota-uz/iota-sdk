import { execFileSync } from 'node:child_process'
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { publishSDK } from './sdk-publisher.mjs'

const repository = 'iota-uz/iota-sdk'
const run = (command, args, options = {}) => execFileSync(command, args, { encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe'], ...options })
const api = async (method, path, body, optional = false) => {
  const args = ['api', '--method', method, `repos/${repository}/${path}`]
  if (body) args.push('--input', '-')
  try {
    return JSON.parse(run('gh', args, { input: body ? JSON.stringify(body) : undefined }) || 'null')
  } catch (error) {
    if (optional && String(error.stderr).includes('HTTP 404')) return null
    throw error
  }
}
const releases = () => api('GET', 'releases?per_page=100')
const releaseFor = async tag => (await releases()).find(release => release.tag_name === tag)
const downloadAsset = asset => execFileSync('gh', ['api', '-H', 'Accept: application/octet-stream', `repos/${repository}/releases/assets/${asset.id}`])
const uploadAsset = (release, name, bytes) => {
  const dir = mkdtempSync(join(tmpdir(), 'iota-sdk-upload-'))
  try {
    const file = join(dir, name)
    writeFileSync(file, bytes)
    run('gh', ['api', '--method', 'POST', '-H', 'Content-Type: application/octet-stream', `repos/${repository}/releases/${release.id}/assets?name=${encodeURIComponent(name)}`, '--input', file])
  } finally {
    rmSync(dir, { recursive: true, force: true })
  }
}
const ensureAsset = async (release, name, bytes) => {
  const existing = release.assets?.find(asset => asset.name === name)
  if (existing) {
    if (!downloadAsset(existing).equals(bytes)) throw new Error(`Durable release asset ${name} differs from the verified artifact`)
    return
  }
  uploadAsset(release, name, bytes)
}

const manifest = JSON.parse(readFileSync('artifacts/frontend/frontend-artifacts.json', 'utf8'))
if (!/^[\w.-]+\.tgz$/.test(manifest.file)) throw new Error('Invalid artifact filename')
const tag = await publishSDK({
  sha: process.env.CANDIDATE_SHA,
  version: process.env.RELEASE_VERSION,
  manifest,
  bytes: readFileSync(`artifacts/frontend/${manifest.file}`),
  api,
  async backup(tag, manifest, bytes) {
    let release = await releaseFor(tag)
    if (release && !release.draft) return
    if (!release) {
      release = await api('POST', 'releases', {
        tag_name: tag, target_commitish: process.env.CANDIDATE_SHA, name: tag,
        body: `Publication in progress for ${process.env.CANDIDATE_SHA}.`, draft: true, prerelease: false,
      })
    }
    await ensureAsset(release, 'frontend-artifacts.json', Buffer.from(`${JSON.stringify(manifest, null, 2)}\n`))
    release = await releaseFor(tag)
    await ensureAsset(release, manifest.file, bytes)
  },
  async registry(version) {
    const response = await fetch(`https://registry.npmjs.org/@iota-uz%2fsdk/${version}`)
    if (response.status === 404) return null
    if (!response.ok) throw new Error(`npm registry returned ${response.status}`)
    return response.json()
  },
  async publish(file) { run('npm', ['publish', `artifacts/frontend/${file}`, '--access', 'public', '--provenance']) },
  async verifyGo(version) {
    const dir = mkdtempSync(join(tmpdir(), 'iota-sdk-go-verify-'))
    try {
      run('go', ['mod', 'init', 'verify.invalid/iota-sdk-release'], { cwd: dir })
      const module = JSON.parse(run('go', ['mod', 'download', '-json', `github.com/iota-uz/iota-sdk@v${version}`], { cwd: dir, env: { ...process.env, GOWORK: 'off' } }))
      if (module.Version !== `v${version}` || !module.Sum || module.Error) throw new Error('Published Go module is not retrievable')
    } finally {
      rmSync(dir, { recursive: true, force: true })
    }
  },
  async complete(tag, sha, body) {
    const release = await releaseFor(tag)
    if (!release) throw new Error('Durable release backup is missing')
    if (!release.draft) {
      if (release.prerelease || release.body !== body) throw new Error('Existing release completion record differs')
      return
    }
    await api('PATCH', `releases/${release.id}`, { tag_name: tag, target_commitish: sha, name: tag, body, draft: false, prerelease: false })
  },
  pause: () => new Promise(resolve => setTimeout(resolve, 5000)),
})
console.log(`Ready: ${tag}`)
