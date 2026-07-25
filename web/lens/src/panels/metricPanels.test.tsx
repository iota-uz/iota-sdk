import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import type { DashboardDocument, Frame, MetricRelationshipDirection, MetricRelationshipType, Panel } from '../contract'
import { DashboardRuntimeProvider, DocumentProvider } from '../runtime'
import { MetricFlowPanel } from './MetricFlowPanel'
import { MetricHierarchyPanel } from './MetricHierarchyPanel'
import { MetricRelationshipPanel } from './MetricRelationshipPanel'
import { PanelSkeletonBody } from './Skeleton'

afterEach(cleanup)

/** Mirrors panels/parity.test.tsx's documentWith/renderDocument template. */
function documentWith(panels: Panel[], frames: Record<string, Frame>): DashboardDocument {
  return {
    version: '1.0.0',
    snapshotId: 'metric-panels-snapshot',
    meta: { dashboardId: 'metric-panels', title: '', generatedAt: '2026-07-19T00:00:00Z', locale: 'en' },
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

function renderDocument(document: DashboardDocument, children: React.ReactNode, dark = false) {
  return render(
    <div className="lens-root" data-theme={dark ? 'dark' : undefined}>
      <DocumentProvider initialDocument={document}>
        <DashboardRuntimeProvider locale="en">{children}</DashboardRuntimeProvider>
      </DocumentProvider>
    </div>,
  )
}

/* -------------------------------------------------------------------------- */
/* MetricFlowPanel                                                            */
/* -------------------------------------------------------------------------- */

const numberFormat = { kind: 'number' as const, minorUnits: false, precision: 0 }

function flowPanel(overrides: Partial<Panel> = {}): Panel {
  return {
    id: 'flow', kind: 'metric_flow', title: 'Result bridge', semantics: 'reconciliation', frame: 'flow:root',
    encoding: { id: 'key', value: 'value' },
    format: { value: numberFormat },
    actions: [],
    metricFlow: {
      stages: [
        { key: 'premium', label: 'Premium', role: 'input' },
        { key: 'fees', label: 'Fees', role: 'add' },
        { key: 'claims', label: 'Claims', role: 'subtract' },
        { key: 'net', label: 'Net', role: 'result' },
      ],
    },
    ...overrides,
  }
}

function flowFrame(rows: Array<[string, unknown]>): Frame {
  return { columns: [{ name: 'key', type: 'string' }, { name: 'value', type: 'number' }], rows }
}

describe('MetricFlowPanel', () => {
  it('renders a populated 4-stage flow with formatted values, operators and result emphasis', () => {
    const panel = flowPanel()
    const frame = flowFrame([['premium', 1000], ['fees', 200], ['claims', 300], ['net', 900]])
    const { container } = renderDocument(documentWith([panel], { 'flow:root': frame }), <MetricFlowPanel panel={panel} />)

    const stages = container.querySelectorAll('.lens-flow-stage')
    expect(stages).toHaveLength(4)
    expect(container.querySelector('.lens-flow-stage-input .lens-flow-stage-value')?.textContent).toBe('1,000')
    expect(container.querySelector('.lens-flow-stage-add .lens-flow-stage-value')?.textContent).toBe('200')
    expect(container.querySelector('.lens-flow-stage-add .lens-flow-op')?.textContent).toBe('+')
    expect(container.querySelector('.lens-flow-stage-subtract .lens-flow-stage-value')?.textContent).toBe('300')
    expect(container.querySelector('.lens-flow-stage-subtract .lens-flow-op')?.textContent).toBe('−')
    const resultStage = container.querySelector('.lens-flow-stage-result')
    expect(resultStage?.querySelector('.lens-flow-stage-value')?.textContent).toBe('900')
    expect(resultStage).toHaveAttribute('data-result', 'true')
  })

  it('renders a real zero as a formatted "0", not unavailable', () => {
    const panel = flowPanel({ metricFlow: { stages: [{ key: 'zero', label: 'Zero stage', role: 'input' }] } })
    const frame = flowFrame([['zero', 0]])
    const { container } = renderDocument(documentWith([panel], { 'flow:root': frame }), <MetricFlowPanel panel={panel} />)

    expect(container.querySelector('.lens-flow-stage-value')?.textContent).toBe('0')
    expect(container.querySelector('.lens-flow-stage-value')?.textContent).not.toBe('—')
  })

  it('renders a missing key as Unavailable/em dash, never "0"', () => {
    const panel = flowPanel({ metricFlow: { stages: [{ key: 'ghost', label: 'Ghost stage', role: 'input' }] } })
    const frame = flowFrame([['premium', 1000]])
    const { container } = renderDocument(documentWith([panel], { 'flow:root': frame }), <MetricFlowPanel panel={panel} />)

    const value = container.querySelector('.lens-flow-stage-value')
    expect(value?.textContent).toBe('—')
    expect(value?.textContent).not.toContain('0')
    expect(container.querySelector('.lens-flow-stage')).toHaveAttribute('aria-label', expect.stringContaining('Unavailable'))
  })

  it.each([['null', null], ['NaN', Number.NaN]])('renders a %s value as unavailable', (_name, raw) => {
    const panel = flowPanel({ metricFlow: { stages: [{ key: 'bad', label: 'Bad stage', role: 'input' }] } })
    const frame = flowFrame([['bad', raw]])
    const { container } = renderDocument(documentWith([panel], { 'flow:root': frame }), <MetricFlowPanel panel={panel} />)

    expect(container.querySelector('.lens-flow-stage-value')?.textContent).toBe('—')
  })

  it('surfaces a missing configured value column as a panel error state', () => {
    const panel = flowPanel({ encoding: { id: 'key', value: 'amount_not_in_frame' } })
    const frame = flowFrame([['premium', 1000]])
    const { container } = renderDocument(documentWith([panel], { 'flow:root': frame }), <MetricFlowPanel panel={panel} />)

    const error = container.querySelector('.lens-panel-state-error')
    expect(error).not.toBeNull()
    expect(error?.textContent).toContain('amount_not_in_frame')
    expect(container.querySelector('.lens-flow')).toBeNull()
  })

  it('surfaces duplicate frame keys as a panel error state', () => {
    const panel = flowPanel()
    const frame = flowFrame([['premium', 1000], ['premium', 1200]])
    const { container } = renderDocument(documentWith([panel], { 'flow:root': frame }), <MetricFlowPanel panel={panel} />)

    const error = container.querySelector('.lens-panel-state-error')
    expect(error).not.toBeNull()
    expect(error?.textContent).toContain('premium')
  })

  it('renders the declared structure as all-unavailable for an empty frame, distinct from the loading skeleton', () => {
    const panel = flowPanel()
    const frame: Frame = { columns: [{ name: 'key', type: 'string' }, { name: 'value', type: 'number' }], rows: [] }
    const { container } = renderDocument(documentWith([panel], { 'flow:root': frame }), <MetricFlowPanel panel={panel} />)

    expect(container.querySelectorAll('.lens-flow-stage')).toHaveLength(4)
    expect([...container.querySelectorAll('.lens-flow-stage-value')].every((node) => node.textContent === '—')).toBe(true)
    expect(container.querySelector('.lens-panel-state-empty')).toBeNull()
    expect(container.querySelector('.lens-panel-skeleton')).toBeNull()

    cleanup()
    const skeleton = render(<PanelSkeletonBody kind="metric_flow" />)
    expect(skeleton.container.querySelector('.lens-panel-skeleton')).not.toBeNull()
    expect(skeleton.container.querySelector('.lens-flow-stage')).toBeNull()
  })

  it('normalizes a negative movement under "subtract" to a single "+" and absolute magnitude, never "− −"', () => {
    const panel = flowPanel({
      metricFlow: {
        stages: [
          { key: 'claims', label: 'Claims', role: 'subtract' },
        ],
      },
    })
    // A negative frame value under a subtract role means the movement itself
    // reduced the deduction — contribution flips positive, so the rendered
    // operator normalizes to "+" over the absolute magnitude.
    const frame = flowFrame([['claims', -300]])
    const { container } = renderDocument(documentWith([panel], { 'flow:root': frame }), <MetricFlowPanel panel={panel} />)

    expect(container.querySelector('.lens-flow-op')?.textContent).toBe('+')
    expect(container.querySelector('.lens-flow-stage-value')?.textContent).toBe('300')
    expect(container.textContent).not.toContain('−−')
    expect(container.textContent).not.toContain('-300')
  })

  it('keeps the declared subtract operator when its value is unavailable', () => {
    const panel = flowPanel({
      metricFlow: {
        stages: [
          { key: 'claims', label: 'Claims', role: 'subtract', availability: 'unavailable' },
        ],
      },
    })
    const frame = flowFrame([['claims', null]])
    const { container } = renderDocument(documentWith([panel], { 'flow:root': frame }), <MetricFlowPanel panel={panel} />)

    expect(container.querySelector('.lens-flow-op')?.textContent).toBe('−')
    expect(container.querySelector('.lens-flow-stage-value')?.textContent).toBe('—')
  })

  it('shows a reconciliation note beyond tolerance and hides it within tolerance', () => {
    const panelWithReconcile = (tolerance: number) => flowPanel({
      metricFlow: {
        stages: [
          { key: 'premium', label: 'Premium', role: 'input' },
          { key: 'fees', label: 'Fees', role: 'add' },
          { key: 'net', label: 'Net', role: 'result' },
        ],
        reconcile: { tolerance },
      },
    })
    const frame = flowFrame([['premium', 1000], ['fees', 200], ['net', 1210]]) // sum 1200, delta 10

    const mismatched = renderDocument(
      documentWith([panelWithReconcile(1)], { 'flow:root': frame }),
      <MetricFlowPanel panel={panelWithReconcile(1)} />,
    )
    expect(mismatched.container.querySelector('.lens-flow-reconcile')?.textContent).toContain('Difference')

    cleanup()
    const matched = renderDocument(
      documentWith([panelWithReconcile(20)], { 'flow:root': frame }),
      <MetricFlowPanel panel={panelWithReconcile(20)} />,
    )
    expect(matched.container.querySelector('.lens-flow-reconcile')).toBeNull()
  })

  it('never recomputes the result stage: a mismatched result still shows the server value', () => {
    const panel = flowPanel({
      metricFlow: {
        stages: [
          { key: 'premium', label: 'Premium', role: 'input' },
          { key: 'fees', label: 'Fees', role: 'add' },
          { key: 'net', label: 'Net', role: 'result' },
        ],
        reconcile: { tolerance: 1 },
      },
    })
    const frame = flowFrame([['premium', 1000], ['fees', 200], ['net', 999]]) // sum 1200, but server says 999
    const { container } = renderDocument(documentWith([panel], { 'flow:root': frame }), <MetricFlowPanel panel={panel} />)

    expect(container.querySelector('.lens-flow-stage-result .lens-flow-stage-value')?.textContent).toBe('999')
    expect(container.querySelector('.lens-flow-reconcile')?.textContent).toContain('Difference')
  })

  it('renders a navigable stage action', () => {
    const panel = flowPanel({
      metricFlow: {
        stages: [{
          key: 'premium', label: 'Premium', role: 'input',
          action: { kind: 'navigate', urlSource: { kind: 'literal', value: '/metrics/premium' }, params: [], payload: {} },
        }],
      },
    })
    const frame = flowFrame([['premium', 1000]])
    renderDocument(documentWith([panel], { 'flow:root': frame }), <MetricFlowPanel panel={panel} />)

    const link = screen.getByRole('link', { name: 'Open Premium' })
    expect(link).toHaveAttribute('href', expect.stringContaining('/metrics/premium'))
  })

  it('renders <ol role="list"> with a per-stage aria-label containing operator, label and value', () => {
    const panel = flowPanel({
      metricFlow: {
        stages: [
          { key: 'fees', label: 'Fees', role: 'add' },
          { key: 'net', label: 'Net', role: 'result' },
        ],
      },
    })
    const frame = flowFrame([['fees', 200], ['net', 200]])
    const { container } = renderDocument(documentWith([panel], { 'flow:root': frame }), <MetricFlowPanel panel={panel} />)

    const list = container.querySelector('ol.lens-flow')
    expect(list).toHaveAttribute('role', 'list')
    expect(list).toHaveAttribute('aria-label', 'Result bridge stages')
    const items = container.querySelectorAll('li.lens-flow-stage')
    expect(items[0]).toHaveAttribute('aria-label', 'plus Fees, 200')
    expect(items[1]).toHaveAttribute('aria-label', 'equals Net, 200')
  })

  it('renders correctly under the dark theme (smoke)', () => {
    const panel = flowPanel()
    const frame = flowFrame([['premium', 1000], ['fees', 200], ['claims', 300], ['net', 900]])
    const { container } = renderDocument(documentWith([panel], { 'flow:root': frame }), <MetricFlowPanel panel={panel} />, true)

    expect(container.querySelector('.lens-root')).toHaveAttribute('data-theme', 'dark')
    expect(container.querySelectorAll('.lens-flow-stage')).toHaveLength(4)
  })
})

/* -------------------------------------------------------------------------- */
/* MetricHierarchyPanel                                                       */
/* -------------------------------------------------------------------------- */

function hierarchyPanel(overrides: Partial<Panel> = {}): Panel {
  return {
    id: 'hierarchy', kind: 'metric_hierarchy', title: 'Composition', semantics: 'partition', frame: 'hierarchy:root',
    encoding: { id: 'key', value: 'value' },
    format: { value: numberFormat },
    actions: [],
    metricHierarchy: {
      rows: [
        { key: 'total', label: 'Total' },
        { key: 'segment-a', label: 'Segment A', parent: 'total' },
        { key: 'segment-a-1', label: 'Sub A1', parent: 'segment-a' },
        { key: 'segment-a-unalloc', label: 'Unallocated remainder', parent: 'segment-a', unallocated: true },
      ],
    },
    ...overrides,
  }
}

function hierarchyFrame(rows: Array<[string, unknown]>): Frame {
  return { columns: [{ name: 'key', type: 'string' }, { name: 'value', type: 'number' }], rows }
}

describe('MetricHierarchyPanel', () => {
  it('renders a 3-level hierarchy with derived indent, distinct parent styling and the unallocated row', () => {
    const panel = hierarchyPanel()
    const frame = hierarchyFrame([
      ['total', 1000], ['segment-a', 600], ['segment-a-1', 500], ['segment-a-unalloc', 100],
    ])
    const { container } = renderDocument(documentWith([panel], { 'hierarchy:root': frame }), <MetricHierarchyPanel panel={panel} />)

    const rows = container.querySelectorAll('.lens-hierarchy-row')
    expect(rows).toHaveLength(4)
    expect((rows[0] as HTMLElement).style.getPropertyValue('--lens-depth')).toBe('0')
    expect((rows[1] as HTMLElement).style.getPropertyValue('--lens-depth')).toBe('1')
    expect((rows[2] as HTMLElement).style.getPropertyValue('--lens-depth')).toBe('2')
    expect((rows[3] as HTMLElement).style.getPropertyValue('--lens-depth')).toBe('2')

    // total and segment-a are referenced as someone's parent → parent styling.
    expect(rows[0]).toHaveClass('lens-hierarchy-row-parent')
    expect(rows[1]).toHaveClass('lens-hierarchy-row-parent')
    expect(rows[2]).not.toHaveClass('lens-hierarchy-row-parent')

    expect(rows[3]).toHaveClass('lens-hierarchy-row-unallocated')
    expect(screen.getByText('Unallocated')).toBeInTheDocument()
  })

  it('shows "Allocated 100%" when children reconcile to the parent within tolerance', () => {
    const panel = hierarchyPanel({ metricHierarchy: { ...hierarchyPanel().metricHierarchy!, reconcile: { tolerance: 1 } } })
    const frame = hierarchyFrame([
      ['total', 1000], ['segment-a', 600], ['segment-a-1', 500], ['segment-a-unalloc', 100],
    ])
    const { container } = renderDocument(documentWith([panel], { 'hierarchy:root': frame }), <MetricHierarchyPanel panel={panel} />)

    const segmentRow = container.querySelectorAll('.lens-hierarchy-row')[1]
    expect(segmentRow?.querySelector('.lens-hierarchy-reconcile')?.textContent).toBe('Allocated 100%')
    expect(segmentRow?.querySelector('.lens-hierarchy-reconcile')).toHaveClass('lens-hierarchy-reconcile-balanced')
  })

  it('shows "Difference: …" when children do not reconcile to the parent', () => {
    const panel = hierarchyPanel({ metricHierarchy: { ...hierarchyPanel().metricHierarchy!, reconcile: { tolerance: 1 } } })
    const frame = hierarchyFrame([
      ['total', 1000], ['segment-a', 600], ['segment-a-1', 450], ['segment-a-unalloc', 100],
    ])
    const { container } = renderDocument(documentWith([panel], { 'hierarchy:root': frame }), <MetricHierarchyPanel panel={panel} />)

    const segmentRow = container.querySelectorAll('.lens-hierarchy-row')[1]
    const note = segmentRow?.querySelector('.lens-hierarchy-reconcile')
    expect(note?.textContent).toContain('Difference:')
    expect(note).toHaveClass('lens-hierarchy-reconcile-off')
  })

  it('marks aria-current on the single selected row', () => {
    const panel = hierarchyPanel({
      metricHierarchy: {
        rows: [
          { key: 'total', label: 'Total' },
          { key: 'segment-a', label: 'Segment A', parent: 'total', selected: true },
        ],
      },
    })
    const frame = hierarchyFrame([['total', 1000], ['segment-a', 600]])
    const { container } = renderDocument(documentWith([panel], { 'hierarchy:root': frame }), <MetricHierarchyPanel panel={panel} />)

    const rows = container.querySelectorAll('.lens-hierarchy-row')
    expect(rows[0]).not.toHaveAttribute('aria-current')
    expect(rows[1]).toHaveAttribute('aria-current', 'true')
  })

  it('renders a row action as a link', () => {
    const panel = hierarchyPanel({
      metricHierarchy: {
        rows: [{
          key: 'total', label: 'Total',
          action: { kind: 'navigate', urlSource: { kind: 'literal', value: '/metrics/total' }, params: [], payload: {} },
        }],
      },
    })
    const frame = hierarchyFrame([['total', 1000]])
    renderDocument(documentWith([panel], { 'hierarchy:root': frame }), <MetricHierarchyPanel panel={panel} />)

    const link = screen.getByRole('link', { name: /Total/ })
    expect(link).toHaveAttribute('href', expect.stringContaining('/metrics/total'))
  })

  it('renders share through the format pipeline and never recomputes it', () => {
    const panel = hierarchyPanel({
      encoding: { id: 'key', value: 'value', share: 'share' },
      format: { value: numberFormat, share: { kind: 'percent', minorUnits: false, precision: 0 } },
      metricHierarchy: {
        rows: [{ key: 'total', label: 'Total' }],
      },
    })
    const frame: Frame = {
      columns: [{ name: 'key', type: 'string' }, { name: 'value', type: 'number' }, { name: 'share', type: 'number' }],
      rows: [['total', 1000, 60]],
    }
    const { container } = renderDocument(documentWith([panel], { 'hierarchy:root': frame }), <MetricHierarchyPanel panel={panel} />)

    expect(container.querySelector('.lens-hierarchy-share')?.textContent).toBe('60%')
  })
})

/* -------------------------------------------------------------------------- */
/* MetricRelationshipPanel                                                    */
/* -------------------------------------------------------------------------- */

function relationshipPanel(
  type: MetricRelationshipType,
  direction: MetricRelationshipDirection | undefined,
  overrides: Partial<Panel> = {},
): Panel {
  return {
    id: 'relationship', kind: 'metric_relationship', title: 'Link', semantics: 'evidence', frame: 'relationship:root',
    encoding: { id: 'key', value: 'value' },
    format: { value: numberFormat },
    actions: [],
    metricRelationship: {
      type,
      direction,
      source: { key: 'a', label: 'Metric A' },
      target: { key: 'b', label: 'Metric B' },
    },
    ...overrides,
  }
}

function relationshipFrame(rows: Array<[string, unknown]>): Frame {
  return { columns: [{ name: 'key', type: 'string' }, { name: 'value', type: 'number' }], rows }
}

describe('MetricRelationshipPanel', () => {
  it('renders a symmetric ⇄ connector and a visually-hidden sentence for association', () => {
    const panel = relationshipPanel('association', undefined)
    const frame = relationshipFrame([['a', 100], ['b', 100]])
    const { container } = renderDocument(documentWith([panel], { 'relationship:root': frame }), <MetricRelationshipPanel panel={panel} />)

    expect(container.querySelector('.lens-relationship-glyph-h')?.textContent).toBe('⇄')
    expect(container.querySelector('.lens-visually-hidden')?.textContent).toBe('Metric A is economically linked to Metric B')
  })

  it('renders → for source_to_target derivation and ← for target_to_source', () => {
    const forward = relationshipPanel('derivation', 'source_to_target')
    const frame = relationshipFrame([['a', 100], ['b', 100]])
    const forwardView = renderDocument(documentWith([forward], { 'relationship:root': frame }), <MetricRelationshipPanel panel={forward} />)
    expect(forwardView.container.querySelector('.lens-relationship-glyph-h')?.textContent).toBe('→')

    cleanup()
    const backward = relationshipPanel('derivation', 'target_to_source')
    const backwardView = renderDocument(documentWith([backward], { 'relationship:root': frame }), <MetricRelationshipPanel panel={backward} />)
    expect(backwardView.container.querySelector('.lens-relationship-glyph-h')?.textContent).toBe('←')
  })

  it('renders a symmetric ⇄ connector for bidirectional reconciliation', () => {
    const panel = relationshipPanel('reconciliation', 'bidirectional')
    const frame = relationshipFrame([['a', 100], ['b', 100]])
    const { container } = renderDocument(documentWith([panel], { 'relationship:root': frame }), <MetricRelationshipPanel panel={panel} />)

    expect(container.querySelector('.lens-relationship-glyph-h')?.textContent).toBe('⇄')
    expect(container.querySelector('.lens-visually-hidden')?.textContent).toBe('Metric A reconciles with Metric B')
  })

  it('degrades one end independently when only it is unavailable', () => {
    const panel = relationshipPanel('association', undefined)
    const frame = relationshipFrame([['a', 100]]) // 'b' is missing
    const { container } = renderDocument(documentWith([panel], { 'relationship:root': frame }), <MetricRelationshipPanel panel={panel} />)

    expect(container.querySelector('.lens-relationship-end-source .lens-relationship-end-value')?.textContent).toBe('100')
    expect(container.querySelector('.lens-relationship-end-target .lens-relationship-end-value')?.textContent).toBe('—')
  })

  it('renders end actions as links', () => {
    const panel = relationshipPanel('association', undefined, {
      metricRelationship: {
        type: 'association',
        source: {
          key: 'a', label: 'Metric A',
          action: { kind: 'navigate', urlSource: { kind: 'literal', value: '/metrics/a' }, params: [], payload: {} },
        },
        target: { key: 'b', label: 'Metric B' },
      },
    })
    const frame = relationshipFrame([['a', 100], ['b', 100]])
    renderDocument(documentWith([panel], { 'relationship:root': frame }), <MetricRelationshipPanel panel={panel} />)

    const link = screen.getByRole('link', { name: 'Open Metric A' })
    expect(link).toHaveAttribute('href', expect.stringContaining('/metrics/a'))
  })
})
