import { describe, expect, it } from 'vitest'
import { ARROW_GAP, fitRoute, trimEnd } from './edgefit'
import { createRouter, routePath, type Point, type RouteLink } from './route'
import type { Rect } from './geometry'

/** Radii of the corners of a drawn path: for each `L a Q c b`, the distance from the corner `c` to where the curve starts. */
function radii(d: string): number[] {
  const out: number[] = []
  const re = /L(-?[\d.]+),(-?[\d.]+)Q(-?[\d.]+),(-?[\d.]+) (-?[\d.]+),(-?[\d.]+)/g
  for (const m of d.matchAll(re)) {
    const [ax, ay, cx, cy] = m.slice(1, 5).map(Number) as [number, number, number, number]
    out.push(Math.hypot(cx - ax, cy - ay))
  }
  return out
}

const card = (x: number, y: number, width = 224): Rect => ({ x, y, width, height: 96 })
/** The drawn path of one link, as the canvas draws it: routed, fitted onto the handles, ended short for the arrow. */
function drawn(rects: Map<string, Rect>, links: RouteLink[], key: string): string {
  const router = createRouter(rects, links)
  const link = links.find((l) => l.key === key)!
  const found = router.route(key)!
  const fit = fitRoute(found.points, link.port, link.entry)!
  return routePath(trimEnd(fit, ARROW_GAP), 10, found.clear)
}

describe('drawn edges', () => {
  // Rows 90 px apart: a card's bottom at 96, the next row's top at 186.
  const rects = new Map<string, Rect>([
    ['a', card(0, 0, 320)],
    ['l', card(-200, 186)],
    ['m', card(48, 186)],
    ['r', card(300, 186)],
  ])
  const links: RouteLink[] = [
    { key: 'a.left', from: 'a', to: 'l', port: { x: 81, y: 96 }, entry: { x: -88, y: 186 } },
    { key: 'a.mid', from: 'a', to: 'm', port: { x: 160.5, y: 96 }, entry: { x: 160, y: 186 } },
    { key: 'a.right', from: 'a', to: 'r', port: { x: 240.5, y: 96 }, entry: { x: 412, y: 186 } },
  ]

  it('every corner of an edge across a 90 px row gap keeps a radius of at least 8', () => {
    for (const key of ['a.left', 'a.right']) {
      const d = drawn(rects, links, key)
      const rs = radii(d)
      expect(rs.length, d).toBe(2)
      for (const r of rs) expect(r, d).toBeGreaterThanOrEqual(8)
    }
  })

  it('runs the horizontal stretch through the middle of the row gap', () => {
    const fit = fitRoute(createRouter(rects, links).route('a.right')!.points, links[2]!.port, links[2]!.entry)!
    const ys = fit.slice(1, -1).map((p: Point) => p.y)
    // The balanced middle (the arrow takes 6 px off the last stub), give or take a lane.
    for (const y of ys) expect(Math.abs(y - (96 + 186 - 6) / 2)).toBeLessThanOrEqual(12)
  })

  it('a straight edge whose ends are under 2 px apart is one line, no curve', () => {
    const d = drawn(rects, links, 'a.mid')
    expect(d).not.toContain('Q')
    expect(d).toBe('M160.5,96L160.5,180')
  })

  it('skips corners too small to draw and drops sub-pixel stretches', () => {
    const fit = fitRoute([{ x: 100, y: 0 }, { x: 100, y: 40 }, { x: 100.4, y: 40 }, { x: 100.4, y: 90 }], { x: 100, y: 0 }, { x: 100.4, y: 90 })!
    expect(fit).toEqual([{ x: 100, y: 0 }, { x: 100, y: 90 }])
    expect(routePath([{ x: 0, y: 0 }, { x: 0, y: 40 }, { x: 1.5, y: 40 }, { x: 1.5, y: 90 }])).not.toContain('Q')
  })
})

describe('lane changes', () => {
  it('a step of a few px across is one smooth S, not two kinks', () => {
    const d = routePath(trimEnd([{ x: 106, y: 672 }, { x: 106, y: 696 }, { x: 112, y: 696 }, { x: 112, y: 720 }]))
    expect(d).toBe('M106,672L106,686C106,696 112,696 112,706L112,714')
    expect(d).not.toContain('Q')
  })
  it('a step wider than two corners keeps its two full corners', () => {
    expect(radii(routePath([{ x: 0, y: 0 }, { x: 0, y: 45 }, { x: 60, y: 45 }, { x: 60, y: 90 }]))).toEqual([10, 10])
  })
})

describe('tight rows', () => {
  it('a 48 px row gap still gives both corners their full radius', () => {
    const rects = new Map<string, Rect>([['a', card(0, 0, 320)], ['r', card(400, 144)]])
    const links: RouteLink[] = [{ key: 'a.x', from: 'a', to: 'r', port: { x: 240, y: 96 }, entry: { x: 512, y: 144 } }]
    const d = drawn(rects, links, 'a.x')
    expect(radii(d), d).toEqual([10, 10])
  })
})
