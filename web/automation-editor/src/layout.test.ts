import { describe, expect, it } from 'vitest'
import { foreachJoin, layoutCatalog, multiTrigger, wide } from '../fixture/layoutGraphs'
import { sample } from '../fixture/graphs'
import { GRID, rectsOverlap } from './geometry'
import { createHistory, record, undo } from './history'
import { ensureLayout, FAILURE_OUTCOMES, itemRect, layoutGroups, isTidied, placeNew, SHARED_GROUP, tidyLayout, unmarked } from './layout'
import { edgeCrossings, layoutMetrics, mixedGroups, overlappingNodes } from './quality'
import { createRouter, linksOf, routeEdges, routePath, SNAP, type Point, type Route } from './route'
import { legacyTidyLayout } from './test/legacyTidy'
import type { AutomationGraph, AutomationLayout } from './types'

const catalog = layoutCatalog
const ZERO = { overlaps: 0, throughNodes: 0, mixedGroups: 0 }

/**
 * The graph as it was before a third of its steps were added without coordinates: the rest is where the
 * Tidy of that earlier graph left it.
 */
function withGaps(graph: AutomationGraph, every = 3): AutomationLayout {
  const all = [...graph.triggers.map((t) => t.id), ...graph.nodes.map((n) => n.id)]
  const known = new Set(all.filter((_, i) => i % every !== 1 && i !== 2))
  const earlier: AutomationGraph = {
    ...graph,
    triggers: graph.triggers.filter((t) => known.has(t.id)),
    nodes: graph.nodes.filter((n) => known.has(n.id)),
    edges: graph.edges.filter((e) => known.has(e.from.node) && known.has(e.to)),
  }
  return tidyLayout(earlier, catalog)
}

/** The layout an earlier Tidy left, before the groups for which `drop` is true were added. */
function earlierTidy(graph: AutomationGraph, drop: (key: string) => boolean): AutomationLayout {
  const keep = new Set(layoutGroups(graph).filter((g) => !drop(g.key)).flatMap((g) => g.items))
  return tidyLayout({
    ...graph,
    triggers: graph.triggers.filter((t) => keep.has(t.id)),
    nodes: graph.nodes.filter((n) => keep.has(n.id)),
    edges: graph.edges.filter((e) => keep.has(e.from.node) && keep.has(e.to)),
  }, catalog)
}

/** A card as a person leaves it after a drag: moved, snapped to whole numbers, no longer Tidy's. */
const byHand = (p: { x: number; y: number }, dx = 0, dy = 0) => ({ x: Math.round(p.x) + dx, y: Math.round(p.y) + dy })

/** Crossings of the straight lines from a port to the target handle: independent of any router. */
function straightCrossings(graph: AutomationGraph, layout: AutomationLayout, target: 'top' | 'left'): number {
  const { links } = linksOf(graph, catalog, layout)
  const lines = links.map((l) => ({
    ...l,
    b: target === 'top' ? l.entry : { x: layout[l.to]!.x, y: layout[l.to]!.y + 48 },
  }))
  const ccw = (a: { x: number; y: number }, b: { x: number; y: number }, c: { x: number; y: number }) => (c.y - a.y) * (b.x - a.x) - (b.y - a.y) * (c.x - a.x)
  let count = 0
  for (let i = 0; i < lines.length; i += 1) {
    for (let j = i + 1; j < lines.length; j += 1) {
      const p = lines[i]!
      const q = lines[j]!
      if (p.from === q.from || p.to === q.to) continue
      if (ccw(p.port, q.port, q.b) * ccw(p.b, q.port, q.b) < 0 && ccw(p.port, p.b, q.port) * ccw(p.port, p.b, q.b) < 0) count += 1
    }
  }
  return count
}

describe.each([
  ['four sources, ~30 steps', multiTrigger],
  ['for each and join', foreachJoin],
])('Tidy v2 on %s', (_name, graph) => {
  const layout = tidyLayout(graph, catalog)

  it('puts every item on the grid, without overlaps, link through a card or mixed groups', () => {
    expect(Object.keys(layout)).toHaveLength(graph.triggers.length + graph.nodes.length)
    for (const p of Object.values(layout)) {
      expect(Math.round(p.x) % GRID).toBe(0)
      expect(Math.round(p.y) % GRID).toBe(0)
      expect(isTidied(p)).toBe(true)
    }
    const metrics = layoutMetrics(graph, catalog, layout)
    expect({ overlaps: metrics.overlaps, throughNodes: metrics.throughNodes, mixedGroups: metrics.mixedGroups }).toEqual(ZERO)
  })

  it('crosses no more links than the algorithm it replaces', () => {
    const before = legacyTidyLayout(graph, catalog)
    expect(layoutMetrics(graph, catalog, layout).crossings).toBeLessThanOrEqual(layoutMetrics(graph, catalog, before).crossings)
    expect(straightCrossings(graph, layout, 'top')).toBeLessThanOrEqual(straightCrossings(graph, before, 'left'))
  })

  it('lays the same graph out the same way twice', () => {
    expect(tidyLayout(graph, catalog)).toEqual(layout)
  })

  it('is as good when a third of the steps have no coordinates and the graph is laid out again', () => {
    const partial = withGaps(graph)
    const all = graph.triggers.length + graph.nodes.length
    expect((all - Object.keys(partial).length) * 3).toBeGreaterThan(all) // more than a third
    const next = ensureLayout(graph, catalog, partial)
    expect(next).toEqual(layout)
    expect(layoutMetrics(graph, catalog, next)).toMatchObject(ZERO)
  })
})

