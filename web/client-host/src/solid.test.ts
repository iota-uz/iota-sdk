// @vitest-environment jsdom
import { createComponent, type Component } from 'solid-js'
import { describe, expect, it, vi } from 'vitest'
import { CLIENT_BOOTSTRAP_VERSION } from './bootstrap'
import { SDK_IDENTITY } from './identity'
import { mountSolidClientRoute, mountSolidFeatureCatalogFromDocument } from './solid'
import { Portal, WidgetSlot } from './solid-portals'

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
    const background = document.createElement('main')
    const portalOwner = document.createElement('div')
    portalOwner.dataset.theme = 'shell'
    background.append(root)
    document.body.append(background, portalOwner)
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
    const component: Component = () => {
      createComponent(Portal, { surface: 'modal', label: 'Failure' })
      throw new Error('render failed')
    }
    expect(() => mountSolidClientRoute({ root, portals: portalOwner, background, component, props: {}, context, services })).toThrow('render failed')
    expect(services.dispose).toHaveBeenCalledOnce()
    expect(root.dataset.iotaClientOwner).toBeUndefined()
    expect(portalOwner.childElementCount).toBe(0)
    expect(portalOwner.dataset.theme).toBe('shell')
    expect(background.inert).toBeFalsy()
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

  it('uses canonical portal and widget surfaces with focus, inert and cleanup semantics', () => {
    document.body.replaceChildren()
    document.documentElement.style.overflow = 'clip'
    const background = document.createElement('main')
    background.dataset.clientRouteBackground = ''
    background.inert = false
    background.setAttribute('aria-hidden', 'false')
    const trigger = document.createElement('button')
    trigger.textContent = 'Open'
    const root = document.createElement('div')
    root.id = 'iota-client-route-mount'
    const portalOwner = document.createElement('div')
    portalOwner.id = 'iota-client-route-portals'
    portalOwner.dataset.theme = 'shell'
    const widget = document.createElement('div')
    widget.dataset.iotaWidgetSlot = 'header-actions'
    background.append(trigger, root)
    document.body.append(background, portalOwner, widget)
    trigger.focus()

    const close = document.createElement('button')
    close.textContent = 'Close'
    const secondary = document.createElement('button')
    secondary.textContent = 'Secondary'
    const widgetButton = document.createElement('button')
    widgetButton.textContent = 'Widget'
    const onEscape = vi.fn()
    const services = {
      session: { snapshot: () => ({}), refresh: async () => ({}) },
      navigation: { guard: () => () => undefined, canNavigate: () => true },
      theme: { current: () => 'dark' as const, set: () => undefined, subscribe: () => () => undefined },
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
      theme: 'dark' as const,
    }
    const component: Component = () => [
      createComponent(WidgetSlot, { name: 'header-actions', get children() { return widgetButton } }),
      createComponent(Portal, {
        surface: 'drawer',
        label: 'Details',
        class: 'drawer-content',
        onEscape,
        get children() { return [close, secondary] },
      }),
    ]

    const dispose = mountSolidClientRoute({ root, component, props: {}, context, services })
    const surface = portalOwner.querySelector<HTMLElement>('[data-iota-surface="drawer"]')
    expect(surface?.className).toBe('drawer-content')
    expect(surface?.getAttribute('role')).toBe('dialog')
    expect(surface?.getAttribute('aria-modal')).toBe('true')
    expect(portalOwner.querySelector('[data-iota-portal="drawer"]')).not.toBeNull()
    expect(portalOwner.dataset.theme).toBe('dark')
    expect(widget.textContent).toBe('Widget')
    expect(background.inert).toBe(true)
    expect(background.getAttribute('aria-hidden')).toBe('true')
    expect(document.documentElement.style.overflow).toBe('hidden')
    expect(document.activeElement).toBe(close)

    secondary.focus()
    secondary.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true }))
    expect(document.activeElement).toBe(close)
    close.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, bubbles: true, cancelable: true }))
    expect(document.activeElement).toBe(secondary)
    secondary.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }))
    expect(onEscape).toHaveBeenCalledOnce()

    dispose()
    expect(document.activeElement).toBe(trigger)
    expect(background.inert).toBe(false)
    expect(background.getAttribute('aria-hidden')).toBe('false')
    expect(document.documentElement.style.overflow).toBe('clip')
    expect(portalOwner.dataset.theme).toBe('shell')
    expect(portalOwner.childElementCount).toBe(0)
    expect(widget.childElementCount).toBe(0)
    expect(services.dispose).toHaveBeenCalledOnce()
    document.documentElement.style.overflow = ''
  })

  it('keeps toast surfaces non-blocking and does not steal focus', () => {
    document.body.replaceChildren()
    const background = document.createElement('main')
    background.inert = false
    const root = document.createElement('div')
    const portalOwner = document.createElement('div')
    const trigger = document.createElement('button')
    background.append(trigger, root)
    document.body.append(background, portalOwner)
    trigger.focus()
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
    const component: Component = () => createComponent(Portal, {
      surface: 'toast',
      label: 'Saved',
      get children() { return 'Saved successfully' },
    })

    const dispose = mountSolidClientRoute({ root, portals: portalOwner, background, component, props: {}, context, services })
    const toast = portalOwner.querySelector<HTMLElement>('[data-iota-surface="toast"]')
    expect(toast?.getAttribute('role')).toBe('status')
    expect(toast?.hasAttribute('aria-modal')).toBe(false)
    expect(background.inert).toBe(false)
    expect(document.documentElement.style.overflow).toBe('')
    expect(document.activeElement).toBe(trigger)
    dispose()
  })

  it('keeps the newest portal theme when shared routes dispose out of order', () => {
    const portalOwner = document.createElement('div')
    portalOwner.dataset.theme = 'shell'
    const firstRoot = document.createElement('div')
    const secondRoot = document.createElement('div')
    document.body.append(firstRoot, secondRoot, portalOwner)
    const services = () => ({
      session: { snapshot: () => ({}), refresh: async () => ({}) },
      navigation: { guard: () => () => undefined, canNavigate: () => true },
      theme: { current: () => 'light' as const, set: () => undefined, subscribe: () => () => undefined },
      locale: { language: 'en', t: (key: string) => key },
      telemetry: { emit: () => undefined },
      dispose: vi.fn(),
    })
    const context = (theme: 'light' | 'dark') => ({
      bootstrapVersion: CLIENT_BOOTSTRAP_VERSION,
      protocolVersion: SDK_IDENTITY.protocolVersion,
      sdkReleaseVersion: SDK_IDENTITY.releaseVersion,
      sdkCommit: SDK_IDENTITY.sourceCommit,
      initial: {},
      theme,
    })
    const component: Component = () => document.createElement('span')
    const disposeFirst = mountSolidClientRoute({ root: firstRoot, portals: portalOwner, component, props: {}, context: context('light'), services: services() })
    const disposeSecond = mountSolidClientRoute({ root: secondRoot, portals: portalOwner, component, props: {}, context: context('dark'), services: services() })
    expect(portalOwner.dataset.theme).toBe('dark')
    disposeFirst()
    expect(portalOwner.dataset.theme).toBe('dark')
    disposeSecond()
    expect(portalOwner.dataset.theme).toBe('shell')
  })
})

