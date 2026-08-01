import { useCallback, useEffect, useRef, type ReactNode, type RefObject } from 'react'
import { createPortal } from 'react-dom'
import { useOverlayContainer } from '../runtime/overlayContainer'

export interface PanelOverlayProps {
  label: string
  sourceRef: RefObject<Element | null>
  onClose: () => void
  children: ReactNode
}

const focusableSelector = [
  'a[href]', 'button:not([disabled])', 'input:not([disabled])', 'select:not([disabled])',
  'textarea:not([disabled])', '[tabindex]:not([tabindex="-1"])',
].join(',')

/**
 * An expanded panel is a modal dialog, not a `position: fixed` card.
 *
 * Rendering it in place cannot work: any ancestor with a transform, filter,
 * or its own z-index creates a stacking context that traps the panel below
 * its siblings — which is exactly how sibling cards, badges and legends ended
 * up painting over a "fullscreen" panel. The dialog is therefore portaled
 * into its own element at the end of `body`, where nothing on the page can
 * outrank it, and it carries an opaque backdrop so nothing shows through.
 */
export function PanelOverlay({ label, sourceRef, onClose, children }: PanelOverlayProps) {
  const container = useOverlayContainer(true, sourceRef)
  const dialogRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (typeof document === 'undefined') return undefined
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = previousOverflow }
  }, [])

  const focusables = useCallback(
    () => [...(dialogRef.current?.querySelectorAll<HTMLElement>(focusableSelector) ?? [])]
      .filter((element) => element.offsetParent !== null || element === document.activeElement),
    [],
  )

  useEffect(() => {
    if (typeof document === 'undefined') return undefined
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.stopPropagation()
        onClose()
        return
      }
      if (event.key !== 'Tab' || !dialogRef.current) return
      // A modal keeps focus inside itself; without this, Tab walks into the
      // page behind the backdrop, which is unreachable by pointer.
      const elements = focusables()
      if (elements.length === 0) {
        event.preventDefault()
        dialogRef.current.focus()
        return
      }
      const first = elements[0]!
      const last = elements[elements.length - 1]!
      const active = document.activeElement
      if (!event.shiftKey && active === last) {
        event.preventDefault()
        first.focus()
      } else if (event.shiftKey && (active === first || active === dialogRef.current)) {
        event.preventDefault()
        last.focus()
      }
    }
    document.addEventListener('keydown', onKeyDown, true)
    return () => document.removeEventListener('keydown', onKeyDown, true)
  }, [focusables, onClose])

  useEffect(() => {
    if (container) dialogRef.current?.focus()
  }, [container])

  if (!container) return null

  return createPortal(
    // eslint-disable-next-line jsx-a11y/no-static-element-interactions -- the backdrop delegates dismissal while the nested dialog owns focus and semantics.
    <div
      className="lens-panel-overlay"
      // mousedown, not click: a drag that starts inside the dialog and ends on
      // the backdrop must not be read as "dismiss".
      onMouseDown={(event) => { if (event.target === event.currentTarget) onClose() }}
    >
      <div
        aria-label={label}
        aria-modal="true"
        className="lens-panel-overlay-frame"
        ref={dialogRef}
        role="dialog"
        tabIndex={-1}
      >
        {children}
      </div>
    </div>,
    container,
  )
}
