import {
  createContext,
  lazy,
  Suspense,
  useContext,
  useEffect,
  useId,
  useRef,
  useState,
  type CSSProperties,
  type KeyboardEvent as ReactKeyboardEvent,
  type ReactNode,
} from 'react'
import type { LayoutGroup, LayoutItem, LayoutRow, Panel } from './contract'
import { useDashboard, useDocumentState, useDrawer, useDrawerHeader, usePrint, useTranslate } from './runtime'
import { ExportMenu } from './panels/ExportMenu'
import { RegisteredPanel, type PanelRegistry } from './panels/registry'
import { ShareSliceButton } from './panels/ShareSliceButton'
import { StatMetric, StatusChip } from './panels/StatPanel'
import { PanelChromeContext, type PanelChrome } from './panels/context'
import { ArrowClockwise, CircleNotch, Clock, X } from './icons'
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

/** Beyond this, a strip's cells are slivers whatever the viewport. */
const metricColumnCap = 6

/**
 * The widest strip on the dashboard, which is how many columns every strip on
 * it is drawn on.
 *
 * A strip is identified by its group id, which is stable per strip, so counting
 * members by id is the same partition the renderer clusters into.
 */
export function metricColumnCount(rows: LayoutRow[]): number {
  const sizes = new Map<string, number>()
  for (const row of rows) {
    for (const item of row.panels) {
      for (const group of chainOf(item)) {
        if (group.kind !== 'metrics') continue
        sizes.set(group.id, (sizes.get(group.id) ?? 0) + 1)
      }
    }
  }
  const widest = Math.max(1, ...sizes.values())
  return Math.min(metricColumnCap, widest)
}

/**
 * How many of the strip's columns each member occupies.
 *
 * Every strip on a dashboard is drawn on one column grid — that is what makes
 * the vertical rules of a three-member strip and a four-member strip meet
 * instead of zig-zagging past each other. A strip whose member count does not
 * fill its last row hands the leftover columns to the cells that are there, so
 * the row still reaches the card's edge (no half-row of white reading as a card
 * that failed to load) while every seam still lands on a column line.
 */
export function metricSpans(count: number, columns: number): number[] {
  const cols = Math.max(1, Math.floor(columns))
  if (count <= 0) return []
  const spans = new Array<number>(count).fill(1)
  const tail = count % cols
  if (tail === 0) return spans
  const base = Math.floor(cols / tail)
  const extra = cols % tail
  for (let index = 0; index < tail; index += 1) {
    spans[count - tail + index] = base + (index < extra ? 1 : 0)
  }
  return spans
}

/**
 * The strip's column count. Set once per document from the widest strip so a
 * member can compute its span; 4 is the shape a producer chunks strips into
 * when nothing else is known.
 */
const MetricColumnsContext = createContext(4)

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
  const { isRefreshing } = useDocumentState()
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
            heading rather than inside any one member's card. It is a
            server-produced string that names the period the figures below are
            for, so while a new document is in flight it describes the previous
            one — it dims with them rather than standing at full strength over
            superseded numbers. */}
        {group.caption && (
          <p className="lens-panel-caption" data-stale={isRefreshing || undefined}>{group.caption}</p>
        )}
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
  const columns = useContext(MetricColumnsContext)
  // One span per breakpoint: the document's own column count on a wide card,
  // two columns on a narrow one. The cell carries both, and the sheet picks
  // which one applies — a container query cannot compute an integer.
  const spans = metricSpans(items.length, columns)
  const spansNarrow = metricSpans(items.length, 2)
  return (
    <GroupCard group={group}>
      {/* Class names stay literal: Tailwind's content scan cannot see an
          interpolated modifier and would drop the rule. */}
      <div className={`lens-metric-row ${group.layout === 'rows' ? 'lens-metric-row-rows' : 'lens-metric-row-columns'}`}>
        {items.map((item, index) => {
          const panel = panels.get(item.panelId)
          // Every member sits in the same kind of box, whatever it contains.
          // A cell that wrapped its own content (a navigable stat used to) sized
          // itself differently from one that did not, which is how one strip
          // ended up with 447px and 480px cells side by side.
          return (
            <div
              className="lens-metric-cell"
              style={{
                '--lens-metric-span': spans[index] ?? 1,
                '--lens-metric-span-2': spansNarrow[index] ?? 1,
              } as CSSProperties}
              key={item.panelId}
            >
              {!panel
                ? <MissingPanel panelId={item.panelId} />
                // Only stat panels have a chrome-free metric form; anything else
                // keeps its own card so the group degrades instead of breaking.
                // A stat that hosts a drill root needs its card chrome (the trail
                // and the breakdown affordance live there), so it opts out of the
                // compact metric form rather than losing its exploration.
                : panel.kind === 'stat' && !panel.drillRoot
                  ? <StatMetric panel={panel} />
                  : <PanelSlot panel={panel} registry={registry} />}
            </div>
          )
        })}
      </div>
    </GroupCard>
  )
}

