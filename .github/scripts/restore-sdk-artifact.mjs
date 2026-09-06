import { createHash } from 'node:crypto'
import { execFileSync } from 'node:child_process'
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'

const repository = 'iota-uz/iota-sdk'
const directory = 'artifacts/frontend'
const sha = process.env.CANDIDATE_SHA
const version = process.env.RELEASE_VERSION
const run = args => execFileSync('gh', args, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] })
const download = (asset, path) => writeFileSync(path, execFileSync('gh', ['api', '-H', 'Accept: application/octet-stream', `repos/${repository}/releases/assets/${asset.id}`]))

mkdirSync(directory, { recursive: true })
const manifestPath = `${directory}/frontend-artifacts.json`
const findRelease = () => JSON.parse(run(['api', `repos/${repository}/releases?per_page=100`])).find(value => value.tag_name === `v${version}`)
if (!existsSync(manifestPath)) {
  const asset = findRelease()?.assets?.find(value => value.name === 'frontend-artifacts.json')
  if (!asset) throw new Error('Verified artifact is absent from both Actions and the durable draft release')
  download(asset, manifestPath)
}
const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'))
if (manifest.sdkCommit !== sha || manifest.sdkReleaseVersion !== version || manifest.packageVersion !== version || !/^[\w.-]+\.tgz$/.test(manifest.file)) {
  throw new Error('Restored artifact identity does not match the release candidate')
}
const archivePath = `${directory}/${manifest.file}`
if (!existsSync(archivePath)) {
  const asset = findRelease()?.assets?.find(value => value.name === manifest.file)
  if (!asset) throw new Error('Durable release archive is missing')
  download(asset, archivePath)
}
if (createHash('sha256').update(readFileSync(archivePath)).digest('hex') !== manifest.sha256) throw new Error('Restored artifact checksum mismatch')
