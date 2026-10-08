import {
  batch,
  createContext,
  createEffect,
  createMemo,
  createSignal,
  Show,
  on,
  onCleanup,
  onMount,
  untrack,
  useContext,
  type Accessor,
  type JSX,
} from 'solid-js'
import { produce, reconcile, createStore } from 'solid-js/store'
import type { CompareValue, DashboardDocument, FieldFormat, Filter, Frame, NodeKey, NodePath, Panel, PanelBatchResult, PanelCalculation, PeriodValue, QueryPage, QueryRequest, TableSort, TableSummary } from '../contract'
import { fetchDocument } from './document'
import {
  compareValues,
  declaredFilters,
  periodTransitionValues,
  readFilterValues,
  sameFilterValues,
  segmentedValues,
  srcWithFilterParams,
  writeFilterValues,
  type FilterValues,
} from './filters'
import {
  dynamicParentPath,
  isPerspectiveFork,
  levelForPath,
  panelForNavigation,
  pathResolves,
  perspectivesForPosition,
  queryPathForNavigation,
  rootNavigation,
  withFrameChildren,
  withInlineFrameChildren,
} from './drill'
import { DashboardSkeleton, defaultSkeletonRows, drawerSkeletonRows } from '../panels/Skeleton'
import { formatAxis, formatFieldValue, formatFieldValueAtReference, formatFieldValueExact } from './format'
import {
  createNavigationState,
  navigationActions,
  navigationReducer,
  type NavigationState,
  type NavigationView,
} from './navigation'
import { LensDrawer } from './drawer'
import type { IdlePrefetchQueue } from './idlePrefetch'
import type { DocumentCache } from './prefetch'
import type { PrintReport } from './print'
import { QueryClient } from './query'
import { PanelClient } from './panel'
import { QueryError, SnapshotGoneError } from './query'
import { queryWithSnapshotRecovery } from './recovery'
import { drawerNavigationFromSource, navigationFromURL, navigationToURL, sameNavigationURL, siteRelativeURL } from './url'
import { navigateTo } from './navigate'
import { X } from '../icons'

export interface DocumentContextValue {
  readonly document?: DashboardDocument
  readonly isLoading: boolean
  /** A background refetch (focus-triggered) is in flight; current data stays. */
  readonly isRefreshing: boolean
  readonly error: Error | null
  refresh: () => Promise<DashboardDocument>
  dismissError: () => void
  /**
   * Refetches the document with these filter parameters on the src. The
   * current document stays on screen until the new one lands; every later
   * refresh — including snapshot recovery — keeps the applied parameters.
   */
  applyFilters: (values: FilterValues) => void
}

const DocumentContext = createContext<DocumentContextValue>()

/** A document older than this is refetched when the window regains focus. */
const staleDocumentAgeMs = 5 * 60 * 1000

export interface DocumentProviderProps {
  src?: string
  initialDocument?: DashboardDocument
  csrf?: string
  fetcher?: typeof fetch
  /** Warmed drill documents; a hit seeds the initial document and skips fetch. */
  cache?: Pick<DocumentCache, 'configure' | 'get' | 'load'>
  children?: JSX.Element
}

/**
 * Classifies a rendered panel into a bounded transport-only viewport band.
 * Layout remains the source of truth for the first wave; this only reorders
 * later rows according to what a reader can already see or will reach next.
 */