/** One `PanelChrome` value, so the provider does not remount its subtree. */
const redundantTitle: PanelChrome = { titleIsRedundant: true }

function comparableTitle(value: string): string {
  return value.trim().replace(/\s+/gu, ' ').toLocaleLowerCase()
}

/**
 * Whether the tab's label already names the one panel behind it.
 *
 * A tab group is itself a card: heading, border, radius. A single panel inside
 * it brings a second set, and when the tab label and the panel title are the
 * same words the reader is shown one name twice, forty pixels apart, in two
 * typographic treatments. Only the exact-match, single-panel case is silenced —
 * a tab holding several panels needs every one of them named, and a tab label
 * that says something different from the panel title is not a duplicate.
 */
function namesItsOnlyPanel(
  tab: string,
  items: LayoutItem[],
  depth: number,
  panels: Map<string, Panel>,
): boolean {
  if (!tab.trim()) return false
  const members = items.filter((item) => (groupAt(item, depth)?.tab ?? '') === tab)
  if (members.length !== 1) return false
  const only = members[0]
  // A member that is itself grouped deeper renders its own container; the tab
  // label is not naming a card in that case.
  if (only && chainOf(only).length > depth + 1) return false
  const title = only ? panels.get(only.panelId)?.title : undefined
  return Boolean(title) && comparableTitle(title!) === comparableTitle(tab)
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
  const [focused, setFocused] = useState(current)
  const focusCurrent = tabs.includes(focused) ? focused : current

  const select = (tab: string) => {
    store?.set(group.id, tab)
    setActive(tab)
    setFocused(tab)
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
    setFocused(nextTab)
    tabRefs.current[next]?.focus()
  }

  const tabId = (index: number) => `${baseId}-tab-${index}`
  const panelId = (index: number) => `${baseId}-panel-${index}`

  return (
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
            tabIndex={tab === focusCurrent ? 0 : -1}
            type="button"
            onKeyUp={(event) => {
              if (event.key !== 'Enter' && event.key !== ' ') return
              event.preventDefault()
              select(tab)
            }}
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
            <PanelChromeContext.Provider value={namesItsOnlyPanel(tab, items, depth, panels) ? redundantTitle : undefined}>
              <GroupChain
                depth={depth + 1}
                items={items.filter((item) => (groupAt(item, depth)?.tab ?? '') === tab)}
                panels={panels}
                registry={registry}
              />
            </PanelChromeContext.Provider>
          )}
        </div>
      ))}
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
 * Past this age the stamp stops being a detail and becomes a caveat on every
 * figure below it.
 */
const stalenessThresholdMs = 60 * 60_000

/**
 * The document's live "updated X ago" read, or null when it cannot be shown
 * (inside a drawer, under visual regression, or without a parseable timestamp).
 * Ticks once a minute so the relative label stays current. Shared by the lone
 * freshness line and the header subtitle that folds it in.
 */
function useFreshness(): { label: string; isRefreshing: boolean; absolute: string; stale: boolean } | null {
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
  const absolute = new Intl.DateTimeFormat(document.meta.locale, { dateStyle: 'medium', timeStyle: 'short' })
    .format(generatedAt)
  return { label, isRefreshing, absolute, stale: Date.now() - generatedAt > stalenessThresholdMs }
}

/**
 * How old the figures on screen are.
 *
 * It used to be the tail of the identity paragraph, joined to a static blurb by
 * a middot: «Аналитика продуктового портфеля… · Рассчитано 20 минут назад» —
 * one 12px grey sentence carrying a marketing line and the single fact a report
 * reader checks before trusting a number, with the run-on wrapping at 600px so
 * the age landed mid-line. It is its own control-height stamp now, beside
 * Recompute — the action it invites — with a mark, the absolute timestamp on
 * hover, and an escalation once the reading is old enough to matter. A relative
 * label alone reads as uptime: it ticks up with wall-clock time whether or not
 * anything went stale, so the timestamp is what it actually claims.
 */
function FreshnessStamp() {
  const freshness = useFreshness()
  if (!freshness) return null
  return (
    <p
      aria-live="polite"
      className="lens-dashboard-updated"
      data-refreshing={freshness.isRefreshing || undefined}
      data-stale={freshness.stale || undefined}
      title={freshness.absolute}
    >
      <Clock />
      <span>{freshness.label}</span>
    </p>
  )
}

/**
 * Force every panel past the server cache.
 *
 * Three things it did not say before. What it does: "Recompute" names the
 * machine's activity, not the reader's question, so the explanation rides on the
 * control. Whether it is worth doing: the button is quiet while the reading is
 * fresh and takes the warning tone once the figures are old enough for the
 * freshness stamp beside it to escalate — a control with no resting state is one
 * readers learn to click superstitiously. And that it is running: a spinner in
 * place of the arrow, with the label unchanged, because swapping the label to
 * «Пересчитывается…» widened the button by 40px and moved Экспорт out from under
 * the pointer mid-gesture.
 */
