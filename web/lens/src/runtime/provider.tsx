import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useReducer,
  useRef,
  useState,
  useSyncExternalStore,
} from 'react'
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
import { SnapshotGoneError } from './query'
import { queryWithSnapshotRecovery } from './recovery'
import { drawerNavigationFromSource, navigationFromURL, navigationToURL, sameNavigationURL, siteRelativeURL } from './url'
import { X } from '../icons'

/* eslint-disable react-refresh/only-export-components */

interface DocumentContextValue {
  document?: DashboardDocument
  isLoading: boolean
  /** A background refetch (focus-triggered) is in flight; current data stays. */
  isRefreshing: boolean
  error: Error | null
  refresh: () => Promise<DashboardDocument>
  dismissError: () => void
  /**
   * Refetches the document with these filter parameters on the src. The
   * current document stays on screen until the new one lands; every later
   * refresh — including snapshot recovery — keeps the applied parameters.
   */
  applyFilters: (values: FilterValues) => void
}

const DocumentContext = createContext<DocumentContextValue | undefined>(undefined)

/** A document older than this is refetched when the window regains focus. */
const staleDocumentAgeMs = 5 * 60 * 1000

export interface DocumentProviderProps {
  src?: string
  initialDocument?: DashboardDocument
  csrf?: string
  fetcher?: typeof fetch
  /** Warmed drill documents; a hit seeds the initial document and skips fetch. */
  cache?: Pick<DocumentCache, 'configure' | 'get' | 'load'>
  children: ReactNode
}

export function DocumentProvider({ src, initialDocument, csrf, fetcher, cache, children }: DocumentProviderProps) {
  const [document, setDocument] = useState<DashboardDocument | undefined>(
    () => src ? (cache?.get(src) ?? initialDocument) : initialDocument,
  )
  const [isLoading, setIsLoading] = useState(Boolean(src) && !(src && (cache?.get(src) ?? initialDocument)))
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [error, setError] = useState<Error | null>(null)
  const controllers = useRef(new Set<AbortController>())
  const inFlight = useRef<Promise<DashboardDocument>>()
  const loadedAt = useRef<number>(src && cache?.get(src) ? Date.now() : 0)
  const csrfRef = useRef(csrf)
  const fetcherRef = useRef(fetcher)
  /** Every filter parameter the runtime has driven so far. */
  const drivenFilterParams = useRef(new Set<string>())
  const appliedFilters = useRef<FilterValues>()
  const filterController = useRef<AbortController>()

  useEffect(() => {
    csrfRef.current = csrf
    fetcherRef.current = fetcher
    cache?.configure({ csrf, fetcher })
  }, [cache, csrf, fetcher])

  const effectiveSrc = useCallback(() => {
    if (!src || !appliedFilters.current) return src
    return srcWithFilterParams(src, drivenFilterParams.current, appliedFilters.current)
  }, [src])

  const refresh = useCallback(() => {
    if (!src) {
      if (!initialDocument) return Promise.reject(new Error('Lens document source is required'))
      setDocument(initialDocument)
      setError(null)
      return Promise.resolve(initialDocument)
    }
    if (inFlight.current) return inFlight.current
    const controller = new AbortController()
    controllers.current.add(controller)
    setIsLoading(true)
    const target = effectiveSrc() ?? src
    // The initial drawer load must join a hover/idle prefetch already in
    // flight. Explicit refreshes bypass the cache once a document has loaded,
    // preserving refresh/recovery semantics.
    const request = cache && loadedAt.current === 0 && target === src
      ? cache.load(target)
      : fetchDocument(target, { csrf: csrfRef.current, fetcher: fetcherRef.current, signal: controller.signal })
    const pending = request
      .then((next) => {
        if (controller.signal.aborted) return next
        loadedAt.current = Date.now()
        setDocument(next)
        setError(null)
        return next
      })
      .catch((cause: unknown) => {
        const nextError = cause instanceof Error ? cause : new Error('document request failed')
        if (!controller.signal.aborted) setError(nextError)
        throw nextError
      })
      .finally(() => {
        controllers.current.delete(controller)
        inFlight.current = undefined
        if (!controller.signal.aborted) setIsLoading(false)
      })
    inFlight.current = pending
    return pending
  }, [cache, effectiveSrc, initialDocument, src])

  // A focus-triggered refetch keeps the current document on screen and swaps
  // only on success; a failure is logged and otherwise silent, so a transient
  // network blip never replaces good data with an error state.
  const refreshInBackground = useCallback(() => {
    if (!src || inFlight.current) return
    const controller = new AbortController()
    controllers.current.add(controller)
    setIsRefreshing(true)
    void fetchDocument(effectiveSrc() ?? src, { csrf: csrfRef.current, fetcher: fetcherRef.current, signal: controller.signal })
      .then((next) => {
        loadedAt.current = Date.now()
        setDocument(next)
        setError(null)
      })
      .catch((cause: unknown) => {
        if (!controller.signal.aborted) console.error('[lens] background document refresh failed', cause)
      })
      .finally(() => {
        controllers.current.delete(controller)
        if (!controller.signal.aborted) setIsRefreshing(false)
      })
  }, [effectiveSrc, src])

  const applyFilters = useCallback((values: FilterValues) => {
    if (!src) return
    for (const name of Object.keys(values)) drivenFilterParams.current.add(name)
    if (appliedFilters.current && sameFilterValues(appliedFilters.current, values)) return
    appliedFilters.current = values
    // The latest selection wins: a still-flying older filter fetch is stale.
    filterController.current?.abort()
    const controller = new AbortController()
    filterController.current = controller
    controllers.current.add(controller)
    setIsRefreshing(true)
    void fetchDocument(effectiveSrc() ?? src, { csrf: csrfRef.current, fetcher: fetcherRef.current, signal: controller.signal })
      .then((next) => {
        loadedAt.current = Date.now()
        setDocument(next)
        setError(null)
      })
      .catch((cause: unknown) => {
        if (controller.signal.aborted) return
        setError(cause instanceof Error ? cause : new Error('document request failed'))
      })
      .finally(() => {
        controllers.current.delete(controller)
        if (filterController.current === controller) filterController.current = undefined
        if (!controller.signal.aborted) setIsRefreshing(false)
      })
  }, [effectiveSrc, src])

  useEffect(() => {
    const cached = src ? cache?.get(src) : undefined
    const seeded = cached ?? initialDocument
    setDocument(src ? seeded : initialDocument)
    setError(null)
    if (seeded) {
      loadedAt.current = Date.now()
      setIsLoading(false)
    } else if (src) {
      void refresh().catch(() => undefined)
    }
  }, [cache, initialDocument, refresh, src])

  useEffect(() => {
    if (!src || typeof window === 'undefined') return
    const onFocus = () => {
      if (loadedAt.current > 0 && Date.now() - loadedAt.current >= staleDocumentAgeMs) refreshInBackground()
    }
    window.addEventListener('focus', onFocus)
    return () => window.removeEventListener('focus', onFocus)
  }, [refreshInBackground, src])

  useEffect(() => () => {
    for (const controller of controllers.current) controller.abort()
    controllers.current.clear()
  }, [])

  const dismissError = useCallback(() => setError(null), [])

  const value = useMemo(
    () => ({ document, isLoading, isRefreshing, error, refresh, dismissError, applyFilters }),
    [applyFilters, dismissError, document, error, isLoading, isRefreshing, refresh],
  )
  return <DocumentContext.Provider value={value}>{children}</DocumentContext.Provider>
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
  canGoBack: boolean
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
  canOpen?: boolean
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
  available: boolean
  state: (panelId?: string) => ExportState
  run: (panelId?: string) => Promise<void>
}

