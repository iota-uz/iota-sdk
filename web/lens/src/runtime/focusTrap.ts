import { createEffect, onCleanup } from 'solid-js'

const focusableSelector = [
  'a[href]', 'button:not([disabled])', 'input:not([disabled])', 'select:not([disabled])',
  'textarea:not([disabled])', '[tabindex]:not([tabindex="-1"])',
].join(',')

/** Keeps keyboard focus within an open dialog and restores its trigger. */
export function useFocusTrap(
  dialog: () => HTMLElement | null | undefined,
  active: boolean,
  onEscape: () => void,
  initialFocus?: () => HTMLElement | null | undefined,
  restoreFocus?: () => HTMLElement | null | undefined,
): void {
  createEffect(() => {
    if (!active || typeof document === 'undefined') return
    const restoreTarget = restoreFocus?.()
    const focusFrame = requestAnimationFrame(() => (initialFocus?.() ?? dialog())?.focus())
    const keydown = (event: KeyboardEvent) => {
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
      restoreTarget?.focus()
    })
  })
}
