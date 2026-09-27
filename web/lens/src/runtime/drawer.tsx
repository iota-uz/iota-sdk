import { Portal } from '@iota-uz/sdk/client-host'
import { useRef, type ReactNode } from 'react'
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

export function LensDrawer({ children, closeLabel, dark = false, eyebrow, label, onClose, restoreFocus, theme }: LensDrawerProps) {
  const closeRef = useRef<HTMLButtonElement>(null)
  // The loaded document owns the heading: it names the metric (eyebrow), the
  // scope (title) and the period (caption) once, so the drawer never repeats a
  // page heading and per-panel titles. Until it lands, the generic eyebrow prop
  // holds the top bar.
  const header = useDrawerHeader()
  const headerEyebrow = header?.eyebrow?.trim() || eyebrow
  const headerTitle = header?.title?.trim()
  const headerCaption = header?.caption?.trim()
  // The document opts into the wide variant (~1080px) for cross-metric detail
  // views that host a full focus canvas; the slide-in motion is unchanged.
  const wide = header?.size === 'wide'

  void restoreFocus
  void theme
  return (
    <Portal surface="drawer" label={label} onEscape={onClose} className={`lens-root lens-drawer-root${dark ? ' dark' : ''}`} theme={dark ? 'dark' : 'light'}>
      {/* eslint-disable-next-line jsx-a11y/no-static-element-interactions -- backdrop dismissal is delegated while the nested dialog owns focus and semantics. */}
      <div
        className="lens-drawer-backdrop"
        // mousedown, not click: a drag that starts inside the dialog and ends on
        // the backdrop must not be read as "dismiss".
        onMouseDown={(event) => { if (event.target === event.currentTarget) onClose() }}
      >
        <div
          className={`lens-drawer${wide ? ' lens-drawer-wide' : ''}`}
        >
          <header className="lens-drawer-header">
            <div className="lens-drawer-identity">
              <span className="lens-drawer-eyebrow">{headerEyebrow}</span>
              {/* The title is what the drawer is about — a product name, a
                counterparty — and it is the one line here that truncates. Panel
                cards already carry their title as a tooltip; without the same
                here, an ellipsis is where the subject's identity ends. */}
              {headerTitle && <span className="lens-drawer-title" title={headerTitle}>{headerTitle}</span>}
              {headerCaption && <span className="lens-drawer-caption" title={headerCaption}>{headerCaption}</span>}
            </div>
            <button
              aria-label={closeLabel}
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
    </Portal>
  )
}
