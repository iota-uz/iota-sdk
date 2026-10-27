
import {
  createContext,
  createMemo,
  createSignal,
  createUniqueId,
  lazy,
  onCleanup,
  onMount,
  useContext,
  Suspense,
  untrack,
  type Accessor,
  type JSX,
} from 'solid-js'
import { For, Show } from 'solid-js'
import type { Filter, LayoutGroup, LayoutItem, LayoutRow, Panel } from './contract'
import type { LensToolbarContext } from './LensDashboard'
import { useDashboard, useDocumentState, useDrawer, useDrawerHeader, useFilters, usePanelFrames, usePrint, useTranslate } from './runtime'
import { ExportMenu } from './panels/ExportMenu'
import { RegisteredPanel, type PanelRegistry } from './panels/registry'
import { ShareSliceButton } from './panels/ShareSliceButton'
import { StatMetric, StatusChip } from './panels/StatPanel'
import { PanelChromeContext, type PanelChrome } from './panels/context'
import { ArrowClockwise, CircleNotch, Clock, X } from './icons'
import { ExplorePanel } from './explore'
import { FilterBar, FilterControls, type CalendarDate } from './controls'
import { ContextGroupDetails, ContextGroupToolbar, createContextGroupDisclosure, hasContextGroupToolbar } from './controls/ScopedFilterControls'
import { isVisualRegression } from './visualRegression'
import { ReportVersionNotice } from './reports/ReportVersionNotice'
import { ReportPlacementMenu } from './reports/ReportPlacementMenu'
import { placementFreshness, ReportPlacementStatus } from './reports/ReportPlacementStatus'
import { reportPlacementMenu, reportPlacementUnavailable, useReportPlacementHost } from './reports/placementHost'
import { reportUnavailableState, reportUnavailableTitle } from './reports/unavailable'
import { reportPlacementActions } from './reports/version'
import { DrillGateContext, ExportGateContext, useDrillAllowed, useExportAllowed, withoutDrill } from './panels/drillGate'
import { ReportPlacementProvider } from './reports/placementHost'
import type { Placement } from './reports/contract/types'
import { spanOf } from './reports/layout'
import type { DashboardEditor } from './reports/dashboardEdit'
import type { LensDashboardChrome, LensDashboardEditing } from './reports/dashboardEditTypes'
import { DashboardEmptyState } from './reports/DashboardEmptyState'
import type { LensThemeMode } from './runtime'

type EditModule = typeof import('./reports/dashboardEdit')

 

// Print keeps its React renderer during the migration; typed here so the
// composition root compiles against the pending port.
const LazyPrintReport = lazy(async () => ({
  default: (await import('./print/PrintReport')).PrintReport as unknown as (props: object) => JSX.Element,
}))

export interface DashboardPanelsProps {
  toolbar?: (context: LensToolbarContext) => JSX.Element
  registry?: PanelRegistry
  /** Fixed calendar "today" for deterministic stories and visual regression. */
  filterToday?: CalendarDate
  edit?: LensDashboardEditing
  chrome?: LensDashboardChrome
  theme?: LensThemeMode
  locale?: string
  csrf?: string
  fetcher?: typeof fetch
}

function boundedSpan(span: number): number {
  if (!Number.isFinite(span)) return 12
  return Math.min(12, Math.max(1, Math.round(span)))
}

