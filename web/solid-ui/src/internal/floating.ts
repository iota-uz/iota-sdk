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
  let resizeObserver: ResizeObserver | undefined
  let observedAnchor: HTMLElement | undefined
  let observedFloating: HTMLElement | undefined

  const observeSize = (anchorElement: HTMLElement, floatingElement: HTMLElement) => {
    const owner = anchorElement.ownerDocument.defaultView
    if (!owner?.ResizeObserver) return
    if (!resizeObserver) resizeObserver = new owner.ResizeObserver(update)
    if (observedAnchor !== anchorElement) {
      if (observedAnchor) resizeObserver.unobserve(observedAnchor)
      resizeObserver.observe(anchorElement)
      observedAnchor = anchorElement
    }
    if (observedFloating !== floatingElement) {
      if (observedFloating) resizeObserver.unobserve(observedFloating)
      resizeObserver.observe(floatingElement)
      observedFloating = floatingElement
    }
  }

  const update = () => {
    const anchorElement = anchor()
    const floatingElement = floating()
    if (!anchorElement || !floatingElement || !open()) return

    const owner = anchorElement.ownerDocument
    const viewport = owner.defaultView
    if (!viewport) return
    observeSize(anchorElement, floatingElement)
    const anchorRect = anchorElement.getBoundingClientRect()
    const floatingRect = floatingElement.getBoundingClientRect()
    const viewportWidth = owner.documentElement.clientWidth || viewport.innerWidth
    const viewportHeight = owner.documentElement.clientHeight || viewport.innerHeight
    const align: FloatingAlign = anchorRect.left + floatingRect.width > viewportWidth - margin ? 'end' : 'start'
    const roomBelow = viewportHeight - anchorRect.bottom - margin
    const roomAbove = anchorRect.top - margin
    const side: FloatingSide = floatingRect.height > roomBelow && roomAbove > roomBelow ? 'top' : 'bottom'
    setPlacement({ align, side })
  }

  createEffect(() => {
    if (open()) Promise.resolve().then(update)
  })
  onMount(() => {
    const owner = anchor()?.ownerDocument ?? floating()?.ownerDocument
    owner?.defaultView?.addEventListener('resize', update)
    owner?.addEventListener('scroll', update, true)
  })
  onCleanup(() => {
    const owner = anchor()?.ownerDocument ?? floating()?.ownerDocument
    owner?.defaultView?.removeEventListener('resize', update)
    owner?.removeEventListener('scroll', update, true)
    resizeObserver?.disconnect()
  })

  return { placement, update }
}

export function floatingPlacementClasses(placement: FloatingPlacement, gapClass = 'mt-1.5') {
  return [
    placement.align === 'end' ? 'right-0' : 'left-0',
    placement.side === 'top' ? `bottom-full ${gapClass.replace('mt-', 'mb-')}` : `top-full ${gapClass}`,
  ]
}
