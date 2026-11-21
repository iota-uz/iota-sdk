// Copy/paste of selected items with their internal edges. The clipboard text is JSON, so pasting
// works between tabs; ids are renamed when they are taken.
import { GRID, snap } from './geometry'
import { findItem, IGNORE, takenIds, uniqueId } from './graph'
import type { AutomationGraph, AutomationLayout, GraphEdge, GraphNode, GraphTrigger, XY } from './types'

export const CLIPBOARD_KIND = 'iota.automation.clip/1'

export type Clip = {
  kind: typeof CLIPBOARD_KIND
  triggers: GraphTrigger[]
  nodes: GraphNode[]
  edges: GraphEdge[]
  /** Positions relative to the top-left item of the clip. */
  layout: AutomationLayout
}

export function copyItems(graph: AutomationGraph, layout: AutomationLayout, ids: Iterable<string>): Clip | null {
  const selected = new Set(ids)
  const triggers: GraphTrigger[] = []
  const nodes: GraphNode[] = []
  for (const id of selected) {
    const ref = findItem(graph, id)
    if (ref?.kind === 'trigger') triggers.push(structuredClone(ref.item))
    else if (ref?.kind === 'node') nodes.push(structuredClone(ref.item))
  }
  if (triggers.length + nodes.length === 0) return null
  const edges = graph.edges
    .filter((e) => selected.has(e.from.node) && (selected.has(e.to) || e.to === IGNORE))
    .map((e) => structuredClone(e))
  const positions = [...selected].map((id) => layout[id]).filter((p): p is XY => p !== undefined)
  const minX = positions.length > 0 ? Math.min(...positions.map((p) => p.x)) : 0
  const minY = positions.length > 0 ? Math.min(...positions.map((p) => p.y)) : 0
  const relative: AutomationLayout = {}
  for (const id of selected) {
    const p = layout[id]
    if (p) relative[id] = { x: p.x - minX, y: p.y - minY }
  }
  return { kind: CLIPBOARD_KIND, triggers, nodes, edges, layout: relative }
}

export function serializeClip(clip: Clip): string {
  return JSON.stringify(clip)
}

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value)

/** Parses clipboard text; anything that is not an editor clip yields null. */
export function parseClip(text: string): Clip | null {
  let value: unknown
  try {
    value = JSON.parse(text)
  } catch {
    return null
  }
  if (!isRecord(value) || value.kind !== CLIPBOARD_KIND) return null
  const { triggers, nodes, edges, layout } = value
  if (!Array.isArray(triggers) || !Array.isArray(nodes) || !Array.isArray(edges) || !isRecord(layout)) return null
  const validTrigger = (t: unknown): t is GraphTrigger => isRecord(t) && typeof t.id === 'string' && typeof t.event === 'string'
  const validNode = (n: unknown): n is GraphNode => isRecord(n) && typeof n.id === 'string' && typeof n.type === 'string' && isRecord(n.params)
  const validEdge = (e: unknown): e is GraphEdge =>
    isRecord(e) && isRecord(e.from) && typeof e.from.node === 'string' && typeof e.from.outcome === 'string' && typeof e.to === 'string'
  if (!triggers.every(validTrigger) || !nodes.every(validNode) || !edges.every(validEdge)) return null
  const positions: AutomationLayout = {}
  for (const [id, p] of Object.entries(layout)) {
    if (isRecord(p) && typeof p.x === 'number' && typeof p.y === 'number') positions[id] = { x: p.x, y: p.y }
  }
  return { kind: CLIPBOARD_KIND, triggers, nodes, edges, layout: positions }
}

/** Pastes a clip at a flow point; taken ids get `_2`, `_3`… suffixes. Returns the new ids. */
export function pasteClip(
  graph: AutomationGraph,
  layout: AutomationLayout,
  clip: Clip,
  at: XY,
): { graph: AutomationGraph; layout: AutomationLayout; ids: string[] } {
  const taken = takenIds(graph)
  const map = new Map<string, string>()
  const rename = (id: string) => {
    const next = uniqueId(id, taken)
    taken.add(next)
    map.set(id, next)
    return next
  }
  const triggers = clip.triggers.map((t) => ({ ...structuredClone(t), id: rename(t.id) }))
  const nodes = clip.nodes.map((n) => ({ ...structuredClone(n), id: rename(n.id) }))
  const edges = clip.edges
    .filter((e) => map.has(e.from.node) && (map.has(e.to) || e.to === IGNORE))
    .map((e) => ({ from: { node: map.get(e.from.node)!, outcome: e.from.outcome }, to: map.get(e.to) ?? e.to }))
  const nextLayout: AutomationLayout = { ...layout }
  let row = 0
  for (const [oldId, newId] of map) {
    const relative = clip.layout[oldId] ?? { x: 0, y: row++ * GRID * 8 }
    nextLayout[newId] = { x: snap(at.x + relative.x), y: snap(at.y + relative.y) }
  }
  return {
    graph: { ...graph, triggers: [...graph.triggers, ...triggers], nodes: [...graph.nodes, ...nodes], edges: [...graph.edges, ...edges] },
    layout: nextLayout,
    ids: [...map.values()],
  }
}
