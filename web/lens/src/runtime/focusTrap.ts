import { useEffect, type RefObject } from 'react'

const focusableSelector = [
  'a[href]', 'button:not([disabled])', 'input:not([disabled])', 'select:not([disabled])',
  'textarea:not([disabled])', '[tabindex]:not([tabindex="-1"])',
].join(',')

/** Keeps keyboard focus within an open dialog and restores its trigger. */
export function useFocusTrap(
  dialog: RefObject<HTMLElement>,
  active: boolean,
  onEscape: () => void,
  initialFocus?: RefObject<HTMLElement>,
  restoreFocus?: RefObject<HTMLElement>,
) {
  useEffect(() => {
    if (!active || typeof document === 'undefined') return undefined
    const restoreTarget = restoreFocus?.current
    const focusFrame = requestAnimationFrame(() => (initialFocus?.current ?? dialog.current)?.focus())
    const keydown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        event.stopPropagation()
        onEscape()
        return
      }
      if (event.key !== 'Tab' || !dialog.current) return
      const elements = [...dialog.current.querySelectorAll<HTMLElement>(focusableSelector)]
        .filter((element) => element.offsetParent !== null || element === document.activeElement)
      if (elements.length === 0) {
        event.preventDefault()
        dialog.current.focus()
        return
      }
      const first = elements[0]!
      const last = elements[elements.length - 1]!
      if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first.focus()
      } else if (event.shiftKey && (document.activeElement === first || document.activeElement === dialog.current)) {
        event.preventDefault()
        last.focus()
      }
    }
    document.addEventListener('keydown', keydown, true)
    return () => {
      cancelAnimationFrame(focusFrame)
      document.removeEventListener('keydown', keydown, true)
      restoreTarget?.focus()
    }
  }, [active, dialog, initialFocus, onEscape, restoreFocus])
}
