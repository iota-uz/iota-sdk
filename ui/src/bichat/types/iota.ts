/**
 * BiChat context types layered on top of canonical applet-core context contracts.
 */

import type {
  AppConfig as AppletAppConfig,
  InitialContext as AppletInitialContext,
  LocaleContext as AppletLocaleContext,
  TenantContext as AppletTenantContext,
  UserContext as AppletUserContext,
} from '../../applet-core/types'

export type UserContext = AppletUserContext
export type TenantContext = AppletTenantContext
export type LocaleContext = AppletLocaleContext

export type AppConfig = AppletAppConfig & {
  streamEndpoint: string
  basePath: string
  assetsBasePath: string
  rpcUIEndpoint: string
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
  llm?: {
    provider?: string
    apiKeyConfigured?: boolean
  }
  debug?: {
    limits?: {
      policyMaxTokens: number
      modelMaxTokens: number
      effectiveMaxTokens: number
      completionReserveTokens: number
    }
  }
}

export type IotaContext = Omit<AppletInitialContext, 'config' | 'extensions'> & {
  config: AppConfig
  extensions?: Extensions
}

declare global {
  interface Window {
    __BICHAT_CONTEXT__: IotaContext
    __CSRF_TOKEN__: string
  }
}
