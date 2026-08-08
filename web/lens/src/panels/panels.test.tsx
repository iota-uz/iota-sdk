import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { DashboardDocument, Frame, Panel, PanelKind } from '../contract'
import type { PanelFrameState } from '../runtime'
import type { ChartAdapter, ChartInput } from '../charts/adapter'

const runtime = vi.hoisted(() => ({
  frame: undefined as PanelFrameState | undefined,
  drillInto: vi.fn(),
  document: { theme: { palette: {}, series: {} }, drill: { edges: {}, inlineDepth: 0 } } as DashboardDocument,
  navigation: { path: [], history: [] } as { panelId?: string; path: Array<string>; perspectiveId?: string; history: Array<unknown> },
  level: undefined as unknown,
  refreshing: false,
}))

vi.mock('../runtime', () => ({
  usePanelFrame: () => runtime.frame,
  useDocumentRefreshing: () => runtime.refreshing,
  usePanelPagination: () => ({ loadPage: vi.fn() }),
  useExport: () => ({ status: 'idle', available: false, run: vi.fn() }),
  useFormat: () => (value: unknown) => {
    if (value === null || value === undefined) return '—'
    if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') {
      return String(value)
    }
    return '—'
  },
  useFormatExact: () => () => undefined,
  useFormatAtReference: () => (value: unknown) => {
    if (value === null || value === undefined) return '—'
    if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') {
      return String(value)
    }
    return '—'
  },
  compactReference: () => undefined,
  useAxisFormat: () => (value: unknown) => String(value),
  axisUnit: () => '',
  clampedDeltaPercent: () => undefined,
  useTranslate: () => (_key: string, fallback: string, vars?: Record<string, string | number>) => (
    vars ? fallback.replace(/\{(\w+)\}/g, (match, name: string) => (name in vars ? String(vars[name]) : match)) : fallback
  ),
  useDrill: () => ({ drillInto: runtime.drillInto }),
  useDrawer: () => ({ depth: 0, open: vi.fn(), close: vi.fn() }),
  useFilters: () => ({ applyURL: vi.fn() }),
  useDashboard: () => ({ document: runtime.document, navigation: runtime.navigation }),
  levelForPath: () => runtime.level,
}))

import { legendLabels, legendRow, legendSwatches, legendValues, markRow } from './legend.test-utils'
import { BarPanel, collapseExpandableTopN, collapseMinorDonutSlices, DistributionPanel, donutRemainderKey, LinePanel, PiePanel, rowIndexForKey } from './ChartPanel'
import { CoveragePanel } from './CoveragePanel'
import { GaugePanel } from './GaugePanel'
import { MapPanel } from './MapPanel'
import { buildCascadeStages, buildWaterfallItems, buildWaterfallModel, CascadePanel, waterfallAxisStep } from './CascadePanel'
import { WaterfallPlot } from './WaterfallPlot'
import { panelRegistry, RegisteredPanel } from './registry'
import { StatMetric, StatPanel } from './StatPanel'
import { TablePanel } from './TablePanel'

const dataFrame: Frame = {
  columns: [
    { name: 'id', type: 'string' },
    { name: 'label', type: 'string' },
    { name: 'category', type: 'time' },
    { name: 'series', type: 'string' },
    { name: 'value', type: 'number' },
  ],
  rows: [['root/a', 'Alpha', '2026-07-01T00:00:00Z', 'Actual', 42]],
}

function panel(kind: PanelKind, overrides: Partial<Panel> = {}): Panel {
  return {
    id: `panel-${kind}`,
    kind,
    title: `${kind} panel`,
    semantics: kind === 'pie' || kind === 'donut' ? 'partition' : 'series',
    frame: `frame-${kind}`,
    encoding: { id: 'id', label: 'label', category: 'category', series: 'series', value: 'value' },
    format: {},
    actions: [],
    ...(kind === 'gauge' ? { radial: { mode: 'progress' as const, max: 100 } } : {}),
    ...(kind === 'map' ? {
      map: {
        source: { inline: {
          type: 'FeatureCollection',
          features: [{
            type: 'Feature', properties: { code: 'root/a', name: 'Alpha' },
            geometry: { type: 'Polygon', coordinates: [[[0, 0], [1, 0], [1, 1], [0, 0]]] },
          }],
        } },
        featureProperty: 'code', labelProperty: 'name',
      },
    } : {}),
    ...overrides,
  } as Panel
}

function state(name: 'loading' | 'empty' | 'error' | 'stale' | 'superseded' | 'data'): PanelFrameState {
  const retry = vi.fn()
  if (name === 'loading') return { isLoading: true, isStale: false, error: null, retry }
  if (name === 'empty') return { data: { ...dataFrame, rows: [] }, isLoading: false, isStale: false, error: null, retry }
  if (name === 'error') return { isLoading: false, isStale: false, error: new Error('Frame failed'), retry }
  if (name === 'stale') return { data: dataFrame, isLoading: true, isStale: true, error: null, retry }
  // What a filter change produces across the whole board: the previous
  // period's data still drawn, dimmed and marked, with no panel-local request
  // of its own in flight.
  if (name === 'superseded') return { data: dataFrame, isLoading: false, isStale: true, error: null, retry }
  return { data: dataFrame, isLoading: false, isStale: false, error: null, retry }
}

function fakeAdapter(capture?: (input: ChartInput) => void): ChartAdapter {
  return {
    mount(el, input, events) {
      capture?.(input)
      const button = document.createElement('button')
      button.textContent = 'chart data'
      button.onclick = () => events.onSelect('root/a')
      button.onpointerenter = () => events.onHover('root/a')
      button.onpointerleave = () => events.onHover(null)
      el.append(button)
      return { update: capture ?? (() => undefined), dispose: () => el.replaceChildren() }
    },
  }
}

function renderKind(kind: PanelKind) {
  const value = panel(kind)
  if (kind === 'stat') return render(<StatPanel panel={value} />)
  if (kind === 'cascade') return render(<CascadePanel panel={value} />)
  if (kind === 'table') return render(<TablePanel panel={value} />)
  if (kind === 'coverage') return render(<CoveragePanel panel={value} />)
  if (kind === 'gauge') return render(<GaugePanel panel={value} />)
  if (kind === 'map') return render(<MapPanel panel={value} adapter={fakeAdapter()} />)
  if (kind === 'histogram' || kind === 'boxplot' || kind === 'heatmap') return render(<DistributionPanel panel={value} adapter={fakeAdapter()} />)
  if (kind === 'pie' || kind === 'donut' || kind === 'radial') return render(<PiePanel panel={value} adapter={fakeAdapter()} />)
  if (kind === 'bar' || kind === 'hbar') return render(<BarPanel panel={value} adapter={fakeAdapter()} />)
  return render(<LinePanel panel={value} adapter={fakeAdapter()} />)
}

const baseDocument = runtime.document

afterEach(() => {
  cleanup()
  runtime.drillInto.mockReset()
  runtime.navigation = { path: [], history: [] }
  runtime.level = undefined
  runtime.refreshing = false
  runtime.document = baseDocument
  window.history.replaceState({}, '', '/')
})

const panelKinds: PanelKind[] = ['stat', 'pie', 'donut', 'radial', 'bar', 'hbar', 'line', 'area', 'gauge', 'histogram', 'boxplot', 'heatmap', 'map', 'cascade', 'coverage', 'table']

describe.each<PanelKind>(panelKinds)('%s panel states', (kind) => {
  it.each(['loading', 'empty', 'error', 'stale', 'superseded', 'data'] as const)('renders %s', async (stateName) => {
    runtime.frame = state(stateName)
    const view = renderKind(kind)
    const panelElement = screen.getByLabelText(`${kind} panel`)

    if (stateName === 'loading') {
      // The placeholder mirrors the panel shape and is hidden from assistive
      // technology; aria-busy on the panel carries the state instead.
      expect(panelElement).toHaveAttribute('aria-busy', 'true')
      expect(view.container.querySelector('.lens-panel-skeleton')).not.toBeNull()
    }
    if (stateName === 'empty') {
      expect(screen.getByText('No data')).toBeInTheDocument()
      if (['pie', 'donut', 'radial', 'bar', 'hbar', 'line', 'area'].includes(kind)) {
        expect(view.container.querySelector('.lens-chart-compact')).not.toBeNull()
      }
    }
    if (stateName === 'error') {
      fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
      expect(runtime.frame.retry).toHaveBeenCalledTimes(1)
    }
    if (stateName === 'stale') {
      // A refetch over existing data reuses the initial-load skeleton rather
      // than dimming the stale content behind an "Updating" chip.
      expect(panelElement).toHaveAttribute('data-stale', 'true')
      expect(panelElement).toHaveAttribute('aria-busy', 'true')
      expect(view.container.querySelector('.lens-panel-skeleton')).not.toBeNull()
      expect(screen.queryByText('Updating')).toBeNull()
    }
    if (stateName === 'superseded') {
      // Dimmed data being replaced is as busy as an empty skeleton is, and it
      // says so where a reader and a screen reader can both find it. The data
      // itself stays: losing every figure for the seconds a period change takes
      // costs more than it protects.
      expect(panelElement).toHaveAttribute('data-stale', 'true')
      expect(panelElement).toHaveAttribute('aria-busy', 'true')
      // A map is loading in its own right until its geometry arrives, so it
      // keeps the skeleton; every other kind dims the data it already has.
      if (kind !== 'map') {
        expect(view.container.querySelector('.lens-panel-skeleton')).toBeNull()
        expect(screen.getByText('Updating')).toBeInTheDocument()
      }
    }
    if (stateName === 'data') {
      if (kind === 'stat') expect(screen.getByText('42')).toBeInTheDocument()
      else if (kind === 'cascade') expect(screen.getByRole('list', { name: 'cascade panel stages' })).toBeInTheDocument()
      else if (kind === 'table') expect(screen.getByRole('table')).toBeInTheDocument()
      else if (kind === 'coverage') expect(panelElement.querySelector('.lens-coverage-headline')).not.toBeNull()
      else if (kind === 'gauge') expect(screen.getByRole('meter')).toHaveAttribute('aria-valuenow', '42')
      else if (kind === 'histogram' || kind === 'boxplot' || kind === 'heatmap') await screen.findByRole('button', { name: 'chart data' })
      else if (kind === 'map') await screen.findByRole('button', { name: 'chart data' })
      else expect(panelElement.querySelector('.lens-chart-compact')).toHaveTextContent('42')
    }
    view.unmount()
  })
})

/**
 * A native `title` is this runtime's answer to text the layout clips — the full
 * name behind a two-line clamp, an ellipsized label — and never a second copy of
 * a figure the panel already draws. It is a poor data channel: it waits about a
 * second, ignores the theme, cannot be styled or reached by touch, and it is the
 * one surface here the sheet has no say over.
 *
 * Where it sits is what separates the two uses. A `title` inside a subtree the
 * panel itself marks `aria-hidden` cannot be naming that element's own clipped
 * text — decoration has no text — and a screen reader never receives it either,
 * so whatever it says is said to nobody twice over. Coverage had three, printing
 * «label: amount» over a track whose legend prints the label, the amount and the
 * share one row below it.
 */
describe('native tooltips', () => {
  // The metric family — flow, hierarchy, relationship — answers the same sweep
  // beside its own fixtures in metricPanels.test.tsx. Those kinds are absent
  // from `panelKinds` because this file's harness cannot draw them: `renderKind`
  // has no case for them and would quietly hand a LinePanel a metric document.
  it.each(panelKinds)('do not hide inside %s decoration', (kind) => {
    runtime.frame = state('data')
    const view = renderKind(kind)
    // The hidden element counts as well as its descendants: a `title` on the
    // coverage track itself is the same tooltip said to the same nobody.
    const hidden = [...view.container.querySelectorAll('[aria-hidden="true"][title], [aria-hidden="true"] [title]')]
      .map((element) => `${element.className || element.tagName}: ${element.getAttribute('title')}`)
    expect(hidden, 'put the text where it can be read, or leave it to the legend').toEqual([])
    view.unmount()
  })
})

