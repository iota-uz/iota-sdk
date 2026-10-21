import test from 'node:test'
import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { publishSDK } from './sdk-publisher.mjs'
import { releaseRegistry, RetryableRegistryError } from './npm-release-registry.mjs'

// These tests would be falsely green if the fake registry always returned the
// expected identity; each mismatch test changes the external registry response.
function fixture() {
  const sha = 'a'.repeat(40)
  const version = '0.6.0'
  const bytes = Buffer.from('verified tarball')
  const integrity = `sha512-${createHash('sha512').update(bytes).digest('base64')}`
  const registryValue = { dist: { integrity, attestations: { provenance: { predicateType: 'https://slsa.dev/provenance/v1' } } } }
  const state = { tag: null, release: null, npm: null, publishCount: 0, writes: [] }
  return {
    state, registryValue,
    args: {
      sha, version, bytes,
      manifest: { sdkCommit: sha, sdkReleaseVersion: version, packageVersion: version, package: '@iota-uz/sdk', file: 'sdk.tgz', sha256: createHash('sha256').update(bytes).digest('hex') },
      async api(method, path, body) {
        if (method === 'GET' && path.startsWith('git/ref/')) return state.tag
        if (method === 'GET' && path.startsWith('releases/')) return state.release
        state.writes.push(path)
        if (path === 'git/refs') state.tag = { object: { type: 'commit', sha: body.sha } }
        else if (path === 'releases') state.release = body
        else throw new Error(`Unexpected API ${method} ${path}`)
      },
      async registry() { return state.npm },
      async publish() { state.publishCount++; state.npm = registryValue },
      async verifyGo() {},
      async pause() {},
    },
  }
}

test('publishes once and resumes after completion without another npm write', async () => {
  const f = fixture()
  await publishSDK(f.args)
  await publishSDK(f.args)
  assert.equal(f.state.publishCount, 1)
  assert.deepEqual(f.state.writes, ['git/refs'])
})

test('resumes a tag-only partial release at the same SHA', async () => {
  const f = fixture()
  f.state.tag = { object: { type: 'commit', sha: f.args.sha } }
  await publishSDK(f.args)
  assert.deepEqual(f.state.writes, [])
  assert.equal(f.state.publishCount, 1)
})

test('resumes npm success followed by Go lookup failure without republishing', async () => {
  const f = fixture()
  const verifyGo = f.args.verifyGo
  f.args.verifyGo = async () => { throw new Error('Go unavailable') }
  await assert.rejects(publishSDK(f.args), /Go unavailable/)
  assert.equal(f.state.release, null)
  f.args.verifyGo = verifyGo
  await publishSDK(f.args)
  assert.equal(f.state.publishCount, 1)
  assert.equal(f.state.publishCount, 1)
})

test('rejects a conflicting tag without publishing npm', async () => {
  const f = fixture()
  f.state.tag = { object: { type: 'commit', sha: 'b'.repeat(40) } }
  await assert.rejects(publishSDK(f.args), /another commit/)
  assert.equal(f.state.publishCount, 0)
})

test('never declares completion for different registry bytes', async () => {
  const f = fixture()
  f.state.npm = structuredClone(f.registryValue)
  f.state.npm.dist.integrity = 'sha512-other'
  await assert.rejects(publishSDK(f.args), error => error.code === 'npm_integrity_mismatch')
  assert.equal(f.state.release, null)
  assert.equal(f.state.publishCount, 0)
})

test('never declares completion without provenance', async () => {
  const f = fixture()
  f.state.npm = { dist: { integrity: f.registryValue.dist.integrity } }
  await assert.rejects(publishSDK(f.args), error => error.code === 'npm_provenance_timeout')
  assert.equal(f.state.release, null)
})

test('rejects a corrupted artifact before creating any public version', async () => {
  const f = fixture()
  f.args.bytes = Buffer.from('corrupted')
  await assert.rejects(publishSDK(f.args), /checksum/)
  assert.deepEqual(f.state.writes, [])
})

test('does not declare completion until the Go module is retrievable', async () => {
  const f = fixture()
  f.args.verifyGo = async () => { throw new Error('Go proxy unavailable') }
  await assert.rejects(publishSDK(f.args), /Go proxy unavailable/)
  assert.equal(f.state.release, null)
})

test('rejects a package for another commit before creating a tag', async () => {
  const f = fixture()
  f.args.manifest.sdkCommit = 'b'.repeat(40)
  await assert.rejects(publishSDK(f.args), /candidate/)
  assert.deepEqual(f.state.writes, [])
})

// Falsely green if publish() immediately makes metadata visible to every read.
test('waits for delayed npm visibility without republishing the verified artifact', async () => {
  const f = fixture()
  let reads = 0
  let verified = false
  f.args.registry = async () => ++reads > 20 ? f.registryValue : null
  f.args.publish = async () => { f.state.publishCount++ }
  f.args.verifyGo = async () => { verified = true }
  assert.equal(await publishSDK(f.args), `v${f.args.version}`)
  assert.equal(f.state.publishCount, 1)
  assert.equal(verified, true)
  assert.deepEqual(f.state.writes, ['git/refs'])
})