describe('trigger columns', () => {
  const layout = tidyLayout(multiTrigger, catalog)
  const groups = layoutGroups(multiTrigger)

  it('gives every trigger a column of its own, side by side, with the shared steps below, centred', () => {
    const keys = groups.map((g) => g.key)
    expect(keys).toEqual(['user_created', 'client_created', 'schedule', 'manual', SHARED_GROUP])
    const area = (items: string[]) => {
      const rects = items.map((id) => itemRect(multiTrigger, catalog, layout, id)!)
      return { left: Math.min(...rects.map((r) => r.x)), right: Math.max(...rects.map((r) => r.x + r.width)), top: Math.min(...rects.map((r) => r.y)), bottom: Math.max(...rects.map((r) => r.y + r.height)) }
    }
    const columns = groups.filter((g) => g.key !== SHARED_GROUP).map((g) => area(g.items))
    for (let i = 1; i < columns.length; i += 1) {
      // The gap between two columns is twice the step between two cards of a row.
      expect(columns[i]!.left - columns[i - 1]!.right).toBeGreaterThanOrEqual(128 - GRID)
    }
    const shared = area(groups.find((g) => g.key === SHARED_GROUP)!.items)
    expect(shared.top).toBeGreaterThan(Math.max(...columns.map((c) => c.bottom)))
    // Centred under the columns that feed it.
    const centre = (shared.left + shared.right) / 2
    expect(centre).toBeGreaterThan(columns[0]!.left)
    expect(centre).toBeLessThan(columns[columns.length - 1]!.right)
  })

  it('puts the steps entered only through failed, timeout or low_confidence in a lane right of their chain', () => {
    const incoming = new Map<string, { from: string; outcome: string }[]>()
    for (const e of multiTrigger.edges) incoming.set(e.to, [...(incoming.get(e.to) ?? []), { from: e.from.node, outcome: e.from.outcome }])
    const isFailure = (id: string): boolean => {
      const ins = incoming.get(id) ?? []
      return ins.length > 0 && ins.every((l) => FAILURE_OUTCOMES.has(l.outcome) || isFailure(l.from))
    }
    const failure = new Set(multiTrigger.nodes.map((n) => n.id).filter(isFailure))
    expect([...failure].sort()).toEqual(['a_alert', 'a_alert_end', 'b_fail', 'c_end', 'c_fail', 'c_review', 'd_end', 'd_timeout', 's_alert'])
    for (const group of groups) {
      const main = group.items.filter((id) => !failure.has(id))
      const lane = group.items.filter((id) => failure.has(id))
      if (lane.length === 0) continue
      const mainRight = Math.max(...main.map((id) => itemRect(multiTrigger, catalog, layout, id)!).map((r) => r.x + r.width))
      for (const id of lane) expect(layout[id]!.x).toBeGreaterThanOrEqual(mainRight)
    }
  })

  it('stands a child under the port of its outcome, not under the middle of a wide step', () => {
    const wideParent = itemRect(multiTrigger, catalog, layout, 'b_route')!
    const vip = itemRect(multiTrigger, catalog, layout, 'b_vip')!
    const trial = itemRect(multiTrigger, catalog, layout, 'b_pause')!
    // Cases are ordered vip, regular, trial: the children follow the ports from left to right.
    expect(vip.x + vip.width / 2).toBeLessThan(trial.x + trial.width / 2)
    expect(vip.y).toBeGreaterThan(wideParent.y + wideParent.height)
    const { links } = linksOf(multiTrigger, catalog, layout)
    const vipLink = links.find((l) => l.key === 'b_route.vip')!
    // The nearer the child is to its port the shorter the way across.
    expect(Math.abs(vipLink.entry.x - vipLink.port.x)).toBeLessThan(wideParent.width)
  })
})

