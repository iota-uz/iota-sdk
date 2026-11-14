import { createEffect, createSignal, onCleanup, onMount, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'
import { Button } from '../forms/Button'

export interface DialogProps extends Omit<JSX.DialogHtmlAttributes<HTMLDialogElement>, 'open' | 'onClose' | 'onCancel'> {
  open?: boolean
  defaultOpen?: boolean
  onOpenChange?: (open: boolean) => void
  onEscape?: () => void
  initialFocus?: () => HTMLElement | undefined
  canonicalClass?: string
}

const focusableSelector = 'a[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])'

const headingSelector = '[data-dialog-title]'

// Below md a focused control opens the on-screen keyboard, so the heading takes focus (as in the templ dialog).
function compactViewport(element: HTMLElement) {
  const view = element.ownerDocument.defaultView
  return typeof view?.matchMedia === 'function' && view.matchMedia('(max-width: 767px)').matches
}

export function Dialog(props: DialogProps) {
  const [local, native] = splitProps(props, ['open', 'defaultOpen', 'onOpenChange', 'onEscape', 'initialFocus', 'canonicalClass', 'class', 'children', 'ref', 'onKeyDown'])
  const [internal, setInternal] = createSignal(local.defaultOpen ?? false)
  const expanded = () => local.open ?? internal()
  let element!: HTMLDialogElement
  let restoreFocus: HTMLElement | null = null
  const setOpen = (open: boolean) => {
    if (local.open === undefined) setInternal(open)
    local.onOpenChange?.(open)
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
  onMount(sync)
  onCleanup(() => restoreFocus?.focus())
  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key === 'Escape') {
      event.preventDefault()
      local.onEscape?.()
      setOpen(false)
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
      onCancel={(event) => { event.preventDefault(); local.onEscape?.(); setOpen(false) }}
      onClose={() => {
        if (!expanded()) return
        setOpen(false)
        queueMicrotask(() => { if (element.isConnected) sync() })
      }}
      onKeyDown={onKeyDown}
    >{local.children}</dialog>
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
  const close = () => setOpen(false)
  return (
    <div>
      <Dialog {...dialogProps} open={expanded()} onOpenChange={setOpen}>
        <form method="dialog">
          <header class="flex items-center justify-between gap-3 border-b border-subtle py-1 pl-4 pr-2">
            <h3 class="font-medium" data-dialog-title tabindex="-1">{local.heading}</h3>
            <button type="button" value="cancel" class="drawer-close" aria-label={local.closeLabel ?? 'Close'} onClick={() => { local.onCancel?.(); close() }}><CloseIcon /></button>
          </header>
          <article class="py-3 px-4 flex flex-col items-center justify-center gap-2 min-h-36">
            {local.icon && <div class="w-12 h-12 bg-red-500/10 rounded-full flex items-center justify-center text-red-500">{local.icon}</div>}
            {local.text && <p class="text-center">{local.text}</p>}
            {local.children}
          </article>
          <footer class="px-4 py-3 pb-[max(0.75rem,env(safe-area-inset-bottom))]"><menu class="flex flex-col-reverse gap-3 sm:flex-row">
            <Button type="button" variant="secondary" class="min-h-11 flex-1 justify-center" value="cancel" data-test-id="dialog-cancel-btn" onClick={(event) => { event.preventDefault(); local.onCancel?.(); close() }}>{local.cancelText ?? 'Cancel'}</Button>
            <Button type="button" variant={local.danger ? 'danger' : 'primary'} class="min-h-11 flex-1 justify-center" value="confirm" data-test-id="dialog-confirm-btn" onClick={(event) => { event.preventDefault(); local.onConfirm?.(); close() }}>{local.confirmText ?? 'Confirm'}</Button>
          </menu></footer>
        </form>
      </Dialog>
    </div>
  )
}
