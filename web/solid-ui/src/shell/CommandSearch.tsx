import { createEffect, createSignal, createUniqueId, For, onCleanup, onMount, Show, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'
import { Loader } from '../data/Loader'

function SearchIcon(props: { size?: number }) { return <svg aria-hidden="true" width={props.size ?? 18} height={props.size ?? 18} viewBox="0 0 256 256" fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="16"><circle cx="112" cy="112" r="72" /><line x1="163" y1="163" x2="216" y2="216" /></svg> }

export interface CommandResultData { key: string; title: JSX.Element; href?: string; subtitle?: JSX.Element; meta?: JSX.Element; icon?: JSX.Element; badges?: readonly JSX.Element[]; group?: string; disabled?: boolean }

export interface CommandResultProps extends Omit<JSX.LiHTMLAttributes<HTMLLIElement>, 'onSelect'> { result: CommandResultData; highlighted?: boolean; index?: number; resultId?: string; onSelect?: (result: CommandResultData, event: MouseEvent) => void }
export function CommandResult(props: CommandResultProps) {
  const [local, native] = splitProps(props, ['result', 'highlighted', 'index', 'resultId', 'onSelect', 'class'])
  const fallbackId = createUniqueId()
  const content = <>{local.result.icon && <span class="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border border-secondary bg-surface-400 text-300 transition-colors duration-150 group-hover:bg-surface-200">{local.result.icon}</span>}<div class="min-w-0 flex-1"><div class="flex items-start justify-between gap-3"><div class="min-w-0 flex-1"><div class="truncate text-sm font-semibold leading-5 text-100">{local.result.title}</div>{local.result.subtitle && <div class="mt-0.5 truncate text-xs leading-5 text-300">{local.result.subtitle}</div>}{local.result.meta && <div class="mt-1 truncate text-[11px] leading-5 text-300/85">{local.result.meta}</div>}</div>{local.result.badges && <div class="flex max-w-[42%] flex-wrap justify-end gap-1"><For each={local.result.badges}>{(badge) => badge}</For></div>}</div></div></>
  return <li {...native} id={local.resultId ?? (local.index === undefined ? undefined : `command-result-${fallbackId}-${local.index}`)} aria-disabled={local.result.disabled || undefined} data-command-item data-command-key={local.result.key} class={classes('mx-1.5 my-1 rounded-xl border border-transparent bg-transparent px-3 py-3 text-100 transition-all duration-100 hover:border-secondary hover:bg-surface-100/70', local.highlighted && 'border-primary-300/30 bg-primary-500/8 shadow-sm', local.result.disabled && 'opacity-50', local.class)}>{local.result.href ? <a href={local.result.href} tabIndex={-1} class="group flex w-full items-start gap-3" onClick={(event) => { if (local.result.disabled) event.preventDefault(); else local.onSelect?.(local.result, event) }}>{content}</a> : <button type="button" tabIndex={-1} disabled={local.result.disabled} class="group flex w-full items-start gap-3 text-left" onClick={(event) => local.onSelect?.(local.result, event)}>{content}</button>}</li>
}

export function CommandGroup(props: JSX.LiHTMLAttributes<HTMLLIElement>) { const [local, native] = splitProps(props, ['class', 'children']); return <li {...native} class={classes('sticky top-0 z-10 mx-1.5 mt-2 rounded-lg bg-surface-300/95 px-3 py-2 text-[11px] font-semibold uppercase tracking-[0.2em] text-300 backdrop-blur', local.class)}>{local.children}</li> }
export function CommandEmpty(props: JSX.LiHTMLAttributes<HTMLLIElement>) { const [local, native] = splitProps(props, ['class', 'children']); return <li {...native} class={classes('px-5 py-8 text-center', local.class)}><div class="text-300"><SearchIcon size={24} /></div><p class="mt-2 text-sm font-medium text-200">{local.children ?? 'Nothing found'}</p></li> }

export interface CommandSearchProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'onChange' | 'onSelect'> { results: readonly CommandResultData[]; query?: string; defaultQuery?: string; onQueryChange?: (query: string) => void; open?: boolean; defaultOpen?: boolean; onOpenChange?: (open: boolean) => void; onSelect?: (result: CommandResultData, event?: MouseEvent | KeyboardEvent) => void; loading?: boolean; label?: string; placeholder?: string; empty?: JSX.Element; trigger?: JSX.Element; footer?: JSX.Element; shortcut?: boolean }

export function CommandSearch(props: CommandSearchProps) {
  const [local, native] = splitProps(props, ['results', 'query', 'defaultQuery', 'onQueryChange', 'open', 'defaultOpen', 'onOpenChange', 'onSelect', 'loading', 'label', 'placeholder', 'empty', 'trigger', 'footer', 'shortcut', 'class'])
  const [internalQuery, setInternalQuery] = createSignal(local.defaultQuery ?? '')
  const [internalOpen, setInternalOpen] = createSignal(local.defaultOpen ?? false)
  const [highlighted, setHighlighted] = createSignal(0)
  const instance = createUniqueId()
  const resultsId = `command-results-${instance}`
  const query = () => local.query ?? internalQuery()
  const expanded = () => local.open ?? internalOpen()
  let input!: HTMLInputElement
  let trigger!: HTMLButtonElement
  const setQuery = (value: string) => { if (local.query === undefined) setInternalQuery(value); setHighlighted(0); local.onQueryChange?.(value) }
  const setOpen = (value: boolean) => { if (local.open === undefined) setInternalOpen(value); local.onOpenChange?.(value); queueMicrotask(() => { if (value && expanded() && input?.isConnected) input.focus(); else if (!value && !expanded() && trigger?.isConnected) trigger.focus() }) }
  const activeResults = () => local.results.filter((result) => !result.disabled)
  const move = (delta: number) => { const count = activeResults().length; if (count > 0) setHighlighted((highlighted() + delta + count) % count) }
  const select = (event: KeyboardEvent) => { const result = activeResults()[highlighted()]; if (result) local.onSelect?.(result, event) }
  const shortcut = (event: KeyboardEvent) => { if (!event.defaultPrevented && (local.shortcut ?? true) && (event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') { event.preventDefault(); setOpen(!expanded()) } }
  onMount(() => trigger.ownerDocument.addEventListener('keydown', shortcut))
  onCleanup(() => trigger?.ownerDocument.removeEventListener('keydown', shortcut))
  createEffect(() => { if (highlighted() >= activeResults().length) setHighlighted(Math.max(0, activeResults().length - 1)) })
  return <div {...native} class={classes('relative', local.class)}>
    <button ref={trigger} class="flex h-10 cursor-pointer items-center gap-2 rounded-xl border border-secondary bg-surface-300 px-3 text-300 shadow-sm transition-colors duration-150 hover:bg-surface-300/80 hover:text-100" type="button" aria-label={local.label ?? 'Search'} aria-expanded={expanded()} onClick={() => setOpen(true)}>{local.trigger ?? <><SearchIcon /><span class="sr-only">{local.label ?? 'Search'}</span><kbd class="text-[11px] font-medium">⌘K</kbd></>}</button>
    <Show when={expanded()}><div class="fixed inset-0 z-50 flex w-screen items-center justify-center bg-slate-950/45 px-4 pb-[8vh] pt-[12vh] backdrop-blur-sm" onPointerDown={(event) => { if (event.target === event.currentTarget) setOpen(false) }}><div class="flex max-h-full w-full max-w-3xl flex-col overflow-hidden rounded-2xl border border-secondary bg-surface-300 shadow-2xl" role="dialog" aria-modal="true" aria-label={local.label ?? 'Search'}>
      <div class="border-b border-secondary px-5 py-4"><div class="flex items-center gap-3"><div class="flex shrink-0 items-center text-300"><SearchIcon size={20} /></div><div class="min-w-0 flex-1"><input ref={input} type="search" role="searchbox" aria-controls={resultsId} class="w-full bg-transparent text-base font-medium text-100 placeholder:text-300/80 focus:outline-none" placeholder={local.placeholder ?? 'Search...'} autocomplete="off" value={query()} onInput={(event) => setQuery(event.currentTarget.value)} onKeyDown={(event) => { if (event.key === 'ArrowDown') { event.preventDefault(); move(1) } else if (event.key === 'ArrowUp') { event.preventDefault(); move(-1) } else if (event.key === 'Enter') { event.preventDefault(); select(event) } else if (event.key === 'Escape') { event.preventDefault(); setOpen(false) } }} /></div></div></div>
      <div class="min-h-0 flex-1 overflow-y-auto overscroll-contain"><Show when={query() !== ''} fallback={<div class="px-5 py-8 text-center"><p class="text-sm text-300">{local.empty ?? 'Start typing to search'}</p></div>}><ul id={resultsId} class="px-2 py-2"><Show when={!local.loading && local.results.length > 0} fallback={local.loading ? <li><Loader class="flex justify-center items-center py-8" /></li> : <CommandEmpty>{local.empty}</CommandEmpty>}><For each={local.results}>{(result, index) => <CommandResult result={result} index={index()} resultId={`${resultsId}-result-${index()}`} highlighted={activeResults()[highlighted()]?.key === result.key} onSelect={(item, event) => local.onSelect?.(item, event)} />}</For></Show></ul></Show></div>
      <div class="border-t border-secondary bg-surface-400/50 px-5 py-3">{local.footer ?? <div class="flex items-center justify-between gap-4"><span class="text-[11px] text-300">↑↓ Navigate · Enter Open · Esc Close</span></div>}</div>
    </div></div></Show>
  </div>
}
