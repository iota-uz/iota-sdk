// Pure graph operations. Nothing here mutates its input: every function returns a new graph.
import { DEFAULT_CRON } from './cron'
import type { AutomationGraph, CatalogEvent, CatalogNode, EditorCatalog, GraphEdge, GraphNode, GraphTrigger } from './types'

export const IGNORE = 'ignore'
export const FIRED = 'fired'

export type ItemRef =
  | { kind: 'trigger'; item: GraphTrigger }
  | { kind: 'node'; item: GraphNode }

export function findItem(graph: AutomationGraph, id: string): ItemRef | null {
  const trigger = graph.triggers.find((t) => t.id === id)
  if (trigger) return { kind: 'trigger', item: trigger }
  const node = graph.nodes.find((n) => n.id === id)
  return node ? { kind: 'node', item: node } : null
}

export function itemIds(graph: AutomationGraph): string[] {
  return [...graph.triggers.map((t) => t.id), ...graph.nodes.map((n) => n.id)]
}

export function catalogNode(catalog: EditorCatalog, type: string): CatalogNode | undefined {
  return catalog.nodes.find((n) => n.type === type)
}

/** The type a node of a front type names (`action` with `{action: "core.user.block"}` → `action.core.user.block`). */
export function dispatchedType(spec: CatalogNode | undefined, params: Record<string, unknown>): string | null {
  const rule = spec?.dispatch
  if (!rule) return null
  const key = params[rule.param]
  return typeof key === 'string' && key.trim() ? rule.prefix + key.trim() : null
}

/**
 * The spec a node behaves as: its own, or for a front type the spec of the type it names (outcomes,
 * output, title, form). A saved node keeps its own type and params; only how it is shown follows.
 */
export function nodeSpec(catalog: EditorCatalog, node: Pick<GraphNode, 'type' | 'params'>): CatalogNode | undefined {
  const own = catalogNode(catalog, node.type)
  const target = dispatchedType(own, node.params)
  return (target ? catalogNode(catalog, target) : undefined) ?? own
}

export function catalogEvent(catalog: EditorCatalog, key: string): CatalogEvent | undefined {
  return catalog.events.find((e) => e.key === key)
}

function stringsOf(value: unknown, field: string | undefined): string[] {
  if (Array.isArray(value)) {
    return value
      .map((entry) => (field && entry && typeof entry === 'object' ? (entry as Record<string, unknown>)[field] : entry))
      .filter((entry): entry is string => typeof entry === 'string' && entry.length > 0)
  }
  if (value && typeof value === 'object') {
    if (field) {
      const nested = (value as Record<string, unknown>)[field]
      if (nested !== undefined) return stringsOf(nested, undefined)
    }
    return Object.keys(value)
  }
  return []
}

/** Outcomes of a node by its catalog rule: fixed, or derived from a param (switch cases), plus fixed ones. */
export function nodeOutcomes(node: GraphNode, spec: CatalogNode | undefined): string[] {
  if (!spec) {
    return []
  }
  const rule = spec.outcome_rule
  let outcomes: string[]
  if (rule && 'from' in rule && rule.from) {
    outcomes = [...stringsOf(node.params[rule.from.param], rule.from.field), ...(rule.fixed ?? [])]
  } else if (rule && 'fixed' in rule && (rule.fixed?.length ?? 0) > 0) {
    outcomes = [...(rule.fixed ?? [])]
  } else {
    outcomes = [...spec.outcomes]
  }
  // An optional outcome exists while its param is set (a wait's timeout).
  const optional = rule?.optional
  if (optional) {
    const value = node.params[optional.param]
    if (value !== undefined && value !== null && value !== '') outcomes.push(optional.outcome)
  }
  return [...new Set(outcomes)]
}

/** Outcomes of a node in the graph: the catalog rule plus any outcome an edge still uses (shown as stale). */
export function itemOutcomes(graph: AutomationGraph, catalog: EditorCatalog, id: string): string[] {
  const ref = findItem(graph, id)
  if (!ref) return []
  if (ref.kind === 'trigger') return [FIRED]
  const outcomes = nodeOutcomes(ref.item, nodeSpec(catalog, ref.item))
  for (const edge of graph.edges) {
    if (edge.from.node === id && !outcomes.includes(edge.from.outcome)) outcomes.push(edge.from.outcome)
  }
  return outcomes
}

export function edgeOf(graph: AutomationGraph, node: string, outcome: string): GraphEdge | undefined {
  return graph.edges.find((e) => e.from.node === node && e.from.outcome === outcome)
}

/** One edge per (node, outcome): a new connection replaces the old one. */
export function connect(graph: AutomationGraph, from: { node: string; outcome: string }, to: string): AutomationGraph {
  const edges = graph.edges.filter((e) => !(e.from.node === from.node && e.from.outcome === from.outcome))
  edges.push({ from: { node: from.node, outcome: from.outcome }, to })
  return { ...graph, edges }
}

