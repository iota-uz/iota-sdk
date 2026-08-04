import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Action, DashboardDocument, Frame, Panel } from '../contract'
import type { ChartAdapter, ChartInput } from '../charts/adapter'
import { fallbackMarkKey } from '../charts/keys'
import { CoveragePanel, ChartPanel, MarkSelectionContext, StatMetric, StatPanel } from './index'
import { CascadePanel } from './CascadePanel'
import { DashboardRuntimeProvider, DocumentProvider } from '../runtime'
import { navigateTo } from '../runtime/navigate'

vi.mock('../runtime/navigate', () => ({ navigateTo: vi.fn() }))

/**
 * Panel-level navigate actions are what made a legacy stat card, segment bar
 * or chart data point clickable. These tests are the regression guard: a
 * dashboard that ships the action must render a reachable link.
 */

afterEach(() => {
  cleanup()
  vi.mocked(navigateTo).mockClear()
  vi.restoreAllMocks()
})

const navigate = (urlTemplate: string, params: Action['params'] = []): Action => ({
  kind: 'navigate', method: 'GET', urlTemplate, params, payload: {},
})

function documentWith(panels: Panel[], frames: Record<string, Frame>): DashboardDocument {
  return {
    version: '1.0.0',
    snapshotId: 'actions-snapshot',
    meta: { dashboardId: 'actions', title: '', generatedAt: '2026-07-20T00:00:00Z', locale: 'en' },
    layout: { rows: [{ panels: panels.map((panel) => ({ panelId: panel.id, span: 12 })) }] },
    panels,
    frames,
    drill: { inlineDepth: 0, edges: {} },
    perspectives: [],
    endpoints: {},
    i18n: {},
    theme: { palette: {}, series: {} },
  }
}

function renderPanel(document: DashboardDocument, children: React.ReactNode) {
  return render(
    <div className="lens-root">
      <DocumentProvider initialDocument={document}>
        <DashboardRuntimeProvider locale="en">{children}</DashboardRuntimeProvider>
      </DocumentProvider>
    </div>,
  )
}

const statFrame: Frame = {
  columns: [{ name: 'label', type: 'string' }, { name: 'value', type: 'number' }],
  rows: [['Loss ratio', 3.1]],
}

function statPanel(actions: Action[]): Panel {
  return {
    id: 'loss-ratio', kind: 'stat', title: 'Loss ratio', semantics: 'series', frame: 'stat:root',
    encoding: { label: 'label', value: 'value' },
    format: { value: { kind: 'percent', minorUnits: false, precision: 1 } },
    actions,
  }
}

describe('stat panels with a panel-level navigate action', () => {
  it('covers the card with a link', () => {
    const panel = statPanel([navigate('/analytics/metrics/loss-ratio')])
    renderPanel(documentWith([panel], { 'stat:root': statFrame }), <StatPanel panel={panel} />)

    expect(screen.getByRole('link', { name: 'Open Loss ratio' }))
      .toHaveAttribute('href', expect.stringContaining('/analytics/metrics/loss-ratio'))
  })

  it('keeps the compact metric form clickable inside a metrics group', () => {
    const panel = statPanel([navigate('/analytics/metrics/loss-ratio')])
    renderPanel(documentWith([panel], { 'stat:root': statFrame }), <StatMetric panel={panel} />)

    expect(screen.getByRole('link', { name: 'Open Loss ratio' })).toBeInTheDocument()
  })

  it('stays inert without an action', () => {
    const panel = statPanel([])
    renderPanel(documentWith([panel], { 'stat:root': statFrame }), <StatPanel panel={panel} />)

    expect(screen.queryByRole('link')).toBeNull()
  })

  it('warms a ready stat drawer through the bounded idle queue and reuses it on click', async () => {
    window.history.replaceState(null, '', '/')
    const action: Action = {
      kind: 'open_drawer', method: 'POST',
      drawerKey: { kind: 'literal', value: 'loss_ratio' },
      params: [], payload: {},
    }
    const panel = statPanel([action])
    const document = documentWith([panel], { 'stat:root': statFrame })
    document.endpoints = { drawer: '/lens/drawer' }
    const child = {
      ...document,
      snapshotId: 'loss-ratio-child',
      meta: { ...document.meta, title: 'Loss ratio detail' },
      endpoints: {},
    }
    let resolverCalls = 0
    const fetcher = vi.fn<typeof fetch>((input) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
      if (url.endsWith('/lens/drawer')) {
        resolverCalls += 1
        return Promise.resolve(new Response(JSON.stringify({ url: `/lens/document?ticket=${resolverCalls}` }), { status: 200 }))
      }
      return Promise.resolve(new Response(JSON.stringify(child), { status: 200 }))
    })

    render(
      <div className="lens-root">
        <DocumentProvider initialDocument={document}>
          <DashboardRuntimeProvider fetcher={fetcher} locale="en">
            <StatMetric panel={panel} />
          </DashboardRuntimeProvider>
        </DocumentProvider>
      </div>,
    )

    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2), { timeout: 2_000 })
    expect(fetcher.mock.calls.map(([input]) => typeof input === 'string' ? input : input instanceof URL ? input.href : input.url))
      .toEqual(['/lens/drawer', '/lens/document?ticket=1'])

    fireEvent.click(screen.getByRole('link', { name: 'Open Loss ratio' }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText('3.1%')).toBeInTheDocument()
    expect(fetcher).toHaveBeenCalledTimes(2)
  })
})

