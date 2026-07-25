import type { Story } from '@ladle/react'
import type { DashboardDocument, Frame, Panel } from './contract'
import { DashboardPanels } from './DashboardPanels'
import { QualityChip, type QualityChipProps } from './panels'
import { DashboardRuntimeProvider, DocumentProvider } from './runtime'
import './styles.css'

/**
 * `MetricComposition` demonstrates the §23.2 one-parent/one-active-lens model
 * with business-neutral fixtures: a MetricFlow reads as the stable parent
 * metric, and the Tabs below it swap the active decomposition lens
 * (association / composition / breakdown) without ever touching the flow's
 * own values. There is no new "lens switcher" component — Tabs plus the three
 * metric primitives (MetricFlow, MetricHierarchy, MetricRelationship) prove
 * the composition on their own.
 */

const money = { kind: 'money', currency: 'USD', minorUnits: false, precision: 0 } as const

function storyDocument(panels: Panel[], frames: Record<string, Frame>, layout: DashboardDocument['layout']): DashboardDocument {
  return {
    version: '1.0.0',
    snapshotId: 'metric-composition-snapshot',
    meta: { dashboardId: 'metric-composition', title: 'Metric composition', generatedAt: '2026-07-19T00:00:00Z', locale: 'en' },
    layout,
    panels,
    frames,
    drill: { inlineDepth: 0, edges: {} },
    perspectives: [],
    endpoints: {},
    i18n: {},
    theme: { palette: { accent: '#2563eb' }, series: {} },
  }
}

function Runtime({ theme, doc }: { theme: 'light' | 'dark'; doc: DashboardDocument }) {
  return (
    <div className="lens-root" data-theme={theme}>
      <DocumentProvider initialDocument={doc}>
        <DashboardRuntimeProvider locale="en"><DashboardPanels /></DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  )
}

/* -------------------------------------------------------------------------- */
/* Full: the target composition.                                              */
/* -------------------------------------------------------------------------- */

const flowPanel: Panel = {
  id: 'composition-flow',
  kind: 'metric_flow',
  title: 'Result bridge',
  semantics: 'reconciliation',
  frame: 'composition-flow:frame',
  encoding: { id: 'id', value: 'value' },
  format: { value: money },
  metricFlow: {
    stages: [
      {
        key: 'flow/opening', label: 'Opening amount', role: 'input', confidence: 'verified',
        action: { kind: 'navigate', method: 'GET', urlTemplate: '/metrics/opening', params: [], payload: {} },
      },
      // No frame row for this key: the missing-value path.
      { key: 'flow/recognized', label: 'Recognized', role: 'add', confidence: 'calculated' },
      { key: 'flow/remaining', label: 'Remaining', role: 'subtract', confidence: 'requires_reconciliation' },
      { key: 'flow/closing', label: 'Closing amount', role: 'result', caption: 'Reconciled against opening balance' },
    ],
    reconcile: { tolerance: 0 },
  },
  actions: [],
}

const flowFrame: Frame = {
  columns: [{ name: 'id', type: 'string' }, { name: 'value', type: 'number' }],
  rows: [['flow/opening', 800000], ['flow/remaining', 120000], ['flow/closing', 680000]],
}

const associationMetric: Panel = {
  id: 'tab1-metric',
  kind: 'stat',
  title: 'Primary metric',
  semantics: 'series',
  frame: 'tab1-metric:frame',
  encoding: { label: 'label', value: 'value' },
  format: { value: money },
  actions: [],
}

const associationMetricFrame: Frame = {
  columns: [{ name: 'label', type: 'string' }, { name: 'value', type: 'number' }],
  rows: [['Primary metric', 342000]],
}

const associationRelationship: Panel = {
  id: 'tab1-relationship',
  kind: 'metric_relationship',
  title: 'Economic link',
  semantics: 'series',
  frame: 'tab1-relationship:frame',
  encoding: { id: 'id', value: 'value' },
  format: { value: money },
  metricRelationship: {
    source: { key: 'r1/source', label: 'Primary metric', confidence: 'verified' },
    target: { key: 'r1/target', label: 'Reference metric', confidence: 'calculated' },
    type: 'association',
    direction: 'bidirectional',
  },
  actions: [],
}

const associationRelationshipFrame: Frame = {
  columns: [{ name: 'id', type: 'string' }, { name: 'value', type: 'number' }],
  rows: [['r1/source', 342000], ['r1/target', 342000]],
}