export type PrintStatus = 'idle' | 'pending' | 'error'

export interface PrintContextValue {
  available: boolean
  active: boolean
  status: PrintStatus
  message?: string
  report?: PrintReport
  /**
   * Builds the report and hands it to the browser's print dialog. In preview
   * mode it stops one step earlier and leaves the composed document on screen —
   * the only way to review printed output, since a print dialog blocks both the
   * page and any automation driving it.
   */
  run: (options?: { preview?: boolean }) => Promise<void>
  preview: boolean
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

class PanelFrameStore {
  private readonly states = new Map<string, PanelFrameState>()
  private readonly subscribers = new Map<string, Set<() => void>>()

  get(panelId: string): PanelFrameState | undefined {
    return this.states.get(panelId)
  }

  set(panelId: string, state: PanelFrameState): void {
    this.states.set(panelId, state)
    for (const subscriber of this.subscribers.get(panelId) ?? []) subscriber()
  }

  subscribe(panelId: string, subscriber: () => void): () => void {
    const subscribers = this.subscribers.get(panelId) ?? new Set()
    subscribers.add(subscriber)
    this.subscribers.set(panelId, subscribers)
    return () => {
      subscribers.delete(subscriber)
      if (subscribers.size === 0) this.subscribers.delete(panelId)
    }
  }
}

const DashboardContext = createContext<DashboardContextValue | undefined>(undefined)
const DrillContext = createContext<DrillContextValue | undefined>(undefined)
const FramesContext = createContext<PanelFrameStore | undefined>(undefined)
const PanelPaginationContext = createContext<PanelPaginationContextValue | undefined>(undefined)
const ExportContext = createContext<ExportContextValue | undefined>(undefined)
const PrintContext = createContext<PrintContextValue | undefined>(undefined)
const DrawerContext = createContext<DrawerContextValue | undefined>(undefined)
const FiltersContext = createContext<FiltersContextValue | undefined>(undefined)
const LocaleContext = createContext('en')
const I18nContext = createContext<Record<string, string>>({})
const emptyFrameStore = new PanelFrameStore()

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

function browserStateFor(navigation: NavigationState, current: unknown): Record<string, unknown> {
  const state = current && typeof current === 'object' ? current as Record<string, unknown> : {}
  const clone = (view: NavigationView): NavigationView => ({
    panelId: view.panelId,
    path: [...view.path],
    perspectiveId: view.perspectiveId,
    ...(view.drawer ? { drawer: { ...view.drawer, path: [...view.drawer.path] } } : {}),
  })
  const lens: BrowserNavigationState = { view: clone(navigation), history: navigation.history.map(clone) }
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

interface IdleDrillPrefetchRoot {
  panelId: string
  path: NodePath
  perspective: string
}

export function idleDrillPrefetchRoots(document: DashboardDocument): Array<IdleDrillPrefetchRoot> {
  const result: Array<IdleDrillPrefetchRoot> = []
  const seen = new Set<string>()
  for (const row of document.layout.rows) {
    for (const item of row.panels) {
      const panel = document.panels.find((candidate) => candidate.id === item.panelId)
      const host = panel?.drillRoot ? document.drill.edges[panel.drillRoot] : undefined
      if (!panel || !host) continue
      for (const perspective of document.perspectives) {
        if (perspective.semantics === 'evidence' || seen.has(perspective.id)) continue
        const root = document.drill.edges[perspective.root]
        if (!root || !host.path.every((key, index) => root.path[index] === key)) continue
        seen.add(perspective.id)
        result.push({ panelId: panel.id, path: [...root.path], perspective: perspective.id })
        if (result.length === 8) return result
      }
    }
  }
  return result
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

interface RuntimeCoreProps {
  document: DashboardDocument
  locale: string
  csrf?: string
  fetcher?: typeof fetch
  refreshDocument: () => Promise<DashboardDocument>
  applyFilters?: (values: FilterValues) => void
  children: ReactNode
  controlledNavigation?: NavigationState
  onControlledNavigationChange?: (view: NavigationView) => void
  onDrawerNavigate?: (src: string) => void
  drawerDepth?: number
}

function RuntimeCore({
  document: sourceDocument, locale, csrf, fetcher, refreshDocument, applyFilters, children,
  controlledNavigation, onControlledNavigationChange, onDrawerNavigate, drawerDepth = 0,
}: RuntimeCoreProps) {
  const [runtimeDocument, setRuntimeDocumentState] = useState(() => ({
    source: sourceDocument,
    resolved: withInlineFrameChildren(sourceDocument),
  }))
  const document = runtimeDocument.source === sourceDocument
    ? runtimeDocument.resolved
    : withInlineFrameChildren(sourceDocument)
  const setRuntimeDocument = useCallback((update: (current: DashboardDocument) => DashboardDocument) => {
    setRuntimeDocumentState((current) => {
      const resolved = current.source === sourceDocument
        ? current.resolved
        : withInlineFrameChildren(sourceDocument)
      const next = update(resolved)
      // Returning the current state object verbatim lets React bail out of the
      // re-render when the update was an identity no-op (e.g. frame children
      // that are already merged).
      if (current.source === sourceDocument && next === resolved) return current
      return { source: sourceDocument, resolved: next }
    })
  }, [sourceDocument])
  const [localNavigation, localDispatch] = useReducer(
    (state: NavigationState, action: Parameters<typeof navigationReducer>[1]) => runtimeNavigationReducer(document, state, action),
    document,
    inferredInitialNavigation,
  )
  // A drawer source can deep-link with path/perspective before its document is
  // loaded, so the outer runtime cannot know the nested host panel yet. Once
  // the document arrives, resolve the panel from that path locally; otherwise
  // ExplorePanel would keep rendering its resting root while the query runs
  // against the correct deeper level.
  const resolvedControlledNavigation = useMemo(() => {
    if (!controlledNavigation) return undefined
    const resolved = resolveView(document, controlledNavigation)
    return resolved
      ? { ...resolved, history: controlledNavigation.history }
      : controlledNavigation
  }, [controlledNavigation, document])
  const navigation = resolvedControlledNavigation ?? localNavigation
  const navigationRef = useRef(navigation)
  navigationRef.current = navigation
  const runtimeViewRef = useRef<NavigationView>()
  if (!runtimeViewRef.current ||
    runtimeViewRef.current.panelId !== navigation.panelId ||
    runtimeViewRef.current.perspectiveId !== navigation.perspectiveId ||
    runtimeViewRef.current.path.length !== navigation.path.length ||
    runtimeViewRef.current.path.some((key, index) => key !== navigation.path[index])) {
    runtimeViewRef.current = {
      panelId: navigation.panelId,
      path: [...navigation.path],
      perspectiveId: navigation.perspectiveId,
    }
  }
  const runtimeView = runtimeViewRef.current
  const dispatch = useCallback((action: Parameters<typeof navigationReducer>[1]) => {
    if (!controlledNavigation) {
      localDispatch(action)
      return
    }
    const current = navigationRef.current
    const next = runtimeNavigationReducer(document, current, action)
    if (next !== current) onControlledNavigationChange?.(next)
  }, [controlledNavigation, document, onControlledNavigationChange])
  const [notice, setNotice] = useState<string>()
  const documentRef = useRef(document)
  documentRef.current = document
  useEffect(() => {
    const endpoint = document.endpoints.release
    if (!endpoint) return
    const key = `${endpoint}\n${document.snapshotId}`
    let active = true
    let dispose: (() => void) | undefined
    void import('./snapshotLease').then(({ acquireSnapshotLease }) => {
      const acquired = acquireSnapshotLease(key, () => {
        void (fetcher ?? fetch)(endpoint, {
          method: 'POST', credentials: 'same-origin', keepalive: true,
          headers: { 'Content-Type': 'application/json', ...(csrf ? { 'X-CSRF-Token': csrf } : {}) },
          body: JSON.stringify({ snapshotId: document.snapshotId }),
        }).catch(() => undefined)
      })
      if (!active) {
        acquired()
        return
      }
      dispose = acquired
    })
    return () => {
      active = false
      dispose?.()
    }
  }, [csrf, document.endpoints.release, document.snapshotId, fetcher])
  const filtersEnabled = !controlledNavigation && drawerDepth === 0
  // Filter state derives from the URL alone: user actions write the URL, the
  // URL drives the refetch, and browser Back needs no resync timers because
  // popstate re-reads the same source of truth.
  const [filterValues, setFilterValuesState] = useState<FilterValues>(() => (
    typeof window === 'undefined' ? {} : readFilterValues(document, new URL(window.location.href))
  ))
  const filterValuesRef = useRef(filterValues)

  // The first document fetch follows the host's raw deep link. Once that
  // server-normalized document declares its filters, replace impossible URL
  // combinations (notably all-time + comparison) with the same canonical
  // values the controls and subsequent requests use.
  useEffect(() => {
    if (!filtersEnabled || typeof window === 'undefined') return
    const current = new URL(window.location.href)
    const canonical = writeFilterValues(current, documentRef.current, filterValuesRef.current)
    if (!sameNavigationURL(current, canonical)) {
      window.history.replaceState(window.history.state, '', canonical)
    }
  }, [document, filtersEnabled])

  const [exportStates, setExportStates] = useState<Record<string, ExportState>>({})
  const [printState, setPrintState] = useState<Omit<PrintContextValue, 'available' | 'run'>>({
    active: false,
    status: 'idle',
    preview: false,
  })
  const exportSnapshotId = useRef(document.snapshotId)
  const [retryToken, setRetryToken] = useState(0)
  const forceRetry = useRef(false)
  const pageLoader = useRef<(panelId: string, page: number, force?: boolean) => Promise<void>>()
  const searchLoader = useRef<(panelId: string, search: string) => Promise<void>>()
  const sortLoader = useRef<(panelId: string, sort: TableSort) => Promise<void>>()
  const tableRequestState = useRef(new Map<string, { search?: string; sort?: TableSort }>())
  const replaceNextURL = useRef(true)
  const drawerOpener = useRef<HTMLElement>()
  // The drawer portals to body and stacks above an expanded panel, so it carries
  // the theme of the root it opened from — captured from the opener when the
  // drawer opens, mirroring PanelFrame's overlay theme capture.
  const drawerTheme = useRef<{ theme?: string; dark: boolean }>({ dark: false })
  const drawerCache = useMemo<Pick<DocumentCache, 'configure' | 'get' | 'load' | 'prefetch'>>(() => {
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
      prefetch: async (src) => (await ready).prefetch(src),
    }
  }, [])
  const [drawerIdleQueue, setDrawerIdleQueue] = useState<IdlePrefetchQueue>()
  useEffect(() => {
    drawerCache.configure({ csrf, fetcher })
  }, [csrf, drawerCache, fetcher])
  useEffect(() => {
    // Idle speculation is deliberately a dynamic feature chunk: it is not on
    // the first-paint path, and panels re-register when the queue becomes
    // available. Snapshot changes dispose the old queue before importing the
    // next one, so no candidate can cross scopes.
    if (!document.snapshotId || drawerDepth !== 0) {
      setDrawerIdleQueue(undefined)
      return
    }
    let active = true
    let queue: IdlePrefetchQueue | undefined
    void import('./idlePrefetch').then(({ IdlePrefetchQueue: Queue }) => {
      if (!active) return
      queue = new Queue({ capacity: 8 })
      setDrawerIdleQueue(queue)
    })
    return () => {
      active = false
      queue?.reset()
      setDrawerIdleQueue(undefined)
    }
  }, [document.snapshotId, drawerDepth])
  useEffect(() => {
    drawerIdleQueue?.setPaused(Boolean(navigation.drawer))
  }, [drawerIdleQueue, navigation.drawer])
  const frameStore = useRef<PanelFrameStore>()
  const [panelAttempts, setPanelAttempts] = useState<Record<string, number>>({})
  const panelForces = useRef(new Set<string>())
  const launchedPanelAttempts = useRef(new Map<string, number>())
  const recomputePending = useRef(new Set<string>())
  const [isRecomputing, setIsRecomputing] = useState(false)
  const schedulePanel = useCallback((panelId: string, recompute = false) => {
    if (recompute) panelForces.current.add(panelId)
    setPanelAttempts((current) => ({ ...current, [panelId]: (current[panelId] ?? 0) + 1 }))
  }, [])
  const retryFrame = useCallback(() => {
    forceRetry.current = true
    setRetryToken((value) => value + 1)
  }, [])
  if (!frameStore.current) frameStore.current = new PanelFrameStore()
  const frames = frameStore.current
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
  const markPanelFramesPending = useCallback(() => {
    for (const panel of sourceDocument.panels) {
      const current = frames.get(panel.id)
      if (!current) continue
      frames.set(panel.id, {
        ...current,
        isStale: true,
        isLoading: !current.data,
        error: null,
      })
    }
  }, [frames, sourceDocument.panels])
  const syncFiltersFromURL = useCallback(() => {
    if (typeof window === 'undefined') return
    const values = readFilterValues(documentRef.current, new URL(window.location.href))
    if (sameFilterValues(values, filterValuesRef.current)) return
    filterValuesRef.current = values
    setFilterValuesState(values)
    // The single funnel every filter action passes through — a period chip, a
    // comparison mode, a facet apply, browser Back — so the staleness of the
    // board is decided in one place and cannot depend on which control moved.
    // Only a host that can actually refetch may dim: marking panels stale with
    // nothing in flight to clear them would leave the board dim for good.
    if (applyFilters) {
      markPanelFramesPending()
      applyFilters(values)
    }
  }, [applyFilters, markPanelFramesPending])
  const translate = useCallback(
    (key: string, fallback: string) => translation(document.i18n, key, fallback),
    [document.i18n],
  )
  const driftNotice = useCallback(() => translate(
    'drill.reset',
    'The previous drill path is no longer available. Lens returned to the root view.',
  ), [translate])
  for (const panel of document.panels) {
    if (!frames.get(panel.id)) {
      frames.set(panel.id, {
        data: document.frames[panel.frame],
        isStale: false,
        isLoading: Boolean(panel.deferred),
        error: null,
        retry: panel.deferred ? () => schedulePanel(panel.id) : retryFrame,
      })
    }
  }
  const endpoint = document.endpoints.query
  const queryClient = useMemo(() => endpoint ? new QueryClient(endpoint, { csrf, fetcher }) : undefined, [csrf, endpoint, fetcher])
  const panelEndpoint = sourceDocument.endpoints.panel
  const panelClient = useMemo(
    () => panelEndpoint ? new PanelClient(panelEndpoint, { csrf, fetcher }) : undefined,
    [csrf, fetcher, panelEndpoint],
  )
  const prefetchDrill = useCallback((nodeKeys: string | Array<string>, panelId?: string) => {
    if (!queryClient) return () => undefined
    const source = documentRef.current
    let next = navigationRef.current
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
    void queryClient.query(request, { signal: controller.signal }).then((response) => {
      const frame = Object.values(response.frames)[0]
      if (frame?.children) {
        setRuntimeDocument((current) => withFrameChildren(current, queryView.path, frame))
      }
    }).catch(() => undefined)
    return () => controller.abort(new DOMException('drill intent ended', 'AbortError'))
  }, [queryClient, setRuntimeDocument])

  useEffect(() => {
    if (!queryClient || drawerDepth !== 0 || !firstContentRowReady(document)) return
    const controller = new AbortController()
    const run = async () => {
      for (const candidate of idleDrillPrefetchRoots(document)) {
        if (controller.signal.aborted) return
        const rootRequest: QueryRequest = {
          snapshotId: document.snapshotId,
          path: candidate.path,
          perspective: candidate.perspective,
          prefetch: true,
          idlePrefetch: true,
        }
        try {
          const response = await queryClient.query(rootRequest, { signal: controller.signal })
          const frame = Object.values(response.frames)[0]
          if (!frame) continue
          if (frame.children) {
            setRuntimeDocument((current) => withFrameChildren(current, candidate.path, frame))
          }
          const children = frame.children?.filter(({ target }) => Boolean(target)) ?? []
          if (children.length === 0 || children.length > 4) continue
          for (const child of children) {
            if (!child.target || controller.signal.aborted) return
            await queryClient.query({
              snapshotId: document.snapshotId,
              path: [...candidate.path, child.key, child.target],
              perspective: candidate.perspective,
              prefetch: true,
              idlePrefetch: true,
            }, { signal: controller.signal })
          }
        } catch {
          if (controller.signal.aborted) return
        }
      }
    }
    void run()
    return () => controller.abort(new DOMException('idle drill prefetch scope changed', 'AbortError'))
  }, [document, drawerDepth, queryClient, setRuntimeDocument])

  useEffect(() => () => queryClient?.dispose(), [queryClient])
  useEffect(() => () => panelClient?.dispose(), [panelClient])

  useEffect(() => {
    if (exportSnapshotId.current === document.snapshotId) return
    exportSnapshotId.current = document.snapshotId
    setExportStates({})
  }, [document.snapshotId])

  useEffect(() => {
  // A document can legitimately return to an earlier snapshot through
  // browser Back. Panel attempts are scoped to the active document lifecycle,
  // not to a snapshot forever: retaining the old `${snapshot}:${panel}` keys
  // makes every deferred panel in the restored document stay on its skeleton
  // because the runtime mistakes a previous hydration for the current one.
    launchedPanelAttempts.current.clear()
    for (const panel of sourceDocument.panels) {
      frames.set(panel.id, {
        data: sourceDocument.frames[panel.frame],
        isStale: false,
        isLoading: Boolean(panel.deferred),
        error: null,
        retry: panel.deferred ? () => schedulePanel(panel.id) : retryFrame,
      })
    }
  }, [frames, retryFrame, schedulePanel, sourceDocument])

  useEffect(() => {
    if (!panelClient) return
    const pending: Array<{ panel: Panel; attempt: number; force: boolean; key: string; previous?: Frame; retry: () => void }> = []
    for (const panel of sourceDocument.panels) {
      const attempt = panelAttempts[panel.id] ?? 0
      const force = panelForces.current.has(panel.id)
      if (!panel.deferred && !force) continue
      const key = `${sourceDocument.snapshotId}:${panel.id}`
      if (launchedPanelAttempts.current.get(key) === attempt) continue
      launchedPanelAttempts.current.set(key, attempt)
      panelForces.current.delete(panel.id)
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
      if (launchedPanelAttempts.current.get(key) !== attempt) return
      if (response.error || !response.frames) {
        frames.set(panel.id, {
          data: previous, isStale: Boolean(previous), isLoading: false,
          error: new Error(response.error?.message ?? `panel ${panel.id} response has no frames`),
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
      setRuntimeDocument((current) => {
        if (current.snapshotId !== sourceDocument.snapshotId || current.frames[panel.frame] === loaded) return current
        return { ...current, frames: { ...current.frames, [panel.frame]: loaded } }
      })
    }
    void panelClient.loadBatch(pending.map(({ panel, force }) => ({
      snapshotId: sourceDocument.snapshotId, panelId: panel.id, ...(force ? { recompute: true } : {}),
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
        if (launchedPanelAttempts.current.get(key) !== attempt) continue
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
        if (!recomputePending.current.delete(panel.id)) continue
        if (recomputePending.current.size === 0) setIsRecomputing(false)
      }
    })
  }, [frames, panelAttempts, panelClient, refreshDocument, schedulePanel, setRuntimeDocument, sourceDocument])

  const recompute = useCallback(() => {
    if (!panelClient || sourceDocument.panels.length === 0) return
    const ids = sourceDocument.panels.map(({ id }) => id)
    recomputePending.current = new Set(ids)
    for (const id of ids) panelForces.current.add(id)
    setIsRecomputing(true)
    setPanelAttempts((current) => {
      const next = { ...current }
      for (const id of ids) next[id] = (next[id] ?? 0) + 1
      return next
    })
  }, [panelClient, sourceDocument.panels])

  useEffect(() => {
    if (pathResolves(document, runtimeView.path, runtimeView.perspectiveId) ||
      dynamicParentPath(document, runtimeView.path)) return
    replaceNextURL.current = true
    dispatch(navigationActions.restore(rootNavigation(document, runtimeView.panelId)))
    setNotice(driftNotice())
  }, [dispatch, document, driftNotice, runtimeView])

  useEffect(() => {
    if (controlledNavigation) return
    if (typeof window === 'undefined') return
    if (!pathResolves(document, navigation.path, navigation.perspectiveId)) return
    const current = new URL(window.location.href)
    const next = navigationToURL(navigation, current)
    const state = browserStateFor(navigation, window.history.state)
    if (replaceNextURL.current || sameNavigationURL(current, next)) window.history.replaceState(state, '', next)
    else window.history.pushState(state, '', next)
    replaceNextURL.current = false
  }, [controlledNavigation, document, navigation])

  useEffect(() => {
    if (controlledNavigation) return
    if (typeof window === 'undefined') return
    const onPopState = (event: PopStateEvent) => {
      const url = new URL(window.location.href)
      const view = navigationFromURL(url)
      const restored = navigationFromBrowserState(document, view, event.state)
      dispatch(navigationActions.restore(restored, restored.history))
      // Marking the panels stale is `syncFiltersFromURL`'s job now: Back is one
      // more way of changing the filter values, not a case of its own.
      syncFiltersFromURL()
    }
    window.addEventListener('popstate', onPopState)
    return () => window.removeEventListener('popstate', onPopState)
  }, [controlledNavigation, dispatch, document, syncFiltersFromURL])

  useEffect(() => {
    const pendingPath = dynamicParentPath(document, runtimeView.path)
    const queryView = pendingPath ? { ...runtimeView, path: pendingPath } : runtimeView
    const panel = panelForNavigation(document, queryView)
    // Leaving a drill level (Back, a breadcrumb jump, a reset) must not leave
    // the level's data on screen: any explore host that is no longer the
    // active drill target falls back to the frame the document shipped.
    for (const candidate of document.panels) {
      if (!candidate.drillRoot || candidate.id === panel?.id) continue
      const documentFrame = document.frames[candidate.frame]
      if (!documentFrame || frames.get(candidate.id)?.data === documentFrame) continue
      frames.set(candidate.id, {
        data: documentFrame, isStale: false, isLoading: false, error: null, retry: retryFrame,
      })
    }
    if (!panel) return
    const resolved = frameForPanel(document, queryView, panel, new Map())
    if (!resolved.shouldQuery || !queryClient) {
      if (resolved.frame) {
        frames.set(panel.id, {
          data: resolved.frame, isStale: false, isLoading: false, error: null,
          retry: retryFrame,
        })
      }
      return
    }

    let active = true
    const previous = frames.get(panel.id)?.data ?? document.frames[panel.frame]
    frames.set(panel.id, {
      data: previous,
      isStale: Boolean(previous),
      isLoading: true,
      error: null,
      retry: retryFrame,
    })
    const currentNavigation = { ...queryView, path: [...queryView.path] }
    const perspective = document.perspectives.find(({ id }) => id === currentNavigation.perspectiveId)
    const request = {
      ...requestFor(document, currentNavigation),
      ...(panel.kind === 'table' || perspective?.semantics === 'evidence' ? { page: 1 } : {}),
    }
    const force = forceRetry.current
    forceRetry.current = false
    void queryWithSnapshotRecovery({
      request,
      navigation: currentNavigation,
      loadDocument: refreshDocument,
      query: (next) => queryClient.query(next, { force }),
    }).then((result) => {
      if (!active) return
      if (result.reset) {
        replaceNextURL.current = true
        dispatch(navigationActions.restore(result.navigation))
        setNotice(driftNotice())
        return
      }
      const frames = Object.entries(result.response.frames)
      const frame = frames[0]?.[1]
      if (frame?.children) {
        setRuntimeDocument((current) => withFrameChildren(current, currentNavigation.path, frame))
      }
      frameStore.current?.set(panel.id, {
        data: frame ?? previous,
        page: result.response.page,
        isStale: false,
        isLoading: false,
        error: null,
        retry: retryFrame,
      })
    }).catch((cause: unknown) => {
      if (!active) return
      const error = cause instanceof Error ? cause : new Error('query request failed')
      frames.set(panel.id, {
        data: previous,
        isStale: Boolean(previous),
        isLoading: false,
        error,
        retry: retryFrame,
      })
    })
    return () => { active = false }
  }, [dispatch, document, driftNotice, frames, queryClient, refreshDocument, retryFrame, retryToken, runtimeView, setRuntimeDocument])

  const loadPage = useCallback(async (panelId: string, page: number, force = false) => {
    const sourcePanel = sourceDocument.panels.find((candidate) => candidate.id === panelId)
    if (panelClient && sourcePanel?.table?.searchable && page >= 1) {
      const previousState = frames.get(panelId)
      const previous = previousState?.data ?? sourceDocument.frames[sourcePanel.frame]
      const retry = () => { void pageLoader.current?.(panelId, page, true) }
      frames.set(panelId, { ...previousState, data: previous, isStale: Boolean(previous), isLoading: true, error: null, retry })
      try {
        const response = await panelClient.load({
          snapshotId: sourceDocument.snapshotId, panelId, ...tableRequestState.current.get(panelId), page,
        })
        const loaded = response.frames[sourcePanel.frame] ?? Object.values(response.frames)[0]
        if (!loaded) throw new Error(`panel ${panelId} response has no frame`)
        frames.set(panelId, { data: loaded, page: response.page, isStale: false, isLoading: false, error: null, retry, calculation: response.calculation, summary: response.summary })
      } catch (cause: unknown) {
        frames.set(panelId, { ...previousState, data: previous, isStale: Boolean(previous), isLoading: false, error: cause instanceof Error ? cause : new Error('panel page failed'), retry })
      }
      return
    }
    const panel = panelForNavigation(document, runtimeView)
    if (!queryClient || !panel || panel.id !== panelId || runtimeView.path.length === 0 || page < 1) return
    const previousState = frames.get(panelId)
    const previous = previousState?.data ?? document.frames[panel.frame]
    const retryPage = () => { void pageLoader.current?.(panelId, page, true) }
    frames.set(panelId, {
      data: previous,
      page: previousState?.page,
      isStale: Boolean(previous),
      isLoading: true,
      error: null,
      retry: retryPage,
    })
    const currentNavigation = { ...runtimeView, path: [...runtimeView.path] }
    try {
      const result = await queryWithSnapshotRecovery({
        request: { ...requestFor(document, currentNavigation), page },
        navigation: currentNavigation,
        loadDocument: refreshDocument,
        query: (next) => queryClient.query(next, { force }),
      })
      if (result.reset) {
        replaceNextURL.current = true
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
  }, [dispatch, document, driftNotice, frames, panelClient, queryClient, refreshDocument, runtimeView, sourceDocument])
  pageLoader.current = loadPage

  const searchPanel = useCallback(async (panelId: string, search: string) => {
    if (!panelClient) return
    const panel = sourceDocument.panels.find((candidate) => candidate.id === panelId)
    if (!panel?.table?.searchable) return
    const previousState = frames.get(panelId)
    const previous = previousState?.data ?? sourceDocument.frames[panel.frame]
    const requestState = { ...tableRequestState.current.get(panelId), search: search || undefined }
    tableRequestState.current.set(panelId, requestState)
    const retry = () => { void searchLoader.current?.(panelId, search) }
    frames.set(panelId, {
      data: previous, isStale: Boolean(previous), isLoading: !previous, error: null, retry,
      calculation: previousState?.calculation, summary: previousState?.summary,
    })
    try {
      const response = await panelClient.load({ snapshotId: sourceDocument.snapshotId, panelId, ...requestState })
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
  }, [frames, panelClient, refreshDocument, sourceDocument])
  searchLoader.current = searchPanel

  const sortPanel = useCallback(async (panelId: string, sort: TableSort) => {
    if (!panelClient) return
    const panel = sourceDocument.panels.find((candidate) => candidate.id === panelId)
    if (!panel || panel.presentation?.sortable === false) return
    const requestState = { ...tableRequestState.current.get(panelId), sort }
    tableRequestState.current.set(panelId, requestState)
    const previousState = frames.get(panelId)
    const previous = previousState?.data ?? sourceDocument.frames[panel.frame]
    const retry = () => { void sortLoader.current?.(panelId, sort) }
    frames.set(panelId, { data: previous, isStale: Boolean(previous), isLoading: !previous, error: null, retry, summary: previousState?.summary })
    try {
      const response = await panelClient.load({ snapshotId: sourceDocument.snapshotId, panelId, ...requestState })
      const loaded = response.frames[panel.frame] ?? Object.values(response.frames)[0]
      if (!loaded) throw new Error(`panel ${panel.id} response has no frame`)
      frames.set(panelId, { data: loaded, page: response.page, isStale: false, isLoading: false, error: null, retry, calculation: response.calculation, summary: response.summary })
    } catch (cause: unknown) {
      frames.set(panelId, { data: previous, isStale: Boolean(previous), isLoading: false, error: cause instanceof Error ? cause : new Error('panel sort failed'), retry, summary: previousState?.summary })
    }
  }, [frames, panelClient, sourceDocument])
  sortLoader.current = sortPanel

  const pagination = useMemo<PanelPaginationContextValue>(() => ({
    loadPage: (panelId, page) => loadPage(panelId, page),
    search: (panelId, value) => searchPanel(panelId, value),
    sort: (panelId, value) => sortPanel(panelId, value),
  }), [loadPage, searchPanel, sortPanel])

  const runExport = useCallback(async (panelId?: string) => {
    const scope = exportScope(panelId)
    const exportEndpoint = document.endpoints.export
    if (!exportEndpoint) return
    setExportStates((states) => ({ ...states, [scope]: { status: 'pending' } }))
    try {
      const exporter = await import('./export')
      const workbook = await exporter.exportWorkbook({
        endpoint: exportEndpoint,
        snapshotId: document.snapshotId,
        panelId,
        csrf,
        fetcher,
      })
      exporter.downloadWorkbook(workbook)
      setExportStates((states) => ({ ...states, [scope]: { status: 'idle' } }))
    } catch (cause: unknown) {
      const { ExportSnapshotGoneError } = await import('./export')
      if (cause instanceof ExportSnapshotGoneError) {
        try {
          await refreshDocument()
          setExportStates((states) => ({
            ...states,
            [scope]: { status: 'retry', message: translate('export.retryHint', 'Snapshot refreshed. Retry export.') },
          }))
        } catch (refreshCause: unknown) {
          const message = refreshCause instanceof Error ? refreshCause.message : 'Snapshot refresh failed'
          setExportStates((states) => ({ ...states, [scope]: { status: 'error', message } }))
        }
        return
      }
      const message = cause instanceof Error ? cause.message : 'Export failed'
      setExportStates((states) => ({ ...states, [scope]: { status: 'error', message } }))
    }
  }, [csrf, document.endpoints.export, document.snapshotId, fetcher, refreshDocument, translate])

  const exportContext = useMemo<ExportContextValue>(() => ({
    available: Boolean(document.endpoints.export),
    state: (panelId) => exportStates[exportScope(panelId)] ?? { status: 'idle' },
    run: runExport,
  }), [document.endpoints.export, exportStates, runExport])

  const runPrint = useCallback(async ({ preview = false }: { preview?: boolean } = {}) => {
    if (drawerDepth > 0 || typeof window === 'undefined' || printState.status === 'pending') return
    setPrintState({ active: true, status: 'pending', preview })
    const printQueryClients = new Map<string, QueryClient>()
    // Audit exports deliberately favour completeness over dashboard-like
    // latency: deep product and period lenses can be expensive but must not
    // silently disappear from the non-interactive report.
    const reportSignal = timeoutSignal(90_000)
    const detailSignal = () => anySignal([reportSignal, timeoutSignal(20_000)])
    try {
      const { buildPrintReport } = await import('./print')
      const report = await buildPrintReport(
        document,
        (request, owner = document) => {
          const endpoint = owner.endpoints.query
          if (!endpoint) {
            return Promise.reject(new Error(
              `lens: ${owner.meta.dashboardId} declares no query endpoint, so its detail cannot be printed`,
            ))
          }
          let client = printQueryClients.get(endpoint)
          if (!client) {
            client = new QueryClient(endpoint, { csrf, fetcher })
            printQueryClients.set(endpoint, client)
          }
          return client.query(request, { signal: detailSignal() })
        },
        2_000,
        (src) => fetchDocument(src, {
          csrf,
          fetcher,
          signal: detailSignal(),
        }),
        new URL(location.href),
        reportSignal,
      )
      for (const client of printQueryClients.values()) client.dispose()
      setPrintState({ active: true, status: 'idle', report, preview })
      // Preview stops here: the composed document stays on screen, on canvas,
      // for review — no print dialog, nothing to dismiss.
      if (preview) return
      // Let React commit the flattened report and let canvas adapters mount
      // before the print engine snapshots the page.
      await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())))
      await globalThis.document.fonts?.ready
      await new Promise<void>((resolve) => setTimeout(resolve, 120))
      globalThis.document.documentElement.classList.add('lens-print-active')
      const cleanup = () => {
        globalThis.document.documentElement.classList.remove('lens-print-active')
        setPrintState({ active: false, status: 'idle', preview: false })
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
      setPrintState({
        active: false,
        status: 'error',
        preview: false,
        message: translate('print.failed', 'PDF export failed'),
      })
    }
  }, [csrf, document, drawerDepth, fetcher, printState.status, translate])

  const printContext = useMemo<PrintContextValue>(() => ({
    available: drawerDepth === 0 && typeof window !== 'undefined',
    ...printState,
    run: runPrint,
  }), [drawerDepth, printState, runPrint])

  const setPeriod = useCallback((filter: Filter, value: PeriodValue) => {
    if (!filtersEnabled || typeof window === 'undefined' || !filter.period) return
    const merged = periodTransitionValues(documentRef.current, filter, value, filterValuesRef.current)
    // An unbounded period has no finite span from which "previous period" or
    // "year ago" can be derived. Canonicalize every comparison bound to this
    // period in the same history entry so the visible off state, copied URL,
    // refetch and export scope can never disagree.
    const current = new URL(window.location.href)
    const next = writeFilterValues(current, documentRef.current, merged)
    if (!sameNavigationURL(current, next)) {
      window.history.pushState(browserStateFor(navigation, window.history.state), '', next)
    }
    syncFiltersFromURL()
  }, [filtersEnabled, navigation, syncFiltersFromURL])

  const applyFilterURL = useCallback((target: string | URL, options?: { newTab?: boolean }) => {
    if (!filtersEnabled || typeof window === 'undefined') return
    const next = new URL(target, window.location.href)
    if (next.origin !== window.location.origin) return
    if (options?.newTab) {
      window.open(next.href, '_blank', 'noopener')
      return
    }
    const current = new URL(window.location.href)
    if (!sameNavigationURL(current, next)) {
      window.history.pushState(browserStateFor(navigation, window.history.state), '', next)
    }
    syncFiltersFromURL()
  }, [filtersEnabled, navigation, syncFiltersFromURL])

  const setCompare = useCallback((filter: Filter, value: CompareValue) => {
    if (!filter.compare || typeof window === 'undefined') return
    const merged = { ...filterValuesRef.current }
    delete merged[filter.compare.startParam]
    delete merged[filter.compare.endParam]
    Object.assign(merged, compareValues(filter.compare, value))
    const next = writeFilterValues(new URL(window.location.href), documentRef.current, merged)
    applyFilterURL(next)
  }, [applyFilterURL])

  // A segmented choice is a single parameter with a closed option set, so it
  // needs no staging step and no cross-filter canonicalization: it merges into
  // the current values and goes through the same applyFilterURL funnel every
  // other control ends in.
  const setSegmented = useCallback((filter: Filter, value: string) => {
    if (!filter.segmented || typeof window === 'undefined') return
    if (!filter.segmented.options.some((option) => option.value === value)) return
    const merged = { ...filterValuesRef.current, ...segmentedValues(filter.segmented, value) }
    applyFilterURL(writeFilterValues(new URL(window.location.href), documentRef.current, merged))
  }, [applyFilterURL])

  const filters = useMemo<FiltersContextValue>(() => ({
    filters: filtersEnabled ? declaredFilters(document) : [],
    values: filterValues,
    setPeriod,
    setCompare,
    setSegmented,
    applyURL: applyFilterURL,
  }), [applyFilterURL, document, filterValues, filtersEnabled, setCompare, setPeriod, setSegmented])

  const drill = useMemo<DrillContextValue>(() => ({
    drillInto: (nodeKey, panelId) => dispatch(navigationActions.drillInto(nodeKey, panelId)),
    prefetch: prefetchDrill,
    back: () => dispatch(navigationActions.back()),
    jumpTo: (breadcrumbIndex) => dispatch(navigationActions.jumpTo(breadcrumbIndex)),
    switchPerspective: (id, options) => {
      if (options?.replace) replaceNextURL.current = true
      dispatch(navigationActions.switchPerspective(id, undefined, options?.replace, options?.enter, options?.panelId))
    },
    reset: () => dispatch(navigationActions.reset()),
    canGoBack: navigation.history.length > 0,
  }), [dispatch, navigation.history.length, prefetchDrill])
  const closeDrawer = useCallback(() => {
    if (!navigation.drawer || controlledNavigation) return
    if (drawerOpener.current && typeof window !== 'undefined') {
      let steps = 1
      for (let index = navigation.history.length - 1; index >= 0; index -= 1) {
        if (!navigation.history[index]?.drawer) break
        steps += 1
      }
      window.history.go(-steps)
      return
    }
    replaceNextURL.current = true
    dispatch(navigationActions.closeDrawer())
  }, [controlledNavigation, dispatch, navigation.drawer, navigation.history])
  const warmDrawerSource = useCallback(async (src: string, signal?: AbortSignal) => {
    if (!isSameOriginDrawerSource(src) || signal?.aborted) return
    await drawerCache?.prefetch(src)
  }, [drawerCache])
  const warmDrawerKey = useCallback(async (metricKey: string, signal?: AbortSignal) => {
    const endpoint = document.endpoints.drawer
    if (!endpoint || !metricKey.trim() || signal?.aborted) return
    const response = await (fetcher ?? fetch)(endpoint, {
      method: 'POST', credentials: 'same-origin', signal,
      headers: { 'Content-Type': 'application/json', ...(csrf ? { 'X-CSRF-Token': csrf } : {}) },
      body: JSON.stringify({ snapshotId: document.snapshotId, metricKey }),
    })
    if (!response.ok) throw new Error(`drawer resolver failed (${response.status})`)
    const payload = await response.json() as { url?: unknown }
    if (typeof payload.url !== 'string' || !isSameOriginDrawerSource(payload.url)) throw new Error('drawer resolver returned an invalid URL')
    if (signal?.aborted) return
    const relative = siteRelativeURL(payload.url, new URL(window.location.href))
    if (relative) await drawerCache?.prefetch(relative)
  }, [csrf, document.endpoints.drawer, document.snapshotId, drawerCache, fetcher])
  const drawer = useMemo<DrawerContextValue>(() => ({
    depth: drawerDepth,
    canOpen: drawerDepth === 0 || Boolean(onDrawerNavigate),
    open: (src, opener) => {
      if (!isSameOriginDrawerSource(src)) return
      if (drawerDepth > 0) {
        onDrawerNavigate?.(src)
        return
      }
      if (navigation.drawer) return
      drawerOpener.current = opener ?? (
        globalThis.document.activeElement instanceof HTMLElement ? globalThis.document.activeElement : undefined
      )
      // The opener stays connected (the fullscreen panel is not collapsed), so
      // its `.lens-root` still resolves; fall back to any root when the drawer
      // opened without a captured element.
      const root = drawerOpener.current?.closest<HTMLElement>('.lens-root')
        ?? (typeof globalThis.document !== 'undefined'
          ? globalThis.document.querySelector<HTMLElement>('.lens-root')
          : null)
      const theme = root?.dataset.theme
      drawerTheme.current = {
        theme,
        dark: theme === 'dark' || root?.classList.contains('dark') === true,
      }
      dispatch(navigationActions.openDrawer(
        src,
        drawerNavigationFromSource(src, new URL(window.location.href)),
      ))
    },
    openKey: (metricKey, opener) => {
      const endpoint = document.endpoints.drawer
      if (!endpoint || !metricKey.trim()) return
      void (fetcher ?? fetch)(endpoint, {
        method: 'POST', credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json', ...(csrf ? { 'X-CSRF-Token': csrf } : {}) },
        body: JSON.stringify({ snapshotId: document.snapshotId, metricKey }),
      }).then(async (response) => {
        if (!response.ok) throw new Error(`drawer resolver failed (${response.status})`)
        const payload = await response.json() as { url?: unknown }
        if (typeof payload.url !== 'string' || !isSameOriginDrawerSource(payload.url)) throw new Error('drawer resolver returned an invalid URL')
        const root = opener?.closest<HTMLElement>('.lens-root')
          ?? globalThis.document.querySelector<HTMLElement>('.lens-root')
        drawerOpener.current = opener
        const theme = root?.dataset.theme
        drawerTheme.current = { theme, dark: theme === 'dark' || root?.classList.contains('dark') === true }
        const current = new URL(window.location.href)
        const relative = siteRelativeURL(payload.url, current)
        if (!relative) throw new Error('drawer resolver returned a cross-origin URL')
        dispatch(navigationActions.openDrawer(relative, drawerNavigationFromSource(relative, current)))
      }).catch((cause: unknown) => setNotice(cause instanceof Error ? cause.message : 'drawer resolver failed'))
    },
    close: closeDrawer,
    prefetch: (src) => {
      if (drawerDepth > 0 || navigation.drawer || !isSameOriginDrawerSource(src)) return () => undefined
      void warmDrawerSource(src)
      return () => undefined
    },
    prefetchKey: (metricKey) => {
      if (drawerDepth > 0 || navigation.drawer || !document.endpoints.drawer || !metricKey.trim()) return () => undefined
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
      return drawerIdleQueue?.register(`src:${src}`, (signal) => warmDrawerSource(src, signal)) ?? (() => undefined)
    },
    prefetchIdleKey: (metricKey) => {
      if (drawerDepth > 0 || !document.endpoints.drawer || !metricKey.trim()) return () => undefined
      return drawerIdleQueue?.register(`key:${metricKey}`, (signal) => warmDrawerKey(metricKey, signal)) ?? (() => undefined)
    },
  }), [closeDrawer, csrf, dispatch, document.endpoints.drawer, document.snapshotId, drawerDepth, drawerIdleQueue, fetcher, navigation.drawer, onDrawerNavigate, warmDrawerKey, warmDrawerSource])
  const dashboard = useMemo(() => ({
    document, navigation, notice, dismissNotice: () => setNotice(undefined),
    canRecompute: Boolean(panelClient), isRecomputing, recompute,
  }), [document, isRecomputing, navigation, notice, panelClient, recompute])

  return (
    <LocaleContext.Provider value={locale}>
      <I18nContext.Provider value={document.i18n}>
        <DashboardContext.Provider value={dashboard}>
          <FiltersContext.Provider value={filters}>
            <DrawerContext.Provider value={drawer}>
              <DrillContext.Provider value={drill}>
                <PanelPaginationContext.Provider value={pagination}>
                  <ExportContext.Provider value={exportContext}>
                    <PrintContext.Provider value={printContext}>
                      <FramesContext.Provider value={frames}>
                        {notice && <RuntimeNotice notice={notice} onDismiss={() => setNotice(undefined)} />}
                        {children}
                        {navigation.drawer && drawerDepth === 0 && (
                        // DocumentProvider wraps the drawer so its sticky top-bar
                        // header can read the loaded document's own identity block
                        // (eyebrow/title/caption) and render it once — while still
                        // mounting the close button immediately, before the document
                        // lands, because DocumentProvider renders its children
                        // regardless of load state.
                          <DocumentProvider src={navigation.drawer.src} csrf={csrf} fetcher={fetcher} cache={drawerCache}>
                            <LensDrawer
                              closeLabel={translate('drawer.close', 'Close details')}
                              dark={drawerTheme.current.dark}
                              eyebrow={translate('drawer.eyebrow', 'Detail view')}
                              label={translate('drawer.label', 'Drill details')}
                              onClose={closeDrawer}
                              restoreFocus={drawerOpener.current}
                              theme={drawerTheme.current.theme}
                            >
                              <DashboardRuntimeProvider
                                controlledNavigation={nestedDrawerState(navigation.drawer, navigation.history)}
                                csrf={csrf}
                                drawerDepth={1}
                                // A drawer-shaped loading placeholder (headline stat +
                                // table), not the dashboard's stat-strip + chart pair,
                                // so the drawer body does not jump when the drill
                                // document lands.
                                fallback={<DashboardSkeleton rows={drawerSkeletonRows} />}
                                fetcher={fetcher}
                                locale={locale}
                                onControlledNavigationChange={(view) => dispatch(navigationActions.updateDrawer(view))}
                                onDrawerNavigate={(src) => dispatch(navigationActions.replaceDrawer(
                                  src,
                                  drawerNavigationFromSource(src, new URL(window.location.href)),
                                ))}
                              >
                                {children}
                              </DashboardRuntimeProvider>
                            </LensDrawer>
                          </DocumentProvider>
                        )}
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
  children: ReactNode
  /** Server-rendered placeholder shown until the first document arrives. */
  fallback?: ReactNode
  controlledNavigation?: NavigationState
  onControlledNavigationChange?: (view: NavigationView) => void
  onDrawerNavigate?: (src: string) => void
  drawerDepth?: number
}

export function DashboardRuntimeProvider({
  locale, csrf, fetcher, children, fallback, controlledNavigation, onControlledNavigationChange, onDrawerNavigate, drawerDepth,
}: DashboardRuntimeProviderProps) {
  const context = useContext(DocumentContext)
  if (!context) throw new Error('DashboardRuntimeProvider must be inside DocumentProvider')
  if (!context.document) {
    if (context.error) {
      return (
        <DocumentLoadError
          locale={locale}
          message={context.error.message}
          onRetry={() => { void context.refresh().catch(() => undefined) }}
        />
      )
    }
    // A layout-shaped placeholder, not a spinner: the page keeps its rhythm
    // and nothing jumps when the document lands.
    return <DocumentLoading locale={locale}>{fallback ?? <DashboardSkeleton rows={defaultSkeletonRows} />}</DocumentLoading>
  }
  return (
    <RuntimeCore
      document={context.document}
      locale={locale}
      csrf={csrf}
      fetcher={fetcher}
      refreshDocument={context.refresh}
      applyFilters={context.applyFilters}
      controlledNavigation={controlledNavigation}
      onControlledNavigationChange={onControlledNavigationChange}
      onDrawerNavigate={onDrawerNavigate}
      drawerDepth={drawerDepth}
    >
      {children}
    </RuntimeCore>
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
  const empty = useMemo<PanelFrameState>(() => ({
    isStale: false,
    isLoading: false,
    error: null,
    retry: () => undefined,
  }), [])
  const subscribe = useCallback((subscriber: () => void) => store.subscribe(panelId, subscriber), [panelId, store])
  const getSnapshot = useCallback(() => store.get(panelId) ?? empty, [empty, panelId, store])
  const state = useSyncExternalStore(subscribe, getSnapshot, getSnapshot)
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
  return { ...context.state(panelId), available: context.available, run: () => context.run(panelId) }
}

export function usePrint(): PrintContextValue {
  const context = useContext(PrintContext)
  if (!context) throw new Error('usePrint must be used inside DashboardRuntimeProvider')
  return context
}

export function useFormat(field?: FieldFormat): (value: unknown) => string {
  const locale = useContext(LocaleContext)
  return useCallback((value: unknown) => formatFieldValue(value, field, locale), [field, locale])
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
export function useFormatAtReference(field: FieldFormat | undefined, reference: number | undefined): (value: unknown) => string {
  const locale = useContext(LocaleContext)
  return useCallback((value: unknown) => (reference === undefined
    ? formatFieldValue(value, field, locale)
    : formatFieldValueAtReference(value, reference, field, locale)), [field, locale, reference])
}

/**
 * The tooltip companion to useFormat for compact fields: returns the exact
 * grouped value («66 064 767 694 UZS») or undefined when nothing was
 * abbreviated away.
 */
export function useFormatExact(field?: FieldFormat): (value: unknown) => string | undefined {
  const locale = useContext(LocaleContext)
  return useCallback((value: unknown) => formatFieldValueExact(value, field, locale), [field, locale])
}

export function useAxisFormat(field?: FieldFormat): (value: unknown) => string {
  const locale = useContext(LocaleContext)
  return useCallback((value: unknown) => formatAxis(value, field, locale), [field, locale])
}

/**
 * The document is what carries the catalogue, so a failure to load it is the
 * one string the runtime can only render in English unless the host page
 * supplies its own fallback UI.
 */
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

function DocumentLoadError({ locale, message, onRetry }: { locale?: string; message: string; onRetry: () => void }) {
  const translate = useTranslate()
  return (
    <div className="lens-placeholder-state" role="alert">
      <span>
        {translate('runtime.loadError', preDocumentText(locale, 'runtime.loadError', 'Unable to load Lens document'))}
        : {message}
      </span>
      {/* A failed document leaves the page with nothing on it. Panels already
          offer their own retry; without one here the only way back is a manual
          reload, which most readers of a dashboard will not think to try. */}
      <button className="lens-placeholder-retry" onClick={onRetry} type="button">
        {translate('runtime.retry', preDocumentText(locale, 'runtime.retry', 'Retry'))}
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

function DocumentLoading({ locale, children }: { locale?: string; children: ReactNode }) {
  const translate = useTranslate()
  const [slow, setSlow] = useState(false)
  useEffect(() => {
    const timer = setTimeout(() => setSlow(true), slowLoadNoticeMs)
    return () => clearTimeout(timer)
  }, [])
  return (
    <div aria-busy="true" className="lens-loading">
      {slow && (
        <p className="lens-loading-notice" role="status">
          {translate(
            'runtime.slowLoad',
            preDocumentText(locale, 'runtime.slowLoad', 'Still computing — a wide period takes longer.'),
          )}
        </p>
      )}
      {children}
    </div>
  )
}

function RuntimeNotice({ notice, onDismiss }: { notice: string; onDismiss: () => void }) {
  const translate = useTranslate()
  return (
    <div className="lens-runtime-notice" role="status">
      <span>{notice}</span>
      <button
        aria-label={translate('runtime.dismissNotice', 'Dismiss notice')}
        onClick={onDismiss}
        type="button"
      >
        <X />
      </button>
    </div>
  )
}

export function useTranslate(): (key: string, fallback: string, vars?: TranslationVars) => string {
  const messages = useContext(I18nContext)
  return useCallback(
    (key: string, fallback: string, vars?: TranslationVars) => translation(messages, key, fallback, vars),
    [messages],
  )
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
export function useDrawerHeader(): DashboardDocument['drawer'] {
  return useContext(DocumentContext)?.document?.drawer
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
