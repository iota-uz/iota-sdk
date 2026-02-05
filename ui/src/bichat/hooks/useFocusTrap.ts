import { useEffect, RefObject } from 'react'

/**
 * Hook to trap focus within a container (for modals, sidebars)
 * Ensures Tab and Shift+Tab cycle through focusable elements only
 *
 * @param containerRef - React ref to the container element
 * @param isActive - Whether the focus trap is currently active
 * @param restoreFocusOnDeactivate - Element to restore focus to when deactivated
 *
 * @example
 * const modalRef = useRef<HTMLDivElement>(null)
 * useFocusTrap(modalRef, isOpen)
 */
export function useFocusTrap(
  containerRef: RefObject<HTMLElement | null>,
  isActive: boolean,
  restoreFocusOnDeactivate?: HTMLElement | null
) {
  useEffect(() => {
    if (!isActive || !containerRef.current) return

    const container = containerRef.current
    const previouslyFocused = document.activeElement as HTMLElement

    // Get all focusable elements
    const getFocusableElements = (): HTMLElement[] => {
      const selector = [
        'button:not([disabled])',
        '[href]',
        'input:not([disabled])',
        'select:not([disabled])',
        'textarea:not([disabled])',
        '[tabindex]:not([tabindex="-1"])',
      ].join(', ')

      return Array.from(container.querySelectorAll(selector)) as HTMLElement[]
    }

    // Focus first element on activation
    const focusableElements = getFocusableElements()
    if (focusableElements.length > 0) {
      focusableElements[0].focus()
    }

    // Handle Tab key to cycle focus
    const handleTabKey = (e: KeyboardEvent) => {
      if (e.key !== 'Tab') return

      const focusableElements = getFocusableElements()
      if (focusableElements.length === 0) return

      const firstElement = focusableElements[0]
      const lastElement = focusableElements[focusableElements.length - 1]

      if (e.shiftKey) {
        // Shift+Tab: cycle backwards
        if (document.activeElement === firstElement) {
          e.preventDefault()
          lastElement.focus()
        }
      } else {
        // Tab: cycle forwards
        if (document.activeElement === lastElement) {
          e.preventDefault()
          firstElement.focus()
        }
      }
    }

    container.addEventListener('keydown', handleTabKey)

    // Cleanup and restore focus
    return () => {
      container.removeEventListener('keydown', handleTabKey)

      // Restore focus to previously focused element or custom element
      if (restoreFocusOnDeactivate) {
        restoreFocusOnDeactivate.focus()
      } else if (previouslyFocused instanceof HTMLElement) {
        previouslyFocused.focus()
      }
    }
  }, [containerRef, isActive, restoreFocusOnDeactivate])
}
