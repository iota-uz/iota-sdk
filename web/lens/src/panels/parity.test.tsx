import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { DashboardDocument, Frame, Panel } from '../contract'
import { clusterRow, DashboardPanels } from '../DashboardPanels'
import { DashboardRuntimeProvider, DocumentProvider } from '../runtime'
import type { ChartAdapter, ChartInput } from '../charts/adapter'
import { ChartPanel } from './ChartPanel'
import { CoveragePanel } from './CoveragePanel'
import { StatMetric, StatPanel } from './StatPanel'
import { PanelSkeletonBody } from './Skeleton'
import { TablePanel } from './TablePanel'

afterEach(cleanup)

function documentWith(panels: Panel[], frames: Record<string, Frame>, layout?: DashboardDocument['layout']): DashboardDocument {
  return {
    version: '1.0.0',
    snapshotId: 'parity-snapshot',
    meta: { dashboardId: 'parity', title: '', generatedAt: '2026-07-19T00:00:00Z', locale: 'en' },
    layout: layout ?? { rows: [{ panels: panels.map((panel) => ({ panelId: panel.id, span: 12 })) }] },
    panels,
    frames,
    drill: { inlineDepth: 0, edges: {} },
    perspectives: [],
    endpoints: {},
    i18n: {},
    theme: { palette: {}, series: {} },
  }
}

function renderDocument(document: DashboardDocument, children: React.ReactNode) {
  return render(
    <div className="lens-root">
      <DocumentProvider initialDocument={document}>
        <DashboardRuntimeProvider locale="en">{children}</DashboardRuntimeProvider>
      </DocumentProvider>
    </div>,
  )
}

const statPanel: Panel = {
  id: 'loss-ratio', kind: 'stat', title: 'Loss ratio', semantics: 'series', frame: 'stat:root',
  encoding: { label: 'label', value: 'value' },
  format: { value: { kind: 'percent', minorUnits: false, precision: 1 } },
  accent: '#2f56d9',
  status: { label: 'Estimate', tone: 'warning' },
  actions: [],
}

const statFrame: Frame = {
  columns: [{ name: 'label', type: 'string' }, { name: 'value', type: 'number' }],
  rows: [['Loss ratio', 3.1]],
}

describe('stat panels', () => {
  it('renders the metric form with a bullet, uppercase label and status chip', () => {
    const { container } = renderDocument(
      documentWith([statPanel], { 'stat:root': statFrame }),
      <StatMetric panel={statPanel} />,
    )

    expect(container.querySelector('.lens-stat-metric-bullet')).toHaveStyle({ background: '#2f56d9' })
    expect(screen.getByText('Estimate')).toHaveClass('lens-status-chip-warning')
    expect(screen.getByText('3.1%')).toHaveClass('lens-stat-metric-value')
  })

  it('drops a dataset label that only repeats the panel title', () => {
    const { container } = renderDocument(
      documentWith([statPanel], { 'stat:root': statFrame }),
      <StatPanel panel={statPanel} />,
    )

    // "Loss ratio" is the panel title, so the card shows it once, in the header.
    expect(screen.getAllByText('Loss ratio')).toHaveLength(1)
    // The status chip still forces the label row to exist.
    expect(container.querySelector('.lens-stat-label')).not.toBeNull()
  })

  it('keeps a dataset label that says something new', () => {
    const document = documentWith([statPanel], {
      'stat:root': { ...statFrame, rows: [['Net of reinsurance', 3.1]] },
    })
    renderDocument(document, <StatPanel panel={statPanel} />)

    expect(screen.getByText('Net of reinsurance')).toBeInTheDocument()
  })

  it('renders multiline captions without flattening their content', () => {
    const panel: Panel = { ...statPanel, caption: 'First caveat\nSecond caveat' }
    const { container } = renderDocument(
      documentWith([panel], { 'stat:root': statFrame }),
      <StatPanel panel={panel} />,
    )

    expect(container.querySelector('.lens-panel-caption')?.textContent).toBe('First caveat\nSecond caveat')
  })
})

