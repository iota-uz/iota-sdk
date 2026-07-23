import { useEffect, useRef, type KeyboardEvent, type ReactNode } from 'react'
import { X } from '../icons'
import { useDrawerHeader } from './provider'

interface LensDrawerProps {
  children: ReactNode
  closeLabel: string
  /** Fallback eyebrow used until the document supplies its own drawer header. */
  eyebrow: string
  label: string
  onClose: () => void
  restoreFocus?: HTMLElement
}

function focusableElements(host: HTMLElement): HTMLElement[] {
  return Array.from(host.querySelectorAll<HTMLElement>(
    'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
  )).filter((element) => !element.hasAttribute('hidden'))
}

export function LensDrawer({ children, closeLabel, eyebrow, label, onClose, restoreFocus }: LensDrawerProps) {
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

  useEffect(() => {
    const overflow = globalThis.document.body.style.overflow
    globalThis.document.body.style.overflow = 'hidden'
    const backdrop = dialogRef.current?.parentElement
    const background = Array.from(backdrop?.parentElement?.children ?? [])
      .filter((element): element is HTMLElement => element instanceof HTMLElement && element !== backdrop)
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
  }, [restoreFocus])

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

  return (
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
    </div>
  )
}
