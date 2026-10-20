import { createUniqueId, onCleanup, onMount, Show, type JSX } from 'solid-js'
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
  /** The element focused again after the dialog closes; the active element when omitted. */
  restoreFocus?: HTMLElement
  /** Field focused on open; the dialog itself when omitted. */
  initialFocus?: () => HTMLElement | null | undefined
  theme?: string
  size?: 'default' | 'wide'
  role?: 'dialog' | 'alertdialog'
  footer?: JSX.Element
  children: JSX.Element
  testId?: string
}

/**
 * The modal frame of report picking and editing. It portals a themed Lens
 * root to the body, traps focus, closes on Escape, and on close returns focus
 * and the page's scroll position to where the reader left them.
 */
export function ReportDialog(props: ReportDialogProps): JSX.Element {
  const translate = useTranslate()
  const titleId = createUniqueId()
  const descriptionId = createUniqueId()
  const restore = props.restoreFocus ?? (document.activeElement instanceof HTMLElement ? document.activeElement : undefined)
  const scroll = { x: window.scrollX, y: window.scrollY }
  const container = useOverlayContainer(() => true, () => restore, 'lens-report-dialog-root', { theme: props.theme })
  let dialogRef: HTMLDivElement | undefined

  onMount(() => {
    const previousOverflow = document.documentElement.style.overflow
    document.documentElement.style.overflow = 'hidden'
    onCleanup(() => {
      document.documentElement.style.overflow = previousOverflow
      window.scrollTo(scroll.x, scroll.y)
    })
  })

  useFocusTrap(() => dialogRef, () => Boolean(container()), () => props.onClose(), () => props.initialFocus?.(), () => restore)

  return (
    <Show when={container()}>
      <Portal mount={container()}>
        {/* eslint-disable-next-line jsx-a11y/no-static-element-interactions -- the backdrop delegates dismissal while the nested dialog owns focus and semantics. */}
        <div
          class="lens-report-dialog-backdrop"
          onMouseDown={(event) => { if (event.target === event.currentTarget) props.onClose() }}
        >
          <div
            aria-describedby={props.description ? descriptionId : undefined}
            aria-labelledby={titleId}
            aria-modal="true"
            class="lens-report-dialog"
            data-size={props.size ?? 'default'}
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
                onClick={() => props.onClose()}
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
