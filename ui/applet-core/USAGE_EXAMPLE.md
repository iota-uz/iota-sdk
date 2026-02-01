# Usage Example

This file demonstrates how to use `@iota-uz/applet-core` in a React applet.

## Installation

```bash
npm install @iota-uz/applet-core
```

## Basic Setup

```tsx
// App.tsx
import { AppletProvider } from '@iota-uz/applet-core'
import ChatInterface from './ChatInterface'

export default function App() {
  return (
    <AppletProvider windowKey="__BICHAT_CONTEXT__">
      <ChatInterface />
    </AppletProvider>
  )
}
```

## Using Hooks in Components

```tsx
// ChatInterface.tsx
import {
  useUser,
  usePermissions,
  useTranslation,
  useSession,
  useConfig,
  useRoute
} from '@iota-uz/applet-core'

export default function ChatInterface() {
  // Access user info
  const { firstName, lastName, email } = useUser()

  // Check permissions
  const { hasPermission, hasAnyPermission } = usePermissions()

  // Access translations
  const { t, language } = useTranslation()

  // Handle session
  const { isExpiringSoon, refreshSession, csrfToken } = useSession()

  // Get config
  const { graphQLEndpoint, streamEndpoint } = useConfig()

  // Access route
  const { path, params, query } = useRoute()

  // Permission check
  if (!hasPermission('bichat.access')) {
    return <div>{t('Common.NoAccess')}</div>
  }

  // Auto-refresh session
  if (isExpiringSoon) {
    refreshSession().catch(console.error)
  }

  return (
    <div>
      <h1>{t('BiChat.Welcome', { name: firstName })}</h1>
      <p>{t('Common.Language')}: {language}</p>
      <p>{t('Common.CurrentPath')}: {path}</p>

      {/* Use CSRF token in API calls */}
      <button onClick={() => {
        fetch(graphQLEndpoint, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'X-CSRF-Token': csrfToken
          },
          body: JSON.stringify({ query: '...' })
        })
      }}>
        {t('BiChat.SendMessage')}
      </button>
    </div>
  )
}
```

## Using SSE Streaming

```tsx
// MessageStream.tsx
import { useState } from 'react'
import { useStreaming, useSession } from '@iota-uz/applet-core'

async function* createMessageStream(url: string, csrfToken: string) {
  const response = await fetch(url, {
    headers: { 'X-CSRF-Token': csrfToken }
  })

  const reader = response.body?.getReader()
  if (!reader) return

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    yield new TextDecoder().decode(value)
  }
}

export default function MessageStream() {
  const [content, setContent] = useState('')
  const { isStreaming, processStream, cancel } = useStreaming()
  const { csrfToken } = useSession()

  const handleStream = async () => {
    const stream = createMessageStream('/bichat/stream', csrfToken)

    await processStream(stream, (chunk) => {
      setContent(prev => prev + chunk)
    })
  }

  return (
    <div>
      <button onClick={handleStream} disabled={isStreaming}>
        Start Stream
      </button>

      {isStreaming && (
        <button onClick={cancel}>Cancel</button>
      )}

      <pre>{content}</pre>
    </div>
  )
}
```

## Testing with ConfigProvider

```tsx
// App.test.tsx
import { render, screen } from '@testing-library/react'
import { ConfigProvider } from '@iota-uz/applet-core'
import type { InitialContext } from '@iota-uz/applet-core'
import App from './App'

const mockContext: InitialContext = {
  user: {
    id: 1,
    email: 'test@example.com',
    firstName: 'John',
    lastName: 'Doe',
    permissions: ['bichat.access', 'bichat.create']
  },
  tenant: {
    id: 'tenant-1',
    name: 'Test Tenant'
  },
  locale: {
    language: 'en',
    translations: {
      'BiChat.Welcome': 'Welcome {name}!',
      'Common.NoAccess': 'Access Denied'
    }
  },
  config: {
    graphQLEndpoint: '/graphql',
    streamEndpoint: '/stream'
  },
  route: {
    path: '/sessions/123',
    params: { id: '123' },
    query: { tab: 'history' }
  },
  session: {
    expiresAt: Date.now() + 3600000,
    refreshURL: '/auth/refresh',
    csrfToken: 'test-csrf-token'
  }
}

test('renders app with context', () => {
  render(
    <ConfigProvider config={mockContext}>
      <App />
    </ConfigProvider>
  )

  expect(screen.getByText(/Welcome John!/)).toBeInTheDocument()
})
```

## Type Safety

All hooks return strongly-typed values:

```tsx
import type {
  UserContext,
  PermissionsHook,
  TranslationHook,
  SessionHook,
  RouteContext,
  AppConfig
} from '@iota-uz/applet-core'

// Use types for custom logic
function buildUserDisplayName(user: UserContext): string {
  return `${user.firstName} ${user.lastName}`
}

function checkAdminAccess(permissions: PermissionsHook): boolean {
  return permissions.hasAnyPermission('admin.read', 'admin.write')
}
```

## Pattern Alignment with Go Backend

The package maintains consistency with Go backend patterns:

### Permissions
```go
// Go backend
sdkcomposables.CanUser(ctx, permissions.BiChatAccess)
```

```tsx
// React frontend
hasPermission('bichat.access')
```

### Translations
```go
// Go backend
pageCtx.T("BiChat.Title")
pageCtx.T("BiChat.Welcome", map[string]interface{}{"name": user.FirstName})
```

```tsx
// React frontend
t("BiChat.Title")
t("BiChat.Welcome", { name: firstName })
```

All user permissions and translations are automatically passed from the backend - no manual mapping required.
