import { describe, expect, it } from 'vitest'
import { catalog } from '../fixture/catalog'
import { bench, connected, sample } from '../fixture/graphs'
import { copyItems, parseClip, pasteClip, serializeClip } from './clipboard'
import { cardGeometry, CARD_HEIGHT, CARD_MIN_WIDTH, GRID, rectsOverlap, snap } from './geometry'
import { addNode, addTrigger, connect, duplicateItems, edgeOf, idBase, nodeOutcomes, pathOf, reachabilityOrder, removeItems, setIgnore, uniqueId } from './graph'
import { canRedo, canUndo, createHistory, record, redo, undo } from './history'
import { backEdges, ensureLayout, itemSize, placeNew, tidyLayout } from './layout'
import type { AutomationGraph, AutomationLayout, GraphEdge } from './types'
import { connectionLost, CONNECTION_UNAVAILABLE, errorsByItem, errorTarget, localErrors, rebaseErrors } from './validation'

const spec = (type: string) => catalog.nodes.find((n) => n.type === type)!

describe('geometry', () => {
  it('sizes cards from outcomes with a fixed height, on the grid', () => {
    const one = cardGeometry(['fired'], false)
    expect(one.width).toBe(CARD_MIN_WIDTH)
    expect(one.height).toBe(CARD_HEIGHT)
    expect(one.target).toBeNull()
    const many = cardGeometry(['first_case', 'second_case', 'third_case', 'fourth_case', 'default'], true)
    expect(many.width).toBeGreaterThan(CARD_MIN_WIDTH)
    expect(many.width % GRID).toBe(0)
    expect(many.height).toBe(CARD_HEIGHT)
    for (const slot of many.outcomes) {
      expect(slot.handle.y).toBe(CARD_HEIGHT)
      expect(slot.x).toBeGreaterThanOrEqual(0)
      expect(slot.x + slot.width).toBeLessThanOrEqual(many.width)
    }
  })

  it('never grows past the widest card: slots shrink', () => {
    const huge = cardGeometry(Array.from({ length: 30 }, (_, i) => `outcome_number_${i}`), true)
    expect(huge.width).toBe(640)
    const last = huge.outcomes[huge.outcomes.length - 1]!
    expect(last.x + last.width).toBeLessThanOrEqual(640)
  })

  it('snaps to the 16 px grid', () => {
    expect(snap(7)).toBe(0)
    expect(snap(9)).toBe(16)
    expect(snap(-9)).toBe(-16)
  })
})

describe('graph operations', () => {
  it('one edge per (node, outcome): connecting replaces', () => {
    const next = connect(connected, { node: 'check', outcome: 'true' }, 'no')
    expect(next.edges.filter((e) => e.from.node === 'check' && e.from.outcome === 'true')).toEqual([{ from: { node: 'check', outcome: 'true' }, to: 'no' }])
    expect(next.edges).toHaveLength(connected.edges.length)
    expect(connected.edges.find((e) => e.from.node === 'check' && e.from.outcome === 'true')!.to).toBe('yes')
  })

  it('marks an outcome ignored and back', () => {
    const ignored = setIgnore(connected, { node: 'check', outcome: 'true' }, true)
    expect(edgeOf(ignored, 'check', 'true')!.to).toBe('ignore')
    expect(edgeOf(setIgnore(ignored, { node: 'check', outcome: 'true' }, false), 'check', 'true')).toBeUndefined()
  })

  it('removing a step removes the edges into and out of it', () => {
    const next = removeItems(connected, ['yes'])
    expect(next.nodes.map((n) => n.id)).not.toContain('yes')
    expect(next.edges.some((e) => e.to === 'yes' || e.from.node === 'yes')).toBe(false)
  })

  it('outcomes follow the catalog rule: fixed, from a param, optional', () => {
    expect(nodeOutcomes({ id: 'b', type: 'branch', params: {} }, spec('branch'))).toEqual(['true', 'false'])
    expect(nodeOutcomes({ id: 's', type: 'switch', params: { cases: [{ value: 'x', outcome: 'vip' }, { value: 'y', outcome: 'new' }] } }, spec('switch'))).toEqual(['vip', 'new', 'default'])
    expect(nodeOutcomes({ id: 'w', type: 'wait', params: { event: 'e' } }, spec('wait'))).toEqual(['event'])
    expect(nodeOutcomes({ id: 'w', type: 'wait', params: { event: 'e', timeout: '1h' } }, spec('wait'))).toEqual(['event', 'timeout'])
  })

  it('new ids are readable and never collide', () => {
    expect(idBase('core.user.created.v1')).toBe('user_created')
    expect(idBase('core.send_email')).toBe('send_email')
    expect(uniqueId('schedule', new Set(['schedule']))).toBe('schedule_2')
    expect(uniqueId('schedule', new Set(['schedule', 'schedule_2']))).toBe('schedule_3')
    const event = catalog.events.find((e) => e.schedule)!
    const first = addTrigger(sample, event, event.key)
    expect(first.id).toBe('schedule_2')
    expect(first.graph.triggers.at(-1)!.params).toEqual({ cron: '0 9 * * *' })
    const node = addNode(sample, spec('delay'))
    expect(node.id).toBe('delay_2')
    expect(node.graph.nodes.at(-1)!.params).toEqual({ duration: '1h' })
  })

  it('duplicates items with their internal edges only', () => {
    const result = duplicateItems(connected, ['check', 'yes'])
    expect(result.ids).toEqual(['check_2', 'yes_2'])
    const copied = result.graph.edges.filter((e) => e.from.node === 'check_2' || e.from.node === 'yes_2')
    expect(copied).toEqual([{ from: { node: 'check_2', outcome: 'true' }, to: 'yes_2' }])
  })

  it('orders items by reachability for the outline', () => {
    expect(reachabilityOrder(sample)).toEqual(['user_created', 'schedule', 'has_email', 'send_email', 'end', 'delay'])
  })

  it('the path of a step covers its ancestors and descendants', () => {
    const path = pathOf(connected, 'yes')
    expect([...path.items].sort()).toEqual(['check', 'end', 'user_created', 'yes'].sort())
    expect(path.items.has('no')).toBe(false)
  })
})

