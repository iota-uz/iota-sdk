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

describe('a fix follows its node', () => {
  it('names the same node after a node before it is removed', async () => {
    const { rebaseErrors } = await import('./validation')
    const two = {
      name: 'g', triggers: [], edges: [],
      nodes: [{ id: 'first', type: 'mcp_tool', params: { tool_schema: {} } }, { id: 'second', type: 'mcp_tool', params: { tool_schema: {} } }],
    } as unknown as AutomationGraph
    const warning = { path: 'nodes[1].params.tool_schema', message: 'changed', code: 'tool_schema_changed', fix: { remove: 'nodes[1].params.tool_schema', label: 'Update' } }
    const without = { ...two, nodes: [two.nodes[1]!] } as AutomationGraph
    const [moved] = rebaseErrors([warning], two, without)
    expect(moved?.path).toBe('nodes[0].params.tool_schema')
    expect(moved?.fix?.remove).toBe('nodes[0].params.tool_schema')
    const fixed = applyFix(without, moved!.fix!.remove)
    expect((fixed?.nodes[0]?.params as Record<string, unknown>).tool_schema).toBeUndefined()
    // The flagged node gone: its fix is gone too.
    expect(rebaseErrors([warning], two, { ...two, nodes: [two.nodes[0]!] } as AutomationGraph)).toEqual([])
  })
})
