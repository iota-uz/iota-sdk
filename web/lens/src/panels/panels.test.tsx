import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
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
  useAxisFormat: () => (value: unknown) => String(value),
  clampedDeltaPercent: () => undefined,
  useTranslate: () => (_key: string, fallback: string, vars?: Record<string, string | number>) => (
    vars ? fallback.replace(/\{(\w+)\}/g, (match, name: string) => (name in vars ? String(vars[name]) : match)) : fallback
  ),
  useDrill: () => ({ drillInto: runtime.drillInto }),
  useDrawer: () => ({ depth: 0, open: vi.fn(), close: vi.fn() }),
  useDashboard: () => ({ document: runtime.document, navigation: runtime.navigation }),
  levelForPath: () => runtime.level,
}))

import { BarPanel, LinePanel, PiePanel, rowIndexForKey } from './ChartPanel'
import { CoveragePanel } from './CoveragePanel'
import { buildCascadeStages, buildWaterfallItems, CascadePanel, waterfallAxisStep } from './CascadePanel'
import { panelRegistry, RegisteredPanel, UNSUPPORTED } from './registry'
import { StatPanel } from './StatPanel'
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
    ...overrides,
  }
}

function state(name: 'loading' | 'empty' | 'error' | 'stale' | 'data'): PanelFrameState {
  const retry = vi.fn()
  if (name === 'loading') return { isLoading: true, isStale: false, error: null, retry }
  if (name === 'empty') return { data: { ...dataFrame, rows: [] }, isLoading: false, isStale: false, error: null, retry }
  if (name === 'error') return { isLoading: false, isStale: false, error: new Error('Frame failed'), retry }
  if (name === 'stale') return { data: dataFrame, isLoading: true, isStale: true, error: null, retry }
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
})

describe.each<PanelKind>(['stat', 'pie', 'donut', 'radial', 'bar', 'hbar', 'line', 'area', 'cascade', 'coverage', 'table'])('%s panel states', (kind) => {
  it.each(['loading', 'empty', 'error', 'stale', 'data'] as const)('renders %s', async (stateName) => {
    runtime.frame = state(stateName)
    const view = renderKind(kind)
    const panelElement = screen.getByLabelText(`${kind} panel`)

    if (stateName === 'loading') {
      // The placeholder mirrors the panel shape and is hidden from assistive
      // technology; aria-busy on the panel carries the state instead.
      expect(panelElement).toHaveAttribute('aria-busy', 'true')
      expect(view.container.querySelector('.lens-panel-skeleton')).not.toBeNull()
    }
    if (stateName === 'empty') expect(screen.getByText('No data')).toBeInTheDocument()
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
    if (stateName === 'data') {
      if (kind === 'stat') expect(screen.getByText('42')).toBeInTheDocument()
      else if (kind === 'cascade') expect(screen.getByRole('list', { name: 'cascade panel stages' })).toBeInTheDocument()
      else if (kind === 'table') expect(screen.getByRole('table')).toBeInTheDocument()
      else if (kind === 'coverage') expect(panelElement.querySelector('.lens-coverage-headline')).not.toBeNull()
      else await waitFor(() => expect(screen.getByText('chart data')).toBeInTheDocument())
    }
    view.unmount()
  })
})

