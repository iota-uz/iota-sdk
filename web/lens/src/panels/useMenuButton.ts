import {
  useCallback, useEffect, useLayoutEffect, useRef, useState, type KeyboardEvent as ReactKeyboardEvent,
} from 'react'

export interface MenuPlacement {
  /** Which side of the trigger the menu opens on. */
  side: 'up' | 'down'
  /** Which of the menu's edges is flush with the trigger's. */
  align: 'start' | 'end'
}

const viewportGutter = 8

/**
 * Where a menu of this size fits, given its trigger.
 *
 * A menu is bound to its trigger and to the viewport, and the two can disagree:
 * a panel menu opened low on the page ran «Vector (SVG)» below the fold, and a
 * page-level menu wider than its trigger read as detached because neither edge
 * lined up with anything. The preferred alignment is the trigger's leading
 * edge, which is where the eye already is; it flips only when the menu would
 * leave the viewport, and the menu drops upward only when it does not fit
 * below and does fit above.
 */
export function menuPlacement(
  trigger: Pick<DOMRect, 'left' | 'right' | 'top' | 'bottom'>,
  menu: Pick<DOMRect, 'width' | 'height'>,
  viewport: { width: number; height: number },
  preferred: MenuPlacement['align'],
): MenuPlacement {
  const fitsBelow = trigger.bottom + menu.height <= viewport.height - viewportGutter
  const fitsAbove = trigger.top - menu.height >= viewportGutter
  const startFits = trigger.left + menu.width <= viewport.width - viewportGutter
  const endFits = trigger.right - menu.width >= viewportGutter
  const align = preferred === 'start'
    ? (startFits || !endFits ? 'start' : 'end')
    : (endFits || !startFits ? 'end' : 'start')
  return { side: fitsBelow || !fitsAbove ? 'down' : 'up', align }
}

/** Shared focus, dismissal, placement, and arrow navigation for Lens menu buttons. */
export function useMenuButton(preferredAlign: MenuPlacement['align'] = 'end') {
  const [open, setOpen] = useState(false)
  const container = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const menu = useRef<HTMLDivElement>(null)
  const [placement, setPlacement] = useState<MenuPlacement>({ side: 'down', align: preferredAlign })
  const items = useRef(new Map<string, HTMLButtonElement>())
  const close = useCallback(() => setOpen(false), [])
  const closeAndFocusTrigger = useCallback(() => {
    close()
    trigger.current?.focus()
  }, [close])

  useEffect(() => {
    if (!open || typeof document === 'undefined') return undefined
    const focusFrame = requestAnimationFrame(() => [...items.current.values()][0]?.focus())
    const pointerdown = (event: PointerEvent) => {
      if (event.target instanceof Node && !container.current?.contains(event.target)) close()
    }
    const keydown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      event.stopPropagation()
      closeAndFocusTrigger()
    }
    document.addEventListener('pointerdown', pointerdown, true)
    document.addEventListener('keydown', keydown, true)
    return () => {
      cancelAnimationFrame(focusFrame)
      document.removeEventListener('pointerdown', pointerdown, true)
      document.removeEventListener('keydown', keydown, true)
    }
  }, [close, closeAndFocusTrigger, open])

  // Measured on open and again while it is open, because the page behind it
  // scrolls and resizes under a menu that is anchored in the flow.
  const place = useCallback(() => {
    const anchor = trigger.current?.getBoundingClientRect()
    const box = menu.current?.getBoundingClientRect()
    if (!anchor || !box) return
    const next = menuPlacement(
      anchor,
      box,
      { width: globalThis.innerWidth || 1024, height: globalThis.innerHeight || 768 },
      preferredAlign,
    )
    setPlacement((current) => current.side === next.side && current.align === next.align ? current : next)
  }, [preferredAlign])

  useLayoutEffect(() => {
    if (open) place()
  }, [open, place])

  useEffect(() => {
    if (!open) return undefined
    globalThis.addEventListener('resize', place)
    globalThis.addEventListener('scroll', place, true)
    return () => {
      globalThis.removeEventListener('resize', place)
      globalThis.removeEventListener('scroll', place, true)
    }
  }, [open, place])

  const itemRef = useCallback((key: string) => (element: HTMLButtonElement | null) => {
    if (element) items.current.set(key, element)
    else items.current.delete(key)
  }, [])
  const onMenuKeyDown = useCallback((event: ReactKeyboardEvent) => {
    if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
    event.preventDefault()
    const activeItems = [...items.current.values()].filter((item) => item.isConnected)
    if (activeItems.length === 0) return
    if (event.key === 'Home') {
      activeItems[0]?.focus()
      return
    }
    if (event.key === 'End') {
      activeItems[activeItems.length - 1]?.focus()
      return
    }
    const current = activeItems.indexOf(document.activeElement as HTMLButtonElement)
    const delta = event.key === 'ArrowDown' ? 1 : -1
    activeItems[(current + delta + activeItems.length) % activeItems.length]?.focus()
  }, [])

  return {
    close,
    closeAndFocusTrigger,
    container,
    itemRef,
    menu,
    menuPlacementProps: { 'data-align': placement.align, 'data-side': placement.side },
    onMenuKeyDown,
    open,
    setOpen,
    trigger,
  }
}
