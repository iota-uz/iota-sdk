import { createHash } from 'node:crypto'

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
  if (!published) await publish(manifest.file)
  for (let attempt = 0; attempt < 12; attempt++) {
    published = await registry(version)
    if (published?.dist?.integrity && published.dist.attestations?.provenance?.predicateType === 'https://slsa.dev/provenance/v1') break
    await pause()
  }
  if (published?.dist?.integrity !== integrity || published?.dist?.attestations?.provenance?.predicateType !== 'https://slsa.dev/provenance/v1') {
    throw new Error('npm release integrity or provenance does not match the verified tarball')
  }
  await verifyGo(version)
  const body = `sdk-ready:${tag}\n\nVerified source: ${sha}\n\nGo and @iota-uz/sdk share version ${version}.`
  const release = await api('GET', `releases/tags/${tag}`, undefined, true)
  if (release) {
    if (release.draft || release.prerelease || release.body !== body) throw new Error('Existing release completion record differs')
  } else {
    await api('POST', 'releases', { tag_name: tag, target_commitish: sha, name: tag, body, draft: false, prerelease: false })
  }
  return tag
}