describe('routes', () => {
  const graph = multiTrigger
  const layout = tidyLayout(graph, catalog)
  const routes = routeEdges(graph, catalog, layout)

  it('are orthogonal, leave from the bottom of a card and enter at the top of another', () => {
    const { links } = linksOf(graph, catalog, layout)
    expect(routes.size).toBe(links.length)
    for (const link of links) {
      const route = routes.get(link.key)!
      expect(route.blocked).toBe(false)
      const points = route.points
      expect(points[0]).toEqual(link.port)
      // Ends less than SNAP apart across are one straight column: the route ends right above the handle.
      expect(points[points.length - 1]!.y).toBe(link.entry.y)
      expect(Math.abs(points[points.length - 1]!.x - link.entry.x)).toBeLessThan(SNAP)
      expect(points[1]!.x).toBe(link.port.x)
      expect(points[1]!.y).toBeGreaterThan(link.port.y)
      for (let i = 1; i < points.length; i += 1) expect(points[i]!.x === points[i - 1]!.x || points[i]!.y === points[i - 1]!.y).toBe(true)
    }
  })

  /** Length that two routes run along the same line. */
  const sharedLength = (a: Route, b: Route) => {
    let sum = 0
    for (let i = 1; i < a.points.length; i += 1) {
      for (let j = 1; j < b.points.length; j += 1) {
        const [p1, p2, q1, q2] = [a.points[i - 1]!, a.points[i]!, b.points[j - 1]!, b.points[j]!]
        if (p1.y === p2.y && q1.y === q2.y && p1.y === q1.y) sum += Math.max(0, Math.min(Math.max(p1.x, p2.x), Math.max(q1.x, q2.x)) - Math.max(Math.min(p1.x, p2.x), Math.min(q1.x, q2.x)))
        if (p1.x === p2.x && q1.x === q2.x && p1.x === q1.x) sum += Math.max(0, Math.min(Math.max(p1.y, p2.y), Math.max(q1.y, q2.y)) - Math.max(Math.min(p1.y, p2.y), Math.min(q1.y, q2.y)))
      }
    }
    return sum
  }

  it('share a trunk: two edges of one source run along the same line for a stretch', () => {
    const { links } = linksOf(graph, catalog, layout)
    const out = links.filter((l) => l.from === 's_crm')
    expect(out).toHaveLength(2)
    expect(sharedLength(routes.get(out[0]!.key)!, routes.get(out[1]!.key)!)).toBeGreaterThan(0)
  })

  it('share a trunk between a source and two targets far below', () => {
    const rects = new Map([
      ['s', { x: 0, y: 0, width: 288, height: 96 }],
      ['a', { x: 300, y: 800, width: 224, height: 96 }],
      ['b', { x: 700, y: 800, width: 224, height: 96 }],
    ])
    const router = createRouter(rects, [
      { key: 's.one', from: 's', to: 'a', port: { x: 100, y: 96 }, entry: { x: 412, y: 800 } },
      { key: 's.two', from: 's', to: 'b', port: { x: 188, y: 96 }, entry: { x: 812, y: 800 } },
    ])
    expect(sharedLength(router.route('s.one')!, router.route('s.two')!)).toBeGreaterThan(0)
  })

  it('count crossings of unrelated edges only: those that share a source or a target meet by design', () => {
    const line = (...points: [number, number][]): Route => ({ points: points.map(([x, y]) => ({ x, y })), blocked: false })
    const routes = new Map<string, Route>([
      ['a.x', line([0, 50], [100, 50])],
      ['c.x', line([50, 0], [50, 100])], // crosses a.x, unrelated
      ['a.y', line([70, 0], [70, 100])], // crosses a.x, same source
      ['d.x', line([30, 0], [30, 100])], // crosses a.x, same target
    ])
    const ends = new Map([
      ['a.x', { from: 'a', to: 'b' }],
      ['c.x', { from: 'c', to: 'e' }],
      ['a.y', { from: 'a', to: 'f' }],
      ['d.x', { from: 'd', to: 'b' }],
    ])
    expect(edgeCrossings(routes, ends)).toBe(1)
    expect(edgeCrossings(new Map([['a.x', routes.get('a.x')!], ['a.y', routes.get('a.y')!], ['d.x', routes.get('d.x')!]]), ends)).toBe(0)
  })
})

describe('arranged by hand', () => {
  it('keeps the positions of a group the person arranged when a new branch is laid out', () => {
    const known = earlierTidy(multiTrigger, (key) => key === 'manual' || key === SHARED_GROUP)
    const hand = layoutGroups(multiTrigger).find((g) => g.key === 'client_created')!.items
    // The person moved two steps of the client column by hand.
    known.b_vip = byHand(known.b_vip!, 48, 32)
    known.b_mail = byHand(known.b_mail!, -16, 64)
    const before = Object.fromEntries(hand.map((id) => [id, { ...known[id]! }]))
    // The whole manual column and the shared steps have no coordinates: a branch and more than a third.
    const next = ensureLayout(multiTrigger, catalog, known)
    for (const id of hand) expect(next[id]).toEqual(before[id])
    // The other groups, which were as Tidy left them, are laid out again and clear of the kept ones.
    expect(Object.keys(next)).toHaveLength(multiTrigger.triggers.length + multiTrigger.nodes.length)
    expect(overlappingNodes(multiTrigger, catalog, next)).toBe(0)
    expect(mixedGroups(multiTrigger, catalog, next)).toBe(0)
  })

  it('does not touch positions when nothing is missing', () => {
    const layout = tidyLayout(multiTrigger, catalog)
    const moved = { ...layout, b_vip: byHand(layout.b_vip!, 16, 0) }
    expect(ensureLayout(multiTrigger, catalog, moved)).toEqual(moved)
  })

  it('places one new step under its source without moving anything', () => {
    const layout = tidyLayout(sample, catalog)
    const graph: AutomationGraph = {
      ...sample,
      nodes: [...sample.nodes, { id: 'extra', type: 'end', params: {} }],
      edges: [...sample.edges, { from: { node: 'delay', outcome: 'done' }, to: 'extra' }],
    }
    const next = ensureLayout(graph, catalog, layout)
    for (const [id, p] of Object.entries(layout)) expect(next[id]).toEqual(p)
    const delay = itemRect(graph, catalog, layout, 'delay')!
    expect(next.extra!.y).toBeGreaterThan(delay.y + delay.height)
  })
})

