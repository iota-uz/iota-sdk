// Auto-layout (Tidy v2). The flow runs top to bottom:
//  - every trigger owns a column: the items reachable from that trigger alone are laid out in it, and
//    the items reachable from several triggers (or from none) sit below the columns, centred;
//  - inside a group the items are layered by their longest path, ordered row by row to keep links from
//    crossing (barycentre sweeps, port-aware), and placed so a child stands under the port of the
//    outcome that leads to it;
//  - items entered only through failed / timeout / low_confidence outcomes form a failure lane to the
//    right of the chain they belong to, so the failure branches stay out of the main flow;
//  - long links get a reserved slot in every row they pass, so a router always finds a free channel.
// Cycle edges are found by a DFS and take no part in layering: they run back around the cards.
// Everything is a pure function of the graph and the catalog; positions are snapped to the grid.
import { cardGeometry, CARD_HEIGHT, GRID, rectsOverlap, snap, type CardGeometry, type Rect } from './geometry'
import { findItem, IGNORE, itemIds, itemOutcomes } from './graph'
import type { AutomationGraph, AutomationLayout, EditorCatalog, XY } from './types'

/** Free space between two cards of one row. */
export const COLUMN_GAP = 64
/** Free space between two rows; it grows when many links run through it. */
export const ROW_GAP = 48
/** Space between two trigger columns: twice the layer step. */
export const GROUP_GAP = COLUMN_GAP * 2
/** Space between the trigger columns and the shared items below them: twice the row step. */
export const GROUP_ROW_GAP = ROW_GAP * 2
/** Space between a chain and its failure lane. */
export const LANE_GAP = COLUMN_GAP + 32
/** Outcomes that lead into the failure lane. */
export const FAILURE_OUTCOMES: ReadonlySet<string> = new Set(['failed', 'timeout', 'low_confidence'])
/** The group of the items reachable from several triggers or from none; its key is not a valid trigger id. */
export const SHARED_GROUP = '#shared'

/**
 * What a Tidy leaves in a position: a quarter of a pixel on both coordinates. The layout is saved as a bare
 * id → position map, so this is the only place for "the system put it here": a card that a person moves
 * (drag, Alt+arrows) is snapped or rounded to whole numbers and loses it. The canvas draws positions
 * rounded (`unmarked`), so nothing shows.
 */
export const TIDY_MARK = 0.25
/** Was this position left by Tidy (or by the automatic placing) and not touched since? */
export const isTidied = (p: XY): boolean => p.x - Math.floor(p.x) === TIDY_MARK && p.y - Math.floor(p.y) === TIDY_MARK
/** A position as the editor chose it itself, marked as such. */
export const tidied = (p: XY): XY => ({ x: Math.round(p.x) + TIDY_MARK, y: Math.round(p.y) + TIDY_MARK })
/** The layout as it is drawn: whole numbers, without the mark. */
export function unmarked(layout: AutomationLayout): AutomationLayout {
  // Nothing marked, nothing to copy.
  if (Object.values(layout).every((p) => Number.isInteger(p.x) && Number.isInteger(p.y))) return layout
  const out: AutomationLayout = {}
  for (const [id, p] of Object.entries(layout)) out[id] = { x: Math.round(p.x), y: Math.round(p.y) }
  return out
}

const DUMMY_WIDTH = 16
const DUMMY_GAP = 24
const MAX_ROW_GAP = 160

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

