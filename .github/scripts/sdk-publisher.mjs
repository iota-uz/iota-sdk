import { createHash } from 'node:crypto'

export class PublicationVerificationError extends Error {
  constructor(code, version, expectedIntegrity, actualIntegrity) {
    super(`${code}: npm ${version}; expected integrity ${expectedIntegrity}; observed ${actualIntegrity ?? 'not visible'}`)
    this.name = 'PublicationVerificationError'
    this.code = code
    this.version = version
  }
}

export async function publishSDK({ sha, version, manifest, bytes, api, registry, publish, verifyGo, pause }) {
  if (!/^[a-f0-9]{40}$/.test(sha ?? '') || !/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/.test(version ?? '')) {
    throw new Error('Invalid release identity')
  }
  if (manifest.sdkCommit !== sha || manifest.sdkReleaseVersion !== version || manifest.packageVersion !== version || manifest.package !== '@iota-uz/sdk' || !/^[\w.-]+\.tgz$/.test(manifest.file)) {
    throw new Error('Verified artifact does not match the release candidate')
  }
  if (createHash('sha256').update(bytes).digest('hex') !== manifest.sha256) throw new Error('Artifact checksum mismatch')
  const integrity = `sha512-${createHash('sha512').update(bytes).digest('base64')}`
  const tag = `v${version}`
  const existingTag = await api('GET', `git/ref/tags/${tag}`, undefined, true)
  if (existingTag) {
    if (existingTag.object.type !== 'commit' || existingTag.object.sha !== sha) throw new Error('Tag is already bound to another commit')
  } else {
    await api('POST', 'git/refs', { ref: `refs/tags/${tag}`, sha })
  }
  let published = await registry(version)
  const checkConflict = metadata => {
    const actualIntegrity = metadata?.dist?.integrity
    if (actualIntegrity && actualIntegrity !== integrity) {
      throw new PublicationVerificationError('npm_integrity_mismatch', version, integrity, actualIntegrity)
    }
    const predicate = metadata?.dist?.attestations?.provenance?.predicateType
    if (predicate && predicate !== 'https://slsa.dev/provenance/v1') {
      throw new PublicationVerificationError('npm_provenance_mismatch', version, integrity, actualIntegrity)
    }
  }
  checkConflict(published)
  if (!published) await publish(manifest.file)
  for (let attempt = 0; attempt < 36; attempt++) {
    published = await registry(version)
    const actualIntegrity = published?.dist?.integrity
    checkConflict(published)
    const predicate = published?.dist?.attestations?.provenance?.predicateType
    if (actualIntegrity && predicate) break
    if (attempt < 35) await pause()
  }
  if (published?.dist?.integrity !== integrity || published?.dist?.attestations?.provenance?.predicateType !== 'https://slsa.dev/provenance/v1') {
    throw new PublicationVerificationError(published?.dist?.integrity ? 'npm_provenance_timeout' : 'npm_visibility_timeout', version, integrity, published?.dist?.integrity)
  }
  await verifyGo(version)
  return tag
}
