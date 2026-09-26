/* eslint-disable react/no-unknown-property -- Solid JSX uses `class`, the React-era rule expects `className`; the lint config migrates with the Solid port. */
import { createEffect, on, onCleanup, onMount } from 'solid-js'
import { Portal } from 'solid-js/web'
import type { JSXElement } from 'solid-js'
import { Show } from 'solid-js'
import { useOverlayContainer } from './overlayContainer'
import { useTranslate } from '../runtime'
import { X } from '../icons'

export interface PanelOverlayProps {
  label: string
  source: () => Element | null | undefined
  onClose: () => void
  children: JSXElement
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
export function PanelOverlay(props: PanelOverlayProps) {
  const container = useOverlayContainer(() => true, props.source)
  let dialogRef: HTMLDivElement | undefined
  const translate = useTranslate()

  onMount(() => {
    if (typeof document === 'undefined') return
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    onCleanup(() => { document.body.style.overflow = previousOverflow })
  })

  const focusables = () => [...(dialogRef?.querySelectorAll<HTMLElement>(focusableSelector) ?? [])]
    .filter((element) => element.offsetParent !== null || element === document.activeElement)

  createEffect(() => {
    if (typeof document === 'undefined') return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.stopPropagation()
        props.onClose()
        return
      }
      if (event.key !== 'Tab' || !dialogRef) return
      // A modal keeps focus inside itself; without this, Tab walks into the
      // page behind the backdrop, which is unreachable by pointer.
      const elements = focusables()
      if (elements.length === 0) {
        event.preventDefault()
        dialogRef.focus()
        return
      }
      const first = elements[0]!
      const last = elements[elements.length - 1]!
      const active = document.activeElement
      if (!event.shiftKey && active === last) {
        event.preventDefault()
        first.focus()
      } else if (event.shiftKey && (active === first || active === dialogRef)) {
        event.preventDefault()
        last.focus()
      }
    }
    document.addEventListener('keydown', onKeyDown, true)
    onCleanup(() => document.removeEventListener('keydown', onKeyDown, true))
  })

  createEffect(on(container, (current) => {
    if (current) dialogRef?.focus()
  }))

  return (
    <Show when={container()}>
      <Portal mount={container()}>
        {/* eslint-disable-next-line jsx-a11y/no-static-element-interactions -- the backdrop delegates dismissal while the nested dialog owns focus and semantics. */}
        <div
          class="lens-panel-overlay"
          // mousedown, not click: a drag that starts inside the dialog and ends on
          // the backdrop must not be read as "dismiss".
          onMouseDown={(event) => { if (event.target === event.currentTarget) props.onClose() }}
        >
          <div
            aria-label={props.label}
            aria-modal="true"
            class="lens-panel-overlay-frame"
            ref={(el) => { dialogRef = el }}
            role="dialog"
            tabIndex={-1}
          >
            {/* A dialog states how to leave it. The collapse glyph in the panel's
                own header is the inverse of the control that opened it — it reads
                as "make smaller", not "close" — and Escape and the scrim are both
                invisible. A named × on the dialog's own chrome says it once. */}
            <button
              aria-label={translate('drawer.close', 'Close')}
              class="lens-panel-overlay-close"
              onClick={props.onClose}
              title={translate('drawer.close', 'Close')}
              type="button"
            >
              <X />
              <span>{translate('drawer.close', 'Close')}</span>
            </button>
            {props.children}
          </div>
        </div>
      </Portal>
    </Show>
  )
}
