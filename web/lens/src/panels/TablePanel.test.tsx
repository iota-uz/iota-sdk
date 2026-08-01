import { readFileSync } from 'node:fs'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { DashboardDocument, Panel } from '../contract'
import { DashboardRuntimeProvider, DocumentProvider } from '../runtime'
import { TablePanel } from './TablePanel'

const tablePanel: Panel = {
  id: 'evidence',
  kind: 'table',
  title: 'Evidence',
  semantics: 'evidence',
  frame: 'evidence:root',
  encoding: { id: 'record_id', label: 'name', value: 'amount' },
  format: { amount: { kind: 'money', currency: 'USD', minorUnits: false, precision: 0 } },
  drillRoot: 'evidence',
  actions: [{
    kind: 'navigate_to_leaf',
    urlTemplate: '/records/{id}',
    params: [{ name: 'id', source: { kind: 'field', name: 'record_id' } }],
    payload: {},
    preserveQuery: true,
  }],
}

const tableDocument: DashboardDocument = {
  version: '1.0.0',
  snapshotId: 'table-snapshot',
  meta: { dashboardId: 'table', title: 'Table', generatedAt: '2026-07-19T00:00:00Z', locale: 'en' },
  layout: { rows: [{ panels: [{ panelId: tablePanel.id, span: 12 }] }] },
  panels: [tablePanel],
  frames: {},
  drill: {
    inlineDepth: 0,
    edges: {
      evidence: {
        path: ['evidence'], label: 'Evidence', children: [], frame: 'evidence:root', encoding: tablePanel.encoding, perspectives: [],
      },
    },
  },
  perspectives: [],
  endpoints: { query: '/lens/query' },
  i18n: {},
  theme: { palette: {}, series: {} },
}

function pageResponse(page: number, hasNext = page === 1): Response {
  const rows = page === 1
    ? [['A 1', 'Alpha', 20, true], ['B-2', 'Beta', 10, false]]
    : page === 2 ? [['C-3', 'Gamma', 30, true]] : []
  return new Response(JSON.stringify({
    frames: {
      'evidence:root': {
        columns: [
          { name: 'record_id', type: 'string' },
          { name: 'name', type: 'string' },
          { name: 'amount', type: 'number' },
          { name: 'posted', type: 'bool' },
        ],
        rows,
      },
    },
    page: { number: page, size: 2, hasNext },
  }), { status: 200, headers: { 'Content-Type': 'application/json' } })
}

afterEach(() => {
  cleanup()
  window.history.replaceState(null, '', '/')
})

it('keeps compact horizontal overflow controls clear of table values', () => {
  const styles = readFileSync('src/styles.css', 'utf8')
  const edgeRule = styles.match(/\.lens-table-overflow-edge \{(?<rule>[^}]+)\}/)?.groups?.rule
  const markerRule = styles.match(/\.lens-table-overflow-edge::after \{(?<rule>[^}]+)\}/)?.groups?.rule

  expect(edgeRule).toContain('height: 40px')
  expect(edgeRule).toContain('width: 28px')
  expect(edgeRule).toContain('top: 50%')
  expect(markerRule).not.toContain('repeat-y')
  expect(styles).toContain("margin-right: var(--lens-table-sticky-width, 0px)")
})

const columnsPanel: Panel = {
  id: 'profitability',
  kind: 'table',
  title: 'Profitability',
  semantics: 'evidence',
  frame: 'profitability:root',
  encoding: { id: 'client_id', label: 'name' },
  format: {
    earned: { kind: 'money', currency: 'UZS', minorUnits: false, precision: 0 },
    growth: { kind: 'money', currency: 'UZS', minorUnits: false, precision: 0 },
    growth_pct: { kind: 'percent', minorUnits: false, precision: 1 },
  },
  columns: [
    {
      field: 'name',
      label: 'Client',
      cell: { kind: 'plain' },
      action: {
        kind: 'navigate_to_leaf',
        urlSource: { kind: 'field', name: 'detail_url' },
        params: [],
        payload: {},
      },
    },
    { field: 'earned', label: 'Earned premium', align: 'right', cell: { kind: 'bar' } },
    { field: 'growth', label: 'Growth', align: 'right', cell: { kind: 'delta', secondaryField: 'growth_pct' } },
  ],
  actions: [],
}

