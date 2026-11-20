import { createContext, createEffect, createSignal, onCleanup, onMount, Show, splitProps, useContext, type JSX } from 'solid-js'
import { classes } from '../internal/classes'
import { Button } from '../forms/Button'

export interface DialogProps extends Omit<JSX.DialogHtmlAttributes<HTMLDialogElement>, 'open' | 'onClose' | 'onCancel'> {
  open?: boolean
  defaultOpen?: boolean
  onOpenChange?: (open: boolean) => void
  onEscape?: () => void
  initialFocus?: () => HTMLElement | undefined
  canonicalClass?: string
  /**
   * A request started from the layer is running. Every close (✕, Esc, backdrop,
   * Cancel) is ignored and the layer carries `aria-busy="true"`. Its content is
   * dimmed and takes no pointer input, except the loading action
   * (`aria-busy="true"`, as Button `loading` sets it), close controls marked
   * `data-dialog-close` and the status line. A close attempt shows `busyMessage`.
   */
  busy?: boolean
  /** Status line shown when someone tries to close a busy layer; by default a built-in text in the host's `lang` (en, ru, uz, uz-Cyrl). */
  busyMessage?: JSX.Element
  /** Clicking outside the panel (the backdrop) closes the layer, as in the templ dialog. Default true. */
  dismissOnBackdrop?: boolean
}

/** Built-in status line of a busy layer, by language; hosts may pass `busyMessage` instead. */
export const DEFAULT_BUSY_MESSAGES = {
  en: 'Please wait: the request is still running and will not be sent twice.',
  ru: 'Подождите: запрос ещё выполняется и не будет отправлен повторно.',
  uz: 'Kuting: soʻrov hali bajarilmoqda va qayta yuborilmaydi.',
  'uz-Cyrl': 'Кутинг: сўров ҳали бажарилмоқда ва қайта юборилмайди.',
} as const

export const DEFAULT_BUSY_MESSAGE = DEFAULT_BUSY_MESSAGES.en

/** The built-in busy message for a BCP 47 language tag (the host's `lang`); English when unknown. */
export function defaultBusyMessage(lang: string | null | undefined): string {
  const tag = (lang ?? '').toLowerCase()
  if (tag.startsWith('ru')) return DEFAULT_BUSY_MESSAGES.ru
  if (tag.startsWith('uz-cyrl')) return DEFAULT_BUSY_MESSAGES['uz-Cyrl']
  if (tag.startsWith('uz')) return DEFAULT_BUSY_MESSAGES.uz
  return DEFAULT_BUSY_MESSAGES.en
}

export interface DialogControl {
  /** True while the layer is busy. */
  busy(): boolean
  /** True after a close was refused because the layer is busy; resets when busy clears. */
  closeRefused(): boolean
  busyMessage(): JSX.Element
  /** Closes the layer unless it is busy; returns whether it closed. */
  requestClose(): boolean
  /** @internal the status line registers itself so the dialog does not render a second one. */
  registerStatus(): () => void
}

const DialogContext = createContext<DialogControl>()

/** Close control of the enclosing Dialog/Drawer, for custom close buttons inside it. */
export function useDialog(): DialogControl | undefined {
  return useContext(DialogContext)
}

/**
 * Reserved status line (`role="status"`) that explains why a busy layer did not
 * close. Place it above the layer's buttons; its height is reserved so the
 * layout does not shift. A Dialog without one renders it after its content.
 */
export function DialogBusyStatus(props: { class?: string }) {
  return <BusyStatusLine class={props.class} register />
}

const statusRowClass = 'col-start-1 row-start-1 flex min-h-5 items-center gap-2'
const statusDot = () => <span aria-hidden="true" class="h-1.5 w-1.5 shrink-0 rounded-full bg-primary-500" />

function BusyStatusLine(props: { class?: string; register?: boolean }) {
  const control = useDialog()
  if (control && props.register) onCleanup(control.registerStatus())
  const visible = () => Boolean(control?.busy() && control.closeRefused())
  // The hidden copy of the message holds the line at its final height (also when
  // it wraps on a phone), so showing the message never moves the buttons.
  return (
    <div class={classes('grid text-sm text-200', props.class)} data-dialog-busy-status>
      <p aria-hidden="true" class={classes(statusRowClass, 'invisible')}>{statusDot()}{control?.busyMessage()}</p>
      <p role="status" class={statusRowClass}><Show when={visible()}>{statusDot()}{control?.busyMessage()}</Show></p>
    </div>
  )
}

const focusableSelector = 'a[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])'

const headingSelector = '[data-dialog-title]'

// Below md a focused control opens the on-screen keyboard, so the heading takes focus (as in the templ dialog).
function compactViewport(element: HTMLElement) {
  const view = element.ownerDocument.defaultView
  return typeof view?.matchMedia === 'function' && view.matchMedia('(max-width: 767px)').matches
}

