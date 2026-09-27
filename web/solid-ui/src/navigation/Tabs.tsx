import { createContext, createUniqueId, onCleanup, splitProps, useContext, type Accessor, type JSX } from 'solid-js'
import { createSignal } from 'solid-js'
import { classes } from '../internal/classes'

interface TabsContextValue {
  active: Accessor<string>
  select(value: string): void
  id: string
  remote: Accessor<JSX.Element | undefined>
  loading: Accessor<boolean>
  load(href: string, loader: BoostedTabLoader): Promise<void>
  contentId: string
}

const TabsContext = createContext<TabsContextValue>()

function useTabs(): TabsContextValue {
  const context = useContext(TabsContext)
  if (!context) throw new Error('Tab components must be rendered inside Tabs')
  return context
}

export interface TabsProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'onChange'> {
  value?: string
  defaultValue?: string
  onValueChange?: (value: string) => void
}

export interface BoostedTabRequest { href: string; signal: AbortSignal }
export type BoostedTabLoader = (request: BoostedTabRequest) => Promise<JSX.Element>

export function Tabs(props: TabsProps) {
  const [local, native] = splitProps(props, ['value', 'defaultValue', 'onValueChange', 'children'])
  const [internal, setInternal] = createSignal(local.defaultValue ?? local.value ?? '')
  const [remote, setRemote] = createSignal<JSX.Element>()
  const [loading, setLoading] = createSignal(false)
  const id = createUniqueId()
  let request: AbortController | undefined
  let revision = 0
  const active = () => local.value ?? internal()
  const select = (value: string) => {
    if (local.value === undefined) setInternal(value)
    local.onValueChange?.(value)
  }
  onCleanup(() => request?.abort())
  const context: TabsContextValue = {
    active,
    select,
    id,
    remote,
    loading,
    contentId: `${id}-boosted-content`,
    async load(href, loader) {
      request?.abort()
      const current = ++revision
      request = new AbortController()
      setLoading(true)
      try {
        const content = await loader({ href, signal: request.signal })
        if (current === revision && !request.signal.aborted) setRemote(() => content)
      } finally {
        if (current === revision) setLoading(false)
      }
    },
  }
  return <TabsContext.Provider value={context}><div {...native}>{local.children}</div></TabsContext.Provider>
}

function moveTab(event: KeyboardEvent): void {
  if (!['ArrowRight', 'ArrowLeft', 'Home', 'End'].includes(event.key)) return
  const list = event.currentTarget as HTMLElement
  const tabs = Array.from(list.querySelectorAll<HTMLElement>('[role="tab"]:not([disabled])'))
  const current = tabs.indexOf(event.target as HTMLElement)
  if (current < 0 || tabs.length === 0) return
  event.preventDefault()
  const next = event.key === 'Home' ? 0
    : event.key === 'End' ? tabs.length - 1
      : (current + (event.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length
  tabs[next]?.focus()
  tabs[next]?.click()
}

export function TabsList(props: JSX.HTMLAttributes<HTMLDivElement>) {
  const [local, native] = splitProps(props, ['class', 'children', 'onKeyDown'])
  return (
    <div
      {...native}
      role="tablist"
      class={classes('flex flex-nowrap gap-2 border-b overflow-x-auto', local.class)}
      onKeyDown={(event) => { moveTab(event); if (typeof local.onKeyDown === 'function') local.onKeyDown(event) }}
    >
      {local.children}
    </div>
  )
}

export interface TabsTriggerProps extends Omit<JSX.ButtonHTMLAttributes<HTMLButtonElement>, 'value'> { value: string }

export function TabsTrigger(props: TabsTriggerProps) {
  const context = useTabs()
  const [local, native] = splitProps(props, ['value', 'class', 'children', 'onClick'])
  const active = () => context.active() === local.value
  return (
    <button
      {...native}
      id={`${context.id}-tab-${local.value}`}
      type="button"
      role="tab"
      data-tab-value={local.value}
      aria-selected={active()}
      aria-controls={`${context.id}-panel-${local.value}`}
      tabIndex={active() ? 0 : -1}
      class={classes('shrink-0 btn cursor-pointer btn-ghost btn-normal rounded-none focus-visible:outline-offset-[1px] after:absolute after:left-0 after:bottom-0 after:h-0.5 after:w-full', active() && 'after:bg-brand-500', local.class)}
      onClick={(event) => { context.select(local.value); if (typeof local.onClick === 'function') local.onClick(event) }}
    >
      {local.children}<div class="btn-loading-indicator" />
    </button>
  )
}

export interface TabsContentProps extends JSX.HTMLAttributes<HTMLDivElement> { value: string }

export function TabsContent(props: TabsContentProps) {
  const context = useTabs()
  const [local, native] = splitProps(props, ['value', 'children'])
  return <div {...native} id={`${context.id}-panel-${local.value}`} role="tabpanel" aria-labelledby={`${context.id}-tab-${local.value}`} hidden={context.active() !== local.value}>{local.children}</div>
}

export interface TabLinkProps extends JSX.AnchorHTMLAttributes<HTMLAnchorElement> { active?: boolean }

export function TabLink(props: TabLinkProps) {
  const [local, native] = splitProps(props, ['active', 'class', 'children'])
  return <a {...native} class={classes('shrink-0 btn cursor-pointer btn-ghost btn-normal rounded-none focus-visible:outline-offset-[1px] after:absolute after:left-0 after:bottom-0 after:h-0.5 after:w-full', local.active && 'after:bg-brand-500', local.class)}>{local.children}<div class="btn-loading-indicator" /></a>
}

export interface BoostedLinkProps extends Omit<JSX.ButtonHTMLAttributes<HTMLButtonElement>, 'onError' | 'onLoad'> {
  href: string
  push?: boolean
  loader: BoostedTabLoader
  onLoad?: (href: string) => void
  onError?: (error: unknown, href: string) => void
}

export function BoostedLink(props: BoostedLinkProps) {
  const context = useTabs()
  const [local, native] = splitProps(props, ['href', 'push', 'loader', 'onLoad', 'onError', 'class', 'children', 'onClick'])
  const active = () => context.active() === local.href
  return <button {...native} type="button" aria-controls={context.contentId} aria-current={active() ? 'page' : undefined} class={classes('shrink-0 whitespace-nowrap btn cursor-pointer btn-ghost btn-normal rounded-none focus-visible:outline-offset-[1px] after:absolute after:left-0 after:bottom-0 after:h-0.5 after:w-full', active() && 'after:bg-brand-500', local.class)} onClick={(event) => {
    context.select(local.href)
    if (local.push) event.currentTarget.ownerDocument.defaultView?.history.pushState(null, '', local.href)
    void context.load(local.href, local.loader).then(() => local.onLoad?.(local.href), (error) => { if (!(error instanceof DOMException && error.name === 'AbortError')) local.onError?.(error, local.href) })
    if (typeof local.onClick === 'function') local.onClick(event)
  }}>{local.children}<div class="btn-loading-indicator" /></button>
}

export interface BoostedContentProps extends JSX.HTMLAttributes<HTMLDivElement> { fallback?: JSX.Element }

export function BoostedContent(props: BoostedContentProps) {
  const context = useTabs()
  const [local, native] = splitProps(props, ['fallback', 'class', 'children'])
  return <div {...native} id={native.id ?? context.contentId} aria-live="polite" aria-busy={context.loading()} class={local.class}>{context.remote() ?? (context.loading() ? local.fallback : local.children)}</div>
}