const coverageFrame: Frame = {
  columns: [{ name: 'id', type: 'string' }, { name: 'label', type: 'string' }, { name: 'amount', type: 'number' }],
  rows: [['within', 'Within reserve', 800], ['above', 'Above reserve', 200]],
}

function coveragePanel(actions: Action[]): Panel {
  return {
    id: 'claims', kind: 'coverage', title: 'Claims paid', semantics: 'partition', frame: 'coverage:root',
    encoding: { id: 'id', label: 'label', value: 'amount' },
    format: { amount: { kind: 'number', minorUnits: false, precision: 0 } },
    actions,
  }
}

describe('coverage panels with a panel-level navigate action', () => {
  it('links each legend row once and keeps plotted segments decorative when the action reads a row field', () => {
    const panel = coveragePanel([navigate('/claims/{bucket}', [
      { name: 'bucket', source: { kind: 'field', name: 'id' } },
    ])])
    const { container } = renderPanel(
      documentWith([panel], { 'coverage:root': coverageFrame }),
      <CoveragePanel panel={panel} />,
    )

    const legend = [...container.querySelectorAll<HTMLAnchorElement>('.lens-coverage-legend-link')]
    expect(legend.map((link) => new URL(link.href).pathname)).toEqual(['/claims/within', '/claims/above'])
    expect(container.querySelectorAll('.lens-coverage-track-segment-link')).toHaveLength(0)
    expect(container.querySelector('.lens-coverage-track')).toHaveAttribute('aria-hidden', 'true')
    // A row-scoped action belongs to the segments, never to the whole card.
    expect(container.querySelector('.lens-card-link')).toBeNull()
  })

  it('links a hovered or focused segment to the legend row its drill will open', () => {
    const panel = coveragePanel([navigate('/claims/{bucket}', [
      { name: 'bucket', source: { kind: 'field', name: 'id' } },
    ])])
    const { container } = renderPanel(
      documentWith([panel], { 'coverage:root': coverageFrame }),
      <CoveragePanel panel={panel} />,
    )

    const segments = container.querySelectorAll('.lens-coverage-track-segment')
    const rows = container.querySelectorAll('.lens-coverage-legend-row')
    fireEvent.pointerEnter(segments[0]!)
    expect(segments[0]).toHaveAttribute('data-highlighted', 'true')
    expect(rows[0]).toHaveAttribute('data-highlighted', 'true')
    expect(segments[1]).not.toHaveAttribute('data-highlighted')
    expect(rows[1]).not.toHaveAttribute('data-highlighted')

    fireEvent.pointerLeave(segments[0]!)
    fireEvent.focus(rows[1]!.querySelector('a')!)
    expect(segments[1]).toHaveAttribute('data-highlighted', 'true')
    expect(rows[1]).toHaveAttribute('data-highlighted', 'true')
  })

  it('links the whole card when the action does not depend on a row', () => {
    const panel = coveragePanel([navigate('/claims')])
    const { container } = renderPanel(
      documentWith([panel], { 'coverage:root': coverageFrame }),
      <CoveragePanel panel={panel} />,
    )

    expect(container.querySelector('.lens-card-link')).toHaveAttribute('href', expect.stringContaining('/claims'))
    expect(container.querySelectorAll('.lens-coverage-legend-link')).toHaveLength(0)
  })
})

