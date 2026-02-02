import { createContext, useContext, ReactNode } from 'react'
import type { InitialContext } from '../types'

/**
 * AppletContext provides access to the global context injected by the backend.
 * The context is read from window.__*_CONTEXT__ (configured via windowKey).
 */
const AppletContext = createContext<InitialContext | null>(null)

export interface AppletProviderProps {
  children: ReactNode
  windowKey: string
  context?: InitialContext
}

/**
 * AppletProvider reads context from window global and provides it to hooks.
 *
 * Usage:
 * <AppletProvider windowKey="__BICHAT_CONTEXT__">
 *   <App />
 * </AppletProvider>
 */
export function AppletProvider({ children, windowKey, context }: AppletProviderProps) {
  // Use provided context or read from window global
  const initialContext = context ?? (window as any)[windowKey]

  if (!initialContext) {
    throw new Error(`${windowKey} not found on window. Ensure backend context injection is working.`)
  }

  return (
    <AppletContext.Provider value={initialContext}>
      {children}
    </AppletContext.Provider>
  )
}

/**
 * useAppletContext provides access to the full applet context.
 * Use specialized hooks (useUser, useConfig, etc.) for specific context parts.
 */
export function useAppletContext<T = InitialContext>(): T {
  const context = useContext(AppletContext)
  if (!context) {
    throw new Error('useAppletContext must be used within AppletProvider')
  }
  return context as T
}
