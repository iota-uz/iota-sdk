import { createSignal, For, onCleanup, onMount, Show, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'

const defaultHelpLabel = 'Open help article'

function InfoIcon(props: { size: number }) {
  return <svg aria-hidden="true" width={props.size} height={props.size} viewBox="0 0 256 256"><circle cx="128" cy="128" r="96" fill="none" stroke="currentColor" stroke-width="16"/><line x1="128" y1="116" x2="128" y2="176" stroke="currentColor" stroke-linecap="round" stroke-width="16"/><circle cx="128" cy="84" r="10" fill="currentColor"/></svg>
}

function QuestionIcon(props: { size: number }) {
  return <svg aria-hidden="true" width={props.size} height={props.size} viewBox="0 0 256 256"><circle cx="128" cy="128" r="96" fill="none" stroke="currentColor" stroke-width="16"/><path d="M96 96a32 32 0 1 1 45 29c-8 4-13 11-13 19" fill="none" stroke="currentColor" stroke-linecap="round" stroke-width="16"/><circle cx="128" cy="184" r="10" fill="currentColor"/></svg>
}

export function helpDocURL(basePath = '/help', path = ''): string {
  const resolvedBase = basePath.trim() || '/help'
  const resolvedPath = path.trim()
  if (!resolvedPath) return resolvedBase
  if (/^(?:https?:\/\/|\/)/.test(resolvedPath)) return resolvedPath
  return `${resolvedBase.replace(/\/$/, '')}/${resolvedPath.startsWith('doc/') ? resolvedPath : `doc/${resolvedPath}`}`
}

export interface HelpLinkProps extends JSX.AnchorHTMLAttributes<HTMLAnchorElement> {
  path?: string
  basePath?: string
  label?: string
  tooltip?: string
  newTab?: boolean
}

export function HelpLink(props: HelpLinkProps) {
  const [local, native] = splitProps(props, ['path', 'basePath', 'label', 'tooltip', 'newTab', 'class', 'href'])
  const label = () => local.label?.trim() || defaultHelpLabel
  return (
    <a
      {...native}
      href={local.href ?? helpDocURL(local.basePath, local.path)}
      aria-label={label()}
      title={local.tooltip?.trim() || label()}
      target={local.newTab ? '_blank' : native.target}
      rel={local.newTab ? 'noopener noreferrer' : native.rel}
      class={classes('inline-flex size-6 shrink-0 items-center justify-center rounded-full border border-default bg-white text-gray-500 shadow-sm transition-colors hover:border-brand hover:bg-blue-50 hover:text-blue-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2', local.class)}
    ><InfoIcon size={16} /><span class="sr-only">{label()}</span></a>
  )
}

export interface HelpContextSection {
  title: string
  items: readonly string[]
}

export interface HelpContextProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'title'> {
  title: JSX.Element
  summary?: JSX.Element
  articlePath?: string
  articleLabel?: JSX.Element
  screenPath?: string
  screenLabel?: JSX.Element
  label?: string
  sections?: readonly HelpContextSection[]
  open?: boolean
  defaultOpen?: boolean
  onOpenChange?: (open: boolean) => void
}

export function HelpContext(props: HelpContextProps) {
  const [local, native] = splitProps(props, ['title', 'summary', 'articlePath', 'articleLabel', 'screenPath', 'screenLabel', 'label', 'sections', 'open', 'defaultOpen', 'onOpenChange', 'class', 'ref'])
  const [internalOpen, setInternalOpen] = createSignal(local.defaultOpen ?? false)
  let root!: HTMLDivElement
  let trigger!: HTMLButtonElement
  const open = () => local.open ?? internalOpen()
  const setOpen = (value: boolean, restoreFocus = false) => {
    if (local.open === undefined) setInternalOpen(value)
    local.onOpenChange?.(value)
    if (!value && restoreFocus) queueMicrotask(() => { if (trigger.isConnected && !open()) trigger.focus() })
  }
  onMount(() => {
    const outside = (event: PointerEvent) => { if (!root.contains(event.target as Node)) setOpen(false) }
    const escape = (event: KeyboardEvent) => { if (event.key === 'Escape' && open()) setOpen(false, true) }
    document.addEventListener('pointerdown', outside)
    document.addEventListener('keydown', escape)
    onCleanup(() => { document.removeEventListener('pointerdown', outside); document.removeEventListener('keydown', escape) })
  })
  return (
    <div {...native} ref={(element) => { root = element; if (typeof local.ref === 'function') local.ref(element) }} class={classes('relative inline-flex shrink-0', local.class)}>
      <button ref={trigger} type="button" class="inline-flex size-8 shrink-0 items-center justify-center rounded-lg border border-default bg-surface-300 text-gray-700 shadow-sm transition-colors hover:border-brand hover:bg-surface-400 hover:text-brand-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2" aria-haspopup="dialog" aria-expanded={open()} aria-label={local.label?.trim() || defaultHelpLabel} onClick={() => setOpen(!open())}>
        <QuestionIcon size={17} /><span class="sr-only">{local.label?.trim() || defaultHelpLabel}</span>
      </button>
      <Show when={open()}>
        <div role="dialog" aria-modal="false" class="absolute left-0 top-full z-[90] mt-2 w-[min(23rem,calc(100vw-2rem))] overflow-hidden rounded-xl border border-subtle bg-surface-300 text-left shadow-xl">
          <div class="border-b border-subtle px-4 py-3.5"><div class="flex items-start gap-3"><div class="mt-0.5 grid size-8 shrink-0 place-items-center rounded-lg border border-brand bg-surface-400 text-brand-500"><QuestionIcon size={17} /></div><div class="min-w-0"><h2 class="text-sm font-semibold leading-5 text-gray-900">{local.title}</h2><Show when={local.summary}><p class="mt-1 text-xs leading-5 text-gray-700">{local.summary}</p></Show></div></div></div>
          <Show when={local.sections?.some((section) => section.items.length)}><div class="grid gap-3 px-4 py-3.5"><For each={local.sections}>{(section) => <Show when={section.items.length}><section><h3 class="text-[11px] font-semibold uppercase tracking-[0.06em] text-gray-700">{section.title}</h3><ul class="mt-1.5 grid gap-1.5"><For each={section.items}>{(item) => <li class="flex gap-2 text-xs leading-5 text-gray-700"><span class="mt-[0.45rem] size-1.5 shrink-0 rounded-full bg-brand-500"/><span>{item}</span></li>}</For></ul></section></Show>}</For></div></Show>
          <div class="flex flex-wrap items-center gap-2 border-t border-subtle bg-surface-400 px-4 py-3">
            <Show when={local.articlePath}><a href={helpDocURL('', local.articlePath)} class="inline-flex min-h-8 items-center gap-1.5 rounded-lg bg-brand-500 px-3 text-xs font-semibold text-white transition-colors hover:bg-brand-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2"><span>{local.articleLabel}</span><span aria-hidden="true">→</span></a></Show>
            <Show when={local.screenPath}><a href={local.screenPath} class="inline-flex min-h-8 items-center gap-1.5 rounded-lg border border-default bg-surface-300 px-3 text-xs font-semibold text-gray-700 transition-colors hover:border-brand hover:text-brand-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2">{local.screenLabel}</a></Show>
          </div>
        </div>
      </Show>
    </div>
  )
}

export interface HelpHintProps extends Omit<JSX.HTMLAttributes<HTMLSpanElement>, 'title'> {
  title: JSX.Element
  description: JSX.Element
  next?: JSX.Element
  articlePath?: string
  articleLabel?: JSX.Element
  label?: string
  open?: boolean
  defaultOpen?: boolean
  onOpenChange?: (open: boolean) => void
}

export function HelpHint(props: HelpHintProps) {
  const [local, native] = splitProps(props, ['title', 'description', 'next', 'articlePath', 'articleLabel', 'label', 'open', 'defaultOpen', 'onOpenChange', 'class', 'ref'])
  const [internalOpen, setInternalOpen] = createSignal(local.defaultOpen ?? false)
  let root!: HTMLSpanElement
  let trigger!: HTMLButtonElement
  const open = () => local.open ?? internalOpen()
  const setOpen = (value: boolean, focus = false) => { if (local.open === undefined) setInternalOpen(value); local.onOpenChange?.(value); if (focus) queueMicrotask(() => { if (trigger.isConnected && !open()) trigger.focus() }) }
  onMount(() => {
    const outside = (event: PointerEvent) => { if (!root.contains(event.target as Node)) setOpen(false) }
    const escape = (event: KeyboardEvent) => { if (event.key === 'Escape' && open()) setOpen(false, true) }
    document.addEventListener('pointerdown', outside); document.addEventListener('keydown', escape)
    onCleanup(() => { document.removeEventListener('pointerdown', outside); document.removeEventListener('keydown', escape) })
  })
  return <span {...native} ref={(element) => { root = element; if (typeof local.ref === 'function') local.ref(element) }} class={classes('relative inline-flex align-middle', local.class)}><button ref={trigger} type="button" class="inline-flex size-5 items-center justify-center rounded-full text-gray-500 transition-colors hover:bg-surface-400 hover:text-brand-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500" aria-haspopup="dialog" aria-expanded={open()} aria-label={local.label?.trim() || String(local.title)} onClick={() => setOpen(!open())}><InfoIcon size={14} /></button><Show when={open()}><span role="dialog" class="absolute left-0 top-full z-[90] mt-1.5 w-[min(19rem,calc(100vw-2rem))] rounded-lg border border-subtle bg-surface-300 p-3 text-left shadow-lg"><strong class="block text-xs font-semibold text-gray-900">{local.title}</strong><span class="mt-1 block text-xs leading-5 text-gray-700">{local.description}</span><Show when={local.next}><span class="mt-2 block border-l-2 border-brand pl-2 text-xs leading-5 text-gray-700">{local.next}</span></Show><Show when={local.articlePath}><a href={helpDocURL('', local.articlePath)} class="mt-2 inline-flex items-center gap-1 text-xs font-semibold text-brand-500 hover:text-brand-600">{local.articleLabel}<span aria-hidden="true">→</span></a></Show></span></Show></span>
}
