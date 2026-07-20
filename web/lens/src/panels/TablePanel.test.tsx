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

  it('queries and caches each server page while sorting only the fetched page', async () => {
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
    expect(screen.getByText('Sort applies to this page only')).toBeInTheDocument()
    expect(screen.getAllByRole('link', { name: 'Open record' })[0]).toHaveAttribute(
      'href', expect.stringContaining('/records/A%201?'),
    )

    fireEvent.click(screen.getByRole('button', { name: /amount/ }))
    const dataRows = screen.getAllByRole('row').slice(1)
    expect(dataRows[0]).toHaveTextContent('Beta')

    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    expect(await screen.findByText('Gamma')).toBeInTheDocument()
    expect(requestedPages).toEqual([1, 2])
    expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled()

    fireEvent.click(screen.getByRole('button', { name: 'Previous' }))
    await waitFor(() => expect(screen.getByText('Alpha')).toBeInTheDocument())
    expect(requestedPages).toEqual([1, 2])
  })
})