const coveragePanel: Panel = {
  id: 'claims-coverage', kind: 'coverage', title: 'Claims paid', semantics: 'partition', frame: 'coverage:root',
  encoding: { label: 'label', value: 'amount' },
  format: { amount: { kind: 'money', currency: 'UZS', minorUnits: false, precision: 0 } },
  caption: 'All claims covered by reserve',
  headline: 5_458_561_140,
  actions: [],
}

const coverageFrame: Frame = {
  columns: [{ name: 'label', type: 'string' }, { name: 'amount', type: 'number' }],
  rows: [['Within reserve', 5_458_561_140], ['Above reserve', 0]],
}

describe('coverage panel', () => {
  it('renders a headline, caption, segmented track and legend rows with shares', () => {
    const { container } = renderDocument(
      documentWith([coveragePanel], { 'coverage:root': coverageFrame }),
      <CoveragePanel panel={coveragePanel} />,
    )

    expect(container.querySelector('.lens-coverage-headline')?.textContent).toContain('5,458,561,140')
    expect(screen.getByText('All claims covered by reserve')).toBeInTheDocument()
    // Zero-width segments never enter the track, but keep their legend row.
    expect(container.querySelectorAll('.lens-coverage-track-segment')).toHaveLength(1)
    expect(container.querySelector('.lens-coverage-track-segment')).toHaveStyle({ width: '100%' })
    const shares = [...container.querySelectorAll('.lens-coverage-legend-share')].map((node) => node.textContent)
    expect(shares).toEqual(['100%', '0%'])
  })

  it('honours a precision of 0 in the headline instead of falling back to locale defaults', () => {
    const panel: Panel = { ...coveragePanel, headline: 51_522_007_533.993 }
    const { container } = renderDocument(
      documentWith([panel], { 'coverage:root': coverageFrame }),
      <CoveragePanel panel={panel} />,
    )

    expect(container.querySelector('.lens-coverage-headline')?.textContent).toContain('51,522,007,534')
    expect(container.querySelector('.lens-coverage-headline')?.textContent).not.toContain('.993')
  })
})

const tablePanel: Panel = {
  id: 'groups', kind: 'table', title: 'Groups', semantics: 'evidence', frame: 'groups:root',
  encoding: { id: 'group_id', label: 'name' },
  format: {
    earned: { kind: 'number', minorUnits: false, precision: 2, compact: true, decimalSeparator: '.' },
    balance: { kind: 'number', minorUnits: false, precision: 2, compact: true, decimalSeparator: '.' },
    delta_pct: { kind: 'percent', minorUnits: false, precision: 1, decimalSeparator: '.' },
  },
  columns: [
    { field: 'name', label: 'Product', cell: { kind: 'plain' }, clamp: 2, widthPx: 180 },
    {
      field: 'earned', label: 'Earned', align: 'right', cell: { kind: 'plain' }, affordance: 'pill',
      action: { kind: 'navigate_to_leaf', urlSource: { kind: 'field', name: 'detail_url' }, params: [], payload: {} },
    },
    { field: 'balance', label: 'Balance', align: 'right', cell: { kind: 'underline' } },
    {
      field: 'delta', label: 'Change', align: 'right',
      cell: { kind: 'delta', secondaryField: 'delta_pct', layout: 'stacked' },
    },
    {
      field: '', label: '', text: 'Open', cell: { kind: 'plain' },
      action: { kind: 'navigate_to_leaf', urlSource: { kind: 'field', name: 'detail_url' }, params: [], payload: {} },
    },
  ],
  actions: [{
    kind: 'navigate_to_leaf', urlTemplate: '/groups/{id}',
    params: [{ name: 'id', source: { kind: 'field', name: 'group_id' } }], payload: {},
  }],
}