const chartFrame: Frame = {
  columns: [{ name: 'id', type: 'string' }, { name: 'label', type: 'string' }, { name: 'amount', type: 'number' }],
  rows: [['direct', 'Direct', 600], ['broker', 'Broker', 400]],
}

function chartPanel(actions: Action[]): Panel {
  return {
    id: 'mix', kind: 'pie', title: 'Risk split', semantics: 'partition', frame: 'chart:root',
    encoding: { id: 'id', label: 'label', value: 'amount' },
    format: { amount: { kind: 'number', minorUnits: false, precision: 0 } },
    actions,
  }
}

describe('charts with a panel-level navigate action', () => {
  function renderChart(panel: Panel, frame = chartFrame) {
    let select: ((key: string) => void) | undefined
    const adapter: ChartAdapter = {
      mount: (_element, _input, events) => {
        select = (key) => events.onSelect(key)
        return { update: () => {}, dispose: () => {} }
      },
    }
    const view = renderPanel(
      documentWith([panel], { [panel.frame]: frame }),
      <ChartPanel panel={panel} adapter={adapter} />,
    )
    return { ...view, activate: (key: string) => select?.(key) }
  }

  it('navigates to the clicked mark, and marks the host interactive', async () => {
    const assign = vi.mocked(navigateTo)
    const panel = chartPanel([navigate('/risk/{segment}', [
      { name: 'segment', source: { kind: 'field', name: 'id' } },
    ])])
    const { container, activate } = renderChart(panel)

    await waitFor(() => expect(container.querySelector('[data-drillable]')).not.toBeNull())
    activate('broker')
    expect(assign).toHaveBeenCalledWith(expect.stringContaining('/risk/broker'))
  })

  it('resolves an actionable stacked mark when the frame has no explicit id column', async () => {
    const assign = vi.mocked(navigateTo)
    const frame: Frame = {
      columns: [
        { name: 'day', type: 'string' },
        { name: 'product', type: 'string' },
        { name: 'amount', type: 'number' },
        { name: 'product_code', type: 'string' },
      ],
      rows: [
        ['03.07', 'ОСАГО', 12_000_000, 'OSAGO'],
        ['03.07', 'КАСКО', 2_500_000, 'KASKO'],
      ],
    }
    const panel: Panel = {
      id: 'daily', kind: 'bar', title: 'Daily revenue', semantics: 'series', frame: 'daily:root',
      // Production keeps the legacy `label` role even though the deferred
      // daily frame only contains `day` as its category column.
      encoding: { id: 'id', label: 'label', category: 'day', series: 'product', value: 'amount' },
      format: { amount: { kind: 'number', minorUnits: false, precision: 0 } },
      actions: [navigate('/policies/{product}', [
        { name: 'product', source: { kind: 'field', name: 'product_code' } },
      ])],
    }
    const { container, activate } = renderChart(panel, frame)

    await waitFor(() => expect(container.querySelector('[data-drillable]')).not.toBeNull())
    activate(fallbackMarkKey('03.07', 'КАСКО')!)
    expect(assign).toHaveBeenCalledWith(expect.stringContaining('/policies/KASKO'))
  })

  it('leaves a chart without a drill root or an action inert', async () => {
    const assign = vi.mocked(navigateTo)
    const { container, activate } = renderChart(chartPanel([]))

    await waitFor(() => expect(container.querySelector('.lens-chart-host')).not.toBeNull())
    expect(container.querySelector('[data-drillable]')).toBeNull()
    activate('broker')
    expect(assign).not.toHaveBeenCalled()
  })

  it('resolves ring and category fields from a radial mark', async () => {
    const assign = vi.mocked(navigateTo)
    const frame: Frame = {
      columns: [
        { name: 'id', type: 'string' },
        { name: 'label', type: 'string' },
        { name: 'ring', type: 'string' },
        { name: 'amount', type: 'number' },
      ],
      rows: [
        ['north', 'North', 'actual', 60],
        ['north', 'North', 'plan', 55],
      ],
    }
    const panel: Panel = {
      id: 'mix',
      kind: 'radial',
      title: 'Mix',
      semantics: 'partition',
      frame: 'chart:root',
      encoding: { id: 'id', label: 'label', series: 'ring', value: 'amount' },
      format: { amount: { kind: 'number', minorUnits: false, precision: 0 } },
      radial: {
        mode: 'partition',
        rings: [
          { key: 'actual', label: 'Actual', total: 60 },
          { key: 'plan', label: 'Plan', order: 1, total: 55 },
        ],
      },
      actions: [navigate('/mix/{ring}/{category}', [
        { name: 'ring', source: { kind: 'field', name: 'ring' } },
        { name: 'category', source: { kind: 'field', name: 'id' } },
      ])],
    }
    const { container, activate } = renderChart(panel, frame)

    await waitFor(() => expect(container.querySelector('[data-drillable]')).not.toBeNull())
    activate('radial:["plan","north"]')
    expect(assign).toHaveBeenCalledWith(expect.stringContaining('/mix/plan/north'))
  })
})

