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
        if (element.isConnected && expanded()) (local.initialFocus?.() ?? element.querySelector<HTMLElement>(focusableSelector) ?? element).focus()
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
  return <svg xmlns="http://www.w3.org/2000/svg" aria-hidden="true" width="20" height="20" viewBox="0 0 256 256"><rect width="256" height="256" fill="none" /><line x1="160" y1="96" x2="96" y2="160" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" /><line x1="96" y1="96" x2="160" y2="160" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" /><circle cx="128" cy="128" r="96" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16" /></svg>
}

export interface ConfirmationDialogProps extends DialogProps {
  heading: JSX.Element
  text?: JSX.Element
  icon?: JSX.Element
  cancelText?: JSX.Element
  confirmText?: JSX.Element
  closeLabel?: string
  onCancel?: () => void
  onConfirm?: () => void
}

export function ConfirmationDialog(props: ConfirmationDialogProps) {
  const [local, dialogProps] = splitProps(props, ['heading', 'text', 'icon', 'cancelText', 'confirmText', 'closeLabel', 'onCancel', 'onConfirm', 'children', 'open', 'defaultOpen', 'onOpenChange'])
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
          <header class="flex items-center gap-3 justify-between px-4 py-3 border-b border-subtle">
            <h3 class="font-medium">{local.heading}</h3>
            <Button type="button" variant="secondary" size="sm" fixed rounded aria-label={local.closeLabel ?? 'Close'} onClick={() => { local.onCancel?.(); close() }}><CloseIcon /></Button>
          </header>
          <article class="py-3 px-4 flex flex-col items-center justify-center gap-2 min-h-36">
            {local.icon && <div class="w-12 h-12 bg-red-500/10 rounded-full flex items-center justify-center text-red-500">{local.icon}</div>}
            {local.text && <p class="text-center">{local.text}</p>}
            {local.children}
          </article>
          <footer class="px-4 py-3"><menu class="flex gap-3">
            <Button type="button" variant="secondary" class="flex-1 justify-center" value="cancel" data-test-id="dialog-cancel-btn" onClick={(event) => { event.preventDefault(); local.onCancel?.(); close() }}>{local.cancelText ?? 'Cancel'}</Button>
            <Button type="button" variant="primary" class="flex-1 justify-center" value="confirm" data-test-id="dialog-confirm-btn" onClick={(event) => { event.preventDefault(); local.onConfirm?.(); close() }}>{local.confirmText ?? 'Confirm'}</Button>
          </menu></footer>
        </form>
      </Dialog>
    </div>
  )
}