describe('panel total badge', () => {
  it('renders the formatted total in the header when panel.total is present', () => {
    runtime.frame = state('data')
    const view = render(<BarPanel panel={panel('bar', { total: 12345 })} adapter={fakeAdapter()} />)
    const badge = view.container.querySelector('.lens-panel-total')
    // The badge names what it totals, like the legacy renderer's "Итого:".
    expect(badge).toHaveTextContent('Total: 12345')
  })

  it('omits the badge when panel.total is absent', () => {
    runtime.frame = state('data')
    const view = render(<BarPanel panel={panel('bar')} adapter={fakeAdapter()} />)
    expect(view.container.querySelector('.lens-panel-total')).toBeNull()
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

describe('document refetch loading', () => {
  it('shows the skeleton while a date/period refetch is in flight', () => {
    runtime.frame = state('data')
    runtime.refreshing = true
    const view = render(<BarPanel panel={panel('bar')} adapter={fakeAdapter()} />)
    const panelElement = screen.getByLabelText('bar panel')
    expect(panelElement).toHaveAttribute('aria-busy', 'true')
    expect(view.container.querySelector('.lens-panel-skeleton')).not.toBeNull()
    // The stale chart is gone; the panel is unmistakably recomputing.
    expect(screen.queryByText('chart data')).toBeNull()
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
      const supported = panelRegistry[kind] !== undefined
      const unsupported = UNSUPPORTED.some((candidate) => candidate === kind)
      expect(Number(supported) + Number(unsupported), kind).toBe(1)
    }
  })

  it('maps every kind and preserves an explicit fallback for custom registries', () => {
    runtime.frame = state('data')
    const view = render(<RegisteredPanel panel={panel('table')} />)
    expect(screen.getByRole('table')).toBeInTheDocument()
    view.rerender(<RegisteredPanel panel={panel('cascade')} registry={{}} />)
    expect(screen.getByText('Unsupported panel: cascade')).toBeInTheDocument()
  })
})

describe('chart encoding and drill behavior', () => {
  it('adds selection and hover affordances only when DrillRoot is present', async () => {
    runtime.frame = state('data')
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

  it('passes time and series encodings through and tolerates missing optional roles', async () => {
    runtime.frame = state('data')
    const inputs: ChartInput[] = []
    const timePanel = panel('line', { encoding: { category: 'category', value: 'value', series: 'series' } })
    const view = render(<LinePanel panel={timePanel} adapter={fakeAdapter((input) => inputs.push(input))} />)
    await waitFor(() => expect(inputs.length).toBeGreaterThan(0))
    expect(inputs[0]?.frame.columns.find((column) => column.name === 'category')?.type).toBe('time')
    expect(inputs[0]?.encoding.series).toBe('series')

    const sparse = panel('bar', { encoding: { value: 'value' } })
    view.rerender(<BarPanel panel={sparse} adapter={fakeAdapter((input) => inputs.push(input))} />)
    await waitFor(() => expect(inputs.at(-1)?.encoding).toEqual({ value: 'value' }))
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
    await waitFor(() => expect(view.container.querySelectorAll('.lens-chart-legend-item').length).toBe(6))
    const shares = Array.from(view.container.querySelectorAll('.lens-chart-legend-value')).map((node) => node.textContent)
    // Both rings declare 100. Grouped by that denominator the six rows sum to
    // 200%, so nothing reconciles and each ring prints 99.9%. Grouped by ring,
    // each thirds-decomposition adds up to exactly 100.0%.
    expect(shares).toEqual(['33.4%', '33.3%', '33.3%', '33.4%', '33.3%', '33.3%'])
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
    render(<PiePanel panel={pie} adapter={fakeAdapter((input) => inputs.push(input))} />)
    await waitFor(() => expect(inputs.length).toBeGreaterThan(0))
    fireEvent.click(screen.getByText('Alpha'))
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
    await waitFor(() => expect(view.container.querySelectorAll('.lens-chart-legend-mark').length).toBe(2))
    const marks = Array.from(view.container.querySelectorAll<HTMLElement>('.lens-chart-legend-mark'))
    expect(marks.map((mark) => mark.style.background)).toEqual(['rgb(17, 17, 17)', 'rgb(34, 34, 34)'])
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

    await waitFor(() => expect(view.container.querySelectorAll('.lens-chart-legend-item')).toHaveLength(2))
    expect(screen.getByText('Written premium')).toBeInTheDocument()
    expect(screen.getByText('Earned premium')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /Earned premium/ }))
    await waitFor(() => expect(inputs.at(-1)?.frame.rows).toEqual([
      ['2025', 'Written premium', 100],
      ['2026', 'Written premium', 120],
    ]))
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

    await waitFor(() => expect(view.container.querySelectorAll('.lens-chart-legend-item')).toHaveLength(2))
    expect(inputs.at(-1)?.seriesColor?.('Paid', 1)).toBe('#d97706')

    fireEvent.click(screen.getByRole('button', { name: /Claimed/ }))
    await waitFor(() => expect(inputs.at(-1)?.frame.rows).toHaveLength(2))
    // The chart now hands index 0 — the only series it can see — and must
    // still be told amber, the colour the legend beside it is still printing.
    expect(inputs.at(-1)?.seriesColor?.('Paid', 0)).toBe('#d97706')
    const marks = Array.from(view.container.querySelectorAll<HTMLElement>('.lens-chart-legend-mark'))
    expect(marks.at(-1)?.style.background).toBe('rgb(217, 119, 6)')
  })

  it('renders the panel title once when the stat label would duplicate it', () => {
    runtime.frame = state('data')
    render(<StatPanel panel={panel('stat', { encoding: { value: 'value' } })} />)
    expect(screen.getAllByText('stat panel')).toHaveLength(1)
    expect(screen.getByText('42')).toBeInTheDocument()
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
    // Its tooltip names the segment and its exact value.
    expect(segments[1]).toHaveAttribute('title', 'Beta: 3')
    expect(view.container.querySelectorAll('.lens-coverage-legend-row')).toHaveLength(2)
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

  it('makes each segment and legend row its own link for a row-scoped action', () => {
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
    expect(segmentLinks).toHaveLength(2)
    expect(segmentLinks[0]?.getAttribute('href')).toContain('/drill/a')
    const legendLinks = view.container.querySelectorAll('a.lens-coverage-legend-link')
    expect(legendLinks[1]?.getAttribute('href')).toContain('/drill/b')
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
})