/** Where the edge of an outcome leaves a card, relative to the card; null when it has no such outcome. */
export function portOffset(graph: AutomationGraph, catalog: EditorCatalog, id: string, outcome: string): XY | null {
  const ref = findItem(graph, id)
  if (!ref) return null
  const geometry = cardGeometry(itemOutcomes(graph, catalog, id), ref.kind === 'node')
  return geometry.outcomes.find((slot) => slot.outcome === outcome)?.handle ?? null
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

// ——— analysis: forward links, groups, failure items ———

type Link = { id: string; outcome: string; port: number }
type Flow = {
  ids: string[]
  geometry: Map<string, CardGeometry>
  /** Forward links by source, in port order. */
  out: Map<string, Link[]>
  /** Forward links by target; `id` is the source. */
  inn: Map<string, Link[]>
  /** Items entered only through failure outcomes, or from such items. */
  failure: Set<string>
  /** Group key of every item: a trigger id, or SHARED_GROUP. */
  owner: Map<string, string>
}

/** Items by group, in graph order: one group per trigger, then the shared one. Cycle edges count: they make an item reachable. */
export function layoutGroups(graph: AutomationGraph): { key: string; items: string[] }[] {
  const ids = itemIds(graph)
  const idSet = new Set(ids)
  const out = new Map<string, string[]>()
  for (const edge of graph.edges) {
    if (edge.to === IGNORE || !idSet.has(edge.from.node) || !idSet.has(edge.to)) continue
    const list = out.get(edge.from.node)
    if (list) list.push(edge.to)
    else out.set(edge.from.node, [edge.to])
  }
  const reached = new Map<string, number>()
  const first = new Map<string, string>()
  for (const trigger of graph.triggers) {
    const seen = new Set<string>([trigger.id])
    const stack = [trigger.id]
    while (stack.length > 0) {
      const id = stack.pop()!
      for (const next of out.get(id) ?? []) {
        if (seen.has(next)) continue
        seen.add(next)
        stack.push(next)
      }
    }
    for (const id of seen) {
      reached.set(id, (reached.get(id) ?? 0) + 1)
      if (!first.has(id)) first.set(id, trigger.id)
    }
  }
  const groups = new Map<string, string[]>(graph.triggers.map((t) => [t.id, []]))
  groups.set(SHARED_GROUP, [])
  for (const id of ids) {
    const isTrigger = graph.triggers.some((t) => t.id === id)
    const key = isTrigger ? id : reached.get(id) === 1 ? first.get(id)! : SHARED_GROUP
    groups.get(key)!.push(id)
  }
  return [...groups].filter(([, items]) => items.length > 0).map(([key, items]) => ({ key, items }))
}

function analyse(graph: AutomationGraph, catalog: EditorCatalog): Flow {
  const ids = itemIds(graph)
  const idSet = new Set(ids)
  const back = backEdges(graph)
  const geometry = new Map<string, CardGeometry>()
  for (const trigger of graph.triggers) geometry.set(trigger.id, cardGeometry(itemOutcomes(graph, catalog, trigger.id), false))
  for (const node of graph.nodes) geometry.set(node.id, cardGeometry(itemOutcomes(graph, catalog, node.id), true))
  const out = new Map<string, Link[]>(ids.map((id) => [id, []]))
  const inn = new Map<string, Link[]>(ids.map((id) => [id, []]))
  for (const edge of graph.edges) {
    const from = edge.from.node
    if (edge.to === IGNORE || !idSet.has(from) || !idSet.has(edge.to) || back.has(`${from}->${edge.to}`)) continue
    const port = Math.max(0, geometry.get(from)!.outcomes.findIndex((slot) => slot.outcome === edge.from.outcome))
    out.get(from)!.push({ id: edge.to, outcome: edge.from.outcome, port })
    inn.get(edge.to)!.push({ id: from, outcome: edge.from.outcome, port })
  }
  for (const links of out.values()) links.sort((a, b) => a.port - b.port)

  // Failure items: reached only through a failure outcome. This looks at every edge, cycle edges too: a
  // step that a cycle edge reaches without a failure is not one, whatever layering dropped.
  const triggers = new Set(graph.triggers.map((t) => t.id))
  const fromEdges = new Map<string, { to: string; failure: boolean }[]>()
  const entered = new Set<string>()
  for (const edge of graph.edges) {
    if (edge.to === IGNORE || !idSet.has(edge.from.node) || !idSet.has(edge.to)) continue
    entered.add(edge.to)
    const list = fromEdges.get(edge.from.node)
    const item = { to: edge.to, failure: FAILURE_OUTCOMES.has(edge.from.outcome) }
    if (list) list.push(item)
    else fromEdges.set(edge.from.node, [item])
  }
  const walk = (starts: string[], any: boolean) => {
    const seen = new Set(starts)
    const stack = [...starts]
    while (stack.length > 0) {
      for (const next of fromEdges.get(stack.pop()!) ?? []) {
        if ((!any && next.failure) || seen.has(next.to)) continue
        seen.add(next.to)
        stack.push(next.to)
      }
    }
    return seen
  }
  // Starts: the triggers, and steps nothing enters.
  const starts = ids.filter((id) => triggers.has(id) || !entered.has(id))
  const reachable = walk(starts, true)
  const safe = walk(starts, false)
  const failure = new Set(ids.filter((id) => !triggers.has(id) && reachable.has(id) && !safe.has(id)))

  const owner = new Map<string, string>()
  for (const group of layoutGroups(graph)) for (const id of group.items) owner.set(id, group.key)
  return { ids, geometry, out, inn, failure, owner }
}

// ——— one group: layers, order, coordinates ———

type VNode = { id: string; width: number; dummy: boolean; lane: boolean; layer: number }
type VLink = { from: string; to: string; dx: number; frac: number; weight: number }
type Virtual = { nodes: Map<string, VNode>; inn: Map<string, VLink[]>; out: Map<string, VLink[]>; depth: number }

type GroupLayout = {
  /** Centre x of every real item, in the group's own coordinates, and its layer. */
  items: Map<string, { cx: number; layer: number }>
  /** Free space under every layer, from the links that run through it. */
  gaps: number[]
  depth: number
  left: number
  right: number
}

const mean = (values: number[]) => values.reduce((a, b) => a + b, 0) / values.length

function separation(a: VNode, b: VNode): number {
  return (a.width + b.width) / 2 + (a.dummy || b.dummy ? DUMMY_GAP : COLUMN_GAP)
}

/**
 * Centres for a row: as close to `desired` as the minimum separations allow, in order. This is the
 * isotonic regression of the desired offsets (pool adjacent violators); `lower` bounds the first centre.
 */
function spread(desired: number[], seps: number[], lower = -Infinity): number[] {
  const n = desired.length
  if (n === 0) return []
  const offset = new Array<number>(n)
  offset[0] = 0
  for (let i = 1; i < n; i += 1) offset[i] = offset[i - 1]! + seps[i - 1]!
  const blocks: { sum: number; count: number }[] = []
  for (let i = 0; i < n; i += 1) {
    blocks.push({ sum: desired[i]! - offset[i]!, count: 1 })
    while (blocks.length > 1) {
      const last = blocks[blocks.length - 1]!
      const prev = blocks[blocks.length - 2]!
      if (prev.sum / prev.count <= last.sum / last.count) break
      prev.sum += last.sum
      prev.count += last.count
      blocks.pop()
    }
  }
  const out: number[] = []
  let at = 0
  for (const block of blocks) {
    const level = Math.max(block.sum / block.count, lower)
    for (let k = 0; k < block.count; k += 1) {
      out.push(level + offset[at]!)
      at += 1
    }
  }
  return out
}

function buildVirtual(flow: Flow, members: string[], layer: Map<string, number>): Virtual {
  const nodes = new Map<string, VNode>()
  const inn = new Map<string, VLink[]>()
  const out = new Map<string, VLink[]>()
  let depth = 0
  for (const id of members) {
    const l = layer.get(id)!
    depth = Math.max(depth, l + 1)
    nodes.set(id, { id, width: flow.geometry.get(id)!.width, dummy: false, lane: flow.failure.has(id), layer: l })
    inn.set(id, [])
    out.set(id, [])
  }
  const add = (link: VLink) => {
    out.get(link.from)!.push(link)
    inn.get(link.to)!.push(link)
  }
  const member = new Set(members)
  for (const u of members) {
    const geometry = flow.geometry.get(u)!
    const ports = geometry.outcomes.length
    const long: Link[] = []
    for (const link of flow.out.get(u)!) {
      if (!member.has(link.id)) continue
      const span = layer.get(link.id)! - layer.get(u)!
      const dx = geometry.outcomes[link.port]!.handle.x - geometry.width / 2
      if (span <= 1) add({ from: u, to: link.id, dx, frac: (link.port + 1) / (ports + 1), weight: 1 })
      else long.push(link)
    }
    if (long.length === 0) continue
    // One chain of slots per source, shared by all its long links: they leave together, as one trunk.
    const reach = Math.max(...long.map((l) => layer.get(l.id)!)) - 1
    const lane = long.every((l) => flow.failure.has(l.id))
    const chain: string[] = []
    for (let k = layer.get(u)! + 1; k <= reach; k += 1) {
      const id = `${u}\u0001${k}`
      nodes.set(id, { id, width: DUMMY_WIDTH, dummy: true, lane, layer: k })
      inn.set(id, [])
      out.set(id, [])
      chain.push(id)
    }
    const first = long[0]!
    add({ from: u, to: chain[0]!, dx: geometry.outcomes[first.port]!.handle.x - geometry.width / 2, frac: (first.port + 1) / (ports + 1), weight: 2 })
    for (let k = 1; k < chain.length; k += 1) add({ from: chain[k - 1]!, to: chain[k]!, dx: 0, frac: 0.5, weight: 2 })
    for (const link of long) add({ from: chain[layer.get(link.id)! - layer.get(u)! - 2]!, to: link.id, dx: 0, frac: 0.5, weight: 1 })
    depth = Math.max(depth, reach + 1)
  }
  for (const links of out.values()) links.sort((a, b) => a.frac - b.frac)
  return { nodes, inn, out, depth }
}

/** Number of crossing link pairs between two neighbouring rows (inversions of target order by source order). */
function crossingsBetween(links: [number, number][]): number {
  if (links.length < 2) return 0
  links.sort((a, b) => a[0] - b[0] || a[1] - b[1])
  const width = links.reduce((m, l) => Math.max(m, l[1]), 0) + 2
  const tree = new Array<number>(width + 1).fill(0)
  let count = 0
  let seen = 0
  for (const [, t] of links) {
    let below = 0
    for (let i = t + 1; i > 0; i -= i & -i) below += tree[i]!
    count += seen - below
    for (let i = t + 1; i <= width; i += i & -i) tree[i]! += 1
    seen += 1
  }
  return count
}

function orderRows(vg: Virtual, layers: string[][], pinned: boolean): string[][] {
  const position = new Map<string, number>()
  const index = (rows: string[][]) => rows.forEach((row) => row.forEach((id, i) => position.set(id, i)))
  const total = (rows: string[][]) => {
    index(rows)
    let sum = 0
    for (let l = 0; l + 1 < rows.length; l += 1) {
      const links: [number, number][] = []
      for (const id of rows[l]!) {
        for (const link of vg.out.get(id)!) {
          const to = position.get(link.to)
          if (to !== undefined && !vg.nodes.get(link.to)!.lane) links.push([position.get(id)! + link.frac, to])
        }
      }
      sum += crossingsBetween(links)
    }
    return sum
  }
  let best = layers.map((row) => [...row])
  let bestCrossings = total(best)
  for (let sweep = 0; sweep < 8 && bestCrossings > 0; sweep += 1) {
    const next = best.map((row) => [...row])
    index(next)
    const reorder = (l: number, key: (id: string) => number | null) => {
      const keyed = next[l]!.map((id, i) => ({ id, i, key: key(id) ?? i }))
      keyed.sort((a, b) => a.key - b.key || a.i - b.i)
      next[l] = keyed.map((k) => k.id)
      next[l]!.forEach((id, i) => position.set(id, i))
    }
    if (sweep % 2 === 0) {
      for (let l = 1; l < next.length; l += 1) {
        reorder(l, (id) => {
          const keys = vg.inn.get(id)!.filter((link) => !vg.nodes.get(link.from)!.lane).map((link) => position.get(link.from)! + link.frac)
          return keys.length > 0 ? mean(keys) : null
        })
      }
    } else {
      for (let l = next.length - 2; l >= (pinned ? 1 : 0); l -= 1) {
        reorder(l, (id) => {
          const keys = vg.out.get(id)!.filter((link) => !vg.nodes.get(link.to)!.lane).map((link) => position.get(link.to)!)
          return keys.length > 0 ? mean(keys) : null
        })
      }
    }
    const crossings = total(next)
    if (crossings < bestCrossings) {
      best = next
      bestCrossings = crossings
    }
  }
  return best
}

function layoutGroup(flow: Flow, members: string[], external: Map<string, number[]> | null): GroupLayout {
  const member = new Set(members)
  // Longest-path layers inside the group.
  const layer = new Map<string, number>()
  const pending = new Map<string, number>()
  for (const id of members) pending.set(id, flow.inn.get(id)!.filter((l) => member.has(l.id)).length)
  const queue = members.filter((id) => pending.get(id) === 0)
  for (const id of queue) layer.set(id, 0)
  for (let head = 0; head < queue.length; head += 1) {
    const id = queue[head]!
    for (const next of flow.out.get(id)!) {
      if (!member.has(next.id)) continue
      layer.set(next.id, Math.max(layer.get(next.id) ?? 0, layer.get(id)! + 1))
      pending.set(next.id, pending.get(next.id)! - 1)
      if (pending.get(next.id) === 0) queue.push(next.id)
    }
  }
  for (const id of members) if (!layer.has(id)) layer.set(id, 0)

  const vg = buildVirtual(flow, members, layer)
  const lane = (id: string) => vg.nodes.get(id)!.lane
  const depth = vg.depth

  // Initial order: depth-first in port order, so a node's first outcome comes first.
  const roots = members.filter((id) => !lane(id) && vg.inn.get(id)!.length === 0)
  if (external) roots.sort((a, b) => mean(external.get(a) ?? [0]) - mean(external.get(b) ?? [0]))
  let layers: string[][] = Array.from({ length: depth }, () => [])
  const visited = new Set<string>()
  const visit = (start: string) => {
    const stack = [start]
    while (stack.length > 0) {
      const id = stack.pop()!
      if (visited.has(id) || lane(id)) continue
      visited.add(id)
      layers[vg.nodes.get(id)!.layer]!.push(id)
      const children = vg.out.get(id)!
      for (let i = children.length - 1; i >= 0; i -= 1) stack.push(children[i]!.to)
    }
  }
  for (const root of roots) visit(root)
  for (const id of vg.nodes.keys()) if (!lane(id) && !visited.has(id)) visit(id)
  layers = orderRows(vg, layers, external !== null)

  // Centres of the main chain.
  const centre = new Map<string, number>()
  const seps = (row: string[]) => row.slice(1).map((id, i) => separation(vg.nodes.get(row[i]!)!, vg.nodes.get(id)!))
  for (const row of layers) {
    const placed = spread(row.map(() => 0), seps(row))
    const shift = row.length > 0 ? (placed[0]! + placed[placed.length - 1]!) / 2 : 0
    row.forEach((id, i) => centre.set(id, placed[i]! - shift))
  }
  const apply = (row: string[], desired: (id: string) => number | null) => {
    const wanted = row.map((id) => desired(id) ?? centre.get(id)!)
    spread(wanted, seps(row)).forEach((x, i) => centre.set(row[i]!, x))
  }
  const downward = () => {
    for (let l = 0; l < layers.length; l += 1) {
      apply(layers[l]!, (id) => {
        const wanted = vg.inn.get(id)!.filter((link) => centre.has(link.from) && !lane(link.from)).map((link) => centre.get(link.from)! + link.dx)
        for (const x of external?.get(id) ?? []) wanted.push(x)
        return wanted.length > 0 ? mean(wanted) : null
      })
    }
  }
  const upward = () => {
    for (let l = layers.length - 2; l >= 0; l -= 1) {
      apply(layers[l]!, (id) => {
        const wanted = vg.out.get(id)!.filter((link) => !lane(link.to)).map((link) => centre.get(link.to)! - link.dx)
        return wanted.length > 0 ? mean(wanted) : null
      })
    }
  }
  const cost = () => {
    let sum = 0
    for (const links of vg.out.values()) {
      for (const link of links) if (centre.has(link.from) && centre.has(link.to)) sum += Math.abs(centre.get(link.to)! - centre.get(link.from)! - link.dx) * link.weight
    }
    return sum
  }
  downward()
  let bestCost = cost()
  let best = new Map(centre)
  for (let round = 0; round < 3; round += 1) {
    upward()
    downward()
    const c = cost()
    if (c < bestCost) {
      bestCost = c
      best = new Map(centre)
    }
  }
  for (const [id, x] of best) centre.set(id, x)

  // The failure lane: to the right of the whole chain, each row ordered by where its parents are.
  const main = [...centre.entries()]
  const mainRight = main.length > 0 ? Math.max(...main.map(([id, x]) => x + vg.nodes.get(id)!.width / 2)) : -Infinity
  const laneRows: string[][] = Array.from({ length: depth }, () => [])
  const orderIndex = new Map(members.map((id, i) => [id, i]))
  for (const [id, node] of vg.nodes) if (node.lane) laneRows[node.layer]!.push(id)
  for (let l = 0; l < depth; l += 1) {
    const row = laneRows[l]!
    if (row.length === 0) continue
    const wanted = (id: string) => {
      const xs = vg.inn.get(id)!.filter((link) => centre.has(link.from)).map((link) => centre.get(link.from)! + link.dx)
      for (const x of external?.get(id) ?? []) xs.push(x)
      return xs.length > 0 ? mean(xs) : mainRight
    }
    const keyed = row.map((id) => ({ id, key: wanted(id), i: orderIndex.get(id) ?? 0 }))
    keyed.sort((a, b) => a.key - b.key || a.i - b.i)
    const sorted = keyed.map((k) => k.id)
    const first = vg.nodes.get(sorted[0]!)!
    const lower = Number.isFinite(mainRight) ? mainRight + LANE_GAP + first.width / 2 : -Infinity
    spread(keyed.map((k) => k.key), seps(sorted), lower).forEach((x, i) => centre.set(sorted[i]!, x))
  }

  // Free space under each row, from the links that run through it.
  const spans: [number, number][][] = Array.from({ length: depth }, () => [])
  for (const [id, node] of vg.nodes) {
    let lo = Infinity
    let hi = -Infinity
    for (const link of vg.out.get(id)!) {
      const a = centre.get(id)! + link.dx
      const b = centre.get(link.to)!
      if (Math.abs(a - b) < 1) continue
      lo = Math.min(lo, a, b)
      hi = Math.max(hi, a, b)
    }
    if (hi > lo) spans[node.layer]!.push([lo, hi])
  }
  const gaps: number[] = []
  for (let l = 0; l + 1 < depth; l += 1) {
    const events = spans[l]!.flatMap(([lo, hi]) => [[lo, 1], [hi + 8, -1]] as [number, number][]).sort((a, b) => a[0] - b[0] || a[1] - b[1])
    let active = 0
    let busiest = 0
    for (const [, delta] of events) {
      active += delta
      busiest = Math.max(busiest, active)
    }
    gaps.push(Math.min(MAX_ROW_GAP, Math.max(ROW_GAP, Math.ceil((24 + 8 * busiest) / GRID) * GRID)))
  }

  const items = new Map<string, { cx: number; layer: number }>()
  let left = Infinity
  let right = -Infinity
  for (const [id, x] of centre) {
    const node = vg.nodes.get(id)!
    left = Math.min(left, x - node.width / 2)
    right = Math.max(right, x + node.width / 2)
    if (!node.dummy) items.set(id, { cx: x, layer: node.layer })
  }
  return { items, gaps, depth, left, right }
}

/** Top-to-bottom layout of the whole graph: a column per trigger, shared items below, snapped to the grid. */
export function tidyLayout(graph: AutomationGraph, catalog: EditorCatalog): AutomationLayout {
  const flow = analyse(graph, catalog)
  if (flow.ids.length === 0) return {}
  const groups = layoutGroups(graph)
  const columns = groups.filter((g) => g.key !== SHARED_GROUP).map((g) => ({ group: g, result: layoutGroup(flow, g.items, null) }))
  const shared = groups.find((g) => g.key === SHARED_GROUP)

  const centre = new Map<string, number>()
  const row = new Map<string, { layer: number; below: boolean }>()
  const place = (result: GroupLayout, shift: number, below: boolean) => {
    for (const [id, item] of result.items) {
      centre.set(id, item.cx + shift)
      row.set(id, { layer: item.layer, below })
    }
  }

  // Columns side by side; their rows are shared so the same layer stands at the same height.
  let cursor = 0
  const shifts: number[] = []
  for (const column of columns) {
    shifts.push(cursor - column.result.left)
    cursor += column.result.right - column.result.left + GROUP_GAP
  }
  const span = columns.length > 0 ? { left: 0, right: cursor - GROUP_GAP } : { left: 0, right: 0 }
  const depth = Math.max(0, ...columns.map((c) => c.result.depth))
  const gapAfter = Array.from({ length: depth }, (_, l) => Math.max(ROW_GAP, ...columns.map((c) => c.result.gaps[l] ?? 0)))
  const rowY = (gaps: number[], base: number) => {
    const ys: number[] = [base]
    for (let l = 1; l <= gaps.length + 1; l += 1) ys.push(ys[l - 1]! + CARD_HEIGHT + (gaps[l - 1] ?? ROW_GAP))
    return ys
  }
  const columnRows = rowY(gapAfter, 0)
  columns.forEach((column, i) => place(column.result, shifts[i]!, false))
  let bottom = 0
  for (const column of columns) {
    for (const item of column.result.items.values()) bottom = Math.max(bottom, columnRows[item.layer]! + CARD_HEIGHT)
  }

  let sharedRows: number[] = []
  if (shared) {
    // Roots of the shared group are ordered by the ports that lead into them; the block is centred
    // under the columns that feed it (under all of them when nothing feeds it).
    const external = new Map<string, number[]>()
    const feeders: number[] = []
    for (const id of shared.items) {
      for (const link of flow.inn.get(id)!) {
        if (flow.owner.get(link.id) === SHARED_GROUP || !centre.has(link.id)) continue
        const g = flow.geometry.get(link.id)!
        const x = centre.get(link.id)! + g.outcomes[link.port]!.handle.x - g.width / 2
        const xs = external.get(id)
        if (xs) xs.push(x)
        else external.set(id, [x])
        feeders.push(x)
      }
    }
    const result = layoutGroup(flow, shared.items, columns.length > 0 ? external : null)
    const anchor = feeders.length > 0 ? (Math.min(...feeders) + Math.max(...feeders)) / 2 : (span.left + span.right) / 2
    const shift = anchor - (result.left + result.right) / 2
    sharedRows = rowY(result.gaps, columns.length > 0 ? bottom + GROUP_ROW_GAP : 0)
    place(result, shift, true)
  }

  const layout: AutomationLayout = {}
  for (const id of flow.ids) {
    const r = row.get(id)
    const c = centre.get(id)
    if (!r || c === undefined) continue
    layout[id] = { x: snap(c - flow.geometry.get(id)!.width / 2), y: r.below ? sharedRows[r.layer]! : columnRows[r.layer]! }
  }
  // Normalise to the origin, and mark every place as Tidy's own.
  const minX = Math.min(...Object.values(layout).map((p) => p.x))
  const minY = Math.min(...Object.values(layout).map((p) => p.y))
  for (const id of Object.keys(layout)) layout[id] = tidied({ x: snap(layout[id]!.x - minX), y: snap(layout[id]!.y - minY) })
  return layout
}

// ——— placing and completing ———

/** A free grid position for a new card: under the port it hangs from, or right of the triggers / below the cards. */
export function placeNew(
  graph: AutomationGraph,
  catalog: EditorCatalog,
  layout: AutomationLayout,
  id: string,
  options: { source?: string | null; outcome?: string | null; near?: XY | null } = {},
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
  } else if (sourceRect && options.source) {
    const outcome = options.outcome ?? graph.edges.find((e) => e.to === id && e.from.node === options.source)?.from.outcome ?? null
    const port = outcome ? portOffset(graph, catalog, options.source, outcome) : null
    const portX = port ? port.x : sourceRect.width / 2
    start = { x: snap(sourceRect.x + portX - size.width / 2), y: snap(sourceRect.y + sourceRect.height + ROW_GAP) }
  } else if (isTrigger) {
    const triggerRects = graph.triggers.map((t) => itemRect(graph, catalog, layout, t.id)).filter((r): r is Rect => r !== null)
    const top = triggerRects.length > 0 ? Math.min(...triggerRects.map((r) => r.y)) : 0
    start = { x: occupied.length > 0 ? snap(Math.max(...occupied.map((r) => r.x + r.width)) + GROUP_GAP) : 0, y: snap(top) }
  } else if (occupied.length > 0) {
    start = { x: snap(Math.min(...occupied.map((r) => r.x))), y: snap(Math.max(...occupied.map((r) => r.y + r.height)) + GROUP_ROW_GAP) }
  } else {
    start = { x: 0, y: 0 }
  }
  const free = (p: XY) => !occupied.some((r) => rectsOverlap({ ...p, ...size }, r, GRID))
  if (free(start)) return start
  const stepX = size.width + COLUMN_GAP
  const stepY = CARD_HEIGHT + ROW_GAP
  for (let r = 0; r < 100; r += 1) {
    for (let k = 0; k <= 12; k += 1) {
      for (const sign of k === 0 ? [1] : [1, -1]) {
        const candidate = { x: snap(start.x + sign * k * stepX), y: snap(start.y + r * stepY) }
        if (free(candidate)) return candidate
      }
    }
  }
  // Everything near was taken: below every card is free by construction.
  const bottom = occupied.reduce((m, r) => Math.max(m, r.y + r.height), start.y)
  return { x: start.x, y: snap(bottom + GROUP_ROW_GAP) }
}

