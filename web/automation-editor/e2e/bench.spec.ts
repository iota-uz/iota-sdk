// Performance regression on every @xyflow/system bump (issue #1331): a 200-step / 500-edge graph with
// the CPU throttled 4×, with the drag and Tidy animations on (#1352). Thresholds: mount ≤ 150 ms (best of three reloads), whole-graph replacement
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

  // Long tasks and frames are recorded all the time with their timestamps and filtered by the drag's
  // interval afterwards (pointerdown → end of the drag, after the last frames and entries are delivered),
  // so a task at either edge of the drag or delivered late is not missed.
  await page.evaluate(() => {
    const w = window as unknown as { __longTasks: { start: number; duration: number }[]; __frames: number[] }
    w.__longTasks = []
    w.__frames = []
    new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) w.__longTasks.push({ start: entry.startTime, duration: entry.duration })
    }).observe({ type: 'longtask', buffered: true })
    const tick = (now: number) => {
      w.__frames.push(now)
      requestAnimationFrame(tick)
    }
    requestAnimationFrame(tick)
    // For the profile: the frames over 50 ms split into tasks before rendering, animation frame callbacks,
    // and style, layout and paint (Long Animation Frames, where the browser has them).
    // And how long the page's own animation frame callbacks (the glide's tween) take in each frame.
    const cb = w as unknown as { __callbacks: { at: number; ms: number }[] }
    cb.__callbacks = []
    const raf = window.requestAnimationFrame.bind(window)
    window.requestAnimationFrame = (run) => raf((now) => {
      const t = performance.now()
      run(now)
      cb.__callbacks.push({ at: t, ms: performance.now() - t })
    })
    const loaf = w as unknown as { __loaf: { start: number; duration: number; script: number; style: number; raf: number }[] }
    loaf.__loaf = []
    try {
      new PerformanceObserver((list) => {
        for (const e of list.getEntries() as (PerformanceEntry & { renderStart: number; styleAndLayoutStart: number })[]) {
          const end = e.startTime + e.duration
          loaf.__loaf.push({
            start: e.startTime,
            duration: Math.round(e.duration),
            script: Math.round((e.renderStart || end) - e.startTime),
            style: Math.round(e.styleAndLayoutStart ? end - e.styleAndLayoutStart : 0),
            raf: Math.round(e.renderStart && e.styleAndLayoutStart ? e.styleAndLayoutStart - e.renderStart : 0),
          })
        }
      }).observe({ type: 'long-animation-frame', buffered: false })
    } catch { /* not in this browser */ }
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
  const from = await page.evaluate(() => performance.now())
  await page.mouse.down()
  await page.mouse.move(start.x + 4, start.y + 4, { steps: 2 })
  for (let i = 1; i <= 60; i += 1) {
    await page.mouse.move(start.x + Math.sin(i / 8) * 160, start.y + Math.sin(i / 6) * 60)
    await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => resolve(null))))
  }
  await page.mouse.up()
  // The end of the drag (onMove, re-render) and the delivery of its entries are inside the interval.
  const to = await page.evaluate(() => new Promise<number>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve(performance.now())))))
  await page.waitForTimeout(300)
  const { frames, longTasks } = await page.evaluate(({ from, to }) => {
    const w = window as unknown as { __longTasks: { start: number; duration: number }[]; __frames: number[] }
    const stamps = w.__frames.filter((t) => t >= from && t <= to)
    return {
      frames: stamps.slice(1).map((t, i) => t - stamps[i]!),
      longTasks: w.__longTasks.filter((e) => e.start < to && e.start + e.duration > from).map((e) => e.duration),
    }
  }, { from, to })

  // Tidy with its glide (#1352): the cards move to the tidied layout in one --motion-slow tween and the
  // edges follow frame by frame. The layout is computed before, and committed at the start of, the measured
  // interval; the glide (there and back) is held to the drag's limits.
  await page.evaluate(() => window.__fixture.prepareTidy!())
  const glides: { frames: number[]; longTasks: number[]; where: string[]; slow: string[]; callbackMs: number[] }[] = []
  for (let i = 0; i < 2; i += 1) {
    // The commit itself (a new layout: routes and memos, the same with or without a glide) is the start;
    // what is measured is the glide after it.
    // The glide ends when the canvas drops its gliding class; the routing for the new layout after it is
    // Tidy's own work, the same without a glide.
    // One evaluate that resolves when the glide ends: nothing polls the page while it glides.
    const { start, end } = await page.evaluate(() => new Promise<{ start: number; end: number }>((resolve) => {
      const canvas = document.querySelector('.ae-canvas')!
      let begun = 0
      const watch = new MutationObserver(() => {
        if (canvas.classList.contains('ae-canvas--tidying')) return
        watch.disconnect()
        resolve({ start: begun, end: performance.now() })
      })
      window.__fixture.tidy!()
      begun = performance.now()
      watch.observe(canvas, { attributes: true, attributeFilter: ['class'] })
    }))
    await page.waitForTimeout(300)
    glides.push(await page.evaluate(({ start, end }) => {
      const w = window as unknown as { __longTasks: { start: number; duration: number }[]; __frames: number[] }
      const stamps = w.__frames.filter((t) => t >= start && t <= end)
      const tasks = w.__longTasks.filter((e) => e.start < end && e.start + e.duration > start)
      const loaf = (w as unknown as { __loaf: { start: number; duration: number; script: number; style: number; raf: number }[] }).__loaf
      const callbacks = (w as unknown as { __callbacks: { at: number; ms: number }[] }).__callbacks.filter((c) => c.at >= start && c.at <= end)
      const perFrame = stamps.slice(1).map((t, i) => callbacks.filter((c) => c.at >= stamps[i]! && c.at < t).reduce((n, c) => n + c.ms, 0))
      return {
        callbackMs: perFrame.map((n) => Math.round(n)),
        slow: loaf.filter((e) => e.start < end && e.start + e.duration > start).map((e) => `${Math.round(e.start - start)}: ${e.duration} = tasks ${e.script} + frame callbacks ${e.raf} + style, layout, paint ${e.style}`),
        frames: stamps.slice(1).map((t, i) => t - stamps[i]!),
        longTasks: tasks.map((e) => e.duration),
        // Where each long task falls in the glide (ms after its start, of its length): for the profile.
        where: tasks.map((e) => `${Math.round(e.start - start)}+${Math.round(e.duration)}/${Math.round(end - start)}`),
      }
    }, { start, end }))
  }
  const tidyFrames = glides.flatMap((g) => g.frames)
  const tidyLongTasks = glides.flatMap((g) => g.longTasks)
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
    tidyFrameP95Ms: Math.round(p95(tidyFrames) * 10) / 10,
    tidyFrames: tidyFrames.length,
    tidyLongTasks,
    tidyLongTasksAt: glides.flatMap((g) => g.where),
    tidyFrameMs: glides.map((g) => g.frames.map((f) => Math.round(f))),
    tidySlowFrames: glides.map((g) => g.slow),
    tidyCallbackMs: glides.map((g) => g.callbackMs),
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
  expect(tidyFrames.length, 'frames recorded during the Tidy glide').toBeGreaterThan(2)
  expect(tidyLongTasks.length, 'long tasks during the Tidy glide').toBe(LIMITS.longTasks)
  expect(profile.tidyFrameP95Ms, 'p95 frame during the Tidy glide').toBeLessThanOrEqual(LIMITS.frameP95Ms)
})
