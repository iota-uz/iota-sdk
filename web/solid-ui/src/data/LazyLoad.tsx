import { createSignal, onCleanup, onMount, Show, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'
import { Spinner } from '../display/Spinner'

export type LazyLoadTrigger = 'load' | 'visible'
export type LazyLoadSwap = 'outerHTML' | 'innerHTML'
export type LazyFragment = string | JSX.Element

export interface LazyLoadRequest {
  endpoint: string
  url: string
  signal: AbortSignal
}

export type LazyLoadAdapter = (request: LazyLoadRequest) => Promise<LazyFragment>

export interface LazyLoadObserver {
  observe(element: Element): void
  disconnect(): void
}

export type LazyLoadObserverFactory = (callback: IntersectionObserverCallback, options: IntersectionObserverInit) => LazyLoadObserver

export interface LazyLoadProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'children' | 'onError' | 'onLoad'> {
  endpoint: string
  params?: URLSearchParams | Readonly<Record<string, string | number | boolean | null | undefined>>
  swap?: LazyLoadSwap
  trigger?: LazyLoadTrigger
  rootMargin?: string
  loaderClass?: string
  adapter?: LazyLoadAdapter
  observerFactory?: LazyLoadObserverFactory
  renderHTML?: (html: string) => JSX.Element
  children?: JSX.Element
  loading?: JSX.Element
  error?: (error: unknown, retry: () => void) => JSX.Element
  loadingLabel?: string
  errorLabel?: string
  retryLabel?: string
  onLoad?: (fragment: LazyFragment) => void
  onError?: (error: unknown) => void
}

export function buildLazyLoadURL(endpoint: string, params?: LazyLoadProps['params']): string {
  if (!params) return endpoint
  const search = params instanceof URLSearchParams ? new URLSearchParams(params) : new URLSearchParams()
  if (!(params instanceof URLSearchParams)) {
    for (const [key, value] of Object.entries(params)) if (value !== undefined && value !== null) search.set(key, String(value))
  }
  const query = search.toString()
  if (!query) return endpoint
  return `${endpoint}${endpoint.includes('?') ? '&' : '?'}${query}`
}

export const fetchLazyFragment: LazyLoadAdapter = async ({ url, signal }) => {
  const response = await fetch(url, { signal, headers: { 'X-Requested-With': 'XMLHttpRequest' } })
  if (!response.ok) throw new Error(`Lazy load failed with ${response.status}`)
  return response.text()
}

function parsedHTML(html: string): Node[] {
  if (typeof document === 'undefined') throw new Error('LazyLoad raw HTML requires a DOM; provide renderHTML when rendering outside a browser')
  const template = document.createElement('template')
  template.innerHTML = html
  return Array.from(template.content.childNodes)
}

export function LazyLoad(props: LazyLoadProps) {
  let element!: HTMLDivElement
  let controller: AbortController | undefined
  let observer: LazyLoadObserver | undefined
  let disposed = false
  let requestVersion = 0
  const [local, native] = splitProps(props, [
    'endpoint', 'params', 'swap', 'trigger', 'rootMargin', 'loaderClass', 'adapter', 'observerFactory', 'renderHTML', 'children', 'loading', 'error', 'loadingLabel', 'errorLabel', 'retryLabel', 'onLoad', 'onError', 'class', 'ref',
  ])
  const [fragment, setFragment] = createSignal<JSX.Element>()
  const [failure, setFailure] = createSignal<unknown>()
  const [loading, setLoading] = createSignal(false)

  const load = async () => {
    const version = ++requestVersion
    controller?.abort()
    controller = new AbortController()
    setFailure(undefined)
    setLoading(true)
    try {
      const result = await (local.adapter ?? fetchLazyFragment)({ endpoint: local.endpoint, url: buildLazyLoadURL(local.endpoint, local.params), signal: controller.signal })
      if (disposed || controller.signal.aborted || version !== requestVersion) return
      const content = typeof result === 'string' ? (local.renderHTML?.(result) ?? parsedHTML(result)) : result
      setFragment(() => content)
      local.onLoad?.(result)
    } catch (error) {
      if (disposed || controller.signal.aborted || version !== requestVersion) return
      setFailure(error)
      local.onError?.(error)
    } finally {
      if (!disposed && version === requestVersion) setLoading(false)
    }
  }

  onMount(() => {
    if ((local.trigger ?? 'load') === 'load') {
      void load()
      return
    }
    try {
      const factory = local.observerFactory ?? ((callback, options) => new IntersectionObserver(callback, options))
      observer = factory((entries) => {
        if (!entries.some((entry) => entry.isIntersecting)) return
        observer?.disconnect()
        observer = undefined
        void load()
      }, { rootMargin: local.rootMargin ?? '200px' })
      observer.observe(element)
    } catch (error) {
      setFailure(error)
      local.onError?.(error)
    }
  })

  onCleanup(() => {
    disposed = true
    requestVersion += 1
    controller?.abort()
    observer?.disconnect()
  })

  const pending = () => local.loading ?? local.children ?? <Spinner class={classes('flex items-center justify-center p-4', local.loaderClass)} label={local.loadingLabel} />
  const errorView = () => local.error?.(failure(), load) ?? (
    <div role="alert" class="flex flex-col items-center justify-center gap-2 p-4 text-sm text-red-600">
      <span>{local.errorLabel ?? 'Unable to load content'}</span>
      <button type="button" class="btn btn-secondary btn-sm" onClick={() => void load()}>{local.retryLabel ?? 'Retry'}</button>
    </div>
  )
  const wrapper = (content: JSX.Element) => (
    <div
      {...native}
      ref={(node) => {
        element = node
        if (typeof local.ref === 'function') local.ref(node)
      }}
      class={classes('lazy-load', local.class)}
      aria-busy={loading() ? 'true' : undefined}
    >
      {content}
    </div>
  )

  return (
    <Show when={fragment()} keyed fallback={wrapper(
      <Show when={failure()} keyed fallback={pending()}>{errorView()}</Show>,
    )}>
      {(content) => (local.swap ?? 'outerHTML') === 'innerHTML' ? wrapper(content) : content}
    </Show>
  )
}