const cascadeFrame: Frame = {
  columns: [
    { name: 'label', type: 'string' },
    { name: 'value', type: 'number' },
    { name: 'cut', type: 'number' },
    { name: 'cutLabel', type: 'string' },
    { name: 'final', type: 'bool' },
    { name: 'action_url', type: 'string' },
  ],
  rows: [
    ['Earned premium', 100, 0, '', false, '/analytics/families/earned'],
    ['After claims', 60, 40, 'Claims paid', false, '/analytics/families/claims'],
    ['Result', 45, 15, 'Operating expenses', true, '/analytics/families/opex'],
  ],
}

function cascadePanel(actions: Action[], waterfall = false): Panel {
  return {
    id: 'bridge', kind: 'cascade', title: 'Result bridge', semantics: 'reconciliation', frame: 'cascade:root',
    encoding: { label: 'label', value: 'value', cut: 'cut', cutLabel: 'cutLabel', final: 'final' },
    format: { value: { kind: 'number', minorUnits: false, precision: 0 } },
    actions,
    ...(waterfall ? { presentation: { bridgeLayout: 'waterfall' as const } } : {}),
  }
}

describe('cascade stages with a row-scoped action', () => {
  const rowAction = (): Action => ({
    kind: 'navigate', method: 'GET', urlTemplate: '',
    urlSource: { kind: 'field', name: 'action_url' },
    params: [], payload: {},
  })

  it('activates the clicked waterfall column through its own row', () => {
    const assign = vi.mocked(navigateTo)
    const panel = cascadePanel([rowAction()], true)
    const { container } = renderPanel(
      documentWith([panel], { 'cascade:root': cascadeFrame }),
      <CascadePanel panel={panel} />,
    )

    // The whole column is the target, not the bar: a step worth a percent of
    // the opening total draws a few pixels tall.
    const columns = [...container.querySelectorAll<HTMLElement>('.lens-waterfall-column')]
    // start + 2 deltas + synthetic closing total.
    expect(columns).toHaveLength(4)
    expect(container.querySelector('.lens-waterfall-bar[role="button"]')).toBeNull()
    const stages = columns.filter((column) => column.getAttribute('role') === 'button')
    expect(stages).toHaveLength(3)
    stages[1]!.click()
    expect(assign).toHaveBeenCalledWith(expect.stringContaining('/analytics/families/claims'))
    // The synthetic closing total repeats the final stage and stays inert.
    expect(columns.at(-1)!.getAttribute('role')).toBeNull()
  })

  it('activates a stacked cascade stage and leaves url-less rows inert', () => {
    const assign = vi.mocked(navigateTo)
    const frame: Frame = { ...cascadeFrame, rows: cascadeFrame.rows.map((row, index) => index === 0 ? [...row.slice(0, 5), ''] : row) }
    const panel = cascadePanel([rowAction()])
    const { container } = renderPanel(
      documentWith([panel], { 'cascade:root': frame }),
      <CascadePanel panel={panel} />,
    )

    const stages = [...container.querySelectorAll<HTMLElement>('.lens-cascade-stage')]
    expect(stages).toHaveLength(3)
    expect(stages[0]!.getAttribute('role')).toBeNull()
    expect(stages[2]!.getAttribute('role')).toBe('button')
    stages[2]!.click()
    expect(assign).toHaveBeenCalledWith(expect.stringContaining('/analytics/families/opex'))
  })

  it('keeps an action-less cascade inert', () => {
    const panel = cascadePanel([], true)
    const { container } = renderPanel(
      documentWith([panel], { 'cascade:root': cascadeFrame }),
      <CascadePanel panel={panel} />,
    )
    expect(container.querySelector('.lens-waterfall')?.getAttribute('role')).toBe('img')
    expect(container.querySelector('.lens-waterfall-column[role="button"]')).toBeNull()
  })
})