describe('placing a new card', () => {
  it('goes under the port of its outcome, in free space, and a new trigger goes right of the others', () => {
    const layout = tidyLayout(sample, catalog)
    const graph: AutomationGraph = {
      ...sample,
      nodes: [...sample.nodes, { id: 'extra', type: 'end', params: {} }],
      edges: [...sample.edges, { from: { node: 'send_email', outcome: 'failed' }, to: 'extra' }].filter((e) => !(e.from.node === 'send_email' && e.from.outcome === 'failed' && e.to === 'ignore')),
    }
    const p = placeNew(graph, catalog, layout, 'extra', { source: 'send_email', outcome: 'failed' })
    const source = itemRect(graph, catalog, layout, 'send_email')!
    expect(p.y).toBeGreaterThan(source.y + source.height)
    for (const id of Object.keys(layout)) {
      const other = itemRect(graph, catalog, layout, id)!
      expect(rectsOverlap({ ...p, width: 224, height: 96 }, other)).toBe(false)
    }
    const added: AutomationGraph = { ...sample, triggers: [...sample.triggers, { id: 'manual', event: 'core.manual' }] }
    const t = placeNew(added, catalog, layout, 'manual')
    expect(t.x).toBeGreaterThan(Math.max(...Object.keys(layout).map((id) => itemRect(sample, catalog, layout, id)!.x)))
  })
})

describe('Tidy is one undo step', () => {
  it('replaces the whole layout in a single history record', () => {
    const initial = { graph: multiTrigger, layout: legacyTidyLayout(multiTrigger, catalog) }
    let history = createHistory(initial)
    history = record(history, { graph: multiTrigger, layout: tidyLayout(multiTrigger, catalog) })
    expect(history.past).toHaveLength(1)
    expect(undo(history).present.layout).toBe(initial.layout)
  })
})

describe('Tidy time', () => {
  it('takes under 100 ms on 240 steps', () => {
    const graph = wide(240)
    tidyLayout(graph, catalog)
    const started = performance.now()
    const layout = tidyLayout(graph, catalog)
    const took = performance.now() - started
    expect(Object.keys(layout)).toHaveLength(241)
    expect(overlappingNodes(graph, catalog, layout)).toBe(0)
    expect(took).toBeLessThan(100)
  })
})