describe('history', () => {
  const snap0 = { graph: connected, layout: {} as AutomationLayout }
  it('records, undoes and redoes whole snapshots', () => {
    let h = createHistory(snap0)
    const g1 = connect(connected, { node: 'check', outcome: 'true' }, 'no')
    h = record(h, { graph: g1, layout: {} })
    expect(canUndo(h)).toBe(true)
    h = undo(h)
    expect(h.present.graph).toBe(connected)
    expect(canRedo(h)).toBe(true)
    h = redo(h)
    expect(h.present.graph).toBe(g1)
    h = record(undo(h), { graph: sample, layout: {} })
    expect(canRedo(h)).toBe(false)
  })
})

describe('clipboard', () => {
  it('round-trips through JSON text and renames taken ids', () => {
    const layout = ensureLayout(connected, catalog, {})
    const clip = copyItems(connected, layout, ['check', 'yes'])!
    const parsed = parseClip(serializeClip(clip))!
    expect(parsed.edges).toEqual([{ from: { node: 'check', outcome: 'true' }, to: 'yes' }])
    const pasted = pasteClip(connected, layout, parsed, { x: 1000, y: 1000 })
    expect(pasted.ids).toEqual(['check_2', 'yes_2'])
    expect(pasted.graph.edges).toContainEqual({ from: { node: 'check_2', outcome: 'true' }, to: 'yes_2' })
    expect(pasted.layout.check_2!.x % 16).toBe(0)
  })

  it('ignores text that is not a clip', () => {
    expect(parseClip('hello')).toBeNull()
    expect(parseClip('{"kind":"other"}')).toBeNull()
    expect(parseClip(JSON.stringify({ kind: 'iota.automation.clip/1', triggers: [], nodes: [{ id: 1 }], edges: [], layout: {} }))).toBeNull()
  })
})

const forwardCrossings = (graph: AutomationGraph, layout: AutomationLayout) => {
  const back = backEdges(graph)
  const segments = graph.edges
    .filter((e: GraphEdge) => e.to !== 'ignore' && !back.has(`${e.from.node}->${e.to}`))
    .map((e) => [layout[e.from.node]!, layout[e.to]!] as const)
  let count = 0
  for (let i = 0; i < segments.length; i += 1) {
    for (let j = i + 1; j < segments.length; j += 1) {
      const [a1, b1] = segments[i]!
      const [a2, b2] = segments[j]!
      if (a1.x === a2.x && b1.x === b2.x && (a1.y - a2.y) * (b1.y - b2.y) < 0) count += 1
    }
  }
  return count
}

