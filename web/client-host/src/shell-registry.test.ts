// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest'
import { ShellRegistry } from './shell-registry'

const base = [
  { key: 'crm', title: 'CRM', order: 1 },
  { key: 'crm.clients', title: 'Clients', parent: 'crm' },
  { key: 'hr', title: 'HR', order: 2 },
]

describe('ShellRegistry navigation', () => {
  it('adds an item from an extension and removes it on dispose', () => {
    const registry = new ShellRegistry(base)
    const dispose = registry.addNavigation('ext', [{ key: 'ext.report', title: 'Report', parent: 'crm', href: '/report' }])
    expect(registry.navigation()[0]!.children.map((item) => item.key)).toEqual(['crm.clients', 'ext.report'])
    dispose()
    expect(registry.navigation()[0]!.children.map((item) => item.key)).toEqual(['crm.clients'])
  })

  it('renames, moves and hides standard items without touching DOM', () => {
    const registry = new ShellRegistry(base)
    registry.patchNavigation('ext', 'crm.clients', { title: 'Customers', parent: 'hr' })
    registry.patchNavigation('ext', 'crm', { hidden: true })
    const tree = registry.navigation()
    expect(tree.map((item) => item.key)).toEqual(['hr'])
    expect(tree[0]!.children[0]).toMatchObject({ key: 'crm.clients', title: 'Customers' })
  })

  it('hides descendants of a hidden item', () => {
    const registry = new ShellRegistry(base)
    registry.patchNavigation('ext', 'crm', { hidden: true })
    expect(JSON.stringify(registry.navigation())).not.toContain('crm.clients')
  })

  it('reports duplicate keys, unknown targets and conflicting patches', () => {
    const registry = new ShellRegistry(base)
    registry.addNavigation('a', [{ key: 'hr', title: 'Dup' }])
    registry.patchNavigation('a', 'missing', { hidden: true })
    registry.patchNavigation('a', 'hr', { title: 'People' })
    registry.patchNavigation('b', 'hr', { title: 'Staff' })
    expect(registry.navigation().find((item) => item.key === 'hr')!.title).toBe('People')
    expect(registry.diagnostics().map((item) => item.code).sort()).toEqual([
      'duplicate-navigation-key',
      'navigation-patch-conflict',
      'unknown-navigation-target',
    ])
  })

  it('breaks parent cycles and unknown parents by showing the item at the root', () => {
    const registry = new ShellRegistry(base)
    registry.patchNavigation('ext', 'crm', { parent: 'crm.clients' })
    registry.patchNavigation('ext', 'hr', { parent: 'nowhere' })
    const codes = registry.diagnostics().map((item) => item.code)
    expect(codes).toContain('navigation-cycle')
    expect(codes).toContain('unknown-navigation-parent')
    expect(registry.navigation().map((item) => item.key).sort()).toEqual(['crm', 'hr'])
  })

  it('disposeOwner removes every contribution of that owner', () => {
    const registry = new ShellRegistry(base)
    registry.addNavigation('ext', [{ key: 'x', title: 'X' }])
    registry.patchNavigation('ext', 'hr', { hidden: true })
    registry.contribute('ext', { key: 'c', region: 'header', mount: () => undefined })
    registry.disposeOwner('ext')
    expect(registry.navigation().map((item) => item.key)).toEqual(['crm', 'hr'])
    expect(registry.contributions('header')).toEqual([])
  })

  it('notifies subscribers on change', () => {
    const registry = new ShellRegistry()
    const listener = vi.fn()
    registry.subscribe(listener)
    registry.addNavigation('ext', [{ key: 'x', title: 'X' }])
    expect(listener).toHaveBeenCalledTimes(1)
  })
})

describe('ShellRegistry regions', () => {
  it('mounts ordered contributions, follows changes and cleans up', () => {
    const registry = new ShellRegistry()
    const cleanup = vi.fn()
    const region = document.createElement('div')
    registry.contribute('ext', { key: 'b', region: 'footer', order: 2, mount: (c) => { c.textContent = 'b'; return cleanup } })
    registry.contribute('ext', { key: 'a', region: 'footer', order: 1, mount: (c) => { c.textContent = 'a' } })
    registry.contribute('ext', { key: 'h', region: 'header', mount: (c) => { c.textContent = 'h' } })
    const stop = registry.mountRegion('footer', region)
    expect(region.textContent).toBe('ab')
    registry.disposeOwner('ext')
    expect(region.textContent).toBe('')
    expect(cleanup).toHaveBeenCalledTimes(1)
    stop()
  })

  it('isolates a throwing contribution and ignores duplicate keys', () => {
    const registry = new ShellRegistry()
    const region = document.createElement('div')
    const error = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    registry.contribute('bad', { key: 'bad', region: 'header', mount: () => { throw new Error('boom') } })
    registry.contribute('ok', { key: 'ok', region: 'header', mount: (c) => { c.textContent = 'ok' } })
    registry.contribute('dup', { key: 'ok', region: 'header', mount: (c) => { c.textContent = 'dup' } })
    const stop = registry.mountRegion('header', region)
    expect(region.textContent).toBe('ok')
    expect(registry.diagnostics().map((item) => item.code)).toEqual(['duplicate-contribution-key'])
    expect(error).toHaveBeenCalled()
    stop()
    expect(region.textContent).toBe('')
    error.mockRestore()
  })
})
