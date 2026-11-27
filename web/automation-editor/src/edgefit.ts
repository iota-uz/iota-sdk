// How an edge's routed polyline meets the handles on screen: fitted onto them, and ended short of the input
// dot for its arrow. Only the canvas (a lazy chunk) uses it, so it stays out of the package entry.
import type { Point } from './route'

const same = (a: number, b: number) => Math.abs(a - b) < 0.5

/**
 * A route found for the committed layout, fitted onto the handles where the cards stand now: its ends
 * move onto the handles and the first and last stretch stay straight (matches the reference editor).
 * Null when an end moved more than `slack` since (a drag in progress): the edge then takes `directPath`
 * until the layout is committed and routed again. Pure.
 */
export function fitRoute(points: readonly Point[] | undefined, source: Point, target: Point, slack = 12): Point[] | null {
  if (!points || points.length < 2) return null
  const first = points[0]!
  const last = points[points.length - 1]!
  if (Math.abs(first.x - source.x) > slack || Math.abs(first.y - source.y) > slack || Math.abs(last.x - target.x) > slack || Math.abs(last.y - target.y) > slack) return null
  // A straight route whose handles ended a few px apart steps over in the middle, never a slanted line.
  const out = points.length === 2 && same(first.x, last.x) && !same(source.x, target.x)
    ? [{ ...first }, { x: first.x, y: (first.y + last.y) / 2 }, { x: last.x, y: (first.y + last.y) / 2 }, { ...last }]
    : points.map((p) => ({ x: p.x, y: p.y }))
  const n = out.length
  if (n > 2) {
    if (same(out[1]!.x, first.x)) out[1]!.x = source.x
    else out[1]!.y = source.y
    if (same(out[n - 2]!.x, last.x)) out[n - 2]!.x = target.x
    else out[n - 2]!.y = target.y
  }
  out[0] = { x: source.x, y: source.y }
  out[n - 1] = { x: target.x, y: target.y }
  return out
}

/** How far short of its target handle's centre an edge ends: its arrow tip touches the top of the dot. */
export const ARROW_GAP = 6

/** The route with its last stretch shortened by `gap` (at most half of it), so the arrow ends before the handle. */
export function trimEnd(points: readonly Point[], gap = ARROW_GAP): Point[] {
  if (points.length < 2 || gap <= 0) return [...points]
  const out = points.map((p) => ({ x: p.x, y: p.y }))
  const b = out[out.length - 1]!
  const a = out[out.length - 2]!
  const length = Math.hypot(b.x - a.x, b.y - a.y)
  if (length === 0) return out
  const cut = Math.min(gap, length / 2)
  out[out.length - 1] = { x: b.x - ((b.x - a.x) / length) * cut, y: b.y - ((b.y - a.y) / length) * cut }
  return out
}
