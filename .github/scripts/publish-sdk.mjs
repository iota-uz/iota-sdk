import { execFileSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { publishSDK } from './sdk-publisher.mjs'

const run = (command, args, input) => execFileSync(command, args, { encoding: 'utf8', input, stdio: ['pipe', 'pipe', 'pipe'] })
const manifest = JSON.parse(readFileSync('artifacts/frontend/frontend-artifacts.json', 'utf8'))
if (!/^[\w.-]+\.tgz$/.test(manifest.file)) throw new Error('Invalid artifact filename')
const tag = await publishSDK({
  sha: process.env.CANDIDATE_SHA,
  version: process.env.RELEASE_VERSION,
  manifest,
  bytes: readFileSync(`artifacts/frontend/${manifest.file}`),
  async api(method, path, body, optional = false) {
    const args = ['api', '--method', method, `repos/iota-uz/iota-sdk/${path}`]
    if (body) args.push('--input', '-')
    try {
      return JSON.parse(run('gh', args, body ? JSON.stringify(body) : undefined) || 'null')
    } catch (error) {
      if (optional && String(error.stderr).includes('HTTP 404')) return null
      throw error
    }
  },
  async registry(version) {
    const response = await fetch(`https://registry.npmjs.org/@iota-uz%2fsdk/${version}`)
    if (response.status === 404) return null
    if (!response.ok) throw new Error(`npm registry returned ${response.status}`)
    return response.json()
  },
  async publish(file) { run('npm', ['publish', `artifacts/frontend/${file}`, '--access', 'public', '--provenance']) },
  async verifyGo(version) {
    const module = JSON.parse(run('go', ['mod', 'download', '-json', `github.com/iota-uz/iota-sdk@v${version}`]))
    if (module.Version !== `v${version}` || !module.Sum || module.Error) throw new Error('Published Go module is not retrievable')
  },
  pause: () => new Promise(resolve => setTimeout(resolve, 5000)),
})
console.log(`Ready: ${tag}`)
