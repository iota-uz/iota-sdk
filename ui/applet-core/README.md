# @iota-uz/applet-core

Core React package for building IOTA SDK applets. Provides context injection, hooks, and utilities for seamless integration with the IOTA SDK runtime.

## Installation

```bash
pnpm install @iota-uz/applet-core
```

## Quick Start

### Basic Setup

```tsx
import { AppletProvider } from '@iota-uz/applet-core'
import App from './App'

function Root() {
  return (
    <AppletProvider windowKey="__BICHAT_CONTEXT__">
      <App />
    </AppletProvider>
  )
}
```

### Using Hooks

```tsx
import {
  useUser,
  usePermissions,
  useTranslation,
  useSession,
  useRoute,
  useConfig
} from '@iota-uz/applet-core'

function App() {
  const { firstName, email } = useUser()
  const { hasPermission } = usePermissions()
  const { t } = useTranslation()
  const { isExpiringSoon, refreshSession } = useSession()
  const { path, params } = useRoute()
  const { graphQLEndpoint } = useConfig()

  // Check permissions
  if (!hasPermission('bichat.access')) {
    return <div>{t('Common.NoAccess')}</div>
  }

  // Auto-refresh expiring session
  if (isExpiringSoon) {
    refreshSession()
  }

  return (
    <div>
      <h1>{t('BiChat.Welcome', { name: firstName })}</h1>
      <p>{t('Common.CurrentPath')}: {path}</p>
    </div>
  )
}
```

## API Reference

### Context Providers

#### AppletProvider

Main context provider that reads context from window global.

```tsx
<AppletProvider windowKey="__BICHAT_CONTEXT__">
  <App />
</AppletProvider>
```

**Props:**
- `windowKey: string` - The window global key to read context from (e.g., `__BICHAT_CONTEXT__`)
- `context?: InitialContext` - Optional: provide context directly instead of reading from window
- `children: ReactNode` - React children

#### ConfigProvider

Alternative provider that accepts context via props (useful for testing/SSR).

```tsx
<ConfigProvider config={initialContext}>
  <App />
</ConfigProvider>
```

**Props:**
- `config: InitialContext` - The initial context object
- `children: ReactNode` - React children

### Core Hooks

#### useUser()

Access current user information.

```tsx
const { id, email, firstName, lastName, permissions } = useUser()
```

**Returns:**
```typescript
{
  id: number
  email: string
  firstName: string
  lastName: string
  permissions: string[]
}
```

#### usePermissions()

Permission checking utilities.

```tsx
const { hasPermission, hasAnyPermission, permissions } = usePermissions()

if (hasPermission('bichat.access')) {
  // User has bichat access
}

if (hasAnyPermission('finance.view', 'finance.edit')) {
  // User has at least one permission
}
```

**Returns:**
```typescript
{
  hasPermission: (permission: string) => boolean
  hasAnyPermission: (...permissions: string[]) => boolean
  permissions: string[]
}
```

#### useTranslation()

i18n translation with interpolation.

```tsx
const { t, language } = useTranslation()

// Simple translation
const title = t('BiChat.Title')

// Translation with params
const welcome = t('Common.Welcome', { name: 'John' })
// If translation is "Welcome {name}!" -> Returns "Welcome John!"
```

**Returns:**
```typescript
{
  t: (key: string, params?: Record<string, unknown>) => string
  language: string
}
```

**Translation Keys:**
React uses the same translation keys as Go backend:
- Go: `pageCtx.T("BiChat.Title")`
- React: `t("BiChat.Title")`

All translations from the locale bundle are automatically available.

#### useSession()

Session and authentication handling.

```tsx
const { isExpiringSoon, refreshSession, csrfToken, expiresAt } = useSession()

// Check if session expires soon
if (isExpiringSoon) {
  await refreshSession()
}

// Include CSRF token in requests
fetch('/api/endpoint', {
  headers: { 'X-CSRF-Token': csrfToken }
})
```

**Returns:**
```typescript
{
  isExpiringSoon: boolean        // True if session expires in < 5 minutes
  refreshSession: () => Promise<void>
  csrfToken: string
  expiresAt: number             // Unix timestamp
}
```