function connectedMissing(graph: AutomationGraph, missing: Set<string>): number {
  // The size of the biggest group of missing items linked to each other: a branch without coordinates (two linked steps are one).
  const parent = new Map<string, string>()
  const find = (id: string): string => {
    let root = id
    while (parent.get(root) !== root) root = parent.get(root)!
    parent.set(id, root)
    return root
  }
  for (const id of missing) parent.set(id, id)
  for (const edge of graph.edges) {
    if (edge.to === IGNORE || !missing.has(edge.from.node) || !missing.has(edge.to)) continue
    parent.set(find(edge.from.node), find(edge.to))
  }
  const sizes = new Map<string, number>()
  let biggest = 0
  for (const id of missing) {
    const root = find(id)
    const size = (sizes.get(root) ?? 0) + 1
    sizes.set(root, size)
    biggest = Math.max(biggest, size)
  }
  return biggest
}

/** Groups with a place a person set: one that no Tidy (or automatic placing) left, so it carries no mark. */
function arrangedByHand(graph: AutomationGraph, layout: AutomationLayout, known: Set<string>): Set<string> {
  const hand = new Set<string>()
  for (const group of layoutGroups(graph)) {
    if (group.items.some((id) => known.has(id) && !isTidied(layout[id]!))) hand.add(group.key)
  }
  return hand
}

