// Motion helpers (#1352) over the tokens of styles/tailwind/iota.css: --motion-instant/fast/base/slow,
// --ease-out/in/standard. Screens never write a duration of their own: CSS reads the tokens, and the few
// timers that must wait for an animation read them here, so prefers-reduced-motion (every token 0) also
// shortens the wait to nothing.
import { createEffect, createSignal, on, onCleanup, type Accessor } from 'solid-js'

export type MotionToken = 'instant' | 'fast' | 'base' | 'slow' | 'fade' | 'flash'

/** The fallback values, as styles/tailwind/iota.css declares them. */
export const MOTION_MS: Readonly<Record<MotionToken, number>> = { instant: 80, fast: 140, base: 200, slow: 320, fade: 200, flash: 4000 }

/** True when the person asked the system for less motion. */
export function prefersReducedMotion(): boolean {
  return typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches
}

/** A token's current duration in milliseconds: the computed CSS value when there is a document, else the fallback. */
export function motionMs(token: MotionToken): number {
  if (typeof document !== 'undefined' && typeof getComputedStyle === 'function' && document.documentElement) {
    const raw = getComputedStyle(document.documentElement).getPropertyValue(`--motion-${token}`).trim()
    if (raw) {
      const value = parseFloat(raw)
      if (Number.isFinite(value)) return raw.endsWith('ms') ? value : raw.endsWith('s') ? value * 1000 : value
    }
  }
  if (prefersReducedMotion()) return token === 'fade' ? 80 : token === 'flash' ? MOTION_MS.flash : 0
  return MOTION_MS[token]
}

export type PresenceState = 'enter' | 'open' | 'exit'

export interface Presence {
  /** Keep the layer in the DOM while this is true: it stays through the exit animation. */
  mounted: Accessor<boolean>
  /** `enter` for the first frames, `open` after, `exit` while leaving: put it in a data attribute or class. */
  state: Accessor<PresenceState>
  /** True while leaving: the layer is `inert` and holds no focus then (A7). */
  leaving: Accessor<boolean>
}

/**
 * A layer that animates out before it unmounts. `when` is the wanted visibility; the layer stays mounted
 * for the `exit` token's duration after it turns false (0 under reduced motion, so it goes at once).
 */
export function createPresence(when: Accessor<boolean>, options: { enter?: MotionToken; exit?: MotionToken } = {}): Presence {
  // Mounting follows `when` in the same pass (a layer that opens with its page is there for the page's first
  // layout); only the unmount waits for the exit.
  const [exiting, setExiting] = createSignal(false)
  const [entered, setEntered] = createSignal(when())
  let timer: ReturnType<typeof setTimeout> | undefined
  createEffect(on(when, (visible, previous) => {
    clearTimeout(timer)
    if (visible) {
      setExiting(false)
      setEntered(false)
      timer = setTimeout(() => setEntered(true), motionMs(options.enter ?? 'base'))
      return
    }
    if (!previous && !exiting()) return
    const ms = motionMs(options.exit ?? 'fast')
    if (ms <= 0) {
      setExiting(false)
      return
    }
    setExiting(true)
    timer = setTimeout(() => setExiting(false), ms)
  }, { defer: true }))
  onCleanup(() => clearTimeout(timer))
  const mounted = () => when() || exiting()
  const state = (): PresenceState => (exiting() && !when() ? 'exit' : when() && !entered() ? 'enter' : 'open')
  return { mounted, state, leaving: () => state() === 'exit' }
}

/**
 * Keys highlighted after a change (F5): `flash(key)` marks one for the `flash` token (about 4 s); `flashing(key)`
 * says whether to put `iota-flash` on it. A key flashed again restarts its highlight.
 */
export function createFlash(): { flash(key: string): void; flashing(key: string): boolean } {
  const [keys, setKeys] = createSignal<ReadonlyMap<string, number>>(new Map())
  const timers = new Map<string, ReturnType<typeof setTimeout>>()
  let serial = 0
  // The newest mark of each key: a deferred mount writes only while its mark is still this one.
  const latest = new Map<string, number>()
  const frame = (run: () => void) => (typeof requestAnimationFrame === 'function' ? requestAnimationFrame(() => run()) : setTimeout(run, 0))
  const flash = (key: string) => {
    clearTimeout(timers.get(key))
    const mark = ++serial
    latest.set(key, mark)
    const mount = () => {
      if (latest.get(key) === mark) setKeys((current) => new Map(current).set(key, mark))
    }
    // Flashed again while it shows: the class comes off for a rendered frame, so the animation starts over.
    if (keys().has(key)) {
      setKeys((current) => {
        const next = new Map(current)
        next.delete(key)
        return next
      })
      frame(() => frame(mount))
    } else mount()
    timers.set(key, setTimeout(() => {
      timers.delete(key)
      if (latest.get(key) === mark) latest.delete(key)
      setKeys((current) => {
        if (current.get(key) !== mark) return current
        const next = new Map(current)
        next.delete(key)
        return next
      })
    }, motionMs('flash')))
  }
  onCleanup(() => {
    timers.forEach((t) => clearTimeout(t))
    latest.clear()
  })
  return { flash, flashing: (key) => keys().has(key) }
}