const columnsDocument: DashboardDocument = {
  version: '1.0.0',
  snapshotId: 'columns-snapshot',
  meta: { dashboardId: 'columns', title: 'Columns', generatedAt: '2026-07-19T00:00:00Z', locale: 'en' },
  layout: { rows: [{ panels: [{ panelId: columnsPanel.id, span: 12 }] }] },
  panels: [columnsPanel],
  frames: {
    'profitability:root': {
      columns: [
        { name: 'client_id', type: 'string' },
        { name: 'name', type: 'string' },
        { name: 'earned', type: 'number' },
        { name: 'growth', type: 'number' },
        { name: 'growth_pct', type: 'number' },
        { name: 'detail_url', type: 'string' },
        { name: 'secret', type: 'string' },
      ],
      rows: [
        ['1', 'Orion', 1_000_000, 200_000, 12.5, '/clients/1', 'top-secret'],
        ['2', 'Northstar', 500_000, -80_000, -4.2, '/clients/2', 'hidden-note'],
      ],
    },
  },
  drill: { inlineDepth: 0, edges: {} },
  perspectives: [],
  endpoints: {},
  i18n: {},
  theme: { palette: {}, series: {} },
}

describe('TablePanel columns', () => {
  it('labels a non-zero absolute delta with a zero comparison baseline as New', () => {
    const document = {
      ...columnsDocument,
      frames: {
        'profitability:root': {
          ...columnsDocument.frames['profitability:root']!,
          rows: [['1', 'Orion', 1_000_000, 200_000, null, '/clients/1', 'top-secret']],
        },
      },
    }
    render(
      <DocumentProvider initialDocument={document}>
        <DashboardRuntimeProvider locale="en"><TablePanel panel={columnsPanel} /></DashboardRuntimeProvider>
      </DocumentProvider>,
    )

    expect(screen.getByText('New')).toBeInTheDocument()
    expect(screen.queryByText('—', { selector: '.lens-table-delta-pct' })).toBeNull()
  })

  it('renders declared columns in order with labels, bar/delta cells, and per-column leaf links', () => {
    const { container } = render(
      <div className="lens-root">
        <DocumentProvider initialDocument={columnsDocument}>
          <DashboardRuntimeProvider locale="en">
            <TablePanel panel={columnsPanel} />
          </DashboardRuntimeProvider>
        </DocumentProvider>
      </div>,
    )

    const headers = screen.getAllByRole('columnheader').map((header) => header.textContent ?? '')
    expect(headers).toHaveLength(3)
    expect(headers[0]).toContain('Client')
    expect(headers[1]).toContain('Earned premium')
    expect(headers[2]).toContain('Growth')

    // Hidden frame fields never render.
    expect(screen.queryByText('top-secret')).toBeNull()
    expect(screen.queryByText('hidden-note')).toBeNull()

    // Bar cells grow from the track midpoint, so the max value fills one half
    // and the sign decides which half.
    const fills = container.querySelectorAll<HTMLElement>('.lens-table-bar-fill')
    expect(fills).toHaveLength(2)
    expect(fills[0]?.style.width).toBe('50%')
    expect(fills[0]?.style.left).toBe('50%')
    expect(fills[1]?.style.width).toBe('25%')

    // Delta cell colors the secondary percentage by sign.
    expect(container.querySelector('.lens-table-delta-pct-negative')).not.toBeNull()

    // Per-column action renders the cell as a same-origin leaf link.
    const links = screen.getAllByRole('link')
    expect(links[0]).toHaveAttribute('href', expect.stringContaining('/clients/1'))
    expect(links[1]).toHaveAttribute('href', expect.stringContaining('/clients/2'))

    // No panel-level "Open record" action column in columns mode.
    expect(screen.queryByText('Open record')).toBeNull()

    // Sorting is on, but every row is already on screen: there is no "this
    // page" for the caveat to be about, and printing it reads as a warning
    // that some of the data is hidden.
    expect(screen.queryByText('Sort applies to this page only')).toBeNull()
    expect(screen.queryByRole('button', { name: 'Next' })).toBeNull()
  })
})

