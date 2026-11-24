// Fixes of validation issues; loaded with the validation panel, not with the canvas.
import type { AutomationGraph } from './types'

/** A fix path the editor can apply: a top-level param of a node ("nodes[2].params.tool_schema"). */
export function removablePath(path: string): { index: number; param: string } | null {
  const m = /^nodes\[(\d+)\]\.params\.([^.[\]]+)$/.exec(path)
  return m ? { index: Number(m[1]), param: m[2]! } : null
}

/** The graph with an issue's fix applied (the param removed), or null when the path is not one it can remove. */
export function applyFix(graph: AutomationGraph, path: string): AutomationGraph | null {
  const at = removablePath(path)
  const node = at ? graph.nodes[at.index] : undefined
  if (!at || !node || !node.params || !(at.param in node.params)) return null
  const { [at.param]: _removed, ...params } = node.params as Record<string, unknown>
  const nodes = graph.nodes.slice()
  nodes[at.index] = { ...node, params }
  return { ...graph, nodes }
}
