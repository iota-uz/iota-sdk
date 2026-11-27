import { createContext, createSignal, createUniqueId, For, onCleanup, onMount, splitProps, useContext, type JSX } from 'solid-js'
import { classes } from '../internal/classes'

export type ToastVariant = 'success' | 'error' | 'danger' | 'warning' | 'info'

/**
 * One verb that acts on the toast's subject ("Undo", "Retry"). Clicking it calls
 * `onAction` once and closes the toast; a failure is reported by the caller.
 */
export interface ToastAction {
  label: string
  onAction(): void | Promise<void>
}

/** A toast with an action stays at least this long so the verb can be reached. */
export const TOAST_ACTION_MIN_DURATION = 8000
const DEFAULT_DURATION = 8000

// The class strings below are shared verbatim with the templ toast
// (components/base/toast); the templ-parity VR check compares both renderings.
const regionClass = [
  'fixed z-[100] pointer-events-none flex flex-col gap-3 top-0 right-0 w-full max-w-sm',
  'p-4 pt-[max(1rem,env(safe-area-inset-top))] pr-[max(1rem,env(safe-area-inset-right))]',
  // Phones: almost full width at the bottom, above a fixed bottom action bar that
  // publishes its height as --iota-toast-offset-bottom; the newest toast is lowest.
  'max-[480px]:top-auto max-[480px]:bottom-[var(--iota-toast-offset-bottom,0px)] max-[480px]:max-w-none max-[480px]:flex-col-reverse',
  'max-[480px]:pt-3 max-[480px]:pl-[max(0.75rem,env(safe-area-inset-left))] max-[480px]:pr-[max(0.75rem,env(safe-area-inset-right))] max-[480px]:pb-[max(0.75rem,env(safe-area-inset-bottom))]',
].join(' ')
const surfaceClass = 'flex items-start gap-3 w-full p-4 bg-toast text-toast rounded-lg shadow-lg border border-toast'
const actionClass = 'flex-shrink-0 self-center -my-1 inline-flex items-center min-h-9 [@media(pointer:coarse)]:min-h-11 px-2 rounded-md text-sm font-semibold text-toast-action hover:bg-primary-500/10 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500 cursor-pointer'
const closeClass = 'flex-shrink-0 rounded-md text-toast-icon hover:text-toast transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-500'

export function ToastRegion(props: JSX.HTMLAttributes<HTMLDivElement>) {
  const [local, native] = splitProps(props, ['class', 'children'])
  // Politeness lives on each toast (status vs alert), not on the region.
  return <div {...native} class={classes(regionClass, local.class)} data-toast-region>{local.children}</div>
}

const stroke = { fill: 'none', stroke: 'currentColor', 'stroke-linecap': 'round', 'stroke-linejoin': 'round', 'stroke-width': '16' } as const

// Phosphor "regular" glyphs, identical to the icons the templ toast renders.
function Icon(props: { children: JSX.Element }) {
  return <svg xmlns="http://www.w3.org/2000/svg" aria-hidden="true" viewBox="0 0 256 256" width="20" height="20"><rect width="256" height="256" fill="none" />{props.children}</svg>
}

const iconColor: Record<ToastVariant, string> = {
  success: 'text-toast-success',
  error: 'text-toast-error',
  danger: 'text-toast-error',
  warning: 'text-toast-warning',
  info: 'text-toast-info',
}

function ToastIcon(props: { variant: ToastVariant }) {
  return (
    <div class={classes('flex-shrink-0 w-5 h-5', iconColor[props.variant])}>
      {props.variant === 'success' && <Icon><polyline points="88 136 112 160 168 104" {...stroke} /><circle cx="128" cy="128" r="96" {...stroke} /></Icon>}
      {(props.variant === 'error' || props.variant === 'danger') && <Icon><line x1="160" y1="96" x2="96" y2="160" {...stroke} /><line x1="96" y1="96" x2="160" y2="160" {...stroke} /><circle cx="128" cy="128" r="96" {...stroke} /></Icon>}
      {props.variant === 'warning' && <Icon><path d="M142.41,40.22l87.46,151.87C236,202.79,228.08,216,215.46,216H40.54C27.92,216,20,202.79,26.13,192.09L113.59,40.22C119.89,29.26,136.11,29.26,142.41,40.22Z" {...stroke} /><line x1="128" y1="144" x2="128" y2="104" {...stroke} /><circle cx="128" cy="180" r="12" fill="currentColor" /></Icon>}
      {props.variant === 'info' && <Icon><circle cx="128" cy="128" r="96" {...stroke} /><path d="M120,120a8,8,0,0,1,8,8v40a8,8,0,0,0,8,8" {...stroke} /><circle cx="124" cy="84" r="12" fill="currentColor" /></Icon>}
    </div>
  )
}

function CloseIcon() {
  return <Icon><line x1="200" y1="56" x2="56" y2="200" {...stroke} fill="currentColor" /><line x1="200" y1="200" x2="56" y2="56" {...stroke} fill="currentColor" /></Icon>
}

export interface ToastProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'title'> {
  variant?: ToastVariant
  title?: JSX.Element
  message?: JSX.Element
  /** Milliseconds before the toast closes itself; 0 keeps it until dismissed. With `action` at least 8000. */
  duration?: number
  closeLabel?: string
  action?: ToastAction
  onDismiss?: () => void
}

export interface ToastNotification {
  id?: string
  variant?: ToastVariant
  title?: JSX.Element
  message?: JSX.Element
  duration?: number
  closeLabel?: string
  /** With an action the toast stays at least 8000 ms and pauses while hovered or focused. */
  action?: ToastAction
}