describe('review round', () => {
  it('counts an item reached through a cycle edge for the trigger that reaches it', () => {
    const graph: AutomationGraph = {
      name: 'cycle', run_as: 'owner',
      triggers: [{ id: 'a', event: 'core.user.created.v1' }, { id: 'b', event: 'crm.client.created.v1' }],
      nodes: [{ id: 'x', type: 'delay', params: {} }, { id: 'y', type: 'delay', params: {} }],
      edges: [
        { from: { node: 'a', outcome: 'fired' }, to: 'x' },
        { from: { node: 'x', outcome: 'done' }, to: 'y' },
        { from: { node: 'b', outcome: 'fired' }, to: 'y' },
        { from: { node: 'y', outcome: 'done' }, to: 'x' },
      ],
    }
    const keys = Object.fromEntries(layoutGroups(graph).map((g) => [g.key, g.items]))
    expect(keys[SHARED_GROUP]).toEqual(['x', 'y'])
  })

  it('keeps a group a person moved as a whole when a new branch is laid out', () => {
    const tidy = earlierTidy(multiTrigger, (key) => key === 'manual' || key === SHARED_GROUP)
    const known: AutomationLayout = { ...tidy }
    const moved = layoutGroups(multiTrigger).find((g) => g.key === 'client_created')!.items
    for (const id of moved) known[id] = byHand(tidy[id]!, 160, 96)
    const next = ensureLayout(multiTrigger, catalog, known)
    for (const id of moved) expect(next[id]).toEqual(known[id])
    expect(overlappingNodes(multiTrigger, catalog, next)).toBe(0)
  })

  it('returns a free position even when every nearby one is taken', () => {
    const graph: AutomationGraph = { name: 'p', run_as: 'owner', triggers: [], nodes: [{ id: 'n', type: 'end', params: {} }], edges: [] }
    const layout: AutomationLayout = {}
    const crowded: AutomationGraph = { ...graph, nodes: [...graph.nodes] }
    for (let i = 0; i < 400; i += 1) {
      crowded.nodes.push({ id: `c${i}`, type: 'end', params: {} })
      layout[`c${i}`] = { x: (i % 20) * 240, y: Math.floor(i / 20) * 144 }
    }
    const p = placeNew(crowded, catalog, layout, 'n', { near: { x: 0, y: 0 } })
    for (const id of Object.keys(layout)) expect(rectsOverlap({ ...p, width: 224, height: 96 }, itemRect(crowded, catalog, layout, id)!, GRID)).toBe(false)
  })

  it('links two close, aligned cards with a straight line', () => {
    const rects = new Map([['a', { x: 0, y: 0, width: 224, height: 96 }], ['b', { x: 0, y: 104, width: 224, height: 96 }]])
    const router = createRouter(rects, [{ key: 'a.done', from: 'a', to: 'b', port: { x: 112, y: 96 }, entry: { x: 112, y: 104 } }])
    expect(router.route('a.done')).toEqual({ points: [{ x: 112, y: 96 }, { x: 112, y: 104 }], blocked: false })
  })

  it('never takes a channel through a card that was not near its endpoints', () => {
    // A wide row of touching cards between the two ends: the clear channels are beyond it.
    const rects = new Map<string, { x: number; y: number; width: number; height: number }>()
    rects.set('s', { x: 0, y: 0, width: 224, height: 96 })
    rects.set('t', { x: 0, y: 1200, width: 224, height: 96 })
    for (let i = 0; i < 14; i += 1) rects.set(`w${i}`, { x: -1500 + i * 224, y: 600, width: 224, height: 96 })
    const router = createRouter(rects, [{ key: 's.done', from: 's', to: 't', port: { x: 112, y: 96 }, entry: { x: 112, y: 1200 } }])
    const route = router.route('s.done')!
    expect(route.blocked).toBe(false)
    for (let i = 1; i < route.points.length; i += 1) {
      const a = route.points[i - 1]!
      const b = route.points[i]!
      for (const [id, r] of rects) {
        if (id === 's' || id === 't') continue
        expect(Math.max(a.x, b.x) > r.x && Math.min(a.x, b.x) < r.x + r.width && Math.max(a.y, b.y) > r.y && Math.min(a.y, b.y) < r.y + r.height).toBe(false)
      }
    }
  })

  it('keeps usable source lanes when the card below the source row is off the source column', () => {
    // The obstacle touches the source row (no lane room under it) but is not over the source column:
    // the edge goes straight down the source column to the free gap above the target.
    const rects = new Map([
      ['s', { x: 0, y: 0, width: 224, height: 96 }],
      ['t', { x: 400, y: 250, width: 224, height: 96 }],
      ['o', { x: 260, y: 96, width: 100, height: 96 }],
    ])
    const route = createRouter(rects, [{ key: 's.done', from: 's', to: 't', port: { x: 112, y: 96 }, entry: { x: 512, y: 250 } }]).route('s.done')!
    expect(route.blocked).toBe(false)
    const o = rects.get('o')!
    for (let i = 1; i < route.points.length; i += 1) {
      const a = route.points[i - 1]!
      const b = route.points[i]!
      expect(Math.max(a.x, b.x) > o.x && Math.min(a.x, b.x) < o.x + o.width && Math.max(a.y, b.y) > o.y && Math.min(a.y, b.y) < o.y + o.height).toBe(false)
    }
  })

  it('plans each link once, earlier ports of a source first', () => {
    const rects = new Map([['a', { x: 0, y: 0, width: 224, height: 96 }], ['b', { x: 0, y: 300, width: 224, height: 96 }], ['c', { x: 300, y: 300, width: 224, height: 96 }]])
    const router = createRouter(rects, [
      { key: 'a.two', from: 'a', to: 'c', port: { x: 140, y: 96 }, entry: { x: 412, y: 300 } },
      { key: 'a.one', from: 'a', to: 'b', port: { x: 80, y: 96 }, entry: { x: 112, y: 300 } },
    ])
    expect(router.plan(['a.two'])).toEqual(['a.one', 'a.two'])
    router.route('a.one')
    expect(router.plan(['a.two'])).toEqual(['a.two'])
  })
})

