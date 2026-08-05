import { describe, expect, it } from 'vitest'
import {
  SDK_IDENTITY,
  SDKIdentityMismatchError,
  assertSDKIdentity,
} from './identity'

describe('SDK identity', () => {
  it('accepts the exact release, source commit, and compatible protocol', () => {
    expect(() => assertSDKIdentity({ ...SDK_IDENTITY })).not.toThrow()
  })

  it('blocks mixed Go and JavaScript revisions before render', () => {
    expect(() => assertSDKIdentity({ ...SDK_IDENTITY, sourceCommit: '0'.repeat(40) }))
      .toThrow(SDKIdentityMismatchError)
    expect(() => assertSDKIdentity({ ...SDK_IDENTITY, releaseVersion: '0.4.44' }))
      .toThrow(SDKIdentityMismatchError)
    expect(() => assertSDKIdentity({ ...SDK_IDENTITY, protocolVersion: '2.0.0' }))
      .toThrow(SDKIdentityMismatchError)
  })
})
