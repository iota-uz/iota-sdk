import { createEffect, onCleanup } from 'solid-js'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { query } from '../contracts.ts'
import { t } from '../schema.ts'
import { deferred, tick } from './fixtures.ts'
import { useQuery } from './hooks.ts'
import { CELL_ATTR, CELL_CONFIG_ATTR, OUTPUT_ATTR, SLOT_ATTR, SLOT_CONFIG_ATTR, startIslandHost, type IslandEnv } from './islands.ts'
import type { SlotContribution } from './slots.ts'

const env: IslandEnv = { rpc: '/_ext/rpc', locale: 'en', protocolVersion: '1.0.0', sdkReleaseVersion: '0.5.6', sdkCommit: 'abc' }
const transport = { invoke: vi.fn(async () => ({ n: 4 })) }

const contribution = (id: string, extra: Partial<SlotContribution> = {}): SlotContribution => ({
  id, package: 'acme.pkg', version: '1.0.0', module: `/_ext/assets/h1h1/client/${id}.js`, export: id, ...extra,
})

/** What Go renders (see pkg/extruntime/ui/islands.go), HTML-escaped the way templ writes attributes. */
function slotHtml(slot: string, props: object, contributions: SlotContribution[]): string {
  const config = JSON.stringify({ props, contributions, env })
  const el = document.createElement('div')
  el.setAttribute(SLOT_ATTR, slot)
  el.setAttribute(SLOT_CONFIG_ATTR, config)
  el.setAttribute('hx-disable', '')
  return el.outerHTML
}

function cellHtml(c: SlotContribution, value: unknown, row: object, fallback: string): string {
  const el = document.createElement('span')
  el.setAttribute(CELL_ATTR, c.id)
  el.setAttribute(CELL_CONFIG_ATTR, JSON.stringify({ contribution: c, value, row, env }))
  el.setAttribute('hx-disable', '')
  el.innerHTML = fallback
  return el.outerHTML
}

/** A slot() value: a component that renders a labelled element. */
const piece = (label: string, cleanup?: () => void) => ({
  component: (props: { clientId: number }) => {
    const el = document.createElement('div')
    el.dataset.piece = label
    el.textContent = `${label}:${props.clientId}`
    if (cleanup) onCleanup(cleanup)
    return el
  },
})

let stop: (() => void) | undefined
const start = (modules: Record<string, unknown>, prepare = () => transport as never) => {
  const importModule = vi.fn(async (url: string) => {
    const mod = modules[url]
    if (mod instanceof Promise) return mod as Promise<Record<string, unknown>>
    if (mod === undefined) throw new Error(`404 ${url}`)
    return mod as Record<string, unknown>
  })
  stop = startIslandHost({ window: window as never, prepare, importModule })
  return importModule
}
const url = (id: string) => `/_ext/assets/h1h1/client/${id}.js`
const pieces = () => Array.from(document.querySelectorAll<HTMLElement>('[data-piece]')).map((e) => e.textContent)

beforeEach(() => {
  document.body.innerHTML = ''
  vi.spyOn(console, 'error').mockImplementation(() => {})
})
afterEach(() => { stop?.(); stop = undefined; vi.restoreAllMocks(); document.body.innerHTML = '' })