function RecomputeButton() {
  const { isRecomputing, recompute } = useDashboard()
  const translate = useTranslate()
  const freshness = useFreshness()
  const hintID = useId()
  const label = translate('dashboard.recompute', 'Recompute')
  const hint = isRecomputing
    ? translate('dashboard.recomputing', 'Recomputing…')
    : translate('dashboard.recomputeHint', 'Refresh every figure, ignoring the cached results')
  return (
    <>
      <button
        aria-busy={isRecomputing}
        aria-describedby={hintID}
        className="lens-export-button lens-recompute"
        data-stale={freshness?.stale || undefined}
        disabled={isRecomputing}
        onClick={recompute}
        title={hint}
        type="button"
      >
        {isRecomputing ? <CircleNotch className="lens-icon-spin" /> : <ArrowClockwise />}
        <span>{label}</span>
      </button>
      <span className="lens-sr-only" id={hintID}>{hint}</span>
    </>
  )
}

/** The document's identity subtitle: the producer's period/scope line. */
function DashboardSubtitle({ subtitle }: { subtitle?: string }) {
  if (!subtitle) return null
  return <p className="lens-dashboard-subtitle">{subtitle}</p>
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
  const { document, canRecompute } = useDashboard()
  const translate = useTranslate()
  const drawer = useDrawer()
  const drawerHeader = useDrawerHeader()
  const print = usePrint()
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
  // A drawer whose document carries a `drawer` block states its own identity in
  // the chrome — eyebrow, title, caption and period. The document then printed
  // its own title and subtitle directly underneath, so «Детализация /
  // Бухгалтерские страховые выплаты / Период: 1 янв. — 2 авг. 2026» was followed
  // by the same metric and the same period again. Where the chrome speaks, the
  // contents do not: the identity belongs to the frame.
  //
  // Gated on the chrome actually carrying a title, not merely on being in a
  // drawer. A document opened without a `drawer` block leaves the chrome's
  // identity line empty, and suppressing the document's own heading there would
  // take the drawer's only name away rather than de-duplicate it.
  const inDrawer = drawer.depth > 0 && Boolean(drawerHeader?.title?.trim())
  const identityTitle = inDrawer ? '' : (header?.title || document.meta.title)
  const hasHeader = Boolean(identityTitle) || Boolean(document.endpoints.export) || print.available ||
    (document.filters?.length ?? 0) > 0 || canRecompute
  const columns = metricColumnCount(document.layout.rows)
  // Comparison is a property of the dashboard, not of the panels that happen to
  // carry a delta: with it on, every strip reserves the chip's row. Otherwise
  // turning it on grows one strip by 26px and leaves the strip beneath it where
  // it was, so two rows of the same component stop matching.
  const comparing = (document.filters ?? []).some((filter) =>
    filter.kind === 'compare' && filter.compare !== undefined && filter.compare.value.mode !== 'off')
  return (
    <TabStateContext.Provider value={tabState}>
      <main
        aria-label={identityTitle || undefined}
        className={`lens-dashboard${comparing ? ' lens-dashboard-comparing' : ''}`}
        style={{ '--lens-metric-columns': columns } as CSSProperties}
      >
        {hasHeader && (
          <header className="lens-dashboard-header">
            {/* The document header owns the page identity: a strong title over a
              muted period + freshness subtitle. Without one, an empty title
              lets a host page own the heading and keeps the dashboard's own
              chrome to the action bar. */}
            {inDrawer ? <span /> : header ? (
              <div className="lens-dashboard-identity">
                {identityTitle ? <h1 className="lens-dashboard-title">{identityTitle}</h1> : <span />}
                <DashboardSubtitle subtitle={header.subtitle} />
              </div>
            ) : (
              document.meta.title ? <h1>{document.meta.title}</h1> : <span />
            )}
            <div className="lens-dashboard-controls">
              <FilterBar today={filterToday} />
              {/* The actions travel as one block. Loose in the controls row they
                  re-ordered themselves against the filters at every wrap, so the
                  fourth control at 1440px was the second one at 1000px. */}
              <div className="lens-dashboard-actions">
                <FreshnessStamp />
                {canRecompute && <RecomputeButton />}
                <ShareSliceButton />
                <ExportMenu />
              </div>
            </div>
          </header>
        )}
        {hasHeader && <DocumentRefetchError />}
        <MetricColumnsContext.Provider value={columns}>
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
        </MetricColumnsContext.Provider>
        {print.active && (
          <Suspense fallback={null}>
            <LazyPrintReport />
          </Suspense>
        )}
      </main>
    </TabStateContext.Provider>
  )
}
