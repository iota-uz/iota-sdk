import { cleanup, render, waitFor } from '@solidjs/testing-library'
import { afterEach, describe, expect, it } from 'vitest'
import fixture from '../../fixtures/report-card.json'
import exploreFixture from '../../fixtures/explore.json'
import { LensDashboard } from '../LensDashboard'
import { PANEL_KIND_CONFIG_FIELDS, parseDocument, type Action, type DashboardDocument, type Panel, type PanelKind } from '../contract'
import { withoutDrill } from '../panels/drillGate'
import type { HostPanelRendererProps } from '../panels/hostRenderers'
import type { ReportPlacementHost } from './placementHost'

afterEach(() => {
  cleanup()
  document.body.replaceChildren()
})

const navigate = (url: string, field?: string): Action => ({
  kind: 'navigate', urlTemplate: url, payload: {},
  params: field ? [{ name: field, source: { kind: 'field', name: field } }] : [],
})
const crossFilter: Action = { kind: 'cross_filter', params: [], payload: {}, filter: { dimension: 'label', value: { kind: 'field', name: 'label' } } }
const continuing = new Set(['navigate', 'navigate_to_leaf', 'open_drawer', 'cube_drill', 'emit_event'])

function actionKinds(value: unknown): string[] {
  if (Array.isArray(value)) return value.flatMap(actionKinds)
  if (!value || typeof value !== 'object') return []
  const record = value as Record<string, unknown>
  const own = typeof record.kind === 'string' && 'payload' in record ? [record.kind] : []
  return [...own, ...Object.values(record).flatMap(actionKinds)]
}

// False green: stripping only panel.actions would leave a table column, a flow
// stage, a hierarchy row or a relationship end still clickable.
describe('withoutDrill', () => {
  it.each(Object.keys(PANEL_KIND_CONFIG_FIELDS) as PanelKind[])('leaves a %s panel no continuing action or drill tree', (kind) => {
    const panel = {
      id: kind, kind, title: kind, semantics: 'series', frame: 'f', encoding: { label: 'label', value: 'value' }, format: {},
      drillRoot: 'root', actions: [navigate('/a'), crossFilter],
      columns: [{ field: 'label', label: 'Label', action: navigate('/c/{label}', 'label') }],
      metricFlow: { stages: [{ key: 's', label: 'S', role: 'source', action: navigate('/s') }] },
      metricHierarchy: { rows: [{ key: 'r', label: 'R', action: navigate('/r') }] },
      metricRelationship: { source: { key: 'a', label: 'A', action: navigate('/x') }, target: { key: 'b', label: 'B', action: navigate('/y') }, type: 'association' },
    } as unknown as Panel
    expect(actionKinds(panel).filter((action) => continuing.has(action))).not.toHaveLength(0)
    const terminal = withoutDrill(panel)
    expect(terminal.drillRoot).toBeUndefined()
    expect(terminal.terminal).toBe(true)
    expect(actionKinds(terminal).filter((action) => continuing.has(action))).toEqual([])
    expect(terminal.actions).toEqual([crossFilter])
    expect(actionKinds(panel).filter((action) => continuing.has(action)).length).toBeGreaterThan(0)
  })
})

/** The card fixture with a continuing action on its KPI, chart and table. */
function actionableCard(): DashboardDocument {
  const doc = parseDocument(structuredClone(fixture.embedded))
  for (const panel of doc.panels) {
    if (!panel.id.startsWith('revenue::')) continue
    delete panel.terminal
    if (panel.kind === 'stat') panel.actions = [navigate('/detail/kpi')]
    if (panel.kind === 'bar') panel.actions = [navigate('/detail/{label}', 'label')]
    if (panel.kind === 'table') panel.actions = [{ ...navigate('/detail/{label}', 'label'), kind: 'navigate_to_leaf' }]
    if (panel.kind === 'table' && panel.columns) panel.columns = panel.columns.map((column) => column.field === 'label' ? { ...column, action: navigate('/detail/{label}', 'label') } : column)
  }
  return doc
}

/** The explore fixture's drill-root donut placed as a report card. */
function drillRootCard(renderer?: string): DashboardDocument {
  const doc = parseDocument(structuredClone(exploreFixture))
  for (const row of doc.layout.rows) {
    row.heading = undefined
    for (const item of row.panels) item.groups = [{ id: 'margin-card', kind: 'grid', label: 'Margin card', span: 12 }]
  }
  if (renderer) doc.panels[0]!.renderer = { name: renderer }
  return doc
}

