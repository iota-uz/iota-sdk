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

/** Errors the editor finds itself before the server answers: node types missing from the catalog. */
export function localErrors(graph: AutomationGraph, known: (type: string) => boolean, message: (type: string) => string): GraphError[] {
  const out: GraphError[] = []
  graph.nodes.forEach((node, index) => {
    if (!known(node.type)) out.push({ path: `nodes[${index}]`, message: message(node.type), code: 'unavailable_type' })
  })
  return out
}

export type ValidationState =
  | { status: 'idle' }
  | { status: 'checking'; errors: GraphError[]; warnings: GraphError[] }
  | { status: 'done'; errors: GraphError[]; warnings: GraphError[] }
  | { status: 'failed'; errors: GraphError[]; warnings: GraphError[] }

export function issuesOf(state: ValidationState): { errors: GraphError[]; warnings: GraphError[] } {
  return state.status === 'idle' ? { errors: [], warnings: [] } : { errors: state.errors, warnings: state.warnings }
}
