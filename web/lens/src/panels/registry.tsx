import { Component, lazy, Suspense, type ComponentType, type ErrorInfo, type ReactNode } from 'react'
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

const ChartPanel: ComponentType<ChartPanelProps> = lazy(async () => ({ default: (await import('./ChartPanel')).ChartPanel }))
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

export const SUPPORTED = {
  stat: StatPanel,
  pie: ChartPanel,
  donut: ChartPanel,
  radial: ChartPanel,
  bar: ChartPanel,
  hbar: ChartPanel,
  line: ChartPanel,
  area: ChartPanel,
  cascade: CascadePanel,
  table: TablePanel,
  coverage: CoveragePanel,
  gauge: GaugePanel,
  histogram: ChartPanel,
  boxplot: ChartPanel,
  heatmap: ChartPanel,
  map: MapPanel,
  metric_flow: MetricFlowPanel,
  metric_hierarchy: MetricHierarchyPanel,
  metric_relationship: MetricRelationshipPanel,
} satisfies Record<PanelKind, PanelComponent>

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
  const translate = useTranslate()
  const Component = registry[panel.kind]
  return Component ? (
    <PanelErrorBoundary
      fallback={translate('panel.error', 'This panel could not be rendered.')}
      panel={panel}
      retryLabel={translate('panel.retry', 'Retry')}
    >
      <Suspense fallback={<PanelModuleFallback panel={panel} />}>
        <Component panel={panel} />
      </Suspense>
    </PanelErrorBoundary>
  ) : <UnsupportedPanel panel={panel} />
}

class PanelErrorBoundary extends Component<{
  children: ReactNode
  fallback: string
  panel: Panel
  retryLabel: string
}, { failed: boolean }> {
  state = { failed: false }

  static getDerivedStateFromError() {
    return { failed: true }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error(`[lens] panel ${this.props.panel.id} failed to render`, error, info)
  }

  componentDidUpdate(previous: Readonly<{ panel: Panel }>) {
    if (this.state.failed && previous.panel !== this.props.panel) this.setState({ failed: false })
  }

  render() {
    if (!this.state.failed) return this.props.children
    return (
      <section aria-label={this.props.panel.title} className="lens-panel lens-panel-error">
        <header className="lens-panel-header"><h3 className="lens-panel-title">{this.props.panel.title}</h3></header>
        <div className="lens-panel-state" role="alert">
          <span>{this.props.fallback}</span>
          <button onClick={() => this.setState({ failed: false })} type="button">{this.props.retryLabel}</button>
        </div>
      </section>
    )
  }
}

function PanelModuleFallback({ panel }: { panel: Panel }) {
  return (
    <section aria-busy="true" aria-label={panel.title} className="lens-panel lens-panel-loading">
      <header className="lens-panel-header"><h3 className="lens-panel-title">{panel.title}</h3></header>
      <div className="lens-panel-skeleton" />
    </section>
  )
}
