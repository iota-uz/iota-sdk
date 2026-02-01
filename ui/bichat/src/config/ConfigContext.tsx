/**
 * Configuration context provider for BiChat
 * Provides props-based configuration with optional window global fallback
 */

import { createContext, useContext, ReactNode } from 'react'

export interface BiChatConfig {
  user: {
    id: string
    email: string
    firstName: string
    lastName: string
    permissions: string[]
  }
  tenant: {
    id: string
    name: string
  }
  locale: {
    language: string
    translations: Record<string, string>
  }
  endpoints: {
    graphQL: string
    stream: string
  }
  csrfToken?: string
}

const ConfigContext = createContext<BiChatConfig | null>(null)

interface ConfigProviderProps {
  config?: BiChatConfig
  useGlobalConfig?: boolean
  children: ReactNode
}

/**
 * ConfigProvider component
 * Provides configuration to the BiChat library
 *
 * @param config - Configuration object (preferred method)
 * @param useGlobalConfig - If true, falls back to window.__BICHAT_CONTEXT__ when config is not provided
 * @param children - React children
 */
export function ConfigProvider({ config, useGlobalConfig = false, children }: ConfigProviderProps) {
  let resolvedConfig: BiChatConfig | null = null

  if (config) {
    resolvedConfig = config
  } else if (useGlobalConfig && typeof window !== 'undefined') {
    const globalContext = (window as any).__BICHAT_CONTEXT__
    const globalCSRF = (window as any).__CSRF_TOKEN__

    if (globalContext) {
      resolvedConfig = {
        user: {
          id: String(globalContext.user?.id || ''),
          email: globalContext.user?.email || '',
          firstName: globalContext.user?.firstName || '',
          lastName: globalContext.user?.lastName || '',
          permissions: globalContext.user?.permissions || [],
        },
        tenant: {
          id: globalContext.tenant?.id || '',
          name: globalContext.tenant?.name || '',
        },
        locale: {
          language: globalContext.locale?.language || 'en',
          translations: globalContext.locale?.translations || {},
        },
        endpoints: {
          graphQL: globalContext.config?.graphQLEndpoint || '/graphql',
          stream: globalContext.config?.streamEndpoint || '/stream',
        },
        csrfToken: globalCSRF,
      }
    }
  }

  return (
    <ConfigContext.Provider value={resolvedConfig}>
      {children}
    </ConfigContext.Provider>
  )
}

/**
 * Hook to access BiChat configuration
 * Returns null if no configuration is available
 */
export function useConfig(): BiChatConfig | null {
  return useContext(ConfigContext)
}

/**
 * Hook to access BiChat configuration (required)
 * Throws an error if configuration is not available
 */
export function useRequiredConfig(): BiChatConfig {
  const config = useContext(ConfigContext)
  if (!config) {
    throw new Error(
      'BiChat configuration not found. ' +
      'Wrap your app with <ConfigProvider config={...}> or use useGlobalConfig={true}.'
    )
  }
  return config
}

/**
 * Check if user has a specific permission
 */
export function hasPermission(config: BiChatConfig | null, permission: string): boolean {
  if (!config) {
    return false
  }
  return config.user.permissions.includes(permission)
}