export function Dialog(props: DialogProps) {
  const [local, native] = splitProps(props, ['open', 'defaultOpen', 'onOpenChange', 'onEscape', 'initialFocus', 'canonicalClass', 'busy', 'busyMessage', 'dismissOnBackdrop', 'class', 'children', 'ref', 'onKeyDown', 'onPointerDown', 'onClick'])
  const [internal, setInternal] = createSignal(local.defaultOpen ?? false)
  const [closeRefused, setCloseRefused] = createSignal(false)
  const [statusSlots, setStatusSlots] = createSignal(0)
  const expanded = () => local.open ?? internal()
  const busy = () => Boolean(local.busy)
  const [lang, setLang] = createSignal<string | null>(null)
  let element!: HTMLDialogElement
  let restoreFocus: HTMLElement | null = null
  let backdropPress = false
  const setOpen = (open: boolean) => {
    if (local.open === undefined) setInternal(open)
    local.onOpenChange?.(open)
  }
  createEffect(() => { if (!busy()) setCloseRefused(false) })
  // Every close path goes through here, so a busy layer refuses all of them alike.
  const requestClose = (onClosing?: () => void) => {
    if (busy()) { setCloseRefused(true); return false }
    onClosing?.()
    setOpen(false)
    return true
  }
  const control: DialogControl = {
    busy,
    closeRefused,
    busyMessage: () => local.busyMessage ?? defaultBusyMessage(lang()),
    requestClose: () => requestClose(),
    registerStatus: () => {
      setStatusSlots((count) => count + 1)
      return () => setStatusSlots((count) => count - 1)
    },
  }
  const sync = () => {
    if (!element) return
    if (expanded() && !element.open) {
      const active = element.ownerDocument.activeElement
      restoreFocus = active && 'focus' in active ? active as HTMLElement : null
      if (typeof element.showModal === 'function') element.showModal()
      else element.open = true
      queueMicrotask(() => {
        if (!element.isConnected || !expanded()) return
        const preferred = local.initialFocus?.()
        if (preferred) { preferred.focus(); return }
        // Below md the heading takes focus: showModal() skips it (tabindex=-1) and would focus the first control.
        const heading = compactViewport(element) ? element.querySelector<HTMLElement>(headingSelector) : null
        if (heading) { heading.focus({ preventScroll: true }); return }
        // showModal() already moved focus into the dialog; a second script focus() would
        // paint a focus ring on the first control, which the templ dialog does not have.
        if (element.contains(element.ownerDocument.activeElement)) return
        ;(element.querySelector<HTMLElement>(focusableSelector) ?? element).focus()
      })
    } else if (!expanded() && element.open) {
      if (typeof element.close === 'function') element.close()
      else element.open = false
      restoreFocus?.focus()
      restoreFocus = null
    }
  }
  createEffect(sync)
  onMount(() => {
    setLang(element.closest('[lang]')?.getAttribute('lang') ?? element.ownerDocument.documentElement.lang)
    sync()
  })
  onCleanup(() => restoreFocus?.focus())
  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key === 'Escape') {
      event.preventDefault()
      requestClose(local.onEscape)
    } else if (event.key === 'Tab') {
      const focusable = Array.from(element.querySelectorAll<HTMLElement>(focusableSelector))
      const first = focusable[0]
      const last = focusable.at(-1)
      if (focusable.length === 0) { event.preventDefault(); element.focus() }
      else if (event.shiftKey && element.ownerDocument.activeElement === first) { event.preventDefault(); last?.focus() }
      else if (!event.shiftKey && element.ownerDocument.activeElement === last) { event.preventDefault(); first?.focus() }
    }
    if (typeof local.onKeyDown === 'function') (local.onKeyDown as (event: KeyboardEvent) => void)(event)
  }
  return (
    <dialog
      {...native}
      ref={(node) => { element = node; if (typeof local.ref === 'function') local.ref(node) }}
      class={classes(local.canonicalClass ?? 'dialog dialog-rounded dialog-btt shadow-lg mb-0 rounded-b-none md:mb-auto md:rounded-b-lg', local.class)}
      aria-modal="true"
      aria-busy={busy() || undefined}
      data-busy={busy() || undefined}
      onCancel={(event) => { event.preventDefault(); requestClose(local.onEscape) }}
      onClose={() => {
        if (!expanded()) return
        // A native close (form method="dialog") of a busy layer is undone by sync().
        if (!busy()) setOpen(false)
        else setCloseRefused(true)
        queueMicrotask(() => { if (element.isConnected) sync() })
      }}
      onPointerDown={(event) => {
        backdropPress = event.target === element
        if (typeof local.onPointerDown === 'function') local.onPointerDown(event)
      }}
      onClick={(event) => {
        // Only a press that started and ended on the backdrop (the dialog element
        // itself, outside its panel) dismisses; a drag out of a field does not.
        const fromBackdrop = backdropPress && event.target === element
        backdropPress = false
        if (fromBackdrop && local.dismissOnBackdrop !== false) requestClose()
        if (typeof local.onClick === 'function') local.onClick(event)
      }}
      onKeyDown={onKeyDown}
    >
      <DialogContext.Provider value={control}>
        {local.children}
        <Show when={local.busy !== undefined && statusSlots() === 0}><BusyStatusLine class="px-4 pb-3" /></Show>
      </DialogContext.Provider>
    </dialog>
  )
}

