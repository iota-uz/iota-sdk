import { describe, expect, it } from 'vitest'
import { CLIENT_BOOTSTRAP_VERSION, ClientHostProtocolError, assertClientBootstrap } from './bootstrap'
import { SDK_IDENTITY } from './identity'

describe('client bootstrap', () => {
  const context = {
    bootstrapVersion: CLIENT_BOOTSTRAP_VERSION,
    protocolVersion: SDK_IDENTITY.protocolVersion,
    sdkReleaseVersion: SDK_IDENTITY.releaseVersion,
    sdkCommit: SDK_IDENTITY.sourceCommit,
    initial: {},
    theme: 'light' as const,
  }

  it('validates bootstrap and SDK runtime identity together', () => {
    expect(() => assertClientBootstrap(context)).not.toThrow()
  })

  it('rejects unsupported and omitted bootstrap versions', () => {
    expect(() => assertClientBootstrap({ ...context, bootstrapVersion: '2.0.0' })).toThrow(ClientHostProtocolError)
    const { bootstrapVersion: _, ...unversioned } = context
    expect(() => assertClientBootstrap(unversioned)).toThrow(ClientHostProtocolError)
  })
})