**CSRF Token Refresh:**
Listen for token refresh events:
```tsx
window.addEventListener('iota:csrf-refresh', (e) => {
  const newToken = e.detail.token
  // Update CSRF token
})
```

#### useRoute()

Access current route context.

```tsx
const { path, params, query } = useRoute()

// Example values:
// path: "/sessions/123"
// params: { id: "123" }
// query: { tab: "history" }
```

**Returns:**
```typescript
{
  path: string
  params: Record<string, string>
  query: Record<string, string>
}
```

#### useConfig()

Access applet configuration.

```tsx
const { graphQLEndpoint, streamEndpoint, restEndpoint } = useConfig()
```

**Returns:**
```typescript
{
  graphQLEndpoint?: string
  streamEndpoint?: string
  restEndpoint?: string
}
```

#### useStreaming()

SSE streaming with cancellation support.

```tsx
const { isStreaming, processStream, cancel, reset } = useStreaming()

// Process async generator stream
await processStream(messageStream, (chunk) => {
  console.log('Received:', chunk)
})

// Cancel ongoing stream
cancel()

// Reset state
reset()
```

**Returns:**
```typescript
{
  isStreaming: boolean
  processStream: <T>(
    generator: AsyncGenerator<T>,
    onChunk: (chunk: T) => void,
    signal?: AbortSignal
  ) => Promise<void>
  cancel: () => void
  reset: () => void
}
```

## TypeScript Types

All types are exported from the main entry point:

```tsx
import type {
  InitialContext,
  UserContext,
  TenantContext,
  LocaleContext,
  AppConfig,
  RouteContext,
  SessionContext,
  TranslationHook,
  PermissionsHook,
  SessionHook,
  StreamingHook
} from '@iota-uz/applet-core'
```

### InitialContext

Complete context object injected by backend:

```typescript
interface InitialContext {
  user: UserContext
  tenant: TenantContext
  locale: LocaleContext
  config: AppConfig
  route: RouteContext
  session: SessionContext
  custom?: Record<string, unknown>
}
```

## Advanced Usage

### Direct Window Global Access

For cases where provider setup is not possible:

```tsx
import { useAppletContextDirect } from '@iota-uz/applet-core'

const context = useAppletContextDirect('__BICHAT_CONTEXT__')
```

### Custom Context Fields

Access custom context fields via the `custom` property:

```tsx
const { custom } = useAppletContext()
const customData = custom?.myCustomField
```

### Server-Side Rendering (Next.js)

Use `ConfigProvider` for SSR:

```tsx
// pages/index.tsx
export async function getServerSideProps(context) {
  const initialContext = context.req.__APPLET_CONTEXT__

  return {
    props: { initialContext }
  }
}

export default function Page({ initialContext }) {
  return (
    <ConfigProvider config={initialContext}>
      <App />
    </ConfigProvider>
  )
}
```

## Pattern Alignment with Go Backend

Applet-core maintains consistency with Go backend patterns:

**Permissions:**
```go
// Go backend
sdkcomposables.CanUser(ctx, permissions.BiChatAccess)
```
```tsx
// React frontend
hasPermission('bichat.access')
```

**Translations:**
```go
// Go backend
pageCtx.T("BiChat.Title")
```
```tsx
// React frontend
t("BiChat.Title")
```

All user permissions and translations are automatically passed from backend - no manual mapping required.

## Best Practices

1. **Use specialized hooks** instead of `useAppletContext()` for better type safety
2. **Check permissions early** in component lifecycle
3. **Handle session expiration** proactively with `useSession().isExpiringSoon`
4. **Use same translation keys** as Go backend for consistency
5. **Include CSRF tokens** in all mutating requests
6. **Cancel streams** on component unmount to prevent memory leaks

## Development

```bash
# Install dependencies
pnpm install

# Build package
pnpm run build

# Type check
pnpm run typecheck

# Lint
pnpm run lint

# Development mode (watch)
pnpm run dev
```

## License

MIT
