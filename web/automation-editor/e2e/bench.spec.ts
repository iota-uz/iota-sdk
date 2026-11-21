// Performance regression on every @xyflow/system bump (issue #1331): a 200-step / 500-edge graph with
// the CPU throttled 4×. Thresholds: mount ≤ 150 ms (best of three reloads), whole-graph replacement
// (undo) ≤ 60 ms (median of ten), a drag
// without long tasks with a p95 frame ≤ 17 ms. The profile is written to e2e/results/bench.json.
import { mkdirSync, writeFileSync } from 'node:fs'
import path from 'node:path'
import { expect, test } from '@playwright/test'
import { card } from './helpers'

const THROTTLE = Number(process.env.AE_BENCH_THROTTLE ?? 4)
const LIMITS = { mountMs: 150, replaceMs: 60, frameP95Ms: 17, longTasks: 0 }

const median = (values: number[]) => {
  const sorted = [...values].sort((a, b) => a - b)
  return sorted[Math.floor(sorted.length / 2)] ?? 0
}

const p95 = (values: number[]) => {
  const sorted = [...values].sort((a, b) => a - b)
  return sorted[Math.min(sorted.length - 1, Math.floor(sorted.length * 0.95))] ?? 0
}

test('200 steps / 500 edges at 4× CPU throttle', async ({ page }) => {
  test.setTimeout(180_000)
  const cdp = await page.context().newCDPSession(page)
  // Warm the HTTP cache and the JIT once without throttling, as a returning user's tab would be.
  await page.goto('/?mode=canvas&graph=bench')
  await page.waitForFunction(() => document.documentElement.dataset.fixtureReady === 'true')
  await cdp.send('Emulation.setCPUThrottlingRate', { rate: THROTTLE })

  const mounts: number[] = []
  for (let i = 0; i < 3; i += 1) {
    await page.reload()
    await page.waitForFunction(() => document.documentElement.dataset.fixtureReady === 'true')
    mounts.push(await page.evaluate(() => window.__fixture.mountMs ?? Infinity))
  }
  const mountMs = Math.min(...mounts)
  // The whole graph is on the minimap; the canvas mounts what is in view.
  expect(await page.evaluate(() => [...document.querySelectorAll('.ae-minimap__node')].reduce((n, el) => n + (el.getAttribute('d')?.match(/M/g)?.length ?? 0), 0))).toBe(201)
  const mounted = { cards: await page.locator('.iota-flow__node').count(), edges: await page.locator('path.ae-edge').count() }
  expect(mounted.cards).toBeGreaterThan(10)

  // Two replacements warm the code paths (an undo comes after edits), then ten are measured; the median
  // is held to the limit and the slowest is reported.
  for (let i = 0; i < 2; i += 1) await page.evaluate(() => window.__fixture.replace!())
  const replaces: number[] = []
  for (let i = 0; i < 10; i += 1) replaces.push(await page.evaluate(() => window.__fixture.replace!()))
  const replaceMs = median(replaces)

  await page.evaluate(() => {
    const w = window as unknown as { __longTasks: number[]; __frames: number[]; __recording: boolean }
    w.__longTasks = []
    w.__frames = []
    w.__recording = false
    new PerformanceObserver((list) => {
      if (!w.__recording) return
      for (const entry of list.getEntries()) w.__longTasks.push(entry.duration)
    }).observe({ type: 'longtask', buffered: false })
    let last = 0
    const tick = (now: number) => {
      if (w.__recording && last) w.__frames.push(now - last)
      last = now
      requestAnimationFrame(tick)
    }
    requestAnimationFrame(tick)
  })
  // A step in the middle of the view, so the drag measures the canvas, not the edge auto-pan.
  const target = await page.evaluate(() => {
    const cx = innerWidth / 2
    const cy = innerHeight / 2
    let best = ''
    let distance = Infinity
    for (const el of document.querySelectorAll('.iota-flow__node')) {
      const r = el.getBoundingClientRect()
      const d = Math.hypot(r.x + r.width / 2 - cx, r.y + r.height / 2 - cy)
      if (d < distance) {
        distance = d
        best = el.getAttribute('data-id') ?? ''
      }
    }
    return best
  })
  const box = (await card(page, target).locator('.ae-card__title').boundingBox())!
  const start = { x: box.x + 10, y: box.y + box.height / 2 }
  await page.mouse.move(start.x, start.y)
  await page.mouse.down()
  await page.mouse.move(start.x + 4, start.y + 4, { steps: 2 })
  await page.evaluate(() => { (window as unknown as { __recording: boolean }).__recording = true })
  for (let i = 1; i <= 60; i += 1) {
    await page.mouse.move(start.x + Math.sin(i / 8) * 160, start.y + Math.sin(i / 6) * 60)
    await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => resolve(null))))
  }
  await page.evaluate(() => { (window as unknown as { __recording: boolean }).__recording = false })
  await page.mouse.up()
  const { frames, longTasks } = await page.evaluate(() => {
    const w = window as unknown as { __longTasks: number[]; __frames: number[] }
    return { frames: w.__frames, longTasks: w.__longTasks }
  })
  await cdp.send('Emulation.setCPUThrottlingRate', { rate: 1 })

  const profile = {
    throttle: THROTTLE,
    graph: { nodes: 200, edges: 500 },
    mountMs: Math.round(mountMs * 10) / 10,
    mounts,
    mounted,
    replaceMs: Math.round(replaceMs * 10) / 10,
    replaceMaxMs: Math.round(Math.max(...replaces) * 10) / 10,
    replaces,
    frameP95Ms: Math.round(p95(frames) * 10) / 10,
    frames: frames.length,
    longTasks,
    limits: LIMITS,
  }
  const out = path.resolve('e2e/results')
  mkdirSync(out, { recursive: true })
  writeFileSync(path.join(out, 'bench.json'), `${JSON.stringify(profile, null, 2)}\n`)
  console.log(JSON.stringify(profile))
  expect(profile.mountMs, 'mount').toBeLessThanOrEqual(LIMITS.mountMs)
  expect(profile.replaceMs, 'replace the whole graph').toBeLessThanOrEqual(LIMITS.replaceMs)
  expect(longTasks.length, 'long tasks during a drag').toBe(LIMITS.longTasks)
  expect(profile.frameP95Ms, 'p95 frame during a drag').toBeLessThanOrEqual(LIMITS.frameP95Ms)
})