export function panelViewportRank(panelId: string): number | undefined {
  if (typeof document === 'undefined' || typeof window === 'undefined') return undefined
  const escaped = typeof CSS !== 'undefined' && typeof CSS.escape === 'function' ? CSS.escape(panelId) : panelId.replace(/["\\]/g, '\\$&')
  const element = document.querySelector<HTMLElement>(`[data-panel-id="${escaped}"]`)
  if (!element) return undefined
  const bounds = element.getBoundingClientRect()
  const height = Math.max(1, window.innerHeight || document.documentElement.clientHeight)
  if (bounds.bottom >= 0 && bounds.top <= height) return 0
  if (bounds.bottom >= -height && bounds.top <= height * 2) return 1
  return 2
}

export function DocumentProvider(props: DocumentProviderProps): JSX.Element {
  const seeded = () => props.src ? (props.cache?.get(props.src) ?? props.initialDocument) : props.initialDocument
  const [state, setState] = createStore({
    document: seeded(),
    isLoading: Boolean(props.src) && !(props.src && (props.cache?.get(props.src) ?? props.initialDocument)),
    isRefreshing: false,
    error: null as Error | null,
  })
  const controllers = new Set<AbortController>()
  let inFlight: Promise<DashboardDocument> | undefined
  let inFlightSrc: string | undefined
  let loadedAt = props.src && props.cache?.get(props.src) ? Date.now() : 0
  /** Every filter parameter the runtime has driven so far. */
  const drivenFilterParams = new Set<string>()
  let appliedFilters: FilterValues | undefined
  let filterController: AbortController | undefined

  createEffect(() => {
    props.cache?.configure({ csrf: props.csrf, fetcher: props.fetcher })
  })

  const effectiveSrc = (): string | undefined => {
    const src = props.src
    if (!src || !appliedFilters) return src
    return srcWithFilterParams(src, drivenFilterParams, appliedFilters)
  }

  const refresh = (): Promise<DashboardDocument> => {
    const src = props.src
    if (!src) {
      if (!props.initialDocument) return Promise.reject(new Error('Lens document source is required'))
      setState({ document: props.initialDocument, error: null })
      return Promise.resolve(props.initialDocument)
    }
    const target0 = effectiveSrc() ?? src
    if (inFlight && inFlightSrc === target0) return inFlight
    const controller = new AbortController()
    controllers.add(controller)
    setState('isLoading', true)
    const target = effectiveSrc() ?? src
    // The initial drawer load must join a hover/idle prefetch already in
    // flight. Explicit refreshes bypass the cache once a document has loaded,
    // preserving refresh/recovery semantics.
    const request = props.cache && loadedAt === 0 && target === src
      ? props.cache.load(target)
      : fetchDocument(target, { csrf: props.csrf, fetcher: props.fetcher, signal: controller.signal })
    const pending = request
      .then((next) => {
        if (controller.signal.aborted) return next
        loadedAt = Date.now()
        setState({ document: next, error: null })
        return next
      })
      .catch((cause: unknown) => {
        const nextError = cause instanceof Error ? cause : new Error('document request failed')
        if (!controller.signal.aborted) setState('error', nextError)
        throw nextError
      })
      .finally(() => {
        controllers.delete(controller)
        inFlight = undefined
        if (!controller.signal.aborted) setState('isLoading', false)
      })
    inFlight = pending
    inFlightSrc = target
    return pending
  }

  // A focus-triggered refetch keeps the current document on screen and swaps
  // only on success; a failure is logged and otherwise silent, so a transient
  // network blip never replaces good data with an error state.
  const refreshInBackground = (): void => {
    const src = props.src
    if (!src || inFlight) return
    const controller = new AbortController()
    controllers.add(controller)
    setState('isRefreshing', true)
    void fetchDocument(effectiveSrc() ?? src, { csrf: props.csrf, fetcher: props.fetcher, signal: controller.signal })
      .then((next) => {
        loadedAt = Date.now()
        setState({ document: next, error: null })
      })
      .catch((cause: unknown) => {
        if (!controller.signal.aborted) console.error('[lens] background document refresh failed', cause)
      })
      .finally(() => {
        controllers.delete(controller)
        if (!controller.signal.aborted) setState('isRefreshing', false)
      })
  }

  const applyFilters = (values: FilterValues): void => {
    const src = props.src
    if (!src) return
    for (const name of Object.keys(values)) drivenFilterParams.add(name)
    if (appliedFilters && sameFilterValues(appliedFilters, values)) return
    appliedFilters = values
    // The latest selection wins: a still-flying older filter fetch is stale.
    filterController?.abort()
    const controller = new AbortController()
    filterController = controller
    controllers.add(controller)
    setState('isRefreshing', true)
    void fetchDocument(effectiveSrc() ?? src, { csrf: props.csrf, fetcher: props.fetcher, signal: controller.signal })
      .then((next) => {
        loadedAt = Date.now()
        setState({ document: next, error: null })
      })
      .catch((cause: unknown) => {
        if (controller.signal.aborted) return
        setState('error', cause instanceof Error ? cause : new Error('document request failed'))
      })
      .finally(() => {
        controllers.delete(controller)
        if (filterController === controller) filterController = undefined
        if (!controller.signal.aborted) setState('isRefreshing', false)
      })
  }

  createEffect(on(() => [props.src, props.initialDocument, props.cache], () => {
    const src = props.src
    const cached = src ? props.cache?.get(src) : undefined
    const initial = cached ?? props.initialDocument
    setState({ document: src ? initial : props.initialDocument, error: null })
    if (initial) {
      loadedAt = Date.now()
      setState('isLoading', false)
    } else if (src) {
      void refresh().catch(() => undefined)
    }
  }))

  createEffect(() => {
    const src = props.src
    if (!src || typeof window === 'undefined') return
    const onFocus = () => {
      if (loadedAt > 0 && Date.now() - loadedAt >= staleDocumentAgeMs) refreshInBackground()
    }
    window.addEventListener('focus', onFocus)
    onCleanup(() => window.removeEventListener('focus', onFocus))
  })

  onCleanup(() => {
    for (const controller of controllers) controller.abort()
    controllers.clear()
  })

  const dismissError = (): void => setState('error', null)

  const value: DocumentContextValue = {
    get document() { return state.document },
    get isLoading() { return state.isLoading },
    get isRefreshing() { return state.isRefreshing },
    get error() { return state.error },
    refresh,
    dismissError,
    applyFilters,
  }
  return <DocumentContext.Provider value={value}>{props.children}</DocumentContext.Provider>
}

export interface DashboardContextValue {
  document: DashboardDocument
  navigation: NavigationState
  notice?: string
  dismissNotice: () => void
  canRecompute: boolean
  isRecomputing: boolean
  recompute: () => void
}

export interface DrillContextValue {
  drillInto: (nodeKey: string, panelId?: string) => void
  prefetch: (nodeKeys: string | Array<string>, panelId?: string) => () => void
  back: () => void
  jumpTo: (breadcrumbIndex: number) => void
  /**
   * `enter` first steps into that segment, so picking a view for it costs one
   * transition instead of two with a data-less fork in between.
   */
  switchPerspective: (id: string, options?: { replace?: boolean; enter?: string; panelId?: string }) => void
  reset: () => void
  readonly canGoBack: boolean
}

export interface FiltersContextValue {
  /** Declared controls, empty inside drawers and controlled hosts. */
  filters: Array<Filter>
  /** URL-derived values; a control falls back to its declared value. */
  values: FilterValues
  setPeriod: (filter: Filter, value: PeriodValue) => void
  setCompare: (filter: Filter, value: CompareValue) => void
  setSegmented: (filter: Filter, value: string) => void
  applyURL: (url: string | URL, options?: { newTab?: boolean }) => void
}

export interface DrawerContextValue {
  depth: number
  /** True when open() can open the root drawer or replace its document. */
  readonly canOpen?: boolean
  open: (src: string, opener?: HTMLElement) => void
  openKey: (metricKey: string, opener?: HTMLElement) => void
  close: () => void
  /** Warm a drill-drawer document on hover/focus intent before it is opened. */
  prefetch: (src: string) => () => void
  /** Resolve and warm a compact snapshot-scoped drawer key without opening it. */
  prefetchKey: (metricKey: string) => () => void
  /** Queue bounded best-effort work after a low-cardinality target appears. */
  prefetchIdle: (src: string) => () => void
  prefetchIdleKey: (metricKey: string) => () => void
}

export interface PanelFrameState {
  data?: Frame
  page?: QueryPage
  isStale: boolean
  isLoading: boolean
  error: Error | null
  retry: () => void
  calculation?: PanelCalculation
  summary?: TableSummary
}

export type ExportStatus = 'idle' | 'pending' | 'retry' | 'error'

export interface ExportState {
  status: ExportStatus
  message?: string
}

export interface PanelPaginationContextValue {
  loadPage: (panelId: string, page: number) => Promise<void>
  search: (panelId: string, value: string) => Promise<void>
  sort: (panelId: string, value: TableSort) => Promise<void>
}

export interface ExportContextValue {
  readonly available: boolean
  state: (panelId?: string) => ExportState
  run: (panelId?: string) => Promise<void>
}

export type PrintStatus = 'idle' | 'pending' | 'error'

export interface PrintContextValue {
  readonly available: boolean
  readonly active: boolean
  readonly status: PrintStatus
  readonly message?: string
  readonly report?: PrintReport
  readonly preview: boolean
  /**
   * Builds the report and hands it to the browser's print dialog. In preview
   * mode it stops one step earlier and leaves the composed document on screen —
   * the only way to review printed output, since a print dialog blocks both the
   * page and any automation driving it.
   */
  run: (options?: { preview?: boolean }) => Promise<void>
}

// `AbortSignal.timeout` and `AbortSignal.any` are recent enough that a browser
// still in the field can lack them, and there they would throw where a report
// is being built rather than degrade. The fallbacks are the same contract.
function timeoutSignal(ms: number): AbortSignal {
  if (typeof AbortSignal.timeout === 'function') return AbortSignal.timeout(ms)
  const controller = new AbortController()
  setTimeout(() => controller.abort(new DOMException('signal timed out', 'TimeoutError')), ms)
  return controller.signal
}

function anySignal(signals: Array<AbortSignal>): AbortSignal {
  if (typeof AbortSignal.any === 'function') return AbortSignal.any(signals)
  const controller = new AbortController()
  const aborted = signals.find((signal) => signal.aborted)
  if (aborted) controller.abort(aborted.reason)
  else {
    for (const signal of signals) {
      signal.addEventListener('abort', () => controller.abort(signal.reason), { once: true })
    }
  }
  return controller.signal
}

function exportScope(panelId?: string): string {
  return panelId ? `panel:${panelId}` : 'dashboard'
}

/**
 * One reactive store per panel: the identity of the state object is stable, so
 * a consumer that captured `frame` once keeps reading live properties, while
 * every `set` updates in place. Keys are assigned atomically instead of being
 * reconciled: `reconcile` merges deeply into the store's raw object, and the
 * `data` frame is shared with the caller's document — a merge that rewrites its
 * leaves would corrupt that document.
 */
class PanelFrameStore {
  private readonly entries = new Map<string, { store: PanelFrameState; set: (state: PanelFrameState) => void }>()

  get(panelId: string): PanelFrameState | undefined {
    return this.entries.get(panelId)?.store
  }

  set(panelId: string, state: PanelFrameState): void {
    const existing = this.entries.get(panelId)
    if (existing) existing.set(state)
    else {
      const [store, setStore] = createStore<PanelFrameState>(state)
      this.entries.set(panelId, {
        store,
        set: (next) => {
          // Assignment semantics via produce: a plain object `set` merges,
          // which would splice one frame's rows into another's, and a
          // reconciled merge would rewrite the leaves of `data` — an object
          // shared with the caller's document.
          setStore(produce((current) => {
            current.data = next.data
            current.page = next.page
            current.isStale = next.isStale
            current.isLoading = next.isLoading
            current.error = next.error
            current.retry = next.retry
            current.calculation = next.calculation
            current.summary = next.summary
          }))
        },
      })
    }
  }
}

const DashboardContext = createContext<DashboardContextValue>()
const DrillContext = createContext<DrillContextValue>()
const FramesContext = createContext<PanelFrameStore>()
const PanelPaginationContext = createContext<PanelPaginationContextValue>()
const ExportContext = createContext<ExportContextValue>()
const PrintContext = createContext<PrintContextValue>()
const DrawerContext = createContext<DrawerContextValue>()
const FiltersContext = createContext<FiltersContextValue>()
const LocaleContext = createContext<Accessor<string>>()
const I18nContext = createContext<Accessor<Record<string, string>>>()
const emptyFrameStore = new PanelFrameStore()
const emptyFrameState: PanelFrameState = {
  isStale: false,
  isLoading: false,
  error: null,
  retry: () => undefined,
}

export type TranslationVars = Readonly<Record<string, string | number>>

function translation(
  messages: Record<string, string>,
  key: string,
  fallback: string,
  vars?: TranslationVars,
): string {
  const value = messages[key]
  const text = typeof value === 'string' && value.trim() !== '' ? value : fallback
  if (!vars) return text
  // Placeholders keep word order translatable: a catalogue can move {name}
  // wherever its language needs it.
  return text.replace(/\{(\w+)\}/g, (match, name: string) => (
    name in vars ? String(vars[name]) : match
  ))
}

const browserHistoryKey = '__iotaLensNavigation'

interface BrowserNavigationState {
  view: NavigationView
  history: Array<NavigationView>
}

function sameView(left: NavigationView, right: NavigationView): boolean {
  const sameDrawer = left.drawer === undefined && right.drawer === undefined || (
    left.drawer !== undefined && right.drawer !== undefined && left.drawer.src === right.drawer.src &&
    left.drawer.panelId === right.drawer.panelId && left.drawer.perspectiveId === right.drawer.perspectiveId &&
    left.drawer.path.length === right.drawer.path.length &&
    left.drawer.path.every((key, index) => key === right.drawer?.path[index])
  )
  return left.panelId === right.panelId && left.perspectiveId === right.perspectiveId && sameDrawer &&
    left.path.length === right.path.length && left.path.every((key, index) => key === right.path[index])
}

function resolveView(document: DashboardDocument, view: NavigationView): NavigationView | undefined {
  if (!pathResolves(document, view.path, view.perspectiveId)) return undefined
  return {
    path: [...view.path],
    perspectiveId: view.perspectiveId,
    panelId: panelForNavigation(document, view)?.id,
    ...(view.drawer ? { drawer: { ...view.drawer, path: [...view.drawer.path] } } : {}),
  }
}

function parseBrowserView(value: unknown): NavigationView | undefined {
  if (!value || typeof value !== 'object') return undefined
  const candidate = value as Record<string, unknown>
  if (!Array.isArray(candidate.path) || !candidate.path.every((key) => typeof key === 'string')) return undefined
  if (candidate.panelId !== undefined && typeof candidate.panelId !== 'string') return undefined
  if (candidate.perspectiveId !== undefined && typeof candidate.perspectiveId !== 'string') return undefined
  let drawer: NavigationView['drawer']
  if (candidate.drawer !== undefined) {
    if (!candidate.drawer || typeof candidate.drawer !== 'object') return undefined
    const value = candidate.drawer as Record<string, unknown>
    if (typeof value.src !== 'string' || !Array.isArray(value.path) || !value.path.every((key) => typeof key === 'string')) return undefined
    if (value.panelId !== undefined && typeof value.panelId !== 'string') return undefined
    if (value.perspectiveId !== undefined && typeof value.perspectiveId !== 'string') return undefined
    drawer = {
      src: value.src,
      path: [...value.path] as Array<string>,
      panelId: value.panelId,
      perspectiveId: value.perspectiveId,
    }
  }
  return {
    path: [...candidate.path] as Array<string>,
    panelId: candidate.panelId,
    perspectiveId: candidate.perspectiveId,
    ...(drawer ? { drawer } : {}),
  }
}

function derivedHistory(document: DashboardDocument, view: NavigationView): Array<NavigationView> {
  const history: Array<NavigationView> = []
  for (let length = 0; length < view.path.length; length += 1) {
    const path = view.path.slice(0, length)
    const withPerspective = { path, perspectiveId: view.perspectiveId }
    const candidate = resolveView(document, withPerspective) ?? resolveView(document, { path })
    if (candidate) history.push(candidate)
  }
  if (view.drawer) history.push({ panelId: view.panelId, path: [...view.path], perspectiveId: view.perspectiveId })
  return history
}

function navigationFromBrowserState(
  document: DashboardDocument,
  view: NavigationView,
  state: unknown,
): NavigationState {
  const pendingPath = dynamicParentPath(document, view.path)
  const pendingPanel = pendingPath ? panelForNavigation(document, { ...view, path: pendingPath }) : undefined
  const pending = pendingPath ? { ...view, panelId: pendingPanel?.id } : undefined
  const resolved = resolveView(document, view) ?? pending ?? rootNavigation(document, view.panelId)
  const value = state && typeof state === 'object'
    ? (state as Record<string, unknown>)[browserHistoryKey]
    : undefined
  if (value && typeof value === 'object') {
    const stored = value as Record<string, unknown>
    const storedView = parseBrowserView(stored.view)
    const storedHistory = Array.isArray(stored.history) ? stored.history.map(parseBrowserView) : []
    if (storedView && sameView(resolveView(document, storedView) ?? storedView, resolved) &&
      storedHistory.every((entry): entry is NavigationView => entry !== undefined)) {
      const history = storedHistory.map((entry) => resolveView(document, entry)).filter((entry): entry is NavigationView => Boolean(entry))
      return { ...resolved, history }
    }
  }
  return { ...resolved, history: derivedHistory(document, resolved) }
}

function browserStateFor(navigation: NavigationView, current: unknown): Record<string, unknown> {
  const state = current && typeof current === 'object' ? current as Record<string, unknown> : {}
  const clone = (view: NavigationView): NavigationView => ({
    panelId: view.panelId,
    path: [...view.path],
    perspectiveId: view.perspectiveId,
    ...(view.drawer ? { drawer: { ...view.drawer, path: [...view.drawer.path] } } : {}),
  })
  const history = (navigation as NavigationState).history ?? []
  const navigationState: NavigationState = { ...navigation, history }
  const lens: BrowserNavigationState = { view: clone(navigationState), history: history.map(clone) }
  return { ...state, [browserHistoryKey]: lens }
}

function nestedDrawerState(drawer: NonNullable<NavigationView['drawer']>, history: Array<NavigationView>): NavigationState {
  return {
    panelId: drawer.panelId,
    path: [...drawer.path],
    perspectiveId: drawer.perspectiveId,
    history: history.flatMap((view) => view.drawer ? [{
      panelId: view.drawer.panelId,
      path: [...view.drawer.path],
      perspectiveId: view.drawer.perspectiveId,
    }] : []),
  }
}

function isSameOriginDrawerSource(src: string): boolean {
  if (typeof window === 'undefined') return false
  try {
    return new URL(src, window.location.href).origin === window.location.origin
  } catch {
    return false
  }
}

function inferredInitialNavigation(document: DashboardDocument): NavigationState {
  if (typeof window === 'undefined') return createNavigationState()
  const fromURL = navigationFromURL(new URL(window.location.href))
  return navigationFromBrowserState(document, fromURL, window.history.state)
}

function requestFor(document: DashboardDocument, navigation: NavigationView): QueryRequest {
  return {
    snapshotId: document.snapshotId,
    // The wire shape interleaves point selections with the nodes they select
    // into, so a point-parameterised level is queried for the selected slice
    // rather than for the node's unparameterised aggregate.
    path: queryPathForNavigation(document, navigation.path),
    ...(navigation.perspectiveId ? { perspective: navigation.perspectiveId } : {}),
    ...(navigation.revision ? { revision: navigation.revision } : {}),
  }
}

function firstContentRowReady(document: DashboardDocument): boolean {
  const row = document.layout.rows.find((candidate) => candidate.panels.length > 0)
  if (!row) return false
  return row.panels.every(({ panelId }) => {
    const panel = document.panels.find((candidate) => candidate.id === panelId)
    return Boolean(panel && document.frames[panel.frame])
  })
}

/** Absolute path of the level a node key leads to, resolved against the document. */
function pathForNode(
  document: DashboardDocument,
  state: NavigationState,
  nodeKey: NodeKey,
  panel: Panel | undefined,
  panelChanged: boolean,
): NodePath | undefined {
  const fromRoot = panelChanged || state.path.length === 0
  const level = fromRoot
    ? (panel?.drillRoot ? document.drill.edges[panel.drillRoot] : undefined)
    : levelForPath(document, state.path)
  const base = fromRoot ? level?.path : state.path
  const child = level?.children.find((candidate) => candidate.key === nodeKey)
  const target = child?.target ? document.drill.edges[child.target] : undefined
  if (nodeKey === panel?.drillRoot) return document.drill.edges[nodeKey]?.path
  // A child with an edge is entered through its own key so the path keeps the
  // concrete selection: the level it leads to is parameterised by that point,
  // and collapsing onto the target node's canonical ancestry would make every
  // sibling drill address the same unparameterised level.
  if (child?.target && target && base) return [...base, child.key]
  return target?.path ?? child?.path
}

function runtimeNavigationReducer(
  document: DashboardDocument,
  state: NavigationState,
  action: Parameters<typeof navigationReducer>[1],
): NavigationState {
  if (action.type === 'drillInto') {
    const panelChanged = Boolean(action.panelId && action.panelId !== state.panelId)
    const panel = action.panelId ? document.panels.find((candidate) => candidate.id === action.panelId) : undefined
    const path = pathForNode(document, state, action.nodeKey, panel, panelChanged)
    const perspectiveId = panelChanged ? undefined : state.perspectiveId
    if (!path || !pathResolves(document, path, perspectiveId)) return state
    const next = navigationReducer(state, navigationActions.drillInto(action.nodeKey, action.panelId, path))
    return panelChanged ? { ...next, perspectiveId: undefined } : next
  }
  if (action.type === 'switchPerspective') {
    // With an `enterKey` the perspective belongs to the level that key leads
    // to, not the one on screen: the whole point is to reach it in one step.
    const panel = action.panelId ? document.panels.find((candidate) => candidate.id === action.panelId) : undefined
    const panelChanged = Boolean(action.panelId && action.panelId !== state.panelId)
    const entered = action.enterKey
      ? pathForNode(document, state, action.enterKey, panel, panelChanged)
      : undefined
    if (action.enterKey && !entered) return state
    // At a resting focus-canvas host the navigation path is still empty, so the
    // switch position is the host panel's drill root: a root lens can be picked
    // before any drill has entered a level. The panel id (carried by the root
    // lens selector) is what lets the reducer find that root.
    const level = levelForPath(document, entered ?? state.path)
      ?? (panel?.drillRoot ? document.drill.edges[panel.drillRoot] : undefined)
    // A perspective root's switchable set is its branch's sibling perspectives,
    // not only the refs recorded on the level — the same set the lens selector
    // offers, or a pill click would be silently rejected here.
    if (!level || !perspectivesForPosition(document, level).some((perspective) => perspective.id === action.perspectiveId)) return state
    const perspective = document.perspectives.find((candidate) => candidate.id === action.perspectiveId)
    const root = perspective ? document.drill.edges[perspective.root] : undefined
    if (!root) return state
    return navigationReducer(
      state,
      navigationActions.switchPerspective(action.perspectiveId, root.path, action.replace, undefined, action.panelId),
    )
  }
  if (action.type === 'jumpTo') {
    const next = navigationReducer(state, action)
    if (next === state || pathResolves(document, next.path, next.perspectiveId)) return next
    return pathResolves(document, next.path) ? { ...next, perspectiveId: undefined } : state
  }
  return navigationReducer(state, action)
}

function frameForPanel(
  document: DashboardDocument,
  navigation: NavigationView,
  panel: Panel,
  loadedFrames: ReadonlyMap<string, Frame>,
): { frame?: Frame; shouldQuery: boolean } {
  const active = panelForNavigation(document, navigation)
  if (!active || active.id !== panel.id || navigation.path.length === 0) {
    return { frame: document.frames[panel.frame], shouldQuery: false }
  }
  // From here on the panel is showing a drill level, and the invariant is
  // absolute: it may only render a frame that belongs to the level on screen.
  // Falling back to the panel's own frame would put the parent's numbers under
  // the child's title — numbers that look plausible and are wrong, which is
  // the same failure the browser-Back path used to have.
  const level = levelForPath(document, navigation.path)
  if (!level) return { shouldQuery: false }
  if (level.frame) {
    const frame = loadedFrames.get(level.frame) ?? document.frames[level.frame]
    if (frame) return { frame, shouldQuery: false }
    return { shouldQuery: Boolean(document.endpoints.query) }
  }
  // A fork has nothing to fetch until a perspective is chosen; any other
  // frameless level is asked for from the query endpoint.
  if (isPerspectiveFork(document, level)) return { shouldQuery: false }
  return { shouldQuery: Boolean(document.endpoints.query) }
}

/** Deep copy of JSON-shaped data; non-plain values pass through by reference. */
function detachDocument<T>(value: T): T {
  if (Array.isArray(value)) {
    return (value as Array<unknown>).map((entry) => detachDocument(entry)) as unknown as T
  }
  if (value !== null && typeof value === 'object') {
    if (Object.getPrototypeOf(value) !== Object.prototype) return value
    const source = value as Record<string, unknown>
    const out: Record<string, unknown> = {}
    for (const key of Object.keys(source)) out[key] = detachDocument(source[key])
    return out as T
  }
  return value
}

interface RuntimeCoreProps {
  document: DashboardDocument
  locale: string
  csrf?: string
  fetcher?: typeof fetch
  refreshDocument: () => Promise<DashboardDocument>
  applyFilters?: (values: FilterValues) => void
  /** Factory: the board is instantiated twice when a drawer is open (board + drawer body). */
  children: () => JSX.Element
  controlledNavigation?: NavigationState
  onControlledNavigationChange?: (view: NavigationView) => void
  onDrawerNavigate?: (src: string) => void
  drawerDepth?: number
}

function RuntimeCore(props: RuntimeCoreProps): JSX.Element {
  const drawerDepth = props.drawerDepth ?? 0
  const refreshDocument = props.refreshDocument
  const applyFilters = props.applyFilters

  // The resolved document (with inline frame children merged) as plain data:
  // reducers and request builders must read it untracked, while the store below
  // is the reactive mirror the UI reads. `docVersion` fires on every swap.
  let resolvedDoc = withInlineFrameChildren(props.document)
  const [docVersion, bumpDocVersion] = createSignal(0, { equals: false })
  // The store's document is a detached copy: store setters (reconcile in
  // particular) write into their raw object, and the raw must never be the
  // caller's document, whose objects are shared with hosts and caches. Plain
  // data is deep-copied; anything non-plain (class instances, functions) is
  // kept by reference, since a store would not merge through it anyway.
  let navigationState = inferredInitialNavigation(resolvedDoc)
  const initialControlled = props.controlledNavigation
  const initialResolved = initialControlled && resolveView(resolvedDoc, initialControlled)
  const initialNavigation = initialControlled
    ? initialResolved ? { ...initialResolved, history: initialControlled.history } : initialControlled
    : navigationState
  const [dashboard, setDashboard] = createStore<DashboardContextValue>({
    document: detachDocument(resolvedDoc),
    navigation: initialNavigation,
    dismissNotice: () => setDashboard('notice', undefined),
    canRecompute: false,
    isRecomputing: false,
    recompute: () => recompute(),
  })
  createEffect(on(() => props.document, (source) => {
    resolvedDoc = withInlineFrameChildren(source)
    bumpDocVersion(0)
    setDashboard('document', reconcile(resolvedDoc))
  }))

  // Plain navigation state is the source of truth; the store mirrors it.
  const [navVersion, setNavVersion] = createSignal(0, { equals: false })
  const currentNavigation = createMemo<NavigationState>(() => {
    const controlled = props.controlledNavigation
    if (controlled) {
      // A drawer source can deep-link with path/perspective before its document
      // is loaded, so the outer runtime cannot know the nested host panel yet.
      // Once the document arrives, resolve the panel from that path locally;
      // otherwise ExplorePanel would keep rendering its resting root while the
      // query runs against the correct deeper level.
      void docVersion()
      const resolved = resolveView(resolvedDoc, controlled)
      return resolved ? { ...resolved, history: controlled.history } : controlled
    }
    void navVersion()
    return navigationState
  })
  let navigationPlain = currentNavigation()
  createEffect(() => {
    navigationPlain = currentNavigation()
    setDashboard('navigation', reconcile(navigationPlain))
  })

  let runtimeViewSnapshot: NavigationView | undefined
  const runtimeView = createMemo<NavigationView>(() => {
    const navigation = currentNavigation()
    if (!runtimeViewSnapshot ||
      runtimeViewSnapshot.panelId !== navigation.panelId ||
      runtimeViewSnapshot.perspectiveId !== navigation.perspectiveId ||
      runtimeViewSnapshot.path.length !== navigation.path.length ||
      runtimeViewSnapshot.path.some((key, index) => key !== navigation.path[index])) {
      runtimeViewSnapshot = {
        panelId: navigation.panelId,
        path: [...navigation.path],
        perspectiveId: navigation.perspectiveId,
      }
    }
    return runtimeViewSnapshot
  })

  const dispatch = (action: Parameters<typeof navigationReducer>[1]): void => {
    if (!props.controlledNavigation) {
      const next = runtimeNavigationReducer(resolvedDoc, navigationState, action)
      if (next === navigationState) return
      batch(() => {
        navigationState = next
        setNavVersion(0)
      })
      return
    }
    const current = navigationPlain
    const next = runtimeNavigationReducer(resolvedDoc, current, action)
    if (next !== current) props.onControlledNavigationChange?.(next)
  }

  const [notice, setNotice] = createSignal<string>()
  createEffect(() => {
    setDashboard('notice', notice())
  })

  createEffect(() => {
    const doc = dashboard.document
    const endpoint = doc.endpoints.release
    const csrf = props.csrf
    const fetcher = props.fetcher
    const snapshotId = doc.snapshotId
    if (!endpoint) return
    const key = `${endpoint}\n${snapshotId}`
    let active = true
    let dispose: (() => void) | undefined
    void import('./snapshotLease').then(({ acquireSnapshotLease }) => {
      const acquired = acquireSnapshotLease(key, () => {
        void (fetcher ?? fetch)(endpoint, {
          method: 'POST', credentials: 'same-origin', keepalive: true,
          headers: { 'Content-Type': 'application/json', ...(csrf ? { 'X-CSRF-Token': csrf } : {}) },
          body: JSON.stringify({ snapshotId }),
        }).catch(() => undefined)
      })
      if (!active) {
        acquired()
        return
      }
      dispose = acquired
    })
    onCleanup(() => {
      active = false
      dispose?.()
    })
  })

  const filtersEnabled = !props.controlledNavigation && drawerDepth === 0
  // Filter state derives from the URL alone: user actions write the URL, the
  // URL drives the refetch, and browser Back needs no resync timers because
  // popstate re-reads the same source of truth.
  const [filterValues, setFilterValues] = createSignal<FilterValues>(
    typeof window === 'undefined' ? {} : readFilterValues(resolvedDoc, new URL(window.location.href)),
  )
  let filterValuesCurrent = filterValues()
  createEffect(() => {
    filterValuesCurrent = filterValues()
  })

  // The first document fetch follows the host's raw deep link. Once that
  // server-normalized document declares its filters, replace impossible URL
  // combinations (notably all-time + comparison) with the same canonical
  // values the controls and subsequent requests use.
  createEffect(() => {
    const enabled = filtersEnabled && typeof window !== 'undefined'
    void docVersion()
    if (!enabled) return
    const current = new URL(window.location.href)
    const canonical = writeFilterValues(current, resolvedDoc, filterValuesCurrent)
    if (!sameNavigationURL(current, canonical)) {
      window.history.replaceState(window.history.state, '', canonical)
    }
  })

  const [exportStates, setExportStates] = createStore<Record<string, ExportState>>({})
  const [printState, setPrintState] = createStore<Omit<PrintContextValue, 'available' | 'run'>>({
    active: false,
    status: 'idle',
    preview: false,
  })
  let exportSnapshotId = resolvedDoc.snapshotId
  createEffect(() => {
    const snapshotId = dashboard.document.snapshotId
    if (exportSnapshotId === snapshotId) return
    exportSnapshotId = snapshotId
    setExportStates(reconcile({}))
  })
  const [retryToken, bumpRetryToken] = createSignal(0, { equals: false })
  let forceRetry = false
  // eslint-disable-next-line prefer-const -- reassigned after the closures below are defined
  let pageLoader: (panelId: string, page: number, force?: boolean) => Promise<void>
  // eslint-disable-next-line prefer-const -- reassigned after the closures below are defined
  let searchLoader: (panelId: string, search: string) => Promise<void>
  // eslint-disable-next-line prefer-const -- reassigned after the closures below are defined
  let sortLoader: (panelId: string, sort: TableSort) => Promise<void>
  const tableRequestState = new Map<string, { search?: string; sort?: TableSort }>()
  let replaceNextURL = true
  let drawerOpener: HTMLElement | undefined
  // The drawer portals to body and stacks above an expanded panel, so it carries
  // the theme of the root it opened from — captured from the opener when the
  // drawer opens, mirroring PanelFrame's overlay theme capture.
  const drawerTheme: { theme?: string; dark: boolean } = { dark: false }
  const drawerKeySources = new Map<string, string>()
  const drawerKeyResolvers = new Map<string, Promise<string>>()
  const drawerCache = createMemo<Pick<DocumentCache, 'configure' | 'get' | 'load' | 'prefetch'>>(() => {
    let resolved: DocumentCache | undefined
    let options: { csrf?: string; fetcher?: typeof fetch } = {}
    const ready = import('./prefetch').then(({ DocumentCache: Cache }) => {
      resolved = new Cache({ capacity: 8, ...options })
      return resolved
    })
    return {
      configure: (next) => {
        options = next
        resolved?.configure(next)
      },
      get: (src) => resolved?.get(src),
      load: async (src) => (await ready).load(src),
      prefetch: async (src, signal) => (await ready).prefetch(src, signal),
    }
  })
  const [drawerIdleQueue, setDrawerIdleQueue] = createSignal<IdlePrefetchQueue>()
  createEffect(() => {
    drawerCache().configure({ csrf: props.csrf, fetcher: props.fetcher })
  })
  createEffect(() => {
    const doc = dashboard.document
    void doc.endpoints.drawer
    void doc.snapshotId
    drawerKeySources.clear()
    drawerKeyResolvers.clear()
  })
  createEffect(() => {
    // Idle speculation is deliberately a dynamic feature chunk: it is not on
    // the first-paint path, and panels re-register when the queue becomes
    // available. Snapshot changes dispose the old queue before importing the
    // next one, so no candidate can cross scopes.
    const snapshotId = dashboard.document.snapshotId
    if (!snapshotId || drawerDepth !== 0) {
      setDrawerIdleQueue(undefined)
      return
    }
    let queue: IdlePrefetchQueue | undefined
    void import('./idlePrefetch').then(({ IdlePrefetchQueue: Queue }) => {
      queue = new Queue({ capacity: 8 })
      setDrawerIdleQueue(queue)
    })
    onCleanup(() => {
      queue?.reset()
      setDrawerIdleQueue(undefined)
    })
  })
  createEffect(() => {
    const queue = drawerIdleQueue()
    const drawer = dashboard.navigation.drawer
    queue?.setPaused(Boolean(drawer))
  })

  const frames = new PanelFrameStore()
  const [panelAttempts, setPanelAttempts] = createSignal<Record<string, number>>({})
  const panelForces = new Set<string>()
  const launchedPanelAttempts = new Map<string, number>()
  const recomputePending = new Set<string>()
  const [isRecomputing, setIsRecomputing] = createSignal(false)
  createEffect(() => {
    setDashboard('isRecomputing', isRecomputing())
  })
  const schedulePanel = (panelId: string, recompute = false): void => {
    if (recompute) panelForces.add(panelId)
    setPanelAttempts((current) => ({ ...current, [panelId]: (current[panelId] ?? 0) + 1 }))
  }
  const retryFrame = (): void => {
    forceRetry = true
    bumpRetryToken(0)
  }
  const initializeFrameStore = (source: DashboardDocument): void => {
    launchedPanelAttempts.clear()
    for (const panel of source.panels) {
      frames.set(panel.id, {
        data: source.frames[panel.frame],
        isStale: false,
        isLoading: Boolean(panel.deferred),
        error: null,
        retry: panel.deferred ? () => schedulePanel(panel.id) : retryFrame,
      })
    }
  }
  initializeFrameStore(props.document)
  /**
   * A filter change invalidates the whole board at once.
   *
   * Every figure on screen belongs to the period the reader just left, and it
   * says so before the first request resolves rather than after: on production
   * volume the document alone takes seconds, and for that whole time the old
   * numbers used to sit there at full strength under the new chip. The set also
   * has to move together — a board that dims in eight staggered steps, one per
   * response, is worse than one that never dims at all.
   *
   * A panel with data keeps it, dimmed and marked stale, so the reader keeps
   * their bearings; only a panel with nothing to dim falls back to a skeleton.
   */
  const markPanelFramesPending = (): void => {
    for (const panel of props.document.panels) {
      const current = frames.get(panel.id)
      if (!current) continue
      frames.set(panel.id, {
        ...current,
        isStale: true,
        isLoading: !current.data,
        error: null,
      })
    }
  }
  const syncFiltersFromURL = (): void => {
    if (typeof window === 'undefined') return
    const values = readFilterValues(resolvedDoc, new URL(window.location.href))
    if (sameFilterValues(values, filterValuesCurrent)) return
    filterValuesCurrent = values
    setFilterValues(values)
    // The single funnel every filter action passes through — a period chip, a
    // comparison mode, a facet apply, browser Back — so the staleness of the
    // board is decided in one place and cannot depend on which control moved.
    // Only a host that can actually refetch may dim: marking panels stale with
    // nothing in flight to clear them would leave the board dim for good.
    if (applyFilters) {
      markPanelFramesPending()
      applyFilters(values)
    }
  }
  const translate = (key: string, fallback: string): string => translation(resolvedDoc.i18n, key, fallback)
  const driftNotice = (): string => translate(
    'drill.reset',
    'The previous drill path is no longer available. Lens returned to the root view.',
  )
  for (const panel of resolvedDoc.panels) {
    if (!frames.get(panel.id)) {
      frames.set(panel.id, {
        data: resolvedDoc.frames[panel.frame],
        isStale: false,
        isLoading: Boolean(panel.deferred),
        error: null,
        retry: panel.deferred ? () => schedulePanel(panel.id) : retryFrame,
      })
    }
  }
  const queryClient = createMemo(() => {
    const endpoint = resolvedDoc.endpoints.query
    const csrf = props.csrf
    const fetcher = props.fetcher
    void docVersion()
    return endpoint ? new QueryClient(endpoint, { csrf, fetcher }) : undefined
  })
  const panelClient = createMemo(() => {
    const endpoint = props.document.endpoints.panel
    const csrf = props.csrf
    const fetcher = props.fetcher
    return endpoint ? new PanelClient(endpoint, { csrf, fetcher }) : undefined
  })
  createEffect(() => {
    const client = queryClient()
    onCleanup(() => client?.dispose())
  })
  createEffect(() => {
    const client = panelClient()
    onCleanup(() => client?.dispose())
  })
  createEffect(() => {
    setDashboard('canRecompute', Boolean(panelClient()))
  })

  const prefetchDrill = (nodeKeys: string | Array<string>, panelId?: string): (() => void) => {
    const client = untrack(() => queryClient())
    if (!client) return () => undefined
    const source = resolvedDoc
    let next = navigationPlain
    for (const nodeKey of Array.isArray(nodeKeys) ? nodeKeys : [nodeKeys]) {
      const resolved = runtimeNavigationReducer(source, next, navigationActions.drillInto(nodeKey, panelId))
      if (resolved === next) return () => undefined
      next = resolved
      panelId = undefined
    }
    const pendingPath = dynamicParentPath(source, next.path)
    const queryView = pendingPath ? { ...next, path: pendingPath } : next
    const targetPanel = panelForNavigation(source, queryView)
    if (!targetPanel || !frameForPanel(source, queryView, targetPanel, new Map()).shouldQuery) return () => undefined
    const perspective = source.perspectives.find(({ id }) => id === queryView.perspectiveId)
    const request: QueryRequest = {
      ...requestFor(source, queryView),
      prefetch: true,
      ...(targetPanel.kind === 'table' || perspective?.semantics === 'evidence' ? { page: 1 } : {}),
    }
    const controller = new AbortController()
    void client.query(request, { signal: controller.signal }).then((response) => {
      const frame = Object.values(response.frames)[0]
      if (frame?.children) {
        setResolvedDocument((current) => withFrameChildren(current, queryView.path, frame))
      }
    }).catch(() => undefined)
    return () => controller.abort(new DOMException('drill intent ended', 'AbortError'))
  }

  const setResolvedDocument = (update: (current: DashboardDocument) => DashboardDocument): void => {
    const next = update(resolvedDoc)
    // Returning the current document verbatim lets the runtime skip the store
    // update when the change was an identity no-op (e.g. frame children that
    // are already merged).
    if (next === resolvedDoc) return
    resolvedDoc = next
    bumpDocVersion(0)
    setDashboard('document', reconcile(next))
  }

  const rootContentReady = createMemo(() => firstContentRowReady(dashboard.document))
  createEffect(() => {
    const client = queryClient()
    const ready = rootContentReady()
    if (!client || drawerDepth !== 0 || !ready) return
    // Runtime child frames enrich the document, but they must not restart the
    // speculative traversal that produced them. Scope the worker to the source
    // snapshot and let it own that snapshot until cancellation or completion.
    const prefetchDocument = resolvedDoc
    const controller = new AbortController()
    void import('./drillPrefetch').then(({ prefetchIdleDrillStates }) => prefetchIdleDrillStates({
      document: prefetchDocument,
      queryClient: client,
      signal: controller.signal,
      onChildren: (path, frame) => {
        setResolvedDocument((current) => withFrameChildren(current, path, frame))
      },
    })).catch(() => undefined)
    onCleanup(() => controller.abort(new DOMException('idle drill prefetch scope changed', 'AbortError')))
  })

  // A document can legitimately return to an earlier snapshot through browser
  // Back. Panel attempts are scoped to the active document lifecycle, not to a
  // snapshot forever: retaining the old `${snapshot}:${panel}` keys makes every
  // deferred panel in the restored document stay on its skeleton because the
  // runtime mistakes a previous hydration for the current one.
  createEffect(on(() => props.document, (source) => {
    initializeFrameStore(source)
  }))

  createEffect(() => {
    const source = props.document
    const attempts = panelAttempts()
    // Read once, untracked: the client is replaced whenever the document is,
    // and a second trigger wave here would relaunch every deferred panel.
    const client = untrack(panelClient)
    if (!client) return
    const pending: Array<{ panel: Panel; attempt: number; force: boolean; key: string; previous?: Frame; retry: () => void }> = []
    for (const panel of source.panels) {
      const attempt = attempts[panel.id] ?? 0
      const force = panelForces.has(panel.id)
      if (!panel.deferred && !force) continue
      const key = `${source.snapshotId}:${panel.id}`
      if (launchedPanelAttempts.get(key) === attempt) continue
      launchedPanelAttempts.set(key, attempt)
      panelForces.delete(panel.id)
      const previous = frames.get(panel.id)?.data
      const retry = () => schedulePanel(panel.id)
      frames.set(panel.id, {
        data: previous,
        isStale: Boolean(previous),
        isLoading: true,
        error: null,
        retry,
        calculation: frames.get(panel.id)?.calculation,
      })
      pending.push({ panel, attempt, force, key, previous, retry })
    }
    if (pending.length === 0) return
    const pendingByID = new Map(pending.map((item) => [item.panel.id, item]))
    const received = new Set<string>()
    const applyPanelResult = (panelId: string, response: PanelBatchResult) => {
      const item = pendingByID.get(panelId)
      if (!item) return
      received.add(panelId)
      const { panel, attempt, key, previous, retry } = item
      if (launchedPanelAttempts.get(key) !== attempt) return
      if (response.error || !response.frames) {
        // A classified failure travels as a QueryError so the panel can say
        // what happened. A plain Error here would flatten every cause back into
        // the one generic sentence, which is what the frame used to show.
        const failure = response.error
          ? new QueryError(response.error.error, response.error.message, 200, response.error.reason)
          : new Error(`panel ${panel.id} response has no frames`)
        frames.set(panel.id, {
          data: previous, isStale: Boolean(previous), isLoading: false,
          error: failure,
          retry, calculation: frames.get(panel.id)?.calculation,
        })
        return
      }
      const loaded = response.frames[panel.frame] ?? Object.values(response.frames)[0]
      if (!loaded) {
        frames.set(panel.id, { data: previous, isStale: Boolean(previous), isLoading: false, error: new Error(`panel ${panel.id} response has no frame`), retry })
        return
      }
      frames.set(panel.id, {
        data: loaded, page: response.page, isStale: false, isLoading: false, error: null, retry,
        calculation: response.calculation,
        summary: response.summary,
      })
      setResolvedDocument((current) => {
        if (current.snapshotId !== source.snapshotId || current.frames[panel.frame] === loaded) return current
        return { ...current, frames: { ...current.frames, [panel.frame]: loaded } }
      })
    }
    void client.loadBatch(pending.map(({ panel, force }) => ({
      snapshotId: source.snapshotId, panelId: panel.id, ...(force ? { recompute: true } : {}),
      viewportRank: panelViewportRank(panel.id),
    })), { onResult: applyPanelResult }).catch((cause: unknown) => {
      // A cancelled batch is not a failed one. Every other rejection path in
      // this file checks its signal first; this one did not, so unmounting the
      // runtime — or any refetch that disposes the client while a slow panel is
      // still in flight — painted «Не удалось отобразить панель» on whichever
      // panels had not answered yet. On the sales report that is reliably the
      // region map, which takes ~10s uncached while its siblings return in
      // under one, so it alone showed an error beside nine working panels.
      if (PanelClient.isAbort(cause)) return
      for (const { panel, attempt, key, previous, retry } of pending) {
        if (received.has(panel.id)) continue
        if (launchedPanelAttempts.get(key) !== attempt) continue
        if (cause instanceof SnapshotGoneError) {
          void refreshDocument().catch(() => undefined)
          break
        }
        frames.set(panel.id, {
          data: previous,
          isStale: Boolean(previous),
          isLoading: false,
          error: cause instanceof Error ? cause : new Error('panel request failed'),
          retry,
          calculation: frames.get(panel.id)?.calculation,
        })
      }
    }).finally(() => {
      for (const { panel, force } of pending) {
        if (!force) continue
        if (!recomputePending.delete(panel.id)) continue
        if (recomputePending.size === 0) setIsRecomputing(false)
      }
    })
  })

  const recompute = (): void => {
    const client = panelClient()
    const source = props.document
    if (!client || source.panels.length === 0) return
    const ids = source.panels.map(({ id }) => id)
    recomputePending.clear()
    for (const id of ids) recomputePending.add(id)
    for (const id of ids) panelForces.add(id)
    setIsRecomputing(true)
    setPanelAttempts((current) => {
      const next = { ...current }
      for (const id of ids) next[id] = (next[id] ?? 0) + 1
      return next
    })
  }

  // Leaving a drill level (Back, a breadcrumb jump, a reset) must not leave the
  // level's data on screen: any explore host that is no longer the active drill
  // target falls back to the frame the document shipped.
  createEffect(on([runtimeView, docVersion], () => {
    const doc = resolvedDoc
    const view = runtimeView()
    if (pathResolves(doc, view.path, view.perspectiveId) ||
      dynamicParentPath(doc, view.path)) return
    replaceNextURL = true
    dispatch(navigationActions.restore(rootNavigation(doc, view.panelId)))
    setNotice(driftNotice())
  }))

  createEffect(on([currentNavigation, docVersion], () => {
    if (props.controlledNavigation) return
    if (typeof window === 'undefined') return
    const navigation = navigationPlain
    if (!pathResolves(resolvedDoc, navigation.path, navigation.perspectiveId)) return
    const current = new URL(window.location.href)
    const next = navigationToURL(navigation, current)
    const state = browserStateFor(navigation, window.history.state)
    if (replaceNextURL || sameNavigationURL(current, next)) window.history.replaceState(state, '', next)
    else window.history.pushState(state, '', next)
    replaceNextURL = false
  }))

  createEffect(on([docVersion, () => props.controlledNavigation], () => {
    if (props.controlledNavigation) return
    if (typeof window === 'undefined') return
    const onPopState = (event: PopStateEvent) => {
      const url = new URL(window.location.href)
      const view = navigationFromURL(url)
      const restored = navigationFromBrowserState(resolvedDoc, view, event.state)
      dispatch(navigationActions.restore(restored, restored.history))
      // Marking the panels stale is `syncFiltersFromURL`'s job now: Back is one
      // more way of changing the filter values, not a case of its own.
      syncFiltersFromURL()
    }
    window.addEventListener('popstate', onPopState)
    onCleanup(() => window.removeEventListener('popstate', onPopState))
  }))

  createEffect(on([runtimeView, docVersion, retryToken], () => {
    const doc = resolvedDoc
    const client = queryClient()
    const view = runtimeView()
    const pendingPath = dynamicParentPath(doc, view.path)
    const queryView = pendingPath ? { ...view, path: pendingPath } : view
    const panel = panelForNavigation(doc, queryView)
    // Leaving a drill level (Back, a breadcrumb jump, a reset) must not leave
    // the level's data on screen: any explore host that is no longer the
    // active drill target falls back to the frame the document shipped.
    for (const candidate of doc.panels) {
      if (!candidate.drillRoot || candidate.id === panel?.id) continue
      const documentFrame = doc.frames[candidate.frame]
      if (!documentFrame || frames.get(candidate.id)?.data === documentFrame) continue
      frames.set(candidate.id, {
        data: documentFrame, isStale: false, isLoading: false, error: null, retry: retryFrame,
      })
    }
    if (!panel) return
    const resolved = frameForPanel(doc, queryView, panel, new Map())
    if (!resolved.shouldQuery || !client) {
      if (resolved.frame) {
        frames.set(panel.id, {
          data: resolved.frame, isStale: false, isLoading: false, error: null,
          retry: retryFrame,
        })
      }
      return
    }

    const previous = frames.get(panel.id)?.data ?? doc.frames[panel.frame]
    frames.set(panel.id, {
      data: previous,
      isStale: Boolean(previous),
      isLoading: true,
      error: null,
      retry: retryFrame,
    })
    const currentNavigation = { ...queryView, path: [...queryView.path] }
    const perspective = doc.perspectives.find(({ id }) => id === currentNavigation.perspectiveId)
    const request = {
      ...requestFor(doc, currentNavigation),
      ...(panel.kind === 'table' || perspective?.semantics === 'evidence' ? { page: 1 } : {}),
    }
    const force = forceRetry
    forceRetry = false
    void queryWithSnapshotRecovery({
      request,
      navigation: currentNavigation,
      loadDocument: refreshDocument,
      query: (next) => client.query(next, { force }),
    }).then((result) => {
      if (result.reset) {
        replaceNextURL = true
        dispatch(navigationActions.restore(result.navigation))
        setNotice(driftNotice())
        return
      }
      const frameEntries = Object.entries(result.response.frames)
      const frame = frameEntries[0]?.[1]
      if (frame?.children) {
        setResolvedDocument((current) => withFrameChildren(current, currentNavigation.path, frame))
      }
      frames.set(panel.id, {
        data: frame ?? previous,
        page: result.response.page,
        isStale: false,
        isLoading: false,
        error: null,
        retry: retryFrame,
      })
    }).catch((cause: unknown) => {
      const error = cause instanceof Error ? cause : new Error('query request failed')
      frames.set(panel.id, {
        data: previous,
        isStale: Boolean(previous),
        isLoading: false,
        error,
        retry: retryFrame,
      })
    })
  }))

  const loadPage = async (panelId: string, page: number, force = false): Promise<void> => {
    const source = props.document
    const sourcePanel = source.panels.find((candidate) => candidate.id === panelId)
    const client = panelClient()
    if (client && sourcePanel?.table?.searchable && page >= 1) {
      const previousState = frames.get(panelId)
      const previous = previousState?.data ?? source.frames[sourcePanel.frame]
      const retry = () => { void pageLoader(panelId, page, true) }
      frames.set(panelId, { ...previousState, data: previous, isStale: Boolean(previous), isLoading: true, error: null, retry })
      try {
        const response = await client.load({
          snapshotId: source.snapshotId, panelId, ...tableRequestState.get(panelId), page,
        })
        const loaded = response.frames[sourcePanel.frame] ?? Object.values(response.frames)[0]
        if (!loaded) throw new Error(`panel ${panelId} response has no frame`)
        frames.set(panelId, { data: loaded, page: response.page, isStale: false, isLoading: false, error: null, retry, calculation: response.calculation, summary: response.summary })
      } catch (cause: unknown) {
        frames.set(panelId, { ...previousState, data: previous, isStale: Boolean(previous), isLoading: false, error: cause instanceof Error ? cause : new Error('panel page failed'), retry })
      }
      return
    }
    const doc = resolvedDoc
    const view = runtimeView()
    const panel = panelForNavigation(doc, view)
    const qClient = queryClient()
    if (!qClient || !panel || panel.id !== panelId || view.path.length === 0 || page < 1) return
    const previousState = frames.get(panelId)
    const previous = previousState?.data ?? doc.frames[panel.frame]
    const retryPage = () => { void pageLoader(panelId, page, true) }
    frames.set(panelId, {
      data: previous,
      page: previousState?.page,
      isStale: Boolean(previous),
      isLoading: true,
      error: null,
      retry: retryPage,
    })
    const currentNavigation = { ...view, path: [...view.path] }
    try {
      const result = await queryWithSnapshotRecovery({
        request: { ...requestFor(doc, currentNavigation), page },
        navigation: currentNavigation,
        loadDocument: refreshDocument,
        query: (next) => qClient.query(next, { force }),
      })
      if (result.reset) {
        replaceNextURL = true
        dispatch(navigationActions.restore(result.navigation))
        setNotice(driftNotice())
        return
      }
      const frame = Object.values(result.response.frames)[0]
      frames.set(panelId, {
        data: frame ?? previous,
        page: result.response.page,
        isStale: false,
        isLoading: false,
        error: null,
        retry: retryPage,
      })
    } catch (cause: unknown) {
      frames.set(panelId, {
        data: previous,
        page: previousState?.page,
        isStale: Boolean(previous),
        isLoading: false,
        error: cause instanceof Error ? cause : new Error('query request failed'),
        retry: retryPage,
      })
    }
  }
  pageLoader = loadPage

  const searchPanel = async (panelId: string, search: string): Promise<void> => {
    const client = panelClient()
    if (!client) return
    const panel = props.document.panels.find((candidate) => candidate.id === panelId)
    if (!panel?.table?.searchable) return
    const previousState = frames.get(panelId)
    const previous = previousState?.data ?? props.document.frames[panel.frame]
    const requestState = { ...tableRequestState.get(panelId), search: search || undefined }
    tableRequestState.set(panelId, requestState)
    const retry = () => { void searchLoader(panelId, search) }
    frames.set(panelId, {
      data: previous, isStale: Boolean(previous), isLoading: !previous, error: null, retry,
      calculation: previousState?.calculation, summary: previousState?.summary,
    })
    try {
      const response = await client.load({ snapshotId: props.document.snapshotId, panelId, ...requestState })
      const loaded = response.frames[panel.frame] ?? Object.values(response.frames)[0]
      if (!loaded) throw new Error(`panel ${panel.id} response has no frame`)
      frames.set(panelId, {
        data: loaded, page: response.page, isStale: false, isLoading: false, error: null, retry,
        calculation: response.calculation, summary: response.summary,
      })
    } catch (cause: unknown) {
      if (cause instanceof SnapshotGoneError) {
        await refreshDocument().catch(() => undefined)
        return
      }
      frames.set(panelId, {
        data: previous, isStale: Boolean(previous), isLoading: false,
        error: cause instanceof Error ? cause : new Error('panel search failed'), retry,
        calculation: previousState?.calculation, summary: previousState?.summary,
      })
    }
  }
  searchLoader = searchPanel

  const sortPanel = async (panelId: string, sort: TableSort): Promise<void> => {
    const client = panelClient()
    if (!client) return
    const panel = props.document.panels.find((candidate) => candidate.id === panelId)
    if (!panel || panel.presentation?.sortable === false) return
    const requestState = { ...tableRequestState.get(panelId), sort }
    tableRequestState.set(panelId, requestState)
    const previousState = frames.get(panelId)
    const previous = previousState?.data ?? props.document.frames[panel.frame]
    const retry = () => { void sortLoader(panelId, sort) }
    frames.set(panelId, { data: previous, isStale: Boolean(previous), isLoading: !previous, error: null, retry, summary: previousState?.summary })
    try {
      const response = await client.load({ snapshotId: props.document.snapshotId, panelId, ...requestState })
      const loaded = response.frames[panel.frame] ?? Object.values(response.frames)[0]
      if (!loaded) throw new Error(`panel ${panel.id} response has no frame`)
      frames.set(panelId, { data: loaded, page: response.page, isStale: false, isLoading: false, error: null, retry, calculation: response.calculation, summary: response.summary })
    } catch (cause: unknown) {
      frames.set(panelId, { data: previous, isStale: Boolean(previous), isLoading: false, error: cause instanceof Error ? cause : new Error('panel sort failed'), retry, summary: previousState?.summary })
    }
  }
  sortLoader = sortPanel

  const pagination: PanelPaginationContextValue = {
    loadPage: (panelId, page) => loadPage(panelId, page),
    search: (panelId, value) => searchPanel(panelId, value),
    sort: (panelId, value) => sortPanel(panelId, value),
  }

  const runExport = async (panelId?: string): Promise<void> => {
    const scope = exportScope(panelId)
    const exportEndpoint = resolvedDoc.endpoints.export
    if (!exportEndpoint) return
    setExportStates(scope, reconcile({ status: 'pending' }))
    try {
      const exporter = await import('./export')
      const workbook = await exporter.exportWorkbook({
        endpoint: exportEndpoint,
        snapshotId: resolvedDoc.snapshotId,
        panelId,
        csrf: props.csrf,
        fetcher: props.fetcher,
      })
      exporter.downloadWorkbook(workbook)
      setExportStates(scope, reconcile({ status: 'idle' }))
    } catch (cause: unknown) {
      const { ExportSnapshotGoneError } = await import('./export')
      if (cause instanceof ExportSnapshotGoneError) {
        try {
          await refreshDocument()
          setExportStates(scope, reconcile({
            status: 'retry', message: translate('export.retryHint', 'Snapshot refreshed. Retry export.'),
          }))
        } catch (refreshCause: unknown) {
          const message = refreshCause instanceof Error ? refreshCause.message : 'Snapshot refresh failed'
          setExportStates(scope, reconcile({ status: 'error', message }))
        }
        return
      }
      const message = cause instanceof Error ? cause.message : 'Export failed'
      setExportStates(scope, reconcile({ status: 'error', message }))
    }
  }

  const runPrint = async ({ preview = false }: { preview?: boolean } = {}): Promise<void> => {
    if (drawerDepth > 0 || typeof window === 'undefined' || printState.status === 'pending') return
    setPrintState(reconcile({ active: true, status: 'pending', preview }))
    const printQueryClients = new Map<string, QueryClient>()
    // Audit exports deliberately favour completeness over dashboard-like
    // latency: deep product and period lenses can be expensive but must not
    // silently disappear from the non-interactive report.
    const reportSignal = timeoutSignal(90_000)
    const detailSignal = () => anySignal([reportSignal, timeoutSignal(20_000)])
    try {
      const { buildPrintReport } = await import('./print')
      const report = await buildPrintReport(
        resolvedDoc,
        (request, owner = resolvedDoc) => {
          const endpoint = owner.endpoints.query
          if (!endpoint) {
            return Promise.reject(new Error(
              `lens: ${owner.meta.dashboardId} declares no query endpoint, so its detail cannot be printed`,
            ))
          }
          let client = printQueryClients.get(endpoint)
          if (!client) {
            client = new QueryClient(endpoint, { csrf: props.csrf, fetcher: props.fetcher })
            printQueryClients.set(endpoint, client)
          }
          return client.query(request, { signal: detailSignal() })
        },
        2_000,
        (src) => fetchDocument(src, {
          csrf: props.csrf,
          fetcher: props.fetcher,
          signal: detailSignal(),
        }),
        new URL(location.href),
        reportSignal,
      )
      for (const client of printQueryClients.values()) client.dispose()
      // The report body is a lazy chunk and the print-state update is what
      // mounts it, so the wait below is racing a network fetch it never
      // accounts for: two frames plus 120ms clear a warm cache and nothing
      // else, and when they do not, the print engine snapshots the placeholder
      // state and the sheet comes out blank. Fetch the chunk while there is
      // still nothing on screen — then the only thing left to wait for is the
      // renderer committing it.
      await import('../print/PrintReport')
      setPrintState(reconcile({ active: true, status: 'idle', report, preview }))
      // Preview stops here: the composed document stays on screen, on canvas,
      // for review — no print dialog, nothing to dismiss.
      if (preview) return
      // Let the report commit and let canvas adapters mount before the print
      // engine snapshots the page.
      await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())))
      await globalThis.document.fonts?.ready
      await new Promise<void>((resolve) => setTimeout(resolve, 120))
      globalThis.document.documentElement.classList.add('lens-print-active')
      const cleanup = () => {
        globalThis.document.documentElement.classList.remove('lens-print-active')
        setPrintState(reconcile({ active: false, status: 'idle', preview: false }))
      }
      window.addEventListener('afterprint', cleanup, { once: true })
      try {
        window.print()
      } catch (cause: unknown) {
        window.removeEventListener('afterprint', cleanup)
        cleanup()
        throw cause
      }
    } catch {
      for (const client of printQueryClients.values()) client.dispose()
      globalThis.document.documentElement.classList.remove('lens-print-active')
      setPrintState(reconcile({
        active: false,
        status: 'error',
        preview: false,
        message: translate('print.failed', 'PDF export failed'),
      }))
    }
  }

  const setPeriod = (filter: Filter, value: PeriodValue): void => {
    if (!filtersEnabled || typeof window === 'undefined' || !filter.period) return
    const merged = periodTransitionValues(resolvedDoc, filter, value, filterValuesCurrent)
    // An unbounded period has no finite span from which "previous period" or
    // "year ago" can be derived. Canonicalize every comparison bound to this
    // period in the same history entry so the visible off state, copied URL,
    // refetch and export scope can never disagree.
    const current = new URL(window.location.href)
    const next = writeFilterValues(current, resolvedDoc, merged)
    if (!sameNavigationURL(current, next)) {
      window.history.pushState(browserStateFor(navigationPlain, window.history.state), '', next)
    }
    syncFiltersFromURL()
  }

  const applyFilterURL = (target: string | URL, options?: { newTab?: boolean }): void => {
    if (!filtersEnabled || typeof window === 'undefined') return
    const next = new URL(target, window.location.href)
    if (next.origin !== window.location.origin) return
    if (options?.newTab) {
      window.open(next.href, '_blank', 'noopener')
      return
    }
    const current = new URL(window.location.href)
    // Cross-filtering belongs to the mounted document only while the target
    // stays on that document. Cube drills may intentionally point at a
    // separate report page; pushState would change the address without
    // mounting that page, leaving the old dashboard visible until reload.
    if (next.pathname !== current.pathname) {
      navigateTo(siteRelativeURL(next.href, current) ?? next.href)
      return
    }
    if (!sameNavigationURL(current, next)) {
      window.history.pushState(browserStateFor(navigationPlain, window.history.state), '', next)
    }
    syncFiltersFromURL()
  }

  const setCompare = (filter: Filter, value: CompareValue): void => {
    if (!filter.compare || typeof window === 'undefined') return
    const merged = { ...filterValuesCurrent }
    delete merged[filter.compare.startParam]
    delete merged[filter.compare.endParam]
    Object.assign(merged, compareValues(filter.compare, value))
    const next = writeFilterValues(new URL(window.location.href), resolvedDoc, merged)
    applyFilterURL(next)
  }

  // A segmented choice is a single parameter with a closed option set, so it
  // needs no staging step and no cross-filter canonicalization: it merges into
  // the current values and goes through the same applyFilterURL funnel every
  // other control ends in.
  const setSegmented = (filter: Filter, value: string): void => {
    if (!filter.segmented || typeof window === 'undefined') return
    if (!filter.segmented.options.some((option) => option.value === value)) return
    const merged = { ...filterValuesCurrent, ...segmentedValues(filter.segmented, value) }
    applyFilterURL(writeFilterValues(new URL(window.location.href), resolvedDoc, merged))
  }

  const [filtersStore, setFiltersStore] = createStore<FiltersContextValue>({
    filters: filtersEnabled ? declaredFilters(resolvedDoc) : [],
    values: filterValuesCurrent,
    setPeriod,
    setCompare,
    setSegmented,
    applyURL: applyFilterURL,
  })
  createEffect(() => {
    // Reconcile preserves document identity; every document revision must also
    // refresh comparison values nested inside its existing filter objects.
    void docVersion()
    const next = filtersEnabled ? declaredFilters(resolvedDoc) : []
    setFiltersStore('filters', reconcile(next))
  })
  createEffect(() => {
    setFiltersStore('values', reconcile(filterValues()))
  })

  const closeDrawer = (): void => {
    if (!navigationPlain.drawer || props.controlledNavigation) return
    if (drawerOpener && typeof window !== 'undefined') {
      let steps = 1
      for (let index = navigationPlain.history.length - 1; index >= 0; index -= 1) {
        if (!navigationPlain.history[index]?.drawer) break
        steps += 1
      }
      window.history.go(-steps)
      return
    }
    replaceNextURL = true
    dispatch(navigationActions.closeDrawer())
  }
  const warmDrawerSource = async (src: string, signal?: AbortSignal): Promise<void> => {
    if (!isSameOriginDrawerSource(src) || signal?.aborted) return
    await drawerCache().prefetch(src, signal)
  }
  const resolveDrawerKey = async (metricKey: string, signal?: AbortSignal): Promise<string> => {
    const doc = resolvedDoc
    const endpoint = doc.endpoints.drawer
    const normalizedKey = metricKey.trim()
    if (!endpoint || !normalizedKey || signal?.aborted) throw new DOMException('Aborted', 'AbortError')
    const cacheKey = `${doc.snapshotId}\u0000${endpoint}\u0000${normalizedKey}`
    const cached = drawerKeySources.get(cacheKey)
    if (cached) return cached
    const existing = drawerKeyResolvers.get(cacheKey)
    if (existing) return existing
    const pending = (async () => {
      const response = await (props.fetcher ?? fetch)(endpoint, {
        method: 'POST', credentials: 'same-origin', signal,
        headers: { 'Content-Type': 'application/json', ...(props.csrf ? { 'X-CSRF-Token': props.csrf } : {}) },
        body: JSON.stringify({ snapshotId: doc.snapshotId, metricKey: normalizedKey }),
      })
      if (!response.ok) throw new Error(`drawer resolver failed (${response.status})`)
      const payload = await response.json() as { url?: unknown }
      if (typeof payload.url !== 'string' || !isSameOriginDrawerSource(payload.url)) throw new Error('drawer resolver returned an invalid URL')
      if (signal?.aborted) throw new DOMException('Aborted', 'AbortError')
      const relative = siteRelativeURL(payload.url, new URL(window.location.href))
      if (!relative) throw new Error('drawer resolver returned a cross-origin URL')
      drawerKeySources.set(cacheKey, relative)
      return relative
    })().finally(() => {
      if (drawerKeyResolvers.get(cacheKey) === pending) drawerKeyResolvers.delete(cacheKey)
    })
    drawerKeyResolvers.set(cacheKey, pending)
    return pending
  }
  const warmDrawerKey = async (metricKey: string, signal?: AbortSignal): Promise<void> => {
    const relative = await resolveDrawerKey(metricKey, signal)
    if (!signal?.aborted) await drawerCache().prefetch(relative, signal)
  }
  const drawer: DrawerContextValue = {
    depth: drawerDepth,
    get canOpen() { return drawerDepth === 0 || Boolean(props.onDrawerNavigate) },
    open: (src, opener) => {
      if (!isSameOriginDrawerSource(src)) return
      if (drawerDepth > 0) {
        props.onDrawerNavigate?.(src)
        return
      }
      if (navigationPlain.drawer) return
      drawerOpener = opener ?? (
        globalThis.document.activeElement instanceof HTMLElement ? globalThis.document.activeElement : undefined
      )
      // The opener stays connected (the fullscreen panel is not collapsed), so
      // its `.lens-root` still resolves; fall back to any root when the drawer
      // opened without a captured element.
      const root = drawerOpener?.closest<HTMLElement>('.lens-root')
        ?? (typeof globalThis.document !== 'undefined'
          ? globalThis.document.querySelector<HTMLElement>('.lens-root')
          : null)
      const theme = root?.dataset.theme
      drawerTheme.theme = theme
      drawerTheme.dark = theme === 'dark' || root?.classList.contains('dark') === true
      dispatch(navigationActions.openDrawer(
        src,
        drawerNavigationFromSource(src, new URL(window.location.href)),
      ))
    },
    openKey: (metricKey, opener) => {
      const endpoint = resolvedDoc.endpoints.drawer
      if (!endpoint || !metricKey.trim()) return
      void resolveDrawerKey(metricKey).then((relative) => {
        const root = opener?.closest<HTMLElement>('.lens-root')
          ?? globalThis.document.querySelector<HTMLElement>('.lens-root')
        drawerOpener = opener
        const theme = root?.dataset.theme
        drawerTheme.theme = theme
        drawerTheme.dark = theme === 'dark' || root?.classList.contains('dark') === true
        const current = new URL(window.location.href)
        dispatch(navigationActions.openDrawer(relative, drawerNavigationFromSource(relative, current)))
      }).catch((cause: unknown) => setNotice(cause instanceof Error ? cause.message : 'drawer resolver failed'))
    },
    close: closeDrawer,
    prefetch: (src) => {
      if (drawerDepth > 0 || navigationPlain.drawer || !isSameOriginDrawerSource(src)) return () => undefined
      void warmDrawerSource(src)
      return () => undefined
    },
    prefetchKey: (metricKey) => {
      if (drawerDepth > 0 || navigationPlain.drawer || !resolvedDoc.endpoints.drawer || !metricKey.trim()) return () => undefined
      const controller = new AbortController()
      void warmDrawerKey(metricKey, controller.signal).catch((cause: unknown) => {
        // Speculative warm-up is silent; the click path remains the
        // authoritative retry and reports its own failure if one exists.
        if (!controller.signal.aborted) console.debug('[lens] drawer prefetch skipped', cause)
      })
      return () => controller.abort()
    },
    prefetchIdle: (src) => {
      if (drawerDepth > 0 || !isSameOriginDrawerSource(src)) return () => undefined
      return drawerIdleQueue()?.register(`src:${src}`, (signal) => warmDrawerSource(src, signal)) ?? (() => undefined)
    },
    prefetchIdleKey: (metricKey) => {
      if (drawerDepth > 0 || !resolvedDoc.endpoints.drawer || !metricKey.trim()) return () => undefined
      return drawerIdleQueue()?.register(`key:${metricKey}`, (signal) => warmDrawerKey(metricKey, signal)) ?? (() => undefined)
    },
  }

  const drill: DrillContextValue = {
    drillInto: (nodeKey, panelId) => dispatch(navigationActions.drillInto(nodeKey, panelId)),
    prefetch: prefetchDrill,
    back: () => dispatch(navigationActions.back()),
    jumpTo: (breadcrumbIndex) => dispatch(navigationActions.jumpTo(breadcrumbIndex)),
    switchPerspective: (id, options) => {
      if (options?.replace) replaceNextURL = true
      dispatch(navigationActions.switchPerspective(id, undefined, options?.replace, options?.enter, options?.panelId))
    },
    reset: () => dispatch(navigationActions.reset()),
    get canGoBack() { return dashboard.navigation.history.length > 0 },
  }

  const exportContext: ExportContextValue = {
    get available() { return Boolean(dashboard.document.endpoints.export) },
    state: (panelId) => exportStates[exportScope(panelId)] ?? { status: 'idle' as const },
    run: runExport,
  }

  const printContext: PrintContextValue = {
    get available() { return drawerDepth === 0 && typeof window !== 'undefined' },
    get active() { return printState.active },
    get status() { return printState.status },
    get message() { return printState.message },
    get report() { return printState.report },
    get preview() { return printState.preview },
    run: runPrint,
  }

  const locale = createMemo(() => props.locale)

  return (
    <LocaleContext.Provider value={locale}>
      <I18nContext.Provider value={createMemo(() => dashboard.document.i18n)}>
        <DashboardContext.Provider value={dashboard}>
          <FiltersContext.Provider value={filtersStore}>
            <DrawerContext.Provider value={drawer}>
              <DrillContext.Provider value={drill}>
                <PanelPaginationContext.Provider value={pagination}>
                  <ExportContext.Provider value={exportContext}>
                    <PrintContext.Provider value={printContext}>
                      <FramesContext.Provider value={frames}>
                        <Show when={dashboard.notice}>
                          {(message) => <RuntimeNotice notice={message()} onDismiss={() => setNotice(undefined)} />}
                        </Show>
                        {props.children()}
                        <Show when={drawerDepth === 0 && dashboard.navigation.drawer}>
                          {(drawer) => (
                            // DocumentProvider wraps the drawer so its sticky top-bar
                            // header can read the loaded document's own identity block
                            // (eyebrow/title/caption) and render it once — while still
                            // mounting the close button immediately, before the document
                            // lands, because DocumentProvider renders its children
                            // regardless of load state.
                            <DocumentProvider src={drawer().src} csrf={props.csrf} fetcher={props.fetcher} cache={drawerCache()}>
                              <LensDrawer
                                closeLabel={translate('drawer.close', 'Close details')}
                                dark={drawerTheme.dark}
                                eyebrow={translate('drawer.eyebrow', 'Detail view')}
                                label={translate('drawer.label', 'Drill details')}
                                onClose={closeDrawer}
                                restoreFocus={drawerOpener}
                                theme={drawerTheme.theme}
                              >
                                <DashboardRuntimeProvider
                                  controlledNavigation={nestedDrawerState(drawer(), dashboard.navigation.history)}
                                  csrf={props.csrf}
                                  drawerDepth={1}
                                  // A drawer-shaped loading placeholder (headline stat +
                                  // table), not the dashboard's stat-strip + chart pair,
                                  // so the drawer body does not jump when the drill
                                  // document lands.
                                  fallback={<DashboardSkeleton rows={drawerSkeletonRows} />}
                                  fetcher={props.fetcher}
                                  locale={props.locale}
                                  onControlledNavigationChange={(next) => dispatch(navigationActions.updateDrawer(next))}
                                  onDrawerNavigate={(src) => dispatch(navigationActions.replaceDrawer(
                                    src,
                                    drawerNavigationFromSource(src, new URL(window.location.href)),
                                  ))}
                                >
                                  {props.children}
                                </DashboardRuntimeProvider>
                              </LensDrawer>
                            </DocumentProvider>
                          )}
                        </Show>
                      </FramesContext.Provider>
                    </PrintContext.Provider>
                  </ExportContext.Provider>
                </PanelPaginationContext.Provider>
              </DrillContext.Provider>
            </DrawerContext.Provider>
          </FiltersContext.Provider>
        </DashboardContext.Provider>
      </I18nContext.Provider>
    </LocaleContext.Provider>
  )
}

