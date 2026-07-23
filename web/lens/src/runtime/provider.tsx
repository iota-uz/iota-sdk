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
import type { DashboardDocument, FieldFormat, Filter, Frame, NodeKey, NodePath, Panel, PeriodValue, QueryPage, QueryRequest } from '../contract'
import { fetchDocument } from './document'
import {
  declaredFilters,
  periodValues,
  readFilterValues,
  sameFilterValues,
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
  queryPathForNavigation,
  rootNavigation,
  withFrameChildren,
  withInlineFrameChildren,
} from './drill'
import { downloadWorkbook, ExportSnapshotGoneError, exportWorkbook } from './export'
import { DashboardSkeleton, defaultSkeletonRows, drawerSkeletonRows } from '../panels/Skeleton'
import { formatAxis, formatFieldValue, formatFieldValueExact } from './format'
import {
  createNavigationState,
  navigationActions,
  navigationReducer,
  type NavigationState,
  type NavigationView,
} from './navigation'
import { LensDrawer } from './drawer'
import { DocumentCache } from './prefetch'
import { QueryClient } from './query'
import { queryWithSnapshotRecovery } from './recovery'
import { navigationFromURL, navigationToURL, sameNavigationURL } from './url'
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
  cache?: DocumentCache
  children: ReactNode
}

export function DocumentProvider({ src, initialDocument, csrf, fetcher, cache, children }: DocumentProviderProps) {
  const [document, setDocument] = useState<DashboardDocument | undefined>(
    () => src ? cache?.get(src) : initialDocument,
  )
  const [isLoading, setIsLoading] = useState(Boolean(src) && !(src && cache?.get(src)))
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
  }, [csrf, fetcher])

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
    const pending = fetchDocument(effectiveSrc() ?? src, { csrf: csrfRef.current, fetcher: fetcherRef.current, signal: controller.signal })
      .then((next) => {
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
  }, [effectiveSrc, initialDocument, src])

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
    setDocument(src ? cached : initialDocument)
    setError(null)
    if (cached) {
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
}

export interface DrillContextValue {
  drillInto: (nodeKey: string, panelId?: string) => void
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
}

export interface DrawerContextValue {
  depth: number
  open: (src: string, opener?: HTMLElement) => void
  close: () => void
  /** Warm a drill-drawer document on hover/focus intent before it is opened. */
  prefetch: (src: string) => void
}

export interface PanelFrameState {
  data?: Frame
  page?: QueryPage
  isStale: boolean
  isLoading: boolean
  error: Error | null
  retry: () => void
}

export type ExportStatus = 'idle' | 'pending' | 'retry' | 'error'

export interface ExportState {
  status: ExportStatus
  message?: string
}

export interface PanelPaginationContextValue {
  loadPage: (panelId: string, page: number) => Promise<void>
}

export interface ExportContextValue {
  available: boolean
  state: (panelId?: string) => ExportState
  run: (panelId?: string) => Promise<void>
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
  }
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
    const level = levelForPath(document, entered ?? state.path)
    if (!level?.perspectives.some((perspective) => perspective.id === action.perspectiveId)) return state
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
  drawerDepth?: number
}

