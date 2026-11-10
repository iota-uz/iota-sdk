// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ShellHost, type ShellRenderer } from './shell-host'
import { ShellRegistry } from './shell-registry'

const theme = { current: () => 'light' as const, set: () => undefined, subscribe: () => () => undefined }

function host(): ShellHost {
  return new ShellHost(document, { registry: new ShellRegistry(), theme, user: { name: 'Ada', email: 'ada@example.com' } })
}

beforeEach(() => {
  document.body.innerHTML = '<div data-iota-shell-standard id="std"><nav>standard</nav><div data-iota-shell-outlet id="out"><p id="page">page</p></div><footer id="after"></footer></div>'
})

describe('ShellHost', () => {
  it('moves the same outlet element into the renderer and restores it', () => {
    const outlet = document.getElementById('out')!
    const renderer: ShellRenderer = { id: 'custom', mount: ({ root, outlet: moved }) => { root.append(moved) } }
    const shell = host()
    expect(shell.activate(renderer)).toBe(true)
    expect(shell.activeRenderer()).toBe('custom')
    expect(document.getElementById('std')!.hidden).toBe(true)
    expect(outlet.parentElement?.dataset.iotaShellRenderer).toBe('custom')
    shell.deactivate()
    expect(shell.activeRenderer()).toBeNull()
    expect(document.getElementById('std')!.hidden).toBe(false)
    expect(document.getElementById('out')).toBe(outlet)
    expect(outlet.parentElement?.id).toBe('std')
    expect(document.querySelector('[data-iota-shell-renderer]')).toBeNull()
  })

  it('hands the renderer user, navigation and logout, and runs its cleanup', () => {
    const cleanup = vi.fn()
    const logout = vi.fn()
    const registry = new ShellRegistry([{ key: 'a', title: 'A' }])
    const shell = new ShellHost(document, { registry, theme, user: { name: 'Ada', email: 'a@x' }, logout })
    let seen: { user: unknown; nav: number } | undefined
    shell.activate({ id: 'r', mount: (ctx) => { seen = { user: ctx.user, nav: ctx.navigation.navigation().length }; ctx.actions.logout(); return cleanup } })
    expect(seen).toEqual({ user: { name: 'Ada', email: 'a@x' }, nav: 1 })
    expect(logout).toHaveBeenCalledTimes(1)
    shell.deactivate()
    expect(cleanup).toHaveBeenCalledTimes(1)
  })

  it('lets the renderer follow notification changes', () => {
    const shell = host()
    const listener = vi.fn()
    let stop: () => void = () => undefined
    shell.activate({ id: 'n', mount: (ctx) => { stop = ctx.notifications.subscribe(listener) } })
    document.body.dispatchEvent(new CustomEvent('notificationsChanged'))
    stop()
    document.body.dispatchEvent(new CustomEvent('notificationsChanged'))
    expect(listener).toHaveBeenCalledTimes(1)
  })

  it('allows only one active renderer and reports the conflict', () => {
    const shell = host()
    expect(shell.activate({ id: 'one', mount: () => undefined })).toBe(true)
    expect(shell.activate({ id: 'two', mount: () => undefined })).toBe(false)
    expect(shell.activeRenderer()).toBe('one')
    expect(shell.diagnostics()).toMatchObject([{ code: 'renderer-conflict', renderer: 'two' }])
  })

  it('falls back to the standard shell when mount throws', () => {
    const shell = host()
    expect(shell.activate({ id: 'bad', mount: () => { throw new Error('boom') } })).toBe(false)
    expect(shell.activeRenderer()).toBeNull()
    expect(document.getElementById('std')!.hidden).toBe(false)
    expect(document.getElementById('out')!.parentElement?.id).toBe('std')
    expect(shell.diagnostics()[0]).toMatchObject({ code: 'renderer-failed', renderer: 'bad' })
  })

  it('falls back when the renderer reports an asynchronous failure', () => {
    const shell = host()
    const cleanup = vi.fn()
    let fail: (cause: unknown) => void = () => undefined
    expect(shell.activate({ id: 'late', mount: (ctx) => { fail = ctx.fail; return cleanup } })).toBe(true)
    fail(new Error('later'))
    expect(shell.activeRenderer()).toBeNull()
    expect(cleanup).toHaveBeenCalledTimes(1)
    expect(document.getElementById('std')!.hidden).toBe(false)
  })

  it('handles a failure reported synchronously during mount', () => {
    const shell = host()
    const cleanup = vi.fn()
    expect(shell.activate({ id: 'sync', mount: (ctx) => { ctx.fail('nope'); return cleanup } })).toBe(false)
    expect(cleanup).toHaveBeenCalledTimes(1)
    expect(document.getElementById('std')!.hidden).toBe(false)
  })

  it('reports a page without a standard shell', () => {
    document.body.innerHTML = ''
    const shell = host()
    expect(shell.activate({ id: 'x', mount: () => undefined })).toBe(false)
    expect(shell.diagnostics()[0]!.code).toBe('standard-shell-missing')
  })

  it('stays on the standard shell when it is forced by query', () => {
    window.history.pushState({}, '', '/?iota-shell=standard')
    const shell = host()
    expect(shell.activate({ id: 'x', mount: () => undefined })).toBe(false)
    expect(shell.diagnostics()[0]!.code).toBe('renderer-disabled')
    window.history.pushState({}, '', '/')
  })
})
