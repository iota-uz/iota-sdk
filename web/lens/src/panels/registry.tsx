/* eslint-disable react/no-unknown-property -- Solid JSX uses `class`, the React-era rule expects `className`; the lint config migrates with the Solid port. */
import { createMemo, ErrorBoundary, For, lazy, Show, Suspense, type Component, type JSXElement } from 'solid-js'
import { Dynamic } from 'solid-js/web'
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
import { PanelSkeletonBody } from './Skeleton'
import { StatPanel, type StatPanelProps } from './StatPanel'
import type { TablePanelProps } from './TablePanel'

const ChartPanel: Component<ChartPanelProps> = lazy(async () => ({ default: (await import('./ChartPanel')).ChartPanel }))
const CascadePanel: Component<CascadePanelProps> = lazy(async () => ({ default: (await import('./CascadePanel')).CascadePanel }))
const CoveragePanel: Component<CoveragePanelProps> = lazy(async () => ({ default: (await import('./CoveragePanel')).CoveragePanel }))
const GaugePanel: Component<GaugePanelProps> = lazy(async () => ({ default: (await import('./GaugePanel')).GaugePanel }))
const MetricFlowPanel: Component<MetricFlowPanelProps> = lazy(async () => ({ default: (await import('./MetricFlowPanel')).MetricFlowPanel }))
const MetricHierarchyPanel: Component<MetricHierarchyPanelProps> = lazy(async () => ({ default: (await import('./MetricHierarchyPanel')).MetricHierarchyPanel }))
const MetricRelationshipPanel: Component<MetricRelationshipPanelProps> = lazy(async () => ({ default: (await import('./MetricRelationshipPanel')).MetricRelationshipPanel }))
const MapPanel: Component<MapPanelProps> = lazy(async () => ({ default: (await import('./MapPanel')).MapPanel }))
const TablePanel: Component<TablePanelProps> = lazy(async () => ({ default: (await import('./TablePanel')).TablePanel }))

/* eslint-disable react-refresh/only-export-components */
export type PanelComponent = Component<
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

export function UnsupportedPanel(props: { panel: Panel }) {
  const translate = useTranslate()
  return (
    <section class="lens-panel lens-panel-unsupported" aria-label={props.panel.title}>
      <header class="lens-panel-header"><h3 class="lens-panel-title">{props.panel.title}</h3></header>
      <div class="lens-panel-state" role="status">
        {translate('panel.unsupported', 'Unsupported panel: {kind}', { kind: props.panel.kind })}
      </div>
    </section>
  )
}

export function RegisteredPanel(props: RegisteredPanelProps) {
  const translate = useTranslate()
  const resolve = createMemo(() => (props.registry ?? panelRegistry)[props.panel.kind])
  return (
    <Show when={resolve()} fallback={<UnsupportedPanel panel={props.panel} />}>
      {(Resolved) => (
        <PanelErrorBoundary
          fallback={translate('panel.error', 'This panel could not be rendered.')}
          panel={props.panel}
          retryLabel={translate('panel.retry', 'Retry')}
        >
          <Suspense fallback={<PanelModuleFallback panel={props.panel} />}>
            <Dynamic component={Resolved()} panel={props.panel} />
          </Suspense>
        </PanelErrorBoundary>
      )}
    </Show>
  )
}

/**
 * An error boundary that resets when the panel object changes, mirroring the
 * React version's componentDidUpdate reset: a fresh document (or a drill level)
 * remounts the boundary with a fresh render instead of a stuck failure card.
 */
function PanelErrorBoundary(props: {
  children: JSXElement
  fallback: string
  panel: Panel
  retryLabel: string
}) {
  return (
    <For each={[props.panel]}>
      {(current) => (
        <ErrorBoundary
          fallback={(_error: unknown, reset: () => void) => (
            <section aria-label={current.title} class="lens-panel lens-panel-error">
              <header class="lens-panel-header"><h3 class="lens-panel-title">{current.title}</h3></header>
              <div class="lens-panel-state" role="alert">
                <span>{props.fallback}</span>
                <button onClick={reset} type="button">{props.retryLabel}</button>
              </div>
            </section>
          )}
        >
          {props.children}
        </ErrorBoundary>
      )}
    </For>
  )
}

/**
 * The card a panel occupies while its module is still downloading.
 *
 * It is the same card the data-loading state uses, because it is the same
 * moment to the reader: a titled panel that does not have its content yet. A
 * bare `lens-panel-skeleton` slab here meant the whole point of the skeleton
 * design — "the same rows, the same spans and a shape per panel kind, so
 * nothing jumps when the data lands" — was bypassed for module loading, and a
 * table-shaped card arriving in place of a chart-shaped slab is exactly the
 * jump the shapes exist to prevent.
 */
function PanelModuleFallback(props: { panel: Panel }) {
  return (
    <section aria-busy="true" aria-label={props.panel.title} class="lens-panel lens-panel-loading">
      <header class="lens-panel-header"><h3 class="lens-panel-title">{props.panel.title}</h3></header>
      <PanelSkeletonBody kind={props.panel.kind} />
    </section>
  )
}