describe('one panel, one click behaviour', () => {
  function renderTreeChart(panel: Panel, onMark?: (key: string) => void) {
    let select: ((key: string) => void) | undefined
    const adapter: ChartAdapter = {
      mount: (_element, _input, events) => {
        select = (key) => events.onSelect(key)
        return { update: () => {}, dispose: () => {} }
      },
    }
    const document = documentWith([panel], { 'chart:root': chartFrame })
    document.drill = {
      inlineDepth: 1,
      edges: {
        root: {
          id: 'root', label: 'Risk split', path: ['root'], frame: 'chart:root', perspectives: [],
          children: [
            { key: 'direct', label: 'Direct', path: ['root', 'direct'], target: 'detail' },
            { key: 'broker', label: 'Broker', path: ['root', 'broker'], target: 'detail' },
          ],
        },
        detail: {
          id: 'detail', label: 'Detail', path: ['root', 'direct'], frame: 'chart:root',
          perspectives: [], children: [],
        },
      },
    } as typeof document.drill
    const view = renderPanel(
      document,
      <MarkSelectionContext.Provider value={onMark}>
        <ChartPanel panel={panel} adapter={adapter} />
      </MarkSelectionContext.Provider>,
    )
    return { ...view, activate: (key: string) => select?.(key) }
  }

  it('never navigates from a mark when the panel owns a drill tree', async () => {
    const onMark = vi.fn()
    // The same panel carries a navigate action: the tree still wins, so the
    // click can only open the overlay.
    const panel: Panel = {
      ...chartPanel([navigate('/risk/{segment}', [{ name: 'segment', source: { kind: 'field', name: 'id' } }])]),
      drillRoot: 'root',
    }
    const { activate } = renderTreeChart(panel, onMark)

    await waitFor(() => expect(screen.getByLabelText(/chart/)).toBeInTheDocument())
    activate('broker')
    expect(onMark).toHaveBeenCalledWith('broker', undefined)
    expect(vi.mocked(navigateTo)).not.toHaveBeenCalled()
  })

  it('never opens an overlay from a mark when the panel has no tree', async () => {
    const onMark = vi.fn()
    const panel = chartPanel([navigate('/risk/{segment}', [
      { name: 'segment', source: { kind: 'field', name: 'id' } },
    ])])
    let select: ((key: string) => void) | undefined
    const adapter: ChartAdapter = {
      mount: (_element, _input, events) => {
        select = (key) => events.onSelect(key)
        return { update: () => {}, dispose: () => {} }
      },
    }
    renderPanel(
      documentWith([panel], { 'chart:root': chartFrame }),
      <MarkSelectionContext.Provider value={onMark}>
        <ChartPanel panel={panel} adapter={adapter} />
      </MarkSelectionContext.Provider>,
    )

    await waitFor(() => expect(screen.getByLabelText(/chart/)).toBeInTheDocument())
    select?.('broker')
    expect(onMark).not.toHaveBeenCalled()
    expect(vi.mocked(navigateTo)).toHaveBeenCalledWith(expect.stringContaining('/risk/broker'))
  })

  it('keeps a card link off a panel that owns a drill tree', () => {
    const panel: Panel = { ...statPanel([navigate('/metrics/loss')]), drillRoot: 'root' }
    const { container } = renderPanel(
      documentWith([panel], { 'stat:root': statFrame }),
      <StatPanel panel={panel} />,
    )

    expect(container.querySelector('.lens-card-link')).toBeNull()
  })
})

describe('chart tooltips', () => {
  it('renders at body level, confined to the viewport, above the expanded-panel dialog', async () => {
    const { tooltipChrome, tooltipZIndex, tooltipClassName } = await import('../charts/echarts/options')
    const chrome = tooltipChrome({
      card: '#fff', text: '#111', mutedText: '#666', border: '#eee', divider: '#eee',
      faintText: '#94a3b8', warn: '#d97706', warnSoft: '#fffbeb', accent: '#2563eb', trend: '#7c3aed',
      selectedBorder: '#000', fontFamily: 'Inter',
      popoverShadow: '0 1px 2px rgba(0,0,0,0.1)', cardRadius: 8, type: { xs: 10, sm: 11, base: 12, md: 14 },
      colors: [], seriesColor: () => undefined,
    })

    expect(chrome.appendTo).toBe('body')
    expect(chrome.confine).toBe(true)
    // The expanded-panel overlay sits at 2147483000.
    expect(tooltipZIndex).toBeGreaterThan(2147483000)
    expect(chrome.extraCssText).toContain(`z-index: ${tooltipZIndex}`)
    // Living outside the chart container, a tooltip needs a name of its own to
    // be findable — by a test, and by anyone chasing one that outlived its chart.
    expect(chrome.className).toBe(tooltipClassName)
  })
})