const tableFrame: Frame = {
  columns: [
    { name: 'group_id', type: 'string' }, { name: 'name', type: 'string' },
    { name: 'earned', type: 'number' }, { name: 'balance', type: 'number' },
    { name: 'delta', type: 'number' }, { name: 'delta_pct', type: 'number' },
    { name: 'detail_url', type: 'string' },
  ],
  rows: [
    ['a', 'Group A', 9_364_442_607, 150_530_000, -12_030_000, -0.6, '/groups/a'],
    ['b', 'Group B', 4_100_000_000, -75_000_000, 13_400_000, 13, '/groups/b'],
  ],
}

describe('table parity treatments', () => {
  it('renders pill, underline, stacked delta, clamp, action text and a panel-level leaf column', () => {
    const { container } = renderDocument(
      documentWith([tablePanel], { 'groups:root': tableFrame }),
      <TablePanel panel={tablePanel} />,
    )

    // Compact cells with the pinned separator, wrapped in a drill pill.
    const pill = container.querySelector('.lens-table-cell-pill')
    expect(pill?.textContent).toContain('9.36B')
    // The drill arrow is a Phosphor glyph now, not a text character.
    expect(pill?.querySelector('svg')).not.toBeNull()

    // Underline rules follow the sign and never shrink into a stray hyphen:
    // they span the value instead of encoding magnitude.
    const rules = container.querySelectorAll<HTMLElement>('.lens-table-underline-rule')
    expect(rules).toHaveLength(2)
    expect(rules[1]).toHaveClass('lens-table-underline-rule-negative')
    expect(rules[0]?.style.width).toBe('')
    expect(container.querySelector('.lens-table-underline')?.textContent).not.toContain('-—')

    // Stacked delta puts the percent first and the amount below it.
    const stacked = container.querySelector('.lens-table-delta-stacked')
    expect(stacked?.firstElementChild).toHaveClass('lens-table-delta-pct-negative')
    expect(stacked?.lastElementChild).toHaveClass('lens-table-delta-value')

    // Clamped product names and pinned width.
    expect(container.querySelector('.lens-table-clamp')).toHaveStyle({ '-webkit-line-clamp': '2' })
    expect(container.querySelector('th')).toHaveStyle({ 'min-width': '180px' })

    // An action-only column renders its literal text, not an em dash.
    expect(screen.getAllByText('Open')).toHaveLength(2)

    // The panel-level leaf action reaches the DOM in columns mode.
    const openRecord = screen.getAllByText('Open record')
    expect(openRecord).toHaveLength(2)
    expect(openRecord[0]).toHaveAttribute('href', expect.stringContaining('/groups/a'))
  })

  it('offers no sort control on an action-only column', () => {
    renderDocument(documentWith([tablePanel], { 'groups:root': tableFrame }), <TablePanel panel={tablePanel} />)

    const headers = screen.getAllByRole('columnheader')
    // 5 declared columns + the appended leaf action column.
    expect(headers).toHaveLength(6)
    expect(headers[4]?.querySelector('button')).toBeNull()
  })
})

