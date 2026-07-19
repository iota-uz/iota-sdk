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
} from 'react'
import type { DashboardDocument, FieldFormat, Frame, Panel, QueryRequest } from '../contract'
import { fetchDocument } from './document'
import { levelForPath, panelForNavigation, pathResolves, rootNavigation } from './drill'
import { formatFieldValue } from './format'
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
    const pending = fetchDocument(src, { csrf, fetcher, signal: controller.signal })
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
  }, [csrf, fetcher, initialDocument, src])

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
  switchPerspective: (id: string) => void
  reset: () => void
  canGoBack: boolean
}

export interface PanelFrameState {
  data?: Frame
  isStale: boolean
  isLoading: boolean
  error: Error | null
  retry: () => void
}

interface FramesContextValue {
  states: ReadonlyMap<string, PanelFrameState>
  retry: (panelId: string) => void
}

const DashboardContext = createContext<DashboardContextValue | undefined>(undefined)
const DrillContext = createContext<DrillContextValue | undefined>(undefined)
const FramesContext = createContext<FramesContextValue | undefined>(undefined)
const LocaleContext = createContext('en')

function inferredInitialView(document: DashboardDocument): NavigationView {
  if (typeof window === 'undefined') return { path: [] }
  const fromURL = navigationFromURL(new URL(window.location.href))
  if (!pathResolves(document, fromURL.path, fromURL.perspectiveId)) return { path: [] }
  return { ...fromURL, panelId: panelForNavigation(document, fromURL)?.id }
}

function requestFor(document: DashboardDocument, navigation: NavigationView): QueryRequest {
  return {
    snapshotId: document.snapshotId,
    path: navigation.path,
    ...(navigation.perspectiveId ? { perspective: navigation.perspectiveId } : {}),
  }
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
  const [navigation, dispatch] = useReducer(navigationReducer, document, (value) => createNavigationState(inferredInitialView(value)))
  const [notice, setNotice] = useState<string>()
  const [states, setStates] = useState<ReadonlyMap<string, PanelFrameState>>(() => new Map())
  const statesRef = useRef(states)
  const [retryToken, setRetryToken] = useState(0)
  const forceRetry = useRef(false)
  const urlMode = useRef<'push' | 'replace' | 'pop'>('replace')
  const endpoint = document.endpoints.query
  const queryClient = useMemo(() => endpoint ? new QueryClient(endpoint, { csrf, fetcher }) : undefined, [csrf, endpoint, fetcher])

  useEffect(() => () => queryClient?.dispose(), [queryClient])
  useEffect(() => { statesRef.current = states }, [states])

  useEffect(() => {
    if (pathResolves(document, navigation.path, navigation.perspectiveId)) return
    urlMode.current = 'replace'
    dispatch(navigationActions.restore(rootNavigation(document, navigation.panelId)))
    setNotice('The previous drill path is no longer available. Lens returned to the root view.')
  }, [document, navigation.panelId, navigation.path, navigation.perspectiveId])

  useEffect(() => {
    if (typeof window === 'undefined') return
    const current = new URL(window.location.href)
    const next = navigationToURL(navigation, current)
    if (urlMode.current === 'pop') {
      urlMode.current = 'push'
      return
    }
    if (!sameNavigationURL(current, next)) {
      if (urlMode.current === 'replace') window.history.replaceState(window.history.state, '', next)
      else window.history.pushState(window.history.state, '', next)
    }
    urlMode.current = 'push'
  }, [navigation])

  useEffect(() => {
    if (typeof window === 'undefined') return
    const onPopState = () => {
      urlMode.current = 'pop'
      const view = navigationFromURL(new URL(window.location.href))
      dispatch(navigationActions.restore({ ...view, panelId: panelForNavigation(document, view)?.id }))
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
        setStates((current) => new Map(current).set(panel.id, {
          data: resolved.frame, isStale: false, isLoading: false, error: null,
          retry: () => { forceRetry.current = true; setRetryToken((value) => value + 1) },
        }))
      }
      return
    }

    let active = true
    const previous = statesRef.current.get(panel.id)?.data ?? document.frames[panel.frame]
    setStates((current) => new Map(current).set(panel.id, {
      data: previous,
      isStale: Boolean(previous),
      isLoading: true,
      error: null,
      retry: () => { forceRetry.current = true; setRetryToken((value) => value + 1) },
    }))
    const currentNavigation = { ...navigation, path: [...navigation.path] }
    const request = requestFor(document, currentNavigation)
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
        urlMode.current = 'replace'
        dispatch(navigationActions.restore(result.navigation))
        setNotice('The previous drill path is no longer available. Lens returned to the root view.')
        return
      }
      const frames = Object.entries(result.response.frames)
      const frame = frames[0]?.[1]
      setStates((current) => new Map(current).set(panel.id, {
        data: frame ?? previous,
        isStale: false,
        isLoading: false,
        error: null,
        retry: () => { forceRetry.current = true; setRetryToken((value) => value + 1) },
      }))
    }).catch((cause: unknown) => {
      if (!active) return
      const error = cause instanceof Error ? cause : new Error('query request failed')
      setStates((current) => new Map(current).set(panel.id, {
        data: previous,
        isStale: Boolean(previous),
        isLoading: false,
        error,
        retry: () => { forceRetry.current = true; setRetryToken((value) => value + 1) },
      }))
    })
    return () => { active = false }
  }, [document, navigation, queryClient, refreshDocument, retryToken])

  const drill = useMemo<DrillContextValue>(() => ({
    drillInto: (nodeKey, panelId) => dispatch(navigationActions.drillInto(nodeKey, panelId)),
    back: () => dispatch(navigationActions.back()),
    jumpTo: (breadcrumbIndex) => dispatch(navigationActions.jumpTo(breadcrumbIndex)),
    switchPerspective: (id) => dispatch(navigationActions.switchPerspective(id)),
    reset: () => dispatch(navigationActions.reset()),
    canGoBack: navigation.history.length > 0,
  }), [navigation.history.length])
  const dashboard = useMemo(() => ({ document, navigation, notice, dismissNotice: () => setNotice(undefined) }), [document, navigation, notice])
  const frames = useMemo(() => ({
    states,
    retry: () => { forceRetry.current = true; setRetryToken((value) => value + 1) },
  }), [states])

  return (
    <LocaleContext.Provider value={locale}>
      <DashboardContext.Provider value={dashboard}>
        <DrillContext.Provider value={drill}>
          <FramesContext.Provider value={frames}>
            {notice && (
              <div className="lens-runtime-notice" role="status">
                <span>{notice}</span>
                <button type="button" onClick={() => setNotice(undefined)} aria-label="Dismiss notice">×</button>
              </div>
            )}
            {children}
          </FramesContext.Provider>
        </DrillContext.Provider>
      </DashboardContext.Provider>
    </LocaleContext.Provider>
  )
}