/**
 * A drill is per-segment or it is not a drill.
 *
 * The audit's blocker was a donut whose every arc opened the same whole-panel
 * drawer: the segment was discarded between the click and the URL, so a chart
 * that highlights each arc on hover answered every one of them identically.
 * The document now carries one drawer key per row; these guard the runtime half
 * — that the clicked mark, not the panel or its first row, is what resolves.
 */
describe('per-segment drawer drill', () => {
  const openDrawerPerRow: Action = {
    kind: 'open_drawer', method: 'POST',
    drawerKey: { kind: 'field', name: 'drawer_key' },
    params: [], payload: {},
  }
  const segmentFrame: Frame = {
    columns: [
      { name: 'id', type: 'string' },
      { name: 'label', type: 'string' },
      { name: 'drawer_key', type: 'string' },
      { name: 'amount', type: 'number' },
    ],
    rows: [
      ['direct', 'Direct', 'family:expenses?branch=direct', 600],
      ['broker', 'Broker', 'family:expenses?branch=broker', 400],
    ],
  }

  function renderDonut() {
    window.history.replaceState(null, '', '/')
    const panel: Panel = { ...chartPanel([openDrawerPerRow]), kind: 'donut', frame: 'chart:root' }
    const document = documentWith([panel], { 'chart:root': segmentFrame })
    document.endpoints = { drawer: '/lens/drawer' }
    const fetcher = vi.fn<typeof fetch>(() => Promise.resolve(
      new Response(JSON.stringify({ url: '/lens/document' }), { status: 200 }),
    ))
    let select: ((key: string) => void) | undefined
    let hover: ((key: string | null) => void) | undefined
    let input: ChartInput | undefined
    const adapter: ChartAdapter = {
      mount: (_element, initial, events) => {
        select = (key) => events.onSelect(key)
        hover = (key) => events.onHover(key)
        input = initial
        return { update: (next) => { input = next }, dispose: () => {} }
      },
    }
    const view = render(
      <div className="lens-root">
        <DocumentProvider initialDocument={document}>
          <DashboardRuntimeProvider fetcher={fetcher} locale="en">
            <ChartPanel adapter={adapter} panel={panel} />
          </DashboardRuntimeProvider>
        </DocumentProvider>
      </div>,
    )
    return {
      ...view,
      fetcher,
      activate: (key: string) => select?.(key),
      hover: (key: string | null) => hover?.(key),
      chartMounted: () => hover !== undefined,
      chartInput: () => input,
    }
  }

  it('asks for the clicked segment’s drawer, not the panel’s', async () => {
    const { container, fetcher, activate } = renderDonut()
    await waitFor(() => expect(container.querySelector('[data-drillable]')).not.toBeNull())
    // The resolver call, not the document fetch the opened drawer makes after it.
    const resolved = () => fetcher.mock.calls
      .filter(([url]) => url === '/lens/drawer')
      .map(([, init]) => typeof init?.body === 'string' ? init.body : '')

    activate('broker')
    await waitFor(() => expect(resolved()).toHaveLength(1))
    expect(resolved()[0]).toContain('family:expenses?branch=broker')

    activate('direct')
    await waitFor(() => expect(resolved()).toHaveLength(2))
    expect(resolved()[1]).toContain('family:expenses?branch=direct')
  })

  it('resolves the hovered segment into the shared drawer cache before click', async () => {
    const { chartMounted, container, fetcher, hover } = renderDonut()
    await waitFor(() => expect(container.querySelector('[data-drillable]')).not.toBeNull())
    await waitFor(() => expect(chartMounted()).toBe(true))
    const resolved = () => fetcher.mock.calls
      .filter(([url]) => url === '/lens/drawer')
      .map(([, init]) => typeof init?.body === 'string' ? init.body : '')

    hover('broker')
    await new Promise((resolve) => setTimeout(resolve, 100))
    expect(resolved()).toHaveLength(1)
    expect(resolved()[0]).toContain('family:expenses?branch=broker')
    hover(null)
  })

  it('warms every low-cardinality segment in the bounded idle queue', async () => {
    const { container, fetcher } = renderDonut()
    await waitFor(() => expect(container.querySelector('[data-drillable]')).not.toBeNull())
    const resolved = () => fetcher.mock.calls
      .filter(([url]) => url === '/lens/drawer')
      .map(([, init]) => typeof init?.body === 'string' ? init.body : '')

    await waitFor(() => expect(resolved()).toHaveLength(2), { timeout: 1_500 })
    expect(resolved()[0]).toContain('family:expenses?branch=direct')
    expect(resolved()[1]).toContain('family:expenses?branch=broker')
  })

  it('cancels a transient segment hover before it starts speculative work', async () => {
    const { chartMounted, container, fetcher, hover } = renderDonut()
    await waitFor(() => expect(container.querySelector('[data-drillable]')).not.toBeNull())
    await waitFor(() => expect(chartMounted()).toBe(true))

    hover('broker')
    hover(null)
    await new Promise((resolve) => setTimeout(resolve, 100))
    expect(fetcher.mock.calls.filter(([url]) => url === '/lens/drawer')).toHaveLength(0)
  })

  it('carries the drill hint on the tooltip rather than in a corner of the card', async () => {
    const { container, chartInput } = renderDonut()
    await waitFor(() => expect(container.querySelector('[data-drillable]')).not.toBeNull())

    // Under the pointer, on the card that already names the mark the click
    // would open — not a glyph parked in the corner furthest from it.
    expect(chartInput()?.actionHint).toBe('Select to explore')
    expect(container.querySelector('.lens-chart-drill-hint')).toBeNull()
  })
})

