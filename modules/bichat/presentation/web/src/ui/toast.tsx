import { createContext, createSignal, For, onCleanup, onMount, useContext, type JSX } from 'solid-js'
import { AlertIcon, CheckIcon } from './icons'

interface ToastItem {
  id: number
  kind: 'success' | 'error'
  message: string
}

export interface ToastApi {
  success: (message: string) => void
  error: (message: string) => void
}

const ToastContext = createContext<ToastApi>()

const MAX_TOASTS = 4
const AUTO_DISMISS_MS = 4000

let nextToastId = 1

function ToastCard(props: { toast: ToastItem; onDismiss: () => void }): JSX.Element {
  const [visible, setVisible] = createSignal(false)
  onMount(() => {
    const frame = requestAnimationFrame(() => setVisible(true))
    onCleanup(() => cancelAnimationFrame(frame))
  })
  return (
    <div
      role="status"
      class={`pointer-events-auto flex items-start gap-2.5 rounded-xl px-3.5 py-2.5 text-sm text-white shadow-lg transition-all duration-200 ${
        props.toast.kind === 'error' ? 'bg-red-600' : 'bg-neutral-900'
      } ${visible() ? 'translate-y-0 opacity-100' : 'translate-y-2 opacity-0'}`}
    >
      {props.toast.kind === 'error' ? (
        <AlertIcon class="mt-0.5 h-4 w-4 shrink-0" />
      ) : (
        <CheckIcon class="mt-0.5 h-4 w-4 shrink-0" />
      )}
      <span class="min-w-0 break-words">{props.toast.message}</span>
    </div>
  )
}

export function ToastProvider(props: { children: JSX.Element }): JSX.Element {
  const [toasts, setToasts] = createSignal<ToastItem[]>([])

  const remove = (id: number) => setToasts((list) => list.filter((toast) => toast.id !== id))

  const push = (kind: ToastItem['kind'], message: string) => {
    const id = nextToastId++
    setToasts((list) => [...list.slice(-(MAX_TOASTS - 1)), { id, kind, message }])
    setTimeout(() => remove(id), AUTO_DISMISS_MS)
  }

  const api: ToastApi = {
    success: (message) => push('success', message),
    error: (message) => push('error', message),
  }

  return (
    <ToastContext.Provider value={api}>
      {props.children}
      <div
        class="pointer-events-none fixed right-4 bottom-4 z-50 flex w-80 max-w-[calc(100vw-2rem)] flex-col gap-2"
        aria-live="polite"
      >
        <For each={toasts()}>
          {(toast) => (
            <ToastCard toast={toast} onDismiss={() => remove(toast.id)} />
          )}
        </For>
      </div>
    </ToastContext.Provider>
  )
}

export function useAppToast(): ToastApi {
  const api = useContext(ToastContext)
  if (!api) throw new Error('useAppToast must be used within ToastProvider')
  return api
}
