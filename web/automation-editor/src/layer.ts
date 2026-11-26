// Shared behaviour of menus, popovers and palettes (K3, K4): close on any pointerdown outside (capture
// phase, and that press does not reach the element under it), on wheel/zoom outside, on window blur and
// on a change of the window width (not height: the on-screen keyboard changes the height).
import { createEffect, onCleanup, onMount } from 'solid-js'

export function useDismiss(element: () => HTMLElement | undefined, close: () => void, options: { swallow?: boolean } = {}): void {
  onMount(() => {
    let width = window.innerWidth
    const inside = (target: EventTarget | null) => {
      const el = element()
      return Boolean(el && target instanceof Node && el.contains(target))
    }
    const onPointerDown = (event: PointerEvent) => {
      if (inside(event.target)) return
      if (options.swallow !== false) {
        // The press that closed the layer does not act on the element under it.
        const swallow = (e: Event) => {
          e.preventDefault()
          e.stopPropagation()
        }
        window.addEventListener('click', swallow, { capture: true, once: true })
        setTimeout(() => window.removeEventListener('click', swallow, { capture: true }), 400)
      }
      close()
    }
    const onWheel = (event: WheelEvent) => {
      if (!inside(event.target)) close()
    }
    const onBlur = () => close()
    const onResize = () => {
      if (window.innerWidth !== width) {
        width = window.innerWidth
        close()
      }
    }
    window.addEventListener('pointerdown', onPointerDown, true)
    window.addEventListener('wheel', onWheel, { capture: true, passive: true })
    window.addEventListener('blur', onBlur)
    window.addEventListener('resize', onResize)
    onCleanup(() => {
      window.removeEventListener('pointerdown', onPointerDown, true)
      window.removeEventListener('wheel', onWheel, true)
      window.removeEventListener('blur', onBlur)
      window.removeEventListener('resize', onResize)
    })
  })
}

/**
 * Gives focus back to the opener as a layer closes (K3), once: when it starts leaving (A7: the leaving layer is
 * inert and holds no focus) or when it unmounts. Focus that already moved elsewhere (a card the pick created)
 * is left where it is.
 */
export function createFocusReturn(element: () => HTMLElement | undefined, opener: HTMLElement | null): () => void {
  let done = false
  return () => {
    if (done) return
    done = true
    const active = document.activeElement
    const root = element()
    if (active && active !== document.body && root && !root.contains(active)) return
    if (opener && opener.isConnected) opener.focus({ preventScroll: true })
  }
}

/** A leaving layer (A7): the shared leaving animation, `inert`, and focus back to the opener at once. */
export function useLeaving(element: () => HTMLElement | undefined, leaving: (() => boolean | undefined) | undefined, restore: () => void): void {
  createEffect(() => {
    const el = element()
    const now = Boolean(leaving?.())
    if (!el) return
    el.classList.toggle('ae-layer--leaving', now)
    el.toggleAttribute('inert', now)
    if (now) restore()
  })
}

/** Keeps Tab inside a layer and returns focus to the opener when it closes (K3). */
export function useFocusTrap(element: () => HTMLElement | undefined, options: { autofocus?: boolean; leaving?: () => boolean | undefined } = {}): void {
  onMount(() => {
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null
    const restore = createFocusReturn(element, opener)
    useLeaving(element, options.leaving, restore)
    const el = element()
    if (el && options.autofocus !== false) {
      const first = el.querySelector<HTMLElement>('[data-autofocus]') ?? focusables(el)[0]
      first?.focus({ preventScroll: true })
    } else {
      el?.focus({ preventScroll: true })
    }
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== 'Tab') return
      const root = element()
      if (!root) return
      const list = focusables(root)
      if (list.length === 0) return
      const first = list[0]!
      const last = list[list.length - 1]!
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first.focus()
      }
    }
    el?.addEventListener('keydown', onKey)
    onCleanup(() => {
      el?.removeEventListener('keydown', onKey)
      restore()
    })
  })
}

function focusables(root: HTMLElement): HTMLElement[] {
  return [...root.querySelectorAll<HTMLElement>('button:not([disabled]), [href], input:not([disabled]), textarea:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])')]
    .filter((el) => el.offsetParent !== null || el === document.activeElement)
}

export function isCoarse(): boolean {
  return typeof matchMedia === 'function' && matchMedia('(pointer: coarse)').matches
}
