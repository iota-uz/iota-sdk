import { createMemo, createSignal, createUniqueId, For, onCleanup, onMount, Show, splitProps, type JSX } from 'solid-js'
import { Spinner } from '../display/Spinner'
import { classes } from '../internal/classes'

export type SpotlightMode = 'search' | 'ai'
export type SpotlightBadgeTone = 'exact' | 'match' | 'navigation' | 'directory' | 'conversation' | 'entity' | string

export interface SpotlightResultBadge { label: string; tone?: SpotlightBadgeTone }
export interface SpotlightResult {
  key: string
  title: string
  subtitle?: string
  meta?: string
  link?: string
  icon?: JSX.Element
  badges?: readonly SpotlightResultBadge[]
  group?: string
}

export interface SpotlightLabels {
  search: string
  searchPlaceholder: string
  aiPlaceholder: string
  empty: string
  emptyFooter: string
  nothingFound: string
  nothingFoundBody: string
  loading: string
  complete: string
  navigate: string
  open: string
  close: string
  aiEmpty: string
  aiLoading: string
}

export interface SpotlightProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'onError'> {
  open?: boolean
  defaultOpen?: boolean
  onOpenChange?: (open: boolean) => void
  query?: string
  defaultQuery?: string
  onQueryChange?: (query: string) => void
  mode?: SpotlightMode
  defaultMode?: SpotlightMode
  onModeChange?: (mode: SpotlightMode) => void
  search?: (query: string, signal: AbortSignal) => Promise<readonly SpotlightResult[]>
  askAI?: (query: string, signal: AbortSignal) => Promise<readonly SpotlightResult[]>
  onNavigate?: (result: SpotlightResult) => void
  onError?: (error: unknown) => void
  debounceMs?: number
  enableAI?: boolean
  shortcut?: boolean
  results?: readonly SpotlightResult[]
  loading?: boolean
  labels?: Partial<SpotlightLabels>
}

const defaultLabels: SpotlightLabels = {
  search: 'Search', searchPlaceholder: 'Search...', aiPlaceholder: 'Ask AI anything...', empty: 'Start typing to search.',
  emptyFooter: 'Search across the application', nothingFound: 'Nothing found', nothingFoundBody: 'Try another search.',
  loading: 'Finding matches', complete: 'Search complete', navigate: '↑↓ Navigate', open: '↵ Open', close: 'Esc Close',
  aiEmpty: 'Ask a question to get started.', aiLoading: 'Searching…',
}

export function spotlightBadgeClass(tone: SpotlightBadgeTone = ''): string {
  const base = 'inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-semibold tracking-wide uppercase'
  const tones: Record<string, string> = {
    exact: 'border-emerald-300/30 bg-emerald-50 text-emerald-700 dark:border-emerald-400/40 dark:bg-emerald-500/10 dark:text-emerald-300',
    match: 'border-amber-300/30 bg-amber-50 text-amber-700 dark:border-amber-400/40 dark:bg-amber-500/10 dark:text-amber-200',
    navigation: 'border-default/30 bg-slate-50 text-slate-700 dark:bg-slate-500/10 dark:text-slate-200',
    directory: 'border-violet-300/30 bg-violet-50 text-violet-700 dark:border-violet-400/30 dark:bg-violet-500/10 dark:text-violet-200',
    conversation: 'border-fuchsia-300/30 bg-fuchsia-50 text-fuchsia-700 dark:border-fuchsia-400/30 dark:bg-fuchsia-500/10 dark:text-fuchsia-200',
    entity: 'border-sky-300/30 bg-sky-50 text-sky-700 dark:border-sky-400/30 dark:bg-sky-500/10 dark:text-sky-200',
  }
  return `${base} ${tones[tone] ?? 'border-secondary bg-surface-400 text-300'}`
}

export function SpotlightResultBadge(props: SpotlightResultBadge) {
  return <span class={spotlightBadgeClass(props.tone)}>{props.label}</span>
}

