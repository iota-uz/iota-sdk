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

/**
 * Calls `reposition` whenever an open, fixed-position overlay can have drifted
 * from its anchor: a viewport resize, a scroll in any container, a size change
 * of the observed elements, or a web font finishing loading. The last one
 * matters on first paint: an anchor measured in a fallback face (KaTeX glyphs,
 * the dashboard's own font) moves once the real face arrives, and a one-shot
 * measurement would leave the overlay pointing at where the anchor used to be.
 * Call it inside an effect that runs while the overlay is open; it registers
 * its own cleanup.
 */
export function followAnchorLayout(reposition: () => void, observed: ReadonlyArray<Element | null | undefined> = []): void {
  if (typeof globalThis.addEventListener !== 'function') return
  const observer = typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver(() => reposition())
  for (const element of observed) if (element) observer?.observe(element)
  const fonts = typeof document === 'undefined' ? undefined : document.fonts as FontFaceSet | undefined
  const onFonts = () => reposition()
  globalThis.addEventListener('resize', reposition)
  // Capture scrolls from the dashboard's own scroll container as well as the
  // document; a fixed portal must continue to follow its trigger.
  globalThis.addEventListener('scroll', reposition, true)
  fonts?.addEventListener?.('loadingdone', onFonts)
  onCleanup(() => {
    observer?.disconnect()
    globalThis.removeEventListener('resize', reposition)
    globalThis.removeEventListener('scroll', reposition, true)
    fonts?.removeEventListener?.('loadingdone', onFonts)
  })
}

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
