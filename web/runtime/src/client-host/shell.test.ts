import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { documentShellRegistry, ShellHost, type ThemeService } from '@iota-uz/sdk/client-host'
import type { Component } from 'solid-js'
import type { ShellProps } from '../client.ts'
import { bootRuntime } from './entry.ts'
import { SHARED_MODULES } from './shared.ts'
import * as hooks from './hooks.ts'
import { legacyDocument, pageData, pageDocument, runtimeContext, tick } from './fixtures.ts'
import { buildNavModel, activateRuntimeShell, type ShellNode } from './shell.ts'
import { RUNTIME_CONTEXT_ELEMENT_ID, readRuntimeContext } from './protocol.ts'

const NAV = [
  { key: 'crm', title: 'CRM', order: 0 },
  { key: 'crm.client', title: 'Clients', href: '/crm/clients', parent: 'crm' },
  { key: 'core.dashboard', title: 'Dashboard', href: '/', workspace: 'ops' },
  { key: 'acme.insurance', title: 'Insurance', workspace: 'sales' },
  { key: 'acme.insurance.policies', title: 'Policies', href: '/insurance/policies', parent: 'acme.insurance', workspace: 'sales' },
]

function theme(): ThemeService & { set: ReturnType<typeof vi.fn> } {
  let current: 'light' | 'dark' = 'light'
  const listeners = new Set<(t: 'light' | 'dark') => void>()
  return {
    current: () => current,
    set: vi.fn((next: 'light' | 'dark') => { current = next; listeners.forEach((l) => l(next)) }),
    subscribe: (l) => { listeners.add(l); return () => listeners.delete(l) },
  }
}

function newHost(logout = vi.fn()) {
  const registry = documentShellRegistry(document)
  const themeService = theme()
  return { host: new ShellHost(document, { registry, theme: themeService, user: { name: 'Ann Doe', email: 'ann@example.com' }, logout }), registry, themeService, logout }
}

/** A package shell: its layout wraps the host-owned outlet. */
const shellModule = (seen: { props?: ShellProps } = {}): Record<string, unknown> => ({
  shell: {
    component: ((props: ShellProps) => {
      seen.props = props
      const frame = document.createElement('div')
      frame.id = 'package-shell'
      const nav = document.createElement('nav')
      nav.textContent = props.nav.groups.map((g) => g.label || '·').join('|')
      frame.append(nav, props.outlet)
      return frame
    }) as Component<ShellProps>,
  },
})

beforeEach(() => { vi.spyOn(console, 'error').mockImplementation(() => {}); vi.spyOn(console, 'warn').mockImplementation(() => {}) })
afterEach(() => { vi.restoreAllMocks(); document.body.innerHTML = '' })

describe('buildNavModel', () => {
  const tree: ShellNode[] = [
    { key: 'crm', title: 'CRM', children: [{ key: 'crm.client', title: 'Clients', href: '/crm/clients', children: [] }] },
    { key: 'core.dashboard', title: 'Dashboard', href: '/', workspace: 'ops', children: [] },
    { key: 'acme', title: 'Insurance', workspace: 'sales', children: [{ key: 'acme.p', title: 'Policies', href: '/insurance/policies', children: [] }] },
  ]
  const ctx = runtimeContext()

  it('groups roots with children, keeps leaves apart and filters by the active workspace', () => {
    const ops = buildNavModel(tree, ctx, 'ops', { pathname: '/crm/clients' } as Location)
    expect(ops.workspaces.map((w) => [w.id, w.active])).toEqual([['ops', true], ['sales', false]])
    expect(ops.groups.map((g) => g.id)).toEqual(['', 'crm']) // dashboard leaf, then CRM (no workspace = the first)
    expect(ops.groups[1]?.items[0]).toMatchObject({ id: 'crm.client', label: 'Clients', href: '/crm/clients', active: true })
    const sales = buildNavModel(tree, ctx, 'sales', { pathname: '/insurance/policies/9' } as Location)
    expect(sales.groups.map((g) => g.id)).toEqual(['acme'])
    expect(sales.groups[0]?.items[0]?.active).toBe(true)
    expect(sales.palette).toEqual([{ id: 'p1', label: 'Go to policies', href: '/insurance/policies', keywords: ['insurance'] }])
  })

  it('shows everything when no workspace is declared', () => {
    const model = buildNavModel(tree, runtimeContext({ navigation: { workspaces: [], palette: [] } }), '', undefined)
    expect(model.groups.flatMap((g) => g.items).length).toBe(3)
    expect(model.workspaces).toEqual([])
  })
})

describe('runtime context', () => {
  it('reads the published element and refuses an incompatible version', () => {
    legacyDocument(runtimeContext())
    expect(readRuntimeContext(document)?.shell?.id).toBe('acme.shell.main')
    expect(document.getElementById(RUNTIME_CONTEXT_ELEMENT_ID)).not.toBeNull()
    legacyDocument(runtimeContext({ version: '2.0.0' }))
    expect(() => readRuntimeContext(document)).toThrow(/incompatible/)
    legacyDocument(null)
    expect(readRuntimeContext(document)).toBeNull()
  })
})

