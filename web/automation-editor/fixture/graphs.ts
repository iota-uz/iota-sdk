import type { AutomationGraph, AutomationLayout } from '../src/types'

export const sample: AutomationGraph = {
  name: 'Welcome new users',
  description: 'Greets a user and follows up a day later.',
  run_as: 'owner',
  triggers: [
    { id: 'user_created', event: 'core.user.created.v1' },
    { id: 'schedule', event: 'core.schedule', params: { cron: '0 20 * * *', timezone: 'Asia/Tashkent' } },
  ],
  nodes: [
    { id: 'has_email', type: 'branch', params: { expression: 'user.email != ""' } },
    { id: 'send_email', type: 'core.send_email', params: { to: '{{ user.email }}', subject: 'Welcome, {{ user.first_name }}', body: 'Hello!', priority: 'normal' } },
    { id: 'delay', type: 'delay', params: { duration: '1d' } },
    { id: 'end', type: 'end', params: {} },
  ],
  edges: [
    { from: { node: 'user_created', outcome: 'fired' }, to: 'has_email' },
    { from: { node: 'has_email', outcome: 'true' }, to: 'send_email' },
    { from: { node: 'has_email', outcome: 'false' }, to: 'end' },
    { from: { node: 'send_email', outcome: 'sent' }, to: 'delay' },
    { from: { node: 'send_email', outcome: 'failed' }, to: 'ignore' },
  ],
}

/** Every outcome is connected: the case where there is nothing left to drag but rewiring (ED3). */
export const connected: AutomationGraph = {
  name: 'All outcomes connected',
  run_as: 'owner',
  triggers: [{ id: 'user_created', event: 'core.user.created.v1' }],
  nodes: [
    { id: 'check', type: 'branch', params: { expression: 'true' } },
    { id: 'yes', type: 'delay', params: { duration: '1h' } },
    { id: 'no', type: 'delay', params: { duration: '2h' } },
    { id: 'end', type: 'end', params: {} },
  ],
  edges: [
    { from: { node: 'user_created', outcome: 'fired' }, to: 'check' },
    { from: { node: 'check', outcome: 'true' }, to: 'yes' },
    { from: { node: 'check', outcome: 'false' }, to: 'no' },
    { from: { node: 'yes', outcome: 'done' }, to: 'end' },
    { from: { node: 'no', outcome: 'done' }, to: 'end' },
  ],
}

export const empty: AutomationGraph = { name: '', run_as: 'owner', triggers: [], nodes: [], edges: [] }

/** 200 nodes and 500 edges for the benchmark: layered, every node a switch with up to 5 outcomes. */
export function bench(nodes = 200, edges = 500, seed = 0): { graph: AutomationGraph; layout: AutomationLayout } {
  const graph: AutomationGraph = { name: 'Benchmark', run_as: 'owner', triggers: [{ id: 'start', event: 'core.user.created.v1' }], nodes: [], edges: [] }
  const layout: AutomationLayout = { start: { x: 0, y: 0 } }
  const perColumn = 10
  const outcomes = ['a', 'b', 'c']
  for (let i = 0; i < nodes; i += 1) {
    graph.nodes.push({ id: `n${i}`, type: 'switch', params: { expression: seed ? 'event.kind + ""' : 'event.kind', cases: outcomes.map((o) => ({ value: o, outcome: o })) } })
    layout[`n${i}`] = { x: 352 + Math.floor(i / perColumn) * 352, y: (i % perColumn) * 144 + seed * 16 }
  }
  graph.edges.push({ from: { node: 'start', outcome: 'fired' }, to: 'n0' })
  let made = 1
  // Each node: a, b, c and default — 4 outcomes × 200 = 800 slots; fill 499 of them forward.
  const slots = [...outcomes, 'default']
  outer: for (let s = 0; s < slots.length; s += 1) {
    for (let i = 0; i < nodes; i += 1) {
      if (made >= edges) break outer
      const target = Math.min(nodes - 1, i + 1 + ((i * 7 + s * 13 + seed * 5) % 17))
      if (target === i) continue
      graph.edges.push({ from: { node: `n${i}`, outcome: slots[s]! }, to: `n${target}` })
      made += 1
    }
  }
  return { graph, layout }
}

export const sampleLayout: AutomationLayout = {}