describe('map panel', () => {
  it('passes the exact feature join to ECharts and drills with the region key', async () => {
    runtime.frame = state('data')
    const inputs: ChartInput[] = []
    render(<MapPanel panel={panel('map', { drillRoot: 'root' })} adapter={fakeAdapter((input) => inputs.push(input))} />)

    const mark = await screen.findByRole('button', { name: 'chart data' })
    expect(inputs.at(-1)?.kind).toBe('map')
    expect(inputs.at(-1)?.map).toMatchObject({ featureProperty: 'code', labelProperty: 'name' })
    fireEvent.click(mark)
    expect(runtime.drillInto).toHaveBeenCalledWith('root/a', 'panel-map')
  })
})

describe('panel total badge', () => {
  it('renders the formatted total in the header when panel.total is present', () => {
    runtime.frame = state('data')
    const view = render(<BarPanel panel={panel('bar', { total: 12345 })} adapter={fakeAdapter()} />)
    const badge = view.container.querySelector('.lens-panel-total')
    // The badge names what it totals instead of showing an unlabeled number.
    expect(badge).toHaveTextContent('Total: 12345')
  })

  it('omits the badge when panel.total is absent', () => {
    runtime.frame = state('data')
    const view = render(<BarPanel panel={panel('bar')} adapter={fakeAdapter()} />)
    expect(view.container.querySelector('.lens-panel-total')).toBeNull()
  })

  it('uses the hydrated frame total instead of a deferred shell placeholder', () => {
    runtime.frame = {
      data: { ...dataFrame, total: 42 },
      isLoading: false,
      isStale: false,
      error: null,
      retry: vi.fn(),
    }
    const view = render(<PiePanel
      adapter={fakeAdapter()}
      panel={panel('pie', { presentation: { totalBadge: 'plot' }, total: 3 })}
    />)

    expect(view.container.querySelector('.lens-plot-total')).toHaveTextContent('Total: 42')
    expect(view.container.querySelector('.lens-plot-total')).not.toHaveTextContent('Total: 3')
  })

  it('leaves the plot total to the donut hub, which already prints it', () => {
    runtime.frame = {
      data: { ...dataFrame, total: 42 },
      isLoading: false,
      isStale: false,
      error: null,
      retry: vi.fn(),
    }
    const view = render(<PiePanel
      adapter={fakeAdapter()}
      panel={panel('donut', { presentation: { totalBadge: 'plot' }, total: 42 })}
    />)

    expect(view.container.querySelector('.lens-plot-total')).toBeNull()
  })

  it('does not leak a deferred shell total into an empty hydrated frame', () => {
    runtime.frame = state('empty')
    const view = render(<PiePanel
      adapter={fakeAdapter()}
      panel={panel('donut', { presentation: { totalBadge: 'plot' }, total: 3 })}
    />)

    expect(view.container.querySelector('.lens-plot-total')).toBeNull()
    expect(view.container).not.toHaveTextContent('Total: 3')
  })

  it('totals the drilled level frame, not the root, in the header badge', () => {
    // The panel is showing a drill level: navigation targets this panel with a
    // non-empty path, and the frame on screen is the level's rows. `panel.total`
    // is still the root frame's figure (723) — the badge must print the level
    // total (30 + 70 = 100), the same base the slice percentages normalize to.
    const levelFrame: Frame = {
      columns: [
        { name: 'id', type: 'string' },
        { name: 'label', type: 'string' },
        { name: 'value', type: 'number' },
      ],
      rows: [
        ['root/a', 'Alpha', 30],
        ['root/b', 'Beta', 70],
      ],
    }
    runtime.frame = { data: levelFrame, isLoading: false, isStale: false, error: null, retry: vi.fn() }
    runtime.navigation = { panelId: 'panel-bar', path: ['root'], history: [] }
    const view = render(<BarPanel panel={panel('bar', { total: 723 })} adapter={fakeAdapter()} />)
    const badge = view.container.querySelector('.lens-panel-total')
    expect(badge).toHaveTextContent('Total: 100')
    expect(badge).not.toHaveTextContent('723')
  })
})

describe('gauge panel', () => {
  it('renders a formatted reading against the declared non-percent range', () => {
    runtime.frame = {
      data: { columns: [{ name: 'value', type: 'number' }], rows: [[125]] },
      isLoading: false, isStale: false, error: null, retry: vi.fn(),
    }
    const view = render(<GaugePanel panel={panel('gauge', { encoding: { value: 'value' }, radial: { mode: 'progress', max: 250 } })} />)
    expect(screen.getByRole('meter')).toHaveAttribute('aria-valuenow', '125')
    expect(screen.getByRole('meter')).toHaveAttribute('aria-valuemax', '250')
    expect(view.container.querySelector('.lens-gauge-reading')).toHaveTextContent('125of 250')
    expect(view.container.querySelector('.lens-gauge-value-arc')).toHaveStyle({ strokeDasharray: '50 100' })
  })
})

describe('temporal overlay controls', () => {
  it('keeps regression off by default and selects one documented moving-average window', async () => {
    runtime.frame = {
      ...state('data'),
      data: {
        columns: [
          ...dataFrame.columns,
          { name: 'regression', type: 'number' },
          { name: 'sma_3', type: 'number' },
          { name: 'sma_7', type: 'number' },
        ],
        rows: [
          ['root/a', 'Alpha', '2026-07-01T00:00:00Z', 'Actual', 42, 40, null, null],
          ['root/b', 'Beta', '2026-07-02T00:00:00Z', 'Actual', 45, 43, 43, null],
          ['root/c', 'Gamma', '2026-07-03T00:00:00Z', 'Actual', 48, 46, 45, 44],
        ],
      },
    }
    const inputs: ChartInput[] = []
    const temporal = panel('line', {
      temporal: {
        regression: { field: 'regression', label: 'Trend' },
        movingAverages: [
          { window: 3, field: 'sma_3', label: 'SMA 3' },
          { window: 7, field: 'sma_7', label: 'SMA 7' },
        ],
        referenceLines: [{ value: 50, label: 'Threshold' }],
      },
    })
    const view = render(<LinePanel panel={temporal} adapter={fakeAdapter((input) => inputs.push(input))} />)
    await waitFor(() => expect(inputs.length).toBeGreaterThan(0))
    // The header control and the legend entry are two switches on one state,
    // so the header one is addressed through its own group.
    const controls = within(view.container.querySelector<HTMLElement>('.lens-temporal-controls')!)
    // The panel hands the chart every overlay the document declared and says
    // separately which of them the reader has switched off, so the legend can
    // list an overlay that is not currently drawn and offer it back.
    expect(inputs.at(-1)?.temporal).toMatchObject({ referenceLines: [{ value: 50, label: 'Threshold' }] })
    expect(inputs.at(-1)?.temporal?.regression?.field).toBe('regression')
    expect([...(inputs.at(-1)?.hiddenOverlays ?? [])].sort()).toEqual(['average:3', 'average:7', 'trend'])

    fireEvent.click(controls.getByRole('button', { name: 'Trend' }))
    await waitFor(() => expect(inputs.at(-1)?.hiddenOverlays?.has('trend')).toBe(false))
    expect(controls.getByRole('button', { name: 'Trend' })).toHaveAttribute('aria-pressed', 'true')

    fireEvent.change(controls.getByRole('combobox', { name: 'Moving average' }), { target: { value: '7' } })
    await waitFor(() => expect([...(inputs.at(-1)?.hiddenOverlays ?? [])].sort()).toEqual(['average:3']))
  })

  it('lists every overlay in the legend, including the ones no data row can name', async () => {
    runtime.frame = {
      ...state('data'),
      data: {
        columns: [...dataFrame.columns, { name: 'regression', type: 'number' }],
        rows: [
          ['root/a', 'Alpha', '2026-07-01T00:00:00Z', 'Actual', 42, 40],
          ['root/b', 'Beta', '2026-07-02T00:00:00Z', 'Actual', 45, 43],
        ],
      },
    }
    const temporal = panel('line', {
      temporal: {
        regression: { field: 'regression', label: 'Trend' },
        referenceLines: [{ value: 50, label: 'Threshold' }],
        annotations: [{ at: '2026-07-02T00:00:00Z', label: 'Method changed' }],
      },
    })
    const view = render(<LinePanel panel={temporal} adapter={fakeAdapter()} />)

    await screen.findByRole('list', { name: 'Chart marks' })
    expect(markRow(view.container, 'Threshold')).toHaveAttribute('aria-pressed', 'true')
    expect(markRow(view.container, 'Method changed')).toHaveAttribute('aria-pressed', 'true')
    // The trend is off until the header control turns it on, and the legend
    // says so rather than omitting it.
    expect(markRow(view.container, 'Trend')).toHaveAttribute('aria-pressed', 'false')

    fireEvent.click(markRow(view.container, 'Threshold'))
    await waitFor(() => expect(markRow(view.container, 'Threshold')).toHaveAttribute('aria-pressed', 'false'))
  })
})

describe('logarithmic scale warning', () => {
  it('keeps the non-linear scale visible on the chart reading surface', () => {
    runtime.frame = {
      ...state('data'),
      data: { ...dataFrame, rows: [
        ['a', 'A', 'A', 'Actual', 2],
        ['b', 'B', 'B', 'Actual', 100],
        ['c', 'C', 'C', 'Actual', 10_000],
      ] },
    }
    render(
      <BarPanel
        panel={panel('bar', { valueAxis: { scale: 'logarithmic', logBase: 10 } })}
        adapter={fakeAdapter()}
      />,
    )

    const warning = screen.getByRole('note', { name: 'Values are shown on a logarithmic scale' })
    expect(warning).toHaveTextContent('Logarithmic scale')
    expect(warning).toHaveAttribute('title', 'Values are shown on a logarithmic scale')
  })

  it('does not warn for an ordinary linear chart', () => {
    runtime.frame = state('data')
    render(<BarPanel panel={panel('bar')} adapter={fakeAdapter()} />)

    expect(screen.queryByRole('note')).toBeNull()
  })

  it('recomputes the warning from the same visible frame sent to the axis', async () => {
    runtime.frame = {
      ...state('data'),
      data: { ...dataFrame, rows: [
        ['a', 'A', 'A', 'Small', 2],
        ['b', 'B', 'B', 'Middle', 100],
        ['c', 'C', 'C', 'Outlier', 10_000],
      ] },
    }
    const inputs: ChartInput[] = []
    const view = render(<BarPanel panel={panel('bar', {
      valueAxis: { scale: 'logarithmic', logBase: 10 }, presentation: { legend: 'below' },
    })} adapter={fakeAdapter((input) => inputs.push(input))} />)

    expect(screen.getByRole('note')).toBeInTheDocument()
    fireEvent.click(legendRow(view.container, 'Outlier'))
    await waitFor(() => expect(inputs.at(-1)?.frame.rows).toHaveLength(2))
    expect(screen.queryByRole('note')).toBeNull()
  })

  it('silently falls back to linear for one category', () => {
    runtime.frame = state('data')
    render(<BarPanel panel={panel('bar', { valueAxis: { scale: 'logarithmic', logBase: 10 } })} adapter={fakeAdapter()} />)
    expect(screen.queryByRole('note')).toBeNull()
    expect(screen.getByText('42')).toBeInTheDocument()
  })
})

describe('background document refetch isolation', () => {
  it('keeps a usable panel visible while the document refreshes', () => {
    runtime.frame = state('data')
    runtime.refreshing = true
    const view = render(<BarPanel panel={panel('bar')} adapter={fakeAdapter()} />)
    const panelElement = screen.getByLabelText('bar panel')
    expect(panelElement).toHaveAttribute('aria-busy', 'false')
    expect(view.container.querySelector('.lens-panel-skeleton')).toBeNull()
    expect(screen.getByText('42')).toBeInTheDocument()
  })
})

