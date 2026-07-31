import {
  createContext,
  lazy,
  Suspense,
  useContext,
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type KeyboardEvent as ReactKeyboardEvent,
  type ReactNode,
} from 'react'
import type { LayoutGroup, LayoutItem, Panel } from './contract'
import { useDashboard, useDocumentState, useDrawer, usePrint, useTranslate } from './runtime'
import { ExportMenu, RegisteredPanel, SavedViewsMenu, ShareSliceButton, StatMetric, StatusChip, type PanelRegistry } from './panels'
import { LegendVisibilityContext } from './panels/context'
import { X } from './icons'
import { ExplorePanel } from './explore'
import { FilterBar, type CalendarDate } from './controls'
import { isVisualRegression } from './visualRegression'

/* eslint-disable react-refresh/only-export-components */

const LazyPrintReport = lazy(() => import('./print/PrintReport').then(({ PrintReport }) => ({ default: PrintReport })))

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

/**
 * The group chain for an item, outermost→innermost.
 */
function chainOf(item: LayoutItem): LayoutGroup[] {
  return item.groups ?? []
}

function groupAt(item: LayoutItem, depth: number): LayoutGroup | undefined {
  return chainOf(item)[depth]
}

/**
 * Consecutive items sharing a group id at the given chain depth form one
 * cluster; an item with no group at that depth stays a singleton (loose)
 * cluster that renders as a bare grid panel.
 */
function clusterAtDepth(items: LayoutItem[], depth: number): LayoutCluster[] {
  const clusters: LayoutCluster[] = []
  for (const item of items) {
    const group = groupAt(item, depth)
    const previous = clusters[clusters.length - 1]
    if (group && previous?.group?.id === group.id) {
      previous.items.push(item)
      continue
    }
    clusters.push({ group, items: [item] })
  }
  return clusters
}

/** Level-0 clustering; retained for backward compatibility and single-level callers. */
export function clusterRow(items: LayoutItem[]): LayoutCluster[] {
  return clusterAtDepth(items, 0)
}

/**
 * Per-group active-tab memory keyed by group id, held for the dashboard's
 * lifetime. A nested tab group unmounts when its outer tab switches away;
 * restoring its active tab from this store preserves the inner selection when
 * the outer tab returns.
 */
const TabStateContext = createContext<Map<string, string> | null>(null)

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
        {(group.label || group.status) && (
          <header className="lens-panel-header lens-panel-group-header">
            {group.label && <h3 className="lens-panel-title">{group.label}</h3>}
            {/* A uniform-status group shows one hoisted chip here; the members
                below then render without their own repeated chip. */}
            {group.status && <StatusChip status={group.status} />}
          </header>
        )}
        {/* A group's caption reads for the whole strip, so it sits under the
            heading rather than inside any one member's card. */}
        {group.caption && <p className="lens-panel-caption">{group.caption}</p>}
        {children}
      </section>
    </div>
  )
}

/** A single ungrouped panel occupying its own grid slot. */
function LeafItem({ item, panels, registry }: {
  item: LayoutItem
  panels: Map<string, Panel>
  registry?: PanelRegistry
}) {
  const panel = panels.get(item.panelId)
  if (!panel) {
    return (
      <div className="lens-panel lens-panel-unsupported lens-grid-item" style={spanStyle(item.span)}>
        <MissingPanel panelId={item.panelId} />
      </div>
    )
  }
  return (
    <div className="lens-grid-item" style={spanStyle(item.span)}>
      <PanelSlot panel={panel} registry={registry} />
    </div>
  )
}

/**
 * Renders one level of the group chain as grid children: loose items become
 * bare grid panels, a metrics chain becomes a StatGroup card, and a tabs chain
 * becomes a TabsGroup whose active tabpanel recurses into the next chain level.
 * The same function drives the top-level grid and every nested tabpanel, so
 * arbitrary compositions (tabs-in-tabs, metrics-in-tabs, tabs after passthrough
 * containers, grouped mixed with ungrouped) fall out of one recursion.
 */