function hierarchyLensPanel(id: string, title: string): Panel {
  return {
    id,
    kind: 'metric_hierarchy',
    title,
    semantics: 'partition',
    frame: `${id}:frame`,
    encoding: { id: 'id', value: 'value' },
    format: { value: money },
    metricHierarchy: {
      rows: [
        { key: `${id}/root`, label: 'Category', confidence: 'verified' },
        { key: `${id}/a`, label: 'Category A', parent: `${id}/root`, confidence: 'calculated' },
        { key: `${id}/b`, label: 'Category B', parent: `${id}/root`, confidence: 'calculated' },
        { key: `${id}/c`, label: 'Category C', parent: `${id}/root`, confidence: 'proxy' },
      ],
    },
    actions: [],
  }
}

function hierarchyLensFrame(id: string, root: number): Frame {
  const third = Math.round(root / 3)
  return {
    columns: [{ name: 'id', type: 'string' }, { name: 'value', type: 'number' }],
    rows: [[`${id}/root`, root], [`${id}/a`, third], [`${id}/b`, third], [`${id}/c`, root - 2 * third]],
  }
}

const stockHierarchy = hierarchyLensPanel('stock-hierarchy', 'Stock')
const stockHierarchyFrame = hierarchyLensFrame('stock-hierarchy', 900000)
const movementHierarchy = hierarchyLensPanel('movement-hierarchy', 'Movement')
const movementHierarchyFrame = hierarchyLensFrame('movement-hierarchy', 450000)

const compositionRelationship: Panel = {
  id: 'tab2-relationship',
  kind: 'metric_relationship',
  title: 'Stock/movement reconciliation',
  semantics: 'reconciliation',
  frame: 'tab2-relationship:frame',
  encoding: { id: 'id', value: 'value' },
  format: { value: money },
  metricRelationship: {
    source: { key: 'r2/source', label: 'Stock total', confidence: 'verified' },
    target: { key: 'r2/target', label: 'Movement total', confidence: 'calculated' },
    type: 'reconciliation',
    direction: 'bidirectional',
  },
  actions: [],
}

const compositionRelationshipFrame: Frame = {
  columns: [{ name: 'id', type: 'string' }, { name: 'value', type: 'number' }],
  rows: [['r2/source', 900000], ['r2/target', 450000]],
}

const breakdownHierarchy: Panel = {
  id: 'breakdown-hierarchy',
  kind: 'metric_hierarchy',
  title: 'Breakdown',
  semantics: 'partition',
  frame: 'breakdown-hierarchy:frame',
  encoding: { id: 'id', value: 'value' },
  format: { value: money },
  metricHierarchy: {
    rows: [
      { key: 'breakdown/root', label: 'Category', confidence: 'verified' },
      { key: 'breakdown/a', label: 'Category A', parent: 'breakdown/root', confidence: 'calculated' },
      { key: 'breakdown/b', label: 'Category B', parent: 'breakdown/root', confidence: 'calculated' },
      { key: 'breakdown/a1', label: 'Category A1', parent: 'breakdown/a', confidence: 'verified' },
      { key: 'breakdown/a2', label: 'Category A2', parent: 'breakdown/a', confidence: 'proxy' },
      {
        key: 'breakdown/a-unallocated', label: 'Unallocated', parent: 'breakdown/a', unallocated: true,
        confidence: 'requires_reconciliation',
      },
    ],
    reconcile: { tolerance: 0 },
  },
  actions: [],
}

const breakdownHierarchyFrame: Frame = {
  columns: [{ name: 'id', type: 'string' }, { name: 'value', type: 'number' }],
  rows: [
    ['breakdown/root', 600000],
    ['breakdown/a', 350000],
    ['breakdown/b', 250000],
    ['breakdown/a1', 150000],
    ['breakdown/a2', 150000],
    ['breakdown/a-unallocated', 50000],
  ],
}

const OUTER_TABS_GROUP_ID = 'composition-tabs'
const INNER_TABS_GROUP_ID = 'stock-movement-tabs'

function outerGroup(tab: string) {
  return { id: OUTER_TABS_GROUP_ID, kind: 'tabs' as const, span: 12, tab }
}

function innerGroup(tab: string) {
  return { id: INNER_TABS_GROUP_ID, kind: 'tabs' as const, span: 12, tab }
}