export interface DashboardRuntimeProviderProps {
  locale: string
  csrf?: string
  fetcher?: typeof fetch
  /** Factory: instantiated once for the board and once for the drawer body. */
  children: () => JSX.Element
  /** Server-rendered placeholder shown until the first document arrives. */
  fallback?: JSX.Element
  controlledNavigation?: NavigationState
  onControlledNavigationChange?: (view: NavigationView) => void
  onDrawerNavigate?: (src: string) => void
  drawerDepth?: number
}

export function DashboardRuntimeProvider(props: DashboardRuntimeProviderProps): JSX.Element {
  const context = useContext(DocumentContext)
  if (!context) throw new Error('DashboardRuntimeProvider must be inside DocumentProvider')
  return (
    <Show
      when={context.document}
      fallback={
        <Show
          when={context.error}
          fallback={
            // A layout-shaped placeholder, not a spinner: the page keeps its rhythm
            // and nothing jumps when the document lands.
            <DocumentLoading locale={props.locale}>
              {props.fallback ?? <DashboardSkeleton rows={defaultSkeletonRows} />}
            </DocumentLoading>
          }
        >
          {(error) => (
            <DocumentLoadError
              locale={props.locale}
              message={error().message}
              onRetry={() => { void context.refresh().catch(() => undefined) }}
            />
          )}
        </Show>
      }
    >
      {(document) => (
        <RuntimeCore
          document={document()}
          locale={props.locale}
          csrf={props.csrf}
          fetcher={props.fetcher}
          refreshDocument={context.refresh}
          applyFilters={context.applyFilters}
          controlledNavigation={props.controlledNavigation}
          onControlledNavigationChange={props.onControlledNavigationChange}
          onDrawerNavigate={props.onDrawerNavigate}
          drawerDepth={props.drawerDepth}
        >
          {props.children}
        </RuntimeCore>
      )}
    </Show>
  )
}

