// Orthogonal edge routing. An edge leaves the bottom of a card (down, from the port of its outcome),
// runs through the free channels between rows and columns, and enters the top of its target. A route
// has at most three channels: a horizontal one under the source, a vertical one, and a horizontal one
// above the target; the candidates come from the gaps around the cards, so a route never crosses one
// while a free channel exists. Edges leaving one card share their channels (a common trunk).
// Nothing here knows the DOM: it takes rectangles and ports and returns polylines.
import { cardGeometry, CARD_HEIGHT, type Rect } from './geometry'
import { edgeKey, findItem, IGNORE, itemOutcomes } from './graph'
import type { AutomationGraph, AutomationLayout, EditorCatalog } from './types'

export type Point = { x: number; y: number }
export type Route = {
  /** From the port to the target handle; corners only. */
  points: Point[]
  /** No free channel was found: the route goes through a card. */
  blocked: boolean
  /** Set when the route keeps less than the usual distance from the cards (a narrow gap); corners must stay inside it. */
  clear?: number
}
export type RouteLink = {
  /** `node.outcome` */
  key: string
  from: string
  to: string
  /** The port: where the edge leaves the source card. */
  port: Point
  /** The target handle: where it enters. */
  entry: Point
}
export type Router = {
  route(key: string): Route | undefined
  /**
   * The links to route, in the order that routes each exactly once, for the asked ones: every link of a
   * source that comes before an asked one in port order is routed first (they share channels), so it is
   * listed too. Links already routed are left out.
   */
  plan(keys: Iterable<string>): string[]
  /** Routes every link, in source order. */
  all(): Map<string, Route>
}

/** Distance of the first and last channel from a card. Channels step by LANE from there. */
export const STUB = 12
export const LANE = 12
const LANES = 5
/** The shortest stub (from the port down to the first channel, from the last one into the target): room for two full corners. */
const MIN_STUB = 22
/** The stretch the arrow takes off the end of an edge (`ARROW_GAP` in edgefit). */
const ARROW_ROOM = 6
/** How far below a card a channel is looked for when nothing is below it (a row gap). */
const ROW_REACH = 90
/** Ends closer than this across are drawn as one straight line. */
export const SNAP = 2
const CLEAR = 4
const BEND_COST = 24
const TRUNK_REWARD = 0.85

/** Cards as obstacles, with a coarse grid to find the ones near a segment. */
class Obstacles {
  private cells = new Map<number, number[]>()
  private stamp = new Uint32Array(0)
  private mark = 0
  constructor(readonly rects: Rect[]) {
    this.stamp = new Uint32Array(rects.length)
    rects.forEach((r, i) => {
      for (let cx = this.cell(r.x); cx <= this.cell(r.x + r.width); cx += 1) {
        for (let cy = this.cell(r.y); cy <= this.cell(r.y + r.height); cy += 1) {
          const key = cx * 100003 + cy
          const list = this.cells.get(key)
          if (list) list.push(i)
          else this.cells.set(key, [i])
        }
      }
    })
  }
  private cell(v: number) {
    return Math.floor(v / 256)
  }
  /** Rectangles that may touch the box. */
  near(x1: number, y1: number, x2: number, y2: number): Rect[] {
    const cx1 = this.cell(Math.min(x1, x2))
    const cx2 = this.cell(Math.max(x1, x2))
    const cy1 = this.cell(Math.min(y1, y2))
    const cy2 = this.cell(Math.max(y1, y2))
    // A box wider than the grid is cheaper to answer from the cards than from its empty cells.
    if ((cx2 - cx1 + 1) * (cy2 - cy1 + 1) > this.rects.length * 4) {
      const lx = Math.min(x1, x2)
      const hx = Math.max(x1, x2)
      const ly = Math.min(y1, y2)
      const hy = Math.max(y1, y2)
      return this.rects.filter((r) => hx >= r.x && lx <= r.x + r.width && hy >= r.y && ly <= r.y + r.height)
    }
    this.mark += 1
    const out: Rect[] = []
    for (let cx = cx1; cx <= cx2; cx += 1) {
      for (let cy = cy1; cy <= cy2; cy += 1) {
        for (const i of this.cells.get(cx * 100003 + cy) ?? []) {
          if (this.stamp[i] === this.mark) continue
          this.stamp[i] = this.mark
          out.push(this.rects[i]!)
        }
      }
    }
    return out
  }
  /** Does an axis-aligned segment cross a card (kept `clear` away from it)? */
  blocked(x1: number, y1: number, x2: number, y2: number, clear = CLEAR): boolean {
    const lx = Math.min(x1, x2)
    const hx = Math.max(x1, x2)
    const ly = Math.min(y1, y2)
    const hy = Math.max(y1, y2)
    for (const r of this.near(lx, ly, hx, hy)) {
      if (hx > r.x - clear && lx < r.x + r.width + clear && hy > r.y - clear && ly < r.y + r.height + clear) return true
    }
    return false
  }
}