function CloseIcon() {
  return <svg xmlns="http://www.w3.org/2000/svg" aria-hidden="true" width="20" height="20" viewBox="0 0 256 256"><rect width="256" height="256" fill="none" /><line x1="200" y1="56" x2="56" y2="200" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" /><line x1="200" y1="200" x2="56" y2="56" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" /></svg>
}

export interface ConfirmationDialogProps extends DialogProps {
  heading: JSX.Element
  text?: JSX.Element
  icon?: JSX.Element
  cancelText?: JSX.Element
  confirmText?: JSX.Element
  closeLabel?: string
  /** Renders the confirm button as destructive. */
  danger?: boolean
  onCancel?: () => void
  /**
   * Runs on confirm. When it sets `busy` (the request started), the dialog stays
   * open with the confirm button loading; the owner closes it when the request ends.
   */
  onConfirm?: () => void
}

export function ConfirmationDialog(props: ConfirmationDialogProps) {
  const [local, dialogProps] = splitProps(props, ['heading', 'text', 'icon', 'cancelText', 'confirmText', 'closeLabel', 'danger', 'onCancel', 'onConfirm', 'children', 'open', 'defaultOpen', 'onOpenChange'])
  const [internal, setInternal] = createSignal(local.defaultOpen ?? false)
  const expanded = () => local.open ?? internal()
  const setOpen = (open: boolean) => {
    if (local.open === undefined) setInternal(open)
    local.onOpenChange?.(open)
  }
  return (
    <div>
      <Dialog {...dialogProps} open={expanded()} onOpenChange={setOpen}>
        <ConfirmationBody {...local} busy={Boolean(dialogProps.busy)} reserveStatus={dialogProps.busy !== undefined} />
      </Dialog>
    </div>
  )
}

function ConfirmationBody(props: Pick<ConfirmationDialogProps, 'heading' | 'text' | 'icon' | 'cancelText' | 'confirmText' | 'closeLabel' | 'danger' | 'onCancel' | 'onConfirm' | 'children'> & { busy: boolean; reserveStatus: boolean }) {
  const dialog = useDialog()!
  const cancel = () => {
    if (dialog.busy()) { dialog.requestClose(); return }
    props.onCancel?.()
    dialog.requestClose()
  }
  const confirm = () => {
    if (dialog.busy()) return
    props.onConfirm?.()
    // onConfirm may have started a request and turned the dialog busy: keep it open then.
    if (!dialog.busy()) dialog.requestClose()
  }
  return (
    <form method="dialog">
      <header class="flex items-center justify-between gap-3 border-b border-subtle py-1 pl-4 pr-2">
        <h3 class="font-medium" data-dialog-title tabindex="-1">{props.heading}</h3>
        <button type="button" value="cancel" class="drawer-close" aria-label={props.closeLabel ?? 'Close'} data-dialog-close onClick={cancel}><CloseIcon /></button>
      </header>
      <article class="py-3 px-4 flex flex-col items-center justify-center gap-2 min-h-36" data-dialog-body>
        {props.icon && <div class="w-12 h-12 bg-red-500/10 rounded-full flex items-center justify-center text-red-500">{props.icon}</div>}
        {props.text && <p class="text-center">{props.text}</p>}
        {props.children}
      </article>
      <footer class="px-4 py-3 pb-[max(0.75rem,env(safe-area-inset-bottom))]">
        {props.reserveStatus && <DialogBusyStatus class="mb-2 -mt-1" />}
        <menu class="flex flex-col-reverse gap-3 sm:flex-row">
          <Button type="button" variant="secondary" class="min-h-11 flex-1 justify-center" value="cancel" data-test-id="dialog-cancel-btn" data-dialog-close aria-disabled={props.busy || undefined} onClick={(event) => { event.preventDefault(); cancel() }}>{props.cancelText ?? 'Cancel'}</Button>
          <Button type="button" variant={props.danger ? 'danger' : 'primary'} class="min-h-11 flex-1 justify-center" value="confirm" data-test-id="dialog-confirm-btn" loading={props.busy} onClick={(event) => { event.preventDefault(); confirm() }}>{props.confirmText ?? 'Confirm'}</Button>
        </menu>
      </footer>
    </form>
  )
}
