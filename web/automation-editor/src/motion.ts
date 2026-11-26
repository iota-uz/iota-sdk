// Motion tokens (#1352) as the canvas reads them. The same contract as `motionMs` in @iota-uz/sdk/solid-ui
// (web/solid-ui/src/utilities/Motion.ts), kept local so the canvas chunk does not pull in the component kit.
// Durations come from the CSS custom properties of styles/tailwind/iota.css, which are 0 under
// prefers-reduced-motion (fade keeps 80 ms), so a timer that waits for an animation waits for nothing then.
import { createEffect, createSignal, on, onCleanup, type Accessor } from 'solid-js'

export type MotionToken = 'instant' | 'fast' | 'base' | 'slow' | 'fade'

const FALLBACK: Record<MotionToken, number> = { instant: 80, fast: 140, base: 200, slow: 320, fade: 200 }

export function reducedMotion(): boolean {
  return typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches
}

export function motionMs(token: MotionToken): number {
  if (typeof document !== 'undefined' && typeof getComputedStyle === 'function' && document.documentElement) {
    const raw = getComputedStyle(document.documentElement).getPropertyValue(`--motion-${token}`).trim()
    const value = parseFloat(raw)
    if (raw && Number.isFinite(value)) return raw.endsWith('ms') ? value : raw.endsWith('s') ? value * 1000 : value
  }
  if (reducedMotion()) return token === 'fade' ? 80 : 0
  return FALLBACK[token]
}

/**
 * Keeps the last value of `source` mounted while it leaves: `value()` is the source, or its last value for the
 * --motion-fast exit; `leaving()` is true meanwhile (the layer gets `ae-layer--leaving` and `inert`).
 */
export function createLeaving<T>(source: Accessor<T | null | undefined | false>): { value: Accessor<T | null>; leaving: Accessor<boolean> } {
  // The layer follows `source` in the same pass; only its last value is held for the exit.
  const [held, setHeld] = createSignal<T | null>(null)
  let last: T | null = null
  let timer: ReturnType<typeof setTimeout> | undefined
  createEffect(on(source, (next) => {
    clearTimeout(timer)
    if (next) {
      last = next
      setHeld(null)
      return
    }
    if (last === null) return
    const gone = last
    last = null
    const ms = motionMs('fast')
    if (ms <= 0) return
    setHeld(() => gone)
    timer = setTimeout(() => setHeld(null), ms)
  }))
  onCleanup(() => clearTimeout(timer))
  const value = (): T | null => {
    const now = source()
    return now ? now : held()
  }
  return { value, leaving: () => !source() && held() !== null }
}
