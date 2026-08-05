import { CLIENT_HOST_PROTOCOL_VERSION } from './protocol'

export const SDK_RELEASE_VERSION = '0.5.1'
export const SDK_SOURCE_COMMIT = '__IOTA_SDK_SOURCE_COMMIT__'
export const SDK_PROTOCOL_VERSION = CLIENT_HOST_PROTOCOL_VERSION
export const SDK_IDENTITY = Object.freeze({
  releaseVersion: SDK_RELEASE_VERSION,
  sourceCommit: SDK_SOURCE_COMMIT,
  protocolVersion: SDK_PROTOCOL_VERSION,
})

export interface SDKRuntimeIdentity {
  releaseVersion: string
  sourceCommit: string
  protocolVersion: string
}

export class SDKIdentityMismatchError extends Error {
  readonly code = 'SDK_IDENTITY_MISMATCH'

  constructor(readonly expected: SDKRuntimeIdentity, readonly actual: SDKRuntimeIdentity) {
    super(`SDK runtime ${actual.releaseVersion}@${actual.sourceCommit} is incompatible with ${expected.releaseVersion}@${expected.sourceCommit}`)
    this.name = 'SDKIdentityMismatchError'
  }
}

export function assertSDKIdentity(actual: SDKRuntimeIdentity): void {
  const expected = SDK_IDENTITY
  if (
    actual.releaseVersion !== expected.releaseVersion
    || actual.sourceCommit !== expected.sourceCommit
    || actual.protocolVersion.split('.', 1)[0] !== expected.protocolVersion.split('.', 1)[0]
  ) {
    throw new SDKIdentityMismatchError(expected, actual)
  }
}
