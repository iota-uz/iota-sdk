import { createContext, createSignal, createUniqueId, For, onCleanup, onMount, splitProps, useContext, type JSX } from 'solid-js'
import { classes } from '../internal/classes'

export type ToastVariant = 'success' | 'error' | 'danger' | 'warning' | 'info'

export function ToastRegion(props: JSX.HTMLAttributes<HTMLDivElement>) {
  const [local, native] = splitProps(props, ['class', 'children'])
  return <div {...native} class={classes('fixed top-0 right-0 z-[100] w-full max-w-sm p-4 space-y-3 pointer-events-none', local.class)} aria-live={native['aria-live'] ?? 'assertive'}>{local.children}</div>
}

function Icon(props: { children: JSX.Element }) {
  return <svg aria-hidden="true" width="20" height="20" viewBox="0 0 256 256" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16">{props.children}</svg>
}

function ToastIcon(props: { variant: ToastVariant }) {
  return (
    <span aria-hidden="true" class={classes('inline-flex items-center justify-center w-5 h-5', props.variant === 'success' && 'text-green-500', (props.variant === 'error' || props.variant === 'danger') && 'text-red-500', props.variant === 'warning' && 'text-yellow-500', props.variant === 'info' && 'text-blue-500')}>
      {props.variant === 'success' && <Icon><circle cx="128" cy="128" r="96" /><polyline points="172 104 113.3 160 84 132" /></Icon>}
      {(props.variant === 'error' || props.variant === 'danger') && <Icon><circle cx="128" cy="128" r="96" /><line x1="96" y1="96" x2="160" y2="160" /><line x1="160" y1="96" x2="96" y2="160" /></Icon>}
      {props.variant === 'warning' && <Icon><path d="M114.2 40 26.6 192a16 16 0 0 0 13.9 24h175a16 16 0 0 0 13.9-24L141.8 40a16 16 0 0 0-27.6 0Z" /><line x1="128" y1="104" x2="128" y2="144" /><circle cx="128" cy="180" r="8" fill="currentColor" stroke="none" /></Icon>}
      {props.variant === 'info' && <Icon><circle cx="128" cy="128" r="96" /><line x1="128" y1="112" x2="128" y2="176" /><circle cx="128" cy="80" r="8" fill="currentColor" stroke="none" /></Icon>}
    </span>
  )
}

function CloseIcon() {
  return <Icon><line x1="64" y1="64" x2="192" y2="192" /><line x1="192" y1="64" x2="64" y2="192" /></Icon>
}

export interface ToastProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'title'> {
  variant?: ToastVariant
  title?: JSX.Element
  message?: JSX.Element
  duration?: number
  closeLabel?: string
  onDismiss?: () => void
}

export interface ToastNotification {
  id?: string
  variant?: ToastVariant
  title?: JSX.Element
  message?: JSX.Element
  duration?: number
  closeLabel?: string
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
  return <ToastContext.Provider value={controller}>{local.children}<ToastRegion {...regionProps} ref={region}><For each={notifications()}>{(notification) => <Toast variant={notification.variant} title={notification.title} message={notification.message} duration={notification.duration ?? local.defaultDuration} closeLabel={notification.closeLabel} onDismiss={() => controller.remove(notification.id)} />}</For></ToastRegion></ToastContext.Provider>
}

export const ToastContainer = ToastProvider

export function Toast(props: ToastProps) {
  const [local, native] = splitProps(props, ['variant', 'title', 'message', 'duration', 'closeLabel', 'onDismiss', 'class', 'children', 'onMouseEnter', 'onMouseLeave'])
  const variant = () => local.variant ?? 'info'
  const [remaining, setRemaining] = createSignal(local.duration ?? 8000)
  let timer: ReturnType<typeof setTimeout> | undefined
  let started = 0
  const clear = () => { if (timer !== undefined) clearTimeout(timer); timer = undefined }
  const start = () => {
    clear()
    if (remaining() <= 0) return
    started = Date.now()
    timer = setTimeout(() => local.onDismiss?.(), remaining())
  }
  const pause = () => { if (timer !== undefined) setRemaining(Math.max(0, remaining() - (Date.now() - started))); clear() }
  onMount(start)
  onCleanup(clear)
  return (
    <div
      {...native}
      class={classes('pointer-events-auto', local.class)}
      onMouseEnter={(event) => { pause(); if (typeof local.onMouseEnter === 'function') local.onMouseEnter(event) }}
      onMouseLeave={(event) => { start(); if (typeof local.onMouseLeave === 'function') local.onMouseLeave(event) }}
    >
      <div class="flex items-start gap-3 w-full p-4 bg-white dark:bg-neutral-900 rounded-lg shadow-lg border border-subtle" role={variant() === 'success' || variant() === 'info' ? 'status' : 'alert'}>
        <div class="flex-shrink-0 w-5 h-5"><ToastIcon variant={variant()} /></div>
        <div class="flex-1 min-w-0">
          {local.title && <p class="text-sm font-semibold text-neutral-900 dark:text-white">{local.title}</p>}
          {local.message && <p class="text-sm text-neutral-600 dark:text-neutral-400 mt-1">{local.message}</p>}
          {local.children}
        </div>
        <button type="button" class="flex-shrink-0 text-neutral-400 hover:text-neutral-600 dark:hover:text-neutral-200 transition-colors" aria-label={local.closeLabel ?? 'Close notification'} onClick={() => local.onDismiss?.()}><CloseIcon /></button>
      </div>
    </div>
  )
}
