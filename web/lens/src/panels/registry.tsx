import { lazy, Suspense, type ComponentType } from 'react'
import type { Panel, PanelKind } from '../contract'
import { useTranslate } from '../runtime'
import type { CascadePanelProps } from './CascadePanel'
import type { CoveragePanelProps } from './CoveragePanel'
import type { GaugePanelProps } from './GaugePanel'
import type { ChartPanelProps } from './ChartPanel'
import type { MetricFlowPanelProps } from './MetricFlowPanel'
import type { MetricHierarchyPanelProps } from './MetricHierarchyPanel'
import type { MetricRelationshipPanelProps } from './MetricRelationshipPanel'
import type { MapPanelProps } from './MapPanel'
import { StatPanel, type StatPanelProps } from './StatPanel'
import type { TablePanelProps } from './TablePanel'

const PiePanel: ComponentType<ChartPanelProps> = lazy(async () => ({ default: (await import('./ChartPanel')).PiePanel }))
const BarPanel: ComponentType<ChartPanelProps> = lazy(async () => ({ default: (await import('./ChartPanel')).BarPanel }))
const LinePanel: ComponentType<ChartPanelProps> = lazy(async () => ({ default: (await import('./ChartPanel')).LinePanel }))
const DistributionPanel: ComponentType<ChartPanelProps> = lazy(async () => ({ default: (await import('./ChartPanel')).DistributionPanel }))
const CascadePanel: ComponentType<CascadePanelProps> = lazy(async () => ({ default: (await import('./CascadePanel')).CascadePanel }))
const CoveragePanel: ComponentType<CoveragePanelProps> = lazy(async () => ({ default: (await import('./CoveragePanel')).CoveragePanel }))
const GaugePanel: ComponentType<GaugePanelProps> = lazy(async () => ({ default: (await import('./GaugePanel')).GaugePanel }))
const MetricFlowPanel: ComponentType<MetricFlowPanelProps> = lazy(async () => ({ default: (await import('./MetricFlowPanel')).MetricFlowPanel }))
const MetricHierarchyPanel: ComponentType<MetricHierarchyPanelProps> = lazy(async () => ({ default: (await import('./MetricHierarchyPanel')).MetricHierarchyPanel }))
const MetricRelationshipPanel: ComponentType<MetricRelationshipPanelProps> = lazy(async () => ({ default: (await import('./MetricRelationshipPanel')).MetricRelationshipPanel }))
const MapPanel: ComponentType<MapPanelProps> = lazy(async () => ({ default: (await import('./MapPanel')).MapPanel }))
const TablePanel: ComponentType<TablePanelProps> = lazy(async () => ({ default: (await import('./TablePanel')).TablePanel }))

/* eslint-disable react-refresh/only-export-components */

export type PanelComponent = ComponentType<
  | StatPanelProps
  | ChartPanelProps
  | CascadePanelProps
  | TablePanelProps
  | CoveragePanelProps
  | GaugePanelProps
  | MetricFlowPanelProps
  | MetricHierarchyPanelProps
  | MetricRelationshipPanelProps
  | MapPanelProps
>
export type PanelRegistry = Partial<Record<PanelKind, PanelComponent>>

export const UNSUPPORTED = [] as const satisfies readonly PanelKind[]
type UnsupportedKind = (typeof UNSUPPORTED)[number]
type SupportedKind = Exclude<PanelKind, UnsupportedKind>

export const SUPPORTED = {
  stat: StatPanel,
  pie: PiePanel,
  donut: PiePanel,
  radial: PiePanel,
  bar: BarPanel,
  hbar: BarPanel,
  line: LinePanel,
  area: LinePanel,
  cascade: CascadePanel,
  table: TablePanel,
  coverage: CoveragePanel,
  gauge: GaugePanel,
  histogram: DistributionPanel,
  boxplot: DistributionPanel,
  heatmap: DistributionPanel,
  map: MapPanel,
  metric_flow: MetricFlowPanel,
  metric_hierarchy: MetricHierarchyPanel,
  metric_relationship: MetricRelationshipPanel,
} satisfies Record<SupportedKind, PanelComponent>

function unsupportedPartition<const Kinds extends readonly PanelKind[]>(kinds: Kinds) {
  return Object.fromEntries(kinds.map((kind) => [kind, null])) as Record<Kinds[number], null>
}

export const PANEL_KIND_PARTITION = {
  ...SUPPORTED,
  ...unsupportedPartition(UNSUPPORTED),
} satisfies Record<PanelKind, PanelComponent | null>

export const panelRegistry: PanelRegistry = SUPPORTED

export interface RegisteredPanelProps {
  panel: Panel
  registry?: PanelRegistry
}

export function UnsupportedPanel({ panel }: { panel: Panel }) {
  const translate = useTranslate()
  return (
    <section className="lens-panel lens-panel-unsupported" aria-label={panel.title}>
      <header className="lens-panel-header"><h3 className="lens-panel-title">{panel.title}</h3></header>
      <div className="lens-panel-state" role="status">
        {translate('panel.unsupported', 'Unsupported panel: {kind}', { kind: panel.kind })}
      </div>
    </section>
  )
}

export function RegisteredPanel({ panel, registry = panelRegistry }: RegisteredPanelProps) {
  const Component = registry[panel.kind]
  return Component ? (
    <Suspense fallback={<PanelModuleFallback panel={panel} />}>
      <Component panel={panel} />
    </Suspense>
  ) : <UnsupportedPanel panel={panel} />
}

function PanelModuleFallback({ panel }: { panel: Panel }) {
  return (
    <section aria-busy="true" aria-label={panel.title} className="lens-panel lens-panel-loading">
      <div className="lens-panel-skeleton" />
    </section>
  )
}
