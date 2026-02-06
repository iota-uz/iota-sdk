import type { Preview } from '@storybook/react'


import './preview.generated.css'

import { ThemeProvider } from '../src/bichat/theme/ThemeProvider'
import { IotaContextProvider } from '../src/bichat/context/IotaContext'

import type { IotaContext } from '../src/bichat/types/iota'
import type { InitialContext } from '../src/applet-core/types'

type ThemeMode = 'light' | 'dark'

function applyThemeMode(mode: ThemeMode) {
  const root = document.documentElement
  if (mode === 'dark') root.classList.add('dark')
  else root.classList.remove('dark')
}

function seedGlobals() {
  // Used by BiChat (`IotaContextProvider`) and shared CSRF utilities.
  const bichatContext: IotaContext = {
    user: {
      id: 1,
      email: 'storybook@example.com',
      firstName: 'Story',
      lastName: 'Book',
      permissions: ['*'],
    },
    tenant: { id: 'tenant-1', name: 'Storybook Tenant' },
    locale: {
      language: 'en',
      translations: {},
    },
    config: {
      streamEndpoint: '/stream',
      basePath: '/bi-chat',
      assetsBasePath: '/bi-chat/assets',
      rpcUIEndpoint: '/bi-chat/rpc',
    },
    extensions: {
      branding: {
        appName: 'BiChat (Storybook)',
      },
      features: {
        vision: true,
        webSearch: true,
        codeInterpreter: true,
        multiAgent: true,
      },
    },
  }

  window.__BICHAT_CONTEXT__ = bichatContext
  window.__CSRF_TOKEN__ = 'storybook-csrf-token'

  // Used by applet-core `AppletProvider` (windowKey-driven).
  const appletContext: InitialContext = {
    user: bichatContext.user,
    tenant: bichatContext.tenant,
    locale: bichatContext.locale,
    config: {
      graphQLEndpoint: '/graphql',
      streamEndpoint: bichatContext.config.streamEndpoint,
      restEndpoint: '/api',
      basePath: bichatContext.config.basePath,
      assetsBasePath: bichatContext.config.assetsBasePath,
      rpcUIEndpoint: bichatContext.config.rpcUIEndpoint,
    },
    route: {
      path: '/storybook',
      params: {},
      query: {},
    },
    session: {
      expiresAt: Date.now() + 1000 * 60 * 60,
      refreshURL: '/auth/refresh',
      csrfToken: window.__CSRF_TOKEN__,
    },
    error: null,
    extensions: {},
  }

  ;(window as any).__APPLET_CONTEXT__ = appletContext
}

const preview: Preview = {
  parameters: {
    actions: { argTypesRegex: '^on[A-Z].*' },
    controls: {
      matchers: {
        color: /(background|color)$/i,
        date: /Date$/i,
      },
    },
  },
  globalTypes: {
    theme: {
      description: 'Theme mode',
      defaultValue: 'light',
      toolbar: {
        title: 'Theme',
        items: [
          { value: 'light', title: 'Light' },
          { value: 'dark', title: 'Dark' },
        ],
      },
    },
  },
  decorators: [
    (Story, ctx) => {
      seedGlobals()
      applyThemeMode((ctx.globals.theme as ThemeMode) || 'light')

      return (
        <ThemeProvider theme={ctx.globals.theme as ThemeMode}>
          <IotaContextProvider>
            <div className="min-h-screen bg-gray-50 dark:bg-gray-900 text-gray-900 dark:text-gray-100">
              <Story />
            </div>
          </IotaContextProvider>
        </ThemeProvider>
      )
    },
  ],
}

export default preview
