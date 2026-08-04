import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import fixture from '../fixtures/small.json'
import { LensDashboard } from './LensDashboard'
import { parseDocument } from './contract'
import { registerLensDashboardElement } from './element'

afterEach(() => {
  document.body.replaceChildren()
  vi.unstubAllGlobals()
})

describe('LensDashboard', () => {
  it('renders the bundled document when src is omitted', () => {
    const view = render(<LensDashboard locale="en" />)

    expect(screen.getByText('$4,286,000')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Operations overview' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Headline metrics' })).toBeInTheDocument()
    expect(view.container.querySelector('[style*="--lens-panel-span: 4"]')).not.toBeNull()
  })

  it('renders optional facet filters and loads only human-readable options', async () => {
    window.history.replaceState({}, '', '/report?_f=product%3Aone&_f=product%3Atwo&_f=region%3Atashkent')
    // Keep this contract test independent from chart rendering. ECharts owns
    // asynchronous canvas work, which can outlive jsdom teardown and turn a
    // focused filter test into a source of unrelated background errors.
    const facetFixture = {
      version: '1.0.0',
      snapshotId: 'facet-test',
      meta: {
        dashboardId: 'facet-test',
        title: 'Facet test',
        generatedAt: '2026-07-31T00:00:00Z',
        locale: 'en',
      },
      layout: { rows: [{ panels: [{ panelId: 'total', span: 4 }] }] },
      panels: [{
        id: 'total',
        kind: 'stat',
        title: 'Total',
        semantics: 'series',
        frame: 'panel:total',
        encoding: { label: 'label', value: 'value' },
        format: {},
        actions: [],
        terminal: true,
      }],
      frames: {
        'panel:total': {
          columns: [{ name: 'label', type: 'string' }, { name: 'value', type: 'number' }],
          rows: [['Total', 42]],
        },
      },
      drill: { edges: {}, inlineDepth: 0 },
      perspectives: [],
      filters: [{
        id: 'facet-region',
        kind: 'facet',
        label: 'Region',
        facet: {
          dimension: 'region',
          optionsEndpoint: '/lens/facets?_facet=region&_f=product%3Aone&_f=product%3Atwo',
          searchParam: '_facet_search',
          selections: [{ label: 'Tashkent', removeUrl: '/report?_f=product%3Aone' }],
          clearUrl: '/report',
        },
      }, {
        id: 'facet-product',
        kind: 'facet',
        label: 'Product',
        facet: {
          dimension: 'product',
          optionsEndpoint: '/lens/facets?_facet=product',
          selections: [{ label: 'OSAGO', removeUrl: '/report?_f=region%3Atashkent' }],
          clearUrl: '/report',
        },
      }],
      activeFilters: [{
        dimension: 'region', value: 'tashkent', label: 'Tashkent', removeUrl: '/report?_f=product%3Aone',
      }],
      resetFiltersUrl: '/report',
      endpoints: {},
      i18n: {},
      theme: { palette: {}, series: {} },
    }
    const document = parseDocument(facetFixture)
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      applyUrl: '/report?_f=product%3Aone&_f=product%3Atwo',
      options: [
        { label: 'Tashkent', value: 'tashkent', count: 20, selected: true, toggleUrl: '/report?_f=product%3Aone' },
        { label: 'Samarkand', value: 'samarkand', count: 12, toggleUrl: '/report?_f=region%3Asamarkand' },
      ],
    }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    vi.stubGlobal('fetch', fetchMock)

    render(<LensDashboard initialDocument={document} />)
    expect(screen.getByRole('link', { name: /Remove filter: Tashkent/ })).toHaveAttribute(
      'href', '/report?_f=product%3Aone',
    )
    // "Clear all" is a command, not a destination: a button on the chip row.
    expect(screen.getAllByRole('button', { name: 'Clear all' })).toHaveLength(1)
    // Both facets live behind one trigger, which counts what is applied.
    const trigger = screen.getByRole('button', { name: /Filters/ })
    expect(trigger).toHaveTextContent('2')
    expect(fetchMock).not.toHaveBeenCalled()
    fireEvent.click(trigger)

    // The rail names every dimension; the first one's options are the pane.
    expect(screen.getByRole('tab', { name: /Region/ })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('tab', { name: /Product/ })).toBeInTheDocument()
    const samarkand = await screen.findByRole('checkbox', { name: /Samarkand/ })
    expect(screen.getByRole('checkbox', { name: /Tashkent/ })).toBeChecked()
    expect(globalThis.document.body.querySelector('.lens-facet-option-bar')).toHaveStyle({ width: '100%' })
    fireEvent.click(samarkand)
    expect(new URL(window.location.href).searchParams.getAll('_f')).toEqual([
      'product:one', 'product:two', 'region:tashkent',
    ])
    fireEvent.click(screen.getByRole('button', { name: 'Apply' }))
    expect(new URL(window.location.href).searchParams.getAll('_f')).toEqual([
      'product:one', 'product:two', 'region:tashkent', 'region:samarkand',
    ])
    expect(fetchMock.mock.calls[0]?.[0]).toContain('_f=product%3Aone&_f=product%3Atwo')
  })

  it('loads a document with same-origin credentials and csrf', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(fixture),
    })
    vi.stubGlobal('fetch', fetchMock)

    render(<LensDashboard src="/lens/example" locale="en" csrf="token" />)

    await waitFor(() => expect(screen.getByText('42')).toBeInTheDocument())
    expect(fetchMock).toHaveBeenCalledWith(
      '/lens/example',
      expect.objectContaining({
        credentials: 'same-origin',
        headers: { 'X-CSRF-Token': 'token' },
      }),
    )
  })

  it('renders initial loading and fetch error states', async () => {
    let rejectRequest: ((reason: Error) => void) | undefined
    const fetcher = vi.fn(() => new Promise<Response>((_resolve, reject) => { rejectRequest = reject }))
    const view = render(<LensDashboard src="/lens/document" fetcher={fetcher} />)

    expect(view.container.querySelector('.lens-loading')).toHaveAttribute('aria-busy', 'true')
    expect(view.container.querySelectorAll('.lens-skeleton-card').length).toBeGreaterThan(0)
    rejectRequest?.(new Error('offline'))
    expect(await screen.findByRole('alert')).toHaveTextContent('Unable to load Lens document: offline')
  })

  it('offers a retry when the document itself failed, and refetches on click', async () => {
    let rejectRequest: ((reason: Error) => void) | undefined
    const fetcher = vi.fn(() => new Promise<Response>((resolve, reject) => {
      rejectRequest = reject
      if (fetcher.mock.calls.length > 1) resolve({ ok: true, json: () => Promise.resolve(fixture) } as Response)
    }))
    render(<LensDashboard src="/lens/document" fetcher={fetcher} />)

    rejectRequest?.(new Error('offline'))
    // A failed document leaves nothing else on the page to act on.
    const retry = await screen.findByRole('button', { name: 'Retry' })
    retry.click()

    await waitFor(() => expect(fetcher.mock.calls.length).toBeGreaterThan(1))
  })

  it('explains a long wait instead of leaving the skeleton silent', () => {
    vi.useFakeTimers()
    try {
      const fetcher = vi.fn(() => new Promise<Response>(() => undefined))
      const view = render(<LensDashboard src="/lens/document" fetcher={fetcher} />)

      expect(view.container.querySelector('.lens-loading-notice')).toBeNull()
      act(() => { vi.advanceTimersByTime(9000) })
      expect(view.container.querySelector('.lens-loading-notice')).not.toBeNull()
    } finally {
      vi.useRealTimers()
    }
  })

  it('keeps the server-rendered skeleton until the document arrives', () => {
    const fetcher = vi.fn(() => new Promise<Response>(() => undefined))
    const view = render(
      <LensDashboard src="/lens/document" fetcher={fetcher} fallbackHTML='<div class="server-skeleton">shell</div>' />,
    )

    expect(view.container.querySelector('.server-skeleton')).not.toBeNull()
  })

  it('renders the no-panel fallback', () => {
    const empty = parseDocument({
      ...fixture,
      layout: { rows: [] },
      panels: [],
    })
    render(<LensDashboard initialDocument={empty} />)
    expect(screen.getByText('The document contains no panels.')).toBeInTheDocument()
  })

  it('keeps recompute visible for a headerless deferred dashboard', () => {
    const headerless = parseDocument({
      ...fixture,
      meta: { ...fixture.meta, title: '' },
      header: undefined,
      panels: [{ ...fixture.panels[0], deferred: true }],
      frames: {},
      endpoints: { panel: '/lens/panel' },
    })

    render(<LensDashboard initialDocument={headerless} />)

    // The age of the data and the remedy for it are one control: the reading is
    // the label, so the button says whether it is worth pressing.
    expect(screen.getByRole('button', { name: /Updated/ })).toBeInTheDocument()
  })

  it('names itself when there is no age to report', () => {
    // A relative time is not reproducible, so visual regression suppresses the
    // reading. The control must not vanish with it: it falls back to naming the
    // action, which is also what a document with an unreadable stamp gets.
    document.documentElement.dataset.lensVr = 'true'
    try {
      const recomputable = parseDocument({
        ...fixture,
        panels: [{ ...fixture.panels[0], deferred: true }],
        frames: {},
        endpoints: { panel: '/lens/panel' },
      })
      render(<LensDashboard initialDocument={recomputable} />)

      expect(screen.getByRole('button', { name: 'Recompute' })).toBeInTheDocument()
    } finally {
      delete document.documentElement.dataset.lensVr
    }
  })

  it('leads the action row, spins in place, and explains what it does', () => {
    const recomputable = parseDocument({
      ...fixture,
      panels: [{ ...fixture.panels[0], deferred: true }],
      frames: {},
      endpoints: { panel: '/lens/panel' },
    })
    const view = render(<LensDashboard initialDocument={recomputable} />)

    const button = screen.getByRole('button', { name: /Updated/ })
    // "Recompute" is the machine's word, so the control carries the reader's.
    expect(button).toHaveAccessibleDescription(/ignoring the cached results/)

    // Its label is a relative time, so it changes width on its own. That is
    // safe only in this position: the cluster is right-aligned, so growth in
    // its first member moves nothing after it — Export stays under the pointer.
    const actions = view.container.querySelector('.lens-dashboard-actions')!
    expect(actions.firstElementChild).toBe(button)

    fireEvent.click(button)
    // In flight it spins in place rather than swapping the glyph for a word.
    expect(button).toHaveAttribute('aria-busy', 'true')
    expect(view.container.querySelector('.lens-recompute .lens-icon-spin')).not.toBeNull()
  })

  it('wires dashboard and panel exports when the document exposes an endpoint', () => {
    const exportable = parseDocument({ ...fixture, endpoints: { export: '/lens/export' } })
    render(<LensDashboard initialDocument={exportable} />)

    // The dashboard's formats live behind one trigger; a panel still exports
    // its own single artefact in place.
    expect(screen.getByRole('button', { name: 'Export' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Export panel' })).toBeInTheDocument()
  })

  it('aborts the document fetch on unmount', () => {
    let signal: AbortSignal | undefined
    const fetcher = vi.fn<typeof fetch>().mockImplementation((_input, init) => {
      signal = init?.signal as AbortSignal
      return new Promise<Response>(() => undefined)
    })
    const view = render(<LensDashboard src="/lens/document" fetcher={fetcher} />)
    view.unmount()
    expect(signal?.aborted).toBe(true)
  })
})

describe('<lens-dashboard>', () => {
  it('decodes an embedded document as UTF-8', async () => {
    registerLensDashboardElement()
    const documentFixture = structuredClone(fixture)
    documentFixture.meta.title = 'Тренды страхового бизнеса'
    const bytes = new TextEncoder().encode(JSON.stringify(documentFixture))
    const encoded = btoa(Array.from(bytes, (byte) => String.fromCharCode(byte)).join(''))
    const element = document.createElement('lens-dashboard')
    element.setAttribute('initial-document', encoded)

    act(() => document.body.append(element))

    await waitFor(() => expect(screen.getByRole('heading', { name: 'Тренды страхового бизнеса' })).toBeInTheDocument())
  })

  it('re-renders on attribute changes and unmounts on disconnect', () => {
    registerLensDashboardElement()
    const element = document.createElement('lens-dashboard')

    act(() => document.body.append(element))
    expect(element.querySelector('[data-theme="light"]')).not.toBeNull()

    act(() => element.setAttribute('theme', 'dark'))
    expect(element.querySelector('[data-theme="dark"]')).not.toBeNull()

    act(() => element.remove())
    expect(element.childElementCount).toBe(0)
    expect(Reflect.get(element, 'root')).toBeUndefined()
  })
})