export interface ToastController {
  notify(notification: ToastNotification): string
  remove(id: string): void
  clear(): void
}

const ToastContext = createContext<ToastController>()

export function useToast(): ToastController {
  const controller = useContext(ToastContext)
  if (!controller) throw new Error('useToast must be used inside ToastProvider')
  return controller
}

export interface ToastProviderProps extends JSX.HTMLAttributes<HTMLDivElement> {
  max?: number
  defaultDuration?: number
  notifyEvent?: string
  removeEvent?: string
  children?: JSX.Element
}

export function ToastProvider(props: ToastProviderProps) {
  const [local, regionProps] = splitProps(props, ['max', 'defaultDuration', 'notifyEvent', 'removeEvent', 'children'])
  const prefix = createUniqueId()
  const [notifications, setNotifications] = createSignal<(ToastNotification & { id: string })[]>([])
  let sequence = 0
  let region!: HTMLDivElement
  const controller: ToastController = {
    notify(notification) {
      const id = notification.id ?? `${prefix}-toast-${sequence++}`
      setNotifications((current) => [{ ...notification, id }, ...current.filter((item) => item.id !== id)].slice(0, local.max ?? 20))
      return id
    },
    remove(id) { setNotifications((current) => current.filter((item) => item.id !== id)) },
    clear() { setNotifications([]) },
  }
  onMount(() => {
    const owner = region.ownerDocument.defaultView
    if (!owner) return
    const notify = (event: Event) => controller.notify((event as CustomEvent<ToastNotification>).detail ?? {})
    const remove = (event: Event) => controller.remove(String((event as CustomEvent<string>).detail))
    owner.addEventListener(local.notifyEvent ?? 'notify', notify)
    owner.addEventListener(local.removeEvent ?? 'remove-notification', remove)
    onCleanup(() => {
      owner.removeEventListener(local.notifyEvent ?? 'notify', notify)
      owner.removeEventListener(local.removeEvent ?? 'remove-notification', remove)
    })
  })
  return <ToastContext.Provider value={controller}>{local.children}<ToastRegion {...regionProps} ref={region}><For each={notifications()}>{(notification) => <Toast variant={notification.variant} title={notification.title} message={notification.message} duration={notification.duration ?? local.defaultDuration} closeLabel={notification.closeLabel} action={notification.action} onDismiss={() => controller.remove(notification.id)} />}</For></ToastRegion></ToastContext.Provider>
}

export const ToastContainer = ToastProvider

/** Effective auto-dismiss delay: 0 never dismisses, an action keeps the toast at least 8 s. */
export function toastDuration(duration: number | undefined, hasAction: boolean): number {
  const base = duration ?? DEFAULT_DURATION
  if (base <= 0) return 0
  return hasAction ? Math.max(base, TOAST_ACTION_MIN_DURATION) : base
}

export function Toast(props: ToastProps) {
  const [local, native] = splitProps(props, ['variant', 'title', 'message', 'duration', 'closeLabel', 'action', 'onDismiss', 'class', 'children', 'onMouseEnter', 'onMouseLeave', 'onFocusIn', 'onFocusOut'])
  const variant = () => local.variant ?? 'info'
  const polite = () => variant() === 'success' || variant() === 'info'
  let remaining = toastDuration(local.duration, Boolean(local.action))
  let timer: ReturnType<typeof setTimeout> | undefined
  let started = 0
  let hovered = false
  let focused = false
  let dismissed = false
  const dismiss = () => {
    if (dismissed) return
    dismissed = true
    clear()
    local.onDismiss?.()
  }
  const clear = () => { if (timer !== undefined) clearTimeout(timer); timer = undefined }
  const sync = () => {
    if (hovered || focused) {
      if (timer !== undefined) remaining = Math.max(0, remaining - (Date.now() - started))
      clear()
      return
    }
    if (timer !== undefined || remaining <= 0 || dismissed) return
    started = Date.now()
    timer = setTimeout(dismiss, remaining)
  }
  onMount(sync)
  onCleanup(clear)
  const runAction = () => {
    const action = local.action
    if (!action || dismissed) return
    try {
      void action.onAction()
    } finally {
      dismiss()
    }
  }
  return (
    <div
      {...native}
      class={classes('pointer-events-auto', local.class)}
      onMouseEnter={(event) => { hovered = true; sync(); if (typeof local.onMouseEnter === 'function') local.onMouseEnter(event) }}
      onMouseLeave={(event) => { hovered = false; sync(); if (typeof local.onMouseLeave === 'function') local.onMouseLeave(event) }}
      onFocusIn={(event) => { focused = true; sync(); if (typeof local.onFocusIn === 'function') local.onFocusIn(event) }}
      onFocusOut={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) { focused = false; sync() }
        if (typeof local.onFocusOut === 'function') local.onFocusOut(event)
      }}
    >
      <div class={surfaceClass} role={polite() ? 'status' : 'alert'} aria-live={polite() ? 'polite' : undefined} data-toast data-variant={variant()}>
        <ToastIcon variant={variant()} />
        <div class="flex-1 min-w-0">
          {local.title && <p class="text-sm font-semibold text-toast">{local.title}</p>}
          {local.message && <p class="text-sm text-toast-muted mt-1">{local.message}</p>}
          {local.children}
        </div>
        {local.action && <button type="button" class={actionClass} data-toast-action onClick={runAction}>{local.action.label}</button>}
        <button type="button" class={closeClass} aria-label={local.closeLabel ?? 'Close notification'} onClick={dismiss}><CloseIcon /></button>
      </div>
    </div>
  )
}
