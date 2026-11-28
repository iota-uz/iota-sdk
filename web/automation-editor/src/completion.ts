// Client-side completion for CEL fields and `{{ }}` templates. The roots follow the Go scope of
// pkg/automation/graph (ExprScope): the subject's context variables at the top level, `event` (the
// trigger kind's payload plus `kind`; a map of dyn when triggers differ), `nodes.<id>.output`,
// `tenant` (a map of dyn) and `now` (the step's time).
import { nodeSpec } from './graph'
import type { AutomationGraph, EditorCatalog, JsonSchema } from './types'

export type CompletionPath = { path: string; type: string; description?: string; source: 'context' | 'trigger' | 'node' }

const MAX_DEPTH = 6

function schemaType(schema: JsonSchema | null | undefined): string {
  if (!schema) return 'dyn'
  const raw = schema.type
  const types = (Array.isArray(raw) ? raw : raw === undefined ? [] : [raw]).filter((t): t is string => typeof t === 'string' && t !== 'null')
  const type = types[0]
  if (!type) return schema.enum ? 'string' : 'dyn'
  if (type === 'string' && schema.format === 'date-time') return 'timestamp'
  switch (type) {
    case 'integer': return 'int'
    case 'number': return 'double'
    case 'boolean': return 'bool'
    case 'object': return schema.properties ? 'object' : 'map'
    case 'array': return 'list'
    default: return type
  }
}

/** Every path of a JSON Schema below a prefix: object properties, recursively. */
export function schemaPaths(prefix: string, schema: JsonSchema | null | undefined, source: CompletionPath['source'], depth = 0): CompletionPath[] {
  if (!schema || depth > MAX_DEPTH) return []
  const out: CompletionPath[] = []
  const properties = schema.properties as Record<string, JsonSchema> | undefined
  if (!properties || typeof properties !== 'object') return out
  for (const [name, child] of Object.entries(properties)) {
    if (!child || typeof child !== 'object') continue
    const path = prefix ? `${prefix}.${name}` : name
    const description = typeof child.description === 'string' ? child.description : typeof child.title === 'string' ? child.title : undefined
    out.push(description ? { path, type: schemaType(child), description, source } : { path, type: schemaType(child), source })
    out.push(...schemaPaths(path, child, source, depth + 1))
  }
  return out
}

/** Paths a node's output offers, `nodes.<id>.output.*`, for the Data tab. */
export function outputPaths(nodeId: string, outputSchema: JsonSchema | null | undefined): CompletionPath[] {
  const root = `nodes.${nodeId}.output`
  if (!outputSchema) return []
  return [{ path: root, type: schemaType(outputSchema), source: 'node' }, ...schemaPaths(root, outputSchema, 'node')]
}

function ancestors(graph: AutomationGraph, id: string): Set<string> {
  const back = new Map<string, string[]>()
  for (const edge of graph.edges) back.set(edge.to, [...(back.get(edge.to) ?? []), edge.from.node])
  const seen = new Set<string>()
  const stack = [...(back.get(id) ?? [])]
  while (stack.length > 0) {
    const current = stack.pop()!
    if (seen.has(current)) continue
    seen.add(current)
    stack.push(...(back.get(current) ?? []))
  }
  seen.delete(id)
  return seen
}

/** Paths an expression of `nodeId` may read: context, the triggers' event and upstream node outputs. */
export function contextPaths(input: { catalog: EditorCatalog; graph: AutomationGraph; nodeId: string | null }): CompletionPath[] {
  const { catalog, graph, nodeId } = input
  const out: CompletionPath[] = [...schemaPaths('', catalog.context_schema, 'context')]
  out.push({ path: 'tenant', type: 'map', source: 'context' })
  out.push({ path: 'now', type: 'timestamp', source: 'context' })
  out.push({ path: 'event', type: 'object', source: 'trigger' })
  out.push({ path: 'event.kind', type: 'string', source: 'trigger' })
  const kinds = [...new Set(graph.triggers.map((t) => t.event))]
  if (kinds.length === 1) {
    const event = catalog.events.find((e) => e.key === kinds[0])
    if (event) out.push(...schemaPaths('event', event.payload, 'trigger').filter((p) => p.path !== 'event.kind'))
  }
  const isNode = nodeId !== null && graph.nodes.some((n) => n.id === nodeId)
  if (isNode) {
    // The graph's memory between runs, read by key in a step (not in a trigger's filter): state.get(key) is the
    // value or null, state.has(key) a bool.
    out.push({ path: 'state', type: 'map', description: 'state.get(key), state.has(key)', source: 'context' })
    const upstream = ancestors(graph, nodeId!)
    for (const node of graph.nodes) {
      if (!upstream.has(node.id)) continue
      const spec = nodeSpec(catalog, node)
      if (spec?.output) out.push(...outputPaths(node.id, spec.output))
    }
  }
  const seen = new Set<string>()
  return out.filter((p) => (seen.has(p.path) ? false : (seen.add(p.path), true)))
}

const PATH_CHAR = /[A-Za-z0-9_.]/

/** The completion at the caret: the dotted identifier before it (inside an open `{{` for templates). */
export function completeAt(
  text: string,
  caret: number,
  mode: 'cel' | 'template',
  paths: readonly CompletionPath[],
): { from: number; to: number; items: CompletionPath[] } {
  const none = { from: caret, to: caret, items: [] as CompletionPath[] }
  const before = text.slice(0, caret)
  if (mode === 'template') {
    const open = before.lastIndexOf('{{')
    if (open < 0 || before.lastIndexOf('}}') > open) return none
  }
  let from = caret
  while (from > 0 && PATH_CHAR.test(text[from - 1]!)) from -= 1
  let to = caret
  while (to < text.length && /[A-Za-z0-9_]/.test(text[to]!)) to += 1
  const prefix = text.slice(from, caret)
  if (mode === 'template') {
    const open = before.lastIndexOf('{{')
    if (from < open + 2) return none
  }
  if (prefix.length > 0 && /^[0-9]/.test(prefix)) return none
  // Inside a string literal no completion.
  const quotes = (before.slice(mode === 'template' ? before.lastIndexOf('{{') : 0).match(/["']/g) ?? []).length
  if (quotes % 2 === 1) return none
  const lower = prefix.toLowerCase()
  const depth = prefix.split('.').length
  const items = paths
    .filter((p) => p.path.toLowerCase().startsWith(lower) && p.path !== prefix)
    // Offer the next segment level first, deeper paths after.
    .sort((a, b) => {
      const da = Math.abs(a.path.split('.').length - depth)
      const db = Math.abs(b.path.split('.').length - depth)
      return da - db || a.path.localeCompare(b.path)
    })
    .slice(0, 50)
  return { from, to, items }
}
