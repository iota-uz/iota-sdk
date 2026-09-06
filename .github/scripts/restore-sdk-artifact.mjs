import { createHash } from 'node:crypto'
import { readFileSync } from 'node:fs'

const directory = 'artifacts/frontend'
const sha = process.env.CANDIDATE_SHA
const version = process.env.RELEASE_VERSION
const manifestPath = `${directory}/frontend-artifacts.json`
const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'))
if (manifest.sdkCommit !== sha || manifest.sdkReleaseVersion !== version || manifest.packageVersion !== version || !/^[\w.-]+\.tgz$/.test(manifest.file)) {
  throw new Error('Restored artifact identity does not match the release candidate')
}
const archivePath = `${directory}/${manifest.file}`
if (createHash('sha256').update(readFileSync(archivePath)).digest('hex') !== manifest.sha256) throw new Error('Restored artifact checksum mismatch')
