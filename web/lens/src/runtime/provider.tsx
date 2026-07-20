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
import type { DashboardDocument, FieldFormat, Frame, Panel, QueryPage, QueryRequest } from '../contract'
import { fetchDocument } from './document'
import { levelForPath, panelForNavigation, pathResolves, rootNavigation } from './drill'
import { downloadWorkbook, ExportSnapshotGoneError, exportWorkbook } from './export'
import { DashboardSkeleton, defaultSkeletonRows } from '../panels/Skeleton'
import { formatAxis, formatFieldValue } from './format'
import {
  createNavigationState,
  navigationActions,
  navigationReducer,
  type NavigationState,
  type NavigationView,
} from './navigation'
import { QueryClient } from './query'
import { queryWithSnapshotRecovery } from './recovery'
import { navigationFromURL, navigationToURL, sameNavigationURL } from './url'

/* eslint-disable react-refresh/only-export-components */

interface DocumentContextValue {
  document?: DashboardDocument
  isLoading: boolean
  error: Error | null
  refresh: () => Promise<DashboardDocument>
}

const DocumentContext = createContext<DocumentContextValue | undefined>(undefined)

export interface DocumentProviderProps {
  src?: string
  initialDocument?: DashboardDocument
  csrf?: string
  fetcher?: typeof fetch
  children: ReactNode
}

export function DocumentProvider({ src, initialDocument, csrf, fetcher, children }: DocumentProviderProps) {
  const [document, setDocument] = useState<DashboardDocument | undefined>(() => src ? undefined : initialDocument)
  const [isLoading, setIsLoading] = useState(Boolean(src))
  const [error, setError] = useState<Error | null>(null)
  const controllers = useRef(new Set<AbortController>())
  const inFlight = useRef<Promise<DashboardDocument>>()
  const csrfRef = useRef(csrf)
  const fetcherRef = useRef(fetcher)

  useEffect(() => {
    csrfRef.current = csrf
    fetcherRef.current = fetcher
  }, [csrf, fetcher])

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
    setError(null)
    const pending = fetchDocument(src, { csrf: csrfRef.current, fetcher: fetcherRef.current, signal: controller.signal })
      .then((next) => {
        setDocument(next)
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
  }, [initialDocument, src])

  useEffect(() => {
    setDocument(src ? undefined : initialDocument)
    setError(null)
    if (src) void refresh().catch(() => undefined)
  }, [initialDocument, refresh, src])

  useEffect(() => () => {
    for (const controller of controllers.current) controller.abort()
    controllers.current.clear()
  }, [])

  const value = useMemo(() => ({ document, isLoading, error, refresh }), [document, error, isLoading, refresh])
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
  switchPerspective: (id: string, options?: { replace?: boolean }) => void
  reset: () => void
  canGoBack: boolean
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
  return left.panelId === right.panelId && left.perspectiveId === right.perspectiveId &&
    left.path.length === right.path.length && left.path.every((key, index) => key === right.path[index])
}

function resolveView(document: DashboardDocument, view: NavigationView): NavigationView | undefined {
  if (!pathResolves(document, view.path, view.perspectiveId)) return undefined
  return {
    path: [...view.path],
    perspectiveId: view.perspectiveId,
    panelId: panelForNavigation(document, view)?.id,
  }
}

function parseBrowserView(value: unknown): NavigationView | undefined {
  if (!value || typeof value !== 'object') return undefined
  const candidate = value as Record<string, unknown>
  if (!Array.isArray(candidate.path) || !candidate.path.every((key) => typeof key === 'string')) return undefined
  if (candidate.panelId !== undefined && typeof candidate.panelId !== 'string') return undefined
  if (candidate.perspectiveId !== undefined && typeof candidate.perspectiveId !== 'string') return undefined
  return {
    path: [...candidate.path] as Array<string>,
    panelId: candidate.panelId,
    perspectiveId: candidate.perspectiveId,
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
  return history
}

function navigationFromBrowserState(
  document: DashboardDocument,
  view: NavigationView,
  state: unknown,
): NavigationState {
  const resolved = resolveView(document, view) ?? rootNavigation(document, view.panelId)
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
  })
  const lens: BrowserNavigationState = { view: clone(navigation), history: navigation.history.map(clone) }
  return { ...state, [browserHistoryKey]: lens }
}