describe('slot islands', () => {
  it('mounts every contribution in the server order with the host page props, in its own output', async () => {
    document.body.innerHTML = `<main>${slotHtml('crm.slots.clientSidebar', { clientId: 7 }, [contribution('b'), contribution('a')])}</main>`
    start({ [url('a')]: { a: piece('A') }, [url('b')]: { b: piece('B') } })
    await tick()
    expect(pieces()).toEqual(['B:7', 'A:7']) // the order is the server's, not load order
    const host = document.querySelector(`[${SLOT_ATTR}]`)!
    expect(host.querySelector(`:scope > [${OUTPUT_ATTR}]`)).not.toBeNull()
    expect(host.hasAttribute('hx-disable')).toBe(true)
  })

  it('gives contributions the runtime transport (useQuery) of the island', async () => {
    const Count = query('acme.pkg.count', { input: t.object({}), output: t.object({ n: t.int() }) })
    document.body.innerHTML = slotHtml('s', { clientId: 1 }, [contribution('q')])
    start({
      [url('q')]: {
        q: {
          component: () => {
            const q = useQuery(Count, {})
            const el = document.createElement('p')
            el.dataset.piece = 'q'
            createEffect(() => { el.textContent = `n=${q.data()?.n ?? '…'}` })
            return el
          },
        },
      },
    })
    await tick()
    await tick()
    expect(pieces()).toEqual(['n=4'])
  })

  it('leaves its neighbours alone when one contribution is broken', async () => {
    document.body.innerHTML = slotHtml('s', { clientId: 1 }, [contribution('missing'), contribution('noexport'), contribution('boom'), contribution('ok')])
    start({
      [url('noexport')]: { other: piece('X') },
      [url('boom')]: { boom: { component: () => { throw new Error('boom') } } },
      [url('ok')]: { ok: piece('OK') },
    })
    await tick()
    expect(pieces()).toEqual(['OK:1'])
    expect(console.error).toHaveBeenCalled()
  })

  it('mounts what an HTMX swap brings in and disposes what it removes', async () => {
    document.body.innerHTML = `<div id="target">${slotHtml('s', { clientId: 1 }, [contribution('a')])}</div>`
    const cleanup = vi.fn()
    start({ [url('a')]: { a: piece('A', cleanup) }, [url('b')]: { b: piece('B') } })
    await tick()
    expect(pieces()).toEqual(['A:1'])
    expect(cleanup).not.toHaveBeenCalled()

    // htmx replaces the fragment: the old root must be released, the new element mounted.
    document.getElementById('target')!.innerHTML = slotHtml('s', { clientId: 2 }, [contribution('b')])
    await tick()
    expect(cleanup).toHaveBeenCalledTimes(1)
    expect(pieces()).toEqual(['B:2'])
  })

  it('disposes the island when its page is navigated away (the element leaves the document)', async () => {
    document.body.innerHTML = `<main>${slotHtml('s', { clientId: 1 }, [contribution('a')])}</main>`
    const cleanup = vi.fn()
    start({ [url('a')]: { a: piece('A', cleanup) } })
    await tick()
    document.body.replaceChildren(document.createElement('main')) // a boosted navigation swaps the body
    await tick()
    expect(cleanup).toHaveBeenCalledTimes(1)
    expect(pieces()).toEqual([])
  })

  it('ignores a module that arrives after its element is gone', async () => {
    document.body.innerHTML = `<section>${slotHtml('s', { clientId: 1 }, [contribution('late')])}</section>`
    const late = deferred<Record<string, unknown>>()
    const created = vi.fn()
    start({ [url('late')]: late.promise })
    await tick()
    document.body.innerHTML = '' // navigated away while the chunk was still downloading
    await tick()
    late.resolve({ late: { component: () => { created(); return document.createElement('i') } } })
    await tick()
    await tick()
    expect(created).not.toHaveBeenCalled()
    expect(document.querySelector('i')).toBeNull()
  })

  it('does not remount an element that is only moved', async () => {
    document.body.innerHTML = `<div id="a">${slotHtml('s', { clientId: 1 }, [contribution('a')])}</div><div id="b"></div>`
    const cleanup = vi.fn()
    const importModule = start({ [url('a')]: { a: piece('A', cleanup) } })
    await tick()
    document.getElementById('b')!.append(document.querySelector(`[${SLOT_ATTR}]`)!) // the shell moves the outlet
    await tick()
    expect(cleanup).not.toHaveBeenCalled()
    expect(pieces()).toEqual(['A:1'])
    expect(importModule).toHaveBeenCalledTimes(1)
  })

  it('refuses an island the host cannot serve and reports an unreadable configuration', async () => {
    document.body.innerHTML = `${slotHtml('s', { clientId: 1 }, [contribution('a')])}<div ${SLOT_ATTR}="broken" ${SLOT_CONFIG_ATTR}="{nope"></div>`
    const importModule = start({ [url('a')]: { a: piece('A') } }, () => undefined as never)
    await tick()
    expect(pieces()).toEqual([])
    expect(importModule).not.toHaveBeenCalled()
    expect(console.error).toHaveBeenCalled()
  })

  it('stop() releases every island', async () => {
    document.body.innerHTML = slotHtml('s', { clientId: 1 }, [contribution('a')])
    const cleanup = vi.fn()
    start({ [url('a')]: { a: piece('A', cleanup) } })
    await tick()
    stop?.()
    stop = undefined
    expect(cleanup).toHaveBeenCalledTimes(1)
  })
})