describe('layout groups', () => {
  const first: Panel = { ...statPanel, id: 'metric-a', title: 'Metric A' }
  const second: Panel = { ...statPanel, id: 'metric-b', title: 'Metric B' }
  const metricsLayout: DashboardDocument['layout'] = {
    rows: [{
      heading: 'Key ratios',
      panels: [
        { panelId: 'metric-a', span: 3, group: { id: 'ratios', kind: 'metrics', label: 'By earned premium', layout: 'columns', span: 12 } },
        { panelId: 'metric-b', span: 3, group: { id: 'ratios', kind: 'metrics', label: 'By earned premium', layout: 'columns', span: 12 } },
      ],
    }],
  }

  it('groups consecutive items that share a group id', () => {
    const clusters = clusterRow([
      { panelId: 'a', span: 3, group: { id: 'g', kind: 'metrics', span: 12 } },
      { panelId: 'b', span: 3, group: { id: 'g', kind: 'metrics', span: 12 } },
      { panelId: 'c', span: 6 },
      { panelId: 'd', span: 3, group: { id: 'h', kind: 'metrics', span: 12 } },
    ])

    expect(clusters.map((cluster) => cluster.items.map((item) => item.panelId))).toEqual([['a', 'b'], ['c'], ['d']])
  })

  it('renders a metrics group as one card with a metric row', () => {
    const document = documentWith([first, second], { 'stat:root': statFrame }, metricsLayout)
    const { container } = renderDocument(document, <DashboardPanels />)

    expect(container.querySelectorAll('.lens-panel-group')).toHaveLength(1)
    expect(container.querySelectorAll('.lens-stat-metric')).toHaveLength(2)
    expect(screen.getByText('By earned premium')).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('Key ratios')
  })

  it('renders a tabs group as a segmented strip showing one tab at a time', () => {
    const tabsLayout: DashboardDocument['layout'] = {
      rows: [{
        panels: [
          { panelId: 'metric-a', span: 12, group: { id: 'result', kind: 'tabs', span: 12, tab: 'Cash' } },
          { panelId: 'metric-b', span: 12, group: { id: 'result', kind: 'tabs', span: 12, tab: 'Underwriting' } },
        ],
      }],
    }
    const document = documentWith([first, second], { 'stat:root': statFrame }, tabsLayout)
    renderDocument(document, <DashboardPanels />)

    const tabs = screen.getAllByRole('tab')
    expect(tabs.map((tab) => tab.textContent)).toEqual(['Cash', 'Underwriting'])
    expect(tabs[0]).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('tabpanel').textContent).toContain('Metric A')
    expect(screen.getByRole('tabpanel').textContent).not.toContain('Metric B')
  })
})

describe('loading placeholders', () => {
  it('mirrors the layout instead of showing a spinner while the document loads', () => {
    const fetcher = vi.fn<typeof fetch>().mockReturnValue(new Promise<Response>(() => undefined))
    const { container } = render(
      <div className="lens-root">
        <DocumentProvider src="/lens/document" fetcher={fetcher}>
          <DashboardRuntimeProvider locale="en"><DashboardPanels /></DashboardRuntimeProvider>
        </DocumentProvider>
      </div>,
    )

    expect(container.querySelector('.lens-loading')).toHaveAttribute('aria-busy', 'true')
    expect(container.querySelectorAll('.lens-skeleton-card').length).toBeGreaterThan(0)
    expect(container.textContent).not.toContain('Loading dashboard')
  })

  it('shapes the panel placeholder from the panel kind', () => {
    const { container } = render(<PanelSkeletonBody kind="stat" />)
    expect(container.querySelector('.lens-skeleton-card-stat')).not.toBeNull()

    cleanup()
    const table = render(<PanelSkeletonBody kind="table" />)
    expect(table.container.querySelector('.lens-skeleton-card-plot')).not.toBeNull()
  })
})

const plainPillPanel: Panel = {
  id: 'plain-pill', kind: 'table', title: 'Groups', semantics: 'series', frame: 'groups:root',
  encoding: { id: 'group_id', label: 'name' },
  format: { earned: { kind: 'number', minorUnits: false, precision: 2, compact: true, decimalSeparator: '.' } },
  columns: [
    { field: 'name', label: 'Product', cell: { kind: 'plain' } },
    // Pill without a wire action: the drill lives in the host renderer.
    { field: 'earned', label: 'Earned', align: 'right', cell: { kind: 'plain' }, affordance: 'pill' },
  ],
  actions: [],
}

