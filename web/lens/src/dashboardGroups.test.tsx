import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import type { DashboardDocument, Frame, LayoutGroup, LayoutItem, Panel } from './contract'
import { DashboardPanels } from './DashboardPanels'
import type { ChartAdapter } from './charts/adapter'
import { ChartPanel } from './panels/ChartPanel'
import type { PanelRegistry } from './panels'
import { DashboardRuntimeProvider, DocumentProvider } from './runtime'

afterEach(cleanup)

/** Mirrors panels/parity.test.tsx's documentWith/renderDocument template. */
function documentWith(panels: Panel[], frames: Record<string, Frame>, layout: DashboardDocument['layout']): DashboardDocument {
  return {
    version: '1.0.0',
    snapshotId: 'dashboard-groups-snapshot',
    meta: { dashboardId: 'dashboard-groups', title: '', generatedAt: '2026-07-19T00:00:00Z', locale: 'en' },
    layout,
    panels,
    frames,
    drill: { inlineDepth: 0, edges: {} },
    perspectives: [],
    endpoints: {},
    i18n: {},
    theme: { palette: {}, series: {} },
  }
}

function renderDocument(document: DashboardDocument, registry?: PanelRegistry) {
  return render(
    <div className="lens-root">
      <DocumentProvider initialDocument={document}>
        <DashboardRuntimeProvider locale="en"><DashboardPanels registry={registry} /></DashboardRuntimeProvider>
      </DocumentProvider>
    </div>,
  )
}

function statPanel(id: string, title: string): Panel {
  return {
    id, kind: 'stat', title, semantics: 'series', frame: `${id}:root`,
    encoding: { label: 'label', value: 'value' },
    format: { value: { kind: 'number', minorUnits: false, precision: 0 } },
    actions: [],
  }
}

function statFrame(value: number): Frame {
  return { columns: [{ name: 'label', type: 'string' }, { name: 'value', type: 'number' }], rows: [['Value', value]] }
}

/* -------------------------------------------------------------------------- */
/* Nested tabs (Stock / Movement outer, Detail / Summary inner)               */
/* -------------------------------------------------------------------------- */

const outerGroup = (tab: string): LayoutGroup => ({ id: 'outer', kind: 'tabs', span: 12, label: 'Lens', tab })
const innerGroup = (tab: string): LayoutGroup => ({ id: 'inner', kind: 'tabs', span: 12, label: 'Detail level', tab })

function nestedTabsLayout(): DashboardDocument['layout'] {
  const items: LayoutItem[] = [
    { panelId: 'metric-a', span: 12, groups: [outerGroup('Stock'), innerGroup('Detail')] },
    { panelId: 'metric-b', span: 12, groups: [outerGroup('Stock'), innerGroup('Summary')] },
    { panelId: 'metric-c', span: 12, groups: [outerGroup('Movement')] },
  ]
  return { rows: [{ panels: items }] }
}

function renderNestedTabs() {
  const panels = [statPanel('metric-a', 'Metric A'), statPanel('metric-b', 'Metric B'), statPanel('metric-c', 'Metric C')]
  const frames = { 'metric-a:root': statFrame(1), 'metric-b:root': statFrame(2), 'metric-c:root': statFrame(3) }
  return renderDocument(documentWith(panels, frames, nestedTabsLayout()))
}