export function useDashboard(): DashboardContextValue {
  const context = useContext(DashboardContext)
  if (!context) throw new Error('useDashboard must be used inside DashboardRuntimeProvider')
  return context
}

export function useDrill(): DrillContextValue {
  const context = useContext(DrillContext)
  if (!context) throw new Error('useDrill must be used inside DashboardRuntimeProvider')
  return context
}

export function useFilters(): FiltersContextValue {
  const context = useContext(FiltersContext)
  if (!context) throw new Error('useFilters must be used inside DashboardRuntimeProvider')
  return context
}

export function useDrawer(): DrawerContextValue {
  const context = useContext(DrawerContext)
  if (!context) throw new Error('useDrawer must be used inside DashboardRuntimeProvider')
  return context
}

export function usePanelFrame(panelId: string): PanelFrameState {
  const frames = useContext(FramesContext)
  const store = frames ?? emptyFrameStore
  const state = store.get(panelId) ?? emptyFrameState
  if (!frames) throw new Error('usePanelFrame must be used inside DashboardRuntimeProvider')
  return state
}

export function usePanelPagination(): PanelPaginationContextValue {
  const context = useContext(PanelPaginationContext)
  if (!context) throw new Error('usePanelPagination must be used inside DashboardRuntimeProvider')
  return context
}