const host = (drill: boolean): ReportPlacementHost => ({ capabilities: () => ({ drill, save: false, export: true }) })
const card = (id: string) => document.querySelector<HTMLElement>(`[data-report-placement="${id}"]`)!

describe('drill gate inside a report card', () => {
  // False green: asserting absence alone passes when the fixture carried no affordance; the allowed run proves it did.
  it('removes KPI, chart and table click targets only when the placement cannot drill', async () => {
    render(() => <LensDashboard initialDocument={actionableCard()} reportPlacementHost={host(true)} />)
    await waitFor(() => expect(card('revenue').querySelector('[data-panel-kind="bar"] [data-drillable]')).not.toBeNull())
    expect(card('revenue').querySelector('.lens-metric-cell a[href$="/detail/kpi"]')).not.toBeNull()
    await waitFor(() => expect(card('revenue').querySelector('[data-panel-kind="table"] a[href$="/detail/Online"]')).not.toBeNull())
    cleanup()

    render(() => <LensDashboard initialDocument={actionableCard()} reportPlacementHost={host(false)} />)
    await waitFor(() => expect(card('revenue').querySelector('[data-panel-kind="table"] table')).not.toBeNull())
    await waitFor(() => expect(card('revenue').querySelector('[data-panel-kind="bar"] .lens-chart-host')).not.toBeNull())
    expect(card('revenue').querySelector('[data-drillable]')).toBeNull()
    expect(card('revenue').querySelector('.lens-chart-host-drillable')).toBeNull()
    expect(card('revenue').querySelectorAll('a[href]')).toHaveLength(0)
    expect(card('revenue').textContent).toContain('7,200')
  })

  // False green: a source that allows drill but a context that cannot carry it must still be terminal.
  it('follows the context when it marks drill unsupported', async () => {
    const doc = actionableCard()
    for (const row of doc.layout.rows) for (const item of row.panels) {
      const group = item.groups?.[0]
      if (group?.id === 'revenue' && group.filterContext) group.filterContext = { ...group.filterContext, unsupportedOperations: { drill: [{ dimension: 'product', values: ['A'], origin: { scope: 'dashboard', id: 'd' } }] } }
    }
    render(() => <LensDashboard initialDocument={doc} reportPlacementHost={{}} />)
    await waitFor(() => expect(card('revenue').querySelector('[data-panel-kind="table"] table')).not.toBeNull())
    expect(card('revenue').querySelectorAll('a[href]')).toHaveLength(0)
    expect(card('revenue').querySelector('[data-drillable]')).toBeNull()
  })

  // False green: an explore panel keeps its own breakdown and mark drill unless its drill tree is removed.
  it('renders a drill-root panel terminal when the placement cannot drill', async () => {
    render(() => <LensDashboard initialDocument={drillRootCard()} reportPlacementHost={host(true)} />)
    await waitFor(() => expect(card('margin-card').querySelector('.lens-explore-affordance')).not.toBeNull())
    cleanup()

    render(() => <LensDashboard initialDocument={drillRootCard()} reportPlacementHost={host(false)} />)
    await waitFor(() => expect(card('margin-card').querySelector('[data-panel-id="margin"]')).toHaveTextContent('$1,840,000'))
    expect(card('margin-card').querySelector('[data-drillable]')).toBeNull()
    expect(card('margin-card').querySelector('.lens-explore-affordance')).toBeNull()
    expect(card('margin-card').querySelector('[data-explore-view]')).toBeNull()
  })
})

describe('host renderers on drill-root panels', () => {
  // False green: an explore panel drawing its own donut would satisfy "renders something" for an unregistered view.
  it('draws the registered host view inside the explore panel and the compatibility state otherwise', async () => {
    const Host = (props: HostPanelRendererProps) => <p data-testid="host-view">{props.panel.title}</p>
    render(() => <LensDashboard initialDocument={drillRootCard('legacy-donut')} renderers={{ 'legacy-donut': Host }} />)
    expect(await waitFor(() => document.querySelector('[data-testid="host-view"]'))).not.toBeNull()
    expect(document.querySelector('[data-explore-view]')).not.toBeNull()
    expect(document.querySelector('.lens-chart-host')).toBeNull()
    cleanup()

    render(() => <LensDashboard initialDocument={drillRootCard('legacy-donut')} />)
    await waitFor(() => expect(document.querySelector('[data-panel-state="compatibility"]')).not.toBeNull())
    expect(document.querySelector('.lens-chart-host')).toBeNull()
  })
})