describe('nested tabs', () => {
  it('renders one outer tablist and one inner tablist', () => {
    renderNestedTabs()

    const tablists = screen.getAllByRole('tablist')
    expect(tablists).toHaveLength(2)
    expect(within(tablists[0]!).getAllByRole('tab').map((tab) => tab.textContent)).toEqual(['Stock', 'Movement'])
    expect(within(tablists[1]!).getAllByRole('tab').map((tab) => tab.textContent)).toEqual(['Detail', 'Summary'])
  })

  it('ArrowRight moves inner focus without activating or touching the outer tab', () => {
    renderNestedTabs()

    const [outerTablist, innerTablist] = screen.getAllByRole('tablist')
    const outerTabs = within(outerTablist!).getAllByRole('tab')
    const innerTabs = within(innerTablist!).getAllByRole('tab')
    expect(outerTabs[0]).toHaveAttribute('aria-selected', 'true')
    expect(innerTabs[0]).toHaveAttribute('aria-selected', 'true')

    fireEvent.keyDown(innerTabs[0]!, { key: 'ArrowRight' })

    // Outer selection is untouched.
    expect(within(screen.getAllByRole('tablist')[0]!).getAllByRole('tab')[0]).toHaveAttribute('aria-selected', 'true')
    // Manual activation keeps the expensive panel mounted until Enter/Space.
    const innerTabsAfter = within(screen.getAllByRole('tablist')[1]!).getAllByRole('tab')
    expect(innerTabsAfter[0]).toHaveAttribute('aria-selected', 'true')
    expect(innerTabsAfter[1]).toHaveAttribute('aria-selected', 'false')
    expect(innerTabsAfter[1]).toHaveFocus()
    expect(screen.getByText('Metric A')).toBeVisible()
    expect(screen.queryByText('Metric B')).toBeNull()

    fireEvent.keyUp(innerTabsAfter[1]!, { key: 'Enter' })
    expect(innerTabsAfter[1]).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByText('Metric B')).toBeVisible()
  })

  it('wraps ArrowLeft/ArrowRight at the ends of the tablist', () => {
    renderNestedTabs()
    const outerTabs = within(screen.getAllByRole('tablist')[0]!).getAllByRole('tab')

    fireEvent.keyDown(outerTabs[0]!, { key: 'ArrowLeft' })
    expect(within(screen.getAllByRole('tablist')[0]!).getAllByRole('tab')[1]).toHaveFocus()
    expect(within(screen.getAllByRole('tablist')[0]!).getAllByRole('tab')[0]).toHaveAttribute('aria-selected', 'true')

    fireEvent.keyDown(within(screen.getAllByRole('tablist')[0]!).getAllByRole('tab')[1]!, { key: 'ArrowRight' })
    expect(within(screen.getAllByRole('tablist')[0]!).getAllByRole('tab')[0]).toHaveFocus()
  })

  it('Home and End select the first and last tab', () => {
    renderNestedTabs()
    const outerTabs = within(screen.getAllByRole('tablist')[0]!).getAllByRole('tab')

    fireEvent.keyDown(outerTabs[0]!, { key: 'End' })
    expect(within(screen.getAllByRole('tablist')[0]!).getAllByRole('tab')[1]).toHaveFocus()
    expect(within(screen.getAllByRole('tablist')[0]!).getAllByRole('tab')[0]).toHaveAttribute('aria-selected', 'true')

    fireEvent.keyDown(within(screen.getAllByRole('tablist')[0]!).getAllByRole('tab')[1]!, { key: 'Home' })
    expect(within(screen.getAllByRole('tablist')[0]!).getAllByRole('tab')[0]).toHaveFocus()
  })

  it('keeps roving tabIndex: the active tab is 0, others are -1', () => {
    renderNestedTabs()
    const outerTabs = within(screen.getAllByRole('tablist')[0]!).getAllByRole('tab')

    expect(outerTabs[0]).toHaveAttribute('tabindex', '0')
    expect(outerTabs[1]).toHaveAttribute('tabindex', '-1')
  })

  it('wires aria-controls/aria-labelledby to real ids on both tab levels', () => {
    renderNestedTabs()

    for (const tablist of screen.getAllByRole('tablist')) {
      for (const tab of within(tablist).getAllByRole('tab')) {
        const controls = tab.getAttribute('aria-controls')
        expect(controls).toBeTruthy()
        const panel = document.getElementById(controls!)
        expect(panel).not.toBeNull()
        expect(panel).toHaveAttribute('aria-labelledby', tab.id)
      }
    }
  })

  it('preserves the inner active tab when the outer tab is switched away and back', () => {
    renderNestedTabs()

    // Select inner "Summary" (Metric B).
    fireEvent.click(screen.getByRole('tab', { name: 'Summary' }))
    expect(screen.getByText('Metric B')).toBeVisible()

    // Switch outer tab away (unmounts the inner group) and back.
    fireEvent.click(screen.getByRole('tab', { name: 'Movement' }))
    expect(screen.queryByRole('tab', { name: 'Summary' })).toBeNull()
    fireEvent.click(screen.getByRole('tab', { name: 'Stock' }))

    const innerTabs = within(screen.getAllByRole('tablist')[1]!).getAllByRole('tab')
    expect(innerTabs.find((tab) => tab.textContent === 'Summary')).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByText('Metric B')).toBeVisible()
  })
})

describe('legend visibility across tabs', () => {
  it('keeps hidden state panel-scoped while preserving it for the originating panel', () => {
    const group = (tab: string): LayoutGroup => ({
      id: 'daily-sales',
      kind: 'tabs',
      span: 12,
      label: 'Daily sales',
      tab,
    })
    const chartPanel = (id: string, title: string): Panel => ({
      id,
      kind: 'bar',
      title,
      semantics: 'series',
      frame: `${id}:root`,
      encoding: { category: 'day', series: 'product', value: 'value' },
      format: { value: { kind: 'number', minorUnits: false, precision: 0 } },
      presentation: { legend: 'below' },
      actions: [],
    })
    const chartFrame = (osago: number, kasko: number): Frame => ({
      columns: [
        { name: 'day', type: 'string' },
        { name: 'product', type: 'string' },
        { name: 'value', type: 'number' },
      ],
      rows: [
        ['31.07', 'ОСАГО', osago],
        ['31.07', 'КАСКО', kasko],
      ],
    })
    const adapter: ChartAdapter = {
      mount: () => ({ update: () => {}, dispose: () => {} }),
    }
    const TestChart = ({ panel }: { panel: Panel }) => <ChartPanel adapter={adapter} panel={panel} />
    const registry: PanelRegistry = { bar: TestChart }
    const revenue = chartPanel('revenue', 'Revenue')
    const count = chartPanel('count', 'Count')

    renderDocument(documentWith(
      [revenue, count],
      {
        'revenue:root': chartFrame(1_000, 500),
        'count:root': chartFrame(10, 5),
      },
      {
        rows: [{
          panels: [
            { panelId: revenue.id, span: 12, groups: [group('Revenue')] },
            { panelId: count.id, span: 12, groups: [group('Count')] },
          ],
        }],
      },
    ), registry)

    fireEvent.click(screen.getByRole('button', { name: /ОСАГО/ }))
    expect(screen.getByRole('button', { name: /ОСАГО/ })).toHaveAttribute('aria-pressed', 'false')

    fireEvent.click(screen.getByRole('tab', { name: 'Count' }))
    expect(screen.getByRole('button', { name: /ОСАГО/ })).toHaveAttribute('aria-pressed', 'true')

    fireEvent.click(screen.getByRole('tab', { name: 'Revenue' }))
    expect(screen.getByRole('button', { name: /ОСАГО/ })).toHaveAttribute('aria-pressed', 'false')
  })
})