// Falsely green if provenance is already present in the first registry read.
test('waits for provenance to become visible on the same existing npm version', async () => {
  const f = fixture()
  let reads = 0
  f.args.registry = async () => ++reads > 20 ? f.registryValue : { dist: { integrity: f.registryValue.dist.integrity } }
  assert.equal(await publishSDK(f.args), `v${f.args.version}`)
  assert.equal(f.state.publishCount, 0)
})

// Falsely green if an absent version is mistaken for a corrupt published tarball.
test('reports bounded visibility timeout and preserves the partial publication', async () => {
  const f = fixture()
  let polls = 0
  let verified = false
  f.args.registry = async () => { polls++; return null }
  f.args.publish = async () => { f.state.publishCount++ }
  f.args.verifyGo = async () => { verified = true }
  await assert.rejects(publishSDK(f.args), error => error.code === 'npm_visibility_timeout')
  assert.equal(f.state.publishCount, 1)
  assert.equal(polls, 37)
  assert.equal(verified, false)
  assert.equal(f.state.tag.object.sha, f.args.sha)
})

// Falsely green if retry polling accepts registry bytes from another release.
test('fails a verified integrity conflict immediately instead of waiting it out', async () => {
  const f = fixture()
  let pauses = 0
  f.args.pause = async () => { pauses++ }
  f.state.npm = { dist: { integrity: 'sha512-other' } }
  await assert.rejects(publishSDK(f.args), error => error.code === 'npm_integrity_mismatch')
  assert.equal(pauses, 0)
  assert.equal(f.state.publishCount, 0)
})

// Falsely green if the verification adapter repeatedly requests a cached 404 URL.
test('registry verification requests bypass cached metadata and bound each network read', async () => {
  const requests = []
  let timestamp = 100
  const registry = releaseRegistry(async (url, options) => {
    requests.push({ url: String(url), options })
    return new Response(JSON.stringify({ version: '0.6.0' }), { status: 200 })
  }, () => timestamp++)
  await registry('0.6.0')
  await registry('0.6.0')
  assert.notEqual(requests[0].url, requests[1].url)
  assert.equal(requests[0].options.cache, 'no-store')
  assert.equal(requests[0].options.headers['Cache-Control'], 'no-cache')
  assert.ok(requests[0].options.signal instanceof AbortSignal)
})

// Falsely green if later metadata hides a conflict observed before publication.
test('retains the first observed conflict even when later reads would match', async () => {
  for (const code of ['npm_integrity_mismatch', 'npm_provenance_mismatch']) {
    const f = fixture()
    const conflict = structuredClone(f.registryValue)
    if (code === 'npm_integrity_mismatch') conflict.dist.integrity = 'sha512-other'
    else conflict.dist.attestations.provenance.predicateType = 'unexpected-predicate'
    let reads = 0
    let verified = false
    f.args.registry = async () => ++reads === 1 ? conflict : f.registryValue
    f.args.verifyGo = async () => { verified = true }
    await assert.rejects(publishSDK(f.args), error => error.code === code)
    assert.equal(reads, 1)
    assert.equal(f.state.publishCount, 0)
    assert.equal(verified, false)
  }
})

// Falsely green if the registry never fails after the package has been published.
test('retries transient verification failures without republishing', async () => {
  const f = fixture()
  let reads = 0
  f.args.registry = async () => {
    if (++reads === 1) return null
    if (reads < 5) throw new RetryableRegistryError('registry temporarily unavailable')
    return f.registryValue
  }
  await publishSDK(f.args)
  assert.equal(f.state.publishCount, 1)
  assert.equal(reads, 5)
})

// Falsely green if unavailable registry state is treated as permission to publish.
test('keeps the initial registry failure fatal and bounds post-publish retries', async () => {
  const f = fixture()
  const failure = new RetryableRegistryError('registry temporarily unavailable')
  f.args.registry = async () => { throw failure }
  await assert.rejects(publishSDK(f.args), error => error === failure)
  assert.equal(f.state.publishCount, 0)
  let reads = 0
  let verified = false
  f.args.registry = async () => ++reads === 1 ? null : Promise.reject(failure)
  f.args.verifyGo = async () => { verified = true }
  await assert.rejects(publishSDK(f.args), error => error.code === 'npm_registry_timeout' && error.cause === failure)
  assert.equal(reads, 37)
  assert.equal(f.state.publishCount, 1)
  assert.equal(verified, false)
})

// Falsely green if authentication failures or malformed successful JSON become retryable.
test('registry adapter distinguishes transient transport failures from invalid responses', async () => {
  for (const status of [401, 403, 429]) {
    await assert.rejects(releaseRegistry(async () => new Response('', { status }))('0.6.0'), error => !(error instanceof RetryableRegistryError))
  }
  await assert.rejects(releaseRegistry(async () => new Response('', { status: 503 }))('0.6.0'), RetryableRegistryError)
  await assert.rejects(releaseRegistry(async () => { throw new TypeError('network failure') })('0.6.0'), RetryableRegistryError)
  await assert.rejects(releaseRegistry(async () => new Response('invalid json'))('0.6.0'), SyntaxError)
  assert.equal(await releaseRegistry(async () => new Response('', { status: 404 }))('0.6.0'), null)
})