function spanStyle(span: number): JSX.CSSProperties {
  return { '--lens-panel-span': boundedSpan(span) } as JSX.CSSProperties
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
const MetricColumnsContext = createContext<number>(4)

/**
 * Per-group active-tab memory keyed by group id, held for the dashboard's
 * lifetime. A nested tab group unmounts when its outer tab switches away;
 * restoring its active tab from this store preserves the inner selection when
 * the outer tab returns.
 */
const TabStateContext = createContext<Map<string, string> | null>(null)

/** The panel as this slot may show it: terminal inside a placement that cannot drill. */
function useGatedPanel(panel: () => Panel | undefined): () => Panel | undefined {
  const drill = useDrillAllowed()
  return createMemo(() => {
    const current = panel()
    return current && !drill() ? withoutDrill(current) : current
  })
}

function PanelSlot(props: { panel: Panel; registry?: PanelRegistry }) {
  const panel = useGatedPanel(() => props.panel)
  return (
    <Show when={panel()} keyed>
      {(current) => current.drillRoot
        ? <ExplorePanel panel={current} registry={props.registry} />
        : <RegisteredPanel panel={current} registry={props.registry} />}
    </Show>
  )
}

function MissingPanel(props: { panelId: string }) {
  const translate = useTranslate()
  return (
    <div class="lens-panel-state" role="alert">
      {translate('panel.missing', 'Panel “{id}” is missing.', { id: props.panelId })}
    </div>
  )
}

/** A placement card's edit-mode chrome: the handle, the width slider and the drop target. */
type GroupCardEdit = ReturnType<DashboardEditor['cardEdit']>

function GroupCard(props: { group: LayoutGroup; items?: LayoutItem[]; panels?: Map<string, Panel>; edit?: GroupCardEdit; children: JSX.Element }) {
  const documentState = useDocumentState()
  const { document } = useDashboard()
  const translate = useTranslate()
  const [filtersOpen, setFiltersOpen] = createSignal(false)
  const disclosure = createContextGroupDisclosure(filtersOpen, setFiltersOpen)
  const contextToolbar = () => Boolean(props.group.filterContext && hasContextGroupToolbar(document.contextEditor, props.group.filterContext, props.group.id))
  const reportHost = useReportPlacementHost()
  const frames = usePanelFrames()
  // A report placement is an outermost grid group the host addresses, or one
  // the wire already marked unavailable. Only placements get the card menu and
  // status; the strips and grids inside one keep a single header and menu.
  const outermost = () => (props.items?.[0]?.groups?.[0]?.id ?? props.group.id) === props.group.id
  const placement = () => props.group.kind === 'grid' && outermost() && Boolean(reportHost || props.group.unavailable)
  const unavailable = createMemo(() => props.group.kind === 'grid' ? reportPlacementUnavailable(reportHost, props.group) : undefined)
  const blocked = () => Boolean(props.group.filterContext && (props.group.filterContext.conflicts.length || props.group.filterContext.unsupported.length))
  const panelIds = createMemo(() => (props.items ?? []).map((item) => item.panelId))
  const title = () => {
    const reason = unavailable()
    return reason ? reportUnavailableTitle(reason, props.group.label, translate) : props.group.label
  }
  const capabilities = () => reportHost?.capabilities?.(props.group.id)
  // A placement named after its only panel drops the panel's repeated title.
  const chrome = createMemo<PanelChrome | undefined>(() => {
    if (!placement()) return undefined
    const only = props.items?.length === 1 ? props.panels?.get(props.items[0]!.panelId) : undefined
    const redundant = Boolean(only && props.group.label && comparableTitle(only.title) === comparableTitle(props.group.label))
    return redundant ? { titleIsRedundant: true } : undefined
  })
  // Export is gated like drill: a refusal applies to every member below the
  // placement, including nested groups that provide their own chrome.
  const outerExport = useExportAllowed()
  const exportAllowed = createMemo(() => {
    if (!outerExport()) return false
    if (!placement()) return true
    const declared = capabilities()
    return declared ? reportPlacementActions(declared, props.group.filterContext).export : true
  })
  // Drill is the source's to offer: a placement whose source or context does
  // not carry it renders every member terminal, inheriting an outer refusal.
  const outerDrill = useDrillAllowed()
  const drillAllowed = createMemo(() => {
    if (!outerDrill()) return false
    if (!placement()) return true
    const context = props.group.filterContext
    const declared = capabilities()
    return declared ? reportPlacementActions(declared, context).drill : !(context?.unsupportedOperations?.drill?.length)
  })
  const state = createMemo(() => {
    const reason = unavailable()
    if (reason) return reportUnavailableState(reason)
    if (blocked()) return 'unsupported'
    if (!placement()) return undefined
    const reading = placementFreshness(panelIds().map((id) => frames(id)), document.meta.generatedAt, documentState.isRefreshing)
    return reading?.state === 'failed' ? 'error' : reading?.state === 'updating' ? 'updating' : 'ready'
  })
  const menu = () => reportHost && placement() ? reportPlacementMenu(reportHost, props.group.id, { context: props.group.filterContext, unavailable: unavailable() }).length > 0 : false
  const edit = props.edit
  const editing = () => Boolean(props.edit?.editing)
  const live = () => !props.edit?.preview
  const itemStyle = (): JSX.CSSProperties => edit
    ? { ...spanStyle(edit.span()), '--lens-edit-row': String(edit.row + 1) } as JSX.CSSProperties
    : spanStyle(props.group.span)
  return (
    <div
      class="lens-grid-item"
      data-added={(edit && editing() && edit.added) || undefined}
      data-drop-target={(edit && editing() && edit.dropTarget()) || undefined}
      data-edit-placement={edit ? props.group.id : undefined}
      onDragLeave={edit ? () => edit.dropProps.onDragLeave() : undefined}
      onDragOver={edit ? (event) => { if (edit.editing) edit.dropProps.onDragOver(event) } : undefined}
      onDrop={edit ? (event) => { if (edit.editing) edit.dropProps.onDrop(event) } : undefined}
      ref={(element) => edit?.cardRef(element)}
      style={itemStyle()}
    >
      <section
        aria-label={title() || undefined}
        class={`lens-panel lens-panel-group ${`lens-panel-group-${props.group.kind}`}`}
        data-report-placement={placement() ? props.group.id : undefined}
        data-state={state()}
        tabindex={placement() ? -1 : undefined}
      >
        <Show when={Boolean(title() || props.group.status || contextToolbar() || menu() || editing())}>
          <header class="lens-panel-header lens-panel-group-header">
            <Show when={edit && editing()}>{edit!.handle()}</Show>
            <Show when={title()}>
              <h3 class="lens-panel-title">{title()}</h3>
            </Show>
            {/* A uniform-status group shows one hoisted chip here; the members
                below then render without their own repeated chip. */}
            <Show when={props.group.status}>
              <StatusChip status={props.group.status!} />
            </Show>
            <Show when={contextToolbar() && props.group.filterContext}>{(context) => <ContextGroupToolbar id={props.group.id} context={context()} disclosure={disclosure} />}</Show>
            <Show when={menu()}>
              <ReportPlacementMenu placementId={props.group.id} title={title() || props.group.id} context={props.group.filterContext} unavailable={unavailable()} />
            </Show>
          </header>
        </Show>
        {/* A group's caption reads for the whole strip, so it sits under the
            heading rather than inside any one member's card. It is a
            server-produced string that names the period the figures below are
            for, so while a new document is in flight it describes the previous
            one — it dims with them rather than standing at full strength over
            superseded numbers. */}
        <Show when={props.group.caption && unavailable() !== 'report-forbidden'}>
          <p class="lens-panel-caption" data-stale={documentState.isRefreshing || undefined}>{props.group.caption}</p>
        </Show>
        <Show when={placement() && live()}><ReportVersionNotice placementId={props.group.id} unavailable={props.group.unavailable} /></Show>
        <Show when={!unavailable() && props.group.filterContext}>{(context) => <ContextGroupDetails id={props.group.id} context={context()} disclosure={disclosure} />}</Show>
        <Show when={!unavailable() && !blocked()}>
          <ExportGateContext.Provider value={exportAllowed}>
            <DrillGateContext.Provider value={drillAllowed}>
              <PanelChromeContext.Provider value={chrome()}>{props.children}</PanelChromeContext.Provider>
            </DrillGateContext.Provider>
          </ExportGateContext.Provider>
        </Show>
        <Show when={reportHost && placement() && !unavailable() && !blocked() && live()}>
          <ReportPlacementStatus placementId={props.group.id} panelIds={panelIds()} timeZone={props.group.filterContext?.calendar.timezone} />
        </Show>
      </section>
      <Show when={edit && editing()}>{edit!.resize()}</Show>
    </div>
  )
}

/** A single ungrouped panel occupying its own grid slot. */
function LeafItem(props: {
  item: LayoutItem
  panels: Map<string, Panel>
  registry?: PanelRegistry
}) {
  const panel = props.panels.get(props.item.panelId)
  if (!panel) {
    return (
      <div class="lens-panel lens-panel-unsupported lens-grid-item" style={spanStyle(props.item.span)}>
        <MissingPanel panelId={props.item.panelId} />
      </div>
    )
  }
  return (
    <div class="lens-grid-item" style={spanStyle(props.item.span)}>
      <PanelSlot panel={panel} registry={props.registry} />
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
function GroupChain(props: {
  items: LayoutItem[]
  depth: number
  panels: Map<string, Panel>
  registry?: PanelRegistry
  filterToday?: CalendarDate
}) {
  return (
    <For each={clusterAtDepth(props.items, props.depth)}>
      {(cluster) => {
        if (!cluster.group) {
          return (
            <For each={cluster.items}>
              {(item) => <LeafItem item={item} panels={props.panels} registry={props.registry} />}
            </For>
          )
        }
        if (cluster.group.kind === 'grid') {
          // Built where it is placed, so the members see the card's chrome context.
          const contents = () => <div class="lens-panel-grid"><GroupChain depth={props.depth + 1} filterToday={props.filterToday} items={cluster.items} panels={props.panels} registry={props.registry} /></div>
          // An unavailable placement (a forbidden one has no label) still
          // takes its slot and explains itself instead of showing its member.
          return cluster.group.label || cluster.group.caption || cluster.group.unavailable
            ? <GroupCard group={cluster.group} items={cluster.items} panels={props.panels}>{contents()}</GroupCard>
            : <div class="lens-grid-item" style={spanStyle(cluster.group.span)}>{contents()}</div>
        }
        if (cluster.group.kind === 'metrics') {
          return <MetricsGroup group={cluster.group} items={cluster.items} panels={props.panels} registry={props.registry} />
        }
        return (
          <TabsGroup depth={props.depth} filterToday={props.filterToday} group={cluster.group} items={cluster.items} panels={props.panels} registry={props.registry} />
        )
      }}
    </For>
  )
}

function MetricsGroup(props: {
  group: LayoutGroup
  items: LayoutItem[]
  panels: Map<string, Panel>
  registry?: PanelRegistry
}) {
  const columns = useContext(MetricColumnsContext)
  // One span per breakpoint: the document's own column count on a wide card,
  // two columns on a narrow one. The cell carries both, and the sheet picks
  // which one applies — a container query cannot compute an integer.
  const spans = metricSpans(props.items.length, columns)
  const spansNarrow = metricSpans(props.items.length, 2)
  return (
    <GroupCard group={props.group} items={props.items} panels={props.panels}>
      {/* Class names stay literal: Tailwind's content scan cannot see an
          interpolated modifier and would drop the rule. */}
      <div class={`lens-metric-row ${props.group.layout === 'rows' ? 'lens-metric-row-rows' : 'lens-metric-row-columns'}`}>
        <For each={props.items}>
          {(item, index) => {
            const gated = useGatedPanel(() => props.panels.get(item.panelId))
            const panel = gated()
            // Every member sits in the same kind of box, whatever it contains.
            // A cell that wrapped its own content (a navigable stat used to) sized
            // itself differently from one that did not, which is how one strip
            // ended up with 447px and 480px cells side by side.
            return (
              <div
                class="lens-metric-cell"
                style={{
                  '--lens-metric-span': spans[index()] ?? 1,
                  '--lens-metric-span-2': spansNarrow[index()] ?? 1,
                } as JSX.CSSProperties}
              >
                {!panel
                  ? <MissingPanel panelId={item.panelId} />
                  // Only stat panels have a chrome-free metric form; anything else
                  // keeps its own card so the group degrades instead of breaking.
                  // A stat that hosts a drill root needs its card chrome (the trail
                  // and the breakdown affordance live there), so it opts out of the
                  // compact metric form rather than losing its exploration.
                  : panel.kind === 'stat' && !panel.drillRoot && !panel.renderer
                    ? <StatMetric panel={panel} />
                    : <PanelSlot panel={panel} registry={props.registry} />}
              </div>
            )
          }}
        </For>
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

function TabsGroup(props: {
  group: LayoutGroup
  items: LayoutItem[]
  depth: number
  panels: Map<string, Panel>
  registry?: PanelRegistry
  filterToday?: CalendarDate
}) {
  const translate = useTranslate()
  const { filters } = useFilters()
  const print = usePrint()
  const store = useContext(TabStateContext)
  const baseId = createUniqueId()
  const tabs = [...new Set(props.items.map((item) => groupAt(item, props.depth)?.tab ?? ''))]
  // The initial tab is restored from the per-group store so an inner group's
  // selection survives an outer tab switching away and back (which remounts it).
  const [active, setActive] = createSignal(store?.get(props.group.id) ?? tabs[0] ?? '')
  const current = createMemo(() => tabs.includes(active()) ? active() : tabs[0] ?? '')
  const tabRefs: Array<HTMLButtonElement | null> = []
  const [focused, setFocused] = createSignal(current())
  const focusCurrent = createMemo(() => tabs.includes(focused()) ? focused() : current())

  const select = (tab: string) => {
    store?.set(props.group.id, tab)
    setActive(tab)
    setFocused(tab)
  }

  // Roving-tabindex keyboard model (WAI-ARIA tabs). The handler is on each tab
  // button, and it stops propagation, so an inner tablist's arrow keys never
  // reach and move an ancestor tablist.
  const onTabKeyDown = (event: KeyboardEvent, index: number) => {
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
    tabRefs[next]?.focus()
  }

  const tabId = (index: number) => `${baseId}-tab-${index}`
  const panelId = (index: number) => `${baseId}-panel-${index}`
  const filtersForTab = (tab: string): Filter[] => filters.filter((filter) => (
    filter.placement?.groupId === props.group.id && filter.placement.tab === tab
  ))

  return (
    <GroupCard group={props.group}>
      {/* An unlabelled group would otherwise expose its raw id to a screen
          reader; a translated generic name is the honest fallback. */}
      <div class="lens-tabstrip" role="tablist" aria-label={props.group.label || translate('dashboard.tabs', 'Tabs')}>
        <For each={tabs}>
          {(tab, index) => (
            <button
              aria-controls={panelId(index())}
              aria-selected={tab === current()}
              class="lens-tabstrip-tab"
              data-testid={`lens-tabs-${props.group.id}-tab-${index()}`}
              id={tabId(index())}
              onClick={() => select(tab)}
              onKeyDown={(event) => onTabKeyDown(event, index())}
              ref={(node) => { tabRefs[index()] = node }}
              role="tab"
              tabIndex={tab === focusCurrent() ? 0 : -1}
              type="button"
              onKeyUp={(event) => {
                if (event.key !== 'Enter' && event.key !== ' ') return
                event.preventDefault()
                select(tab)
              }}
            >
              {tab}
            </button>
          )}
        </For>
      </div>
      {/* Every tabpanel element exists so each tab's aria-controls resolves, but
          only the active one mounts its content — the inactive ones are hidden
          and empty, so hidden panels never fetch. A print run is the exception:
          the report covers every tab, so the others mount to resolve their
          frames. They stay hidden while it does, or the dashboard behind the
          report flashes every tab at once. */}
      <For each={tabs}>
        {(tab, index) => (
          <div
            aria-labelledby={tabId(index())}
            class="lens-panel-grid lens-tab-panel"
            hidden={tab !== current()}
            id={panelId(index())}
            role="tabpanel"
            tabIndex={0}
          >
            {(print.active || tab === current()) && (
              <>
                {filtersForTab(tab).length > 0 && (
                  <div class="lens-tab-filter-bar" role="group">
                    <FilterControls filters={filtersForTab(tab)} today={props.filterToday} />
                  </div>
                )}
                <PanelChromeContext.Provider value={namesItsOnlyPanel(tab, props.items, props.depth, props.panels) ? redundantTitle : undefined}>
                  <GroupChain
                    depth={props.depth + 1}
                    items={props.items.filter((item) => (groupAt(item, props.depth)?.tab ?? '') === tab)}
                    panels={props.panels}
                    registry={props.registry}
                    filterToday={props.filterToday}
                  />
                </PanelChromeContext.Provider>
              </>
            )}
          </div>
        )}
      </For>
    </GroupCard>
  )
}

/**
 * Relative "updated X ago" using the document's own locale.
 *
 * Short units, because this reading is now the label of the control it invites
 * rather than a sentence beside it: «Рассчитано 17 секунд назад» is a header
 * button 210px wide that says what «17 сек. назад» says in 140.
 */
function relativeTime(timestamp: number, locale: string): string {
  const seconds = Math.round((timestamp - Date.now()) / 1000)
  const format = new Intl.RelativeTimeFormat(locale, { numeric: 'auto', style: 'short' })
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
function useFreshness(): Accessor<{ label: string; isRefreshing: boolean; absolute: string; stale: boolean } | null> {
  const { document } = useDashboard()
  const documentState = useDocumentState()
  const drawer = useDrawer()
  const translate = useTranslate()
  const [tick, setTick] = createSignal(0)

  onMount(() => {
    if (isVisualRegression()) return
    const id = setInterval(() => setTick((value) => value + 1), 60_000)
    onCleanup(() => clearInterval(id))
  })

  return createMemo(() => {
    void tick()
    if (drawer.depth > 0 || isVisualRegression()) return null
    const generatedAt = Date.parse(document.meta.generatedAt)
    if (!Number.isFinite(generatedAt)) return null
    const label = documentState.isRefreshing
      ? translate('panel.updating', 'Updating')
      : translate('dashboard.updated', 'Updated {time}', { time: relativeTime(generatedAt, document.meta.locale) })
    const absolute = new Intl.DateTimeFormat(document.meta.locale, { dateStyle: 'medium', timeStyle: 'short' })
      .format(generatedAt)
    return { label, isRefreshing: documentState.isRefreshing, absolute, stale: Date.now() - generatedAt > stalenessThresholdMs }
  })
}

/**
 * How old the figures on screen are, and the one thing a reader does about it.
 *
 * These were two controls: a stamp reading «Рассчитано 9 минут назад» and a
 * button reading «Пересчитать» beside it. Two boxes, one subject — the stamp
 * stated a fact whose only available response was the box next to it, and
 * «Пересчитать» named the machine's activity rather than answering the question
 * a reader actually has. Fused, the age *is* the label: the control says whether
 * it is worth pressing instead of asking the reader to correlate two adjacent
 * readings, and the action row loses an entry.
 *
 * The label therefore changes with wall-clock time, which the split version
 * deliberately avoided — a widening button used to shove Экспорт out from under
 * the pointer mid-gesture. It cannot here: this is the first member of a
 * right-aligned cluster (`.lens-dashboard-actions` is `justify-end`), so its own
 * left edge absorbs every width change and the controls after it never move.
 *
 * Four states, because two independent facts gate it. Without a recompute
 * endpoint the dashboard cannot act on its own staleness, so the same reading
 * renders as a bare stamp on the same baseline and the row's geometry does not
 * change between dashboards. Without a parseable `generatedAt` — and under
 * visual regression, where a relative time is not reproducible — there is no age
 * to print, so the button falls back to naming itself.
 */
function FreshnessControl() {
  const dashboard = useDashboard()
  const translate = useTranslate()
  const freshness = useFreshness()
  const hintID = createUniqueId()

  const hint = createMemo(() => dashboard.isRecomputing
    ? translate('dashboard.recomputing', 'Recomputing…')
    : translate('dashboard.recomputeHint', 'Refresh every figure, ignoring the cached results'))

  return (
    <Show when={dashboard.canRecompute} fallback={
      <Show when={freshness()}>
        {(value) => (
          <p
            aria-live="polite"
            class="lens-dashboard-updated"
            data-refreshing={value().isRefreshing || undefined}
            data-stale={value().stale || undefined}
            title={value().absolute}
          >
            <Clock />
            <span>{value().label}</span>
          </p>
        )}
      </Show>
    }>
      {/* A relative label alone reads as uptime: it ticks up with wall-clock time
          whether or not anything went stale, so the absolute timestamp is what the
          control actually claims, on hover and in the description. */}
      <button
        aria-busy={dashboard.isRecomputing}
        aria-describedby={hintID}
        aria-live="polite"
        class="lens-export-button lens-recompute"
        data-stale={freshness()?.stale || undefined}
        disabled={dashboard.isRecomputing}
        onClick={() => dashboard.recompute()}
        title={freshness() ? `${freshness()!.absolute} — ${hint()}` : hint()}
        type="button"
      >
        {dashboard.isRecomputing ? <CircleNotch className="lens-icon-spin" /> : <ArrowClockwise />}
        <span>{freshness() ? freshness()!.label : translate('dashboard.recompute', 'Recompute')}</span>
      </button>
      <span class="lens:sr-only" id={hintID}>{hint()}</span>
    </Show>
  )
}

function DocumentRefetchError() {
  const documentState = useDocumentState()
  const translate = useTranslate()

  return (
    <Show when={documentState.error}>
      <div class="lens-document-refetch-error" role="alert">
        <span>{translate('document.refetchFailed', 'Unable to refresh the dashboard. The previous data is still shown.')}</span>
        <div class="lens-document-refetch-error-actions">
          <button onClick={() => void documentState.refresh().catch(() => undefined)} type="button">
            {translate('document.retry', 'Retry')}
          </button>
          <button
            aria-label={translate('runtime.dismissNotice', 'Dismiss notice')}
            class="lens-document-refetch-error-dismiss"
            onClick={() => documentState.dismissError()}
            type="button"
          >
            <X />
          </button>
        </div>
      </div>
    </Show>
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
  let started = false
  onMount(() => {
    if (!requested || started) return
    started = true
    void run({ preview: true })
  })
}

interface PlacementSource {
  group: LayoutGroup
  items: LayoutItem[]
}

/** The report placements of a composed document: outermost grid groups, keyed by placement id, with their items. */
export function documentPlacements(rows: LayoutRow[]): Map<string, PlacementSource> {
  const placements = new Map<string, PlacementSource>()
  for (const row of rows) {
    for (const item of row.panels) {
      const group = item.groups?.[0]
      if (!group || group.kind !== 'grid') continue
      const existing = placements.get(group.id)
      if (existing) existing.items.push(item)
      else placements.set(group.id, { group, items: [item] })
    }
  }
  return placements
}

/**
 * The board drawn from the host's composition draft rather than the document
 * layout. Cards are keyed by placement id, so a move, a resize or a caption
 * change re-renders the card in place: its panels keep their mounted charts and
 * the frames already loaded. One grid holds every row, each card naming its
 * row, so a card crossing rows moves instead of remounting.
 */
function DraftLayout(props: {
  editor: DashboardEditor
  rows: LayoutRow[]
  panels: Map<string, Panel>
  registry?: PanelRegistry
  filterToday?: CalendarDate
  entrance: boolean
  theme: LensThemeMode
  locale?: string
  csrf?: string
  fetcher?: typeof fetch
}) {
  const sources = createMemo(() => documentPlacements(props.rows))
  let grid: HTMLDivElement | undefined
  return (
    <div class="lens-dashboard-rows">
      <div
        class={`lens-panel-grid lens-dashboard-edit-grid${props.entrance ? ' lens-entrance' : ''}`}
        data-editing={props.editor.editing() || undefined}
        data-testid="dashboard-draft-grid"
        ref={(element) => { grid = element }}
      >
        <For each={props.editor.session.ids()}>
          {(id) => (
            <DraftPlacement
              csrf={props.csrf}
              editor={props.editor}
              fetcher={props.fetcher}
              filterToday={props.filterToday}
              grid={() => grid}
              id={id}
              locale={props.locale}
              panels={props.panels}
              registry={props.registry}
              source={() => sources().get(id)}
              theme={props.theme}
            />
          )}
        </For>
      </div>
    </div>
  )
}

function DraftPlacement(props: {
  id: string
  editor: DashboardEditor
  source: () => PlacementSource | undefined
  panels: Map<string, Panel>
  registry?: PanelRegistry
  filterToday?: CalendarDate
  grid: () => HTMLElement | undefined
  theme: LensThemeMode
  locale?: string
  csrf?: string
  fetcher?: typeof fetch
}) {
  const session = props.editor.session
  const current = () => session.byId(props.id)
  const placement = (): Placement | undefined => current()?.placement
  // The figures stay the document's; the caption and width are the draft's.
  // A report the reader may not see keeps the document's withheld label.
  const group = createMemo<LayoutGroup>(() => {
    const base = props.source()?.group
    const value = placement()
    const span = value ? spanOf(value) : base?.span ?? 12
    if (base?.unavailable) return { ...base, span }
    if (!base) return { id: props.id, kind: 'grid', span, label: value ? session.title(value) : props.id }
    const label = value?.title ?? (value ? session.host.reportTitle?.(value.reportId) : undefined) ?? base.label
    return { ...base, span, label }
  })
  const edit = props.editor.cardEdit(props.id, {
    title: () => group().unavailable === 'report-forbidden' ? reportUnavailableTitle('report-forbidden', undefined, session.translate) : group().label || props.id,
    preview: () => !props.source(),
    grid: props.grid,
  })
  return (
    <Show when={placement()}>
      <GroupCard edit={edit} group={group()} items={props.source()?.items} panels={props.panels}>
        <Show
          fallback={props.editor.Added({ csrf: props.csrf, fetcher: props.fetcher, locale: props.locale, placement: placement()!, theme: props.theme })}
          when={props.source()}
        >
          {(source) => (
            <div class="lens-panel-grid">
              <GroupChain depth={1} filterToday={props.filterToday} items={source().items} panels={props.panels} registry={props.registry} />
            </div>
          )}
        </Show>
      </GroupCard>
    </Show>
  )
}

export function DashboardPanels(props: DashboardPanelsProps) {
  const { document } = useDashboard()
  const dashboard = useDashboard()
  const translate = useTranslate()
  const drawer = useDrawer()
  const drawerHeader = useDrawerHeader()
  const print = usePrint()
  const panels = createMemo(() => new Map(document.panels.map((panel) => [panel.id, panel])))
  // First paint only: panels rise/fade in with a small per-panel stagger. The
  // value is fixed for this mount, so drill, perspective, drawer and refetch
  // re-renders keep the same class and never replay the animation. Off inside a
  // drawer and under visual regression, where the final state renders directly.
  const entrance = !isVisualRegression() && drawer.depth === 0
  // Active-tab memory shared by every (possibly nested) tab group in this mount.
  const tabState = new Map<string, string>()
  usePrintPreview()
  const theme = () => props.theme ?? 'light'
  // An edit-capable host draws the board from its composition draft, in view
  // mode too: entering edit mode then keeps every card mounted, and a saved
  // layout stays on screen until the host refreshes the document.
  const edit = untrack(() => props.edit)
  const editable = Boolean(edit) && drawer.depth === 0
  const [editModule, setEditModule] = createSignal<EditModule>()
  const [editLoadFailed, setEditLoadFailed] = createSignal(false)
  // A failed chunk (a deploy replaced it, a flaky network) leaves the board
  // read-only; the retry repeats the import while the dashboard stays mounted.
  const loadEditModule = () => {
    setEditLoadFailed(false)
    void import('./reports/dashboardEdit').then((module) => setEditModule(() => module), (cause: unknown) => {
      console.error('[lens] the dashboard edit mode failed to load', cause)
      setEditLoadFailed(true)
    })
  }
  if (editable) loadEditModule()
  const placementHost = useReportPlacementHost()
  const chrome = () => props.chrome ?? {}

  const documentTitle = () => document.header?.title || document.meta.title
  const identityTitle = createMemo(() => {
    // A drawer whose document carries a `drawer` block states its own identity
    // in the chrome — eyebrow, title, caption and period. The document then
    // printed its own title and subtitle directly underneath, so «Детализация /
    // Бухгалтерские страховые выплаты / Период: 1 янв. — 2 авг. 2026» was
    // followed by the same metric and the same period again. Where the chrome
    // speaks, the contents do not: the identity belongs to the frame.
    //
    // Gated on the chrome actually carrying a title, not merely on being in a
    // drawer. A document opened without a `drawer` block leaves the chrome's
    // identity line empty, and suppressing the document's own heading there
    // would take the drawer's only name away rather than de-duplicate it.
    const inDrawer = drawer.depth > 0 && Boolean(drawerHeader()?.title?.trim())
    return inDrawer || chrome().title === false || chrome().header === false ? '' : documentTitle()
  })
  const hasHeader = (editor?: DashboardEditor) => {
    if (editor) return true
    if (chrome().header === false) return false
    return Boolean(identityTitle()) || Boolean(document.endpoints.export) || print.available ||
      (document.filters?.length ?? 0) > 0 || dashboard.canRecompute
  }
  const showFilters = () => chrome().header !== false
  const subtitle = () => {
    if (chrome().description === false) return undefined
    return drawer.depth > 0 && Boolean(drawerHeader()?.title?.trim()) ? undefined : document.header?.subtitle
  }
  const documentEmpty = () => !(document.layout.rows.length > 0 && document.panels.length > 0)
  const empty = (editor?: DashboardEditor) => editor ? editor.session.ids().length === 0 : documentEmpty()
  const columns = createMemo(() => metricColumnCount(document.layout.rows))
  // Comparison is a property of the dashboard, not of the panels that happen to
  // carry a delta: with it on, every strip reserves the chip's row. Otherwise
  // turning it on grows one strip by 26px and leaves the strip beneath it where
  // it was, so two rows of the same component stop matching.
  const comparing = createMemo(() => (document.filters ?? []).some((filter) =>
    filter.kind === 'compare' && filter.compare !== undefined && filter.compare.value.mode !== 'off'))

  const documentRows = () => (
    <div class="lens-dashboard-rows">
      <For each={document.layout.rows}>
        {(row, rowIndex) => (
          <section
            class={`lens-dashboard-row${row.class ? ` ${row.class}` : ''}`}
            id={row.anchor || undefined}
          >
            <Show when={row.heading}>
              <h2 class="lens-row-heading"><span>{row.heading}</span></h2>
            </Show>
            <div
              class={`lens-panel-grid${entrance ? ' lens-entrance' : ''}`}
              style={entrance ? ({ '--lens-row-delay': `${Math.min(rowIndex() * 60, 180)}ms` } as JSX.CSSProperties) : undefined}
            >
              <GroupChain depth={0} filterToday={props.filterToday} items={row.panels} panels={panels()} registry={props.registry} />
            </div>
          </section>
        )}
      </For>
    </div>
  )

  const board = (editor?: DashboardEditor) => {
    const editing = () => Boolean(editor?.editing())
    return (
      <Show
        when={!documentEmpty() || editable || placementHost}
        fallback={
          <div class="lens-placeholder-state">
            {translate('dashboard.empty', 'The document contains no panels.')}
          </div>
        }
      >
        <TabStateContext.Provider value={tabState}>
          <main
            aria-label={(identityTitle() || (chrome().title === false || chrome().header === false ? documentTitle() : '')) || undefined}
            class={`lens-dashboard${comparing() ? ' lens-dashboard-comparing' : ''}`}
            data-editing={editing() || undefined}
            style={{ '--lens-metric-columns': columns() } as JSX.CSSProperties}
          >
            <Show when={hasHeader(editor)}>
              <header class="lens-dashboard-header">
                {/* Three rows, each answering one question, in the order a reader
                  asks them: what page is this and what can I do with it, what
                  slice am I looking at, and what is currently narrowing it.
                  They used to be two columns — identity opposite a single wrapping
                  cluster that mixed the controls changing the data with the verbs
                  carrying it away, at one weight and with no divider, so nothing
                  said which half did which. */}
                <div class="lens-dashboard-headline">
                  {/* Without a title an empty slot lets a host page own the heading
                    and keeps the dashboard's own chrome to the action row. */}
                  {identityTitle() ? <h1 class="lens-dashboard-title" data-testid="lens-dashboard-title">{identityTitle()}</h1> : <span />}
                  {/* The actions travel as one block. Loose in the row they
                    re-ordered themselves against the filters at every wrap, so the
                    fourth control at 1440px was the second one at 1000px. */}
                  <Show
                    fallback={(
                      <div class="lens-dashboard-actions">
                        <FreshnessControl />
                        <ShareSliceButton />
                        <ExportMenu />
                        <Show when={editor && editor.edit.toggle !== false}>{editor!.Toggle()}</Show>
                      </div>
                    )}
                    when={editing()}
                  >
                    {editor!.Bar()}
                  </Show>
                </div>
                {/* The producer's own scope line rides with the controls that change
                  it rather than under the title: on the analytics boards it
                  restated the period the period control was already printing, so
                  the same fact occupied three surfaces. What is left of it is what
                  no control states — the date the data itself was cut. */}
                <Show when={showFilters()}>
                  <FilterBar compact={chrome().toolbar === 'compact'} subtitle={subtitle()} today={props.filterToday} />
                </Show>
                <Show when={editor}>
                  <p aria-live="polite" class="lens-visually-hidden" data-testid="dashboard-edit-announcement">{editor!.modeAnnouncement()}</p>
                </Show>
              </header>
            </Show>
            <Show when={hasHeader(editor)}>
              <DocumentRefetchError />
            </Show>
            <Show when={editor && editing()}>{editor!.Notices()}</Show>
            {props.toolbar?.({ get document() { return dashboard.document } })}
            <MetricColumnsContext.Provider value={columns()}>
              <Show when={!empty(editor)} fallback={<DashboardEmptyState onAdd={editor?.session.host.catalog ? (trigger) => editor.addReport(trigger) : undefined} />}>
                <Show fallback={documentRows()} when={editor}>
                  <ReportPlacementProvider host={editor!.placementHost(placementHost)}>
                    <DraftLayout
                      csrf={props.csrf}
                      editor={editor!}
                      entrance={entrance}
                      fetcher={props.fetcher}
                      filterToday={props.filterToday}
                      locale={props.locale}
                      panels={panels()}
                      registry={props.registry}
                      rows={document.layout.rows}
                      theme={theme()}
                    />
                  </ReportPlacementProvider>
                </Show>
              </Show>
            </MetricColumnsContext.Provider>
            <Show when={editor}>{editor!.Dialogs()}</Show>
            <Show when={print.active}>
              <Suspense fallback={null}>
                <LazyPrintReport />
              </Suspense>
            </Show>
          </main>
        </TabStateContext.Provider>
      </Show>
    )
  }

  // Until the edit chunk arrives the board renders read-only from the document.
  return (
    <Show
      fallback={(
        <>
          <Show when={editLoadFailed()}>
            <div class="lens-document-refetch-error" data-testid="dashboard-edit-load-failed" role="alert">
              <span>{translate('dashboard.edit.loadFailed', 'Editing could not be loaded. The dashboard stays read-only.')}</span>
              <div class="lens-document-refetch-error-actions">
                <button onClick={loadEditModule} type="button">{translate('document.retry', 'Retry')}</button>
              </div>
            </div>
          </Show>
          {board()}
        </>
      )}
      when={editModule()}
    >
      {(module) => {
        const Scope = module().DashboardEditScope
        return <Scope edit={edit!} theme={theme}>{(editor) => board(editor)}</Scope>
      }}
    </Show>
  )
}