describe('drill affordance', () => {
  it('renders a pill on a plain cell that carries no wire action, without claiming a link', () => {
    const { container } = renderDocument(
      documentWith([plainPillPanel], { 'groups:root': tableFrame }),
      <TablePanel panel={plainPillPanel} />,
    )

    const pills = container.querySelectorAll('.lens-table-cell-pill')
    expect(pills).toHaveLength(2)
    expect(pills[0]?.tagName).toBe('SPAN')
    expect(pills[0]?.textContent).toContain('9.36B')
    // No arrow without a target: the affordance must not promise navigation
    // the runtime cannot perform.
    expect(container.querySelector('.lens-table-cell-link-arrow')).toBeNull()
  })

  it('renders zero and null underline cells without a stray rule', () => {
    const zeroFrame: Frame = {
      ...tableFrame,
      rows: [
        ['a', 'Group A', 1, 0, 0, 0, '/groups/a'],
        ['b', 'Group B', 1, null, 0, 0, '/groups/b'],
      ],
    }
    const { container } = renderDocument(
      documentWith([tablePanel], { 'groups:root': zeroFrame }),
      <TablePanel panel={tablePanel} />,
    )

    // Zero keeps a neutral rule; a missing number has none at all.
    const rules = container.querySelectorAll('.lens-table-underline-rule')
    expect(rules).toHaveLength(1)
    expect(rules[0]).not.toHaveClass('lens-table-underline-rule-negative')
  })
})