function GroupChain({ items, depth, panels, registry }: {
  items: LayoutItem[]
  depth: number
  panels: Map<string, Panel>
  registry?: PanelRegistry
}) {
  return (
    <>
      {clusterAtDepth(items, depth).map((cluster, index) => {
        if (!cluster.group) {
          return cluster.items.map((item) => (
            <LeafItem item={item} key={item.panelId} panels={panels} registry={registry} />
          ))
        }
        const key = `${cluster.group.id}-${depth}-${index}`
        if (cluster.group.kind === 'metrics') {
          return <MetricsGroup group={cluster.group} items={cluster.items} key={key} panels={panels} registry={registry} />
        }
        return (
          <TabsGroup depth={depth} group={cluster.group} items={cluster.items} key={key} panels={panels} registry={registry} />
        )
      })}
    </>
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

function TabsGroup({ group, items, depth, panels, registry }: {
  group: LayoutGroup
  items: LayoutItem[]
  depth: number
  panels: Map<string, Panel>
  registry?: PanelRegistry
}) {
  const translate = useTranslate()
  const print = usePrint()
  const store = useContext(TabStateContext)
  const baseId = useId()
  const tabs = [...new Set(items.map((item) => groupAt(item, depth)?.tab ?? ''))]
  // The initial tab is restored from the per-group store so an inner group's
  // selection survives an outer tab switching away and back (which remounts it).
  const [active, setActive] = useState(() => store?.get(group.id) ?? tabs[0] ?? '')
  const current = tabs.includes(active) ? active : tabs[0] ?? ''
  const tabRefs = useRef<Array<HTMLButtonElement | null>>([])
  const [hiddenSeries, setHiddenSeries] = useState<ReadonlySet<string>>(() => new Set())
  const toggleSeries = useCallback((key: string) => {
    setHiddenSeries((currentHidden) => {
      const next = new Set(currentHidden)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }, [])
  const resetSeries = useCallback(() => setHiddenSeries(new Set()), [])
  const replaceSeries = useCallback((keys: ReadonlySet<string>) => setHiddenSeries(new Set(keys)), [])
  const legendVisibility = useMemo(() => ({
    hidden: hiddenSeries,
    set: replaceSeries,
    toggle: toggleSeries,
    reset: resetSeries,
  }), [hiddenSeries, replaceSeries, resetSeries, toggleSeries])

  const select = (tab: string) => {
    store?.set(group.id, tab)
    setActive(tab)
  }

  // Roving-tabindex keyboard model (WAI-ARIA tabs). The handler is on each tab
  // button, and it stops propagation, so an inner tablist's arrow keys never
  // reach and move an ancestor tablist.
  const onTabKeyDown = (event: ReactKeyboardEvent<HTMLButtonElement>, index: number) => {
    let next = index
    if (event.key === 'ArrowRight') next = (index + 1) % tabs.length
    else if (event.key === 'ArrowLeft') next = (index - 1 + tabs.length) % tabs.length
    else if (event.key === 'Home') next = 0
    else if (event.key === 'End') next = tabs.length - 1
    else return
    const nextTab = tabs[next]
    if (nextTab === undefined) return
    event.preventDefault()
    event.stopPropagation()
    select(nextTab)
    tabRefs.current[next]?.focus()
  }

  const tabId = (index: number) => `${baseId}-tab-${index}`
  const panelId = (index: number) => `${baseId}-panel-${index}`

  return (
    <LegendVisibilityContext.Provider value={legendVisibility}>
      <GroupCard group={group}>
      {/* An unlabelled group would otherwise expose its raw id to a screen
          reader; a translated generic name is the honest fallback. */}
      <div className="lens-tabstrip" role="tablist" aria-label={group.label || translate('dashboard.tabs', 'Tabs')}>
        {tabs.map((tab, index) => (
          <button
            aria-controls={panelId(index)}
            aria-selected={tab === current}
            className="lens-tabstrip-tab"
            id={tabId(index)}
            key={tab}
            onClick={() => select(tab)}
            onKeyDown={(event) => onTabKeyDown(event, index)}
            ref={(node) => { tabRefs.current[index] = node }}
            role="tab"
            tabIndex={tab === current ? 0 : -1}
            type="button"
          >
            {tab}
          </button>
        ))}
      </div>
      {/* Every tabpanel element exists so each tab's aria-controls resolves, but
          only the active one mounts its content — the inactive ones are hidden
          and empty, so hidden panels never fetch. A print run is the exception:
          the report covers every tab, so the others mount to resolve their
          frames. They stay hidden while it does, or the dashboard behind the
          report flashes every tab at once. */}
      {tabs.map((tab, index) => (
        <div
          aria-labelledby={tabId(index)}
          className="lens-panel-grid lens-tab-panel"
          hidden={tab !== current}
          id={panelId(index)}
          key={tab}
          role="tabpanel"
          tabIndex={0}
        >
          {(print.active || tab === current) && (
            <GroupChain
              depth={depth + 1}
              items={items.filter((item) => (groupAt(item, depth)?.tab ?? '') === tab)}
              panels={panels}
              registry={registry}
            />
          )}
        </div>
      ))}
      </GroupCard>
    </LegendVisibilityContext.Provider>
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
 * The document's live "updated X ago" read, or null when it cannot be shown
 * (inside a drawer, under visual regression, or without a parseable timestamp).
 * Ticks once a minute so the relative label stays current. Shared by the lone
 * freshness line and the header subtitle that folds it in.
 */
function useFreshness(): { label: string; isRefreshing: boolean } | null {
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
  return { label, isRefreshing }
}

/**
 * A subtle "updated X ago" line under the dashboard header. It is hidden inside
 * drawers (the host dashboard already carries it) and under visual regression,
 * where a live timestamp would make the screenshot nondeterministic.
 */
function DashboardFreshness() {
  const freshness = useFreshness()
  if (!freshness) return null
  return (
    <p className="lens-dashboard-updated" aria-live="polite" data-refreshing={freshness.isRefreshing || undefined}>
      {freshness.label}
    </p>
  )
}

/**
 * The document's identity subtitle: a producer-localized period line with the
 * live freshness read folded in (« … · updated 5 min ago »). The freshness
 * fragment keeps its own aria-live so a refresh is still announced.
 */
function DashboardSubtitle({ subtitle }: { subtitle?: string }) {
  const freshness = useFreshness()
  if (!subtitle && !freshness) return null
  return (
    <p className="lens-dashboard-subtitle">
      {subtitle && <span>{subtitle}</span>}
      {subtitle && freshness && <span aria-hidden="true" className="lens-dashboard-subtitle-sep"> · </span>}
      {freshness && (
        <span aria-live="polite" data-refreshing={freshness.isRefreshing || undefined}>{freshness.label}</span>
      )}
    </p>
  )
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
          <X />
        </button>
      </div>
    </div>
  )
}

/**
 * `?lens-print-preview=1` composes the printed report and leaves it on screen.
 * Print output is otherwise unreviewable: the browser's print dialog blocks the
 * page, so neither a person nor an automated check can look at the result.
 */
function usePrintPreview(): void {
  const print = usePrint()
  const run = print.run
  const requested = typeof window !== 'undefined' &&
    new URL(window.location.href).searchParams.get('lens-print-preview') === '1'
  const started = useRef(false)
  useEffect(() => {
    if (!requested || started.current) return
    started.current = true
    void run({ preview: true })
  }, [requested, run])
}

export function DashboardPanels({ registry, filterToday }: DashboardPanelsProps) {
  const { document, canRecompute, isRecomputing, recompute } = useDashboard()
  const translate = useTranslate()
  const drawer = useDrawer()
  const print = usePrint()
	const recomputeLabel = isRecomputing
		? translate('dashboard.recomputing', 'Recomputing…')
		: translate('dashboard.recompute', 'Recompute')
  const panels = new Map(document.panels.map((panel) => [panel.id, panel]))
  // First paint only: panels rise/fade in with a small per-panel stagger. The
  // value is fixed for this mount, so drill, perspective, drawer and refetch
  // re-renders keep the same class and never replay the animation. Off inside a
  // drawer and under visual regression, where the final state renders directly.
  const entrance = useRef(!isVisualRegression() && drawer.depth === 0)
  // Active-tab memory shared by every (possibly nested) tab group in this mount.
  const tabState = useRef<Map<string, string>>(new Map()).current
  usePrintPreview()

  if (!document.layout.rows.length || !document.panels.length) {
    return (
      <div className="lens-placeholder-state">
        {translate('dashboard.empty', 'The document contains no panels.')}
      </div>
    )
  }

  const header = document.header
  const identityTitle = header?.title || document.meta.title
  const hasHeader = Boolean(identityTitle) || Boolean(document.endpoints.export) || print.available ||
    (document.filters?.length ?? 0) > 0 || canRecompute
  return (
    <TabStateContext.Provider value={tabState}>
    <main className="lens-dashboard" aria-label={identityTitle}>
      {hasHeader && (
        <header className="lens-dashboard-header">
          {/* The document header owns the page identity: a strong title over a
              muted period + freshness subtitle. Without one, an empty title
              lets a host page own the heading and keeps the dashboard's own
              chrome to the action bar. */}
          {header ? (
            <div className="lens-dashboard-identity">
              {identityTitle ? <h1 className="lens-dashboard-title">{identityTitle}</h1> : <span />}
              <DashboardSubtitle subtitle={header.subtitle} />
            </div>
          ) : (
            document.meta.title ? <h1>{document.meta.title}</h1> : <span />
          )}
          <div className="lens-dashboard-controls">
            <FilterBar today={filterToday} />
            {canRecompute && (
              <button
                className="lens-export-button"
                disabled={isRecomputing}
                onClick={recompute}
                type="button"
              >
                {recomputeLabel}
              </button>
            )}
            <SavedViewsMenu />
            <ShareSliceButton />
            <ExportMenu />
          </div>
        </header>
      )}
      {hasHeader && <DocumentRefetchError />}
      {/* The header folds freshness into its subtitle; only the headerless
          layout still shows the lone updated line. */}
      {hasHeader && !header && <DashboardFreshness />}
      <div className="lens-dashboard-rows">
        {document.layout.rows.map((row, rowIndex) => (
          <section
            className={`lens-dashboard-row${row.class ? ` ${row.class}` : ''}`}
            id={row.anchor || undefined}
            key={`${row.heading ?? 'row'}-${rowIndex}`}
          >
            {row.heading && <h2 className="lens-row-heading"><span>{row.heading}</span></h2>}
            <div
              className={`lens-panel-grid${entrance.current ? ' lens-entrance' : ''}`}
              style={entrance.current ? ({ '--lens-row-delay': `${Math.min(rowIndex * 60, 180)}ms` } as CSSProperties) : undefined}
            >
              <GroupChain depth={0} items={row.panels} panels={panels} registry={registry} />
            </div>
          </section>
        ))}
      </div>
      {print.active && (
        <Suspense fallback={null}>
          <LazyPrintReport />
        </Suspense>
      )}
    </main>
    </TabStateContext.Provider>
  )
}