describe('TablePanel static (non-sortable)', () => {
  it('renders plain column headings, no sort scope note, when presentation.sortable is false', () => {
    const staticPanel: Panel = {
      ...columnsPanel,
      id: 'decomposition',
      presentation: { sortable: false, expandable: false, exportable: false },
    }
    const staticDocument: DashboardDocument = {
      ...columnsDocument,
      meta: { ...columnsDocument.meta, dashboardId: 'static', title: 'Static' },
      snapshotId: 'static-snapshot',
      layout: { rows: [{ panels: [{ panelId: staticPanel.id, span: 12 }] }] },
      panels: [staticPanel],
      frames: { 'decomposition:root': columnsDocument.frames['profitability:root']! },
    }
    staticPanel.frame = 'decomposition:root'

    render(
      <div className="lens-root">
        <DocumentProvider initialDocument={staticDocument}>
          <DashboardRuntimeProvider locale="en">
            <TablePanel panel={staticPanel} />
          </DashboardRuntimeProvider>
        </DocumentProvider>
      </div>,
    )

    // Headings are plain labels, not sort buttons.
    expect(screen.getAllByRole('columnheader').length).toBeGreaterThan(0)
    expect(screen.queryByRole('button', { name: /Earned premium/ })).toBeNull()
    expect(screen.queryByText('Sort applies to this page only')).toBeNull()
    // A drawer-hosted derived table drops the expand and export chrome.
    expect(screen.queryByRole('button', { name: 'Expand panel' })).toBeNull()
    expect(screen.queryByRole('button', { name: /Export/ })).toBeNull()
  })
})

const quietPanel: Panel = {
  id: 'matrix',
  kind: 'table',
  title: 'Matrix',
  semantics: 'evidence',
  frame: 'matrix:root',
  encoding: { id: 'group', label: 'group' },
  format: { ratio: { kind: 'percent', minorUnits: false, precision: 1 } },
  columns: [
    { field: 'group', label: 'Group', badgeField: '__badge', cell: { kind: 'plain' } },
    {
      field: 'ratio',
      label: 'Loss ratio',
      align: 'right',
      affordance: 'quiet',
      cell: { kind: 'plain', toneField: '__tone' },
      action: { kind: 'open_drawer', urlSource: { kind: 'field', name: '__url' }, params: [], payload: {} },
    },
  ],
  actions: [],
}

const quietDocument: DashboardDocument = {
  ...columnsDocument,
  snapshotId: 'quiet-snapshot',
  meta: { ...columnsDocument.meta, dashboardId: 'quiet', title: 'Quiet' },
  layout: { rows: [{ panels: [{ panelId: quietPanel.id, span: 12 }] }] },
  panels: [quietPanel],
  frames: {
    'matrix:root': {
      columns: [
        { name: 'group', type: 'string' },
        { name: 'ratio', type: 'number' },
        { name: '__tone', type: 'string' },
        { name: '__badge', type: 'string' },
        { name: '__url', type: 'string' },
      ],
      rows: [
        ['Group A', 30, '', '', '/groups/A'],
        ['Unknown', 250, 'neg', 'Source rows with no matched product', '/groups/unknown'],
      ],
    },
  },
}

describe('TablePanel quiet drill cells', () => {
  it('renders a whole-cell quiet drill target with tone and a badge', () => {
    const { container } = render(
      <div className="lens-root">
        <DocumentProvider initialDocument={quietDocument}>
          <DashboardRuntimeProvider locale="en">
            <TablePanel panel={quietPanel} />
          </DashboardRuntimeProvider>
        </DocumentProvider>
      </div>,
    )

    // The quiet cell is a link (whole-cell target) carrying the fade-in arrow.
    const quiet = container.querySelectorAll<HTMLElement>('.lens-table-cell-quiet')
    expect(quiet).toHaveLength(2)
    expect(quiet[0]).toHaveAttribute('href', expect.stringContaining('/groups/A'))
    expect(container.querySelector('.lens-table-cell-quiet-arrow')).not.toBeNull()

    // The over-100% loss ratio tints negative; the default row carries no tone.
    expect(container.querySelectorAll('.lens-table-tone-neg')).toHaveLength(1)
    expect(container.querySelector('.lens-table-tone-warn')).toBeNull()

    // The unmatched row's name cell carries a "?" badge with the hint tooltip.
    const badge = container.querySelector<HTMLElement>('.lens-table-cell-badge')
    expect(badge).not.toBeNull()
    expect(badge).toHaveAttribute('title', 'Source rows with no matched product')
  })
})