export interface DashboardRuntimeProviderProps {
  locale: string
  csrf?: string
  fetcher?: typeof fetch
  children: ReactNode
}

export function DashboardRuntimeProvider({ locale, csrf, fetcher, children }: DashboardRuntimeProviderProps) {
  const context = useContext(DocumentContext)
  if (!context) throw new Error('DashboardRuntimeProvider must be inside DocumentProvider')
  if (!context.document) {
    if (context.error) return <div className="lens-placeholder-state" role="alert">Unable to load Lens document: {context.error.message}</div>
    return <div className="lens-placeholder-state lens-skeleton" aria-busy="true">Loading dashboard…</div>
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
  const dashboard = useDashboard()
  if (!frames) throw new Error('usePanelFrame must be used inside DashboardRuntimeProvider')
  const panel = dashboard.document.panels.find((candidate) => candidate.id === panelId)
  const resolved = panel ? frameForPanel(dashboard.document, dashboard.navigation, panel, new Map()) : undefined
  return frames.states.get(panelId) ?? {
    data: resolved?.frame ?? (resolved?.shouldQuery && panel ? dashboard.document.frames[panel.frame] : undefined),
    isStale: Boolean(resolved?.shouldQuery && panel && dashboard.document.frames[panel.frame]),
    isLoading: Boolean(panel && resolved?.shouldQuery),
    error: null,
    retry: () => frames.retry(panelId),
  }
}

export function useFormat(field?: FieldFormat): (value: unknown) => string {
  const locale = useContext(LocaleContext)
  return useCallback((value: unknown) => formatFieldValue(value, field, locale), [field, locale])
}

export function useDocumentState(): DocumentContextValue {
  const context = useContext(DocumentContext)
  if (!context) throw new Error('useDocumentState must be used inside DocumentProvider')
  return context
}
