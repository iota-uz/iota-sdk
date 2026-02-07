/**
 * TypeScript type definitions for IOTA SDK Applet Core
 * Matches Go backend types from pkg/applet/types.go
 */

export interface InitialContext {
  user: UserContext
  tenant: TenantContext
  locale: LocaleContext
  config: AppConfig
  route: RouteContext
  session: SessionContext
  error: ErrorContext | null
  extensions?: Record<string, unknown>
}

export interface UserContext {
  id: number
  email: string
  firstName: string
  lastName: string
  permissions: string[]
}

export interface TenantContext {
  id: string
  name: string
}

export interface LocaleContext {
  language: string
  translations: Record<string, string>
}

export interface AppConfig {
  graphQLEndpoint?: string
  streamEndpoint?: string
  restEndpoint?: string
  basePath?: string
  assetsBasePath?: string
  rpcUIEndpoint?: string
  shellMode?: 'embedded' | 'standalone'
}

export interface RouteContext {
  path: string
  params: Record<string, string>
  query: Record<string, string>
}

export interface SessionContext {
  expiresAt: number
  refreshURL: string
  csrfToken: string
}

export interface ErrorContext {
  supportEmail?: string
  debugMode: boolean
  errorCodes?: Record<string, string>
  retryConfig?: RetryConfig
}

export interface RetryConfig {
  maxAttempts: number
  backoffMs: number
}

/**
 * Hook return types
 */

export interface TranslationHook {
  t: (key: string, params?: Record<string, unknown>) => string
  language: string
}

export interface PermissionsHook {
  hasPermission: (permission: string) => boolean
  hasAnyPermission: (...permissions: string[]) => boolean
  permissions: string[]
}

export interface SessionHook {
  isExpiringSoon: boolean
  refreshSession: () => Promise<void>
  csrfToken: string
  expiresAt: number
}

export interface StreamingHook {
  isStreaming: boolean
  processStream: <T>(
    generator: AsyncGenerator<T>,
    onChunk: (chunk: T) => void,
    signal?: AbortSignal
  ) => Promise<void>
  cancel: () => void
  reset: () => void
}
