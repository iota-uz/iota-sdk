import { createEffect, onCleanup, type Accessor } from 'solid-js'

const focusableSelector = [
  'a[href]', 'button:not([disabled])', 'input:not([disabled])', 'select:not([disabled])',
  'textarea:not([disabled])', '[tabindex]:not([tabindex="-1"])',
].join(',')

// Only the most recently opened trap answers Tab and Escape: a confirmation
// over an editor closes on its own, never together with the editor below.
const openTraps: symbol[] = []

/** Keeps keyboard focus within an open dialog and restores its trigger. */
export function useFocusTrap(
  dialog: () => HTMLElement | null | undefined,
  active: boolean | Accessor<boolean>,
  onEscape: () => void,
  initialFocus?: () => HTMLElement | null | undefined,
  restoreFocus?: () => HTMLElement | null | undefined,
): void {
  createEffect(() => {
    const isActive = typeof active === 'function' ? active() : active
    if (!isActive || typeof document === 'undefined') return
    const restoreTarget = restoreFocus?.()
    const token = Symbol('focus-trap')
    openTraps.push(token)
    const focusFrame = requestAnimationFrame(() => (initialFocus?.() ?? dialog())?.focus())
    const keydown = (event: KeyboardEvent) => {
      if (openTraps[openTraps.length - 1] !== token) return
      if (event.key === 'Escape') {
        event.preventDefault()
        event.stopPropagation()
        onEscape()
        return
      }
      if (event.key !== 'Tab' || !dialog()) return
      const elements = [...dialog()!.querySelectorAll<HTMLElement>(focusableSelector)]
        .filter((element) => element.offsetParent !== null || element === document.activeElement)
      if (elements.length === 0) {
        event.preventDefault()
        dialog()!.focus()
        return
      }
      const first = elements[0]!
      const last = elements[elements.length - 1]!
      if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first.focus()
      } else if (event.shiftKey && (document.activeElement === first || document.activeElement === dialog())) {
        event.preventDefault()
        last.focus()
      }
    }
    document.addEventListener('keydown', keydown, true)
    onCleanup(() => {
      cancelAnimationFrame(focusFrame)
      document.removeEventListener('keydown', keydown, true)
      openTraps.splice(openTraps.indexOf(token), 1)
      restoreTarget?.focus({ preventScroll: true })
    })
  })
}
