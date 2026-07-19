import type { ComponentType } from 'react'
import type { Panel } from '../contract'
import { BarPanel, LinePanel, PiePanel, type ChartPanelProps } from './ChartPanel'
import { StatPanel, type StatPanelProps } from './StatPanel'

/* eslint-disable react-refresh/only-export-components */

export type PanelComponent = ComponentType<StatPanelProps | ChartPanelProps>
export type PanelRegistry = Partial<Record<string, PanelComponent>>

export const panelRegistry: PanelRegistry = {
  stat: StatPanel,
  pie: PiePanel,
  donut: PiePanel,
  bar: BarPanel,
  hbar: BarPanel,
  line: LinePanel,
  area: LinePanel,
}

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
