export function injectMockContext(): void {
  if (import.meta.env.DEV && !(window as any).IOTA_CONTEXT) {
    (window as any).IOTA_CONTEXT = {
      user: {
        id: 1,
        name: 'Dev User',
        email: 'dev@example.com',
      },
      tenant: {
        id: '00000000-0000-0000-0000-000000000000',
        name: 'Dev Tenant',
      },
      config: {
        locale: 'en',
        graphQLEndpoint: '/bi-chat/graphql',
        streamEndpoint: '/bi-chat/stream',
      },
      session: null,
      extensions: {
        features: {
          vision: true,
          webSearch: true,
          codeInterpreter: true,
          multiAgent: false,
        },
      },
    }
  }
}
