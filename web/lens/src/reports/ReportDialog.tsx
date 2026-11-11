import { createSignal, createUniqueId, onCleanup, onMount, Show, type JSX } from 'solid-js'
import { Portal } from 'solid-js/web'
import { X } from '../icons'
import { useFocusTrap } from '../panels/focusTrap'
import { useOverlayContainer } from '../runtime/overlayContainer'
import { useTranslate } from '../runtime'

export interface ReportDialogProps {
  title: string
  /** Secondary line under the title. */
  description?: string
  onClose: () => void
  /** Escape; onClose when omitted. A dialog with a nested view steps back instead of closing. */
  onEscape?: () => void
  /** The element focused again after the dialog closes; the active element when omitted. */
  restoreFocus?: HTMLElement
  /** Field focused on open; the dialog itself when omitted. */
  initialFocus?: () => HTMLElement | null | undefined
  theme?: string
  size?: 'default' | 'wide'
  /**
   * 'modal' is the centred dialog. 'drawer' is a full-height sheet anchored to
   * the inline end of the viewport that slides in and out; it is the frame for
   * work the reader does beside the page rather than instead of it.
   */
  variant?: 'modal' | 'drawer'
  /** Drawer only: false while a dismissal would be refused or would only step back, so the sheet does not slide out for nothing. */
  willClose?: () => boolean
  role?: 'dialog' | 'alertdialog'
  footer?: JSX.Element
  children: JSX.Element
  testId?: string
}

/** How long the drawer's slide-out runs before the close is reported; matches the CSS duration. */
export const reportDrawerExitMs = 180

function prefersReducedMotion(): boolean {
  return typeof window === 'undefined' || typeof window.matchMedia !== 'function' || window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

/**
 * The modal frame of report picking and editing. It portals a themed Lens
 * root to the body, traps focus, closes on Escape, and on close returns focus
 * and the page's scroll position to where the reader left them. The drawer
 * variant slides in from the inline end and, when the reader dismisses it
 * (backdrop, close button, Escape), slides out before onClose is called; a
 * reader who prefers reduced motion, or an environment that cannot animate,
 * gets the immediate close.
 */
export function ReportDialog(props: ReportDialogProps): JSX.Element {
  const translate = useTranslate()
  const titleId = createUniqueId()
  const descriptionId = createUniqueId()
  const restore = props.restoreFocus ?? (document.activeElement instanceof HTMLElement ? document.activeElement : undefined)
  const scroll = { x: window.scrollX, y: window.scrollY }
  const container = useOverlayContainer(() => true, () => restore, 'lens-report-dialog-root', { theme: props.theme })
  const drawer = () => props.variant === 'drawer'
  const [leaving, setLeaving] = createSignal(false)
  let dialogRef: HTMLDivElement | undefined
  let leaveTimer: ReturnType<typeof setTimeout> | undefined
  onCleanup(() => clearTimeout(leaveTimer))

  const dismiss = (action: () => void) => {
    if (!drawer() || prefersReducedMotion() || props.willClose?.() === false) {
      action()
      return
    }
    if (leaving()) return
    setLeaving(true)
    leaveTimer = setTimeout(() => {
      action()
      // A host that keeps the drawer open (a close refused while saving) must not stay half-hidden.
      setLeaving(false)
    }, reportDrawerExitMs)
  }
  const close = () => dismiss(() => props.onClose())
  const escape = () => dismiss(() => (props.onEscape ?? props.onClose)())

  onMount(() => {
    const previousOverflow = document.documentElement.style.overflow
    document.documentElement.style.overflow = 'hidden'
    onCleanup(() => {
      document.documentElement.style.overflow = previousOverflow
      window.scrollTo(scroll.x, scroll.y)
    })
  })

  useFocusTrap(() => dialogRef, () => Boolean(container()), escape, () => props.initialFocus?.(), () => restore)

  return (
    <Show when={container()}>
      <Portal mount={container()}>
        {/* eslint-disable-next-line jsx-a11y/no-static-element-interactions -- the backdrop delegates dismissal while the nested dialog owns focus and semantics. */}
        <div
          class="lens-report-dialog-backdrop"
          data-leaving={leaving() || undefined}
          data-variant={props.variant ?? 'modal'}
          onMouseDown={(event) => { if (event.target === event.currentTarget) close() }}
        >
          <div
            aria-describedby={props.description ? descriptionId : undefined}
            aria-labelledby={titleId}
            aria-modal="true"
            class="lens-report-dialog"
            data-leaving={leaving() || undefined}
            data-size={props.size ?? 'default'}
            data-variant={props.variant ?? 'modal'}
            data-testid={props.testId}
            ref={(element) => { dialogRef = element }}
            role={props.role ?? 'dialog'}
            tabIndex={-1}
          >
            <header class="lens-report-dialog-header">
              <div class="lens-report-dialog-heading">
                <h2 class="lens-report-dialog-title" id={titleId}>{props.title}</h2>
                <Show when={props.description}>
                  <p class="lens-report-dialog-description" id={descriptionId}>{props.description}</p>
                </Show>
              </div>
              <button
                aria-label={translate('drawer.close', 'Close')}
                class="lens-icon-button lens-report-dialog-close"
                onClick={close}
                title={translate('drawer.close', 'Close')}
                type="button"
              >
                <X />
              </button>
            </header>
            <div class="lens-report-dialog-body">{props.children}</div>
            <Show when={props.footer}>
              <footer class="lens-report-dialog-footer">{props.footer}</footer>
            </Show>
          </div>
        </div>
      </Portal>
    </Show>
  )
}