describe('arranged by hand, from positions alone', () => {
  const newBranch = (): AutomationGraph => ({
    ...foreachJoin,
    triggers: [...foreachJoin.triggers, { id: 'again', event: 'core.manual' }],
    nodes: [...foreachJoin.nodes, { id: 'x1', type: 'delay', params: {} }, { id: 'x2', type: 'delay', params: {} }, { id: 'x3', type: 'end', params: {} }],
    edges: [
      ...foreachJoin.edges,
      { from: { node: 'again', outcome: 'fired' }, to: 'x1' },
      { from: { node: 'x1', outcome: 'done' }, to: 'x2' },
      { from: { node: 'x2', outcome: 'done' }, to: 'x3' },
    ],
  })
  const onlyOld = (key: string) => key === 'again'

  it('keeps the only positioned group when the person moved it, and lays the new branch beside it', () => {
    const graph = newBranch()
    const tidy = earlierTidy(graph, onlyOld)
    const moved: AutomationLayout = {}
    for (const [id, p] of Object.entries(tidy)) moved[id] = byHand(p, 32, 16)
    const next = ensureLayout(graph, catalog, moved)
    for (const [id, p] of Object.entries(moved)) expect(next[id]).toEqual(p)
    expect(Object.keys(next)).toHaveLength(graph.triggers.length + graph.nodes.length)
    expect(overlappingNodes(graph, catalog, next)).toBe(0)
  })

  it('keeps a lone trigger the person moved when a branch under it has no positions, also after storage', () => {
    const graph = newBranch()
    const known: AutomationLayout = { ...earlierTidy(graph, onlyOld), again: { x: 1600, y: 32 } }
    const stored = JSON.parse(JSON.stringify(known)) as AutomationLayout
    const next = ensureLayout(graph, catalog, stored)
    expect(next.again).toEqual({ x: 1600, y: 32 })
    expect(next.x1!.y).toBeGreaterThan(32)
    expect(Object.keys(next)).toHaveLength(graph.triggers.length + graph.nodes.length)
    expect(overlappingNodes(graph, catalog, next)).toBe(0)
  })

  it('keeps every group that was moved, whatever the offsets: tied or not', () => {
    const known = earlierTidy(multiTrigger, (key) => key === 'manual' || key === SHARED_GROUP)
    const groups = layoutGroups(multiTrigger)
    const moved: AutomationLayout = { ...known }
    const shifts: Record<string, [number, number]> = { user_created: [48, 0], client_created: [0, 64], schedule: [48, 0] }
    for (const group of groups) {
      const [dx, dy] = shifts[group.key] ?? [0, 0]
      for (const id of group.items) if (moved[id]) moved[id] = byHand(known[id]!, dx, dy)
    }
    const next = ensureLayout(multiTrigger, catalog, moved)
    for (const [id, p] of Object.entries(moved)) expect(next[id]).toEqual(p)
  })

  it('lays a group Tidy left alone out again, also after the layout went through storage', () => {
    const graph = newBranch()
    const stored = JSON.parse(JSON.stringify(earlierTidy(graph, onlyOld))) as AutomationLayout
    expect(ensureLayout(graph, catalog, stored)).toEqual(tidyLayout(graph, catalog))
  })

  it('moves everything on an explicit Tidy, positions by hand or not', () => {
    const graph = newBranch()
    const moved: AutomationLayout = {}
    for (const [id, p] of Object.entries(earlierTidy(graph, onlyOld))) moved[id] = byHand(p, 48, 48)
    // The automatic layout keeps what the person moved; the explicit Tidy puts it back.
    expect(ensureLayout(graph, catalog, moved).start).toEqual(moved.start)
    expect(tidyLayout(graph, catalog).start).toEqual(earlierTidy(graph, onlyOld).start)
    expect(tidyLayout(graph, catalog).start).not.toEqual(moved.start)
  })

  it('returns a position below every card when the whole search window is taken', () => {
    const graph: AutomationGraph = { name: 'p', run_as: 'owner', triggers: [], nodes: [{ id: 'n', type: 'end', params: {} }], edges: [] }
    const layout: AutomationLayout = {}
    const crowded: AutomationGraph = { ...graph, nodes: [...graph.nodes] }
    // Every candidate the search tries: 13 columns either side of the start, 100 rows.
    let count = 0
    for (let k = -13; k <= 13; k += 1) {
      for (let r = 0; r <= 100; r += 1) {
        const id = `c${count}`
        count += 1
        crowded.nodes.push({ id, type: 'end', params: {} })
        layout[id] = { x: k * 288, y: r * 144 }
      }
    }
    const p = placeNew(crowded, catalog, layout, 'n', { near: { x: 0, y: 0 } })
    const bottom = Math.max(...Object.keys(layout).map((id) => layout[id]!.y + 96))
    expect(p.y).toBeGreaterThanOrEqual(bottom)
    for (const id of Object.keys(layout)) expect(rectsOverlap({ ...p, width: 224, height: 96 }, itemRect(crowded, catalog, layout, id)!, GRID)).toBe(false)
  })
})

describe('a trigger called "shared"', () => {
  it('keeps its own column apart from the shared group', () => {
    const graph: AutomationGraph = {
      name: 's', run_as: 'owner',
      triggers: [{ id: 'shared', event: 'core.user.created.v1' }],
      nodes: [{ id: 'a', type: 'delay', params: {} }],
      edges: [{ from: { node: 'shared', outcome: 'fired' }, to: 'a' }],
    }
    const groups = layoutGroups(graph)
    expect(groups).toEqual([{ key: 'shared', items: ['shared', 'a'] }])
    expect(layoutMetrics(graph, catalog, tidyLayout(graph, catalog))).toMatchObject({ overlaps: 0, throughNodes: 0 })
  })
})