describe('whole-shell activation around a legacy page', () => {
  it('moves the page outlet into the package shell and gives it nav, user, locale and theme', async () => {
    legacyDocument(runtimeContext(), NAV)
    const outlet = document.getElementById('outlet')!
    const legacyBody = document.getElementById('legacy-body')!
    const { host, logout, themeService } = newHost()
    const seen: { props?: ShellProps } = {}

    await expect(activateRuntimeShell({ context: readRuntimeContext(document)!, host, window, importModule: async () => shellModule(seen) })).resolves.toBe(true)

    const frame = document.getElementById('package-shell')!
    expect(frame.contains(outlet)).toBe(true) // the same element, so HTMX, scripts and modals keep working
    expect(document.getElementById('outlet')).toBe(outlet)
    expect(legacyBody.isConnected && outlet.contains(legacyBody)).toBe(true)
    expect(document.getElementById('standard')!.hidden).toBe(true)
    expect(host.activeRenderer()).toBe('acme.shell.main')

    const props = seen.props!
    expect(props.locale).toBe('en') // the document language; Go's locale is for formatting the extras
    expect(props.user).toMatchObject({ id: '5', name: 'Ann Doe', email: 'ann@example.com', permissions: ['Policy.Read'] })
    expect(props.nav.workspaces.map((w) => w.id)).toEqual(['ops', 'sales'])
    expect(props.nav.groups.length).toBeGreaterThan(0)
    expect(props.nav.palette).toHaveLength(1)
    props.theme.set('dark')
    expect(themeService.set).toHaveBeenCalledWith('dark')
    expect(props.theme.resolved).toBe('dark')
    await props.logout()
    expect(logout).toHaveBeenCalled()

    // navigation contributed later reaches the shell reactively
    documentShellRegistry(document).addNavigation('test', [{ key: 'late', title: 'Late', href: '/late' }])
    expect(props.nav.groups.flatMap((g) => g.items).some((i) => i.id === 'late')).toBe(true)

    // admin revert: the standard shell and the very same outlet come back
    host.deactivate()
    expect(document.getElementById('package-shell')).toBeNull()
    expect(document.getElementById('standard')!.hidden).toBe(false)
    expect(document.getElementById('content-parent')!.contains(outlet)).toBe(true)
    expect(outlet.contains(legacyBody)).toBe(true)
  })

  it('keeps the legacy theme controller in step: the <html> class and the stored choice follow the shell', async () => {
    legacyDocument(runtimeContext(), NAV)
    const change = vi.fn((choice: string) => {
      document.documentElement.classList.remove('light', 'dark', 'system')
      document.documentElement.classList.add(choice)
    })
    ;(window as unknown as { iotaTheme?: { change(choice: string): void } }).iotaTheme = { change }
    try {
      const { host } = newHost()
      const seen: { props?: ShellProps } = {}
      await activateRuntimeShell({ context: readRuntimeContext(document)!, host, window, importModule: async () => shellModule(seen) })
      seen.props!.theme.set('dark')
      expect(change).toHaveBeenLastCalledWith('dark')
      expect(document.documentElement.classList.contains('dark')).toBe(true)
      seen.props!.theme.set('system') // the choice is passed through; the resolved theme is the media query's
      expect(change).toHaveBeenLastCalledWith('system')
    } finally {
      delete (window as unknown as { iotaTheme?: unknown }).iotaTheme
      document.documentElement.classList.remove('light', 'dark', 'system')
    }
  })

  it('activates the renderer only once the stylesheets of its version loaded', async () => {
    const sheet = '/_ext/assets/5he115he/client/tailwind.css'
    const runtime = runtimeContext()
    runtime.shell!.styles = [sheet]
    legacyDocument(runtime, NAV)
    const { host } = newHost()
    const seen: { props?: ShellProps } = {}
    let active = false
    const activation = activateRuntimeShell({ context: readRuntimeContext(document)!, host, window, importModule: async () => shellModule(seen) }).then((ok) => { active = ok })
    await tick()
    const link = document.querySelector<HTMLLinkElement>(`link[rel~="stylesheet"][href="${sheet}"]`)!
    expect(link).not.toBeNull()
    expect(active).toBe(false) // the module is in; the standard shell stays until the stylesheet is too
    expect(host.activeRenderer()).toBeFalsy()
    link.dispatchEvent(new Event('load'))
    await activation
    expect(active).toBe(true)
    expect(document.querySelectorAll(`link[href="${sheet}"]`)).toHaveLength(1)
    link.remove()
  })

  it('without a legacy theme controller the <html> class is the resolved theme', async () => {
    legacyDocument(runtimeContext(), NAV)
    document.documentElement.classList.add('light')
    const { host } = newHost()
    const seen: { props?: ShellProps } = {}
    await activateRuntimeShell({ context: readRuntimeContext(document)!, host, window, importModule: async () => shellModule(seen) })
    seen.props!.theme.set('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(document.documentElement.classList.contains('light')).toBe(false)
    document.documentElement.classList.remove('dark')
  })

  it('keeps the standard shell when the module fails to load, is invalid, or throws while mounting', async () => {
    legacyDocument(runtimeContext(), NAV)
    const ctx = readRuntimeContext(document)!
    const standard = document.getElementById('standard')!
    const { host } = newHost()

    await expect(activateRuntimeShell({ context: ctx, host, window, importModule: async () => { throw new TypeError('Failed to fetch') } })).resolves.toBe(false)
    expect(standard.hidden).toBe(false)
    await expect(activateRuntimeShell({ context: ctx, host, window, importModule: async () => ({ shell: {} }) })).resolves.toBe(false)
    expect(standard.hidden).toBe(false)

    const broken = { shell: { component: () => { throw new Error('boom') } } }
    await expect(activateRuntimeShell({ context: ctx, host, window, importModule: async () => broken })).resolves.toBe(false)
    expect(standard.hidden).toBe(false)
    expect(document.getElementById('content-parent')!.contains(document.getElementById('outlet'))).toBe(true)
    expect(host.diagnostics().some((d) => d.code === 'renderer-failed')).toBe(true)
  })

  it('honors the ?iota-shell=standard rescue and does nothing without a binding', async () => {
    legacyDocument(runtimeContext(), NAV)
    const ctx = readRuntimeContext(document)!
    const { host } = newHost()
    window.history.replaceState({}, '', '/?iota-shell=standard')
    try {
      await expect(activateRuntimeShell({ context: ctx, host, window, importModule: async () => shellModule() })).resolves.toBe(false)
      expect(document.getElementById('standard')!.hidden).toBe(false)
      expect(host.diagnostics()[0]?.code).toBe('renderer-disabled')
    } finally {
      window.history.replaceState({}, '', '/')
    }
    const importModule = vi.fn()
    await expect(activateRuntimeShell({ context: { ...ctx, shell: undefined }, host, window, importModule })).resolves.toBe(false)
    expect(importModule).not.toHaveBeenCalled()
  })
})

describe('bootRuntime', () => {
  (globalThis as { __IOTA_RUNTIME_NO_AUTOBOOT__?: boolean }).__IOTA_RUNTIME_NO_AUTOBOOT__ = true

  it('activates the shell around a Solid runtime page in the same outlet', async () => {
    pageDocument(pageData({ params: {}, paramTypes: undefined }))
    const runtime = document.createElement('script')
    runtime.id = RUNTIME_CONTEXT_ELEMENT_ID
    runtime.type = 'application/json'
    runtime.textContent = JSON.stringify(runtimeContext())
    document.body.append(runtime)
    const standard = document.querySelector<HTMLElement>('[data-iota-shell-standard]')!
    const outlet = document.querySelector<HTMLElement>('[data-iota-shell-outlet]')!
    const modules: Record<string, Record<string, unknown>> = {
      '/_ext/assets/h1h1h1h1/client/policy.js': { policy: { component: () => { const el = document.createElement('article'); el.id = 'solid-page'; return el } } },
      '/_ext/assets/5he115he/client/shell.js': shellModule(),
    }
    const boot = bootRuntime(window, { importModule: async (url) => modules[url] ?? Promise.reject(new Error(url)) })
    await expect(boot.shell).resolves.toBe(true)
    await tick()
    const frame = document.getElementById('package-shell')!
    expect(frame.contains(outlet)).toBe(true)
    expect(outlet.contains(document.getElementById('solid-page'))).toBe(true) // legacy and Solid pages share one outlet
    expect(standard.hidden).toBe(true)
    boot.dispose()
    expect(standard.hidden).toBe(false)
    expect(document.getElementById('solid-page')).toBeNull()
  })

  it('publishes the host instances that package bundles are externalized against', () => {
    legacyDocument(null)
    const boot = bootRuntime(window)
    const shared = (window as unknown as Record<symbol, Record<string, Record<string, unknown>>>)[SHARED_MODULES]!
    expect(Object.keys(shared)).toEqual(['solid-js', 'solid-js/web', 'solid-js/store', '@iota-uz/sdk/runtime/client'])
    expect(shared['@iota-uz/sdk/runtime/client']!.useQuery).toBe(hooks.useQuery) // the very instance the pages are rendered with
    expect(typeof shared['@iota-uz/sdk/runtime/client']!.page).toBe('function')
    expect(typeof shared['solid-js']!.createSignal).toBe('function')
    boot.dispose()
  })

  it('is a no-op on a page without runtime context', async () => {
    legacyDocument(null)
    const boot = bootRuntime(window, { importModule: async () => { throw new Error('unexpected') } })
    await expect(boot.shell).resolves.toBe(false)
    expect(document.getElementById('standard')!.hidden).toBe(false)
    boot.dispose()
  })
})