/**
 * The panel a cross-filter was taken from is deliberately excluded from it —
 * filtering the control by its own answer would delete the other options. That
 * is only defensible if the panel says so and marks the choice; otherwise a
 * full distribution beside filtered neighbours reads as a panel that failed.
 */
describe('cross-filter source panel', () => {
  const crossFilter: Action = {
    kind: 'cross_filter', method: 'GET', params: [], payload: {},
    filter: { dimension: 'segment', value: { kind: 'field', name: 'id' } },
  }

  function renderSource(search: string) {
    window.history.replaceState({}, '', `/sales${search}`)
    const panel: Panel = { ...chartPanel([crossFilter]), kind: 'hbar' }
    let input: ChartInput | undefined
    const adapter: ChartAdapter = {
      mount: (_element, initial) => {
        input = initial
        return { update: (next) => { input = next }, dispose: () => {} }
      },
    }
    const view = renderPanel(
      documentWith([panel], { 'chart:root': chartFrame }),
      <ChartPanel adapter={adapter} panel={panel} />,
    )
    return { ...view, chartInput: () => input }
  }

  afterEach(() => window.history.replaceState({}, '', '/'))

  it('marks the filtering mark and explains why the panel is unfiltered', async () => {
    const { container, chartInput } = renderSource('?_f=segment%3Abroker')
    await waitFor(() => expect(chartInput()).not.toBeUndefined())

    expect(chartInput()?.selectedKey).toBe('broker')
    expect(container.querySelector('.lens-chart-source-note')?.textContent)
      .toBe('All categories shown — the filter applies to the other panels')
  })

  it('says nothing when the page is filtered by another dimension', async () => {
    const { container, chartInput } = renderSource('?_f=region%3Anorth')
    await waitFor(() => expect(chartInput()).not.toBeUndefined())

    expect(chartInput()?.selectedKey).toBeUndefined()
    expect(container.querySelector('.lens-chart-source-note')).toBeNull()
    // The affordance still names what the click does on this class of panel —
    // now at the foot of the tooltip, where the pointer already is.
    expect(chartInput()?.actionHint).toBe('Select to filter the page')
  })
})

/** Regression: a mark's row is found by id, then by label. */
describe('mark identity', () => {
  it('resolves a row by id and by label', async () => {
    const { rowIndexForKey } = await import('./ChartPanel')
    const panel = chartPanel([])

    expect(rowIndexForKey(chartFrame, panel, 'broker')).toBe(1)
    expect(rowIndexForKey(chartFrame, panel, 'Direct')).toBe(0)
    expect(rowIndexForKey(chartFrame, panel, 'missing')).toBe(-1)
  })
})
