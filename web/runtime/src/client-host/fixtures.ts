// Test fixtures shared by the runtime host tests.
import { CLIENT_BOOTSTRAP_VERSION, SDK_IDENTITY } from '@iota-uz/sdk/client-host'
import type { PageData, RuntimeContext } from './protocol.ts'

export function pageData(overrides: Partial<PageData> = {}): PageData {
  return {
    kind: 'iota.runtime.page',
    page: { id: 'acme.insurance.policy', package: '@acme/insurance', version: 'v1', module: '/_ext/assets/h1h1h1h1/client/policy.js', export: 'policy' },
    params: { id: '7', flag: 'true', ref: 'abc' },
    paramTypes: {
      kind: 'object',
      properties: [
        { name: 'id', type: { kind: 'int' } },
        { name: 'flag', type: { kind: 'bool' } },
        { name: 'ref', type: { kind: 'uuid' } },
      ],
    },
    ...overrides,
  }
}

/** Writes the server-rendered document of a runtime page. */
export function pageDocument(data: PageData = pageData(), extra: Record<string, unknown> = {}): void {
  document.documentElement.removeAttribute('data-theme')
  document.body.innerHTML = `
    <div data-iota-shell-standard><nav>standard nav</nav>
      <div data-iota-shell-outlet>
        <main id="iota-client-route" data-client-route-background><div id="iota-client-route-mount"></div></main>
        <div id="iota-client-route-portals"></div>
      </div>
    </div>`
  const script = document.createElement('script')
  script.id = 'iota-client-context'
  script.type = 'application/json'
  script.textContent = JSON.stringify({
    bootstrapVersion: CLIENT_BOOTSTRAP_VERSION,
    protocolVersion: SDK_IDENTITY.protocolVersion,
    sdkReleaseVersion: SDK_IDENTITY.releaseVersion,
    sdkCommit: SDK_IDENTITY.sourceCommit,
    initial: data,
    theme: 'light',
    route: { id: data.page.id, path: '/insurance/policies/7', featureId: data.page.id },
    locale: { language: 'ru', messages: {} },
    permissions: ['Policy.Read'],
    services: { rpc: '/_ext/rpc' },
    ...extra,
  })
  document.body.append(script)
}

export function runtimeContext(overrides: Partial<RuntimeContext> = {}): RuntimeContext {
  return {
    version: '1.0.0',
    protocolVersion: SDK_IDENTITY.protocolVersion,
    sdkReleaseVersion: SDK_IDENTITY.releaseVersion,
    sdkCommit: SDK_IDENTITY.sourceCommit,
    rpc: '/_ext/rpc',
    locale: 'ru',
    permissions: ['Policy.Read'],
    user: { id: '5', name: 'Ann Doe', email: 'ann@example.com' },
    shell: { id: 'acme.shell.main', package: '@acme/shell', version: 'v2', module: '/_ext/assets/5he115he/client/shell.js', export: 'shell', binding: 'b-1' },
    navigation: {
      workspaces: [{ id: 'sales', label: 'Sales', order: 2 }, { id: 'ops', label: 'Operations', order: 1 }],
      palette: [{ id: 'p1', label: 'Go to policies', href: '/insurance/policies', keywords: ['insurance'] }],
    },
    ...overrides,
  }
}

/** Writes a legacy Templ page: standard shell, outlet with page content, shell context. */
export function legacyDocument(runtime: RuntimeContext | null, navigation: unknown[] = []): void {
  document.documentElement.removeAttribute('data-theme')
  document.body.innerHTML = `
    <script id="iota-shell-context" type="application/json">${JSON.stringify({ version: '1.0.0', navigation, user: { name: 'Ann Doe', email: 'ann@example.com' } })}</script>
    ${runtime ? `<script id="iota-runtime-context" type="application/json">${JSON.stringify(runtime)}</script>` : ''}
    <div data-iota-shell-standard id="standard"><nav>standard nav</nav>
      <div id="content-parent"><div data-iota-shell-outlet id="outlet"><div id="legacy-body" hx-get="/x">legacy page</div></div></div>
    </div>`
}

export const tick = () => new Promise<void>((resolve) => setTimeout(resolve, 0))

export function deferred<T>() {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}
