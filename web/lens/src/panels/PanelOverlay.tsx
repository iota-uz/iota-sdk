import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'

export interface PanelOverlayProps {
  label: string
  /** Theme of the dashboard root the panel came from. */
  theme?: string
  dark?: boolean
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
export function PanelOverlay({ label, theme, dark = false, onClose, children }: PanelOverlayProps) {
  const [container, setContainer] = useState<HTMLElement>()
  const dialogRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (typeof document === 'undefined') return undefined
    const element = document.createElement('div')
    // The portal leaves the dashboard subtree, so it has to carry the Lens
    // root class and theme with it or every custom property resolves to its
    // fallback.
    element.className = `lens-root lens-overlay-root${dark ? ' dark' : ''}`
    if (theme) element.dataset.theme = theme
    document.body.appendChild(element)
    setContainer(element)
    return () => {
      element.remove()
      setContainer(undefined)
    }
  }, [dark, theme])

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
