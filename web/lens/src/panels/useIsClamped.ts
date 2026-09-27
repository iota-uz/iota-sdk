import { createEffect, createSignal, onCleanup, type Accessor } from 'solid-js'

/**
 * Whether an element's own box is hiding some of its text.
 *
 * Two callers, one question, and it is a question about the rendered box rather
 * than about the string — a name that fits at 1400px is cut at 900px, and the
 * clamp that cuts it is a CSS rule nothing in the tree can read.
 *
 * The note behind a metric strip's ⓘ exists to carry what the reader cannot
 * otherwise read, so a caption printed in full two lines below joins it only
 * when the two-line slot actually cut it. A waterfall's column name asks the
 * same thing of its native tooltip: the tooltip is the full name behind a clamp,
 * so where there is no clamp there is nothing for it to say.
 */
export function useIsClamped(target: () => HTMLElement | null | undefined): Accessor<boolean> {
  const [clamped, setClamped] = createSignal(false)
  createEffect(() => {
    const element = target()
    if (!element) return
    const measure = () => setClamped(element.scrollHeight - element.clientHeight > 1)
    measure()
    if (typeof ResizeObserver === 'undefined') return
    const observer = new ResizeObserver(measure)
    observer.observe(element)
    onCleanup(() => observer.disconnect())
  })
  return clamped
}
