import { execFileSync } from 'node:child_process'
import { cp, mkdir, readFile, readdir, rm, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const packageRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const repositoryRoot = path.resolve(packageRoot, '../..')
const dist = path.join(packageRoot, 'dist')
const clientHostDist = path.resolve(packageRoot, '../client-host/dist')
const lensDist = path.resolve(packageRoot, '../lens/package-dist')
const packageJSON = JSON.parse(await readFile(path.join(packageRoot, 'package.json'), 'utf8'))
const sourceCommit = process.env.SDK_SOURCE_SHA?.trim()
  || execFileSync('git', ['rev-parse', 'HEAD'], { cwd: repositoryRoot, encoding: 'utf8' }).trim()
const releaseVersion = process.env.SDK_RELEASE_VERSION?.trim() || packageJSON.version
const protocolVersion = process.env.SDK_PROTOCOL_VERSION?.trim() || '1.0.0'

if (!/^[0-9a-f]{40}$/.test(sourceCommit)) {
  throw new Error(`SDK source commit must be a full Git SHA, got ${sourceCommit}`)
}
if (releaseVersion !== packageJSON.version) {
  throw new Error(`SDK_RELEASE_VERSION ${releaseVersion} does not match package.json ${packageJSON.version}`)
}

await rm(path.join(dist, 'client-host'), { recursive: true, force: true })
await rm(path.join(dist, 'lens'), { recursive: true, force: true })
await mkdir(dist, { recursive: true })
await cp(clientHostDist, path.join(dist, 'client-host'), { recursive: true })
await cp(lensDist, path.join(dist, 'lens'), { recursive: true })

const identity = {
  releaseVersion,
  sourceCommit,
  protocolVersion,
}
await writeFile(
  path.join(dist, 'sdk-identity.js'),
  [
    `export const SDK_RELEASE_VERSION = ${JSON.stringify(releaseVersion)}`,
    `export const SDK_SOURCE_COMMIT = ${JSON.stringify(sourceCommit)}`,
    `export const SDK_PROTOCOL_VERSION = ${JSON.stringify(protocolVersion)}`,
    `export const SDK_IDENTITY = Object.freeze(${JSON.stringify(identity)})`,
    '',
  ].join('\n'),
)
await writeFile(
  path.join(dist, 'sdk-identity.cjs'),
  [
    `'use strict'`,
    `const SDK_RELEASE_VERSION = ${JSON.stringify(releaseVersion)}`,
    `const SDK_SOURCE_COMMIT = ${JSON.stringify(sourceCommit)}`,
    `const SDK_PROTOCOL_VERSION = ${JSON.stringify(protocolVersion)}`,
    `const SDK_IDENTITY = Object.freeze(${JSON.stringify(identity)})`,
    `module.exports = { SDK_RELEASE_VERSION, SDK_SOURCE_COMMIT, SDK_PROTOCOL_VERSION, SDK_IDENTITY }`,
    '',
  ].join('\n'),
)
await writeFile(
  path.join(dist, 'sdk-identity.d.ts'),
  [
    'export declare const SDK_RELEASE_VERSION: string',
    'export declare const SDK_SOURCE_COMMIT: string',
    'export declare const SDK_PROTOCOL_VERSION: string',
    'export declare const SDK_IDENTITY: Readonly<{ releaseVersion: string; sourceCommit: string; protocolVersion: string }>',
    '',
  ].join('\n'),
)

async function replaceCommitMarker(directory) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const target = path.join(directory, entry.name)
    if (entry.isDirectory()) {
      await replaceCommitMarker(target)
      continue
    }
    if (!/\.(?:js|d\.ts)$/.test(entry.name)) continue
    const contents = await readFile(target, 'utf8')
    if (contents.includes('__IOTA_SDK_SOURCE_COMMIT__')) {
      await writeFile(target, contents.replaceAll('__IOTA_SDK_SOURCE_COMMIT__', sourceCommit))
    }
  }
}
await replaceCommitMarker(path.join(dist, 'client-host'))
await replaceCommitMarker(path.join(dist, 'lens'))

await writeFile(path.join(dist, 'sdk-identity.json'), `${JSON.stringify(identity, null, 2)}\n`)
