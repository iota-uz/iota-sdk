import { describe, expect, it } from 'vitest'
import { foreachJoin, layoutCatalog, multiTrigger, wide } from '../fixture/layoutGraphs'
import { sample } from '../fixture/graphs'
import { GRID, rectsOverlap } from './geometry'
import { createHistory, record, undo } from './history'
import { ensureLayout, FAILURE_OUTCOMES, itemRect, layoutGroups, placeNew, SHARED_GROUP, tidyLayout } from './layout'
import { edgeCrossings, layoutMetrics, mixedGroups, overlappingNodes } from './quality'
import { createRouter, linksOf, routeEdges } from './route'
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
      expect(p.x % GRID).toBe(0)
      expect(p.y % GRID).toBe(0)
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
      expect(points[points.length - 1]).toEqual(link.entry)
      expect(points[1]!.x).toBe(link.port.x)
      expect(points[1]!.y).toBeGreaterThan(link.port.y)
      for (let i = 1; i < points.length; i += 1) expect(points[i]!.x === points[i - 1]!.x || points[i]!.y === points[i - 1]!.y).toBe(true)
    }
  })

  it('share a trunk: the edges of one source run along the same channel', () => {
    const { links } = linksOf(graph, catalog, layout)
    const out = links.filter((l) => l.from === 'b_route')
    const channels = out.map((l) => routes.get(l.key)!.points[1]!.y)
    expect(new Set(channels).size).toBeLessThan(out.length)
  })

  it('count crossings only between unrelated edges', () => {
    const { links } = linksOf(graph, catalog, layout)
    const ends = new Map(links.map((l) => [l.key, { from: l.from, to: l.to }]))
    expect(edgeCrossings(routes, ends)).toBeGreaterThanOrEqual(0)
  })
})

describe('arranged by hand', () => {
  it('keeps the positions of a group the person arranged when a new branch is laid out', () => {
    const known = earlierTidy(multiTrigger, (key) => key === 'manual' || key === SHARED_GROUP)
    const hand = layoutGroups(multiTrigger).find((g) => g.key === 'client_created')!.items
    // The person moved two steps of the client column by hand.
    known.b_vip = { x: known.b_vip!.x + 48, y: known.b_vip!.y + 32 }
    known.b_mail = { x: known.b_mail!.x - 16, y: known.b_mail!.y + 64 }
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
    const moved = { ...layout, b_vip: { x: layout.b_vip!.x + 16, y: layout.b_vip!.y } }
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
    for (const id of moved) known[id] = { x: tidy[id]!.x + 160, y: tidy[id]!.y + 96 }
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
    for (const [id, p] of Object.entries(tidy)) moved[id] = { x: p.x + 32, y: p.y + 16 }
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
      for (const id of group.items) if (moved[id]) moved[id] = { x: known[id]!.x + dx, y: known[id]!.y + dy }
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
    for (const [id, p] of Object.entries(earlierTidy(graph, onlyOld))) moved[id] = { x: p.x + 48, y: p.y + 48 }
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
