import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import type { LayoutGroup, LayoutItem, Panel } from './contract'
import { useDashboard, useDocumentState, useDrawer, useTranslate } from './runtime'
import { ExportButton, RegisteredPanel, StatMetric, type PanelRegistry } from './panels'
import { ExplorePanel } from './explore'
import { FilterBar, type CalendarDate } from './controls'
import { isVisualRegression } from './visualRegression'

/* eslint-disable react-refresh/only-export-components */

export interface DashboardPanelsProps {
  registry?: PanelRegistry
  /** Fixed calendar "today" for deterministic stories and visual regression. */
  filterToday?: CalendarDate
}

function boundedSpan(span: number): number {
  if (!Number.isFinite(span)) return 12
  return Math.min(12, Math.max(1, Math.round(span)))
}

function spanStyle(span: number): CSSProperties {
  return { '--lens-panel-span': boundedSpan(span) } as CSSProperties
}

interface LayoutCluster {
  group?: LayoutGroup
  items: LayoutItem[]
}

/** Consecutive items sharing a group id render inside one container card. */
export function clusterRow(items: LayoutItem[]): LayoutCluster[] {
  const clusters: LayoutCluster[] = []
  for (const item of items) {
    const previous = clusters[clusters.length - 1]
    if (item.group && previous?.group?.id === item.group.id) {
      previous.items.push(item)
      continue
    }
    clusters.push({ group: item.group, items: [item] })
  }
  return clusters
}

function PanelSlot({ panel, registry }: { panel: Panel; registry?: PanelRegistry }) {
  return panel.drillRoot
    ? <ExplorePanel panel={panel} registry={registry} />
    : <RegisteredPanel panel={panel} registry={registry} />
}

function MissingPanel({ panelId }: { panelId: string }) {
  const translate = useTranslate()
  return (
    <div className="lens-panel-state" role="alert">
      {translate('panel.missing', 'Panel “{id}” is missing.', { id: panelId })}
    </div>
  )
}

function GroupCard({ group, children }: { group: LayoutGroup; children: ReactNode }) {
  return (
    <div className="lens-grid-item" style={spanStyle(group.span)}>
      <section
        aria-label={group.label || undefined}
        className={`lens-panel lens-panel-group ${group.kind === 'tabs' ? 'lens-panel-group-tabs' : 'lens-panel-group-metrics'}`}
      >
        {group.label && <header className="lens-panel-header"><h3 className="lens-panel-title">{group.label}</h3></header>}
        {children}
      </section>
    </div>
  )
}

function MetricsGroup({ group, items, panels, registry }: {
  group: LayoutGroup
  items: LayoutItem[]
  panels: Map<string, Panel>
  registry?: PanelRegistry
}) {
  return (
    <GroupCard group={group}>
      {/* Class names stay literal: Tailwind's content scan cannot see an
          interpolated modifier and would drop the rule. */}
      <div className={`lens-metric-row ${group.layout === 'rows' ? 'lens-metric-row-rows' : 'lens-metric-row-columns'}`}>
        {items.map((item) => {
          const panel = panels.get(item.panelId)
          if (!panel) return <MissingPanel key={item.panelId} panelId={item.panelId} />
          // Only stat panels have a chrome-free metric form; anything else
          // keeps its own card so the group degrades instead of breaking.
          // A stat that hosts a drill root needs its card chrome (the trail and
          // the breakdown affordance live there), so it opts out of the compact
          // metric form rather than losing its exploration.
          return panel.kind === 'stat' && !panel.drillRoot
            ? <StatMetric key={panel.id} panel={panel} />
            : <PanelSlot key={panel.id} panel={panel} registry={registry} />
        })}
      </div>
    </GroupCard>
  )
}

function TabsGroup({ group, items, panels, registry }: {
  group: LayoutGroup
  items: LayoutItem[]
  panels: Map<string, Panel>
  registry?: PanelRegistry
}) {
  const translate = useTranslate()
  const tabs = [...new Set(items.map((item) => item.group?.tab ?? ''))]
  const [active, setActive] = useState(tabs[0] ?? '')
  const current = tabs.includes(active) ? active : tabs[0] ?? ''
  const visible = items.filter((item) => (item.group?.tab ?? '') === current)

  return (
    <GroupCard group={group}>
      {/* An unlabelled group would otherwise expose its raw id to a screen
          reader; a translated generic name is the honest fallback. */}
      <div className="lens-tabstrip" role="tablist" aria-label={group.label || translate('dashboard.tabs', 'Tabs')}>
        {tabs.map((tab) => (
          <button
            aria-selected={tab === current}
            className="lens-tabstrip-tab"
            key={tab}
            onClick={() => setActive(tab)}
            role="tab"
            type="button"
          >
            {tab}
          </button>
        ))}
      </div>
      <div className="lens-panel-grid lens-tab-panel" role="tabpanel">
        {visible.map((item) => {
          const panel = panels.get(item.panelId)
          return (
            <div className="lens-grid-item" key={item.panelId} style={spanStyle(item.span)}>
              {panel ? <PanelSlot panel={panel} registry={registry} /> : <MissingPanel panelId={item.panelId} />}
            </div>
          )
        })}
      </div>
    </GroupCard>
  )
}

/** Relative "updated X ago" using the document's own locale. */
function relativeTime(timestamp: number, locale: string): string {
  const seconds = Math.round((timestamp - Date.now()) / 1000)
  const format = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' })
  const abs = Math.abs(seconds)
  if (abs < 60) return format.format(seconds, 'second')
  const minutes = Math.round(seconds / 60)
  if (Math.abs(minutes) < 60) return format.format(minutes, 'minute')
  const hours = Math.round(minutes / 60)
  if (Math.abs(hours) < 24) return format.format(hours, 'hour')
  return format.format(Math.round(hours / 24), 'day')
}