export interface SpotlightLinkItemProps extends SpotlightResult { onClick?: JSX.EventHandler<HTMLAnchorElement, MouseEvent>; as?: 'link' | 'content' }
export function SpotlightLinkItem(props: SpotlightLinkItemProps) {
  const content = <>
    <Show when={props.icon}><span class="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border border-secondary bg-surface-400 text-300 transition-colors duration-150 group-hover:bg-surface-200">{props.icon}</span></Show>
    <div class="min-w-0 flex-1"><div class="flex items-start justify-between gap-3"><div class="min-w-0 flex-1"><div class="truncate text-sm font-semibold leading-5 text-100">{props.title}</div><Show when={props.subtitle}><div class="mt-0.5 truncate text-xs leading-5 text-300">{props.subtitle}</div></Show><Show when={props.meta}><div class="mt-1 truncate text-[11px] leading-5 text-300/85">{props.meta}</div></Show></div><Show when={props.badges?.length}><div class="flex max-w-[42%] flex-wrap justify-end gap-1"><For each={props.badges}>{(badge) => <SpotlightResultBadge {...badge} />}</For></div></Show></div></div>
  </>
  return <Show when={props.as === 'content'} fallback={<a href={props.link} class="group flex w-full items-start gap-3" data-spotlight-key={props.key} onClick={props.onClick}>{content}</a>}>
    <div class="group flex w-full items-start gap-3" data-spotlight-key={props.key}>{content}</div>
  </Show>
}

export interface SpotlightItemProps extends JSX.HTMLAttributes<HTMLLIElement> { highlighted?: boolean }
export function SpotlightItem(props: SpotlightItemProps) {
  const [local, native] = splitProps(props, ['highlighted', 'class', 'children'])
  return <li {...native} data-spotlight-item class={classes('mx-1.5 my-1 rounded-xl border border-transparent bg-transparent px-3 py-3 text-100 transition-all duration-100 hover:border-secondary hover:bg-surface-100/70', local.highlighted && 'border-primary-300/30 bg-primary-500/8 shadow-sm', local.class)}>{local.children}</li>
}

export function SpotlightGroup(props: { title: JSX.Element; children?: JSX.Element }) {
  return <><li class="sticky top-0 z-10 mx-1.5 mt-2 rounded-lg bg-surface-300/95 px-3 py-2 text-[11px] font-semibold uppercase tracking-[0.2em] text-300 backdrop-blur">{props.title}</li>{props.children}</>
}

export function SpotlightNotFound(props: { title?: JSX.Element; body?: JSX.Element }) {
  return <li class="px-5 py-8 text-center"><div class="text-300" aria-hidden="true">⌕</div><p class="mt-2 text-sm font-medium text-200">{props.title ?? defaultLabels.nothingFound}</p><p class="mt-1 text-xs text-300">{props.body ?? defaultLabels.nothingFoundBody}</p></li>
}

export function SpotlightResults(props: { children?: JSX.Element }) { return <>{props.children}</> }

export interface SpotlightItemsCollapsibleProps {
  exactItems?: JSX.Element
  moreItems?: JSX.Element
  moreLabel: JSX.Element
}
export function SpotlightItemsCollapsible(props: SpotlightItemsCollapsibleProps) {
  const [expanded, setExpanded] = createSignal(false)
  const id = createUniqueId()
  return <>{props.exactItems}<Show when={!expanded()}><li class="mx-1.5 list-none"><button type="button" aria-expanded="false" aria-controls={id} class="w-full rounded-lg px-3 py-2 text-center text-xs font-medium text-300 hover:bg-surface-100/50 hover:text-200 transition-colors cursor-pointer" onClick={() => setExpanded(true)}>{props.moreLabel}</button></li></Show><Show when={expanded()}><li class="mx-1.5 list-none"><ul id={id} data-spotlight-more role="region" class="list-none m-0 p-0">{props.moreItems}</ul></li></Show></>
}

interface SpotlightRegistration {
  shortcut: () => boolean
  open: () => void
  escape: () => boolean
}

const spotlightRegistrations: SpotlightRegistration[] = []
const globalSpotlightKeydown = (event: KeyboardEvent) => {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
    const owner = [...spotlightRegistrations].reverse().find((item) => item.shortcut())
    if (owner) { event.preventDefault(); owner.open() }
    return
  }
  if (event.key !== 'Escape') return
  const owner = [...spotlightRegistrations].reverse().find((item) => item.escape())
  if (owner) event.preventDefault()
}