describe('expanded panel', () => {
  const expandable: Panel = { ...coveragePanel, id: 'expandable' }

  function openExpanded(children: React.ReactNode) {
    const view = renderDocument(
      documentWith([expandable], { 'coverage:root': coverageFrame }),
      children,
    )
    fireEvent.click(screen.getByRole('button', { name: 'Expand panel' }))
    return view
  }

  it('renders the panel in a modal dialog at the end of body, not inside the grid', () => {
    const { container } = openExpanded(<CoveragePanel panel={expandable} />)

    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveAttribute('aria-modal', 'true')
    expect(dialog).toHaveAttribute('aria-label', 'Claims paid')
    // Portaled out of the dashboard: no ancestor stacking context can trap it.
    expect(container.contains(dialog)).toBe(false)
    expect(dialog.closest('.lens-root')).toHaveClass('lens-overlay-root')
    expect(document.body.lastElementChild).toContainElement(dialog)
    expect(dialog.querySelector('.lens-panel')).not.toBeNull()
    expect(document.body.style.overflow).toBe('hidden')
  })

  it('carries the dashboard theme onto the portal host', () => {
    render(
      <div className="lens-root" data-theme="dark">
        <DocumentProvider initialDocument={documentWith([expandable], { 'coverage:root': coverageFrame })}>
          <DashboardRuntimeProvider locale="en"><CoveragePanel panel={expandable} /></DashboardRuntimeProvider>
        </DocumentProvider>
      </div>,
    )
    fireEvent.click(screen.getByRole('button', { name: 'Expand panel' }))

    expect(screen.getByRole('dialog').closest('.lens-root')).toHaveAttribute('data-theme', 'dark')
  })

  it.each([
    ['Escape key', () => fireEvent.keyDown(document, { key: 'Escape' })],
    ['backdrop click', () => fireEvent.mouseDown(document.querySelector('.lens-panel-overlay')!)],
  ])('closes on %s and restores focus to the expand button', (_name, close) => {
    openExpanded(<CoveragePanel panel={expandable} />)
    close()

    expect(screen.queryByRole('dialog')).toBeNull()
    expect(document.body.style.overflow).not.toBe('hidden')
    expect(screen.getByRole('button', { name: 'Expand panel' })).toHaveFocus()
  })

  it('keeps a click inside the dialog from dismissing it', () => {
    openExpanded(<CoveragePanel panel={expandable} />)
    fireEvent.mouseDown(screen.getByRole('dialog'))

    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('escapes a tab group without being trapped by its card', () => {
    const layout: DashboardDocument['layout'] = {
      rows: [{
        panels: [{ panelId: 'expandable', span: 12, group: { id: 'result', kind: 'tabs', span: 12, tab: 'Cash' } }],
      }],
    }
    const { container } = renderDocument(
      documentWith([expandable], { 'coverage:root': coverageFrame }, layout),
      <DashboardPanels />,
    )
    fireEvent.click(screen.getByRole('button', { name: 'Expand panel' }))

    const dialog = screen.getByRole('dialog')
    expect(container.contains(dialog)).toBe(false)
    expect(container.querySelector('.lens-panel-placeholder')).not.toBeNull()
  })
})

const piePanel: Panel = {
  id: 'mix', kind: 'pie', title: 'Premium mix', semantics: 'partition', frame: 'mix:root',
  encoding: { id: 'id', label: 'label', value: 'amount' },
  format: { amount: { kind: 'number', minorUnits: false, precision: 0 } },
  presentation: { legend: 'below', sliceLabels: 'percent', totalBadge: 'plot' },
  total: 1000,
  actions: [],
}

const pieFrame: Frame = {
  columns: [{ name: 'id', type: 'string' }, { name: 'label', type: 'string' }, { name: 'amount', type: 'number' }],
  rows: [['direct', 'Direct', 600], ['broker', 'Broker', 300], ['inward', 'Inward', 100]],
}

describe('panel header pressure', () => {
  it('gives the title and the total badge their own tooltips so truncation stays recoverable', () => {
    const panel: Panel = {
      ...piePanel,
      title: 'Финансовые расходы и прочие операционные затраты',
      presentation: { legend: 'below', sliceLabels: 'percent' },
    }
    const { container } = renderDocument(
      documentWith([panel], { 'mix:root': pieFrame }),
      <ChartPanel panel={panel} adapter={{ mount: () => ({ update: () => {}, dispose: () => {} }) }} />,
    )

    expect(container.querySelector('.lens-panel-title')).toHaveAttribute('title', panel.title)
    expect(container.querySelector('.lens-panel-total')).toHaveAttribute('title', 'Total: 1,000')
  })
})

describe('chart legend series toggle', () => {
  function renderPie() {
    const inputs: Array<ChartInput> = []
    const adapter: ChartAdapter = {
      mount: (_element, initial) => {
        inputs.push(initial)
        return { update: (next) => inputs.push(next), dispose: () => {} }
      },
    }
    const view = renderDocument(
      documentWith([piePanel], { 'mix:root': pieFrame }),
      <ChartPanel panel={piePanel} adapter={adapter} />,
    )
    return { ...view, inputs }
  }

  it('removes the datum so the chart re-normalizes, and follows the total badge', async () => {
    const { inputs, container } = renderPie()
    await waitFor(() => expect(inputs.length).toBeGreaterThan(0))
    expect(container.querySelector('.lens-plot-total')?.textContent).toContain('1,000')

    fireEvent.click(screen.getByRole('button', { name: /Broker/ }))

    await waitFor(() => {
      // Hidden series leave the data entirely — dimming would keep every slice
      // percentage computed against the old total.
      expect(inputs.at(-1)?.frame.rows.map((row) => row[0])).toEqual(['direct', 'inward'])
    })
    expect(screen.getByRole('button', { name: /Broker/ })).toHaveAttribute('aria-pressed', 'false')
    expect(container.querySelector('.lens-plot-total')?.textContent).toContain('700')
  })

  it('refuses to hide the last visible series', async () => {
    const { inputs } = renderPie()
    await waitFor(() => expect(inputs.length).toBeGreaterThan(0))

    fireEvent.click(screen.getByRole('button', { name: /Broker/ }))
    fireEvent.click(screen.getByRole('button', { name: /Inward/ }))

    await waitFor(() => expect(screen.getByRole('button', { name: /Direct/ })).toBeDisabled())
    fireEvent.click(screen.getByRole('button', { name: /Direct/ }))
    expect(inputs.at(-1)?.frame.rows.map((row) => row[0])).toEqual(['direct'])
  })
})
