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
  /** Routes every link, in source order. */
  all(): Map<string, Route>
}

/** Distance of the first and last channel from a card. Channels step by LANE from there. */
export const STUB = 12
export const LANE = 12
const LANES = 5
const CLEAR = 4
const BEND_COST = 24
const TRUNK_REWARD = 0.85
const COLUMN_CANDIDATES = 96

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
    this.mark += 1
    const out: Rect[] = []
    for (let cx = this.cell(Math.min(x1, x2)); cx <= this.cell(Math.max(x1, x2)); cx += 1) {
      for (let cy = this.cell(Math.min(y1, y2)); cy <= this.cell(Math.max(y1, y2)); cy += 1) {
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

/** Drops repeated points and the middle point of three collinear ones. */
export function simplify(points: Point[]): Point[] {
  const out: Point[] = []
  for (const p of points) {
    const last = out[out.length - 1]
    if (last && same(last.x, p.x) && same(last.y, p.y)) continue
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
  const down = entry.y - STUB * 2 > port.y
  if (down) {
    const y = Math.round((port.y + entry.y) / 2)
    return { points: simplify([port, { x: port.x, y }, { x: entry.x, y }, entry]), blocked: false }
  }
  const x = Math.max(port.x, entry.x) + 48
  return {
    points: simplify([port, { x: port.x, y: port.y + STUB }, { x, y: port.y + STUB }, { x, y: entry.y - STUB }, { x: entry.x, y: entry.y - STUB }, entry]),
    blocked: false,
  }
}

type Trunk = { h: { y: number; lo: number; hi: number }[]; v: { x: number; lo: number; hi: number }[] }

const overlap = (a1: number, a2: number, b1: number, b2: number) => Math.max(0, Math.min(Math.max(a1, a2), Math.max(b1, b2)) - Math.max(Math.min(a1, a2), Math.min(b1, b2)))

function routeOne(link: RouteLink, obstacles: Obstacles, trunk: Trunk, preference: number): Route {
  const { port: s, entry: t } = link
  // The horizontal channels under the source and above the target, nearest first.
  const below: number[] = []
  const above: number[] = []
  for (let k = 0; k < LANES; k += 1) {
    below.push(s.y + STUB + k * LANE)
    above.push(t.y - STUB - k * LANE)
  }
  const stubFree = (ys: number[], x: number, from: number, down: boolean) =>
    ys.filter((y) => !obstacles.blocked(x, from, x, y, 0) && (down ? y > from : y < from))
  const y1s = stubFree(below, s.x, s.y, true)
  const y2s = stubFree(above, t.x, t.y, false)
  if (y1s.length === 0 || y2s.length === 0) return { ...directRoute(s, t), blocked: true }

  // Vertical channels: under the source, above the target, and the gaps around the cards between.
  const reach = 480
  const x0 = Math.min(s.x, t.x) - reach
  const x1 = Math.max(s.x, t.x) + reach
  const yTop = Math.min(y1s[0]!, y2s[0]!) - reach
  const yBottom = Math.max(y1s[0]!, y2s[0]!) + reach
  const xs = new Set<number>([s.x, t.x])
  for (const r of obstacles.near(x0, yTop, x1, yBottom)) {
    for (let k = 0; k < 3; k += 1) {
      xs.add(r.x - STUB - k * LANE)
      xs.add(r.x + r.width + STUB + k * LANE)
    }
  }
  for (const v of trunk.v) xs.add(v.x)
  const columns = [...xs]
    .map((x) => ({ x, bound: Math.abs(x - s.x) + Math.abs(x - t.x) }))
    .sort((a, b) => a.bound - b.bound || a.x - b.x)
    .slice(0, COLUMN_CANDIDATES)

  let best: { points: Point[]; cost: number } | null = null
  let rewardCap = 0
  for (const h of trunk.h) rewardCap += (h.hi - h.lo) * TRUNK_REWARD
  for (const v of trunk.v) rewardCap += (v.hi - v.lo) * TRUNK_REWARD
  const laneBias = (index: number) => Math.abs(index - preference) * 0.6
  for (const column of columns) {
    // The cheapest the route can be with this vertical channel; stop when nothing left can win.
    const floor = (y1s[0]! - s.y) + column.bound + Math.abs(t.y - y2s[0]!) - rewardCap
    if (best && floor >= best.cost) break
    for (let i = 0; i < y1s.length; i += 1) {
      const y1 = y1s[i]!
      if (obstacles.blocked(s.x, y1, column.x, y1)) continue
      for (let j = 0; j < y2s.length; j += 1) {
        const y2 = y2s[j]!
        if (obstacles.blocked(column.x, y1, column.x, y2) || obstacles.blocked(column.x, y2, t.x, y2)) continue
        const points = simplify([s, { x: s.x, y: y1 }, { x: column.x, y: y1 }, { x: column.x, y: y2 }, { x: t.x, y: y2 }, t])
        let length = 0
        for (let k = 1; k < points.length; k += 1) length += Math.abs(points[k]!.x - points[k - 1]!.x) + Math.abs(points[k]!.y - points[k - 1]!.y)
        let shared = 0
        for (const h of trunk.h) {
          if (same(h.y, y1)) shared += overlap(s.x, column.x, h.lo, h.hi)
          if (same(h.y, y2)) shared += overlap(column.x, t.x, h.lo, h.hi)
        }
        for (const v of trunk.v) if (same(v.x, column.x)) shared += overlap(y1, y2, v.lo, v.hi)
        const cost = length - shared * TRUNK_REWARD + (points.length - 2) * BEND_COST + laneBias(i) + laneBias(j)
        if (!best || cost < best.cost) best = { points, cost }
      }
    }
  }
  if (!best) return { ...directRoute(s, t), blocked: true }
  return { points: best.points, blocked: false }
}

const hash = (text: string) => {
  let h = 0
  for (let i = 0; i < text.length; i += 1) h = (h * 31 + text.charCodeAt(i)) | 0
  return Math.abs(h)
}

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
  const done = new Set<string>()
  const routeSource = (source: string) => {
    if (done.has(source)) return
    done.add(source)
    const trunk: Trunk = { h: [], v: [] }
    const preference = hash(source) % 3
    for (const link of bySource.get(source) ?? []) {
      const route = routeOne(link, obstacles, trunk, preference)
      routes.set(link.key, route)
      for (let i = 1; i < route.points.length; i += 1) {
        const a = route.points[i - 1]!
        const b = route.points[i]!
        if (same(a.y, b.y)) trunk.h.push({ y: a.y, lo: Math.min(a.x, b.x), hi: Math.max(a.x, b.x) })
        else if (same(a.x, b.x) && i > 1) trunk.v.push({ x: a.x, lo: Math.min(a.y, b.y), hi: Math.max(a.y, b.y) })
      }
    }
  }
  return {
    route(key) {
      const link = byKey.get(key)
      if (!link) return undefined
      routeSource(link.from)
      return routes.get(key)
    },
    all() {
      for (const source of bySource.keys()) routeSource(source)
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
export function routePath(points: Point[], radius = 10): string {
  if (points.length === 0) return ''
  let d = `M${points[0]!.x},${points[0]!.y}`
  for (let i = 1; i < points.length; i += 1) {
    const prev = points[i - 1]!
    const here = points[i]!
    const next = points[i + 1]
    if (!next) {
      d += `L${here.x},${here.y}`
      continue
    }
    const before = Math.hypot(here.x - prev.x, here.y - prev.y)
    const after = Math.hypot(next.x - here.x, next.y - here.y)
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
