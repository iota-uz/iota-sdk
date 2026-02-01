/**
 * IOTA SDK integration context provider
 * Consumes server-side context from window.__BICHAT_CONTEXT__
 */

import { createContext, useContext, ReactNode } from 'react'
import type { IotaContext as IotaContextType } from '../types/iota'

const IotaContext = createContext<IotaContextType | null>(null)

interface IotaContextProviderProps {
  children: ReactNode
}

export function IotaContextProvider({ children }: IotaContextProviderProps) {
  // Read initial context from window object injected by server
  const initialContext = window.__BICHAT_CONTEXT__

  if (!initialContext) {
    throw new Error('BICHAT_CONTEXT not found. Ensure server injected context into window object.')
  }

  return (
    <IotaContext.Provider value={initialContext}>
      {children}
    </IotaContext.Provider>
  )
}

export function useIotaContext(): IotaContextType {
  const context = useContext(IotaContext)
  if (!context) {
    throw new Error('useIotaContext must be used within IotaContextProvider')
  }
  return context
}

/**
 * Check if user has a specific permission
 */
export function hasPermission(permission: string): boolean {
  const context = window.__BICHAT_CONTEXT__
  if (!context) {
    return false
  }
  return context.user.permissions.includes(permission)
}
