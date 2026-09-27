import { execFileSync } from 'node:child_process'
import { mkdtempSync, readFileSync, rmSync } from 'node:fs'
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
const manifest = JSON.parse(readFileSync('artifacts/frontend/frontend-artifacts.json', 'utf8'))
if (!/^[\w.-]+\.tgz$/.test(manifest.file)) throw new Error('Invalid artifact filename')
const tag = await publishSDK({
  sha: process.env.CANDIDATE_SHA,
  version: process.env.RELEASE_VERSION,
  manifest,
  bytes: readFileSync(`artifacts/frontend/${manifest.file}`),
  api,
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
  pause: () => new Promise(resolve => setTimeout(resolve, 5000)),
})
console.log(`Ready: ${tag}`)
