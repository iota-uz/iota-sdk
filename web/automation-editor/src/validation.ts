// Maps the paths of graph errors (`nodes[2].params.expression`, `triggers[0].filter`, `edges[3]`) to
// the canvas item and the field they are about.
import type { AutomationGraph, GraphError } from './types'

export type ErrorTarget = { id: string | null; field: string | null }

export function errorTarget(graph: AutomationGraph, path: string): ErrorTarget {
  const match = /^(nodes|triggers|edges)\[(\d+)\](?:\.(.*))?$/.exec(path)
  if (!match) return { id: null, field: null }
  const [, collection, index, rest] = match
  const i = Number(index)
  if (collection === 'nodes') {
    const node = graph.nodes[i]
    const field = rest?.startsWith('params.') ? rest.slice('params.'.length) : rest ?? null
    return { id: node?.id ?? null, field: field || null }
  }
  if (collection === 'triggers') {
    const trigger = graph.triggers[i]
    const field = rest?.startsWith('params.') ? rest.slice('params.'.length) : rest ?? null
    return { id: trigger?.id ?? null, field: field || null }
  }
  const edge = graph.edges[i]
  return { id: edge?.from.node ?? null, field: null }
}

/** First error message per item, for the canvas badge. */
export function errorsByItem(graph: AutomationGraph, errors: readonly GraphError[] | undefined): Map<string, string> {
  const out = new Map<string, string>()
  for (const error of errors ?? []) {
    const { id } = errorTarget(graph, error.path)
    if (id && !out.has(id)) out.set(id, error.message)
  }
  return out
}

/**
 * The diagnostic code a host's `validate` returns for a step whose connection is unavailable (ED10). Its
 * message names the connection ("Connection «Sales bot» is unavailable…"); the editor marks the step and
 * lists the message. The editor also reports it itself for steps of a catalog type with `connected: false`.
 */
export const CONNECTION_UNAVAILABLE = 'connection_unavailable'

/** Errors the editor finds itself before the server answers: unknown types and types without a connection. */
export function localErrors(
  graph: AutomationGraph,
  lookup: (type: string) => { connected?: boolean; title: string } | undefined,
  messages: { unknown(type: string): string; noConnection(title: string): string },
): GraphError[] {
  const out: GraphError[] = []
  graph.nodes.forEach((node, index) => {
    const spec = lookup(node.type)
    if (!spec) out.push({ path: `nodes[${index}]`, message: messages.unknown(node.type), code: 'unavailable_type' })
    else if (spec.connected === false) out.push({ path: `nodes[${index}].params.connection`, message: messages.noConnection(spec.title), code: CONNECTION_UNAVAILABLE })
  })
  return out
}

/** Items whose errors say their connection is unavailable. */
export function connectionLost(graph: AutomationGraph, errors: readonly GraphError[] | undefined): Set<string> {
  const out = new Set<string>()
  for (const error of errors ?? []) {
    if (error.code !== CONNECTION_UNAVAILABLE) continue
    const { id } = errorTarget(graph, error.path)
    if (id) out.add(id)
  }
  return out
}

/**
 * Moves error paths from the graph they were computed for to another one by item id: an edit while a
 * check runs (or after it) never points an error at a different step. Errors of removed items are dropped.
 */
export function rebaseErrors(errors: readonly GraphError[], from: AutomationGraph, to: AutomationGraph): GraphError[] {
  if (from === to) return [...errors]
  const out: GraphError[] = []
  for (const error of errors) {
    const match = /^(nodes|triggers|edges)\[(\d+)\](.*)$/.exec(error.path)
    if (!match) {
      out.push(error)
      continue
    }
    const [, collection, index, rest] = match as unknown as [string, 'nodes' | 'triggers' | 'edges', string, string]
    const i = Number(index)
    let next = -1
    if (collection === 'edges') {
      const edge = from.edges[i]
      if (edge) next = to.edges.findIndex((e) => e.from.node === edge.from.node && e.from.outcome === edge.from.outcome && e.to === edge.to)
    } else {
      const id = from[collection][i]?.id
      if (id !== undefined) next = to[collection].findIndex((item) => item.id === id)
    }
    if (next >= 0) out.push({ ...error, path: `${collection}[${next}]${rest}` })
  }
  return out
}

/** Validation results are tied to the graph snapshot they were computed for. */
export type ValidationState =
  | { status: 'idle' }
  | { status: 'checking' | 'done' | 'failed'; errors: GraphError[]; warnings: GraphError[]; graph: AutomationGraph }

export function issuesOf(state: ValidationState): { errors: GraphError[]; warnings: GraphError[] } {
  return state.status === 'idle' ? { errors: [], warnings: [] } : { errors: state.errors, warnings: state.warnings }
}