function RuntimeCore({
  document: sourceDocument, locale, csrf, fetcher, refreshDocument, applyFilters, children,
  controlledNavigation, onControlledNavigationChange, drawerDepth = 0,
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
  const navigation = controlledNavigation ?? localNavigation
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
    const next = runtimeNavigationReducer(document, controlledNavigation, action)
    if (next !== controlledNavigation) onControlledNavigationChange?.(next)
  }, [controlledNavigation, document, onControlledNavigationChange])
  const [notice, setNotice] = useState<string>()
  const documentRef = useRef(document)
  documentRef.current = document
  const filtersEnabled = !controlledNavigation && drawerDepth === 0
  // Filter state derives from the URL alone: user actions write the URL, the
  // URL drives the refetch, and browser Back needs no resync timers because
  // popstate re-reads the same source of truth.
  const [filterValues, setFilterValuesState] = useState<FilterValues>(() => (
    typeof window === 'undefined' ? {} : readFilterValues(document, new URL(window.location.href))
  ))
  const filterValuesRef = useRef(filterValues)
  const syncFiltersFromURL = useCallback(() => {
    if (typeof window === 'undefined') return
    const values = readFilterValues(documentRef.current, new URL(window.location.href))
    if (sameFilterValues(values, filterValuesRef.current)) return
    filterValuesRef.current = values
    setFilterValuesState(values)
    applyFilters?.(values)
  }, [applyFilters])
  const [exportStates, setExportStates] = useState<Record<string, ExportState>>({})
  const exportSnapshotId = useRef(document.snapshotId)
  const [retryToken, setRetryToken] = useState(0)
  const forceRetry = useRef(false)
  const pageLoader = useRef<(panelId: string, page: number, force?: boolean) => Promise<void>>()
  const replaceNextURL = useRef(true)
  const drawerOpener = useRef<HTMLElement>()
  // The drawer portals to body and stacks above an expanded panel, so it carries
  // the theme of the root it opened from — captured from the opener when the
  // drawer opens, mirroring PanelFrame's overlay theme capture.
  const drawerTheme = useRef<{ theme?: string; dark: boolean }>({ dark: false })
  const drawerCache = useRef<DocumentCache>()
  if (drawerDepth === 0 && !drawerCache.current) drawerCache.current = new DocumentCache({ capacity: 8, csrf, fetcher })
  useEffect(() => {
    drawerCache.current?.configure({ csrf, fetcher })
  }, [csrf, fetcher])
  const frameStore = useRef<PanelFrameStore>()
  const retryFrame = useCallback(() => {
    forceRetry.current = true
    setRetryToken((value) => value + 1)
  }, [])
  if (!frameStore.current) frameStore.current = new PanelFrameStore()
  const frames = frameStore.current
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
        isLoading: false,
        error: null,
        retry: retryFrame,
      })
    }
  }
  const endpoint = document.endpoints.query
  const queryClient = useMemo(() => endpoint ? new QueryClient(endpoint, { csrf, fetcher }) : undefined, [csrf, endpoint, fetcher])

  useEffect(() => () => queryClient?.dispose(), [queryClient])

  useEffect(() => {
    if (exportSnapshotId.current === document.snapshotId) return
    exportSnapshotId.current = document.snapshotId
    setExportStates({})
  }, [document.snapshotId])

  useEffect(() => {
    for (const panel of document.panels) {
      frames.set(panel.id, {
        data: document.frames[panel.frame],
        isStale: false,
        isLoading: false,
        error: null,
        retry: retryFrame,
      })
    }
  }, [document, frames, retryFrame])

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
      const view = navigationFromURL(new URL(window.location.href))
      const restored = navigationFromBrowserState(document, view, event.state)
      dispatch(navigationActions.restore(restored, restored.history))
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
  }, [dispatch, document, driftNotice, frames, queryClient, refreshDocument, runtimeView])
  pageLoader.current = loadPage

  const pagination = useMemo<PanelPaginationContextValue>(() => ({
    loadPage: (panelId, page) => loadPage(panelId, page),
  }), [loadPage])

  const runExport = useCallback(async (panelId?: string) => {
    const scope = exportScope(panelId)
    const exportEndpoint = document.endpoints.export
    if (!exportEndpoint) return
    setExportStates((states) => ({ ...states, [scope]: { status: 'pending' } }))
    try {
      const workbook = await exportWorkbook({
        endpoint: exportEndpoint,
        snapshotId: document.snapshotId,
        panelId,
        csrf,
        fetcher,
      })
      downloadWorkbook(workbook)
      setExportStates((states) => ({ ...states, [scope]: { status: 'idle' } }))
    } catch (cause: unknown) {
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

  const setPeriod = useCallback((filter: Filter, value: PeriodValue) => {
    if (!filtersEnabled || typeof window === 'undefined' || !filter.period) return
    const merged = { ...filterValuesRef.current, ...periodValues(filter.period, value) }
    const current = new URL(window.location.href)
    const next = writeFilterValues(current, documentRef.current, merged)
    if (!sameNavigationURL(current, next)) {
      window.history.pushState(browserStateFor(navigation, window.history.state), '', next)
    }
    syncFiltersFromURL()
  }, [filtersEnabled, navigation, syncFiltersFromURL])

  const filters = useMemo<FiltersContextValue>(() => ({
    filters: filtersEnabled ? declaredFilters(document) : [],
    values: filterValues,
    setPeriod,
  }), [document, filterValues, filtersEnabled, setPeriod])

  const drill = useMemo<DrillContextValue>(() => ({
    drillInto: (nodeKey, panelId) => dispatch(navigationActions.drillInto(nodeKey, panelId)),
    back: () => dispatch(navigationActions.back()),
    jumpTo: (breadcrumbIndex) => dispatch(navigationActions.jumpTo(breadcrumbIndex)),
    switchPerspective: (id, options) => {
      if (options?.replace) replaceNextURL.current = true
      dispatch(navigationActions.switchPerspective(id, undefined, options?.replace, options?.enter, options?.panelId))
    },
    reset: () => dispatch(navigationActions.reset()),
    canGoBack: navigation.history.length > 0,
  }), [dispatch, navigation.history.length])
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
  const drawer = useMemo<DrawerContextValue>(() => ({
    depth: drawerDepth,
    open: (src, opener) => {
      if (drawerDepth > 0 || navigation.drawer || !isSameOriginDrawerSource(src)) return
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
      drawerTheme.current = { theme: root?.dataset.theme, dark: root?.classList.contains('dark') ?? false }
      dispatch(navigationActions.openDrawer(src))
    },
    close: closeDrawer,
    prefetch: (src) => {
      if (drawerDepth > 0 || navigation.drawer || !isSameOriginDrawerSource(src)) return
      void drawerCache.current?.prefetch(src)
    },
  }), [closeDrawer, dispatch, drawerDepth, navigation.drawer])
  const dashboard = useMemo(() => ({ document, navigation, notice, dismissNotice: () => setNotice(undefined) }), [document, navigation, notice])

  return (
    <LocaleContext.Provider value={locale}>
      <I18nContext.Provider value={document.i18n}>
      <DashboardContext.Provider value={dashboard}>
        <FiltersContext.Provider value={filters}>
        <DrawerContext.Provider value={drawer}>
        <DrillContext.Provider value={drill}>
          <PanelPaginationContext.Provider value={pagination}>
            <ExportContext.Provider value={exportContext}>
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
                  <DocumentProvider src={navigation.drawer.src} csrf={csrf} fetcher={fetcher} cache={drawerCache.current}>
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
                      >
                        {children}
                      </DashboardRuntimeProvider>
                    </LensDrawer>
                  </DocumentProvider>
                )}
              </FramesContext.Provider>
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
  drawerDepth?: number
}

export function DashboardRuntimeProvider({
  locale, csrf, fetcher, children, fallback, controlledNavigation, onControlledNavigationChange, drawerDepth,
}: DashboardRuntimeProviderProps) {
  const context = useContext(DocumentContext)
  if (!context) throw new Error('DashboardRuntimeProvider must be inside DocumentProvider')
  if (!context.document) {
    if (context.error) return <DocumentLoadError message={context.error.message} />
    // A layout-shaped placeholder, not a spinner: the page keeps its rhythm
    // and nothing jumps when the document lands.
    return (
      <div aria-busy="true" className="lens-loading">
        {fallback ?? <DashboardSkeleton rows={defaultSkeletonRows} />}
      </div>
    )
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

export function useFormat(field?: FieldFormat): (value: unknown) => string {
  const locale = useContext(LocaleContext)
  return useCallback((value: unknown) => formatFieldValue(value, field, locale), [field, locale])
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
function DocumentLoadError({ message }: { message: string }) {
  const translate = useTranslate()
  return (
    <div className="lens-placeholder-state" role="alert">
      {translate('runtime.loadError', 'Unable to load Lens document')}: {message}
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