describe('panel registry', () => {
  it('partitions every contract panel kind into supported or explicitly unsupported', () => {
    const contractKinds = {
      area: true,
      bar: true,
      cascade: true,
      coverage: true,
      donut: true,
      gauge: true,
      histogram: true,
      boxplot: true,
      heatmap: true,
      map: true,
      radial: true,
      hbar: true,
      line: true,
      metric_flow: true,
      metric_hierarchy: true,
      metric_relationship: true,
      pie: true,
      stat: true,
      table: true,
    } satisfies Record<PanelKind, true>

    for (const kind of Object.keys(contractKinds) as PanelKind[]) {
      expect(panelRegistry[kind], kind).toBeDefined()
    }
  })

  it('maps every kind and preserves an explicit fallback for custom registries', async () => {
    runtime.frame = state('data')
    const view = render(<RegisteredPanel panel={panel('table')} />)
    expect(await screen.findByRole('table')).toBeInTheDocument()
    view.rerender(<RegisteredPanel panel={panel('cascade')} registry={{}} />)
    expect(screen.getByText('Unsupported panel: cascade')).toBeInTheDocument()
  })

  it('lets a failed custom panel retry without taking down its siblings', () => {
    runtime.frame = state('data')
    let fail = true
    const Fragile = () => {
      if (fail) throw new Error('private renderer detail')
      return <div>Recovered panel</div>
    }
    render(<RegisteredPanel panel={panel('stat')} registry={{ stat: Fragile }} />)

    expect(screen.getByRole('alert')).toHaveTextContent('This panel could not be rendered.')
    expect(screen.getByRole('alert')).not.toHaveTextContent('private renderer detail')
    fail = false
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(screen.getByText('Recovered panel')).toBeInTheDocument()
  })
})

