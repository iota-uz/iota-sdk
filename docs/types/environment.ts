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
    erp: 'https://erp.example.com',
    website: 'https://example.com'
  },
  staging: {
    erp: 'https://erp-staging.example.com',
    website: 'https://staging.example.com'
  },
  'pre-production': {
    erp: '',
    website: ''
  }
}