describe('tidy layout (ED2)', () => {
  it('lays out left to right with triggers first, on the grid, without overlaps', () => {
    const layout = tidyLayout(sample, catalog)
    for (const p of Object.values(layout)) {
      expect(p.x % GRID).toBe(0)
      expect(p.y % GRID).toBe(0)
    }
    const triggerX = Math.max(...sample.triggers.map((t) => layout[t.id]!.x))
    const nodeX = Math.min(...sample.nodes.map((n) => layout[n.id]!.x))
    expect(triggerX).toBeLessThan(nodeX)
    expect(layout.send_email!.x).toBeGreaterThan(layout.has_email!.x)
    const ids = Object.keys(layout)
    for (let i = 0; i < ids.length; i += 1) {
      for (let j = i + 1; j < ids.length; j += 1) {
        const a = { ...layout[ids[i]!]!, ...itemSize(sample, catalog, ids[i]!) }
        const b = { ...layout[ids[j]!]!, ...itemSize(sample, catalog, ids[j]!) }
        expect(rectsOverlap(a, b)).toBe(false)
      }
    }
  })

  it('removes crossings of forward edges a naive order has', () => {
    const graph: AutomationGraph = {
      name: 'x', run_as: 'owner',
      triggers: [{ id: 't', event: 'core.user.created.v1' }],
      nodes: [
        { id: 'b', type: 'branch', params: {} },
        { id: 'p', type: 'delay', params: {} }, { id: 'q', type: 'delay', params: {} },
        { id: 'r', type: 'end', params: {} }, { id: 's', type: 'end', params: {} },
      ],
      edges: [
        { from: { node: 't', outcome: 'fired' }, to: 'b' },
        { from: { node: 'b', outcome: 'true' }, to: 'q' },
        { from: { node: 'b', outcome: 'false' }, to: 'p' },
        { from: { node: 'p', outcome: 'done' }, to: 's' },
        { from: { node: 'q', outcome: 'done' }, to: 'r' },
      ],
    }
    expect(forwardCrossings(graph, tidyLayout(graph, catalog))).toBe(0)
  })

  it('finds cycle edges so they run along the side', () => {
    const loop = connect(connected, { node: 'yes', outcome: 'done' }, 'check')
    expect(backEdges(loop).has('yes->check')).toBe(true)
    const layout = tidyLayout(loop, catalog)
    expect(layout.check!.x).toBeLessThan(layout.yes!.x)
  })

  it('handles 200 steps and 500 edges quickly', () => {
    const { graph } = bench()
    const started = performance.now()
    const layout = tidyLayout(graph, catalog)
    expect(Object.keys(layout)).toHaveLength(201)
    expect(performance.now() - started).toBeLessThan(2000)
  })

  it('places a new step next to its source in free space, and a new trigger below the others', () => {
    const layout = ensureLayout(sample, catalog, {})
    const { graph, id } = addNode(sample, spec('end'))
    const p = placeNew(graph, catalog, layout, id, { source: 'delay' })
    expect(p.x).toBeGreaterThan(layout.delay!.x)
    const size = itemSize(graph, catalog, id)
    for (const other of Object.keys(layout)) expect(rectsOverlap({ ...p, ...size }, { ...layout[other]!, ...itemSize(graph, catalog, other) })).toBe(false)
    const event = catalog.events[0]!
    const added = addTrigger(sample, event, event.key)
    const t = placeNew(added.graph, catalog, layout, added.id)
    expect(t.y).toBeGreaterThan(Math.max(layout.user_created!.y, layout.schedule!.y))
  })
})

describe('validation paths', () => {
  it('maps error paths to items and fields', () => {
    expect(errorTarget(sample, 'nodes[1].params.subject')).toEqual({ id: 'send_email', field: 'subject' })
    expect(errorTarget(sample, 'triggers[1].filter')).toEqual({ id: 'schedule', field: 'filter' })
    expect(errorTarget(sample, 'edges[0]')).toEqual({ id: 'user_created', field: null })
    expect(errorTarget(sample, 'name')).toEqual({ id: null, field: null })
    expect(errorsByItem(sample, [{ path: 'nodes[0].params.expression', message: 'bad' }]).get('has_email')).toBe('bad')
  })
})

describe('validation snapshots', () => {
  it('rebases error paths onto the current draft by id and drops errors of removed steps', () => {
    const before = sample
    const after = removeItems(sample, ['has_email'])
    const errors = [
      { path: 'nodes[0].params.expression', message: 'gone' },
      { path: 'nodes[1].params.subject', message: 'subject' },
      { path: 'name', message: 'graph-level' },
    ]
    const rebased = rebaseErrors(errors, before, after)
    expect(rebased).toEqual([{ path: 'nodes[0].params.subject', message: 'subject' }, { path: 'name', message: 'graph-level' }])
    expect(errorTarget(after, rebased[0]!.path).id).toBe('send_email')
  })

  it('reports steps of a type without a working connection, as the host diagnostic does (ED10)', () => {
    const graph: AutomationGraph = { ...sample, nodes: [...sample.nodes, { id: 'classify', type: 'ai.classify', params: {} }] }
    const errors = localErrors(graph, (type) => catalog.nodes.find((n) => n.type === type), {
      unknown: (type) => `unknown ${type}`,
      noConnection: (title) => `no connection for ${title}`,
    })
    expect(errors).toEqual([{ path: 'nodes[4].params.connection', message: 'no connection for Classify text', code: CONNECTION_UNAVAILABLE }])
    expect([...connectionLost(graph, errors)]).toEqual(['classify'])
  })
})
