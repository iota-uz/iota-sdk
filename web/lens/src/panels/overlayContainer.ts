import { createEffect, createSignal, onCleanup, type Accessor } from 'solid-js'

/**
 * How long a hover-opened overlay survives the pointer leaving its trigger.
 *
 * Every one of these lives in a body portal with a deliberate visual gap from
 * the thing that opened it, and the pointer has to cross that gap to reach the
 * card's own buttons and its selectable text. Close on the trigger's bare
 * `mouseleave` and the portal unmounts mid-travel, which reads as a flicker.
 */
export const hoverBridgeDelay = 140

interface OverlayThemeFallback {
  dark?: boolean
  theme?: string
}

/** Creates one body-level Lens root and copies the source dashboard theme. */
export function useOverlayContainer(
  active: Accessor<boolean>,
  source: () => Element | null | undefined,
  extraClass = '',
  fallback: OverlayThemeFallback = {},
): Accessor<HTMLElement | undefined> {
  const [container, setContainer] = createSignal<HTMLElement>()
  createEffect(() => {
    if (!active() || typeof document === 'undefined') return
    const element = document.createElement('div')
    const root = source()?.closest<HTMLElement>('.lens-root')
    const dark = root?.classList.contains('dark') ?? fallback.dark ?? false
    element.className = `lens-root lens-overlay-root${extraClass ? ` ${extraClass}` : ''}${dark ? ' dark' : ''}`
    const theme = root?.dataset.theme ?? fallback.theme
    if (theme) element.dataset.theme = theme
    document.body.appendChild(element)
    setContainer(element)
    onCleanup(() => {
      element.remove()
      setContainer(undefined)
    })
  })
  return container
}