/* -------------------------------------------------------------------------- */
/* StatGroup-inside-Tabs + mixed grouped/ungrouped rows                       */
/* -------------------------------------------------------------------------- */

describe('StatGroup inside Tabs', () => {
  it('renders a metrics group inside the active tabpanel', () => {
    const metricsGroup: LayoutGroup = { id: 'ratios', kind: 'metrics', span: 12, label: 'Key ratios', layout: 'columns' }
    const items: LayoutItem[] = [
      { panelId: 'metric-a', span: 6, groups: [outerGroup('Stock'), metricsGroup] },
      { panelId: 'metric-b', span: 6, groups: [outerGroup('Stock'), metricsGroup] },
    ]
    const panels = [statPanel('metric-a', 'Metric A'), statPanel('metric-b', 'Metric B')]
    const frames = { 'metric-a:root': statFrame(1), 'metric-b:root': statFrame(2) }
    const { container } = renderDocument(documentWith(panels, frames, { rows: [{ panels: items }] }))

    const tabpanel = screen.getByRole('tabpanel')
    expect(within(tabpanel).getByText('Key ratios')).toBeInTheDocument()
    expect(container.querySelectorAll('.lens-panel-group-metrics')).toHaveLength(1)
    expect(within(tabpanel).getAllByText(/^[12]$/)).toHaveLength(2)
  })
})

describe('group caption', () => {
  it('renders a group caption once, beneath the group heading', () => {
    const group: LayoutGroup = {
      id: 'ratios', kind: 'metrics', span: 12, label: 'Key ratios', layout: 'columns',
      caption: 'Diagnostic basis, not a competing verdict.',
    }
    const { container } = renderDocument(documentWith(
      [statPanel('metric-a', 'Metric A'), statPanel('metric-b', 'Metric B')],
      { 'metric-a:root': statFrame(1), 'metric-b:root': statFrame(2) },
      { rows: [{ panels: [
        { panelId: 'metric-a', span: 6, groups: [group] },
        { panelId: 'metric-b', span: 6, groups: [group] },
      ] }] },
    ))

    const captions = container.querySelectorAll('.lens-panel-group-metrics > .lens-panel-caption')
    expect(captions).toHaveLength(1)
    expect(captions[0]?.textContent).toBe('Diagnostic basis, not a competing verdict.')
  })
})

describe('metric group context', () => {
  it('renders a metric caption beneath the value', () => {
    const group: LayoutGroup = { id: 'hero', kind: 'metrics', span: 12, layout: 'columns' }
    const panel = { ...statPanel('metric-a', 'Earned premium'), caption: 'Coverage basis · all earning cohorts' }
    renderDocument(documentWith(
      [panel],
      { 'metric-a:root': statFrame(178_880_000_000) },
      { rows: [{ panels: [{ panelId: panel.id, span: 12, groups: [group] }] }] },
    ))

    expect(screen.getByText('Coverage basis · all earning cohorts')).toHaveClass('lens-stat-metric-caption')
  })
})

describe('mixed grouped and ungrouped items in one row', () => {
  it('renders a tabs group and a bare panel side by side', () => {
    const items: LayoutItem[] = [
      { panelId: 'metric-a', span: 6, groups: [outerGroup('Stock')] },
      { panelId: 'metric-b', span: 6, groups: [outerGroup('Movement')] },
      { panelId: 'metric-c', span: 6 },
    ]
    const panels = [statPanel('metric-a', 'Metric A'), statPanel('metric-b', 'Metric B'), statPanel('metric-c', 'Metric C')]
    const frames = { 'metric-a:root': statFrame(1), 'metric-b:root': statFrame(2), 'metric-c:root': statFrame(3) }
    const { container } = renderDocument(documentWith(panels, frames, { rows: [{ panels: items }] }))

    expect(container.querySelectorAll('.lens-panel-group')).toHaveLength(1)
    // The ungrouped panel sits in the same row as a bare grid item, outside any group card.
    const ungrouped = screen.getByText('Metric C').closest('.lens-grid-item')
    expect(ungrouped?.closest('.lens-panel-group')).toBeNull()
    expect(screen.getByRole('tablist')).toBeInTheDocument()
  })
})
