import { describe, expect, it } from 'vitest'
import { completeAt, contextPaths, outputPaths, schemaPaths } from './completion'
import type { AutomationGraph, EditorCatalog } from './types'

const catalog: EditorCatalog = {
  revision: '1',
  context_schema: { type: 'object', properties: { user: { type: 'object', properties: { email: { type: ['string', 'null'] }, age: { type: 'integer' } } } } },
  events: [{ key: 'core.user.created.v1', title: 'User created', group: 'Core', subject: 'core.user', payload: { type: 'object', properties: { id: { type: 'integer' }, at: { type: 'string', format: 'date-time' } } }, trigger_params: null }],
  nodes: [
    { type: 'fetch', title: 'Fetch', description: '', group: 'data', behavior: 'call', params: {}, outcomes: ['ok'], outcome_rule: { fixed: ['ok'] }, expressions: [], templates: [], output: { type: 'object', properties: { body: { type: 'string' }, meta: { type: 'object', properties: { status: { type: 'number' } } } } } },
    { type: 'branch', title: 'Branch', description: '', group: 'logic', behavior: 'branch', params: {}, outcomes: ['true', 'false'], outcome_rule: { fixed: ['true', 'false'] }, expressions: [{ param: 'expression', type: 'bool' }], templates: [], output: null },
  ],
}
const graph: AutomationGraph = {
  name: 'g', run_as: 'owner',
  triggers: [{ id: 'user_created', event: 'core.user.created.v1' }],
  nodes: [{ id: 'fetch', type: 'fetch', params: {} }, { id: 'check', type: 'branch', params: {} }, { id: 'later', type: 'fetch', params: {} }],
  edges: [
    { from: { node: 'user_created', outcome: 'fired' }, to: 'fetch' },
    { from: { node: 'fetch', outcome: 'ok' }, to: 'check' },
    { from: { node: 'check', outcome: 'true' }, to: 'later' },
  ],
}

describe('schemaPaths', () => {
  it('walks nested properties and maps types', () => {
    const paths = schemaPaths('', catalog.context_schema, 'context')
    expect(paths.map((p) => [p.path, p.type])).toEqual([['user', 'object'], ['user.email', 'string'], ['user.age', 'int']])
  })
})

describe('contextPaths', () => {
  it('offers context, the event payload and only upstream outputs', () => {
    const paths = contextPaths({ catalog, graph, nodeId: 'check' }).map((p) => p.path)
    expect(paths).toContain('user.email')
    expect(paths).toContain('event.id')
    expect(paths).toContain('event.kind')
    expect(paths).toContain('nodes.fetch.output.meta.status')
    expect(paths.some((p) => p.startsWith('nodes.later'))).toBe(false)
    expect(paths).toContain('now')
  })
  it('has no node outputs for a trigger', () => {
    expect(contextPaths({ catalog, graph, nodeId: 'user_created' }).some((p) => p.source === 'node')).toBe(false)
  })
  it('falls back to event.kind when triggers differ', () => {
    const mixed = { ...graph, triggers: [...graph.triggers, { id: 'other', event: 'x.y' }] }
    const paths = contextPaths({ catalog, graph: mixed, nodeId: null }).map((p) => p.path)
    expect(paths).toContain('event.kind')
    expect(paths).not.toContain('event.id')
  })
})

describe('outputPaths', () => {
  it('lists nodes.<id>.output.*', () => {
    expect(outputPaths('fetch', catalog.nodes[0]!.output).map((p) => p.path)).toEqual(['nodes.fetch.output', 'nodes.fetch.output.body', 'nodes.fetch.output.meta', 'nodes.fetch.output.meta.status'])
    expect(outputPaths('x', null)).toEqual([])
  })
})

describe('completeAt', () => {
  const paths = contextPaths({ catalog, graph, nodeId: 'check' })
  it('completes a dotted CEL prefix', () => {
    const r = completeAt('size(user.em) > 0', 12, 'cel', paths)
    expect(r.from).toBe(5)
    expect(r.to).toBe(12)
    expect(r.items.map((i) => i.path)).toEqual(['user.email'])
  })
  it('completes inside an open template only', () => {
    expect(completeAt('Hello {{ user.', 14, 'template', paths).items.map((i) => i.path)).toContain('user.email')
    expect(completeAt('Hello user.', 11, 'template', paths).items).toEqual([])
    expect(completeAt('{{ user.email }} user.', 22, 'template', paths).items).toEqual([])
  })
  it('does not complete inside string literals', () => {
    expect(completeAt('x == "user.', 11, 'cel', paths).items).toEqual([])
  })
  it('replaces the whole identifier under the caret', () => {
    const r = completeAt('nodes.fe', 8, 'cel', paths)
    expect(r.items[0]!.path).toBe('nodes.fetch.output')
  })
})