export function disconnect(graph: AutomationGraph, from: { node: string; outcome: string }): AutomationGraph {
  return { ...graph, edges: graph.edges.filter((e) => !(e.from.node === from.node && e.from.outcome === from.outcome)) }
}

export function setIgnore(graph: AutomationGraph, from: { node: string; outcome: string }, ignore: boolean): AutomationGraph {
  return ignore ? connect(graph, from, IGNORE) : disconnect(graph, from)
}

/** Removes items and every edge that leaves them or points at them. */
export function removeItems(graph: AutomationGraph, ids: Iterable<string>): AutomationGraph {
  const gone = new Set(ids)
  if (gone.size === 0) return graph
  return {
    ...graph,
    triggers: graph.triggers.filter((t) => !gone.has(t.id)),
    nodes: graph.nodes.filter((n) => !gone.has(n.id)),
    edges: graph.edges.filter((e) => !gone.has(e.from.node) && !gone.has(e.to)),
  }
}

export function updateNode(graph: AutomationGraph, id: string, params: Record<string, unknown>): AutomationGraph {
  return { ...graph, nodes: graph.nodes.map((n) => (n.id === id ? { ...n, params } : n)) }
}

export function updateTrigger(graph: AutomationGraph, id: string, patch: Partial<Omit<GraphTrigger, 'id'>>): AutomationGraph {
  return {
    ...graph,
    triggers: graph.triggers.map((t) => {
      if (t.id !== id) return t
      const next: GraphTrigger = { ...t, ...patch }
      if (next.filter === undefined || next.filter === '') delete next.filter
      if (next.params === undefined) delete next.params
      return next
    }),
  }
}

/** Readable base of a new id: the last segment of a type, or the event's subject verb. */
export function idBase(source: string): string {
  const cleaned = source.replace(/\.v\d+$/, '')
  const parts = cleaned.split(/[./:]/).filter(Boolean)
  let base = parts.length > 2 ? parts.slice(-2).join('_') : (parts[parts.length - 1] ?? 'node')
  base = base.toLowerCase().replace(/[^a-z0-9_]+/g, '_').replace(/^_+|_+$/g, '')
  if (!/^[a-z]/.test(base)) base = `n_${base}`
  return base || 'node'
}

/** `base`, then `base_2`, `base_3`… — never a uuid. */
export function uniqueId(base: string, taken: Set<string>): string {
  if (!taken.has(base) && base !== IGNORE) return base
  for (let i = 2; ; i += 1) {
    const candidate = `${base}_${i}`
    if (!taken.has(candidate)) return candidate
  }
}

export function takenIds(graph: AutomationGraph): Set<string> {
  return new Set(itemIds(graph))
}

/** Default params of a node type: the `default` of every top-level property. */
export function defaultParams(schema: Record<string, unknown> | null | undefined): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  const properties = (schema?.properties ?? {}) as Record<string, Record<string, unknown>>
  for (const [key, property] of Object.entries(properties)) {
    if (property && 'default' in property) out[key] = structuredClone(property.default)
  }
  return out
}

export function addNode(graph: AutomationGraph, spec: CatalogNode, id?: string): { graph: AutomationGraph; id: string } {
  const nodeId = id ?? uniqueId(idBase(spec.type), takenIds(graph))
  const node: GraphNode = { id: nodeId, type: spec.type, params: defaultParams(spec.params) }
  return { graph: { ...graph, nodes: [...graph.nodes, node] }, id: nodeId }
}

export function triggerIdBase(event: CatalogEvent | undefined, key: string): string {
  if (event?.schedule) return 'schedule'
  if (event?.manual) return 'manual'
  return idBase(key)
}

export function addTrigger(graph: AutomationGraph, event: CatalogEvent | undefined, key: string): { graph: AutomationGraph; id: string } {
  const id = uniqueId(triggerIdBase(event, key), takenIds(graph))
  const params = defaultParams(event?.trigger_params ?? null)
  if (event?.schedule && typeof params.cron !== 'string') params.cron = DEFAULT_CRON
  const trigger: GraphTrigger = { id, event: key }
  if (Object.keys(params).length > 0) trigger.params = params
  return { graph: { ...graph, triggers: [...graph.triggers, trigger] }, id }
}

