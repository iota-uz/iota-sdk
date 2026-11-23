// Layout quality, as pure functions of the graph, the catalog and the positions. Tidy is held to these
// numbers in tests: no overlapping cards, no link through a card, no mixed groups, and no more
// crossings than the layout it replaced.
import type { Rect } from './geometry'
import { itemRect, layoutGroups } from './layout'
import { itemIds } from './graph'
import { linksOf, routeEdges, type Point, type Route } from './route'
import type { AutomationGraph, AutomationLayout, EditorCatalog } from './types'

export type LayoutMetrics = {
  /** Pairs of cards that overlap. */
  overlaps: number
  /** (edge, card) pairs where the route of the edge runs through the card. */
  throughNodes: number
  /** Pairs of edges whose routes cross; edges of one source or into one target do not count. */
  crossings: number
  /** Pairs of trigger groups whose areas overlap. */
  mixedGroups: number
}

function rectsOf(graph: AutomationGraph, catalog: EditorCatalog, layout: AutomationLayout): Map<string, Rect> {
  const rects = new Map<string, Rect>()
  for (const id of itemIds(graph)) {
    const rect = itemRect(graph, catalog, layout, id)
    if (rect) rects.set(id, rect)
  }
  return rects
}

/** Pairs of cards that overlap (a card touching another is not an overlap). */
export function overlappingNodes(graph: AutomationGraph, catalog: EditorCatalog, layout: AutomationLayout): number {
  const rects = [...rectsOf(graph, catalog, layout).values()].sort((a, b) => a.x - b.x)
  let count = 0
  for (let i = 0; i < rects.length; i += 1) {
    const a = rects[i]!
    for (let j = i + 1; j < rects.length && rects[j]!.x < a.x + a.width; j += 1) {
      const b = rects[j]!
      if (a.y < b.y + b.height && a.y + a.height > b.y) count += 1
    }
  }
  return count
}

/** Does a polyline go through the inside of a rectangle? */
function crosses(points: Point[], rect: Rect): boolean {
  const left = rect.x + 1
  const right = rect.x + rect.width - 1
  const top = rect.y + 1
  const bottom = rect.y + rect.height - 1
  for (let i = 1; i < points.length; i += 1) {
    const a = points[i - 1]!
    const b = points[i]!
    if (Math.max(a.x, b.x) > left && Math.min(a.x, b.x) < right && Math.max(a.y, b.y) > top && Math.min(a.y, b.y) < bottom) return true
  }
  return false
}

/** (edge, card) pairs where the route runs through the card, counting the edge's own cards too. */
export function edgesThroughNodes(rects: Map<string, Rect>, routes: Map<string, Route>): number {
  let count = 0
  for (const route of routes.values()) {
    for (const rect of rects.values()) if (crosses(route.points, rect)) count += 1
  }
  return count
}

type Segment = { a: Point; b: Point; key: string; source: string; target: string }

function segmentsOf(routes: Map<string, Route>, ends: Map<string, { from: string; to: string }>): Segment[] {
  const out: Segment[] = []
  for (const [key, route] of routes) {
    const end = ends.get(key)
    if (!end) continue
    for (let i = 1; i < route.points.length; i += 1) out.push({ a: route.points[i - 1]!, b: route.points[i]!, key, source: end.from, target: end.to })
  }
  return out
}

/** Pairs of edges whose routes cross. Edges that share a source or a target meet by design and are skipped. */
export function edgeCrossings(routes: Map<string, Route>, ends: Map<string, { from: string; to: string }>): number {
  const segments = segmentsOf(routes, ends)
  const horizontal = segments.filter((s) => Math.abs(s.a.y - s.b.y) < 0.5 && Math.abs(s.a.x - s.b.x) >= 0.5)
  const vertical = segments.filter((s) => Math.abs(s.a.x - s.b.x) < 0.5 && Math.abs(s.a.y - s.b.y) >= 0.5)
  const pairs = new Set<string>()
  for (const h of horizontal) {
    const y = h.a.y
    const lo = Math.min(h.a.x, h.b.x)
    const hi = Math.max(h.a.x, h.b.x)
    for (const v of vertical) {
      if (h.source === v.source || h.target === v.target) continue
      const x = v.a.x
      if (x > lo + 0.5 && x < hi - 0.5 && y > Math.min(v.a.y, v.b.y) + 0.5 && y < Math.max(v.a.y, v.b.y) - 0.5) {
        pairs.add(h.key < v.key ? `${h.key}|${v.key}` : `${v.key}|${h.key}`)
      }
    }
  }
  return pairs.size
}

/** Pairs of trigger groups (and the shared group) whose areas overlap: their cards are mixed. */
export function mixedGroups(graph: AutomationGraph, catalog: EditorCatalog, layout: AutomationLayout): number {
  const rects = rectsOf(graph, catalog, layout)
  const areas = layoutGroups(graph).map((group) => {
    const own = group.items.map((id) => rects.get(id)).filter((r): r is Rect => r !== undefined)
    if (own.length === 0) return null
    return {
      left: Math.min(...own.map((r) => r.x)),
      right: Math.max(...own.map((r) => r.x + r.width)),
      top: Math.min(...own.map((r) => r.y)),
      bottom: Math.max(...own.map((r) => r.y + r.height)),
    }
  }).filter((a) => a !== null)
  let count = 0
  for (let i = 0; i < areas.length; i += 1) {
    for (let j = i + 1; j < areas.length; j += 1) {
      const a = areas[i]!
      const b = areas[j]!
      if (a.left < b.right && a.right > b.left && a.top < b.bottom && a.bottom > b.top) count += 1
    }
  }
  return count
}

/** All four numbers for a layout; `routes` may be passed when they are already known. */
export function layoutMetrics(graph: AutomationGraph, catalog: EditorCatalog, layout: AutomationLayout, routes?: Map<string, Route>): LayoutMetrics {
  const found = routes ?? routeEdges(graph, catalog, layout)
  const { links } = linksOf(graph, catalog, layout)
  const ends = new Map(links.map((l) => [l.key, { from: l.from, to: l.to }]))
  return {
    overlaps: overlappingNodes(graph, catalog, layout),
    throughNodes: edgesThroughNodes(rectsOf(graph, catalog, layout), found),
    crossings: edgeCrossings(found, ends),
    mixedGroups: mixedGroups(graph, catalog, layout),
  }
}