describe('code review: relayout and routes', () => {
  const without = (layout: AutomationLayout, ids: string[]) => Object.fromEntries(Object.entries(layout).filter(([id]) => !ids.includes(id))) as AutomationLayout
  const sub = (ids: string[]): AutomationGraph => ({
    ...multiTrigger,
    triggers: multiTrigger.triggers.filter((t) => !ids.includes(t.id)),
    nodes: multiTrigger.nodes.filter((n) => !ids.includes(n.id)),
    edges: multiTrigger.edges.filter((e) => !ids.includes(e.from.node) && !ids.includes(e.to)),
  })

  it('lays the graph out again when places of items that were in the last Tidy are lost', () => {
    const full = tidyLayout(multiTrigger, catalog)
    expect(ensureLayout(multiTrigger, catalog, without(full, ['a_mail', 'a_wait', 'a_tag']))).toEqual(full)
  })

  it('treats two linked steps without positions as a branch: the failure lane comes back', () => {
    const lost = ['a_alert', 'a_alert_end']
    const stored = tidyLayout(sub(lost), catalog)
    expect(ensureLayout(multiTrigger, catalog, stored)).toEqual(tidyLayout(multiTrigger, catalog))
  })

  it('still places one new step by itself', () => {
    const stored = tidyLayout(sub(['a_alert_end']), catalog)
    const next = ensureLayout(multiTrigger, catalog, stored)
    for (const [id, p] of Object.entries(stored)) expect(next[id]).toEqual(p)
  })

  it('finds a way round a card that leaves no room for a full stub', () => {
    const rects = new Map([
      ['a', { x: 0, y: 0, width: 224, height: 96 }],
      ['b', { x: 0, y: 104, width: 224, height: 96 }],
      ['c', { x: 0, y: 304, width: 224, height: 96 }],
    ])
    const route = createRouter(rects, [{ key: 'a.done', from: 'a', to: 'c', port: { x: 112, y: 96 }, entry: { x: 112, y: 304 } }]).route('a.done')!
    expect(route.blocked).toBe(false)
    for (let i = 1; i < route.points.length; i += 1) {
      const p = route.points[i - 1]!
      const q = route.points[i]!
      expect(Math.max(p.x, q.x) > 0 && Math.min(p.x, q.x) < 224 && Math.max(p.y, q.y) > 104 && Math.min(p.y, q.y) < 200).toBe(false)
    }
  })
})

describe('code review round 2', () => {
  const sub = (ids: string[]): AutomationGraph => ({
    ...multiTrigger,
    triggers: multiTrigger.triggers.filter((t) => !ids.includes(t.id)),
    nodes: multiTrigger.nodes.filter((n) => !ids.includes(n.id)),
    edges: multiTrigger.edges.filter((e) => !ids.includes(e.from.node) && !ids.includes(e.to)),
  })
  const lost = ['a_mail', 'a_wait', 'a_tag']

  it('keeps cards a person set at the positions a Tidy of the shorter graph gives, also after storage', () => {
    const shorter = unmarked(tidyLayout(sub(lost), catalog))
    const column = layoutGroups(multiTrigger).find((g) => g.key === 'user_created')!.items.filter((id) => !lost.includes(id))
    const stored = JSON.parse(JSON.stringify(Object.fromEntries(Object.entries(tidyLayout(sub(lost), catalog)).map(([id, p]) => [id, column.includes(id) ? shorter[id]! : p])))) as AutomationLayout
    const next = ensureLayout(multiTrigger, catalog, stored)
    for (const id of column) expect(next[id]).toEqual(shorter[id])
  })

  it('marks what Tidy leaves, and a card moved by hand loses the mark', () => {
    const layout = tidyLayout(multiTrigger, catalog)
    expect(Object.values(layout).every(isTidied)).toBe(true)
    expect(Object.values(JSON.parse(JSON.stringify(layout)) as AutomationLayout).every(isTidied)).toBe(true)
    expect(isTidied(byHand(layout.a_check!, 16, 0))).toBe(false)
    expect(Object.values(unmarked(layout)).some(isTidied)).toBe(false)
  })

  it('keeps a layout from before the mark: nothing in it is known to be Tidy\'s', () => {
    const graph = { ...multiTrigger, nodes: multiTrigger.nodes.filter((n) => !['a_alert', 'a_alert_end'].includes(n.id)) }
    const old = unmarked(tidyLayout({ ...graph, edges: graph.edges.filter((e) => !['a_alert', 'a_alert_end'].includes(e.to)) }, catalog))
    const next = ensureLayout(multiTrigger, catalog, old)
    for (const [id, p] of Object.entries(old)) expect(next[id]).toEqual(p)
  })

  it('finds a way round a card with a 7 px gap under the source', () => {
    for (const gap of [7, 8, 3]) {
      const rects = new Map([
        ['a', { x: 0, y: 0, width: 224, height: 96 }],
        ['b', { x: 0, y: 96 + gap, width: 224, height: 96 }],
        ['c', { x: 0, y: 304, width: 224, height: 96 }],
      ])
      const route = createRouter(rects, [{ key: 'a.done', from: 'a', to: 'c', port: { x: 112, y: 96 }, entry: { x: 112, y: 304 } }]).route('a.done')!
      expect(route.blocked).toBe(false)
      for (let i = 1; i < route.points.length; i += 1) {
        const p = route.points[i - 1]!
        const q = route.points[i]!
        expect(Math.max(p.x, q.x) > 0 && Math.min(p.x, q.x) < 224 && Math.max(p.y, q.y) > 96 + gap && Math.min(p.y, q.y) < 192 + gap).toBe(false)
      }
    }
  })
})