export function Spotlight(props: SpotlightProps) {
  const generatedID = createUniqueId()
  const [local, native] = splitProps(props, ['open', 'defaultOpen', 'onOpenChange', 'query', 'defaultQuery', 'onQueryChange', 'mode', 'defaultMode', 'onModeChange', 'search', 'askAI', 'onNavigate', 'onError', 'debounceMs', 'enableAI', 'shortcut', 'results', 'loading', 'labels', 'class'])
  const [internalOpen, setInternalOpen] = createSignal(local.defaultOpen ?? false)
  const [internalQuery, setInternalQuery] = createSignal(local.defaultQuery ?? '')
  const [internalMode, setInternalMode] = createSignal<SpotlightMode>(local.defaultMode ?? 'search')
  const [internalResults, setInternalResults] = createSignal<readonly SpotlightResult[]>([])
  const [internalLoading, setInternalLoading] = createSignal(false)
  const [highlighted, setHighlighted] = createSignal(0)
  let input!: HTMLInputElement
  let timer: ReturnType<typeof setTimeout> | undefined
  let request: AbortController | undefined
  const open = () => local.open ?? internalOpen()
  const query = () => local.query ?? internalQuery()
  const mode = () => local.mode ?? internalMode()
  const results = () => local.results ?? internalResults()
  const loading = () => local.loading ?? internalLoading()
  const labels = () => ({ ...defaultLabels, ...local.labels })
  const panelID = `${generatedID}-spotlight-panel`
  const grouped = createMemo(() => {
    const groups = new Map<string, SpotlightResult[]>()
    for (const result of results()) groups.set(result.group ?? '', [...(groups.get(result.group ?? '') ?? []), result])
    return [...groups.entries()]
  })
  const setOpen = (value: boolean) => { if (local.open === undefined) setInternalOpen(value); local.onOpenChange?.(value); if (value) queueMicrotask(() => { if (input?.isConnected && open()) input.focus() }); else { request?.abort(); setHighlighted(0) } }
  const setMode = (value: SpotlightMode) => { if (local.mode === undefined) setInternalMode(value); local.onModeChange?.(value); setHighlighted(0) }
  const run = async (value: string) => {
    request?.abort()
    const source = mode() === 'ai' ? local.askAI : local.search
    if (!value.trim() || !source) { if (local.results === undefined) setInternalResults([]); setInternalLoading(false); return }
    const controller = new AbortController()
    request = controller
    setInternalLoading(true)
    try {
      const next = await source(value.trim(), controller.signal)
      if (!controller.signal.aborted && request === controller && local.results === undefined) setInternalResults(next)
    } catch (error) { if (!controller.signal.aborted && request === controller) local.onError?.(error) }
    finally { if (!controller.signal.aborted && request === controller) setInternalLoading(false) }
  }
  const updateQuery = (value: string) => {
    if (local.query === undefined) setInternalQuery(value)
    local.onQueryChange?.(value)
    clearTimeout(timer)
    if (mode() === 'search') timer = setTimeout(() => void run(value), local.debounceMs ?? 120)
  }
  const activate = (result: SpotlightResult) => {
    local.onNavigate?.(result)
    if (!local.onNavigate && result.link) window.location.assign(result.link)
  }
  const inputKeydown: JSX.EventHandler<HTMLInputElement, KeyboardEvent> = (event) => {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      if (results().length) setHighlighted((value) => (value + (event.key === 'ArrowDown' ? 1 : -1) + results().length) % results().length)
    } else if (event.key === 'Enter') {
      event.preventDefault()
      if (mode() === 'ai') void run(query())
      else { const result = results()[highlighted()]; if (result) activate(result) }
    } else if (event.key === 'Tab' && local.enableAI !== false) {
      event.preventDefault(); setMode(mode() === 'search' ? 'ai' : 'search')
    }
  }
  onMount(() => {
    const registration: SpotlightRegistration = {
      shortcut: () => local.shortcut !== false,
      open: () => setOpen(true),
      escape: () => {
        if (!open()) return false
        if (mode() === 'ai') setMode('search')
        else setOpen(false)
        return true
      },
    }
    spotlightRegistrations.push(registration)
    if (spotlightRegistrations.length === 1) document.addEventListener('keydown', globalSpotlightKeydown)
    onCleanup(() => {
      const index = spotlightRegistrations.indexOf(registration)
      if (index >= 0) spotlightRegistrations.splice(index, 1)
      if (spotlightRegistrations.length === 0) document.removeEventListener('keydown', globalSpotlightKeydown)
    })
  })
  onCleanup(() => { clearTimeout(timer); request?.abort() })
  return <div {...native} class={classes('relative', local.class)}>
    <button class="flex h-10 cursor-pointer items-center gap-2 rounded-xl border border-secondary bg-surface-300 px-3 text-300 shadow-sm transition-colors duration-150 hover:bg-surface-300/80 hover:text-100" type="button" aria-label={labels().search} aria-controls={panelID} aria-expanded={open()} onClick={() => setOpen(true)}><span aria-hidden="true">⌕</span><span class="sr-only">{labels().search}</span><kbd class="text-[11px] font-medium">⌘K</kbd></button>
    <Show when={open()}><div class="fixed inset-0 z-50 flex w-screen items-center justify-center bg-slate-950/45 px-4 pb-[8vh] pt-[12vh] backdrop-blur-sm" onMouseDown={(event) => { if (event.target === event.currentTarget) setOpen(false) }}>
      <div id={panelID} class="flex max-h-full w-full max-w-3xl flex-col overflow-hidden rounded-2xl border border-secondary bg-surface-300 shadow-2xl" role="dialog" aria-modal="true" aria-label={labels().search}>
        <div class="border-b border-secondary px-5 py-4"><div class="flex items-center gap-3"><div class="flex shrink-0 items-center text-300">⌕</div><div class="min-w-0 flex-1"><input ref={input} type="text" value={query()} class="w-full bg-transparent text-base font-medium text-100 placeholder:text-300/80 focus:outline-none" placeholder={mode() === 'ai' ? labels().aiPlaceholder : labels().searchPlaceholder} autocomplete="off" onInput={(event) => updateQuery(event.currentTarget.value)} onKeyDown={inputKeydown} /></div><Show when={local.enableAI !== false}><div class="flex shrink-0 items-center"><div class="flex rounded-lg border border-secondary bg-surface-400 p-0.5 text-[11px] font-medium"><button type="button" class={classes('cursor-pointer rounded-md px-2.5 py-1 transition-colors', mode() === 'search' ? 'bg-white text-gray-900 shadow-sm dark:bg-surface-300 dark:text-100' : 'text-300 hover:text-200')} onClick={() => setMode('search')}>{labels().search}</button><button type="button" class={classes('flex cursor-pointer items-center gap-1 rounded-md px-2.5 py-1 transition-colors', mode() === 'ai' ? 'bg-white text-gray-900 shadow-sm dark:bg-surface-300 dark:text-100' : 'text-300 hover:text-200')} onClick={() => setMode('ai')}>✦ AI</button></div></div></Show></div></div>
        <div class="min-h-0 flex-1 overflow-y-auto overscroll-contain">
          <Show when={!query()}><div class="px-5 py-8 text-center"><p class="text-sm text-300">{mode() === 'ai' ? labels().aiEmpty : labels().empty}</p></div></Show>
          <Show when={query()}><ul class="px-2 py-2" role="listbox" aria-busy={loading()}>{grouped().map(([group, items]) => <><Show when={group}><SpotlightGroup title={group} /></Show><For each={items}>{(result) => { const index = () => results().indexOf(result); return <SpotlightItem highlighted={highlighted() === index()} role="option" aria-selected={highlighted() === index()} tabindex={highlighted() === index() ? 0 : -1} onClick={() => activate(result)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); activate(result) } }}><SpotlightLinkItem {...result} as="content" /></SpotlightItem> }}</For></>) }<Show when={!loading() && results().length === 0}><SpotlightNotFound title={labels().nothingFound} body={labels().nothingFoundBody} /></Show></ul><Show when={loading()}><Spinner class="flex justify-center items-center py-8" /></Show></Show>
        </div>
        <div class="border-t border-secondary bg-surface-400/50 px-5 py-3"><div class="flex items-center justify-between gap-4"><div class="min-w-0 flex-1"><Show when={!query()} fallback={<span class="inline-flex items-center gap-1.5 text-[11px] font-medium text-300"><span class={classes('h-1.5 w-1.5 rounded-full', loading() ? 'bg-amber-400 animate-pulse' : 'bg-emerald-400')} />{loading() ? (mode() === 'ai' ? labels().aiLoading : labels().loading) : labels().complete}</span>}><p class="text-[11px] text-300">{labels().emptyFooter}</p></Show></div><div class="hidden items-center gap-3 text-[11px] text-300 xl:flex"><span>{labels().navigate}</span><span>{labels().open}</span><span>{labels().close}</span></div></div></div>
      </div>
    </div></Show>
  </div>
}