export function useExport(panelId?: string): ExportState & { available: boolean; run: () => Promise<void> } {
  const context = useContext(ExportContext)
  if (!context) throw new Error('useExport must be used inside DashboardRuntimeProvider')
  return {
    get status() { return context.state(panelId).status },
    get message() { return context.state(panelId).message },
    get available() { return context.available },
    run: () => context.run(panelId),
  }
}

export function usePrint(): PrintContextValue {
  const context = useContext(PrintContext)
  if (!context) throw new Error('usePrint must be used inside DashboardRuntimeProvider')
  return context
}

function useLocale(): Accessor<string> {
  return useContext(LocaleContext) ?? (() => 'en')
}

function useI18nMessages(): Accessor<Record<string, string>> {
  return useContext(I18nContext) ?? (() => ({}))
}

export function useFormat(field?: FieldFormat): (value: unknown) => string {
  const locale = useLocale()
  return (value: unknown) => formatFieldValue(value, field, locale())
}

/**
 * useFormat for a group of values read together — a table column, a tooltip
 * group — that must share one magnitude. A compact field abbreviates each
 * value on its own and stops abbreviating below the compact floor, so a column
 * read downwards printed «111,13 млн», «45 000» and a bare «0» in the same
 * stack of digits. With a reference every cell is written at the magnitude
 * that reference implies. An undefined reference means "no shared magnitude",
 * and the values format exactly as useFormat writes them.
 */
