import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { DashboardDocument, Frame, Panel, PanelKind } from '../contract'
import type { PanelFrameState } from '../runtime'
import type { ChartAdapter, ChartInput } from '../charts/adapter'

const runtime = vi.hoisted(() => ({
  frame: undefined as PanelFrameState | undefined,
  drillInto: vi.fn(),
  document: { theme: { palette: {}, series: {} }, drill: { edges: {}, inlineDepth: 0 } } as DashboardDocument,
  navigation: { path: [], history: [] },
}))

vi.mock('../runtime', () => ({
  usePanelFrame: () => runtime.frame,
  usePanelPagination: () => ({ loadPage: vi.fn() }),
  useExport: () => ({ status: 'idle', available: false, run: vi.fn() }),
  useFormat: () => (value: unknown) => {
    if (value === null || value === undefined) return '—'
    if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint') {
      return String(value)
    }
    return '—'
  },
  useAxisFormat: () => (value: unknown) => String(value),
  useTranslate: () => (_key: string, fallback: string) => fallback,
  useDrill: () => ({ drillInto: runtime.drillInto }),
  useDashboard: () => ({ document: runtime.document, navigation: runtime.navigation }),
}))

import { BarPanel, LinePanel, PiePanel } from './ChartPanel'
import { buildCascadeStages, CascadePanel } from './CascadePanel'
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
  if (kind === 'pie' || kind === 'donut') return render(<PiePanel panel={value} adapter={fakeAdapter()} />)
  if (kind === 'bar' || kind === 'hbar') return render(<BarPanel panel={value} adapter={fakeAdapter()} />)
  return render(<LinePanel panel={value} adapter={fakeAdapter()} />)
}

afterEach(() => {
  cleanup()
  runtime.drillInto.mockReset()
})

describe.each<PanelKind>(['stat', 'pie', 'donut', 'bar', 'hbar', 'line', 'area', 'cascade', 'coverage', 'table'])('%s panel states', (kind) => {
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
      expect(panelElement).toHaveAttribute('data-stale', 'true')
      expect(screen.getByText('Updating')).toBeInTheDocument()
    }
    if (stateName === 'data') {
      if (kind === 'stat') expect(screen.getByText('42')).toBeInTheDocument()
      else if (kind === 'cascade') expect(screen.getByRole('list', { name: 'cascade panel stages' })).toBeInTheDocument()
      else if (kind === 'table') expect(screen.getByRole('table')).toBeInTheDocument()
      else await waitFor(() => expect(screen.getByText('chart data')).toBeInTheDocument())
    }
    view.unmount()
  })
})

describe('panel total badge', () => {
  it('renders the formatted total in the header when panel.total is present', () => {
    runtime.frame = state('data')
    render(<BarPanel panel={panel('bar', { total: 12345 })} adapter={fakeAdapter()} />)
    const badge = screen.getByTitle('Total')
    expect(badge).toHaveTextContent('12345')
  })

  it('omits the badge when panel.total is absent', () => {
    runtime.frame = state('data')
    render(<BarPanel panel={panel('bar')} adapter={fakeAdapter()} />)
    expect(screen.queryByTitle('Total')).toBeNull()
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
      hbar: true,
      line: true,
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
    expect(screen.getByLabelText('pie panel pie chart')).not.toHaveAttribute('data-drillable')
    fireEvent.click(screen.getByText('chart data'))
    expect(runtime.drillInto).not.toHaveBeenCalled()

    view.rerender(<PiePanel panel={panel('pie', { drillRoot: 'root' })} adapter={adapter} />)
    expect(screen.getByLabelText('pie panel pie chart')).toHaveAttribute('data-drillable', 'true')
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

  it('renders the panel title once when the stat label would duplicate it', () => {
    runtime.frame = state('data')
    render(<StatPanel panel={panel('stat', { encoding: { value: 'value' } })} />)
    expect(screen.getAllByText('stat panel')).toHaveLength(1)
    expect(screen.getByText('42')).toBeInTheDocument()
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
})
