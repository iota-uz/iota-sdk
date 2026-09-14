import { createEffect, createSignal, onCleanup, onMount } from 'solid-js'

export type FloatingAlign = 'start' | 'end'
export type FloatingSide = 'top' | 'bottom'

export interface FloatingPlacement {
  align: FloatingAlign
  side: FloatingSide
}

/** Keeps an anchored overlay inside the viewport without moving it out of its DOM context. */
export function createFloatingPlacement(
  open: () => boolean,
  anchor: () => HTMLElement | undefined,
  floating: () => HTMLElement | undefined,
  margin = 8,
) {
  const [placement, setPlacement] = createSignal<FloatingPlacement>({ align: 'start', side: 'bottom' })

  const update = () => {
    const anchorElement = anchor()
    const floatingElement = floating()
    if (!anchorElement || !floatingElement || !open()) return

    const anchorRect = anchorElement.getBoundingClientRect()
    const floatingRect = floatingElement.getBoundingClientRect()
    const viewportWidth = document.documentElement.clientWidth || window.innerWidth
    const viewportHeight = document.documentElement.clientHeight || window.innerHeight
    const align: FloatingAlign = anchorRect.left + floatingRect.width > viewportWidth - margin ? 'end' : 'start'
    const roomBelow = viewportHeight - anchorRect.bottom - margin
    const roomAbove = anchorRect.top - margin
    const side: FloatingSide = floatingRect.height > roomBelow && roomAbove > roomBelow ? 'top' : 'bottom'
    setPlacement({ align, side })
  }

  createEffect(() => {
    if (open()) queueMicrotask(update)
  })
  onMount(() => {
    window.addEventListener('resize', update)
    document.addEventListener('scroll', update, true)
  })
  onCleanup(() => {
    window.removeEventListener('resize', update)
    document.removeEventListener('scroll', update, true)
  })

  return { placement, update }
}

export function floatingPlacementClasses(placement: FloatingPlacement, gapClass = 'mt-1.5') {
  return [
    placement.align === 'end' ? 'right-0' : 'left-0',
    placement.side === 'top' ? `bottom-full ${gapClass.replace('mt-', 'mb-')}` : `top-full ${gapClass}`,
  ]
}