export function useFormatAtReference(
  field: FieldFormat | undefined,
  reference: number | undefined | Accessor<number | undefined>,
): (value: unknown) => string {
  const locale = useLocale()
  const read = () => (typeof reference === 'function' ? reference() : reference)
  return (value: unknown) => {
    const current = read()
    return current === undefined
      ? formatFieldValue(value, field, locale())
      : formatFieldValueAtReference(value, current, field, locale())
  }
}

/**
 * The tooltip companion to useFormat for compact fields: returns the exact
 * grouped value («66 064 767 694 UZS») or undefined when nothing was
 * abbreviated away.
 */
export function useFormatExact(field?: FieldFormat): (value: unknown) => string | undefined {
  const locale = useLocale()
  return (value: unknown) => formatFieldValueExact(value, field, locale())
}

export function useAxisFormat(field?: FieldFormat): (value: unknown) => string {
  const locale = useLocale()
  return (value: unknown) => formatAxis(value, field, locale())
}

/**
 * The strings the runtime may have to show before any document has arrived —
 * and therefore before it has any translations to look them up in.
 *
 * These are the only two states that exist without a document: it failed, or it
 * is taking a long time. English on a Russian dashboard is a worse answer than
 * carrying four short strings here, and the host page cannot supply them
 * either — its own dictionary travels inside the document.
 */
