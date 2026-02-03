/**
 * @iota-uz/iota-sdk - Applet Core for IOTA SDK applets
 *
 * Provides context injection, hooks, and utilities for building React applets
 * that integrate seamlessly with the IOTA SDK runtime.
 */

// Context providers
export { AppletProvider, useAppletContext } from './context/AppletContext'
export { ConfigProvider, useConfigContext } from './context/ConfigProvider'

// Hooks
export { useAppletContext as useAppletContextDirect } from './hooks/useAppletContext'
export { useConfig } from './hooks/useConfig'
export { useUser } from './hooks/useUser'
export { usePermissions } from './hooks/usePermissions'
export { useTranslation } from './hooks/useTranslation'
export { useSession } from './hooks/useSession'
export { useRoute } from './hooks/useRoute'
export { useStreaming } from './hooks/useStreaming'

// Types
export type {
  InitialContext,
  UserContext,
  TenantContext,
  LocaleContext,
  AppConfig,
  RouteContext,
  SessionContext,
  TranslationHook,
  PermissionsHook,
  SessionHook,
  StreamingHook
} from './types'