describe('cell islands', () => {
  const cell = contribution('tierCell')
  const render = (label: string) => ({
    component: (props: { value: unknown; row: Record<string, unknown> }) => {
      const el = document.createElement('b')
      el.dataset.cell = 'x'
      el.textContent = `${label}:${String(props.value)}:${JSON.stringify(props.row)}`
      return el
    },
  })

  it('keeps the server-rendered cell until the renderer is ready, then shows the renderer with value and row', async () => {
    document.body.innerHTML = `<table><tr><td>${cellHtml(cell, 'gold', { ref: { entity: 'crm.client', key: '5' } }, '<span id="std">gold</span>')}</td></tr></table>`
    const ready = deferred<Record<string, unknown>>()
    start({ [url('tierCell')]: ready.promise })
    await tick()
    expect(document.getElementById('std')).not.toBeNull() // the standard cell reads fine while the module loads

    ready.resolve({ tierCell: render('R') })
    await tick()
    await tick()
    expect(document.getElementById('std')).toBeNull()
    expect(document.querySelector('[data-cell]')?.textContent).toBe('R:gold:{"ref":{"entity":"crm.client","key":"5"}}')
  })

  it('keeps the standard cell when the renderer is missing, broken or throws', async () => {
    document.body.innerHTML = `<table><tr>
      <td>${cellHtml(contribution('gone'), 'a', {}, '<i class="std">a</i>')}</td>
      <td>${cellHtml(contribution('boom'), 'b', {}, '<i class="std">b</i>')}</td>
    </tr></table>`
    start({ [url('boom')]: { boom: { component: () => { throw new Error('boom') } } } })
    await tick()
    await tick()
    expect(Array.from(document.querySelectorAll('.std')).map((e) => e.textContent)).toEqual(['a', 'b'])
    expect(document.querySelector('[data-cell]')).toBeNull()
  })

  it('mounts the cells of an HTMX fragment and releases the ones it replaces', async () => {
    document.body.innerHTML = `<table><tbody id="rows"><tr><td>${cellHtml(cell, 'gold', {}, '<span class="std">gold</span>')}</td></tr></tbody></table>`
    start({ [url('tierCell')]: { tierCell: render('R') } })
    await tick()
    await tick()
    expect(document.querySelectorAll('[data-cell]')).toHaveLength(1)
    document.getElementById('rows')!.innerHTML = `<tr><td>${cellHtml(cell, 'silver', {}, '<span class="std">silver</span>')}</td></tr><tr><td>${cellHtml(cell, 'bronze', {}, '<span class="std">bronze</span>')}</td></tr>`
    await tick()
    await tick()
    expect(Array.from(document.querySelectorAll('[data-cell]')).map((e) => e.textContent?.split(':')[1])).toEqual(['silver', 'bronze'])
  })

  it('does not reuse the rendered output of a restored history snapshot', async () => {
    // htmx caches the DOM of a page, island output included; the restored element has no root behind it.
    document.body.innerHTML = `<div ${SLOT_ATTR}="s" ${SLOT_CONFIG_ATTR}='${JSON.stringify({ props: { clientId: 1 }, contributions: [contribution('a')], env })}'><div ${OUTPUT_ATTR}=""><div data-piece="stale">stale</div></div></div>`
    start({ [url('a')]: { a: piece('A') } })
    await tick()
    expect(pieces()).toEqual(['A:1'])
  })
})

describe('version stylesheets of islands', () => {
  const sheet = '/_ext/assets/h1h1/client/tailwind.css'
  const sheets = () => Array.from(document.querySelectorAll<HTMLLinkElement>('link[rel~="stylesheet"]'))
  beforeEach(() => { document.head.innerHTML = '' })
  afterEach(() => { document.head.innerHTML = '' })

  it('links the version stylesheet once for several islands and renders them after it loaded', async () => {
    document.body.innerHTML = `<main>${slotHtml('s', { clientId: 1 }, [contribution('a', { styles: [sheet] }), contribution('b', { styles: [sheet] })])}${slotHtml('s2', { clientId: 2 }, [contribution('a', { styles: [sheet] })])}</main>`
    start({ [url('a')]: { a: piece('A') }, [url('b')]: { b: piece('B') } })
    await tick()
    expect(sheets()).toHaveLength(1)
    expect(pieces()).toEqual([]) // modules are in, the stylesheet is not: no unstyled paint
    sheets()[0]!.dispatchEvent(new Event('load'))
    await tick()
    expect(pieces()).toEqual(['A:1', 'B:1', 'A:2'])
    expect(sheets()).toHaveLength(1)
  })

  it('keeps one stylesheet across a swap that removes the island and one that brings it back', async () => {
    document.body.innerHTML = `<section id="region">${slotHtml('s', { clientId: 1 }, [contribution('a', { styles: [sheet] })])}</section>`
    start({ [url('a')]: { a: piece('A') } })
    await tick()
    sheets()[0]!.dispatchEvent(new Event('load'))
    await tick()
    expect(pieces()).toEqual(['A:1'])

    const region = document.getElementById('region')!
    region.innerHTML = '<h1>heading</h1>' // an HTMX swap that drops the slot
    await tick()
    expect(pieces()).toEqual([])
    expect(sheets()).toHaveLength(1) // the link is the document's, not the island's

    region.innerHTML = slotHtml('s', { clientId: 1 }, [contribution('a', { styles: [sheet] })]) // and one that brings it back
    await tick()
    expect(pieces()).toEqual(['A:1'])
    expect(sheets()).toHaveLength(1)
  })

  it('styles list cells too', async () => {
    const c = contribution('cell', { styles: [sheet] })
    document.body.innerHTML = `<table><tr><td>${cellHtml(c, 'gold', {}, 'gold')}</td></tr></table>`
    start({ [url('cell')]: { cell: { component: (p: { value: string }) => { const el = document.createElement('b'); el.dataset.piece = 'cell'; el.textContent = p.value.toUpperCase(); return el } } } })
    await tick()
    expect(sheets()).toHaveLength(1)
    expect(pieces()).toEqual([])
    sheets()[0]!.dispatchEvent(new Event('load'))
    await tick()
    expect(pieces()).toEqual(['GOLD'])
  })
})