const preDocumentStrings: Record<string, Record<string, string>> = {
  ru: {
    'runtime.loadError': 'Не удалось загрузить дашборд',
    'runtime.retry': 'Повторить',
    'runtime.slowLoad': 'Расчёт продолжается — широкий период считается дольше.',
  },
  uz: {
    'runtime.loadError': 'Boshqaruv panelini yuklab bo‘lmadi',
    'runtime.retry': 'Qayta urinish',
    'runtime.slowLoad': 'Hisoblash davom etmoqda — keng davr uzoqroq hisoblanadi.',
  },
  'uz-Cyrl': {
    'runtime.loadError': 'Бошқарув панелини юклаб бўлмади',
    'runtime.retry': 'Қайта уриниш',
    'runtime.slowLoad': 'Ҳисоблаш давом этмоқда — кенг давр узоқроқ ҳисобланади.',
  },
}

function preDocumentText(locale: string | undefined, key: string, fallback: string): string {
  if (!locale) return fallback
  const exact = preDocumentStrings[locale]?.[key]
  if (exact) return exact
  const primary = locale.split('-')[0]
  return (primary ? preDocumentStrings[primary]?.[key] : undefined) ?? fallback
}

function DocumentLoadError(props: { locale?: string; message: string; onRetry: () => void }): JSX.Element {
  const translate = useTranslate()
  return (
    <div class="lens-placeholder-state" role="alert">
      <span>
        {translate('runtime.loadError', preDocumentText(props.locale, 'runtime.loadError', 'Unable to load Lens document'))}
        : {props.message}
      </span>
      {/* A failed document leaves the page with nothing on it. Panels already
          offer their own retry; without one here the only way back is a manual
          reload, which most readers of a dashboard will not think to try. */}
      <button class="lens-placeholder-retry" onClick={() => props.onRetry()} type="button">
        {translate('runtime.retry', preDocumentText(props.locale, 'runtime.retry', 'Retry'))}
      </button>
    </div>
  )
}

