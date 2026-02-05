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
  streamEndpoint: string
  basePath: string
  assetsBasePath: string
  rpcUIEndpoint: string
  shellMode?: string
}

export interface Extensions {
  branding?: {
    appName?: string
    logoUrl?: string
    theme?: {
      primary?: string
      secondary?: string
      accent?: string
    }
    welcome?: {
      title?: string
      description?: string
      examplePrompts?: Array<{
        category: string
        text: string
        icon: string
      }>
    }
    colors?: {
      primary?: string
      secondary?: string
      accent?: string
    }
    logo?: {
      src?: string
      alt?: string
    }
  }
  features?: {
    vision?: boolean
    webSearch?: boolean
    codeInterpreter?: boolean
    multiAgent?: boolean
  }
}

export interface IotaContext {
  user: UserContext
  tenant: TenantContext
  locale: LocaleContext
  config: AppConfig
  extensions?: Extensions
}

declare global {
  interface Window {
    __BICHAT_CONTEXT__: IotaContext
    __CSRF_TOKEN__: string
  }
}
