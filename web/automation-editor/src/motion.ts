// Motion tokens (#1352) as the canvas reads them. The same contract as `motionMs` in @iota-uz/sdk/solid-ui
// (web/solid-ui/src/utilities/Motion.ts), kept local so the canvas chunk does not pull in the component kit.
// Durations come from the CSS custom properties of styles/tailwind/iota.css, which are 0 under
// prefers-reduced-motion (fade keeps 80 ms), so a timer that waits for an animation waits for nothing then.
import { createMemo, createSignal, onCleanup, type Accessor } from 'solid-js'

export type MotionToken = 'instant' | 'fast' | 'base' | 'slow' | 'fade'

const FALLBACK: Record<MotionToken, number> = { instant: 80, fast: 140, base: 200, slow: 320, fade: 200 }

export function reducedMotion(): boolean {
  return typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches
}

// Read once per token and kept: reading a custom property forces a style recalculation when styles are
// dirty, which on a 200-card canvas at the end of a drag was a long task (#1331 benchmark). The cache is
// dropped when the reduced-motion preference changes, the only thing that changes the tokens at run time.
const cache = new Map<MotionToken, number>()
let watching = false

export function motionMs(token: MotionToken): number {
  const known = cache.get(token)
  if (known !== undefined) return known
  if (!watching && typeof matchMedia === 'function') {
    watching = true
    matchMedia('(prefers-reduced-motion: reduce)').addEventListener?.('change', () => cache.clear())
  }
  let ms = reducedMotion() ? (token === 'fade' ? 80 : 0) : FALLBACK[token]
  if (typeof document !== 'undefined' && typeof getComputedStyle === 'function' && document.documentElement) {
    const raw = getComputedStyle(document.documentElement).getPropertyValue(`--motion-${token}`).trim()
    const value = parseFloat(raw)
    if (raw && Number.isFinite(value)) ms = raw.endsWith('ms') ? value : raw.endsWith('s') ? value * 1000 : value
  }
  cache.set(token, ms)
  return ms
}

/** Forgets the token values read so far (a test or a host that changes the tokens at run time). */
export function resetMotionCache(): void {
  cache.clear()
}

/**
 * Keeps the last value of `source` mounted while it leaves: `value()` is the source, or its last value for the
 * --motion-fast exit; `leaving()` is true meanwhile (the layer gets `ae-layer--leaving` and `inert`).
 */
export function createLeaving<T>(source: Accessor<T | null | undefined | false>): { value: Accessor<T | null>; leaving: Accessor<boolean> } {
  // The hold is decided in the same computation that sees the source close, so a render never observes a
  // gap: the closing layer stays the same instance (its search, its scroll) through its exit. A new value
  // replaces it at once.
  const [ended, setEnded] = createSignal(0)
  let held: T | null = null
  let ending = false
  let timer: ReturnType<typeof setTimeout> | undefined
  const value = createMemo<T | null>((previous) => {
    const now = source()
    ended()
    if (now) {
      clearTimeout(timer)
      held = null
      ending = false
      return now
    }
    if (ending) {
      ending = false
      return null
    }
    if (held !== null) return held
    if (previous === null || previous === undefined) return null
    const ms = motionMs('fast')
    if (ms <= 0) return null
    held = previous
    timer = setTimeout(() => {
      held = null
      ending = true
      setEnded((n) => n + 1)
    }, ms)
    return held
  }, null)
  onCleanup(() => clearTimeout(timer))
  return { value, leaving: () => !source() && value() !== null }
}
