import { createComponent } from 'solid-js'
import { render } from 'solid-js/web'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { Slot as AuthorSlot } from '../client.ts'
import { slotRef } from '../contracts.ts'
import { t } from '../schema.ts'
import { ensureStyles, STYLE_ATTR } from './styles.ts'
import { deferred, pageData, pageDocument, tick } from './fixtures.ts'
import { mountSolidClientRoute, readClientBootstrap, type ClientRouteContext } from '@iota-uz/sdk/client-host'
import { mountRuntimePage, type RouteHost } from './page.ts'
import { createSlotSource, Slot, SlotsProvider, type SlotContribution } from './slots.ts'

const css = '/_ext/assets/h1h1h1h1/client/tailwind.css'
const links = (href?: string) => Array.from(document.querySelectorAll<HTMLLinkElement>('link[rel~="stylesheet"]')).filter((l) => href === undefined || l.getAttribute('href') === href)
/** jsdom loads no stylesheets: the test plays the network. */
const loaded = (link: HTMLLinkElement) => link.dispatchEvent(new Event('load'))

beforeEach(() => {
  document.head.innerHTML = ''
  document.body.innerHTML = ''
  vi.spyOn(console, 'error').mockImplementation(() => {})
})
afterEach(() => { vi.restoreAllMocks(); document.head.innerHTML = ''; document.body.innerHTML = '' })

describe('ensureStyles', () => {
  it('links a stylesheet in the head once and resolves when it loaded', async () => {
    let done = false
    const ready = ensureStyles(document, [css]).then(() => { done = true })
    await tick()
    expect(links(css)).toHaveLength(1)
    expect(links(css)[0]!.parentElement).toBe(document.head)
    expect(links(css)[0]!.hasAttribute(STYLE_ATTR)).toBe(true)
    expect(done).toBe(false)
    loaded(links(css)[0]!)
    await ready
    expect(done).toBe(true)
  })

  it('does not link again for the second island of the same version, and both wait for the one load', async () => {
    const order: string[] = []
    const first = ensureStyles(document, [css]).then(() => order.push('first'))
    const second = ensureStyles(document, [css, css]).then(() => order.push('second'))
    await tick()
    expect(links()).toHaveLength(1)
    loaded(links()[0]!)
    await Promise.all([first, second])
    expect(order.sort()).toEqual(['first', 'second'])
    await ensureStyles(document, [css]) // a later island: nothing to add
    expect(links()).toHaveLength(1)
  })

  it('reuses the link the server wrote in the page HTML, wherever it sits', async () => {
    document.body.innerHTML = `<link rel="stylesheet" href="${css}" data-iota-ext-css><main></main>`
    const ready = ensureStyles(document, [css])
    await tick()
    expect(links()).toHaveLength(1)
    expect(links()[0]!.parentElement).toBe(document.body)
    loaded(links()[0]!)
    await ready
    expect(links()).toHaveLength(1)
  })

  it('compares resolved URLs: an absolute and a root-relative href are the same stylesheet', async () => {
    document.head.innerHTML = `<link rel="stylesheet" href="${location.origin}${css}">`
    const ready = ensureStyles(document, [css])
    loaded(links()[0]!)
    await ready
    expect(links()).toHaveLength(1)
  })

  it('keeps the stylesheet when the element that asked for it leaves the document', async () => {
    document.body.innerHTML = '<section id="fragment"><div data-island></div></section>'
    const ready = ensureStyles(document, [css])
    loaded(links()[0]!)
    await ready
    document.getElementById('fragment')!.remove() // an HTMX swap
    expect(links(css)).toHaveLength(1)
    await ensureStyles(document, [css])
    expect(links(css)).toHaveLength(1) // the next island of the version finds it
  })

  it('reports a stylesheet that fails to load and carries on', async () => {
    const ready = ensureStyles(document, [css])
    await tick()
    links()[0]!.dispatchEvent(new Event('error'))
    await ready
    expect(console.error).toHaveBeenCalledWith(expect.stringContaining(css))
  })

  it('is a no-op without stylesheets or without a document', async () => {
    await ensureStyles(document, undefined)
    await ensureStyles(document, [])
    await ensureStyles(undefined, [css])
    expect(links()).toHaveLength(0)
  })
})

describe('slot contributions wait for their version stylesheets', () => {
  const Widgets = slotRef('acme.board.widgets', t.object({ title: t.string() }))
  const contribution: SlotContribution = { id: 'w', package: 'acme', version: '1', module: '/m/w.js', export: 'w', styles: [css] }
  const widget = { component: (p: { title: string }) => { const el = document.createElement('i'); el.dataset.w = 'w'; el.textContent = p.title; return el } }

  it('renders the component only once the module and the stylesheet are in', async () => {
    expect(AuthorSlot).toBe(Slot)
    document.body.innerHTML = '<div id="root"></div>'
    const source = createSlotSource({ contributions: { 'acme.board.widgets': [contribution] }, importModule: async () => ({ w: widget }), document })
    const dispose = render(
      () => createComponent(SlotsProvider, { source, get children() { return createComponent(Slot, { slot: Widgets, props: { title: 'Main' } }) } }),
      document.getElementById('root')!,
    )
    await tick()
    expect(document.querySelector('[data-w]')).toBeNull() // the module is in, the stylesheet is not
    loaded(links(css)[0]!)
    await tick()
    expect(document.querySelector('[data-w]')?.textContent).toBe('Main')
    expect(links(css)).toHaveLength(1)
    dispose()
  })
})

describe('runtime page', () => {
  const host: RouteHost<ClientRouteContext<unknown>> = { readContext: (doc) => readClientBootstrap(doc), mount: (o) => mountSolidClientRoute(o) }
  const transport = { invoke: vi.fn(async () => ({})) }

  it('renders after the version stylesheets loaded, reusing the link the HTML carries', async () => {
    const data = pageData({ params: {}, paramTypes: undefined })
    data.page.styles = [css]
    pageDocument(data)
    document.body.insertAdjacentHTML('afterbegin', `<link rel="stylesheet" href="${css}" data-iota-ext-css>`)
    const module = deferred<Record<string, unknown>>()
    const dispose = mountRuntimePage({ document, host, transport, importModule: () => module.promise })
    module.resolve({ policy: { component: () => { const el = document.createElement('section'); el.id = 'screen'; return el } } })
    await tick()
    expect(document.getElementById('screen')).toBeNull() // module loaded, stylesheet not yet
    expect(document.querySelector('[data-runtime-page-loading]')).not.toBeNull()
    loaded(links(css)[0]!)
    await tick()
    expect(document.getElementById('screen')).not.toBeNull()
    expect(links(css)).toHaveLength(1)
    dispose()
  })
})