function inferredInitialNavigation(document: DashboardDocument): NavigationState {
  if (typeof window === 'undefined') return createNavigationState()
  const fromURL = navigationFromURL(new URL(window.location.href))
  return navigationFromBrowserState(document, fromURL, window.history.state)
}

function requestFor(document: DashboardDocument, navigation: NavigationView): QueryRequest {
  return {
    snapshotId: document.snapshotId,
    path: navigation.path,
    ...(navigation.perspectiveId ? { perspective: navigation.perspectiveId } : {}),
  }
}

function runtimeNavigationReducer(
  document: DashboardDocument,
  state: NavigationState,
  action: Parameters<typeof navigationReducer>[1],
): NavigationState {
  if (action.type === 'drillInto') {
    const panelChanged = Boolean(action.panelId && action.panelId !== state.panelId)
    const panel = action.panelId ? document.panels.find((candidate) => candidate.id === action.panelId) : undefined
    const level = panelChanged || state.path.length === 0
      ? (panel?.drillRoot ? document.drill.edges[panel.drillRoot] : undefined)
      : levelForPath(document, state.path)
    const child = level?.children.find((candidate) => candidate.key === action.nodeKey)
    const target = child?.target ? document.drill.edges[child.target] : undefined
    const path = action.nodeKey === panel?.drillRoot
      ? document.drill.edges[action.nodeKey]?.path
      : target?.path ?? child?.path
    const perspectiveId = panelChanged ? undefined : state.perspectiveId
    if (!path || !pathResolves(document, path, perspectiveId)) return state
    const next = navigationReducer(state, navigationActions.drillInto(action.nodeKey, action.panelId, path))
    return panelChanged ? { ...next, perspectiveId: undefined } : next
  }
  if (action.type === 'switchPerspective') {
    const level = levelForPath(document, state.path)
    if (!level?.perspectives.some((perspective) => perspective.id === action.perspectiveId)) return state
    const perspective = document.perspectives.find((candidate) => candidate.id === action.perspectiveId)
    const root = perspective ? document.drill.edges[perspective.root] : undefined
    if (!root) return state
    return navigationReducer(state, navigationActions.switchPerspective(action.perspectiveId, root.path, action.replace))
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
  const level = levelForPath(document, navigation.path)
  if (!level) return { frame: document.frames[panel.frame], shouldQuery: false }
  const levelKey = level.path.at(-1)
  const isPerspectiveSegment = level.perspectives.some(({ id }) => {
    const perspective = document.perspectives.find((candidate) => candidate.id === id)
    return perspective?.branchKey === levelKey
  })
  if (isPerspectiveSegment && !level.frame) {
    return { frame: document.frames[panel.frame], shouldQuery: false }
  }
  if (level.frame) {
    const frame = loadedFrames.get(level.frame) ?? document.frames[level.frame]
    if (frame) return { frame, shouldQuery: false }
  }
  return { shouldQuery: Boolean(document.endpoints.query) }
}

interface RuntimeCoreProps {
  document: DashboardDocument
  locale: string
  csrf?: string
  fetcher?: typeof fetch
  refreshDocument: () => Promise<DashboardDocument>
  children: ReactNode
}

function RuntimeCore({ document, locale, csrf, fetcher, refreshDocument, children }: RuntimeCoreProps) {
  const [navigation, dispatch] = useReducer(
    (state: NavigationState, action: Parameters<typeof navigationReducer>[1]) => runtimeNavigationReducer(document, state, action),
    document,
    inferredInitialNavigation,
  )
  const [notice, setNotice] = useState<string>()
  const [exportStates, setExportStates] = useState<Record<string, ExportState>>({})
  const exportSnapshotId = useRef(document.snapshotId)
  const [retryToken, setRetryToken] = useState(0)
  const forceRetry = useRef(false)
  const pageLoader = useRef<(panelId: string, page: number, force?: boolean) => Promise<void>>()
  const replaceNextURL = useRef(true)
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
    if (pathResolves(document, navigation.path, navigation.perspectiveId)) return
    replaceNextURL.current = true
    dispatch(navigationActions.restore(rootNavigation(document, navigation.panelId)))
    setNotice(driftNotice())
  }, [document, driftNotice, navigation.panelId, navigation.path, navigation.perspectiveId])

  useEffect(() => {
    if (typeof window === 'undefined') return
    if (!pathResolves(document, navigation.path, navigation.perspectiveId)) return
    const current = new URL(window.location.href)
    const next = navigationToURL(navigation, current)
    const state = browserStateFor(navigation, window.history.state)
    if (replaceNextURL.current || sameNavigationURL(current, next)) window.history.replaceState(state, '', next)
    else window.history.pushState(state, '', next)
    replaceNextURL.current = false
  }, [document, navigation])

  useEffect(() => {
    if (typeof window === 'undefined') return
    const onPopState = (event: PopStateEvent) => {
      const view = navigationFromURL(new URL(window.location.href))
      const restored = navigationFromBrowserState(document, view, event.state)
      dispatch(navigationActions.restore(restored, restored.history))
    }
    window.addEventListener('popstate', onPopState)
    return () => window.removeEventListener('popstate', onPopState)
  }, [document])

  useEffect(() => {
    const panel = panelForNavigation(document, navigation)
    if (!panel) return
    const resolved = frameForPanel(document, navigation, panel, new Map())
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
    const currentNavigation = { ...navigation, path: [...navigation.path] }
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
  }, [document, driftNotice, frames, navigation, queryClient, refreshDocument, retryFrame, retryToken])

  const loadPage = useCallback(async (panelId: string, page: number, force = false) => {
    const panel = panelForNavigation(document, navigation)
    if (!queryClient || !panel || panel.id !== panelId || navigation.path.length === 0 || page < 1) return
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
    const currentNavigation = { ...navigation, path: [...navigation.path] }
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
  }, [document, driftNotice, frames, navigation, queryClient, refreshDocument])
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

  const drill = useMemo<DrillContextValue>(() => ({
    drillInto: (nodeKey, panelId) => dispatch(navigationActions.drillInto(nodeKey, panelId)),
    back: () => dispatch(navigationActions.back()),
    jumpTo: (breadcrumbIndex) => dispatch(navigationActions.jumpTo(breadcrumbIndex)),
    switchPerspective: (id, options) => {
      if (options?.replace) replaceNextURL.current = true
      dispatch(navigationActions.switchPerspective(id, undefined, options?.replace))
    },
    reset: () => dispatch(navigationActions.reset()),
    canGoBack: navigation.history.length > 0,
  }), [navigation.history.length])
  const dashboard = useMemo(() => ({ document, navigation, notice, dismissNotice: () => setNotice(undefined) }), [document, navigation, notice])

  return (
    <LocaleContext.Provider value={locale}>
      <I18nContext.Provider value={document.i18n}>
      <DashboardContext.Provider value={dashboard}>
        <DrillContext.Provider value={drill}>
          <PanelPaginationContext.Provider value={pagination}>
            <ExportContext.Provider value={exportContext}>
              <FramesContext.Provider value={frames}>
                {notice && <RuntimeNotice notice={notice} onDismiss={() => setNotice(undefined)} />}
                {children}
              </FramesContext.Provider>
            </ExportContext.Provider>
          </PanelPaginationContext.Provider>
        </DrillContext.Provider>
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
}

export function DashboardRuntimeProvider({ locale, csrf, fetcher, children, fallback }: DashboardRuntimeProviderProps) {
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
        ×
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
