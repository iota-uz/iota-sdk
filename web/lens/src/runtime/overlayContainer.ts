import { useEffect, useState, type RefObject } from 'react'

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
