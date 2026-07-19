import type { CSSProperties } from 'react'
import { useDashboard } from './runtime'
import { RegisteredPanel, type PanelRegistry } from './panels'
import { ExplorePanel } from './explore'

export interface DashboardPanelsProps {
  registry?: PanelRegistry
}

function boundedSpan(span: number): number {
  if (!Number.isFinite(span)) return 12
  return Math.min(12, Math.max(1, Math.round(span)))
}

function spanStyle(span: number): CSSProperties {
  return { '--lens-panel-span': boundedSpan(span) } as CSSProperties
}

export function DashboardPanels({ registry }: DashboardPanelsProps) {
  const { document } = useDashboard()
  const panels = new Map(document.panels.map((panel) => [panel.id, panel]))

  if (!document.layout.rows.length || !document.panels.length) {
    return <div className="lens-placeholder-state">The document contains no panels.</div>
  }

  return (
    <main className="lens-dashboard" aria-label={document.meta.title}>
      <header className="lens-dashboard-header">
        <h1>{document.meta.title}</h1>
      </header>
      <div className="lens-dashboard-rows">
        {document.layout.rows.map((row, rowIndex) => (
          <section className={`lens-dashboard-row${row.class ? ` ${row.class}` : ''}`} key={`${row.heading ?? 'row'}-${rowIndex}`}>
            {row.heading && <h2 className="lens-row-heading">{row.heading}</h2>}
            <div className="lens-panel-grid">
              {row.panels.map((item) => {
                const panel = panels.get(item.panelId)
                if (!panel) {
                  return (
                    <div className="lens-panel lens-panel-unsupported lens-grid-item" key={item.panelId} style={spanStyle(item.span)}>
                      <div className="lens-panel-state" role="alert">Panel “{item.panelId}” is missing.</div>
                    </div>
                  )
                }
                return (
                  <div className="lens-grid-item" key={panel.id} style={spanStyle(item.span)}>
                    {panel.drillRoot
                      ? <ExplorePanel panel={panel} registry={registry} />
                      : <RegisteredPanel panel={panel} registry={registry} />}
                  </div>
                )
              })}
            </div>
          </section>
        ))}
      </div>
    </main>
  )
}
