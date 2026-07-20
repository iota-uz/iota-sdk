import type { ComponentType } from 'react'
import type { Panel, PanelKind } from '../contract'
import { CascadePanel, type CascadePanelProps } from './CascadePanel'
import { CoveragePanel, type CoveragePanelProps } from './CoveragePanel'
import { BarPanel, LinePanel, PiePanel, type ChartPanelProps } from './ChartPanel'
import { StatPanel, type StatPanelProps } from './StatPanel'
import { TablePanel, type TablePanelProps } from './TablePanel'

/* eslint-disable react-refresh/only-export-components */

export type PanelComponent = ComponentType<StatPanelProps | ChartPanelProps | CascadePanelProps | TablePanelProps | CoveragePanelProps>
export type PanelRegistry = Partial<Record<PanelKind, PanelComponent>>

export const UNSUPPORTED = [] as const satisfies readonly PanelKind[]
type UnsupportedKind = (typeof UNSUPPORTED)[number]
type SupportedKind = Exclude<PanelKind, UnsupportedKind>

export const SUPPORTED = {
  stat: StatPanel,
  pie: PiePanel,
  donut: PiePanel,
  bar: BarPanel,
  hbar: BarPanel,
  line: LinePanel,
  area: LinePanel,
  cascade: CascadePanel,
  table: TablePanel,
  coverage: CoveragePanel,
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
  return (
    <section className="lens-panel lens-panel-unsupported" aria-label={panel.title}>
      <header className="lens-panel-header"><h3 className="lens-panel-title">{panel.title}</h3></header>
      <div className="lens-panel-state" role="status">Unsupported panel: {panel.kind}</div>
    </section>
  )
}

export function RegisteredPanel({ panel, registry = panelRegistry }: RegisteredPanelProps) {
  const Component = registry[panel.kind]
  return Component ? <Component panel={panel} /> : <UnsupportedPanel panel={panel} />
}
