// The Tidy of #1331: left to right, layers by longest path, barycentre sweeps. Kept as the baseline the
// current Tidy is measured against (it may not cross more links than this did).
import { CARD_HEIGHT, snap } from '../geometry'
import { IGNORE, itemIds } from '../graph'
import { backEdges, itemSize } from '../layout'
import type { AutomationGraph, AutomationLayout, EditorCatalog, XY } from '../types'

const COLUMN_GAP = 96
const ROW_GAP = 48

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
export function legacyTidyLayout(graph: AutomationGraph, catalog: EditorCatalog): AutomationLayout {
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
