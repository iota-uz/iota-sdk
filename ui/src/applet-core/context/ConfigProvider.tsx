import { createContext, useContext, ReactNode } from 'react'
import type { InitialContext } from '../types'

/**
 * ConfigProvider is an alternative to AppletProvider that accepts context via props
 * instead of reading from window global. Useful for testing and server-side rendering.
 */

const ConfigContext = createContext<InitialContext | null>(null)

export interface ConfigProviderProps {
  children: ReactNode
  config: InitialContext
}

/**
 * ConfigProvider accepts context configuration via props.
 *
 * Usage:
 * <ConfigProvider config={initialContext}>
 *   <App />
 * </ConfigProvider>
 */
export function ConfigProvider({ children, config }: ConfigProviderProps) {
  return (
    <ConfigContext.Provider value={config}>
      {children}
    </ConfigContext.Provider>
  )
}

/**
 * useConfigContext provides access to the applet context when using ConfigProvider.
 */
export function useConfigContext<T = InitialContext>(): T {
  const context = useContext(ConfigContext)
  if (!context) {
    throw new Error('useConfigContext must be used within ConfigProvider')
  }
  return context as T
}
