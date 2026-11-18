// Mount runtime of package pages. The canonical client host owns the lifecycle
// (bootstrap validation, services, portals, theme); this module only loads the
// package page module, renders it and manages loading / failure / disposal.
import { createComponent, ErrorBoundary, type Component } from 'solid-js'
import type { PageProps } from '../client.ts'
import { decodeRouteParams } from './params.ts'
import { isPageData, type PageData } from './protocol.ts'
import { RuntimeProvider } from './hooks.ts'
import { createSlotSource, SlotsProvider } from './slots.ts'
import { ensureStyles } from './styles.ts'
import type { RuntimeTransport } from './transport.ts'

/** The slice of @iota-uz/sdk/client-host the page runtime needs, injected so it is testable. */
export interface RouteHost<Context extends { theme: 'light' | 'dark' }> {
  readContext(document: Document): Context
  mount(options: {
    root: HTMLElement
    component: Component<{ route: Context }>
    props: object
    context: Context
  }): () => void
}

/** What a loaded page module exposes: the page value built by `page()` has a component. */
export interface LoadedPage {
  component: Component<PageProps<never>>
}

export interface PageRuntimeOptions<Context extends { theme: 'light' | 'dark'; initial: unknown }> {
  document: Document
  host: RouteHost<Context>
  transport: RuntimeTransport
  /** Dynamic import, injectable for tests. */
  importModule?: (url: string) => Promise<Record<string, unknown>>
  /** Navigates or reloads so a retry starts from a fresh module map. */
  reload?: () => void
  /** Messages for the host-owned states. */
  messages?: { loading: string; failed: string; invalid: string; retry: string }
}

const DEFAULT_MESSAGES = {
  loading: 'Loading…',
  failed: 'The screen could not be loaded.',
  invalid: 'The screen module is invalid.',
  retry: 'Retry',
}

export function documentTheme(owner: Document, fallback: 'light' | 'dark'): 'light' | 'dark' {
  const root = owner.documentElement
  if (root.dataset.theme === 'dark') return 'dark'
  if (root.dataset.theme === 'light') return 'light'
  if (root.classList.contains('dark')) return 'dark'
  if (root.classList.contains('light')) return 'light'
  if (root.classList.contains('system') && owner.defaultView?.matchMedia?.('(prefers-color-scheme: dark)').matches) return 'dark'
  return fallback
}

function resolveExport(module: Record<string, unknown>, name: string | undefined): LoadedPage | undefined {
  const value = (name ? module[name] : module.default) as Partial<LoadedPage> | undefined
  return value && typeof value.component === 'function' ? (value as LoadedPage) : undefined
}

/** Whether the document is a runtime page (its client context carries PageData). */
export function documentPageData(document: Document): PageData | undefined {
  const element = document.getElementById('iota-client-context')
  if (!element?.textContent) return undefined
  try {
    const initial = (JSON.parse(element.textContent) as { initial?: unknown }).initial
    return isPageData(initial) ? initial : undefined
  } catch {
    return undefined
  }
}

/**
 * Mounts the runtime page of the document into the canonical route mount.
 * Returns the disposer: it unmounts the page, restores the mount element and
 * makes any response that arrives later a no-op.
 */
