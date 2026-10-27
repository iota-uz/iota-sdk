import { onCleanup, onMount, type JSXElement } from 'solid-js'
import { Portal } from 'solid-js/web'
import { X } from '../icons'
import { useDrawerHeader } from './provider'

const focusableSelector = 'a[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])'

interface LensDrawerProps {
  children: JSXElement
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

export function LensDrawer(props: LensDrawerProps): JSXElement {
  // The loaded document owns the heading: it names the metric (eyebrow), the
  // scope (title) and the period (caption) once, so the drawer never repeats a
  // page heading and per-panel titles. Until it lands, the generic eyebrow prop
  // holds the top bar.
  const header = useDrawerHeader()
  const headerEyebrow = () => header()?.eyebrow?.trim() || props.eyebrow
  const headerTitle = () => header()?.title?.trim()
  const headerCaption = () => header()?.caption?.trim()
  // The document opts into the wide variant (~1080px) for cross-metric detail
  // views that host a full focus canvas; the slide-in motion is unchanged.
  const wide = () => header()?.size === 'wide'
  let dialogRef: HTMLDivElement | undefined

  onMount(() => {
    const restore = props.restoreFocus
      ?? (document.activeElement instanceof HTMLElement ? document.activeElement : undefined)
    // The portaled dialog takes focus itself; the client-host portal focused the
    // first focusable control (the drawer's own close button) on mount.
    ;(dialogRef?.querySelector<HTMLElement>('button,[href],input,select,textarea,[tabindex]:not([tabindex="-1"])') ?? dialogRef)?.focus()
    // A blocking overlay locks the page behind it for the duration.
    const previousOverflow = document.documentElement.style.overflow
    document.documentElement.style.overflow = 'hidden'
    onCleanup(() => {
      document.documentElement.style.overflow = previousOverflow
      restore?.focus()
    })
  })

  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key === 'Escape') {
      event.preventDefault()
      event.stopPropagation()
      props.onClose()
      return
    }
    if (event.key !== 'Tab' || !dialogRef) return
    const focusable = [...dialogRef.querySelectorAll<HTMLElement>(focusableSelector)]
    if (focusable.length === 0) {
      event.preventDefault()
      dialogRef.focus()
      return
    }
    const first = focusable[0]!
    const last = focusable[focusable.length - 1]!
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault()
      last.focus()
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault()
      first.focus()
    }
  }

  return (
    <Portal mount={document.body}>
      {/* eslint-disable-next-line jsx-a11y/no-noninteractive-element-interactions -- shortcuts are delegated from the focusable dialog inside; the root is not a focus stop. */}
      <div
        aria-label={props.label}
        aria-modal="true"
        class={`lens-root lens-drawer-root${props.dark ? ' dark' : ''}`}
        data-iota-surface="drawer"
        data-theme={props.theme}
        onKeyDown={onKeyDown}
        ref={(el) => { dialogRef = el }}
        role="dialog"
        tabIndex={-1}
      >
        {/* eslint-disable-next-line jsx-a11y/no-static-element-interactions -- backdrop dismissal is delegated while the nested dialog owns focus and semantics. */}
        <div
          class="lens-drawer-backdrop"
          // mousedown, not click: a drag that starts inside the dialog and ends on
          // the backdrop must not be read as "dismiss".
          onMouseDown={(event) => { if (event.target === event.currentTarget) props.onClose() }}
        >
          <div class={`lens-drawer${wide() ? ' lens-drawer-wide' : ''}`}>
            <header class="lens-drawer-header">
              <div class="lens-drawer-identity">
                <span class="lens-drawer-eyebrow">{headerEyebrow()}</span>
                {/* The title is what the drawer is about — a product name, a
                  counterparty — and it is the one line here that truncates. Panel
                  cards already carry their title as a tooltip; without the same
                  here, an ellipsis is where the subject's identity ends. */}
                {headerTitle() && <span class="lens-drawer-title" title={headerTitle()}>{headerTitle()}</span>}
                {headerCaption() && <span class="lens-drawer-caption" title={headerCaption()}>{headerCaption()}</span>}
              </div>
              <button
                aria-label={props.closeLabel}
                class="lens-drawer-close"
                onClick={() => props.onClose()}
                type="button"
              >
                <X />
              </button>
            </header>
            <div class="lens-drawer-document">
              {props.children}
            </div>
          </div>
        </div>
      </div>
    </Portal>
  )
}
