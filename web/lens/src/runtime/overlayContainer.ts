import { useEffect, useState, type RefObject } from 'react'

/**
 * How long a hover-opened overlay survives the pointer leaving its trigger.
 *
 * Every one of these lives in a body portal with a deliberate visual gap from
 * the thing that opened it, and the pointer has to cross that gap to reach the
 * card's own buttons and its selectable text. Close on the trigger's bare
 * `mouseleave` and the portal unmounts mid-travel, which reads as a flicker.
 *
 * It sits here because the gap is a property of portalling to the body, not of
 * any one tip: it was written twice, at 120ms and at 140ms, with the same
 * reasoning stated both times. The longer of the two is the one kept — the cost
 * of overshooting is a tip that lingers imperceptibly, and of undershooting, the
 * bug.
 */
export const hoverBridgeDelay = 140

interface OverlayThemeFallback {
  dark?: boolean
  theme?: string
}

/** Creates one body-level Lens root and copies the source dashboard theme. */
export function useOverlayContainer(
  active: boolean,
  source: RefObject<Element | null>,
  extraClass = '',
  fallback: OverlayThemeFallback = {},
): HTMLElement | undefined {
  const [container, setContainer] = useState<HTMLElement>()
  useEffect(() => {
    if (!active || typeof document === 'undefined') return undefined
    const element = document.createElement('div')
    const root = source.current?.closest<HTMLElement>('.lens-root')
    const dark = root?.classList.contains('dark') ?? fallback.dark ?? false
    element.className = `lens-root lens-overlay-root${extraClass ? ` ${extraClass}` : ''}${dark ? ' dark' : ''}`
    const theme = root?.dataset.theme ?? fallback.theme
    if (theme) element.dataset.theme = theme
    document.body.appendChild(element)
    setContainer(element)
    return () => {
      element.remove()
      setContainer(undefined)
    }
  }, [active, extraClass, fallback.dark, fallback.theme, source])
  return container
}