describe('chart encoding and drill behavior', () => {
  it('discloses when a panel cannot honor the active comparison', () => {
    runtime.frame = state('data')
    render(<StatPanel panel={panel('stat', { comparisonUnsupported: true })} />)

    expect(screen.getByRole('note')).toHaveTextContent('Comparison is not available for this panel.')
  })

  it('renders the server-localized map attribution as an accessible note', async () => {
    runtime.frame = state('data')
    const map = panel('map')
    map.map = { ...map.map!, attribution: '© Example contributors · Boundaries (ODbL)' }
    render(<MapPanel panel={map} adapter={fakeAdapter()} />)

    expect(await screen.findByRole('note', { name: 'Map attribution' })).toHaveTextContent('© Example contributors · Boundaries (ODbL)')
  })

  it('offers zoom reset only for a declared time category and calls the adapter', async () => {
    const resetZoom = vi.fn()
    const mount = vi.fn(() => ({ update: vi.fn(), dispose: vi.fn(), resetZoom }))
    const adapter: ChartAdapter = {
      mount,
    }
    const temporalFrame = { ...dataFrame, rows: [...dataFrame.rows, ['root/b', 'Beta', '2026-07-02T00:00:00Z', 'Actual', 43]] }
    runtime.frame = { ...state('data'), data: temporalFrame }
    const view = render(<LinePanel panel={panel('line')} adapter={adapter} />)
    await waitFor(() => expect(mount).toHaveBeenCalledTimes(1))
    fireEvent.click(await screen.findByRole('button', { name: 'Reset zoom' }))
    await waitFor(() => expect(resetZoom).toHaveBeenCalledTimes(1))

    runtime.frame = {
      ...state('data'),
      data: { ...temporalFrame, columns: temporalFrame.columns.map((column) => column.name === 'category' ? { ...column, type: 'string' as const } : column) },
    }
    view.rerender(<LinePanel panel={panel('line')} adapter={adapter} />)
    expect(screen.queryByRole('button', { name: 'Reset zoom' })).toBeNull()
  })

  it('falls back to the label encoding when the preferred category column is absent', async () => {
    const frame: Frame = {
      columns: [{ name: 'label', type: 'string' }, { name: 'value', type: 'number' }],
      rows: [['Alpha', 10], ['Beta', 20]],
    }
    runtime.frame = { data: frame, isLoading: false, isStale: false, error: null, retry: vi.fn() }
    const inputs: ChartInput[] = []
    render(<BarPanel
      adapter={fakeAdapter((input) => inputs.push(input))}
      panel={panel('hbar', { encoding: { category: 'missing', label: 'label', value: 'value' } })}
    />)

    await waitFor(() => expect(inputs).toHaveLength(1))
    expect(document.querySelector('.lens-chart-compact')).toBeNull()
  })

  it.each(['histogram', 'boxplot', 'heatmap'] as const)('passes %s through the chart runtime', async (kind) => {
    runtime.frame = state('data')
    const inputs: ChartInput[] = []
    render(<DistributionPanel panel={panel(kind)} adapter={fakeAdapter((input) => inputs.push(input))} />)
    await waitFor(() => expect(inputs.at(-1)?.kind).toBe(kind))
  })

  it('adds selection and hover affordances only when DrillRoot is present', async () => {
    runtime.frame = {
      ...state('data'),
      data: { ...dataFrame, rows: [...dataFrame.rows, ['root/b', 'Beta', '2026-07-02T00:00:00Z', 'Actual', 21]] },
    }
    const adapter = fakeAdapter()
    const view = render(<PiePanel panel={panel('pie')} adapter={adapter} />)
    await waitFor(() => expect(screen.getByText('chart data')).toBeInTheDocument())
    expect(screen.getByLabelText('pie panel chart')).not.toHaveAttribute('data-drillable')
    fireEvent.click(screen.getByText('chart data'))
    expect(runtime.drillInto).not.toHaveBeenCalled()

    view.rerender(<PiePanel panel={panel('pie', { drillRoot: 'root' })} adapter={adapter} />)
    expect(screen.getByLabelText('pie panel chart')).toHaveAttribute('data-drillable', 'true')
    fireEvent.click(screen.getByText('chart data'))
    expect(runtime.drillInto).toHaveBeenCalledWith('root/a', 'panel-pie')
  })

  it('keeps a one-category drill panel compact and clickable', () => {
    runtime.frame = state('data')
    render(<BarPanel panel={panel('bar', { drillRoot: 'root' })} adapter={fakeAdapter()} />)

    fireEvent.click(screen.getByRole('button', { name: /2026-07-01T00:00:00Z \/ 42.*Open/ }))
    expect(runtime.drillInto).toHaveBeenCalledWith('root/a', 'panel-bar')
    expect(document.querySelector('.lens-chart-compact')).not.toBeNull()
  })

  it('passes time and series encodings through and tolerates missing optional roles', async () => {
    runtime.frame = {
      ...state('data'),
      data: { ...dataFrame, rows: [...dataFrame.rows, ['root/b', 'Beta', '2026-07-02T00:00:00Z', 'Plan', 21]] },
    }
    const inputs: ChartInput[] = []
    const timePanel = panel('line', { encoding: { category: 'category', value: 'value', series: 'series' } })
    const view = render(<LinePanel panel={timePanel} adapter={fakeAdapter((input) => inputs.push(input))} />)
    await waitFor(() => expect(inputs.length).toBeGreaterThan(0))
    expect(inputs[0]?.frame.columns.find((column) => column.name === 'category')?.type).toBe('time')
    expect(inputs[0]?.encoding.series).toBe('series')

    const sparse = panel('bar', { encoding: { value: 'value' } })
    view.rerender(<BarPanel panel={sparse} adapter={fakeAdapter((input) => inputs.push(input))} />)
    // With no categorical role this is the intentional compact-value state;
    // the missing optional encodings are therefore tolerated without asking
    // the chart adapter to invent an axis.
    expect(screen.getByText('63')).toBeInTheDocument()
  })

  it('passes radial geometry and resolves a ring-specific row selection', async () => {
    const frame: Frame = {
      columns: dataFrame.columns,
      rows: [
        ['north', 'North', '2026-07-01T00:00:00Z', 'actual', 60],
        ['north', 'North', '2026-07-01T00:00:00Z', 'plan', 55],
      ],
    }
    runtime.frame = { data: frame, isLoading: false, isStale: false, error: null, retry: vi.fn() }
    const inputs: ChartInput[] = []
    const radial = panel('radial', {
      semantics: 'partition',
      radial: {
        mode: 'partition',
        rings: [
          { key: 'actual', label: 'Actual', total: 60 },
          { key: 'plan', label: 'Plan', order: 1, total: 55 },
        ],
      },
    })
    const adapter: ChartAdapter = {
      mount(el, input, events) {
        inputs.push(input)
        const button = document.createElement('button')
        button.textContent = 'radial mark'
        button.onclick = () => events.onSelect('radial:["plan","north"]')
        el.append(button)
        return { update: () => undefined, dispose: () => undefined }
      },
    }
    render(<PiePanel panel={radial} adapter={adapter} />)
    await waitFor(() => expect(inputs[0]?.radial?.mode).toBe('partition'))
    expect(rowIndexForKey(frame, radial, 'radial:["plan","north"]')).toBe(1)
    expect(rowIndexForKey(frame, radial, 'radial:["actual","north"]')).toBe(0)
  })

  it('reconciles legend shares inside each ring when two rings declare the same total', async () => {
    const third = 100 / 3
    const frame: Frame = {
      columns: dataFrame.columns,
      rows: [
        ['alpha', 'Alpha', '2026-07-01T00:00:00Z', 'actual', third],
        ['beta', 'Beta', '2026-07-01T00:00:00Z', 'actual', third],
        ['gamma', 'Gamma', '2026-07-01T00:00:00Z', 'actual', third],
        ['delta', 'Delta', '2026-07-01T00:00:00Z', 'plan', third],
        ['epsilon', 'Epsilon', '2026-07-01T00:00:00Z', 'plan', third],
        ['zeta', 'Zeta', '2026-07-01T00:00:00Z', 'plan', third],
      ],
    }
    runtime.frame = { data: frame, isLoading: false, isStale: false, error: null, retry: vi.fn() }
    const radial = panel('radial', {
      semantics: 'partition',
      presentation: { legend: 'below', legendValue: 'percent' },
      radial: {
        mode: 'partition',
        rings: [
          { key: 'actual', label: 'Actual', total: 100 },
          { key: 'plan', label: 'Plan', order: 1, total: 100 },
        ],
      },
    })
    const view = render(<PiePanel panel={radial} adapter={fakeAdapter()} />)
    await waitFor(() => expect(legendLabels(view.container)).toHaveLength(6))
    // Both rings declare 100. Grouped by that denominator the six rows sum to
    // 200%, so nothing reconciles and each ring prints 99.9%. Grouped by ring,
    // each thirds-decomposition adds up to exactly 100.0%.
    expect(legendValues(view.container)).toEqual(['33.4%', '33.3%', '33.3%', '33.4%', '33.3%', '33.3%'])
  })

  it('drops the colour pins of the rows a legend toggle hid', async () => {
    const frame: Frame = {
      columns: dataFrame.columns,
      rows: [
        ['alpha', 'Alpha', '2026-07-01T00:00:00Z', 'actual', 60],
        ['beta', 'Beta', '2026-07-01T00:00:00Z', 'actual', 40],
      ],
      colors: ['#111111', '#222222'],
    }
    runtime.frame = { data: frame, isLoading: false, isStale: false, error: null, retry: vi.fn() }
    const inputs: ChartInput[] = []
    const pie = panel('pie', { presentation: { legend: 'below' } })
    const view = render(<PiePanel panel={pie} adapter={fakeAdapter((input) => inputs.push(input))} />)
    await waitFor(() => expect(inputs.length).toBeGreaterThan(0))
    fireEvent.click(legendRow(view.container, 'Alpha'))
    // The pins are positional: keeping Alpha's after dropping Alpha's row
    // would paint Beta's slice with Alpha's colour.
    await waitFor(() => expect(inputs.at(-1)?.frame.rows).toHaveLength(1))
    expect(inputs.at(-1)?.rowColor?.('Beta', 0, 'beta')).toBe('#222222')
  })

  it('prints a legend swatch in the colour its slice is drawn with', async () => {
    const frame: Frame = {
      columns: dataFrame.columns,
      rows: [
        ['alpha', 'Alpha', '2026-07-01T00:00:00Z', 'actual', 60],
        ['beta', 'Beta', '2026-07-01T00:00:00Z', 'actual', 40],
      ],
      // A served level ships its own palette. The plot has always read it; the
      // legend used to fall through to the theme ramp and print a colour that
      // appeared nowhere on the chart.
      colors: ['#111111', '#222222'],
    }
    runtime.frame = { data: frame, isLoading: false, isStale: false, error: null, retry: vi.fn() }
    const inputs: ChartInput[] = []
    const pie = panel('pie', { presentation: { legend: 'below' } })
    const view = render(<PiePanel panel={pie} adapter={fakeAdapter((input) => inputs.push(input))} />)
    await waitFor(() => expect(legendSwatches(view.container)).toHaveLength(2))
    expect(legendSwatches(view.container)).toEqual(['rgb(17, 17, 17)', 'rgb(34, 34, 34)'])
    expect(inputs.at(-1)?.rowColor?.('Alpha', 0, 'alpha')).toBe('#111111')
    expect(inputs.at(-1)?.rowColor?.('Beta', 1, 'beta')).toBe('#222222')
  })

  it('renders one legend entry per line series and toggles the whole series', async () => {
    const frame: Frame = {
      columns: [
        { name: 'category', type: 'string' },
        { name: 'series', type: 'string' },
        { name: 'value', type: 'number' },
      ],
      rows: [
        ['2025', 'Written premium', 100],
        ['2025', 'Earned premium', 80],
        ['2026', 'Written premium', 120],
        ['2026', 'Earned premium', 90],
      ],
    }
    runtime.frame = { data: frame, isLoading: false, isStale: false, error: null, retry: vi.fn() }
    const inputs: ChartInput[] = []
    const line = panel('line', {
      encoding: { category: 'category', series: 'series', value: 'value' },
      presentation: { legend: 'below' },
    })
    const view = render(<LinePanel panel={line} adapter={fakeAdapter((input) => inputs.push(input))} />)

    await waitFor(() => expect(legendLabels(view.container)).toEqual(['Written premium', 'Earned premium']))
    const data = screen.getByRole('list', { name: 'Chart data for line panel' })
    expect(data).toHaveTextContent('2025, Written premium, 100')
    expect(data).toHaveTextContent('2025, Earned premium, 80')

    fireEvent.click(legendRow(view.container, 'Earned premium'))
    await waitFor(() => expect(inputs.at(-1)?.frame.rows).toEqual([
      ['2025', 'Written premium', 100],
      ['2026', 'Written premium', 120],
    ]))
    expect(data).not.toHaveTextContent('Earned premium')
    expect(data).toHaveTextContent('2026, Written premium, 120')
  })

  it('prints period sums and supports solo and repeat-solo restore without bulk controls for two series', async () => {
    const frame: Frame = {
      columns: [
        { name: 'category', type: 'string' },
        { name: 'series', type: 'string' },
        { name: 'value', type: 'number' },
      ],
      rows: [
        ['2025', 'Written', 100], ['2025', 'Earned', 80],
        ['2026', 'Written', 120], ['2026', 'Earned', 90],
      ],
    }
    runtime.frame = { data: frame, isLoading: false, isStale: false, error: null, retry: vi.fn() }
    const inputs: ChartInput[] = []
    const line = panel('line', {
      encoding: { category: 'category', series: 'series', value: 'value' },
      presentation: { legend: 'below' },
    })
    const view = render(<LinePanel panel={line} adapter={fakeAdapter((input) => inputs.push(input))} />)

    expect(legendRow(view.container, 'Written')).toHaveTextContent('220')
    // Isolating is the row's double-click — the charting idiom — rather than a
    // second button beside it.
    fireEvent.doubleClick(legendRow(view.container, 'Earned'))
    await waitFor(() => expect(inputs.at(-1)?.frame.rows.every((row) => row[1] === 'Earned')).toBe(true))
    expect(new URL(window.location.href).searchParams.getAll('lens-state').length).toBe(1)

    fireEvent.doubleClick(legendRow(view.container, 'Earned'))
    await waitFor(() => expect(inputs.at(-1)?.frame.rows).toHaveLength(4))

    // With only two series, the per-series controls are enough.
    expect(screen.queryByRole('button', { name: 'Invert' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Hide all' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Show all' })).toBeNull()
  })

  it('shows bulk legend controls above four entries and hides them at or below that threshold', async () => {
    const columns: Frame['columns'] = [
      { name: 'category', type: 'string' },
      { name: 'series', type: 'string' },
      { name: 'value', type: 'number' },
    ]
    const line = panel('line', {
      encoding: { category: 'category', series: 'series', value: 'value' },
      presentation: { legend: 'below' },
    })
    runtime.frame = {
      data: { columns, rows: Array.from({ length: 6 }, (_, index) => ['2026', `Series ${index + 1}`, index + 1]) },
      isLoading: false, isStale: false, error: null, retry: vi.fn(),
    }
    const long = render(<LinePanel panel={line} adapter={fakeAdapter()} />)
    // One threshold: the entry count that earns a search box is the same one
    // that earns bulk selection, so the column has two compositions, not four.
    expect(screen.getByRole('searchbox', { name: 'Search legend' })).toBeInTheDocument()
    // The pair is one two-state control, not two loose commands: with every
    // series plotted, "Show all" is the state the legend is already in.
    expect(screen.getByRole('button', { name: 'Show all' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Hide all' })).toHaveAttribute('aria-pressed', 'false')
    fireEvent.click(screen.getByRole('button', { name: 'Hide all' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Hide all' })).toHaveAttribute('aria-pressed', 'true'))
    expect(screen.getByRole('button', { name: 'Show all' })).toHaveAttribute('aria-pressed', 'false')
    // Invert takes it back to the state it started in rather than to a third one.
    fireEvent.click(screen.getByRole('button', { name: 'Invert' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Show all' })).toHaveAttribute('aria-pressed', 'true'))
    long.unmount()

    runtime.frame = {
      data: { columns, rows: Array.from({ length: 3 }, (_, index) => ['2026', `Short ${index + 1}`, index + 1]) },
      isLoading: false, isStale: false, error: null, retry: vi.fn(),
    }
    const short = render(<LinePanel panel={line} adapter={fakeAdapter()} />)
    expect(screen.queryByRole('button', { name: 'Hide all' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Show all' })).toBeNull()
    expect(screen.queryByRole('searchbox', { name: 'Search legend' })).toBeNull()
    short.unmount()

    runtime.frame = {
      data: { columns, rows: Array.from({ length: 4 }, (_, index) => ['2026', `Bulk ${index + 1}`, index + 1]) },
      isLoading: false, isStale: false, error: null, retry: vi.fn(),
    }
    const threshold = render(<LinePanel panel={line} adapter={fakeAdapter()} />)
    expect(screen.queryByRole('button', { name: 'Hide all' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Show all' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Invert' })).toBeNull()
    expect(screen.queryByRole('searchbox', { name: 'Search legend' })).toBeNull()
    threshold.unmount()

    runtime.frame = {
      data: { columns, rows: Array.from({ length: 5 }, (_, index) => ['2026', `Bulk ${index + 1}`, index + 1]) },
      isLoading: false, isStale: false, error: null, retry: vi.fn(),
    }
    render(<LinePanel panel={line} adapter={fakeAdapter()} />)
    expect(screen.getByRole('button', { name: 'Hide all' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Show all' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Invert' })).toBeNull()
    expect(screen.queryByRole('searchbox', { name: 'Search legend' })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Hide all' }))
    await screen.findByRole('button', { name: 'Show all' })
  })

  it('prints the latest ratio reading instead of summing a ratio series', () => {
    runtime.frame = {
      data: {
        columns: [
          { name: 'category', type: 'string' },
          { name: 'series', type: 'string' },
          { name: 'value', type: 'number' },
        ],
        rows: [['2025', 'Loss ratio', 82], ['2026', 'Loss ratio', 91]],
      },
      isLoading: false, isStale: false, error: null, retry: vi.fn(),
    }
    const view = render(<LinePanel
      adapter={fakeAdapter()}
      panel={panel('line', {
        encoding: { category: 'category', series: 'series', value: 'value' },
        format: { value: { kind: 'percent', minorUnits: false, precision: 0 } },
        presentation: { legend: 'below' },
      })}
    />)

    expect(legendRow(view.container, 'Loss ratio')).toHaveTextContent('91')
    expect(legendRow(view.container, 'Loss ratio')).not.toHaveTextContent('173')
  })

  it('exposes every drill mark as a keyboard action', async () => {
    runtime.frame = {
      data: {
        columns: [{ name: 'id', type: 'string' }, { name: 'label', type: 'string' }, { name: 'value', type: 'number' }],
        rows: [['root/a', 'Alpha', 10], ['beta', 'Beta', 20]],
      },
      isLoading: false, isStale: false, error: null, retry: vi.fn(),
    }
    render(<BarPanel
      adapter={fakeAdapter()}
      panel={panel('bar', { drillRoot: 'root', encoding: { id: 'id', label: 'label', value: 'value' } })}
    />)

    const actions = await screen.findAllByRole('button', { name: /Open (Alpha|Beta), (10|20)/ })
    expect(actions).toHaveLength(2)
    fireEvent.click(actions[1]!)
    expect(runtime.drillInto).toHaveBeenCalledWith('beta', 'panel-bar')

    runtime.drillInto.mockReset()
    const keyboardAction = actions[0]!
    keyboardAction.focus()
    fireEvent.keyDown(keyboardAction, { key: 'Enter', code: 'Enter' })
    // Browsers synthesize this click as the default Enter action for a button.
    fireEvent.click(keyboardAction)
    fireEvent.keyUp(keyboardAction, { key: 'Enter', code: 'Enter' })
    expect(runtime.drillInto).toHaveBeenCalledWith('root/a', 'panel-bar')

    runtime.drillInto.mockReset()
    fireEvent.click(screen.getByRole('button', { name: 'chart data' }))
    expect(runtime.drillInto).toHaveBeenCalledWith('root/a', 'panel-bar')
  })

  it('exposes formatted category, series, and value for a non-actionable chart', () => {
    runtime.frame = {
      data: {
        columns: [
          { name: 'category', type: 'string' },
          { name: 'series', type: 'string' },
          { name: 'value', type: 'number' },
        ],
        rows: [['January', 'Revenue', 1200], ['January', 'Cost', 500]],
      },
      isLoading: false, isStale: false, error: null, retry: vi.fn(),
    }
    render(<LinePanel
      adapter={fakeAdapter()}
      panel={panel('line', { encoding: { category: 'category', series: 'series', value: 'value' } })}
    />)

    const data = screen.getByRole('list', { name: 'Chart data for line panel' })
    expect(data).toHaveTextContent('January, Revenue, 1200')
    expect(data).toHaveTextContent('January, Cost, 500')
    expect(data.querySelectorAll('button')).toHaveLength(0)
  })

  it('falls back to a present label column when the default category column is absent', () => {
    runtime.frame = {
      data: {
        columns: [{ name: 'id', type: 'string' }, { name: 'label', type: 'string' }, { name: 'value', type: 'number' }],
        rows: [['osago', 'ОСАГО', 312_200_000], ['travel', 'Путешествия', 10_000_000]],
      },
      isLoading: false, isStale: false, error: null, retry: vi.fn(),
    }
    render(<BarPanel
      adapter={fakeAdapter()}
      panel={panel('hbar', { encoding: { id: 'id', category: 'category', label: 'label', value: 'value' } })}
    />)

    const equivalent = screen.getByRole('list', { name: 'Chart data for hbar panel' })
    expect(equivalent).toHaveTextContent('ОСАГО, 312200000')
    expect(equivalent).toHaveTextContent('Путешествия, 10000000')
  })

  it('gives same-category actionable series unambiguous names with values', async () => {
    runtime.frame = {
      data: {
        columns: [
          { name: 'category', type: 'string' },
          { name: 'series', type: 'string' },
          { name: 'value', type: 'number' },
        ],
        rows: [['January', 'Revenue', 1200], ['January', 'Cost', 500]],
      },
      isLoading: false, isStale: false, error: null, retry: vi.fn(),
    }
    render(<LinePanel
      adapter={fakeAdapter()}
      panel={panel('line', {
        drillRoot: 'root',
        encoding: { category: 'category', series: 'series', value: 'value' },
      })}
    />)

    expect(await screen.findByRole('button', { name: 'Open January, Revenue, 1200' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Open January, Cost, 500' })).toBeInTheDocument()
  })

  it('shows legend search above six series and filters without changing the plot', () => {
    const rows = ['2025', '2026'].flatMap((year) => Array.from({ length: 7 }, (_, index) => [year, `Product ${index + 1}`, index + 1]))
    runtime.frame = {
      data: {
        columns: [
          { name: 'category', type: 'string' },
          { name: 'series', type: 'string' },
          { name: 'value', type: 'number' },
        ],
        rows,
      },
      isLoading: false, isStale: false, error: null, retry: vi.fn(),
    }
    const line = panel('line', {
      encoding: { category: 'category', series: 'series', value: 'value' },
      presentation: { legend: 'below' },
    })
    const view = render(<LinePanel panel={line} adapter={fakeAdapter()} />)
    const search = screen.getByRole('searchbox', { name: 'Search legend' })
    fireEvent.change(search, { target: { value: 'Product 7' } })
    // The search narrows the list a reader reads; it must not narrow the plot,
    // which still has to draw the series the reader filtered out of view.
    expect(legendLabels(view.container)).toEqual(['Product 7'])
    expect(view.container.querySelector('.lens-chart-canvas')).toBeInTheDocument()
  })

  it('rebuilds a hidden stack from zero by removing its rows from the chart input', async () => {
    const frame: Frame = {
      columns: [
        { name: 'category', type: 'string' }, { name: 'series', type: 'string' }, { name: 'value', type: 'number' },
      ],
      rows: [['2026', 'Direct', 100], ['2026', 'Inward', 50], ['2027', 'Direct', 80], ['2027', 'Inward', 30]],
    }
    runtime.frame = { data: frame, isLoading: false, isStale: false, error: null, retry: vi.fn() }
    const inputs: ChartInput[] = []
    const view = render(<BarPanel panel={panel('bar', {
      encoding: { category: 'category', series: 'series', value: 'value' },
      presentation: { legend: 'below', stack: true },
    })} adapter={fakeAdapter((input) => inputs.push(input))} />)

    fireEvent.click(legendRow(view.container, 'Direct'))
    await waitFor(() => expect(inputs.at(-1)?.frame.rows).toEqual([['2026', 'Inward', 50], ['2027', 'Inward', 30]]))
    expect(inputs.at(-1)?.frame.total).toBe(80)
  })

  it('collapses expandable TopN and sub-two-percent donut tails without discarding their source rows', () => {
    const topNFrame: Frame = {
      columns: [
        { name: 'id', type: 'string' }, { name: 'label', type: 'string' },
        { name: 'value', type: 'number' }, { name: '__lens_topn_group', type: 'string' },
      ],
      rows: [['a', 'A', 100, null], ['b', 'B', 4, 'Other'], ['c', 'C', 3, 'Other']],
    }
    const topN = collapseExpandableTopN(topNFrame, panel('bar', { encoding: { id: 'id', label: 'label', value: 'value' } }))
    expect(topN.frame.rows).toEqual([['a', 'A', 100, null], [donutRemainderKey, 'Other', 7, 'Other']])

    const donut = collapseMinorDonutSlices({ ...topNFrame, rows: [['a', 'A', 98, null], ['b', 'B', 1, null], ['c', 'C', 1, null]] }, panel('donut', {
      encoding: { id: 'id', label: 'label', value: 'value' },
    }), 'Other')
    expect(donut.frame.rows.at(-1)?.slice(0, 3)).toEqual([donutRemainderKey, 'Other', 2])
  })

  it('expands a collapsed TopN remainder when its synthetic mark is selected', async () => {
    runtime.frame = {
      data: {
        columns: [
          { name: 'id', type: 'string' }, { name: 'label', type: 'string' },
          { name: 'value', type: 'number' }, { name: '__lens_topn_group', type: 'string' },
        ],
        rows: [['a', 'A', 100, null], ['b', 'B', 4, 'Other'], ['c', 'C', 3, 'Other']],
      },
      isLoading: false, isStale: false, error: null, retry: vi.fn(),
    }
    const inputs: ChartInput[] = []
    const adapter: ChartAdapter = {
      mount(el, input, events) {
        inputs.push(input)
        const button = document.createElement('button')
        button.textContent = 'Other'
        button.onclick = () => events.onSelect(donutRemainderKey)
        el.append(button)
        return { update: (next) => inputs.push(next), dispose: () => el.replaceChildren() }
      },
    }
    render(<BarPanel panel={panel('bar', { encoding: { id: 'id', label: 'label', value: 'value' } })} adapter={adapter} />)

    await waitFor(() => expect(inputs.at(-1)?.frame.rows).toHaveLength(2))
    fireEvent.click(screen.getByRole('button', { name: 'Other' }))
    await waitFor(() => expect(inputs.at(-1)?.frame.rows).toHaveLength(3))
    expect(screen.getByRole('button', { name: 'Collapse Other' })).toBeInTheDocument()
  })

  it('expands a collapsed TopN remainder from its keyboard action when category and id are absent', async () => {
    runtime.frame = {
      data: {
        columns: [
          { name: 'label', type: 'string' }, { name: 'value', type: 'number' },
          { name: '__lens_topn_group', type: 'string' },
        ],
        rows: [['A', 100, null], ['B', 4, 'Other'], ['C', 3, 'Other']],
      },
      isLoading: false, isStale: false, error: null, retry: vi.fn(),
    }
    const inputs: ChartInput[] = []
    render(<BarPanel
      panel={panel('bar', { encoding: { category: 'category', label: 'label', value: 'value' } })}
      adapter={fakeAdapter((input) => inputs.push(input))}
    />)

    await waitFor(() => expect(inputs.at(-1)?.frame.rows).toHaveLength(2))
    fireEvent.click(screen.getByRole('button', { name: 'Open Other, 7' }))
    await waitFor(() => expect(inputs.at(-1)?.frame.rows).toHaveLength(3))
    expect(screen.getByRole('button', { name: 'Collapse Other' })).toBeInTheDocument()
  })

  it('expands the served cube TopN remainder through the keyboard action key', async () => {
    runtime.frame = {
      data: {
        columns: [
          { name: 'filter_value', type: 'string' }, { name: 'id', type: 'string' },
          { name: 'label', type: 'string' }, { name: 'total_policies', type: 'number' },
          { name: 'total_policies_previous', type: 'number' },
          { name: '__lens_topn_group', type: 'string' },
        ],
        rows: [
          ...Array.from({ length: 10 }, (_, index) => [`top-${index}`, `row-${index}`, `Product ${index}`, 100 - index, 90 - index, null]),
          ...Array.from({ length: 8 }, (_, index) => [`tail-${index}`, `row-${index + 10}`, `Tail ${index}`, 9 - index, 8 - index, 'Other']),
        ],
      },
      isLoading: false, isStale: false, error: null, retry: vi.fn(),
    }
    const inputs: ChartInput[] = []
    render(<BarPanel
      panel={panel('hbar', {
        encoding: {
          id: 'filter_value', label: 'label', category: 'label',
          value: 'total_policies', previous: 'total_policies_previous',
        },
      })}
      adapter={fakeAdapter((input) => inputs.push(input))}
    />)

    await waitFor(() => expect(inputs.at(-1)?.frame.rows).toHaveLength(11))
    fireEvent.keyDown(screen.getByRole('button', { name: 'Open Other, 44' }), { key: 'Enter' })
    await waitFor(() => expect(inputs.at(-1)?.frame.rows).toHaveLength(18))
    expect(screen.getByRole('button', { name: 'Collapse Other' })).toBeInTheDocument()
  })

  it('keeps a series in its own colour when the series before it is hidden', async () => {
    const frame: Frame = {
      columns: [
        { name: 'category', type: 'string' },
        { name: 'series', type: 'string' },
        { name: 'value', type: 'number' },
      ],
      rows: [
        ['2025', 'Claimed', 100],
        ['2025', 'Paid', 10],
        ['2026', 'Claimed', 120],
        ['2026', 'Paid', 20],
      ],
    }
    runtime.frame = { data: frame, isLoading: false, isStale: false, error: null, retry: vi.fn() }
    // The pins are panel-scoped and positional, which is what makes this
    // breakable: hiding a series used to slide every series after it one
    // position down, so the payout line repainted itself in the hidden
    // series' colour while its legend swatch kept the old one.
    runtime.document = {
      ...runtime.document,
      theme: { palette: {}, series: { 'panel-line:0': '#2563eb', 'panel-line:1': '#d97706' } },
    } as DashboardDocument
    const inputs: ChartInput[] = []
    const line = panel('line', {
      encoding: { category: 'category', series: 'series', value: 'value' },
      presentation: { legend: 'below' },
    })
    const view = render(<LinePanel panel={line} adapter={fakeAdapter((input) => inputs.push(input))} />)

    await waitFor(() => expect(legendLabels(view.container)).toEqual(['Claimed', 'Paid']))
    expect(inputs.at(-1)?.seriesColor?.('Paid', 1)).toBe('#d97706')

    fireEvent.click(legendRow(view.container, 'Claimed'))
    await waitFor(() => expect(inputs.at(-1)?.frame.rows).toHaveLength(2))
    // The chart now hands index 0 — the only series it can see — and must
    // still be told amber, the colour the legend beside it is still printing.
    expect(inputs.at(-1)?.seriesColor?.('Paid', 0)).toBe('#d97706')
    expect(legendSwatches(view.container).at(-1)).toBe('rgb(217, 119, 6)')
  })

  it('shows the figure\'s own shape while a strip cell is loading, not the empty-value dash', () => {
    runtime.frame = state('loading')
    const { container } = render(<StatMetric panel={panel('stat', { encoding: { value: 'value' } })} />)

    // «—» is what this product prints for a value that is genuinely absent, so
    // a loading cell that printed it made a slow strip and an empty one look
    // the same.
    expect(container.querySelector('.lens-stat-metric-value-shimmer')).not.toBeNull()
    expect(container.querySelector('.lens-stat-metric-value')?.textContent).toBe('')
    expect(container.querySelector('.lens-stat-metric')).toHaveAttribute('aria-busy', 'true')
  })

  it('marks a strip cell whose figure is being replaced', () => {
    // A recompute keeps the old numbers on screen while the request is open.
    // The card form said so («Updating» plus the stale dim) and the strip cell —
    // which is what a KPI actually is — said nothing, so the figures a reader is
    // most likely to act on were the ones that looked settled.
    runtime.frame = state('stale')
    const { container } = render(<StatMetric panel={panel('stat', { encoding: { value: 'value' } })} />)

    const cell = container.querySelector('.lens-stat-metric')
    expect(cell).toHaveAttribute('data-stale', 'true')
    expect(container.querySelector('.lens-stat-metric-value')?.textContent).toBe('42')
  })

  it('renders the panel title once when the stat label would duplicate it', () => {
    runtime.frame = state('data')
    render(<StatPanel panel={panel('stat', { encoding: { value: 'value' } })} />)
    expect(screen.getAllByText('stat panel')).toHaveLength(1)
    expect(screen.getByText('42')).toBeInTheDocument()
  })

  it.each([
    { absolute: 42, expected: 'New' },
    { absolute: 0, expected: 'N/A' },
  ])('renders $expected when a comparison baseline cannot produce a percentage', ({ absolute, expected }) => {
    runtime.frame = {
      data: {
        columns: [
          { name: 'value', type: 'number' },
          { name: 'delta', type: 'number' },
          { name: 'delta_percent', type: 'number' },
        ],
        rows: [[42, absolute, null]],
      },
      isLoading: false, isStale: false, error: null, retry: vi.fn(),
    }
    render(<StatPanel panel={panel('stat', {
      encoding: { value: 'value' },
      trend: { percent: 0, absoluteField: 'delta', percentField: 'delta_percent' },
    })} />)

    expect(screen.getByText(expected)).toBeInTheDocument()
    expect(screen.queryByText('+0.0%')).toBeNull()
  })

  it('shows percentage KPI comparisons in points instead of relative percent change', () => {
    runtime.frame = {
      data: {
        columns: [{ name: 'value', type: 'number' }, { name: 'delta', type: 'number' }, { name: 'delta_percent', type: 'number' }],
        rows: [[3, -11, -78.4]],
      },
      isLoading: false, isStale: false, error: null, retry: vi.fn(),
    }
    render(<StatPanel panel={panel('stat', {
      encoding: { value: 'value' },
      trend: { percent: -78.4, absoluteField: 'delta', percentField: 'delta_percent', absoluteDeltaUnit: 'percentage_points' },
    })} />)

    expect(screen.getByText('-11 pp')).toBeInTheDocument()
    expect(screen.getByText(/was 14/)).toBeInTheDocument()
    expect(screen.queryByText('-78.4%')).toBeNull()
    expect(document.querySelector('.lens-trend-chip')).toHaveAttribute('title', '-11 pp · was 14')
  })
})

describe('coverage panel', () => {
  const coverageFrame = (rows: Array<Array<unknown>>): Frame => ({
    columns: [
      { name: 'id', type: 'string' },
      { name: 'label', type: 'string' },
      { name: 'action_url', type: 'string' },
      { name: 'value', type: 'number' },
    ],
    rows,
  })

  const coveragePanel = (overrides: Partial<Panel> = {}): Panel => panel('coverage', {
    encoding: { id: 'id', label: 'label', value: 'value' },
    ...overrides,
  })

  it('leads with a headline value plus a muted total label', () => {
    runtime.frame = { data: coverageFrame([['a', 'Alpha', '', 60], ['b', 'Beta', '', 40]]), isLoading: false, isStale: false, error: null, retry: vi.fn() }
    const view = render(<CoveragePanel panel={coveragePanel({ headline: 100 })} />)
    const headline = view.container.querySelector('.lens-coverage-headline')
    expect(headline?.querySelector('.lens-coverage-headline-value')).toHaveTextContent('100')
    expect(headline?.querySelector('.lens-coverage-headline-label')).toHaveTextContent('Total')
  })

  it('renders one track segment per positive value with a share width, and a legend row per segment', () => {
    runtime.frame = { data: coverageFrame([['a', 'Alpha', '', 997], ['b', 'Beta', '', 3]]), isLoading: false, isStale: false, error: null, retry: vi.fn() }
    const view = render(<CoveragePanel panel={coveragePanel()} />)
    const segments = view.container.querySelectorAll('.lens-coverage-track-segment')
    expect(segments).toHaveLength(2)
    // The thin (0.3%) slice keeps its true share width; a CSS min-width (not an
    // inline style) guarantees it stays visible, so the encoded share is honest.
    expect((segments[1] as HTMLElement).style.width).toBe('0.3%')
    // What the segment is worth is the legend's job, one row below and in the
    // sheet's own type. The segment used to say it again through a native
    // tooltip; the only `title` left on this panel is the legend label's, and
    // it repeats nothing — the label is truncated to keep the value and share
    // columns aligned, so it is the full name behind the clip.
    expect(segments[1]).not.toHaveAttribute('title')
    const rows = view.container.querySelectorAll('.lens-coverage-legend-row')
    expect(rows).toHaveLength(2)
    expect(rows[1]?.querySelector('.lens-coverage-legend-label')).toHaveAttribute('title', 'Beta')
    expect(rows[1]).toHaveTextContent('Beta')
    expect(rows[1]).toHaveTextContent('3')
  })

  it('drops the track when a single segment is 100%, keeping the headline and legend rows', () => {
    runtime.frame = { data: coverageFrame([['a', 'Alpha', '', 100], ['b', 'Beta', '', 0]]), isLoading: false, isStale: false, error: null, retry: vi.fn() }
    const view = render(<CoveragePanel panel={coveragePanel()} />)
    expect(view.container.querySelector('.lens-coverage-track')).toBeNull()
    expect(view.container.querySelectorAll('.lens-coverage-legend-row')).toHaveLength(2)
  })

  it('keeps a single positive segment visible when a target provides the comparison', () => {
    runtime.frame = { data: coverageFrame([['a', 'Alpha', '', 100], ['b', 'Beta', '', 0]]), isLoading: false, isStale: false, error: null, retry: vi.fn() }
    const view = render(<CoveragePanel panel={coveragePanel({
      target: { value: 125, label: 'Target' },
    })} />)

    expect(view.container.querySelector('.lens-coverage-bullet')).not.toBeNull()
    expect(view.container.querySelectorAll('.lens-coverage-track-segment')).toHaveLength(1)
    expect(view.container.querySelector('.lens-coverage-bullet-marker')).not.toBeNull()
  })

  it('keeps track segments decorative and exposes one legend link per row-scoped action', () => {
    runtime.frame = {
      data: coverageFrame([['a', 'Alpha', '/drill/a', 60], ['b', 'Beta', '/drill/b', 40]]),
      isLoading: false, isStale: false, error: null, retry: vi.fn(),
    }
    const action = {
      kind: 'navigate' as const,
      params: [],
      payload: {},
      urlSource: { kind: 'field' as const, name: 'action_url' },
    }
    const view = render(<CoveragePanel panel={coveragePanel({ actions: [action] })} />)
    const segmentLinks = view.container.querySelectorAll('a.lens-coverage-track-segment-link')
    expect(segmentLinks).toHaveLength(0)
    expect(view.container.querySelector('.lens-coverage-track')).toHaveAttribute('aria-hidden', 'true')
    // Decorative all the way down: the hidden track carries no text of its own
    // for anyone to miss. It used to hold a native tooltip per segment, which
    // in this configuration was addressed to nobody — pointer readers had the
    // legend row it highlights, and the rest of the track was hidden outright.
    expect(view.container.querySelectorAll('.lens-coverage-track[title], .lens-coverage-track [title]')).toHaveLength(0)
    const legendLinks = view.container.querySelectorAll('a.lens-coverage-legend-link')
    expect(legendLinks).toHaveLength(2)
    expect(legendLinks[0]?.getAttribute('href')).toContain('/drill/a')
    expect(legendLinks[1]?.getAttribute('href')).toContain('/drill/b')
    // The link is the row; the row's own label carries the clipped-text title,
    // so no anchor restates it as a tooltip over the whole thing.
    expect(view.container.querySelectorAll('a.lens-coverage-legend-link[title]')).toHaveLength(0)
  })

  it('leaves the bullet track silent and keeps the target label its clipped name', () => {
    runtime.frame = { data: coverageFrame([['a', 'Alpha', '', 100], ['b', 'Beta', '', 20]]), isLoading: false, isStale: false, error: null, retry: vi.fn() }
    const view = render(<CoveragePanel panel={coveragePanel({ target: { value: 125, label: 'Target' } })} />)

    // The bullet's track is `aria-hidden` in every configuration, so a tooltip
    // on its segments is unreachable by definition.
    expect(view.container.querySelectorAll('.lens-coverage-track[title], .lens-coverage-track [title]')).toHaveLength(0)
    // The one exception the rule allows: the marker's label is capped at the
    // room left beside the tick and ellipsizes, so it names itself in full.
    expect(view.container.querySelector('.lens-coverage-bullet-label')).toHaveAttribute('title', 'Target 125')
  })
})

describe('cascade stages', () => {
  it('uses encoding overrides for running totals, connectors, widths, and the final stage', () => {
    const cascade = panel('cascade', {
      encoding: { label: 'stage_name', value: 'balance', cut: 'movement', cutLabel: 'movement_name', final: 'is_total' },
    })
    const frame: Frame = {
      columns: [
        { name: 'stage_name', type: 'string' },
        { name: 'balance', type: 'number' },
        { name: 'movement', type: 'number' },
        { name: 'movement_name', type: 'string' },
        { name: 'is_total', type: 'bool' },
      ],
      rows: [
        ['Gross', 1000, 0, '', false],
        ['After claims', 750, 250, 'Claims', false],
        ['Net', 4, -50, 'Recoveries', true],
      ],
    }
    const money = (value: unknown) => typeof value === 'number' ? `$${value}` : ''
    const stages = buildCascadeStages(cascade, frame, money, money)

    expect(stages.map(({ label, width, final }) => ({ label, width, final }))).toEqual([
      { label: 'Gross', width: 100, final: false },
      { label: 'After claims', width: 75, final: false },
      { label: 'Net', width: 2, final: true },
    ])
    expect(stages[1]?.formattedCut).toBe('−$250')
    expect(stages[2]?.formattedCut).toBe('+$50')
  })

  it('picks an axis step that leaves the tallest bar near the top of the plot', () => {
    // The case from the profitability dashboard: 655.09 bn of earned premium
    // used to sit under a 1 trn ceiling, wasting a third of the plot.
    const step = waterfallAxisStep(0, 655.09e9)
    const top = Math.ceil(655.09e9 / step) * step
    expect(top).toBe(700e9)
    expect(top / step).toBeLessThanOrEqual(7)

    // A deficit still gets whole steps below zero rather than a clipped bar.
    const negative = waterfallAxisStep(-30, 120)
    expect(Math.floor(-30 / negative) * negative).toBeLessThanOrEqual(-30)
    expect(Math.ceil(120 / negative) * negative).toBeGreaterThanOrEqual(120)

    // Degenerate frames must still produce a usable positive step.
    expect(waterfallAxisStep(0, 0)).toBeGreaterThan(0)
  })

  it('renders a conventional vertical waterfall projection when requested', () => {
    const cascade = panel('cascade', {
      encoding: {
        label: 'label', value: 'value', cut: 'cut', cutLabel: 'cutLabel',
        final: 'final', annotation: 'annotation',
      },
      presentation: { bridgeLayout: 'waterfall' },
    })
    const frame: Frame = {
      columns: [
        { name: 'label', type: 'string' },
        { name: 'value', type: 'number' },
        { name: 'cut', type: 'number' },
        { name: 'cutLabel', type: 'string' },
        { name: 'final', type: 'bool' },
        { name: 'annotation', type: 'string' },
      ],
      rows: [
        ['Opening', 235, 0, '', false, ''],
        ['Closing', 56.98, 178.02, 'Net movement', true, '12 above threshold'],
      ],
    }
    const format = (value: unknown) => String(value)
    const items = buildWaterfallItems(buildCascadeStages(cascade, frame, format, format), format)
    expect(items.map(({ label, kind }) => ({ label, kind }))).toEqual([
      { label: 'Opening', kind: 'start' },
      { label: 'Net movement', kind: 'decrease' },
      { label: 'Closing', kind: 'end' },
    ])
    expect(items[1]?.formattedValue).toBe('−178.02')
    expect(items[1]?.annotation).toBe('12 above threshold')

    runtime.frame = { data: frame, isLoading: false, isStale: false, error: null, retry: vi.fn() }
    const view = render(<CascadePanel panel={cascade} />)
    expect(view.container.querySelector('[data-lens-waterfall]')).not.toBeNull()
    expect(view.container.querySelectorAll('.lens-waterfall-bar')).toHaveLength(3)
    expect(view.container.querySelector('.lens-waterfall-bar[data-kind="decrease"]')).not.toBeNull()
    expect(Array.from(view.container.querySelectorAll('.lens-waterfall-bar')).map((bar) => bar.getAttribute('data-label-row'))).toEqual(['0', '1', '0'])
    expect(view.container.querySelector('.lens-waterfall-annotation')).toHaveTextContent('12 above threshold')
  })

  it('draws an explicit pure-total final row once, without a zero-delta duplicate', () => {
    const cascade = panel('cascade', {
      encoding: { label: 'label', value: 'value', cut: 'cut', cutLabel: 'cutLabel', final: 'final' },
      presentation: { bridgeLayout: 'waterfall' },
    })
    const frame: Frame = {
      columns: [
        { name: 'label', type: 'string' },
        { name: 'value', type: 'number' },
        { name: 'cut', type: 'number' },
        { name: 'cutLabel', type: 'string' },
        { name: 'final', type: 'bool' },
      ],
      rows: [
        ['Earned premium', 178.30, 0, '', false],
        ['Reserve movement', 66.34, 111.96, 'Reserve movement', false],
        // Canonical closing row: restates the running total (cut=0) and is
        // marked final. The running totals are floating-point sums, so the
        // restated value differs from the previous stage by a residual epsilon
        // rather than exactly — the tolerance, not strict equality, is what
        // suppresses the "+0" delta bar here.
        ['Underwriting result', 66.34 + 1e-5, 0, '', true],
      ],
    }
    const format = (value: unknown) => String(value)
    const items = buildWaterfallItems(buildCascadeStages(cascade, frame, format, format), format)
    // Exactly: opening total, one deduction, one closing total. No "+0" bar.
    expect(items.map(({ label, kind }) => ({ label, kind }))).toEqual([
      { label: 'Earned premium', kind: 'start' },
      { label: 'Reserve movement', kind: 'decrease' },
      { label: 'Underwriting result', kind: 'end' },
    ])
    expect(items.filter((item) => item.kind === 'end')).toHaveLength(1)
    // No residual near-zero delta slipped through as its own bar.
    expect(items.some((item) =>
      (item.kind === 'increase' || item.kind === 'decrease') && Math.abs(item.value) < 1,
    )).toBe(false)
  })

  it('draws each pure-total final row in place, so a cascade can pass through a checkpoint', () => {
    const cascade = panel('cascade', {
      encoding: { label: 'label', value: 'value', cut: 'cut', cutLabel: 'cutLabel', final: 'final' },
      presentation: { bridgeLayout: 'waterfall' },
    })
    const frame: Frame = {
      columns: [
        { name: 'label', type: 'string' },
        { name: 'value', type: 'number' },
        { name: 'cut', type: 'number' },
        { name: 'cutLabel', type: 'string' },
        { name: 'final', type: 'bool' },
      ],
      rows: [
        ['Earned premium', 200, 0, '', false],
        ['Claims paid', 190, 10, 'Claims paid', false],
        // The statutory result: a total the reader recognises, mid-cascade.
        ['Underwriting result', 190, 0, '', true],
        ['Case reserves', 170, 20, 'Case reserves', false],
        ['Pre-tax result', 170, 0, '', true],
      ],
    }
    const format = (value: unknown) => String(value)
    const items = buildWaterfallItems(buildCascadeStages(cascade, frame, format, format), format)
    expect(items.map(({ label, kind, checkpoint }) => ({ label, kind, checkpoint }))).toEqual([
      { label: 'Earned premium', kind: 'start', checkpoint: undefined },
      { label: 'Claims paid', kind: 'decrease', checkpoint: undefined },
      // Stands on zero where it was declared, not hoisted to the end...
      { label: 'Underwriting result', kind: 'end', checkpoint: true },
      { label: 'Case reserves', kind: 'decrease', checkpoint: undefined },
      // ...and only the last total is the finish.
      { label: 'Pre-tax result', kind: 'end', checkpoint: undefined },
    ])
    // Both totals stand on zero and carry their absolute value, not a delta.
    expect(items.filter((item) => item.kind === 'end').map((item) => item.value)).toEqual([190, 170])
  })

  it('tells a step that did not happen from the smallest step that did', () => {
    const cascade = panel('cascade', {
      encoding: { label: 'label', value: 'value', cut: 'cut', cutLabel: 'cutLabel', final: 'final' },
      presentation: { bridgeLayout: 'waterfall' },
    })
    const frame: Frame = {
      columns: [
        { name: 'label', type: 'string' },
        { name: 'value', type: 'number' },
        { name: 'cut', type: 'number' },
        { name: 'cutLabel', type: 'string' },
        { name: 'final', type: 'bool' },
      ],
      rows: [
        ['Earned premium', 200, 0, '', false],
        // Not applicable in this period — no acquisition cost was booked.
        ['Acquisition', 200, 0, 'Acquisition', false],
        // A real, tiny movement. The two used to render identically.
        ['Operating', 199.9, 0.1, 'Operating', false],
        ['Result', 199.9, 0, '', true],
      ],
    }
    const format = (value: unknown) => String(value)
    const items = buildWaterfallItems(buildCascadeStages(cascade, frame, format, format), format)
    const byLabel = (label: string) => items.find((item) => item.label === label)

    expect(byLabel('Acquisition')?.noMovement).toBe(true)
    expect(byLabel('Acquisition')?.height).toBe(0)
    expect(byLabel('Operating')?.noMovement).toBeUndefined()
    expect(byLabel('Operating')?.height).toBeGreaterThan(0)
    // The opening and closing totals are not movements and cannot be zero ones.
    expect(byLabel('Earned premium')?.noMovement).toBeUndefined()
    expect(byLabel('Result')?.noMovement).toBeUndefined()
  })

  // A stage the producer has no amount for. The frame says null, which is
  // «Нет данных» — not «0 UZS», and the difference between those two sentences
  // is the difference between a bridge and a fiction.
  const unknownCascade = () => panel('cascade', {
    encoding: {
      label: 'label', value: 'value', cut: 'cut', cutLabel: 'cutLabel',
      final: 'final', annotation: 'annotation',
    },
    presentation: { bridgeLayout: 'waterfall' },
  })

  const unknownColumns: Frame['columns'] = [
    { name: 'label', type: 'string' },
    { name: 'value', type: 'number' },
    { name: 'cut', type: 'number' },
    { name: 'cutLabel', type: 'string' },
    { name: 'final', type: 'bool' },
    { name: 'annotation', type: 'string' },
  ]

  it('reads an absent amount as unknown rather than as zero', () => {
    const cascade = unknownCascade()
    const frame: Frame = {
      columns: unknownColumns,
      rows: [
        ['Заработанная премия', 200, 0, '', false, ''],
        // Every shape a producer's "nothing here" arrives in.
        ['Null', null, 0, 'Null', false, ''],
        ['Empty', '', 0, 'Empty', false, ''],
        ['Not a number', 'n/a', 0, 'Not a number', false, ''],
        // A real zero is a measurement and stays one.
        ['Zero', 0, 200, 'Zero', false, ''],
      ],
    }
    const format = (value: unknown) => `${String(value)} UZS`
    const stages = buildCascadeStages(cascade, frame, format, format)

    expect(stages.map(({ label, hasValue }) => ({ label, hasValue }))).toEqual([
      { label: 'Заработанная премия', hasValue: true },
      { label: 'Null', hasValue: false },
      { label: 'Empty', hasValue: false },
      { label: 'Not a number', hasValue: false },
      { label: 'Zero', hasValue: true },
    ])
    // The list projection prints the em dash the metric panels print, never the
    // «0 UZS» a coerced null used to put in front of a reader — and the stage's
    // own movement, equally unknown, does not restate it either.
    expect(stages[1]?.formattedValue).toBe('—')
    expect(stages[1]?.formattedCut).toBe('—')
    expect(stages[1]?.width).toBe(0)
    expect(stages[4]?.formattedValue).toBe('0 UZS')
  })

  it('draws an unknown stage as a gap and keeps measuring from the last known total', () => {
    const cascade = unknownCascade()
    const frame: Frame = {
      columns: unknownColumns,
      rows: [
        ['Заработанная премия', 200, 0, '', false, ''],
        ['Страховые выплаты', 180, 20, 'Страховые выплаты', false, ''],
        // The stage from the live defect: nothing behind it, and a badge that
        // says so. It used to render a −180 deduction — the running total,
        // restated as a movement — and send every later delta off by that much.
        ['Изменение РЗУ', null, 0, 'Изменение РЗУ', false, 'Нет данных'],
        ['Операционные расходы', 150, 30, 'Операционные расходы', false, ''],
        ['Результат', 150, 0, '', true, ''],
      ],
    }
    const format = (value: unknown) => String(value)
    const items = buildWaterfallItems(buildCascadeStages(cascade, frame, format, format), format)
    const byLabel = (label: string) => items.find((item) => item.label === label)

    const gap = byLabel('Изменение РЗУ')
    expect(gap?.unknown).toBe(true)
    expect(gap?.value).toBe(0)
    expect(gap?.height).toBe(0)
    // It shares the hairline the zero-movement column already had; `unknown` is
    // what tells the two apart, in the model and in the stylesheet.
    expect(gap?.noMovement).toBe(true)
    expect(gap?.formattedValue).toBe('—')
    // The badge is the whole point of the row: it is the only thing on the
    // column that says why there is nothing to draw.
    expect(gap?.annotation).toBe('Нет данных')
    // And it keeps its frame row, so the drill the producer wired still opens.
    expect(gap?.rowIndex).toBe(2)
    // Nothing invented: no bar the size of the running total it stands on.
    expect(items.some((item) => Math.abs(item.value) === 180)).toBe(false)

    // The gap did not move the running total, so the stage after it is measured
    // from 180 — the last figure anyone knows — and not from a fabricated zero.
    expect(byLabel('Операционные расходы')?.value).toBe(-30)
    expect(byLabel('Результат')?.kind).toBe('end')
    expect(byLabel('Результат')?.value).toBe(150)

    // The plot model must agree: an unknown column draws no translucent
    // underlay either. Without this, the gap stood on a full-height accent
    // block reaching down to zero — the exact invented height the badge and
    // the zero-value bar above already refuse to draw.
    const model = buildWaterfallModel(buildCascadeStages(cascade, frame, format, format), format, format)
    expect(model.items.find((item) => item.label === 'Изменение РЗУ')?.underlayHeight).toBeUndefined()
  })

  it('closes on an unknown final row in place, without a dead twin beside it', () => {
    const cascade = unknownCascade()
    const frame: Frame = {
      columns: unknownColumns,
      rows: [
        ['Заработанная премия', 200, 0, '', false, ''],
        ['Страховые выплаты', 180, 20, 'Страховые выплаты', false, ''],
        // The closing total the dashboard cannot compute yet. Its bogus delta
        // was too large for the residual test, so it fell through to the
        // movement branch — and the synthetic closing then repeated it as a
        // second, rowIndex-less column that showed «Настроить» and did nothing.
        ['Расчётный результат по МСФО-базе', null, 0, '', true, 'Настроить'],
      ],
    }
    const format = (value: unknown) => String(value)
    const items = buildWaterfallItems(buildCascadeStages(cascade, frame, format, format), format)

    expect(items.map(({ label, kind }) => ({ label, kind }))).toEqual([
      { label: 'Заработанная премия', kind: 'start' },
      { label: 'Страховые выплаты', kind: 'decrease' },
      { label: 'Расчётный результат по МСФО-базе', kind: 'end' },
    ])
    const closing = items.at(-1)
    expect(closing?.unknown).toBe(true)
    expect(closing?.formattedValue).toBe('—')
    expect(closing?.height).toBe(0)
    // The row it was built from, which is what makes it clickable at all.
    expect(closing?.rowIndex).toBe(2)
    expect(closing?.annotation).toBe('Настроить')
    // A total nobody could compute is not an interim total to explain in a key.
    expect(closing?.checkpoint).toBeUndefined()
    // Neither the amount nor a clipboard value exists, so neither is offered.
    expect(closing?.exactValue).toBeUndefined()
    expect(closing?.rawValue).toBeUndefined()
  })

  it('anchors a cascade whose opening total is unknown at zero', () => {
    const cascade = unknownCascade()
    const frame: Frame = {
      columns: unknownColumns,
      rows: [
        // Degenerate but reachable: with no opening figure the cascade has no
        // origin, so zero is the only anchor left. The known totals after it
        // still stand at their own heights rather than at a delta off nothing.
        ['Заработанная премия', null, 0, '', false, 'Нет данных'],
        ['Страховые выплаты', 180, 20, 'Страховые выплаты', false, ''],
        ['Результат', 150, 30, 'Результат', true, ''],
      ],
    }
    const format = (value: unknown) => String(value)
    const items = buildWaterfallItems(buildCascadeStages(cascade, frame, format, format), format)

    expect(items[0]?.kind).toBe('start')
    expect(items[0]?.unknown).toBe(true)
    expect(items[0]?.formattedValue).toBe('—')
    expect(items[1]?.value).toBe(180)
    expect(items.at(-1)?.value).toBe(150)
  })

  it('gives a synthesized closing total the row and the badge of the stage it repeats', () => {
    const cascade = unknownCascade()
    const frame: Frame = {
      columns: unknownColumns,
      rows: [
        ['Заработанная премия', 200, 0, '', false, ''],
        // A final row carrying a genuine movement: it stays a deduction, and
        // the closing total is synthesized after it.
        ['Результат', 150, 50, 'Страховые выплаты', true, 'Проверено'],
      ],
    }
    const format = (value: unknown) => String(value)
    const items = buildWaterfallItems(buildCascadeStages(cascade, frame, format, format), format)

    expect(items.map(({ kind }) => kind)).toEqual(['start', 'decrease', 'end'])
    // It repeats a real stage, so it carries that stage's row: the badge it
    // shows and the drill it opens now come from the same place.
    expect(items.at(-1)?.rowIndex).toBe(1)
    expect(items.at(-1)?.annotation).toBe('Проверено')
    expect(items.every((item) => item.rowIndex !== undefined)).toBe(true)
  })

  it('marks the unknown column in the plot and says out loud what its dash means', () => {
    const cascade = unknownCascade()
    const frame: Frame = {
      columns: unknownColumns,
      rows: [
        ['Заработанная премия', 200, 0, '', false, ''],
        ['Изменение РЗУ', null, 0, 'Изменение РЗУ', false, 'Нет данных'],
        ['Результат', 150, 30, 'Результат', true, ''],
      ],
    }
    runtime.frame = { data: frame, isLoading: false, isStale: false, error: null, retry: vi.fn() }
    const view = render(<CascadePanel panel={cascade} />)

    const unknownBar = view.container.querySelector('.lens-waterfall-bar[data-unknown="true"]')
    expect(unknownBar).not.toBeNull()
    expect(unknownBar?.getAttribute('data-no-movement')).toBe('true')
    expect(unknownBar?.querySelector('strong')).toHaveTextContent('—')
    expect(unknownBar?.querySelector('.lens-sr-only')).toHaveTextContent('Unavailable')
    // Only the one column: the two known totals draw real bars.
    expect(view.container.querySelectorAll('.lens-waterfall-bar[data-unknown="true"]')).toHaveLength(1)
    expect(view.container.querySelector('.lens-waterfall-annotation')).toHaveTextContent('Нет данных')
  })

  it('prints an em dash in the cascade list where the amount is unknown', () => {
    const cascade = panel('cascade', {
      encoding: {
        label: 'label', value: 'value', cut: 'cut', cutLabel: 'cutLabel',
        final: 'final', annotation: 'annotation',
      },
    })
    const frame: Frame = {
      columns: unknownColumns,
      rows: [
        ['Заработанная премия', 200, 0, '', false, ''],
        ['Изменение РЗУ', null, 0, 'Изменение РЗУ', false, 'Нет данных'],
      ],
    }
    runtime.frame = { data: frame, isLoading: false, isStale: false, error: null, retry: vi.fn() }
    const view = render(<CascadePanel panel={cascade} />)

    const stages = [...view.container.querySelectorAll('.lens-cascade-stage')]
    expect(stages).toHaveLength(2)
    expect(stages[1]).toHaveAttribute('data-unknown', 'true')
    expect(stages[1]?.querySelector('.lens-cascade-stage-label strong')).toHaveTextContent('—')
    // Not «0», and not a figure at all: the stacked projection has to say the
    // same thing the plot does.
    expect(stages[1]?.querySelector('.lens-cascade-stage-label strong')?.textContent).not.toContain('0')
    expect(stages[1]?.querySelector('.lens-cascade-stage-annotation')).toHaveTextContent('Нет данных')
    // An em dash is a glyph; a reader who cannot see it gets the word.
    expect(stages[1]?.querySelector('.lens-sr-only')).toHaveTextContent('Unavailable')
    // The movement into it is equally unknown, so the connector says so too.
    expect(view.container.querySelector('.lens-cascade-connector strong'))
      .toHaveAttribute('data-direction', 'unknown')
  })

  it('prints the gridlines in the axis form and the columns in the exact one', () => {
    const cascade = panel('cascade', {
      encoding: { label: 'label', value: 'value', cut: 'cut', cutLabel: 'cutLabel', final: 'final' },
      presentation: { bridgeLayout: 'waterfall' },
    })
    const frame: Frame = {
      columns: [
        { name: 'label', type: 'string' },
        { name: 'value', type: 'number' },
        { name: 'cut', type: 'number' },
        { name: 'cutLabel', type: 'string' },
        { name: 'final', type: 'bool' },
      ],
      rows: [['Premium', 200, 0, '', false], ['Result', 150, 50, 'Claims', true]],
    }
    const model = buildWaterfallModel(
      buildCascadeStages(cascade, frame, (value) => `${String(value)} UZS`, (value) => String(value)),
      (value) => `${String(value)} UZS`,
      (value) => String(value),
    )

    // Eight gridlines repeating «UZS» is a column of noise; the unit is stated
    // once by the plot instead.
    expect(model.ticks.every((tick) => !tick.label.includes('UZS'))).toBe(true)
    expect(model.items[0]?.formattedValue).toBe('200 UZS')
  })

  it('keeps a genuine sub-unit movement on a small-scale waterfall', () => {
    const cascade = panel('cascade', {
      encoding: { label: 'label', value: 'value', cut: 'cut', cutLabel: 'cutLabel', final: 'final' },
      presentation: { bridgeLayout: 'waterfall' },
    })
    const frame: Frame = {
      columns: [
        { name: 'label', type: 'string' },
        { name: 'value', type: 'number' },
        { name: 'cut', type: 'number' },
        { name: 'cutLabel', type: 'string' },
        { name: 'final', type: 'bool' },
      ],
      rows: [
        ['Opening ratio', 1, 0, '', false],
        ['Closing ratio', 1.25, 0.25, 'Adjustment', true],
      ],
    }
    const format = (value: unknown) => String(value)
    const items = buildWaterfallItems(buildCascadeStages(cascade, frame, format, format), format)

    expect(items.some((item) => item.kind === 'increase' && item.value === 0.25)).toBe(true)
  })

  it('bands a split as a fraction of its own bar and ignores one it cannot hold', () => {
    const cascade = panel('cascade', {
      encoding: {
        label: 'label', value: 'value', cut: 'cut', cutLabel: 'cutLabel', final: 'final',
        split: 'split', splitLabel: 'splitLabel',
      },
      presentation: { bridgeLayout: 'waterfall' },
    })
    const frame: Frame = {
      columns: [
        { name: 'label', type: 'string' },
        { name: 'value', type: 'number' },
        { name: 'cut', type: 'number' },
        { name: 'cutLabel', type: 'string' },
        { name: 'final', type: 'bool' },
        { name: 'split', type: 'number' },
        { name: 'splitLabel', type: 'string' },
      ],
      rows: [
        ['Earned premium', 100, 0, '', false, 0, ''],
        // A quarter of this deduction is of a different kind.
        ['Claims', 60, 40, 'Claims', false, 10, 'above reserve'],
        // A split equal to the whole movement divides nothing; one larger than
        // the movement is not a part of it. Both leave the bar undivided. The
        // equal case is stated in the awkward decimals that actually break a
        // strict comparison: 60 - 50.1 is 9.899999999999999, so a declared 9.9
        // reads as larger than the movement it is a part of.
        ['Acquisition', 50.1, 9.9, 'Acquisition', false, 9.9, 'all of it'],
        ['Operating', 40.1, 10, 'Operating', false, 25, 'more than the bar'],
        ['Result', 40.1, 0, '', true, 0, ''],
      ],
    }
    const format = (value: unknown) => String(value)
    const items = buildWaterfallItems(buildCascadeStages(cascade, frame, format, format), format)
    const byLabel = (label: string) => items.find((item) => item.label === label)

    const claims = byLabel('Claims')
    // A quarter of the movement is a quarter of the bar that draws it, whatever
    // share of the plot that bar happens to occupy.
    expect(claims?.splitHeight).toBeCloseTo(25)
    expect(claims?.splitLabel).toBe('above reserve')
    expect(byLabel('Acquisition')?.splitHeight).toBeUndefined()
    expect(byLabel('Operating')?.splitHeight).toBeUndefined()

    // Every floating bar leaves a balance under it; the totals stand on zero.
    expect(byLabel('Claims')?.underlayHeight).toBeGreaterThan(0)
    expect(byLabel('Earned premium')?.underlayHeight).toBeUndefined()
    expect(byLabel('Result')?.underlayHeight).toBeUndefined()
  })

  it('answers a pointer with a floating split tip and keeps the amount readable without one', async () => {
    const cascade = panel('cascade', {
      encoding: {
        label: 'label', value: 'value', cut: 'cut', cutLabel: 'cutLabel', final: 'final',
        split: 'split', splitLabel: 'splitLabel',
      },
      presentation: { bridgeLayout: 'waterfall' },
    })
    const frame: Frame = {
      columns: [
        { name: 'label', type: 'string' },
        { name: 'value', type: 'number' },
        { name: 'cut', type: 'number' },
        { name: 'cutLabel', type: 'string' },
        { name: 'final', type: 'bool' },
        { name: 'split', type: 'number' },
        { name: 'splitLabel', type: 'string' },
      ],
      rows: [
        ['Earned premium', 100, 0, '', false, 0, ''],
        ['Claims', 60, 40, 'Claims', false, 10, 'above reserve'],
        ['Result', 60, 0, '', true, 0, ''],
      ],
    }
    const format = (value: unknown) => String(value)

    runtime.frame = { data: frame, isLoading: false, isStale: false, error: null, retry: vi.fn() }
    const view = render(<CascadePanel panel={cascade} />)
    // The amount is in the accessibility tree with the bar whether or not the
    // reader has a pointer; only the visible tip waits for one.
    const band = view.container.querySelector('.lens-waterfall-bar-split')
    expect(band?.querySelector('.lens-sr-only')).toHaveTextContent('above reserve 10')
    expect(document.querySelector('.lens-waterfall-tip')).toBeNull()

    const column = band!.closest('.lens-waterfall-column')!
    fireEvent.mouseEnter(column)
    // Floated over the plot in a body-level portal, so no card can clip it.
    const tip = document.querySelector('.lens-waterfall-tip')
    expect(tip).not.toBeNull()
    expect(tip).toHaveTextContent('above reserve')
    expect(tip).toHaveTextContent('10')
    expect(tip?.closest('.lens-root')).not.toBe(view.container.closest('.lens-root'))

    // The card carries buttons, so it outlives the pointer leaving the column
    // by the time it takes to reach them.
    fireEvent.mouseLeave(column)
    await waitFor(() => expect(document.querySelector('.lens-waterfall-tip')).toBeNull())

    const printed = render(
      <WaterfallPlot
        label="Bridge"
        model={buildWaterfallModel(buildCascadeStages(cascade, frame, format, format), format)}
        splitCallout="always"
      />,
    )

    expect(printed.container.querySelector('.lens-waterfall-split-callout')?.getAttribute('data-reveal'))
      .toBe('always')
  })
})