/**
 * The document a Go builder would emit for this composition: single-level tab
 * items carry only the legacy singular `group`; the Stock/Movement hierarchies
 * are two chain levels deep, so they carry `groups` outermost-to-innermost.
 */
function fullDocument(): DashboardDocument {
  return storyDocument(
    [
      flowPanel, associationMetric, associationRelationship,
      stockHierarchy, movementHierarchy, compositionRelationship, breakdownHierarchy,
    ],
    {
      'composition-flow:frame': flowFrame,
      'tab1-metric:frame': associationMetricFrame,
      'tab1-relationship:frame': associationRelationshipFrame,
      'stock-hierarchy:frame': stockHierarchyFrame,
      'movement-hierarchy:frame': movementHierarchyFrame,
      'tab2-relationship:frame': compositionRelationshipFrame,
      'breakdown-hierarchy:frame': breakdownHierarchyFrame,
    },
    {
      rows: [
        { panels: [{ panelId: 'composition-flow', span: 12 }] },
        {
          heading: 'Composition',
          panels: [
            { panelId: 'tab1-metric', span: 6, group: outerGroup('Association') },
            { panelId: 'tab1-relationship', span: 6, group: outerGroup('Association') },
            { panelId: 'stock-hierarchy', span: 6, groups: [outerGroup('Composition'), innerGroup('Stock')] },
            { panelId: 'movement-hierarchy', span: 6, groups: [outerGroup('Composition'), innerGroup('Movement')] },
            { panelId: 'tab2-relationship', span: 12, group: outerGroup('Composition') },
            { panelId: 'breakdown-hierarchy', span: 12, group: outerGroup('Breakdown') },
          ],
        },
      ],
    },
  )
}

export const Full: Story = () => <Runtime doc={fullDocument()} theme="light" />
export const FullDark: Story = () => <Runtime doc={fullDocument()} theme="dark" />

/* -------------------------------------------------------------------------- */
/* Narrow: same primitives at small spans, plus a fixed 320px hierarchy.      */
/* -------------------------------------------------------------------------- */

const narrowFlow: Panel = { ...flowPanel, id: 'narrow-flow', frame: 'narrow-flow:frame' }
const narrowHierarchy: Panel = { ...breakdownHierarchy, id: 'narrow-hierarchy', frame: 'narrow-hierarchy:frame' }
const narrowRelationship: Panel = { ...associationRelationship, id: 'narrow-relationship', frame: 'narrow-relationship:frame' }

function narrowDocument(): DashboardDocument {
  return storyDocument(
    [narrowFlow, narrowHierarchy, narrowRelationship],
    {
      'narrow-flow:frame': flowFrame,
      'narrow-hierarchy:frame': breakdownHierarchyFrame,
      'narrow-relationship:frame': associationRelationshipFrame,
    },
    {
      rows: [{
        heading: 'Narrow',
        panels: [
          { panelId: 'narrow-flow', span: 4 },
          { panelId: 'narrow-hierarchy', span: 4 },
          { panelId: 'narrow-relationship', span: 4 },
        ],
      }],
    },
  )
}

const fixed320Hierarchy: Panel = { ...breakdownHierarchy, id: 'fixed-320-hierarchy', frame: 'fixed-320-hierarchy:frame' }
const fixed320HierarchyFrame = breakdownHierarchyFrame

export const Narrow: Story = () => (
  <div className="lens-root">
    <DocumentProvider initialDocument={narrowDocument()}>
      <DashboardRuntimeProvider locale="en"><DashboardPanels /></DashboardRuntimeProvider>
    </DocumentProvider>
    {/* A fixed 320px container exercises the hierarchy panel's narrowest
        container-query breakpoint directly, independent of the grid span. */}
    <div style={{ width: 320 }}>
      <DocumentProvider
        initialDocument={storyDocument([fixed320Hierarchy], { 'fixed-320-hierarchy:frame': fixed320HierarchyFrame }, {
          rows: [{ panels: [{ panelId: 'fixed-320-hierarchy', span: 12 }] }],
        })}
      >
        <DashboardRuntimeProvider locale="en"><DashboardPanels /></DashboardRuntimeProvider>
      </DocumentProvider>
    </div>
  </div>
)

/* -------------------------------------------------------------------------- */
/* QualityChips: all 8 statuses, light + dark side by side.                   */
/* -------------------------------------------------------------------------- */

