import { createContext, useContext, type Accessor, type JSX } from 'solid-js'
import { createSignal } from 'solid-js'
import { readAppletContext, type SolidAppletContext } from '../applet/solidAppletElement'

export interface LLMExtensions {
  llm?: { provider?: string; apiKeyConfigured?: boolean }
  debug?: unknown
}

export interface ResolvedAppContext {
  config: SolidAppletContext['config']
  session: { csrfToken?: string }
  user: { id?: number; email?: string; firstName?: string; lastName?: string; permissions?: string[] }
  tenant: { id?: string; name?: string }
  locale: { language: string; translations: Record<string, string> }
  extensions: LLMExtensions
}

function resolveContext(): ResolvedAppContext {
  const raw = readAppletContext() ?? ({} as SolidAppletContext)
  return {
    config: raw.config ?? { basePath: '', rpcUIEndpoint: '/rpc' },
    session: raw.session ?? {},
    user: (raw.user as ResolvedAppContext['user']) ?? {},
    tenant: (raw.tenant as ResolvedAppContext['tenant']) ?? {},
    locale: {
      language: raw.locale?.language ?? 'en',
      translations: raw.locale?.translations ?? {},
    },
    extensions: (raw.extensions as LLMExtensions) ?? {},
  }
}

const AppContext = createContext<ResolvedAppContext>()

export function resolveAppContext(): ResolvedAppContext {
  return resolveContext()
}

export function AppProvider(props: { children: JSX.Element }): JSX.Element {
  return <AppContext.Provider value={resolveContext()}>{props.children}</AppContext.Provider>
}

export function useAppContext(): ResolvedAppContext {
  const ctx = useContext(AppContext)
  if (!ctx) throw new Error('useAppContext must be used within AppProvider')
  return ctx
}

export function createTouchDevice(): Accessor<boolean> {
  const [isTouch, setIsTouch] = createSignal(
    typeof window !== 'undefined' &&
      (window.navigator.maxTouchPoints > 0 || 'ontouchstart' in window),
  )
  if (typeof window !== 'undefined' && window.matchMedia) {
    const query = window.matchMedia('(pointer: coarse)')
    const listener = (event: MediaQueryListEvent) => setIsTouch(event.matches)
    query.addEventListener('change', listener)
    return isTouch
  }
  return isTouch
}

const TouchContext = createContext<Accessor<boolean>>(() => false)

export function TouchProvider(props: { children: JSX.Element }): JSX.Element {
  const isTouch = createTouchDevice()
  return <TouchContext.Provider value={isTouch}>{props.children}</TouchContext.Provider>
}

export function useTouchDevice(): Accessor<boolean> {
  return useContext(TouchContext)
}
