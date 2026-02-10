export function injectMockContext(): void {
  if (import.meta.env.DEV && !(window as any).__BICHAT_CONTEXT__) {
    ;(window as any).__BICHAT_CONTEXT__ = {
      user: {
        id: 1,
        email: 'dev@example.com',
        firstName: 'Dev',
        lastName: 'User',
        permissions: ['BiChat.Access'],
      },
      tenant: {
        id: '00000000-0000-0000-0000-000000000000',
        name: 'Dev Tenant',
      },
      locale: {
        language: 'en',
        translations: {},
      },
      config: {
        streamEndpoint: '/bi-chat/stream',
        basePath: '',
        assetsBasePath: '/bi-chat/assets',
        rpcUIEndpoint: '/bi-chat/rpc',
        shellMode: 'standalone',
      },
      route: {
        path: '/',
        params: {},
        query: {},
      },
      session: {
        expiresAt: Date.now() + 24 * 60 * 60 * 1000,
        refreshURL: '/auth/refresh',
        csrfToken: 'dev-csrf-token',
      },
      error: null,
      extensions: {
        llm: {
          provider: 'openai',
          apiKeyConfigured: true,
        },
        debug: {
          limits: {
            policyMaxTokens: 180000,
            modelMaxTokens: 272000,
            effectiveMaxTokens: 180000,
            completionReserveTokens: 8000,
          },
        },
      },
    }
  }
}