describe('code review round 3', () => {
  const node = (id: string, type = 'delay') => ({ id, type, params: type === 'branch' ? { expression: 'true' } : {} })
  const cycle = (viaFailure: boolean): AutomationGraph => ({
    name: 'c', run_as: 'owner',
    triggers: [{ id: 't', event: 'core.user.created.v1' }],
    nodes: [node('p', 'core.send_email'), node('x'), node('y')],
    edges: [
      { from: { node: 't', outcome: 'fired' }, to: 'p' },
      { from: { node: 'p', outcome: 'failed' }, to: 'y' },
      ...(viaFailure ? [] : [{ from: { node: 'p', outcome: 'sent' }, to: 'x' }]),
      { from: { node: 'y', outcome: 'done' }, to: 'x' },
      { from: { node: 'x', outcome: 'done' }, to: 'y' },
    ],
  })
  const lane = (graph: AutomationGraph) => {
    const layout = tidyLayout(graph, catalog)
    const mainRight = Math.max(...['t', 'p'].map((id) => layout[id]!.x + 224))
    return (id: string) => layout[id]!.x >= mainRight
  }

  it('does not put a step in the failure lane when a cycle edge reaches it without a failure', () => {
    const graph = cycle(false)
    const inLane = lane(graph)
    expect(inLane('y')).toBe(false)
    expect(inLane('x')).toBe(false)
  })

  it('still puts steps reached only after a failure in the lane, cycle or not', () => {
    const graph = cycle(true)
    const inLane = lane(graph)
    expect(inLane('y')).toBe(true)
    expect(inLane('x')).toBe(true)
  })

  it('uses the middle of a one pixel gap', () => {
    const rects = new Map([
      ['a', { x: 0, y: 0, width: 224, height: 96 }],
      ['b', { x: 0, y: 97, width: 224, height: 96 }],
      ['c', { x: 320, y: 304, width: 224, height: 96 }],
    ])
    const route = createRouter(rects, [{ key: 'a.done', from: 'a', to: 'c', port: { x: 112, y: 96 }, entry: { x: 432, y: 304 } }]).route('a.done')!
    expect(route.blocked).toBe(false)
    for (let i = 1; i < route.points.length; i += 1) {
      const p = route.points[i - 1]!
      const q = route.points[i]!
      expect(Math.max(p.x, q.x) > 0 && Math.min(p.x, q.x) < 224 && Math.max(p.y, q.y) > 97 && Math.min(p.y, q.y) < 193).toBe(false)
    }
  })

  it('keeps the rounded corners of a route that runs close to cards outside them', () => {
    const rects = new Map([
      ['a', { x: 0, y: 0, width: 224, height: 96 }],
      ['t', { x: 400, y: 304, width: 224, height: 96 }],
      ['b1', { x: 0, y: 99, width: 224, height: 96 }],
      ['b2', { x: 286, y: 99, width: 224, height: 96 }],
    ])
    const route = createRouter(rects, [{ key: 'a.done', from: 'a', to: 't', port: { x: 112, y: 96 }, entry: { x: 512, y: 304 } }]).route('a.done')!
    expect(route.blocked).toBe(false)
    // Sample the drawn path: lines and quadratic curves.
    const d = routePath(route.points, 10, route.clear)
    const tokens = d.match(/[MLQ]|-?[\d.]+/g)!
    const points: Point[] = []
    let at: Point = { x: 0, y: 0 }
    for (let i = 0; i < tokens.length;) {
      const cmd = tokens[i++]!
      const n = cmd === 'Q' ? 4 : 2
      const nums = tokens.slice(i, i + n).map(Number)
      i += n
      if (cmd === 'Q') {
        const [cx, cy, ex, ey] = nums as [number, number, number, number]
        for (let k = 1; k <= 20; k += 1) {
          const u = k / 20
          points.push({ x: (1 - u) * (1 - u) * at.x + 2 * (1 - u) * u * cx + u * u * ex, y: (1 - u) * (1 - u) * at.y + 2 * (1 - u) * u * cy + u * u * ey })
        }
        at = { x: ex, y: ey }
      } else {
        const [x, y] = nums as [number, number]
        points.push({ x, y })
        at = { x, y }
      }
    }
    for (const p of points) {
      for (const id of ['b1', 'b2']) {
        const r = rects.get(id)!
        expect(p.x > r.x && p.x < r.x + r.width && p.y > r.y && p.y < r.y + r.height).toBe(false)
      }
    }
  })
})
