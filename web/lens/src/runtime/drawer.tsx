import { useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { X } from '../icons'
import { useDrawerHeader } from './provider'

interface LensDrawerProps {
  children: ReactNode
  closeLabel: string
  /**
   * The drawer can stack on top of an expanded panel, which is itself a
   * body-level portal. So the drawer carries the theme of the dashboard root it
   * came from, mirroring PanelOverlay, or every `--lens-*` custom property on
   * the portaled subtree resolves to its fallback.
   */
  dark?: boolean
  /** Fallback eyebrow used until the document supplies its own drawer header. */
  eyebrow: string
  label: string
  onClose: () => void
  restoreFocus?: HTMLElement
  theme?: string
}

function focusableElements(host: HTMLElement): HTMLElement[] {
  return Array.from(host.querySelectorAll<HTMLElement>(
    'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
  )).filter((element) => !element.hasAttribute('hidden'))
}

export function LensDrawer({ children, closeLabel, dark = false, eyebrow, label, onClose, restoreFocus, theme }: LensDrawerProps) {
  const [container, setContainer] = useState<HTMLElement>()
  const dialogRef = useRef<HTMLDivElement>(null)
  const closeRef = useRef<HTMLButtonElement>(null)
  // The loaded document owns the heading: it names the metric (eyebrow), the
  // scope (title) and the period (caption) once, so the drawer never repeats a
  // page heading and per-panel titles. Until it lands, the generic eyebrow prop
  // holds the top bar.
  const header = useDrawerHeader()
  const headerEyebrow = header?.eyebrow?.trim() || eyebrow
  const headerTitle = header?.title?.trim()
  const headerCaption = header?.caption?.trim()

  // The drawer portals to a body-level host, mirroring PanelOverlay: nested
  // inline in the app root it could never paint above the expand overlay (a
  // body-level portal at a huge z-index) no matter its own z-index. The host
  // re-declares the Lens root class + theme it left behind so custom properties
  // still resolve, and `.lens-drawer-root` sits one rung above the overlay.
  useEffect(() => {
    if (typeof document === 'undefined') return undefined
    const element = globalThis.document.createElement('div')
    element.className = `lens-root lens-drawer-root${dark ? ' dark' : ''}`
    if (theme) element.dataset.theme = theme
    globalThis.document.body.appendChild(element)
    setContainer(element)
    return () => {
      element.remove()
      setContainer(undefined)
    }
  }, [dark, theme])

  useEffect(() => {
    if (!container) return undefined
    const overflow = globalThis.document.body.style.overflow
    globalThis.document.body.style.overflow = 'hidden'
    // The drawer is now a body-level portal, so the background to seal off is
    // the set of sibling body children — the dashboard app root and, when the
    // drawer was opened from an expanded panel, that overlay's own host. Inert
    // every direct child of body except the drawer's own container; walking the
    // old backdrop → parent → children chain would now inert the drawer itself.
    const background = Array.from(globalThis.document.body.children)
      .filter((element): element is HTMLElement => element instanceof HTMLElement && element !== container)
      .map((element) => ({
        element,
        inert: element.inert,
        ariaHidden: element.getAttribute('aria-hidden'),
      }))
    for (const state of background) {
      state.element.inert = true
      state.element.setAttribute('aria-hidden', 'true')
    }
    closeRef.current?.focus()
    return () => {
      globalThis.document.body.style.overflow = overflow
      for (const state of background) {
        state.element.inert = state.inert
        if (state.ariaHidden === null) state.element.removeAttribute('aria-hidden')
        else state.element.setAttribute('aria-hidden', state.ariaHidden)
      }
      if (restoreFocus?.isConnected) restoreFocus.focus()
    }
  }, [container, restoreFocus])

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Escape') {
      event.preventDefault()
      event.stopPropagation()
      onClose()
      return
    }
    if (event.key !== 'Tab' || !dialogRef.current) return
    const focusable = focusableElements(dialogRef.current)
    if (!focusable.length) {
      event.preventDefault()
      dialogRef.current.focus()
      return
    }
    const first = focusable[0]
    const last = focusable.at(-1)
    if (event.shiftKey && globalThis.document.activeElement === first) {
      event.preventDefault()
      last?.focus()
    } else if (!event.shiftKey && globalThis.document.activeElement === last) {
      event.preventDefault()
      first?.focus()
    }
  }

  if (!container) return null

  return createPortal(
    <div
      className="lens-drawer-backdrop"
      // mousedown, not click: a drag that starts inside the dialog and ends on
      // the backdrop must not be read as "dismiss".
      onMouseDown={(event) => { if (event.target === event.currentTarget) onClose() }}
    >
      <div
        aria-label={label}
        aria-modal="true"
        className="lens-drawer"
        onKeyDown={onKeyDown}
        ref={dialogRef}
        role="dialog"
        tabIndex={-1}
      >
        <header className="lens-drawer-header">
          <div className="lens-drawer-identity">
            <span className="lens-drawer-eyebrow">{headerEyebrow}</span>
            {headerTitle && <span className="lens-drawer-title">{headerTitle}</span>}
            {headerCaption && <span className="lens-drawer-caption">{headerCaption}</span>}
          </div>
          <button
            aria-label={closeLabel}
            autoFocus
            className="lens-drawer-close"
            onClick={onClose}
            ref={closeRef}
            type="button"
          >
            <X />
          </button>
        </header>
        <div className="lens-drawer-document">
          {children}
        </div>
      </div>
    </div>,
    container,
  )
}
