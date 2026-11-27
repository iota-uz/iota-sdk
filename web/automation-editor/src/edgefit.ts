// How an edge's routed polyline meets the handles on screen: fitted onto them, and ended short of the input
// dot for its arrow. Only the canvas (a lazy chunk) uses it, so it stays out of the package entry.
import { SNAP, type Point } from './route'

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
  const straight = points.every((p) => Math.abs(p.x - first.x) < SNAP)
  // Handles less than SNAP apart across: one straight line from the source, never a jog of a pixel.
  if (straight && Math.abs(source.x - target.x) < SNAP) return [{ x: source.x, y: source.y }, { x: source.x, y: target.y }]
  // A straight route whose handles ended a few px apart steps over in the middle, never a slanted line.
  const out = straight
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
  return tidy(out)
}

/** Drops stretches shorter than a pixel (merging their ends) and the middle of three collinear points. */
export function tidy(points: Point[]): Point[] {
  const out: Point[] = []
  for (const p of points) {
    const last = out[out.length - 1]
    if (last && Math.abs(last.x - p.x) < 1 && Math.abs(last.y - p.y) < 1) {
      // Keep the ends exact: the first point stays, the last one wins.
      if (out.length > 1) out[out.length - 1] = p
      continue
    }
    out.push(p)
  }
  for (let i = out.length - 2; i > 0; i -= 1) {
    const a = out[i - 1]!
    const b = out[i]!
    const c = out[i + 1]!
    if ((same(a.x, b.x) && same(b.x, c.x)) || (same(a.y, b.y) && same(b.y, c.y))) out.splice(i, 1)
  }
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
