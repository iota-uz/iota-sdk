import { describe, expect, it } from 'vitest'
import type { AutomationGraph } from './types'
import { applyFix, removablePath } from './fix'

const graph = {
  name: 'g', triggers: [], edges: [],
  nodes: [{ id: 'call', type: 'mcp_tool', params: { tool: 'weather', tool_schema: { hash: 'x', input_schema: {} } } }],
} as unknown as AutomationGraph

describe('issue fixes', () => {
  it('removes a top-level param of a node, and nothing else', () => {
    const next = applyFix(graph, 'nodes[0].params.tool_schema')
    expect(next?.nodes[0]?.params).toEqual({ tool: 'weather' })
    expect((graph.nodes[0]!.params as Record<string, unknown>).tool_schema).toBeDefined()
  })
  it('refuses paths it cannot apply', () => {
    expect(removablePath('nodes[0].params.a.b')).toBeNull()
    expect(removablePath('triggers[0].params.a')).toBeNull()
    expect(applyFix(graph, 'nodes[3].params.tool_schema')).toBeNull()
    expect(applyFix(graph, 'nodes[0].params.missing')).toBeNull()
  })
})