describe('generated Solid feature catalog host', () => {
  function routeDocument(featureId = 'feature'): Document {
    document.body.innerHTML = `
      <main id="iota-client-route" data-client-route-background>
        <div id="iota-client-route-mount"></div>
      </main>
      <div id="iota-client-route-portals"></div>
      <script id="iota-client-context" type="application/json"></script>`
    document.getElementById('iota-client-context')!.textContent = JSON.stringify({
      bootstrapVersion: CLIENT_BOOTSTRAP_VERSION,
      protocolVersion: SDK_IDENTITY.protocolVersion,
      sdkReleaseVersion: SDK_IDENTITY.releaseVersion,
      sdkCommit: SDK_IDENTITY.sourceCommit,
      initial: { name: 'Granite' },
      theme: 'light',
      route: { id: 'route', path: '/route', featureId },
      session: {}, locale: { language: 'en', messages: {} }, permissions: [], services: {},
    })
    return document
  }

  it('shows loading, mounts the generated feature and owns pagehide cleanup', async () => {
    const owner = routeDocument()
    let resolveModule!: (module: { default: Component<{ route: { initial: { name: string } } }> }) => void
    const pending = new Promise<{ default: Component<{ route: { initial: { name: string } } }> }>((resolve) => { resolveModule = resolve })
    const dispose = mountSolidFeatureCatalogFromDocument({ catalog: { feature: () => pending }, owner })
    expect(owner.querySelector('[data-solid-route-loading]')?.getAttribute('role')).toBe('status')
    resolveModule({ default: (props) => props.route.initial.name })
    await pending
    await Promise.resolve()
    expect(owner.getElementById('iota-client-route-mount')?.textContent).toBe('Granite')
    owner.defaultView?.dispatchEvent(new Event('pagehide'))
    expect(owner.getElementById('iota-client-route-mount')?.childElementCount).toBe(0)
    dispose()
  })

  it('honors a data-theme value written by the server shell', async () => {
    const owner = routeDocument()
    owner.documentElement.dataset.theme = 'dark'
    const dispose = mountSolidFeatureCatalogFromDocument({
      catalog: { feature: async () => ({ default: (props) => props.route.theme }) },
      owner,
    })
    await Promise.resolve()
    await Promise.resolve()
    expect(owner.getElementById('iota-client-route-mount')?.textContent).toBe('dark')
    dispose()
    delete owner.documentElement.dataset.theme
  })

  it('ignores a module that resolves after route disposal', async () => {
    const owner = routeDocument()
    let resolveModule!: (module: { default: Component }) => void
    const pending = new Promise<{ default: Component }>((resolve) => { resolveModule = resolve })
    const dispose = mountSolidFeatureCatalogFromDocument({ catalog: { feature: () => pending }, owner })
    dispose()
    resolveModule({ default: () => 'late' })
    await pending
    await Promise.resolve()
    expect(owner.getElementById('iota-client-route-mount')?.textContent).toBe('')
  })

  it('renders a bounded error and reloads with no private diagnostics', async () => {
    const owner = routeDocument()
    const load = vi.fn().mockRejectedValueOnce(new Error('private build path'))
    const reload = vi.fn()
    const consoleError = vi.spyOn(owner.defaultView!.console, 'error').mockImplementation(() => undefined)
    const dispose = mountSolidFeatureCatalogFromDocument({ catalog: { feature: load }, owner, reload })
    await Promise.resolve()
    await Promise.resolve()
    const alert = owner.querySelector('[data-solid-route-error]')
    expect(alert?.textContent).toContain('could not be loaded')
    expect(alert?.textContent).not.toContain('private build path')
    ;(owner.querySelector('[data-solid-route-retry]') as HTMLButtonElement).click()
    expect(load).toHaveBeenCalledOnce()
    expect(reload).toHaveBeenCalledOnce()
    consoleError.mockRestore()
    dispose()
  })

  it('rejects catalog drift and duplicate ownership before loading', () => {
    const owner = routeDocument('missing')
    expect(() => mountSolidFeatureCatalogFromDocument({ catalog: {}, owner })).toThrow('absent from the generated catalog')
    owner.getElementById('iota-client-route-mount')!.dataset.iotaClientOwner = 'react'
    expect(() => mountSolidFeatureCatalogFromDocument({ catalog: { missing: async () => ({ default: () => null }) }, owner })).toThrow('already owned')
  })
})