const groupPanel: Panel = {
  id: 'grouped',
  kind: 'table',
  title: 'Grouped',
  semantics: 'series',
  frame: 'grouped:root',
  encoding: { id: 'product', label: 'product' },
  format: {},
  presentation: { rowGroupField: '__group' },
  columns: [
    { field: 'product', label: 'Product', cell: { kind: 'plain' } },
    { field: 'delta', label: 'vs previous', align: 'right', cell: { kind: 'plain' } },
  ],
  actions: [],
}

function groupedDocument(): DashboardDocument {
  const normal = Array.from({ length: 11 }, (_, index) => [`Live ${index + 1}`, index + 1, ''])
  return {
    ...columnsDocument,
    snapshotId: 'grouped-snapshot',
    meta: { ...columnsDocument.meta, dashboardId: 'grouped', title: 'Grouped' },
    layout: { rows: [{ panels: [{ panelId: groupPanel.id, span: 12 }] }] },
    panels: [groupPanel],
    frames: {
      'grouped:root': {
        columns: [
          { name: 'product', type: 'string' },
          { name: 'delta', type: 'number' },
          { name: '__group', type: 'string' },
        ],
        rows: [
          ...normal,
          ['Discontinued products (1)', -100, 'discontinued:toggle'],
          ['Legacy KASKO', 0, 'discontinued'],
        ],
      },
    },
  }
}

describe('TablePanel discontinued grouping', () => {
  it('hides collapsed members until the toggle is expanded and counts real rows', () => {
    render(
      <div className="lens-root">
        <DocumentProvider initialDocument={groupedDocument()}>
          <DashboardRuntimeProvider locale="en">
            <TablePanel panel={groupPanel} />
          </DashboardRuntimeProvider>
        </DocumentProvider>
      </div>,
    )

    // The collapsed member is hidden; the toggle stands in for it, collapsed.
    expect(screen.queryByText('Legacy KASKO')).toBeNull()
    const toggle = screen.getByRole('button', { name: /Discontinued products \(1\)/ })
    expect(toggle).toHaveAttribute('aria-expanded', 'false')

    // The footer counts the 12 real rows (11 live + 1 member), not the toggle.
    expect(screen.getByText('12 rows')).toBeInTheDocument()

    // Expanding reveals the member and flips aria-expanded.
    fireEvent.click(toggle)
    expect(screen.getByText('Legacy KASKO')).toBeInTheDocument()
    expect(toggle).toHaveAttribute('aria-expanded', 'true')
  })
})

