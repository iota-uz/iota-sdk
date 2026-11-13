import { describe, expect, it, vi } from 'vitest'
import { KIT_STYLE_ATTR } from './kit.ts'
import { publishSharedModules, SHARED_LOADERS, SHARED_MODULES } from './shared.ts'

function windowLike() {
  const dom = document.implementation.createHTMLDocument('')
  return Object.assign(Object.create(null) as object, { document: dom }) as { document: Document }
}

describe('the UI kit as a shared module', () => {
  it('is not part of the eager registry: it is published as a lazy loader', () => {
    const target = windowLike()
    const modules = publishSharedModules(target) as Record<string, unknown>
    expect(Object.keys(modules).sort()).toEqual(['@iota-uz/sdk/runtime/client', 'solid-js', 'solid-js/store', 'solid-js/web'])
    const loaders = (target as unknown as Record<symbol, Record<string, () => Promise<object>>>)[SHARED_LOADERS]!
    expect(Object.keys(loaders)).toEqual(['@iota-uz/sdk/solid-ui'])
    expect((target as unknown as Record<symbol, unknown>)[SHARED_MODULES]).toBe(modules)
    // Nothing of the kit is in the document until a package asks for it.
    expect(document.querySelector(`style[${KIT_STYLE_ATTR}]`)).toBeNull()
  })

  it('loads once, installs its stylesheet once, and hands every package the same module', async () => {
    const target = windowLike()
    publishSharedModules(target)
    const load = (target as unknown as Record<symbol, Record<string, () => Promise<object>>>)[SHARED_LOADERS]!['@iota-uz/sdk/solid-ui']!
    const [a, b] = await Promise.all([load(), load()])
    expect(a).toBe(b)
    expect(Object.keys(a)).toEqual(['Button', 'Tabs'])
    expect(await load()).toBe(a)
    const sheets = target.document as Document & { adoptedStyleSheets?: CSSStyleSheet[] }
    const installed = target.document.querySelectorAll(`style[${KIT_STYLE_ATTR}]`).length + (sheets.adoptedStyleSheets?.length ?? 0)
    expect(installed).toBe(1)
  })

  it('does not install the stylesheet a second time in a document that has it', async () => {
    const { installKitStyles } = await import('./kit.ts')
    const doc = document.implementation.createHTMLDocument('')
    // jsdom has no constructable stylesheets: the style element path.
    installKitStyles(doc)
    installKitStyles(doc)
    const sheets = doc as Document & { adoptedStyleSheets?: CSSStyleSheet[] }
    expect(doc.querySelectorAll(`style[${KIT_STYLE_ATTR}]`).length + (sheets.adoptedStyleSheets?.length ?? 0)).toBe(1)
    expect(doc.querySelector(`style[${KIT_STYLE_ATTR}]`)?.textContent ?? '').toContain('cursor-pointer')
  })

  it('does not keep a failed load', async () => {
    vi.resetModules()
    vi.doMock('./kit.js', () => ({ get installKitStyles(): never { throw new Error('chunk 404') }, kit: {} }))
    const { publishSharedModules: publish, SHARED_LOADERS: loaders } = await import('./shared.ts')
    const target = windowLike()
    publish(target)
    const load = (target as unknown as Record<symbol, Record<string, () => Promise<object>>>)[loaders]!['@iota-uz/sdk/solid-ui']!
    await expect(load()).rejects.toThrow('chunk 404')
    vi.doUnmock('./kit.js')
    vi.doMock('./kit.js', () => ({ installKitStyles: () => {}, kit: { Button: 1 } }))
    await expect(load()).resolves.toEqual({ Button: 1 })
    vi.doUnmock('./kit.js')
  })
})
