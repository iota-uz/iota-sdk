// Shared behaviour of menus, popovers and palettes (K3, K4): close on any pointerdown outside (capture
// phase, and that press does not reach the element under it), on wheel/zoom outside, on window blur and
// on a change of the window width (not height: the on-screen keyboard changes the height).
import { onCleanup, onMount } from 'solid-js'

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

/** Keeps Tab inside a layer and returns focus to the opener when it closes (K3). */
export function useFocusTrap(element: () => HTMLElement | undefined, options: { autofocus?: boolean } = {}): void {
  onMount(() => {
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null
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
      if (opener && opener.isConnected) opener.focus({ preventScroll: true })
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
