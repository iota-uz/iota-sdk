import { createComponent } from 'solid-js'
import { render } from 'solid-js/web'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mountSolidClientRoute, readClientBootstrap, type ClientRouteContext } from '@iota-uz/sdk/client-host'
import { Slot as AuthorSlot } from '../client.ts'
import { slotRef } from '../contracts.ts'
import { t } from '../schema.ts'
import { deferred, pageData, pageDocument, tick } from './fixtures.ts'
import { mountRuntimePage, type RouteHost } from './page.ts'
import { createSlotSource, Slot, SlotsProvider, type SlotContribution } from './slots.ts'

const Widgets = slotRef('acme.board.widgets', t.object({ title: t.string() }))
const c = (id: string): SlotContribution => ({ id, package: 'acme', version: '1', module: `/m/${id}.js`, export: id })
const widget = (label: string) => ({ component: (p: { title: string }) => { const el = document.createElement('i'); el.dataset.w = label; el.textContent = `${label}:${p.title}`; return el } })
const shown = () => Array.from(document.querySelectorAll<HTMLElement>('[data-w]')).map((e) => e.textContent)

let dispose: (() => void) | undefined
beforeEach(() => { vi.spyOn(console, 'error').mockImplementation(() => {}); document.body.innerHTML = '<div id="root"></div>' })
afterEach(() => { dispose?.(); dispose = undefined; vi.restoreAllMocks(); document.body.innerHTML = '' })

describe('<Slot> from the runtime client', () => {
  it('is the same component the host renders', () => {
    expect(AuthorSlot).toBe(Slot)
  })

  it('renders nothing outside a host and renders contributions in order inside one', async () => {
    dispose = render(() => createComponent(Slot, { slot: Widgets, props: { title: 'x' } }), document.getElementById('root')!)
    expect(shown()).toEqual([])
    dispose()

    const source = createSlotSource({
      contributions: { 'acme.board.widgets': [c('a'), c('b')] },
      importModule: async (url) => (url.includes('a.js') ? { a: widget('A') } : { b: widget('B') }),
    })
    dispose = render(
      () => createComponent(SlotsProvider, { source, get children() { return createComponent(Slot, { slot: Widgets, props: { title: 'Main' } }) } }),
      document.getElementById('root')!,
    )
    await tick()
    expect(shown()).toEqual(['A:Main', 'B:Main'])
  })

  it('shows only what the server resolved for that slot ID', async () => {
    const source = createSlotSource({ contributions: { other: [c('a')] }, importModule: async () => ({ a: widget('A') }) })
    dispose = render(
      () => createComponent(SlotsProvider, { source, get children() { return createComponent(Slot, { slot: Widgets, props: { title: 'Main' } }) } }),
      document.getElementById('root')!,
    )
    await tick()
    expect(shown()).toEqual([])
  })

  it('drops a module that arrives after the slot was disposed', async () => {
    const late = deferred<Record<string, unknown>>()
    const created = vi.fn()
    const source = createSlotSource({ contributions: { 'acme.board.widgets': [c('a')] }, importModule: () => late.promise })
    dispose = render(
      () => createComponent(SlotsProvider, { source, get children() { return createComponent(Slot, { slot: Widgets, props: { title: 'Main' } }) } }),
      document.getElementById('root')!,
    )
    dispose()
    dispose = undefined
    late.resolve({ a: { component: () => { created(); return document.createElement('i') } } })
    await tick()
    await tick()
    expect(created).not.toHaveBeenCalled()
  })

  it('reports a module that cannot be loaded and keeps the others', async () => {
    const source = createSlotSource({
      contributions: { 'acme.board.widgets': [c('bad'), c('a')] },
      importModule: async (url) => { if (url.includes('bad')) throw new Error('404'); return { a: widget('A') } },
    })
    dispose = render(
      () => createComponent(SlotsProvider, { source, get children() { return createComponent(Slot, { slot: Widgets, props: { title: 'Main' } }) } }),
      document.getElementById('root')!,
    )
    await tick()
    expect(shown()).toEqual(['A:Main'])
    expect(console.error).toHaveBeenCalled()
  })
})

describe('a runtime page with slots', () => {
  const host: RouteHost<ClientRouteContext<unknown>> = {
    readContext: (doc) => readClientBootstrap(doc),
    mount: (options) => mountSolidClientRoute(options),
  }

  it('hands the page the contributions the server put in its data', async () => {
    pageDocument(pageData({ params: {}, paramTypes: undefined, slots: { 'acme.board.widgets': [c('a'), c('b')] } }))
    const importModule = vi.fn(async (url: string) => (url.includes('policy') ? {
      policy: { component: () => createComponent(Slot, { slot: Widgets, props: { title: 'Page' } }) },
    } : { a: widget('A'), b: widget('B') }))
    dispose = mountRuntimePage({ document, host, transport: { invoke: vi.fn() }, importModule })
    await tick()
    await tick()
    expect(shown()).toEqual(['A:Page', 'B:Page'])
  })
})
