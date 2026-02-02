/**
 * Type definitions matching Go structs for server-side context
 */

import type { BrandingConfig, FeatureFlags, Translations } from './index'

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
  graphQLEndpoint: string
  streamEndpoint: string
}

/**
 * Extensions injected by BiChat module for customization.
 */
export interface ContextExtensions {
  /** Feature flags for capabilities */
  features: FeatureFlags
  /** Branding configuration for UI customization */
  branding: BrandingConfig
  /** Translations for the current locale */
  translations: Translations
}

export interface IotaContext {
  user: UserContext
  tenant: TenantContext
  locale: LocaleContext
  config: AppConfig
  /** BiChat-specific extensions for customization */
  extensions?: ContextExtensions
}

declare global {
  interface Window {
    __BICHAT_CONTEXT__: IotaContext
    __CSRF_TOKEN__: string
  }
}