describe('TablePanel pagination', () => {
  it.each([
    { hasNext: false, disabled: true },
    { hasNext: true, disabled: false },
  ])('uses hasNext=$hasNext for an exact-multiple page', async ({ hasNext, disabled }) => {
    window.history.replaceState(null, '', '/?path=evidence&panel=evidence')
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(pageResponse(1, hasNext))
    render(
      <div className="lens-root">
        <DocumentProvider initialDocument={tableDocument}>
          <DashboardRuntimeProvider locale="en" fetcher={fetcher}>
            <TablePanel panel={tablePanel} />
          </DashboardRuntimeProvider>
        </DocumentProvider>
      </div>,
    )

    expect(await screen.findByText('Alpha')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Next' })).toHaveProperty('disabled', disabled)
  })

  it('queries and caches each server page without exposing page-local sorting as global', async () => {
    window.history.replaceState(null, '', '/?path=evidence&panel=evidence&region=north')
    const requestedPages: number[] = []
    const fetcher = vi.fn<typeof fetch>().mockImplementation((_input, init) => {
      const body = JSON.parse(typeof init?.body === 'string' ? init.body : '{}') as { page: number }
      requestedPages.push(body.page)
      return Promise.resolve(pageResponse(body.page))
    })
    render(
      <div className="lens-root">
        <DocumentProvider initialDocument={tableDocument}>
          <DashboardRuntimeProvider locale="en" fetcher={fetcher}>
            <TablePanel panel={tablePanel} />
          </DashboardRuntimeProvider>
        </DocumentProvider>
      </div>,
    )

    expect(await screen.findByText('Alpha')).toBeInTheDocument()
    expect(requestedPages).toEqual([1])
    expect(screen.queryByText('Sort applies to this page only')).not.toBeInTheDocument()
    expect(screen.getAllByRole('link', { name: 'Open record' })[0]).toHaveAttribute(
      'href', expect.stringContaining('/records/A%201?'),
    )

    expect(screen.queryByRole('button', { name: /amount/i })).not.toBeInTheDocument()
    expect(document.querySelectorAll('th')[2]).not.toHaveAttribute('aria-sort')
    expect(screen.getAllByRole('row')[1]).toHaveTextContent('Alpha')

    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    expect(await screen.findByText('Gamma')).toBeInTheDocument()
    expect(requestedPages).toEqual([1, 2])
    expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled()

    fireEvent.click(screen.getByRole('button', { name: 'Previous' }))
    await waitFor(() => expect(screen.getByText('Alpha')).toBeInTheDocument())
    expect(requestedPages).toEqual([1, 2])
  })
})

describe('TablePanel server readability features', () => {
  it('searches the full server frame and renders server totals, shares, heat, clamp, and low-sample context', async () => {
    const panel: Panel = {
      id: 'claims-products', kind: 'table', title: 'Claims by product', semantics: 'series', frame: 'panel:claims-products',
      encoding: { id: 'id', label: 'product', value: 'paid' }, deferred: true, table: { searchable: true },
      format: {
        claims: { kind: 'number', minorUnits: false, precision: 0 },
        paid: { kind: 'money', currency: 'USD', minorUnits: false, precision: 0 },
        share: { kind: 'percent', minorUnits: false, precision: 1 },
        average: { kind: 'money', currency: 'USD', minorUnits: false, precision: 0 },
      },
      columns: [
        { field: 'product', label: 'Product', cell: { kind: 'plain' }, clamp: 2 },
        { field: 'claims', label: 'Claims', cell: { kind: 'plain' }, total: true },
        { field: 'paid', label: 'Paid', cell: { kind: 'plain' }, heat: true, total: true },
        { field: 'share', label: 'Share', cell: { kind: 'plain' }, shareOf: 'paid', total: true },
        { field: 'average', label: 'Average', cell: { kind: 'plain' }, sampleSizeField: 'claims', minSampleSize: 5 },
      ],
      actions: [],
    }
    const document_: DashboardDocument = {
      version: '1.0.0', snapshotId: 'claims-snapshot',
      meta: { dashboardId: 'claims', title: 'Claims', generatedAt: '2026-07-31T00:00:00Z', locale: 'en' },
      layout: { rows: [{ panels: [{ panelId: panel.id, span: 12 }] }] }, panels: [panel],
      frames: { 'panel:claims-products': { columns: [], rows: [] } },
      drill: { inlineDepth: 0, edges: {} }, perspectives: [], endpoints: { panel: '/lens/panel' }, i18n: {}, theme: { palette: {}, series: {} },
    }
    const requests: Array<{ panels: Array<{ search?: string; sort?: { field: string; direction: string } }> }> = []
    const fetcher = vi.fn<typeof fetch>().mockImplementation((_input, init) => {
      const request = JSON.parse(typeof init?.body === 'string' ? init.body : '{}') as { panels: Array<{ search?: string; sort?: { field: string; direction: string } }> }
      requests.push(request)
      const filtered = request.panels[0]?.search === 'motor'
      const result = { frames: {
        'panel:claims-products': {
          columns: [
            { name: 'id', type: 'string' }, { name: 'product', type: 'string' }, { name: 'claims', type: 'number' },
            { name: 'paid', type: 'number' }, { name: 'share', type: 'number' }, { name: 'average', type: 'number' },
          ],
          rows: filtered
            ? [['1', 'Compulsory motor liability insurance with a deliberately long product label', 2, 75, 100, 37.5]]
            : [
              ['1', 'Compulsory motor liability insurance with a deliberately long product label', 2, 75, 75, 37.5],
              ['2', 'Travel', 10, 25, 25, 2.5],
            ],
        },
      },
      calculation: { durationMs: 12, cacheHit: false, calculatedAt: '2026-07-31T00:00:00Z' },
      summary: {
        values: filtered ? { claims: 2, paid: 75, share: 100 } : { claims: 12, paid: 100, share: 100 },
        ...(filtered ? { fullValues: { claims: 12, paid: 100, share: 100 } } : {}),
        filteredRows: filtered ? 1 : 2, totalRows: 2,
      } }
      return Promise.resolve(new Response(
        `${JSON.stringify({ panelId: 'claims-products', result })}\n${JSON.stringify({ complete: true })}\n`,
        { status: 200, headers: { 'Content-Type': 'application/x-ndjson' } },
      ))
    })

    render(
      <div className="lens-root">
        <DocumentProvider initialDocument={document_}>
          <DashboardRuntimeProvider locale="en" fetcher={fetcher}>
            <TablePanel panel={panel} />
          </DashboardRuntimeProvider>
        </DocumentProvider>
      </div>,
    )

    const longProduct = await screen.findByTitle('Compulsory motor liability insurance with a deliberately long product label')
    expect(longProduct).toHaveClass('lens-table-clamp')
    expect(screen.getByLabelText('Small sample: n=2, minimum 5')).toBeInTheDocument()
    expect(screen.getByText('Total')).toBeInTheDocument()
    expect(screen.getAllByRole('cell').some((cell) => cell.getAttribute('style')?.includes('color-mix'))).toBe(true)

    fireEvent.click(screen.getByRole('button', { name: /Paid/ }))
    await waitFor(() => expect(requests.at(-1)?.panels[0]?.sort).toEqual({ field: 'paid', direction: 'asc' }))

    const scroller = screen.getByLabelText('Scrollable table')
    Object.defineProperties(scroller, {
      clientWidth: { configurable: true, value: 505 },
      scrollWidth: { configurable: true, value: 733 },
      scrollLeft: { configurable: true, value: 0, writable: true },
    })
    fireEvent.scroll(scroller)
    const scrollFrame = scroller.closest('.lens-table-scroll-frame')
    expect(scroller).toHaveAttribute('tabindex', '0')
    expect(scrollFrame).toHaveAttribute('data-overflow-left', 'false')
    expect(scrollFrame).toHaveAttribute('data-overflow-right', 'true')
    const scrollLeft = screen.getByRole('button', { name: 'Scroll table left' })
    const scrollRight = screen.getByRole('button', { name: 'Scroll table right' })
    expect(scrollLeft).toBeDisabled()
    expect(scrollRight).toBeEnabled()
    expect(scrollFrame?.querySelectorAll('.lens-table-scroll')).toHaveLength(1)

    fireEvent.click(scrollRight)
    expect(scroller.scrollLeft).toBe(228)
    expect(scroller).toHaveFocus()
    expect(scrollLeft).toBeEnabled()
    expect(scrollRight).toBeDisabled()
    expect(scrollFrame).toHaveAttribute('data-overflow-left', 'true')
    expect(scrollFrame).toHaveAttribute('data-overflow-right', 'false')

    fireEvent.click(scrollLeft)
    expect(scroller.scrollLeft).toBe(0)
    expect(scroller).toHaveFocus()
    expect(scrollLeft).toBeDisabled()
    expect(scrollRight).toBeEnabled()

    scroller.scrollLeft = 228
    fireEvent.scroll(scroller)
    expect(scrollFrame).toHaveAttribute('data-overflow-left', 'true')
    expect(scrollFrame).toHaveAttribute('data-overflow-right', 'false')

    fireEvent.change(screen.getByRole('searchbox', { name: 'Search table' }), { target: { value: 'motor' } })
    await waitFor(() => expect(requests.at(-1)?.panels[0]?.search).toBe('motor'))
    expect(requests.at(-1)?.panels[0]?.sort).toEqual({ field: 'paid', direction: 'asc' })
    await waitFor(() => expect(screen.queryByText('Travel')).not.toBeInTheDocument())
    expect(screen.getAllByText('$75')).toHaveLength(2)
  })
})
