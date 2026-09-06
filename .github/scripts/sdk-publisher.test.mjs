import test from 'node:test'
import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { publishSDK } from './sdk-publisher.mjs'

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
  assert.deepEqual(f.state.writes, ['git/refs', 'releases'])
})

test('resumes a tag-only partial release at the same SHA', async () => {
  const f = fixture()
  f.state.tag = { object: { type: 'commit', sha: f.args.sha } }
  await publishSDK(f.args)
  assert.deepEqual(f.state.writes, ['releases'])
  assert.equal(f.state.publishCount, 1)
})

test('resumes npm success followed by GitHub failure without republishing', async () => {
  const f = fixture()
  const api = f.args.api
  f.args.api = async (method, path, ...rest) => {
    if (path === 'releases') throw new Error('GitHub unavailable')
    return api(method, path, ...rest)
  }
  await assert.rejects(publishSDK(f.args), /GitHub unavailable/)
  assert.equal(f.state.release, null)
  f.args.api = api
  await publishSDK(f.args)
  assert.equal(f.state.publishCount, 1)
  assert.ok(f.state.release)
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
  await assert.rejects(publishSDK(f.args), /integrity or provenance/)
  assert.equal(f.state.release, null)
  assert.equal(f.state.publishCount, 0)
})

test('never declares completion without provenance', async () => {
  const f = fixture()
  f.state.npm = { dist: { integrity: f.registryValue.dist.integrity } }
  await assert.rejects(publishSDK(f.args), /integrity or provenance/)
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
