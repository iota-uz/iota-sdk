// Auto-layout: Tidy lays the graph out left to right in layers, orders each layer to minimise
// crossings (barycenter sweeps), and puts every card on the grid. Cycle edges are found by a DFS and
// do not take part in layering, so they run backwards along the side of the graph.
import { cardGeometry, CARD_HEIGHT, GRID, rectsOverlap, snap, type Rect } from './geometry'
import { findItem, IGNORE, itemIds, itemOutcomes } from './graph'
import type { AutomationGraph, AutomationLayout, EditorCatalog, XY } from './types'

export const COLUMN_GAP = 96
export const ROW_GAP = 48

export function itemSize(graph: AutomationGraph, catalog: EditorCatalog, id: string): { width: number; height: number } {
  const ref = findItem(graph, id)
  const geometry = cardGeometry(itemOutcomes(graph, catalog, id), ref?.kind === 'node')
  return { width: geometry.width, height: geometry.height }
}

export function itemRect(graph: AutomationGraph, catalog: EditorCatalog, layout: AutomationLayout, id: string): Rect | null {
  const position = layout[id]
  if (!position) return null
  return { ...position, ...itemSize(graph, catalog, id) }
}

type Adjacency = { out: Map<string, string[]>; in: Map<string, string[]> }

function adjacency(graph: AutomationGraph, ids: Set<string>): Adjacency {
  const out = new Map<string, string[]>()
  const inn = new Map<string, string[]>()
  for (const id of ids) {
    out.set(id, [])
    inn.set(id, [])
  }
  for (const edge of graph.edges) {
    if (edge.to === IGNORE || !ids.has(edge.from.node) || !ids.has(edge.to) || edge.from.node === edge.to) continue
    out.get(edge.from.node)!.push(edge.to)
    inn.get(edge.to)!.push(edge.from.node)
  }
  return { out, in: inn }
}

/** Edges that close a cycle (target is on the DFS stack), as "from->to" keys. */
export function backEdges(graph: AutomationGraph): Set<string> {
  const ids = new Set(itemIds(graph))
  const { out } = adjacency(graph, ids)
  const state = new Map<string, 0 | 1 | 2>()
  const back = new Set<string>()
  const roots = [...graph.triggers.map((t) => t.id), ...graph.nodes.map((n) => n.id)]
  for (const root of roots) {
    if (state.get(root)) continue
    const stack: { id: string; next: number }[] = [{ id: root, next: 0 }]
    state.set(root, 1)
    while (stack.length > 0) {
      const frame = stack[stack.length - 1]!
      const targets = out.get(frame.id) ?? []
      if (frame.next >= targets.length) {
        state.set(frame.id, 2)
        stack.pop()
        continue
      }
      const target = targets[frame.next++]!
      const seen = state.get(target) ?? 0
      if (seen === 1) back.add(`${frame.id}->${target}`)
      else if (seen === 0) {
        state.set(target, 1)
        stack.push({ id: target, next: 0 })
      }
    }
  }
  for (const edge of graph.edges) if (edge.from.node === edge.to) back.add(`${edge.from.node}->${edge.to}`)
  return back
}

function crossings(layers: string[][], out: Map<string, string[]>): number {
  let count = 0
  for (let l = 0; l + 1 < layers.length; l += 1) {
    const next = new Map(layers[l + 1]!.map((id, index) => [id, index]))
    const segments: [number, number][] = []
    layers[l]!.forEach((id, index) => {
      for (const target of out.get(id) ?? []) {
        const to = next.get(target)
        if (to !== undefined) segments.push([index, to])
      }
    })
    for (let i = 0; i < segments.length; i += 1) {
      for (let j = i + 1; j < segments.length; j += 1) {
        const [a1, b1] = segments[i]!
        const [a2, b2] = segments[j]!
        if ((a1 - a2) * (b1 - b2) < 0) count += 1
      }
    }
  }
  return count
}