const qualitySpecimens: Array<{ name: string; props: QualityChipProps }> = [
  { name: 'confidence.verified', props: { confidence: 'verified' } },
  { name: 'confidence.calculated', props: { confidence: 'calculated' } },
  { name: 'confidence.proxy', props: { confidence: 'proxy' } },
  { name: 'confidence.requires_reconciliation', props: { confidence: 'requires_reconciliation' } },
  { name: 'availability.available', props: { availability: 'available' } },
  { name: 'availability.config_required', props: { availability: 'config_required' } },
  { name: 'availability.empty_source', props: { availability: 'empty_source' } },
  { name: 'availability.unavailable', props: { availability: 'unavailable' } },
]

function QualityChipSpecimens({ theme }: { theme: 'light' | 'dark' }) {
  return (
    <div className="lens-root" data-theme={theme} style={{ width: 420 }}>
      <div className="lens-panel-grid">
        <div className="lens-grid-item" style={{ '--lens-panel-span': 12 } as React.CSSProperties}>
          <section className="lens-panel">
            <header className="lens-panel-header"><h3 className="lens-panel-title">Quality statuses</h3></header>
            <div className="lens-panel-body">
              {/* Each row pairs the translation key (documentation) with the
                  chip it resolves to; `availability.available` renders no
                  chip at all — a normal element defers to the confidence
                  axis, or shows nothing when neither axis says anything. */}
              <ul style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem', margin: 0, padding: 0, listStyle: 'none' }}>
                {qualitySpecimens.map(({ name, props }) => (
                  <li key={name} style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                    <span style={{ flex: '0 0 220px', fontSize: '0.75rem' }} className="lens-text-muted">{name}</span>
                    <QualityChip {...props} />
                  </li>
                ))}
              </ul>
            </div>
          </section>
        </div>
      </div>
    </div>
  )
}

export const QualityChips: Story = () => (
  <div style={{ display: 'flex', gap: '1rem' }}>
    <QualityChipSpecimens theme="light" />
    <QualityChipSpecimens theme="dark" />
  </div>
)

/* -------------------------------------------------------------------------- */
/* RelationshipVariants: association / derivation both directions /          */
/* reconciliation, incl. one end unavailable.                                 */
/* -------------------------------------------------------------------------- */

function relationshipVariant(
  id: string,
  title: string,
  type: 'association' | 'derivation' | 'reconciliation',
  direction: 'bidirectional' | 'source_to_target' | 'target_to_source',
  targetAvailable: boolean,
): { panel: Panel; frame: Frame } {
  const panel: Panel = {
    id,
    kind: 'metric_relationship',
    title,
    semantics: type === 'association' ? 'series' : 'reconciliation',
    frame: `${id}:frame`,
    encoding: { id: 'id', value: 'value' },
    format: { value: money },
    metricRelationship: {
      source: { key: `${id}/source`, label: 'Source metric', confidence: 'verified' },
      target: { key: `${id}/target`, label: 'Target metric', confidence: 'calculated' },
      type,
      direction,
    },
    actions: [],
  }
  const frame: Frame = {
    columns: [{ name: 'id', type: 'string' }, { name: 'value', type: 'number' }],
    rows: targetAvailable
      ? [[`${id}/source`, 220000], [`${id}/target`, 220000]]
      // The target key is declared but has no row: an unavailable end.
      : [[`${id}/source`, 220000]],
  }
  return { panel, frame }
}

const relationshipVariants = [
  relationshipVariant('rv-association', 'Association', 'association', 'bidirectional', true),
  relationshipVariant('rv-derivation-forward', 'Derivation, source to target', 'derivation', 'source_to_target', true),
  relationshipVariant('rv-derivation-backward', 'Derivation, target to source', 'derivation', 'target_to_source', true),
  relationshipVariant('rv-reconciliation-gap', 'Reconciliation, target unavailable', 'reconciliation', 'bidirectional', false),
]

export const RelationshipVariants: Story = () => {
  const doc = storyDocument(
    relationshipVariants.map(({ panel }) => panel),
    Object.fromEntries(relationshipVariants.map(({ panel, frame }) => [panel.frame, frame])),
    {
      rows: [{
        heading: 'Relationship variants',
        panels: relationshipVariants.map(({ panel }) => ({ panelId: panel.id, span: 6 })),
      }],
    },
  )
  return <Runtime doc={doc} theme="light" />
}