/**
 * How long a blank skeleton is allowed to stand before it says something. A
 * dashboard over a wide date range genuinely takes tens of seconds to compute;
 * silence past this point is indistinguishable from a hang.
 */
const slowLoadNoticeMs = 8000

function DocumentLoading(props: { locale?: string; children: JSX.Element }): JSX.Element {
  const translate = useTranslate()
  const [slow, setSlow] = createSignal(false)
  onMount(() => {
    const timer = setTimeout(() => setSlow(true), slowLoadNoticeMs)
    onCleanup(() => clearTimeout(timer))
  })
  return (
    <div aria-busy="true" class="lens-loading">
      <Show when={slow()}>
        <p class="lens-loading-notice" role="status">
          {translate(
            'runtime.slowLoad',
            preDocumentText(props.locale, 'runtime.slowLoad', 'Still computing — a wide period takes longer.'),
          )}
        </p>
      </Show>
      {props.children}
    </div>
  )
}

function RuntimeNotice(props: { notice: string; onDismiss: () => void }): JSX.Element {
  const translate = useTranslate()
  return (
    <div class="lens-runtime-notice" role="status">
      <span>{props.notice}</span>
      <button
        aria-label={translate('runtime.dismissNotice', 'Dismiss notice')}
        onClick={() => props.onDismiss()}
        type="button"
      >
        <X />
      </button>
    </div>
  )
}

export function useTranslate(): (key: string, fallback: string, vars?: TranslationVars) => string {
  const messages = useI18nMessages()
  return (key: string, fallback: string, vars?: TranslationVars) => translation(messages(), key, fallback, vars)
}

export function useDocumentState(): DocumentContextValue {
  const context = useContext(DocumentContext)
  if (!context) throw new Error('useDocumentState must be used inside DocumentProvider')
  return context
}

/**
 * The drawer identity block carried by the currently loaded document, read
 * without throwing so the drawer chrome can render its own top-bar header while
 * the document is still loading (context present, document undefined) or in
 * isolated stories (no DocumentProvider at all). Returns undefined when the
 * document carries no drawer header.
 */
export function useDrawerHeader(): Accessor<DashboardDocument['drawer']> {
  const context = useContext(DocumentContext)
  return createMemo(() => context?.document?.drawer)
}

/**
 * A background document refetch (a date/period change, a focus refresh) is in
 * flight. Read without throwing so a panel can surface the loading state even
 * when it is mounted outside a DocumentProvider (isolated stories, previews);
 * there it simply reports "not refreshing".
 */
export function useDocumentRefreshing(): boolean {
  return useContext(DocumentContext)?.isRefreshing ?? false
}