/** Left-to-right layered layout of the whole graph, snapped to the grid. */
export function tidyLayout(graph: AutomationGraph, catalog: EditorCatalog): AutomationLayout {
  const ids = itemIds(graph)
  if (ids.length === 0) return {}
  const idSet = new Set(ids)
  const back = backEdges(graph)
  const forward: AutomationGraph = { ...graph, edges: graph.edges.filter((e) => !back.has(`${e.from.node}->${e.to}`)) }
  const { out, in: inn } = adjacency(forward, idSet)
  const triggers = new Set(graph.triggers.map((t) => t.id))
  const hasTriggers = triggers.size > 0

  // Longest-path layering over the acyclic part; triggers are always the first column.
  const layer = new Map<string, number>()
  const indegree = new Map(ids.map((id) => [id, inn.get(id)!.length]))
  const queue = ids.filter((id) => indegree.get(id) === 0)
  for (const id of queue) layer.set(id, triggers.has(id) || !hasTriggers ? 0 : 1)
  while (queue.length > 0) {
    const id = queue.shift()!
    for (const target of out.get(id)!) {
      const candidate = Math.max(layer.get(target) ?? 0, (layer.get(id) ?? 0) + 1, hasTriggers && !triggers.has(target) ? 1 : 0)
      layer.set(target, candidate)
      indegree.set(target, indegree.get(target)! - 1)
      if (indegree.get(target) === 0) queue.push(target)
    }
  }
  for (const id of ids) if (!layer.has(id)) layer.set(id, triggers.has(id) ? 0 : 1)
  for (const id of triggers) layer.set(id, 0)

  const depth = Math.max(...layer.values()) + 1
  let layers: string[][] = Array.from({ length: depth }, () => [])
  for (const id of ids) layers[layer.get(id)!]!.push(id)

  // Barycenter sweeps; keep the best ordering seen.
  const order = (current: string[][], l: number, neighbours: Map<string, string[]>, reference: string[]) => {
    const position = new Map(reference.map((id, index) => [id, index]))
    const keyed = current[l]!.map((id, index) => {
      const ps = (neighbours.get(id) ?? []).map((n) => position.get(n)).filter((p): p is number => p !== undefined)
      return { id, key: ps.length > 0 ? ps.reduce((a, b) => a + b, 0) / ps.length : index }
    })
    keyed.sort((a, b) => a.key - b.key)
    current[l] = keyed.map((k) => k.id)
  }
  let best = layers.map((l) => [...l])
  let bestCrossings = crossings(best, out)
  for (let sweep = 0; sweep < 8 && bestCrossings > 0; sweep += 1) {
    const next = best.map((l) => [...l])
    if (sweep % 2 === 0) for (let l = 1; l < depth; l += 1) order(next, l, inn, next[l - 1]!)
    else for (let l = depth - 2; l >= 0; l -= 1) order(next, l, out, next[l + 1]!)
    const c = crossings(next, out)
    if (c < bestCrossings) {
      best = next
      bestCrossings = c
    }
  }
  layers = best

  // Coordinates: columns by the widest card of the previous column; rows follow the parents' centre,
  // then are pushed apart so cards never overlap.
  const layout: AutomationLayout = {}
  let x = 0
  for (let l = 0; l < depth; l += 1) {
    const column = layers[l]!
    const widths = column.map((id) => itemSize(graph, catalog, id).width)
    const pitch = CARD_HEIGHT + ROW_GAP
    const desired = column.map((id, index) => {
      const parents = (inn.get(id) ?? []).map((p) => layout[p]).filter((p): p is XY => p !== undefined)
      return parents.length > 0 ? parents.reduce((sum, p) => sum + p.y, 0) / parents.length : index * pitch
    })
    const ys: number[] = []
    column.forEach((_, index) => {
      const minimum = index === 0 ? -Infinity : ys[index - 1]! + pitch
      ys.push(Math.max(snap(desired[index]!), minimum))
    })
    column.forEach((id, index) => {
      layout[id] = { x: snap(x), y: snap(ys[index]!) }
    })
    x += Math.max(...widths) + COLUMN_GAP
  }
  // Normalise to the origin.
  const minY = Math.min(...Object.values(layout).map((p) => p.y))
  for (const id of Object.keys(layout)) layout[id] = { x: layout[id]!.x, y: snap(layout[id]!.y - minY) }
  return layout
}