/**
 * A subtle "updated X ago" line under the dashboard header. It is hidden inside
 * drawers (the host dashboard already carries it) and under visual regression,
 * where a live timestamp would make the screenshot nondeterministic.
 */
function DashboardFreshness() {
  const { document } = useDashboard()
  const { isRefreshing } = useDocumentState()
  const drawer = useDrawer()
  const translate = useTranslate()
  const [, tick] = useState(0)

  useEffect(() => {
    if (isVisualRegression()) return
    const id = setInterval(() => tick((value) => value + 1), 60_000)
    return () => clearInterval(id)
  }, [])

  if (drawer.depth > 0 || isVisualRegression()) return null
  const generatedAt = Date.parse(document.meta.generatedAt)
  if (!Number.isFinite(generatedAt)) return null
  const label = isRefreshing
    ? translate('panel.updating', 'Updating')
    : translate('dashboard.updated', 'Updated {time}', { time: relativeTime(generatedAt, document.meta.locale) })
  return <p className="lens-dashboard-updated" aria-live="polite" data-refreshing={isRefreshing || undefined}>{label}</p>
}

function DocumentRefetchError() {
  const { error, refresh, dismissError } = useDocumentState()
  const translate = useTranslate()

  if (!error) return null
  return (
    <div className="lens-document-refetch-error" role="alert">
      <span>{translate('document.refetchFailed', 'Unable to refresh the dashboard. The previous data is still shown.')}</span>
      <div className="lens-document-refetch-error-actions">
        <button onClick={() => void refresh().catch(() => undefined)} type="button">
          {translate('document.retry', 'Retry')}
        </button>
        <button
          aria-label={translate('runtime.dismissNotice', 'Dismiss notice')}
          className="lens-document-refetch-error-dismiss"
          onClick={dismissError}
          type="button"
        >
          ×
        </button>
      </div>
    </div>
  )
}

export function DashboardPanels({ registry, filterToday }: DashboardPanelsProps) {
  const { document } = useDashboard()
  const translate = useTranslate()
  const drawer = useDrawer()
  const panels = new Map(document.panels.map((panel) => [panel.id, panel]))
  // First paint only: panels rise/fade in with a small per-panel stagger. The
  // value is fixed for this mount, so drill, perspective, drawer and refetch
  // re-renders keep the same class and never replay the animation. Off inside a
  // drawer and under visual regression, where the final state renders directly.
  const entrance = useRef(!isVisualRegression() && drawer.depth === 0)

  if (!document.layout.rows.length || !document.panels.length) {
    return (
      <div className="lens-placeholder-state">
        {translate('dashboard.empty', 'The document contains no panels.')}
      </div>
    )
  }

  const hasHeader = Boolean(document.meta.title) || Boolean(document.endpoints.export) ||
    (document.filters?.length ?? 0) > 0
  return (
    <main className="lens-dashboard" aria-label={document.meta.title}>
      {hasHeader && (
        <header className="lens-dashboard-header">
          {/* An empty title lets a host page own the heading and keeps the
              dashboard's own chrome to the action bar. */}
          {document.meta.title ? <h1>{document.meta.title}</h1> : <span />}
          <div className="lens-dashboard-controls">
            <FilterBar today={filterToday} />
            <ExportButton />
          </div>
        </header>
      )}
      {hasHeader && <DocumentRefetchError />}
      {hasHeader && <DashboardFreshness />}
      <div className="lens-dashboard-rows">
        {document.layout.rows.map((row, rowIndex) => (
          <section className={`lens-dashboard-row${row.class ? ` ${row.class}` : ''}`} key={`${row.heading ?? 'row'}-${rowIndex}`}>
            {row.heading && <h2 className="lens-row-heading"><span>{row.heading}</span></h2>}
            <div
              className={`lens-panel-grid${entrance.current ? ' lens-entrance' : ''}`}
              style={entrance.current ? ({ '--lens-row-delay': `${Math.min(rowIndex * 60, 180)}ms` } as CSSProperties) : undefined}
            >
              {clusterRow(row.panels).map((cluster, clusterIndex) => {
                if (cluster.group?.kind === 'metrics') {
                  return (
                    <MetricsGroup
                      group={cluster.group}
                      items={cluster.items}
                      key={`${cluster.group.id}-${clusterIndex}`}
                      panels={panels}
                      registry={registry}
                    />
                  )
                }
                if (cluster.group?.kind === 'tabs') {
                  return (
                    <TabsGroup
                      group={cluster.group}
                      items={cluster.items}
                      key={`${cluster.group.id}-${clusterIndex}`}
                      panels={panels}
                      registry={registry}
                    />
                  )
                }
                return cluster.items.map((item) => {
                  const panel = panels.get(item.panelId)
                  if (!panel) {
                    return (
                      <div className="lens-panel lens-panel-unsupported lens-grid-item" key={item.panelId} style={spanStyle(item.span)}>
                        <MissingPanel panelId={item.panelId} />
                      </div>
                    )
                  }
                  return (
                    <div className="lens-grid-item" key={panel.id} style={spanStyle(item.span)}>
                      <PanelSlot panel={panel} registry={registry} />
                    </div>
                  )
                })
              })}
            </div>
          </section>
        ))}
      </div>
    </main>
  )
}
