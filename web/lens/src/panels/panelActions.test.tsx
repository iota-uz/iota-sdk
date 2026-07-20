import { cleanup, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Action, DashboardDocument, Frame, Panel } from '../contract'
import type { ChartAdapter } from '../charts/adapter'
import { CoveragePanel, ChartPanel, StatMetric, StatPanel } from './index'
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
  it('links each segment and legend row when the action reads a row field', () => {
    const panel = coveragePanel([navigate('/claims/{bucket}', [
      { name: 'bucket', source: { kind: 'field', name: 'id' } },
    ])])
    const { container } = renderPanel(
      documentWith([panel], { 'coverage:root': coverageFrame }),
      <CoveragePanel panel={panel} />,
    )

    const legend = [...container.querySelectorAll<HTMLAnchorElement>('.lens-coverage-legend-link')]
    expect(legend.map((link) => new URL(link.href).pathname)).toEqual(['/claims/within', '/claims/above'])
    expect(container.querySelectorAll('.lens-coverage-track-segment-link')).toHaveLength(2)
    // A row-scoped action belongs to the segments, never to the whole card.
    expect(container.querySelector('.lens-card-link')).toBeNull()
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
  function renderChart(panel: Panel) {
    let select: ((key: string) => void) | undefined
    const adapter: ChartAdapter = {
      mount: (_element, _input, events) => {
        select = (key) => events.onSelect(key)
        return { update: () => {}, dispose: () => {} }
      },
    }
    const view = renderPanel(
      documentWith([panel], { 'chart:root': chartFrame }),
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

  it('leaves a chart without a drill root or an action inert', async () => {
    const assign = vi.mocked(navigateTo)
    const { container, activate } = renderChart(chartPanel([]))

    await waitFor(() => expect(container.querySelector('.lens-chart-host')).not.toBeNull())
    expect(container.querySelector('[data-drillable]')).toBeNull()
    activate('broker')
    expect(assign).not.toHaveBeenCalled()
  })
})

describe('chart tooltips', () => {
  it('renders at body level, unconfined, above the expanded-panel dialog', async () => {
    const { tooltipChrome, tooltipZIndex } = await import('../charts/echarts/options')
    const chrome = tooltipChrome({
      card: '#fff', text: '#111', mutedText: '#666', border: '#eee', divider: '#eee',
      selectedBorder: '#000', fontFamily: 'Inter', colors: [], seriesColor: () => undefined,
    })

    expect(chrome.appendTo).toBe('body')
    expect(chrome.confine).toBe(false)
    // The expanded-panel overlay sits at 2147483000.
    expect(tooltipZIndex).toBeGreaterThan(2147483000)
    expect(chrome.extraCssText).toContain(`z-index: ${tooltipZIndex}`)
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
