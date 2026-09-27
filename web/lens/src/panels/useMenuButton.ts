import {
  useCallback, useEffect, useLayoutEffect, useRef, useState,
  type CSSProperties, type KeyboardEvent as ReactKeyboardEvent,
} from 'react'
import { useOverlayContainer } from '../runtime/overlayContainer'

export interface MenuPlacement {
  /** Which side of the trigger the menu opens on. */
  side: 'up' | 'down'
  /** Which of the menu's edges is flush with the trigger's. */
  align: 'start' | 'end'
}

const viewportGutter = 8
/** The breathing room between a trigger and the menu it opens. */
const menuGap = 4

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

/**
 * How far the aligned menu has to slide to stay on screen.
 *
 * Alignment alone cannot always answer the question: a menu wider than the
 * distance from its trigger to either edge fits neither way, and the flush edge
 * then hangs off the viewport — a 551px filter menu whose trigger's right edge
 * sits at 500px opened at x = −51. The shift is the last step of the same rule,
 * so a menu is bound to its trigger *and* to the viewport instead of one or the
 * other.
 */
export function menuShift(
  trigger: Pick<DOMRect, 'left' | 'right'>,
  menu: Pick<DOMRect, 'width'>,
  viewport: { width: number },
  align: MenuPlacement['align'],
): number {
  const left = align === 'start' ? trigger.left : trigger.right - menu.width
  const right = left + menu.width
  if (left < viewportGutter) return Math.round(viewportGutter - left)
  if (right > viewport.width - viewportGutter) return Math.round(viewport.width - viewportGutter - right)
  return 0
}

/** Where the menu is drawn, in viewport coordinates. */
export interface MenuOffset {
  left: number
  top: number
}

/**
 * Where the placed menu's own box starts.
 *
 * The menu is drawn in a body portal, so the answer has to be absolute rather
 * than «flush with whichever edge of my offset parent». It is the same rule the
 * flow-anchored version expressed in CSS — flush with the aligned edge, nudged
 * by the viewport shift, gapped from the trigger — just resolved to numbers.
 */
export function menuOffset(
  trigger: Pick<DOMRect, 'left' | 'right' | 'top' | 'bottom'>,
  menu: Pick<DOMRect, 'width' | 'height'>,
  placement: MenuPlacement,
  shift: number,
): MenuOffset {
  const left = (placement.align === 'start' ? trigger.left : trigger.right - menu.width) + shift
  const top = placement.side === 'down'
    ? trigger.bottom + menuGap
    : trigger.top - menu.height - menuGap
  return { left: Math.round(left), top: Math.round(top) }
}

/** Shared focus, dismissal, placement, and arrow navigation for Lens menu buttons. */
export function useMenuButton(preferredAlign: MenuPlacement['align'] = 'end') {
  const [open, setOpen] = useState(false)
  const container = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const menu = useRef<HTMLDivElement>(null)
  const [placement, setPlacement] = useState<MenuPlacement>({ side: 'down', align: preferredAlign })
  const [offset, setOffset] = useState<MenuOffset>()
  const items = useRef(new Map<string, HTMLButtonElement>())
  // A menu anchored in the flow is clipped by the first ancestor that hides its
  // overflow, and every Lens panel does — a chart panel's own toolbar opened its
  // menu 116px outside the card and the card sliced it in half. The placement
  // maths was already right; only the containing block was wrong, so the menu
  // moves to a body portal and keeps the same rule.
  const overlay = useOverlayContainer(open, container, 'lens-menu-overlay-root')
  const close = useCallback(() => setOpen(false), [])
  const closeAndFocusTrigger = useCallback(() => {
    close()
    trigger.current?.focus()
  }, [close])

  useEffect(() => {
    if (!open || typeof document === 'undefined') return undefined
    const focusFrame = requestAnimationFrame(() => [...items.current.values()][0]?.focus())
    const pointerdown = (event: PointerEvent) => {
      if (!(event.target instanceof Node)) return
      // The menu is no longer a descendant of the trigger's container: a
      // portalled item would otherwise be dismissed on pointerdown and never
      // live long enough to receive the click that chose it.
      if (container.current?.contains(event.target) || menu.current?.contains(event.target)) return
      close()
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
    const viewport = { width: globalThis.innerWidth || 1024, height: globalThis.innerHeight || 768 }
    // The card the trigger belongs to, when it has one. A panel's toolbar is
    // not always on the right of its card — a chart inside a tab group carries
    // its controls at the left — and a menu that hangs from the trigger's right
    // edge there reaches back across the card and out over the application's
    // own chrome. It is still bound to the viewport; the card only decides
    // which of the trigger's two edges it prefers.
    const card = trigger.current?.closest<HTMLElement>('.lens-panel')?.getBoundingClientRect()
    const preferred = card && preferredAlign === 'end' && anchor.right - box.width < card.left
      ? 'start'
      : preferredAlign
    const next = menuPlacement(anchor, box, viewport, preferred)
    setPlacement((current) => current.side === next.side && current.align === next.align ? current : next)
    // Computed from the anchor and the box width, i.e. from where the menu
    // *would* sit unshifted: its own rect already carries the current offset.
    const nextOffset = menuOffset(anchor, box, next, menuShift(anchor, box, viewport, next.align))
    setOffset((current) => current?.left === nextOffset.left && current.top === nextOffset.top ? current : nextOffset)
  }, [preferredAlign])

  useLayoutEffect(() => {
    if (open && overlay) place()
  }, [open, overlay, place])

  useEffect(() => {
    if (!open) setOffset(undefined)
  }, [open])

  useEffect(() => {
    if (!open || !overlay) return undefined
    const observer = typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver(place)
    if (trigger.current) observer?.observe(trigger.current)
    if (menu.current) observer?.observe(menu.current)
    globalThis.addEventListener('resize', place)
    globalThis.addEventListener('scroll', place, true)
    return () => {
      observer?.disconnect()
      globalThis.removeEventListener('resize', place)
      globalThis.removeEventListener('scroll', place, true)
    }
  }, [open, overlay, place])

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
    menuPlacementProps: {
      'data-align': placement.align,
      'data-side': placement.side,
      style: {
        left: offset?.left ?? 0,
        top: offset?.top ?? 0,
        // The first paint is the measurement pass: the menu has to have a box
        // before it can be told where its box goes. Hidden rather than
        // unmounted, so that measurement is of the real thing.
        visibility: offset ? 'visible' : 'hidden',
      } as CSSProperties,
    },
    onMenuKeyDown,
    open,
    overlay,
    setOpen,
    trigger,
  }
}