/** A free grid position for a new card: right of its source, or below the triggers / other cards. */
export function placeNew(
  graph: AutomationGraph,
  catalog: EditorCatalog,
  layout: AutomationLayout,
  id: string,
  options: { source?: string | null; near?: XY | null } = {},
): XY {
  const size = itemSize(graph, catalog, id)
  const occupied = Object.keys(layout)
    .filter((other) => other !== id)
    .map((other) => itemRect(graph, catalog, layout, other))
    .filter((r): r is Rect => r !== null)
  const sourceRect = options.source ? itemRect(graph, catalog, layout, options.source) : null
  const isTrigger = findItem(graph, id)?.kind === 'trigger'
  let start: XY
  if (options.near) {
    start = { x: snap(options.near.x), y: snap(options.near.y) }
  } else if (sourceRect) {
    start = { x: snap(sourceRect.x + sourceRect.width + COLUMN_GAP), y: snap(sourceRect.y) }
  } else if (isTrigger) {
    const triggerRects = graph.triggers.map((t) => itemRect(graph, catalog, layout, t.id)).filter((r): r is Rect => r !== null)
    const nodeRects = graph.nodes.map((n) => itemRect(graph, catalog, layout, n.id)).filter((r): r is Rect => r !== null)
    const left = triggerRects.length > 0
      ? Math.min(...triggerRects.map((r) => r.x))
      : nodeRects.length > 0 ? Math.min(...nodeRects.map((r) => r.x)) - size.width - COLUMN_GAP : 0
    start = { x: snap(left), y: triggerRects.length > 0 ? snap(Math.max(...triggerRects.map((r) => r.y + r.height)) + ROW_GAP) : 0 }
  } else if (occupied.length > 0) {
    start = { x: snap(Math.max(...occupied.map((r) => r.x + r.width)) + COLUMN_GAP), y: snap(Math.min(...occupied.map((r) => r.y))) }
  } else {
    start = { x: 0, y: 0 }
  }
  const free = (p: XY) => !occupied.some((r) => rectsOverlap({ ...p, ...size }, r, GRID))
  if (free(start)) return start
  const step = CARD_HEIGHT + ROW_GAP
  for (let i = 1; i < 400; i += 1) {
    const offset = Math.ceil(i / 2) * step * (i % 2 === 1 ? 1 : -1)
    const candidate = { x: start.x, y: snap(start.y + offset) }
    if (isTrigger && offset < 0) continue
    if (free(candidate)) return candidate
  }
  return { x: start.x, y: snap(start.y + 400 * step) }
}

/** Every item gets a position: a fully missing layout is tidied, missing items are placed next to their source. */
export function ensureLayout(graph: AutomationGraph, catalog: EditorCatalog, layout: AutomationLayout): AutomationLayout {
  const ids = itemIds(graph)
  const known = ids.filter((id) => layout[id])
  if (known.length === 0 && ids.length > 0) return tidyLayout(graph, catalog)
  const missing = ids.filter((id) => !layout[id])
  if (missing.length === 0) {
    const clean: AutomationLayout = {}
    for (const id of ids) clean[id] = layout[id]!
    return clean
  }
  const next: AutomationLayout = {}
  for (const id of ids) if (layout[id]) next[id] = layout[id]!
  for (const id of missing) {
    const source = graph.edges.find((e) => e.to === id && next[e.from.node])?.from.node ?? null
    next[id] = placeNew(graph, catalog, next, id, { source })
  }
  return next
}