/**
 * Every item gets a position. A layout without positions is tidied. Missing items are placed under
 * their source, unless more than a third of the items or a whole branch lack positions: then the graph
 * is laid out again, and the groups the person arranged by hand (a position without Tidy's mark) keep
 * theirs.
 */
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
  const missingSet = new Set(missing)
  const again = missing.length * 3 > ids.length || connectedMissing(graph, missingSet) >= 2
  const next: AutomationLayout = {}
  for (const id of known) next[id] = layout[id]!
  if (!again) return completeByPlacing(graph, catalog, next, missing)

  const knownSet = new Set(known)
  const fresh = tidyLayout(graph, catalog)
  const hand = arrangedByHand(graph, layout, knownSet)
  if (hand.size === 0) return fresh
  const groups = layoutGroups(graph)
  const kept = new Set(groups.filter((g) => hand.has(g.key)).flatMap((g) => g.items.filter((id) => knownSet.has(id))))
  const relaid = groups.filter((g) => !hand.has(g.key)).flatMap((g) => g.items)
  const result: AutomationLayout = {}
  for (const id of kept) result[id] = layout[id]!
  if (relaid.length > 0) {
    // The groups laid out again go to the right of the ones kept, at their height.
    const keptRects = [...kept].map((id) => itemRect(graph, catalog, layout, id)).filter((r): r is Rect => r !== null)
    const right = keptRects.length > 0 ? Math.max(...keptRects.map((r) => r.x + r.width)) + GROUP_GAP : 0
    const top = keptRects.length > 0 ? Math.min(...keptRects.map((r) => r.y)) : 0
    const minX = Math.min(...relaid.map((id) => fresh[id]!.x))
    const minY = Math.min(...relaid.map((id) => fresh[id]!.y))
    for (const id of relaid) result[id] = tidied({ x: snap(fresh[id]!.x - minX + right), y: snap(fresh[id]!.y - minY + top) })
  }
  return completeByPlacing(graph, catalog, result, ids.filter((id) => !result[id]))
}

function completeByPlacing(graph: AutomationGraph, catalog: EditorCatalog, positions: AutomationLayout, missing: string[]): AutomationLayout {
  // An item whose source already has a position goes first; a branch is placed from its top down.
  const pending = [...missing]
  while (pending.length > 0) {
    let index = pending.findIndex((id) => graph.edges.some((e) => e.to === id && positions[e.from.node]))
    if (index < 0) index = 0
    const id = pending.splice(index, 1)[0]!
    const edge = graph.edges.find((e) => e.to === id && positions[e.from.node])
    positions[id] = tidied(placeNew(graph, catalog, positions, id, { source: edge?.from.node ?? null, outcome: edge?.from.outcome ?? null }))
  }
  const ordered: AutomationLayout = {}
  for (const id of itemIds(graph)) if (positions[id]) ordered[id] = positions[id]!
  return ordered
}
