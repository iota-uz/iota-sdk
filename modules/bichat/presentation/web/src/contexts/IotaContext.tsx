import { createContext, useContext, ReactNode } from 'react'
import type { IotaContext } from '@iota-uz/bichat-ui'

/**
 * Extended BiChat context with feature flags
 */
export interface BiChatContext extends IotaContext {
  extensions: {
    features: {
      vision: boolean
      webSearch: boolean
      codeInterpreter: boolean
    }
  }
}

const IotaContextInstance = createContext<BiChatContext | null>(null)

export function IotaContextProvider({ children }: { children: ReactNode }) {
  const context = ((window as any).IOTA_CONTEXT || (window as any).__BICHAT_CONTEXT__) as BiChatContext

  if (!context) {
    throw new Error('BiChat context not found. Ensure the app is served via Go backend.')
  }

  return <IotaContextInstance.Provider value={context}>{children}</IotaContextInstance.Provider>
}

export function useIotaContext() {
  const context = useContext(IotaContextInstance)
  if (!context) throw new Error('useIotaContext must be used within IotaContextProvider')
  return context
}

export function hasPermission(permission: string): boolean {
  const { user } = useIotaContext()
  return user.permissions.includes(permission)
}
