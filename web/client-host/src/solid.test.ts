// @vitest-environment jsdom
import { type Component } from 'solid-js'
import { describe, expect, it, vi } from 'vitest'
import { CLIENT_BOOTSTRAP_VERSION } from './bootstrap'
import { SDK_IDENTITY } from './identity'
import { mountSolidClientRoute } from './solid'

describe('Solid client route host', () => {
  it('owns one subtree and disposes component and host services exactly once', () => {
    const root = document.createElement('div')
    const services = {
      session: { snapshot: () => ({}), refresh: async () => ({}) },
      navigation: { guard: () => () => undefined, canNavigate: () => true },
      theme: { current: () => 'light' as const, set: () => undefined, subscribe: () => () => undefined },
      locale: { language: 'en', t: (key: string) => key },
      telemetry: { emit: () => undefined },
      dispose: vi.fn(),
    }
    const component: Component<{ route: { initial: { name: string } } }> = (props) => {
      const element = document.createElement('span')
      element.textContent = props.route.initial.name
      return element
    }
    const context = {
      bootstrapVersion: CLIENT_BOOTSTRAP_VERSION,
      protocolVersion: SDK_IDENTITY.protocolVersion,
      sdkReleaseVersion: SDK_IDENTITY.releaseVersion,
      sdkCommit: SDK_IDENTITY.sourceCommit,
      initial: { name: 'Granite' },
      theme: 'light' as const,
    }
    const dispose = mountSolidClientRoute({ root, component, props: {}, context, services })
    expect(root.textContent).toBe('Granite')
    expect(root.dataset.iotaClientOwner).toBe('solid')
    expect(() => mountSolidClientRoute({ root, component, props: {}, context, services })).toThrow('already owned')
    dispose()
    dispose()
    expect(root.childElementCount).toBe(0)
    expect(services.dispose).toHaveBeenCalledTimes(1)
    expect(root.dataset.iotaClientOwner).toBeUndefined()
  })

  it('releases ownership and services when rendering fails', () => {
    const root = document.createElement('div')
    const services = {
      session: { snapshot: () => ({}), refresh: async () => ({}) },
      navigation: { guard: () => () => undefined, canNavigate: () => true },
      theme: { current: () => 'light' as const, set: () => undefined, subscribe: () => () => undefined },
      locale: { language: 'en', t: (key: string) => key },
      telemetry: { emit: () => undefined },
      dispose: vi.fn(),
    }
    const context = {
      bootstrapVersion: CLIENT_BOOTSTRAP_VERSION,
      protocolVersion: SDK_IDENTITY.protocolVersion,
      sdkReleaseVersion: SDK_IDENTITY.releaseVersion,
      sdkCommit: SDK_IDENTITY.sourceCommit,
      initial: {},
      theme: 'light' as const,
    }
    const component: Component = () => { throw new Error('render failed') }
    expect(() => mountSolidClientRoute({ root, component, props: {}, context, services })).toThrow('render failed')
    expect(services.dispose).toHaveBeenCalledOnce()
    expect(root.dataset.iotaClientOwner).toBeUndefined()
  })

  it('releases ownership and preserves render failures when service cleanup also fails', () => {
    const root = document.createElement('div')
    const services = {
      session: { snapshot: () => ({}), refresh: async () => ({}) },
      navigation: { guard: () => () => undefined, canNavigate: () => true },
      theme: { current: () => 'light' as const, set: () => undefined, subscribe: () => () => undefined },
      locale: { language: 'en', t: (key: string) => key },
      telemetry: { emit: () => undefined },
      dispose: vi.fn(() => { throw new Error('service cleanup failed') }),
    }
    const context = {
      bootstrapVersion: CLIENT_BOOTSTRAP_VERSION,
      protocolVersion: SDK_IDENTITY.protocolVersion,
      sdkReleaseVersion: SDK_IDENTITY.releaseVersion,
      sdkCommit: SDK_IDENTITY.sourceCommit,
      initial: {},
      theme: 'light' as const,
    }
    const component: Component = () => { throw new Error('render failed') }
    let failure: unknown
    try {
      mountSolidClientRoute({ root, component, props: {}, context, services })
    } catch (cause) {
      failure = cause
    }
    expect(failure).toBeInstanceOf(AggregateError)
    expect((failure as AggregateError).errors).toEqual([
      expect.objectContaining({ message: 'render failed' }),
      expect.objectContaining({ message: 'service cleanup failed' }),
    ])
    expect(root.dataset.iotaClientOwner).toBeUndefined()
  })

  it('releases ownership when service cleanup fails during unmount', () => {
    const root = document.createElement('div')
    const services = {
      session: { snapshot: () => ({}), refresh: async () => ({}) },
      navigation: { guard: () => () => undefined, canNavigate: () => true },
      theme: { current: () => 'light' as const, set: () => undefined, subscribe: () => () => undefined },
      locale: { language: 'en', t: (key: string) => key },
      telemetry: { emit: () => undefined },
      dispose: vi.fn(() => { throw new Error('service cleanup failed') }),
    }
    const context = {
      bootstrapVersion: CLIENT_BOOTSTRAP_VERSION,
      protocolVersion: SDK_IDENTITY.protocolVersion,
      sdkReleaseVersion: SDK_IDENTITY.releaseVersion,
      sdkCommit: SDK_IDENTITY.sourceCommit,
      initial: {},
      theme: 'light' as const,
    }
    const component: Component = () => document.createElement('span')
    const dispose = mountSolidClientRoute({ root, component, props: {}, context, services })
    expect(dispose).toThrow('service cleanup failed')
    expect(services.dispose).toHaveBeenCalledOnce()
    expect(root.dataset.iotaClientOwner).toBeUndefined()
  })
})
