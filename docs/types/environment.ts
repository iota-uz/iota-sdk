export type Environment = 'production' | 'staging' | 'pre-production'

export interface EnvironmentUrls {
  erp: string
  website: string
}

export interface EnvironmentContextType {
  environment: Environment
  setEnvironment: (env: Environment) => void
  getUrl: (path: string, type?: 'erp' | 'website' | 'auto') => string
}

export const ENV_URLS: Record<Environment, EnvironmentUrls> = {
  production: {
    erp: 'https://erp.eai.uz',
    website: 'https://eai.uz'
  },
  staging: {
    erp: 'https://erp-staging.eai.uz',
    website: 'https://eai-staging.uz'
  },
  'pre-production': {
    erp: '',
    website: ''
  }
}
