import { createSignal, onCleanup, Show, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'

export type CopyButtonVariant = 'default' | 'minimal'

export interface CopyButtonLabels {
  copy: string
  copied: string
}

export interface CopyButtonProps extends Omit<JSX.ButtonHTMLAttributes<HTMLButtonElement>, 'onCopy'> {
  text: string
  size?: number | string
  variant?: CopyButtonVariant
  showText?: boolean
  copied?: boolean
  defaultCopied?: boolean
  resetMs?: number
  labels?: Partial<CopyButtonLabels>
  copy?: (text: string) => Promise<void>
  onCopiedChange?: (copied: boolean) => void
  onCopyError?: (error: unknown) => void
}

const defaultLabels: CopyButtonLabels = { copy: 'Copy', copied: 'Copied!' }

function CopyIcon(props: { size: number | string }) {
  return <svg xmlns="http://www.w3.org/2000/svg" aria-hidden="true" width={props.size} height={props.size} viewBox="0 0 256 256"><rect width="256" height="256" fill="none" /><polyline points="168 168 216 168 216 40 88 40 88 88" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" /><rect x="40" y="88" width="128" height="128" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" /></svg>
}

function CheckCircleIcon(props: { size: number | string }) {
  return <svg xmlns="http://www.w3.org/2000/svg" aria-hidden="true" width={props.size} height={props.size} viewBox="0 0 256 256"><rect width="256" height="256" fill="none" /><polyline points="88 136 112 160 168 104" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" /><circle cx="128" cy="128" r="96" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" /></svg>
}

export function CopyButton(props: CopyButtonProps) {
  const [local, native] = splitProps(props, ['text', 'size', 'variant', 'showText', 'copied', 'defaultCopied', 'resetMs', 'labels', 'copy', 'onCopiedChange', 'onCopyError', 'class', 'onClick'])
  const [internalCopied, setInternalCopied] = createSignal(local.defaultCopied ?? false)
  let timer: ReturnType<typeof setTimeout> | undefined
  let alive = true
  const copied = () => local.copied ?? internalCopied()
  const labels = () => ({ ...defaultLabels, ...local.labels })
  const size = () => local.size ?? 16
  const setCopied = (value: boolean) => {
    if (local.copied === undefined) setInternalCopied(value)
    local.onCopiedChange?.(value)
  }
  const copy = async () => {
    try {
      await (local.copy ? local.copy(local.text) : navigator.clipboard.writeText(local.text))
      if (!alive) return
      setCopied(true)
      clearTimeout(timer)
      timer = setTimeout(() => setCopied(false), local.resetMs ?? 2000)
    } catch (error) {
      if (alive) local.onCopyError?.(error)
    }
  }
  onCleanup(() => { alive = false; clearTimeout(timer) })
  return (
    <button
      {...native}
      type="button"
      class={classes(
        'inline-flex items-center gap-1.5 transition-colors duration-150 text-gray-300 hover:text-gray-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-1 rounded cursor-pointer',
        (local.variant ?? 'default') === 'default' ? 'bg-gray-100 hover:bg-gray-200 px-2 py-1' : 'hover:bg-gray-100 p-1',
        copied() && 'text-green-600', local.class,
      )}
      data-copy-text={local.text}
      aria-label={native['aria-label'] ?? labels().copy}
      onClick={(event) => { void copy(); if (typeof local.onClick === 'function') local.onClick(event) }}
    >
      <span class="relative inline-block shrink-0" style={{ width: `${size()}px`, height: `${size()}px` }}>
        <span class={classes('absolute inset-0 flex items-center justify-center transition-all duration-150 ease-out', copied() ? 'opacity-0 scale-50' : 'opacity-100 scale-100')}><CopyIcon size={size()} /></span>
        <span class={classes('absolute inset-0 flex items-center justify-center transition-all duration-200 ease-out', copied() ? 'opacity-100 scale-100' : 'opacity-0 scale-50')}><CheckCircleIcon size={size()} /></span>
      </span>
      <Show when={local.showText}><span class="text-xs font-medium">{copied() ? labels().copied : labels().copy}</span></Show>
    </button>
  )
}

export interface CopyableTextProps extends Omit<JSX.HTMLAttributes<HTMLSpanElement>, 'onCopy'> {
  text?: string
  label?: string
  size?: number | string
  copied?: boolean
  defaultCopied?: boolean
  resetMs?: number
  copy?: (text: string) => Promise<void>
  onCopiedChange?: (copied: boolean) => void
  onCopyError?: (error: unknown) => void
}

export function CopyableText(props: CopyableTextProps) {
  const [local, native] = splitProps(props, ['text', 'label', 'size', 'copied', 'defaultCopied', 'resetMs', 'copy', 'onCopiedChange', 'onCopyError', 'class', 'onClick', 'onKeyDown'])
  const [internalCopied, setInternalCopied] = createSignal(local.defaultCopied ?? false)
  let timer: ReturnType<typeof setTimeout> | undefined
  let alive = true
  const copied = () => local.copied ?? internalCopied()
  const size = () => local.size ?? 16
  const run = async () => {
    if (!local.text) return
    try {
      await (local.copy ? local.copy(local.text) : navigator.clipboard.writeText(local.text))
      if (!alive) return
      if (local.copied === undefined) setInternalCopied(true)
      local.onCopiedChange?.(true)
      clearTimeout(timer)
      timer = setTimeout(() => {
        if (local.copied === undefined) setInternalCopied(false)
        local.onCopiedChange?.(false)
      }, local.resetMs ?? 1500)
    } catch (error) { if (alive) local.onCopyError?.(error) }
  }
  onCleanup(() => { alive = false; clearTimeout(timer) })
  if (!local.text) return <>-</>
  return (
    <span
      {...native}
      role="button"
      tabindex="0"
      class={classes('inline-flex items-center gap-1 cursor-pointer group whitespace-nowrap max-w-full focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 rounded', local.class)}
      data-copy-text={local.text}
      title={local.label}
      aria-label={local.label}
      onClick={(event) => { event.stopPropagation(); void run(); if (typeof local.onClick === 'function') local.onClick(event) }}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); event.stopPropagation(); void run() }
        if (typeof local.onKeyDown === 'function') local.onKeyDown(event)
      }}
    >
      <span class="truncate">{local.text}</span>
      <span class="relative inline-block shrink-0" style={{ width: `${size()}px`, height: `${size()}px` }}>
        <span class={classes('absolute inset-0 flex items-center justify-center text-current opacity-0 transition-all duration-150 ease-out group-hover:opacity-100', copied() ? '!opacity-0 scale-50' : 'scale-100')}><CopyIcon size={size()} /></span>
        <span class={classes('absolute inset-0 flex items-center justify-center text-green-500 opacity-0 scale-50 transition-all duration-200 ease-out', copied() && 'opacity-100 scale-100')}><CheckCircleIcon size={size()} /></span>
      </span>
    </span>
  )
}
