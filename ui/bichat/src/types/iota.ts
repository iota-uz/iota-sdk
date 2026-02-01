/**
 * Type definitions matching Go structs for server-side context
 */

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

export interface IotaContext {
  user: UserContext
  tenant: TenantContext
  locale: LocaleContext
  config: AppConfig
}

declare global {
  interface Window {
    __BICHAT_CONTEXT__: IotaContext
    __CSRF_TOKEN__: string
  }
}