const same = (a: number, b: number) => Math.abs(a - b) < 0.5

/** Drops repeated points (closer than `eps` on both axes) and the middle point of three collinear ones. */
export function simplify(points: Point[], eps = 0.5): Point[] {
  const out: Point[] = []
  for (const p of points) {
    const last = out[out.length - 1]
    if (last && Math.abs(last.x - p.x) < eps && Math.abs(last.y - p.y) < eps) continue
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

/** A route that ignores the cards: down, across, down; around the side when the target is not below. */
export function directRoute(port: Point, entry: Point): Route {
  if (entry.y - STUB * 2 > port.y) {
    const y = Math.round((port.y + entry.y) / 2)
    return { points: simplify([port, { x: port.x, y }, { x: entry.x, y }, entry]), blocked: false }
  }
  const x = Math.max(port.x, entry.x) + 48
  const top = entry.y - STUB
  return { points: simplify([port, { x: port.x, y: port.y + STUB }, { x, y: port.y + STUB }, { x, y: top }, { x: entry.x, y: top }, entry]), blocked: false }
}

/** The path of `directRoute` for the common case, a target below, built without the arrays: what an edge shows until its route is found. */
export function directPath(a: Point, b: Point, radius = 10): string {
  if (b.y - STUB * 2 <= a.y) return routePath(directRoute(a, b).points)
  if (Math.abs(a.x - b.x) < SNAP) return `M${a.x},${a.y}L${a.x},${b.y}`
  const y = Math.round((a.y + b.y) / 2)
  const r = Math.min(radius, (y - a.y) / 2, Math.abs(b.x - a.x) / 2, (b.y - y) / 2)
  const dx = b.x > a.x ? r : -r
  return `M${a.x},${a.y}L${a.x},${y - r}Q${a.x},${y} ${a.x + dx},${y}L${b.x - dx},${y}Q${b.x},${y} ${b.x},${y + r}L${b.x},${b.y}`
}

type Trunk = { h: { y: number; lo: number; hi: number }[]; v: { x: number; lo: number; hi: number }[] }

const overlap = (a1: number, a2: number, b1: number, b2: number) => Math.max(0, Math.min(Math.max(a1, a2), Math.max(b1, b2)) - Math.max(Math.min(a1, a2), Math.min(b1, b2)))

/** Lanes in a free gap between `from` and `to` (from < to): the balanced middle first, then alternately either side by LANE, each keeping `head` from `from` and `tail` from `to`; the middle alone when none fits. */
function lanesIn(from: number, to: number, head: number, tail: number): number[] {
  // The middle of what is left once each end keeps its own extra (the arrow's stretch at the target), so
  // both stubs come out equal; whole pixels where there is room, exact in a narrow gap.
  const gap = to - from
  const centre = gap >= head * 2 ? from + (gap - (tail - head)) / 2 : from + gap / 2
  const mid = gap >= head * 2 ? Math.round(centre) : centre
  const out: number[] = []
  for (let k = 0; out.length < LANES && k <= LANES * 2; k += 1) {
    const y = mid + (k === 0 ? 0 : (k % 2 ? -1 : 1) * Math.ceil(k / 2) * LANE)
    if (y - from >= head && to - y >= tail) out.push(y)
  }
  return out.length ? out : [mid]
}

function routeOne(link: RouteLink, obstacles: Obstacles, trunk: Trunk, preference: number): Route {
  const probe = { relaxable: false }
  const route = routeWith(link, obstacles, trunk, preference, false, probe)
  if (!route.blocked || !probe.relaxable) return route
  const relaxed = routeWith(link, obstacles, trunk, preference, true, probe)
  return relaxed.blocked ? route : relaxed
}

function routeWith(link: RouteLink, obstacles: Obstacles, trunk: Trunk, preference: number, relax: boolean, probe: { relaxable: boolean }): Route {
  const s = link.port
  // Ends less than SNAP apart across are one column: a straight edge, never a jog of a pixel.
  const t = Math.abs(link.entry.x - s.x) < SNAP ? { x: s.x, y: link.entry.y } : link.entry
  // Straight down, when nothing stands between the handles.
  if (s.x === t.x && t.y > s.y && !obstacles.blocked(s.x, s.y, s.x, t.y, 0)) return { points: [s, t], blocked: false }
  const bandTop = Math.min(s.y, t.y) - ROW_REACH - 96
  const bandBottom = Math.max(t.y, s.y) + ROW_REACH + 96
  const first = obstacles.near(Math.min(s.x, t.x) - 96, bandTop, Math.max(s.x, t.x) + 96, bandBottom)
  // The horizontal channels run through the middle of the free gap under the source and over the target
  // (the gap between the rows), lanes spread around it, so both corners of a stub keep their full radius.
  const lo = Math.min(s.x, t.x)
  const hi = Math.max(s.x, t.x)
  const spans = (r: Rect) => r.x < hi + 1 && r.x + r.width > lo - 1
  let floor = t.y > s.y ? t.y : s.y + ROW_REACH
  // A card in the span that leaves no room for a stub under the source and is not over the source column
  // closes the lanes only as the first try; when that finds no route (`relax`), the card stays an obstacle
  // for the search alone and the lanes reach down the free source column.
  const overSource = (r: Rect) => s.x > r.x && s.x < r.x + r.width
  let floorCard: Rect | undefined
  for (const r of first) {
    if (r.y < s.y - 0.5 || r.y >= floor || !(spans(r) || overSource(r))) continue
    if (relax && !overSource(r) && r.y - s.y < MIN_STUB) continue
    floor = r.y
    floorCard = r
  }
  if (floorCard && !overSource(floorCard) && floorCard.y - s.y < MIN_STUB) probe.relaxable = true
  let ceiling = t.y > s.y ? s.y : t.y - ROW_REACH
  for (const r of first) {
    const bottom = r.y + r.height
    if (bottom <= t.y + 0.5 && bottom > ceiling && (spans(r) || (t.x > r.x && t.x < r.x + r.width))) ceiling = bottom
  }
  const below = lanesIn(s.y, floor, MIN_STUB, MIN_STUB + ARROW_ROOM)
  const above = lanesIn(ceiling, t.y, MIN_STUB, MIN_STUB + ARROW_ROOM)
  const stubFree = (ys: number[], x: number, from: number) =>
    ys.filter((y) => {
      const lo = Math.min(from, y)
      const hi = Math.max(from, y)
      for (const r of first) if (x > r.x && x < r.x + r.width && hi > r.y && lo < r.y + r.height) return false
      return true
    })
  let clear = CLEAR
  // A gap too narrow for two full corners keeps a margin that fits it, so the middle channel is usable.
  for (const gap of [floor - s.y, t.y - ceiling]) if (gap > 0 && gap < MIN_STUB * 2 + ARROW_ROOM) clear = Math.min(clear, gap * 0.375)
  let y1s = stubFree(below, s.x, s.y)
  let y2s = stubFree(above, t.x, t.y)
  if (y1s.length === 0 || y2s.length === 0) {
    // Cards closer than a stub: a straight link, when nothing is between the handles.
    if (same(s.x, t.x) && t.y > s.y && !obstacles.blocked(s.x, s.y, s.x, t.y, 0)) return { points: [s, t], blocked: false }
    // Or a channel in the middle of the gap the cards leave under the source and over the target, with a
    // margin that fits the gap.
    if (y1s.length === 0) {
      let top = Infinity
      for (const r of first) if (s.x > r.x && s.x < r.x + r.width && r.y >= s.y - 0.5) top = Math.min(top, r.y)
      const gap = top - s.y
      if (gap > 0 && gap < Infinity) {
        y1s = [s.y + gap / 2]
        clear = Math.min(clear, gap * 0.375)
      }
    }
    if (y2s.length === 0) {
      let bottom = -Infinity
      for (const r of first) if (t.x > r.x && t.x < r.x + r.width && r.y + r.height <= t.y + 0.5) bottom = Math.max(bottom, r.y + r.height)
      const gap = t.y - bottom
      if (gap > 0 && gap < Infinity) {
        y2s = [t.y - gap / 2]
        clear = Math.min(clear, gap * 0.375)
      }
    }
    if (y1s.length === 0 || y2s.length === 0) return { ...directRoute(s, t), blocked: true }
  }

  // Vertical channels: under the source, above the target, and the gaps around the cards between. The
  // search starts with the cards close to the edge and widens only when it finds nothing.
  for (const reach of [96, 480, 100000]) {
    const found = search(reach)
    if (found) return clear < CLEAR ? { points: found, blocked: false, clear } : { points: found, blocked: false }
  }
  return { ...directRoute(s, t), blocked: true }

  function search(reach: number): Point[] | null {
  const x0 = Math.min(s.x, t.x) - reach
  const x1 = Math.max(s.x, t.x) + reach
  const yTop = Math.min(y1s[0]!, y2s[0]!) - reach
  const yBottom = Math.max(y1s[0]!, y2s[0]!) + reach
  const local = reach === 96 ? first : obstacles.near(x0, yTop, x1, yBottom)
  const free = (ax: number, ay: number, bx: number, by: number) => {
    const lx = Math.min(ax, bx) - clear
    const hx = Math.max(ax, bx) + clear
    const ly = Math.min(ay, by) - clear
    const hy = Math.max(ay, by) + clear
    for (const r of local) if (hx > r.x && lx < r.x + r.width && hy > r.y && ly < r.y + r.height) return false
    // `local` only proposes the columns; a segment is checked against every card it passes.
    for (const r of obstacles.near(lx, ly, hx, hy)) if (hx > r.x && lx < r.x + r.width && hy > r.y && ly < r.y + r.height) return false
    return true
  }
  type Column = { x: number; bound: number; special: boolean }
  const column = (x: number): Column => ({ x, bound: Math.abs(x - s.x) + Math.abs(x - t.x), special: x === s.x || x === t.x })
  const around = (): Column[] => {
    const xs = new Set<number>([s.x, t.x])
    // Channels beside the cards near the source and near the target: a vertical run starts and ends there.
    const window = reach === 96 ? 320 : reach
    for (const r of local) {
      if ((r.x > s.x + window || r.x + r.width < s.x - window) && (r.x > t.x + window || r.x + r.width < t.x - window)) continue
      for (let k = 0; k < 3; k += 1) {
        xs.add(r.x - STUB - k * LANE)
        xs.add(r.x + r.width + STUB + k * LANE)
      }
    }
    for (const v of trunk.v) xs.add(v.x)
    return [...xs].map(column).sort((a, b) => a.bound - b.bound || Number(b.special) - Number(a.special) || a.x - b.x)
  }

  let rewardCap = 0
  for (const h of trunk.h) rewardCap += (h.hi - h.lo) * TRUNK_REWARD
  for (const v of trunk.v) rewardCap += (v.hi - v.lo) * TRUNK_REWARD
  const laneBias = (index: number) => Math.abs(index - preference) * 0.6
  const vertical = Math.abs(t.y - s.y)

  type Pick = { x: number; y1: number; y2: number; cost: number }
  /** The price of going down to y1, across to x, along to y2, across to the target and down: length, bends, trunk. */
  const price = (x: number, y1: number, y2: number, i: number, j: number): number => {
    const down0 = y1 - s.y
    const across1 = Math.abs(x - s.x)
    const along = Math.abs(y2 - y1)
    const across3 = Math.abs(t.x - x)
    const down4 = t.y - y2
    // Bends: the axis changes between the segments that are not empty (same-axis neighbours merge).
    let bends = 0
    let axis = -1
    const step = (length: number, a: number) => {
      if (length < 0.5) return
      if (axis >= 0 && axis !== a) bends += 1
      axis = a
    }
    step(down0, 0)
    step(across1, 1)
    step(along, 0)
    step(across3, 1)
    step(down4, 0)
    let shared = 0
    for (const h of trunk.h) {
      if (same(h.y, y1)) shared += overlap(s.x, x, h.lo, h.hi)
      if (same(h.y, y2)) shared += overlap(x, t.x, h.lo, h.hi)
    }
    for (const v of trunk.v) if (same(v.x, x)) shared += overlap(y1, y2, v.lo, v.hi)
    return down0 + across1 + along + across3 + down4 - shared * TRUNK_REWARD + bends * BEND_COST + laneBias(i) + laneBias(j)
  }
  const build = (pick: Pick): Point[] => simplify([s, { x: s.x, y: pick.y1 }, { x: pick.x, y: pick.y1 }, { x: pick.x, y: pick.y2 }, { x: t.x, y: pick.y2 }, t])
  const open = (x: number, y1: number, y2: number) => free(s.x, y1, x, y1) && free(x, y1, x, y2) && free(x, y2, t.x, y2)
  // Down the source's own column the first channel is irrelevant, and down the target's the last: one
  // lane is enough there, and it may not double back.
  const lanes = (col: Column, visit: (y1: number, y2: number, i: number, j: number) => void) => {
    const iMax = col.x === s.x ? 1 : y1s.length
    const jMax = col.x === t.x ? 1 : y2s.length
    for (let i = 0; i < iMax; i += 1) {
      for (let j = 0; j < jMax; j += 1) {
        const y1 = y1s[i]!
        const y2 = y2s[j]!
        if (!col.special && y1 === y2) continue
        if (col.x === t.x && col.x !== s.x && y1 > y2) continue
        if (col.x === s.x && col.x !== t.x && y2 < y1) continue
        visit(y1, y2, i, j)
      }
    }
  }
  // The plain ways: down the source's column or the target's. The candidates are priced first and checked
  // against the cards cheapest first, so most edges need one or two checks.
  const plain = (): Pick | null => {
    const picks: Pick[] = []
    // A long way down goes down first and across just above the target, where the strip is free of other
    // edges; a short one across first. The cheaper of equal ways is tried first.
    const longWay = t.y - s.y > 300
    for (const col of s.x === t.x ? [column(s.x)] : longWay ? [column(s.x), column(t.x)] : [column(Math.min(s.x, t.x)), column(Math.max(s.x, t.x))]) {
      lanes(col, (y1, y2, i, j) => picks.push({ x: col.x, y1, y2, cost: price(col.x, y1, y2, i, j) }))
    }
    picks.sort((a, b) => a.cost - b.cost)
    for (const pick of picks) if (open(pick.x, pick.y1, pick.y2)) return pick
    return null
  }
  // A channel that is neither the source's nor the target's costs two bends more than the plain ways, so
  // it is looked for only when they are blocked.
  const other = (): Pick | null => {
    const held: { best: Pick | null } = { best: null }
    for (const col of around()) {
      const floor = vertical + col.bound + 4 * BEND_COST - rewardCap
      if (held.best && floor >= held.best.cost) break
      lanes(col, (y1, y2, i, j) => {
        if (!open(col.x, y1, y2)) return
        const cost = price(col.x, y1, y2, i, j)
        if (!held.best || cost < held.best.cost) held.best = { x: col.x, y1, y2, cost }
      })
    }
    return held.best
  }
  const found = plain() ?? other()
  return found ? build(found) : null
  }
}

// A lane preference per source, so edges of different sources do not all take the same lane.
const hash = (text: string) => [...text].reduce((h, c) => (h * 31 + c.charCodeAt(0)) | 0, 0) >>> 0

/** A router over cards and links; routes are found per source card, on first use. */
export function createRouter(rects: Map<string, Rect>, links: RouteLink[]): Router {
  const obstacles = new Obstacles([...rects.values()])
  const bySource = new Map<string, RouteLink[]>()
  for (const link of links) {
    const list = bySource.get(link.from)
    if (list) list.push(link)
    else bySource.set(link.from, [link])
  }
  for (const list of bySource.values()) list.sort((a, b) => a.port.x - b.port.x || (a.key < b.key ? -1 : 1))
  const byKey = new Map(links.map((l) => [l.key, l]))
  const routes = new Map<string, Route>()
  // Per source: the trunk so far and how many of its links (in port order) are routed. An edge is routed
  // after the edges before it, which it may share channels with, and not before it is asked for.
  const progress = new Map<string, { trunk: Trunk; next: number; preference: number }>()
  const routeUpTo = (source: string, last: number) => {
    const list = bySource.get(source) ?? []
    let state = progress.get(source)
    if (!state) {
      state = { trunk: { h: [], v: [] }, next: 0, preference: hash(source) % 3 }
      progress.set(source, state)
    }
    for (; state.next <= last && state.next < list.length; state.next += 1) {
      const link = list[state.next]!
      const route = routeOne(link, obstacles, state.trunk, state.preference)
      routes.set(link.key, route)
      for (let i = 1; i < route.points.length; i += 1) {
        const a = route.points[i - 1]!
        const b = route.points[i]!
        if (same(a.y, b.y)) state.trunk.h.push({ y: a.y, lo: Math.min(a.x, b.x), hi: Math.max(a.x, b.x) })
        else if (same(a.x, b.x) && i > 1) state.trunk.v.push({ x: a.x, lo: Math.min(a.y, b.y), hi: Math.max(a.y, b.y) })
      }
    }
  }
  return {
    plan(keys) {
      const need = new Map<string, number>()
      for (const key of keys) {
        const link = byKey.get(key)
        if (!link) continue
        need.set(link.from, Math.max(need.get(link.from) ?? -1, bySource.get(link.from)!.indexOf(link)))
      }
      const out: string[] = []
      for (const [source, last] of need) {
        const list = bySource.get(source)!
        for (let i = progress.get(source)?.next ?? 0; i <= last; i += 1) out.push(list[i]!.key)
      }
      return out
    },
    route(key) {
      const link = byKey.get(key)
      if (!link) return undefined
      if (!routes.has(key)) routeUpTo(link.from, bySource.get(link.from)!.indexOf(link))
      return routes.get(key)
    },
    all() {
      for (const [source, list] of bySource) routeUpTo(source, list.length - 1)
      return routes
    },
  }
}

/** The links of a graph at the given positions, with their ports and handles. */
export function linksOf(graph: AutomationGraph, catalog: EditorCatalog, layout: AutomationLayout): { rects: Map<string, Rect>; links: RouteLink[] } {
  const rects = new Map<string, Rect>()
  const geometry = new Map<string, ReturnType<typeof cardGeometry>>()
  for (const id of [...graph.triggers.map((t) => t.id), ...graph.nodes.map((n) => n.id)]) {
    const p = layout[id]
    if (!p) continue
    const g = cardGeometry(itemOutcomes(graph, catalog, id), findItem(graph, id)?.kind === 'node')
    geometry.set(id, g)
    rects.set(id, { x: p.x, y: p.y, width: g.width, height: CARD_HEIGHT })
  }
  const links: RouteLink[] = []
  for (const edge of graph.edges) {
    if (edge.to === IGNORE) continue
    const p = layout[edge.from.node]
    const q = layout[edge.to]
    const slot = geometry.get(edge.from.node)?.outcomes.find((o) => o.outcome === edge.from.outcome)
    const target = geometry.get(edge.to)?.target
    if (!p || !q || !slot || !target) continue
    links.push({
      key: edgeKey(edge.from.node, edge.from.outcome),
      from: edge.from.node,
      to: edge.to,
      port: { x: p.x + slot.handle.x, y: p.y + slot.handle.y },
      entry: { x: q.x + target.x, y: q.y + target.y },
    })
  }
  return { rects, links }
}

/** Routes of every edge of the graph by `node.outcome`. */
export function routeEdges(graph: AutomationGraph, catalog: EditorCatalog, layout: AutomationLayout): Map<string, Route> {
  const { rects, links } = linksOf(graph, catalog, layout)
  return createRouter(rects, links).all()
}

/** SVG path of a route, with rounded corners. */
export function routePath(points: Point[], radius = 10, clear = CLEAR): string {
  if (points.length === 0) return ''
  // A rounded corner cuts in by a quarter of its radius on each axis: keep that inside the distance the route keeps from cards.
  radius = Math.min(radius, clear * 3.5)
  const len = (a: Point, b: Point) => Math.hypot(b.x - a.x, b.y - a.y)
  let d = `M${points[0]!.x},${points[0]!.y}`
  for (let i = 1; i < points.length; i += 1) {
    const prev = points[i - 1]!
    const here = points[i]!
    const next = points[i + 1]
    if (!next) {
      d += `L${here.x},${here.y}`
      continue
    }
    // A step across shorter than two corners between two vertical runs that go on the same way (a lane
    // change of a few px): one smooth S over a radius above and below, not two kinks.
    const after2 = points[i + 2]
    const sway = radius
    if (
      after2 && same(prev.x, here.x) && same(here.y, next.y) && same(next.x, after2.x) &&
      len(here, next) < radius * 2 && Math.sign(here.y - prev.y) === Math.sign(after2.y - next.y) &&
      len(prev, here) >= sway && len(next, after2) >= sway && (i === 1 || len(prev, here) >= sway * 2) && (i + 2 === points.length - 1 || len(next, after2) >= sway * 2)
    ) {
      const dir = Math.sign(after2.y - next.y)
      d += `L${here.x},${here.y - dir * sway}C${here.x},${here.y} ${next.x},${next.y} ${next.x},${next.y + dir * sway}`
      i += 1
      continue
    }
    const before = len(prev, here)
    const after = len(here, next)
    const r = Math.min(radius, before / 2, after / 2)
    if (r < 1) {
      d += `L${here.x},${here.y}`
      continue
    }
    const a = { x: here.x - ((here.x - prev.x) / before) * r, y: here.y - ((here.y - prev.y) / before) * r }
    const b = { x: here.x + ((next.x - here.x) / after) * r, y: here.y + ((next.y - here.y) / after) * r }
    d += `L${a.x},${a.y}Q${here.x},${here.y} ${b.x},${b.y}`
  }
  return d
}