export function mountRuntimePage<Context extends { theme: 'light' | 'dark'; initial: unknown }>(
  options: PageRuntimeOptions<Context>,
): () => void {
  const { document: owner, host } = options
  const root = owner.getElementById('iota-client-route-mount')
  if (!root) throw new Error('Client route mount element is missing')
  const context = host.readContext(owner)
  const data = context.initial
  if (!isPageData(data)) throw new Error('The client route is not a runtime page')
  if (root.dataset.iotaClientOwner) throw new Error(`Client route root is already owned by ${root.dataset.iotaClientOwner}`)
  context.theme = documentTheme(owner, context.theme)
  const messages = { ...DEFAULT_MESSAGES, ...options.messages }
  const load = options.importModule ?? ((url: string) => import(/* @vite-ignore */ url) as Promise<Record<string, unknown>>)

  const slots = createSlotSource({
    contributions: data.slots ?? {},
    ...(options.importModule ? { importModule: options.importModule } : {}),
    document: owner,
    console: owner.defaultView?.console ?? console,
  })
  let active = true
  let generation = 0
  let unmount: (() => void) | undefined
  root.dataset.iotaClientOwner = 'runtime-loading'

  const show = (child: HTMLElement | undefined) => {
    root.replaceChildren(...(child ? [child] : []))
  }

  const status = (text: string, attr: string, role: string) => {
    const element = owner.createElement('div')
    element.setAttribute(attr, 'true')
    element.setAttribute('role', role)
    element.textContent = text
    return element
  }

  /** The host's failure state: the message and, for a load that may succeed again, Retry. */
  const failure = (text: string, withRetry: boolean) => {
    const alert = status(text, 'data-runtime-page-error', 'alert')
    if (withRetry) {
      const retry = owner.createElement('button')
      retry.type = 'button'
      retry.dataset.runtimePageRetry = 'true'
      retry.textContent = messages.retry
      // A rejected dynamic import stays rejected in the module map until the
      // document is reloaded, so retrying means starting from a fresh one.
      retry.addEventListener('click', () => (options.reload ?? (() => owner.defaultView?.location.reload()))(), { once: true })
      alert.append(retry)
    }
    return alert
  }

  const fail = (text: string, cause: unknown, withRetry: boolean) => {
    root.removeAttribute('aria-busy')
    show(failure(text, withRetry))
    delete root.dataset.iotaClientOwner
    owner.defaultView?.console.error(`Runtime page ${data.page.id}: ${text}`, cause)
  }

  const start = () => {
    const current = ++generation
    root.setAttribute('aria-busy', 'true')
    show(status(messages.loading, 'data-runtime-page-loading', 'status'))
    // The version's stylesheets load alongside its module (the server already links them in the page HTML, so this
    // usually finds them): the page renders only once both are in, so it never paints unstyled.
    void Promise.all([load(data.page.module), ensureStyles(owner, data.page.styles)]).then(
      ([module]) => {
        if (!active || current !== generation) return // disposed while loading: the late module is dropped
        const page = resolveExport(module, data.page.export)
        if (!page) {
          fail(messages.invalid, new Error(`module ${data.page.module} has no page export "${data.page.export ?? 'default'}"`), false)
          return
        }
        let params: Record<string, unknown>
        try {
          params = decodeRouteParams(data.paramTypes, data.params)
        } catch (cause) {
          fail(messages.invalid, cause, false)
          return
        }
        root.removeAttribute('aria-busy')
        show(undefined)
        delete root.dataset.iotaClientOwner
        const Page = page.component
        // Page components are usually `lazy(() => import(...))`: their chunk arrives after the page module, and a
        // chunk that cannot be fetched rejects while the page is already mounted. That failure is a load failure
        // like the module's own: the host shows it, with Retry, instead of leaving an empty page.
        let mounting = true
        let early: { error: unknown } | undefined
        try {
          unmount = host.mount({
            root,
            context,
            props: {},
            component: (hostProps) => createComponent(RuntimeProvider, {
              transport: options.transport,
              get children() {
                return createComponent(ErrorBoundary, {
                  fallback: (cause: unknown) => {
                    if (mounting) {
                      early = { error: cause } // thrown while rendering: an invalid page, reported below
                      return undefined
                    }
                    owner.defaultView?.console.error(`Runtime page ${data.page.id}: ${messages.failed}`, cause)
                    return failure(messages.failed, true)
                  },
                  get children() {
                    return createComponent(SlotsProvider, {
                      source: slots,
                      get children() { return createComponent(Page as Component<PageProps<never> & { route: Context }>, { params: params as never, route: hostProps.route }) },
                    })
                  },
                })
              },
            }),
          })
        } catch (cause) {
          fail(messages.invalid, cause, false)
        } finally {
          mounting = false
        }
        if (early) {
          const failed = early.error
          const release = unmount
          unmount = undefined
          try { release?.() } finally { fail(messages.invalid, failed, false) }
        }
      },
      (cause: unknown) => {
        if (!active || current !== generation) return
        fail(messages.failed, cause, true)
      },
    )
  }

  const dispose = () => {
    if (!active) return
    active = false
    generation++
    owner.defaultView?.removeEventListener('pagehide', dispose)
    owner.removeEventListener('iota:client-route-unmount', dispose)
    try {
      unmount?.()
    } finally {
      root.removeAttribute('aria-busy')
      root.replaceChildren()
      delete root.dataset.iotaClientOwner
    }
  }

  owner.defaultView?.addEventListener('pagehide', dispose, { once: true })
  owner.addEventListener('iota:client-route-unmount', dispose, { once: true })
  start()
  return dispose
}
