// Entry module of the runtime host bundle: the script that pkg/extruntime/ui
// renders on runtime pages and on authenticated pages with a bound shell. It
// wires the canonical client host (session, CSRF, portals, theme, shell
// registry, ShellHost) to the page and shell loaders; it does not create a
// second host.
import {
  assertSDKIdentity,
  createClientHostServices,
  documentShellRegistry,
  mountSolidClientRoute,
  readClientBootstrap,
  ShellHost,
  type ClientRouteContext,
} from '@iota-uz/sdk/client-host'
import { documentPageData, documentTheme, mountRuntimePage, type RouteHost } from './page.ts'
import { RPC_PATH, readRuntimeContext } from './protocol.ts'
import { publishSharedModules } from './shared.ts'
import { activateRuntimeShell } from './shell.ts'
import { createRpcTransport } from './transport.ts'

export interface Boot {
  /** Unmounts the page and deactivates the shell. */
  dispose(): void
  /** Resolves when the shell activation attempt finished (true = active). */
  shell: Promise<boolean>
}

function submitLogout(win: Window & typeof globalThis): void {
  const doc = win.document
  const form = doc.createElement('form')
  form.method = 'post'
  form.action = '/logout'
  doc.body.append(form)
  form.submit()
}

/** Starts whatever the document asks for. Safe to call on any authenticated page. */
export function bootRuntime(
  win: Window & typeof globalThis = window,
  deps: { importModule?: (url: string) => Promise<Record<string, unknown>> } = {},
): Boot {
  const doc = win.document
  const disposers: (() => void)[] = []
  publishSharedModules(win)
  let shell: Promise<boolean> = Promise.resolve(false)

  if (documentPageData(doc)) {
    const context = readClientBootstrap(doc)
    context.theme = documentTheme(doc, context.theme)
    const services = createClientHostServices(context, win)
    const transport = createRpcTransport({
      endpoint: context.services?.rpc ?? RPC_PATH,
      session: services.session,
      ...(context.session?.reauthUrl ? { reauthUrl: context.session.reauthUrl } : {}),
    })
    const host: RouteHost<ClientRouteContext<unknown>> = {
      readContext: () => context,
      mount: (options) => mountSolidClientRoute({ ...options, services }),
    }
    disposers.push(mountRuntimePage({ document: doc, host, transport, ...(deps.importModule ? { importModule: deps.importModule } : {}) }))
  }

  const runtime = readRuntimeContext(doc)
  if (runtime?.shell) {
    assertSDKIdentity({ releaseVersion: runtime.sdkReleaseVersion, sourceCommit: runtime.sdkCommit, protocolVersion: runtime.protocolVersion })
    const registry = documentShellRegistry(doc)
    const services = createClientHostServices({
      bootstrapVersion: '1.0.0',
      protocolVersion: runtime.protocolVersion,
      sdkReleaseVersion: runtime.sdkReleaseVersion,
      sdkCommit: runtime.sdkCommit,
      initial: null,
      theme: documentTheme(doc, 'light'),
      locale: { language: runtime.locale, messages: {} },
      permissions: runtime.permissions,
    }, win)
    const shellHost = new ShellHost(doc, {
      registry,
      theme: services.theme,
      user: runtime.user ? { name: runtime.user.name, email: runtime.user.email ?? '' } : null,
      logout: () => submitLogout(win),
    })
    disposers.push(() => { shellHost.deactivate(); services.dispose() })
    shell = activateRuntimeShell({ context: runtime, host: shellHost, window: win, ...(deps.importModule ? { importModule: deps.importModule } : {}) })
  }

  return { dispose: () => { for (const d of disposers.splice(0).reverse()) d() }, shell }
}

if (typeof document !== 'undefined' && !(globalThis as { __IOTA_RUNTIME_NO_AUTOBOOT__?: boolean }).__IOTA_RUNTIME_NO_AUTOBOOT__) {
  const start = () => { bootRuntime(window) }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start, { once: true })
  else start()
}