/** Duplicates items with their internal edges; returns the new ids in order. */
export function duplicateItems(graph: AutomationGraph, ids: string[]): { graph: AutomationGraph; ids: string[]; map: Map<string, string> } {
  const taken = takenIds(graph)
  const map = new Map<string, string>()
  const triggers: GraphTrigger[] = []
  const nodes: GraphNode[] = []
  for (const id of ids) {
    const ref = findItem(graph, id)
    if (!ref) continue
    const next = uniqueId(id.replace(/_\d+$/, ''), taken)
    taken.add(next)
    map.set(id, next)
    if (ref.kind === 'trigger') triggers.push({ ...structuredClone(ref.item), id: next })
    else nodes.push({ ...structuredClone(ref.item), id: next })
  }
  const edges = graph.edges
    .filter((e) => map.has(e.from.node) && (map.has(e.to) || e.to === IGNORE))
    .map((e) => ({ from: { node: map.get(e.from.node)!, outcome: e.from.outcome }, to: map.get(e.to) ?? e.to }))
  return {
    graph: { ...graph, triggers: [...graph.triggers, ...triggers], nodes: [...graph.nodes, ...nodes], edges: [...graph.edges, ...edges] },
    ids: [...map.values()],
    map,
  }
}

/** Items in order of reachability: triggers, then a breadth-first walk along edges, then unreachable nodes. */
export function reachabilityOrder(graph: AutomationGraph): string[] {
  const order: string[] = []
  const seen = new Set<string>()
  const queue: string[] = []
  for (const t of graph.triggers) {
    seen.add(t.id)
    order.push(t.id)
    queue.push(t.id)
  }
  const nodeIds = new Set(graph.nodes.map((n) => n.id))
  while (queue.length > 0) {
    const current = queue.shift()!
    for (const edge of graph.edges) {
      if (edge.from.node !== current || !nodeIds.has(edge.to) || seen.has(edge.to)) continue
      seen.add(edge.to)
      order.push(edge.to)
      queue.push(edge.to)
    }
  }
  for (const n of graph.nodes) if (!seen.has(n.id)) order.push(n.id)
  return order
}

/** The triggers from which an item can be reached, following the edges. */
export function triggersReaching(graph: AutomationGraph, id: string): GraphTrigger[] {
  const out: GraphTrigger[] = []
  for (const trigger of graph.triggers) {
    const seen = new Set([trigger.id])
    const queue = [trigger.id]
    while (queue.length > 0) {
      const current = queue.shift()!
      if (current === id) { out.push(trigger); break }
      for (const edge of graph.edges) {
        if (edge.from.node === current && !seen.has(edge.to)) { seen.add(edge.to); queue.push(edge.to) }
      }
    }
  }
  return out
}

/**
 * The participants a node may name: the recipient keys every trigger event that reaches it offers. A node
 * no trigger reaches, or one reached by an event without a notification definition, may name none.
 */
export function offeredParticipants(graph: AutomationGraph, catalog: Pick<EditorCatalog, 'events'>, id: string): string[] {
  const reaching = triggersReaching(graph, id)
  if (reaching.length === 0) return []
  let keys: string[] | null = null
  for (const trigger of reaching) {
    const own = catalog.events.find((e) => e.key === trigger.event)?.participants ?? []
    keys = keys === null ? [...own] : keys.filter((k) => own.includes(k))
  }
  return keys ?? []
}

/** Ancestors and descendants of an item (its path through the graph), for highlighting. */
export function pathOf(graph: AutomationGraph, id: string): { items: Set<string>; edges: Set<string> } {
  const items = new Set<string>([id])
  const edges = new Set<string>()
  const forward = new Map<string, GraphEdge[]>()
  const backward = new Map<string, GraphEdge[]>()
  for (const edge of graph.edges) {
    if (edge.to === IGNORE) continue
    forward.set(edge.from.node, [...(forward.get(edge.from.node) ?? []), edge])
    backward.set(edge.to, [...(backward.get(edge.to) ?? []), edge])
  }
  const walk = (start: string, next: (n: string) => GraphEdge[], pick: (e: GraphEdge) => string) => {
    const stack = [start]
    const seen = new Set([start])
    while (stack.length > 0) {
      const current = stack.pop()!
      for (const edge of next(current)) {
        edges.add(edgeKey(edge.from.node, edge.from.outcome))
        const other = pick(edge)
        items.add(other)
        if (!seen.has(other)) {
          seen.add(other)
          stack.push(other)
        }
      }
    }
  }
  walk(id, (n) => forward.get(n) ?? [], (e) => e.to)
  walk(id, (n) => backward.get(n) ?? [], (e) => e.from.node)
  return { items, edges }
}

export function edgeKey(node: string, outcome: string): string {
  return `${node}.${outcome}`
}

/** The parent to focus after deleting: the source of the first edge into the item. */
export function parentOf(graph: AutomationGraph, id: string): string | null {
  return graph.edges.find((e) => e.to === id)?.from.node ?? null
}

export function graphsEqual(a: AutomationGraph, b: AutomationGraph): boolean {
  return JSON.stringify(a) === JSON.stringify(b)
}
