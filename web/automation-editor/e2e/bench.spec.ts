// Performance regression on every @xyflow/system bump (issue #1331): a 200-step / 500-edge graph with
// the CPU throttled 4×, with the drag and Tidy animations on (#1352). Thresholds: mount ≤ 150 ms (best of
// three reloads), whole-graph replacement (undo) ≤ 60 ms (median of ten), a drag without long tasks with a
// p95 frame ≤ 17 ms. Tidy (#1352): the frame that renders the new layout is a whole-layout replacement
// (≤ 60 ms); the glide after it has no long tasks, a median frame of one display frame and a p95 of at most
// three (every card and edge on screen moves on every frame), in the best of three rounds (there and back),
// as the mount is the best of three reloads: one round is about twenty frames, so a single stall of a
// shared runner decides it, while a slower Tidy is slower in every round. The profile is written to e2e/results/bench.json.
import { mkdirSync, writeFileSync } from 'node:fs'
import path from 'node:path'
import { expect, test } from '@playwright/test'
import { card } from './helpers'

// The limits hold for a reference runner (GitHub's ubuntu-24.04) slowed 4×. A runner's own speed varies
// from run to run (a slower host made the mount 140 ms and a replacement 70 ms with the same code), so a
// fixed CPU workload is timed unthrottled first, and on a slower runner the throttle is lowered so the
// effective CPU stays the reference one slowed 4× (as Lighthouse calibrates its throttling). The
// throttle never goes above 4× nor below 2×: a faster runner keeps 4×, and a runner more than twice as
// slow as the reference still fails a real regression. AE_BENCH_THROTTLE sets the throttle directly.
const FIXED_THROTTLE = process.env.AE_BENCH_THROTTLE ? Number(process.env.AE_BENCH_THROTTLE) : undefined
const THROTTLE = 4
const MIN_THROTTLE = 2
const REFERENCE_CALIBRATION_MS = Number(process.env.AE_BENCH_REFERENCE_MS ?? 0) || 9.5
const LIMITS = { mountMs: 150, replaceMs: 60, frameP95Ms: 17, longTasks: 0 }
// One display frame is 16.7 ms; the measured intervals are multiples of it plus a fraction.
const TIDY_LIMITS = { commitFrameMs: 60, glideFrameMedianMs: 17, glideFrameP95Ms: 51, longTasks: 0 }

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
  // The calibration workload: object allocation, sorting and string building, the kind of work a layout
  // and render pass does. The fastest of several runs after a warm-up is the runner's speed.
  const calibrationMs = await page.evaluate(() => {
    const work = () => {
      const t = performance.now()
      let rows = Array.from({ length: 60_000 }, (_, i) => ({ id: `n${(i * 7919) % 60_000}`, x: (i * 31) % 997, y: (i * 17) % 991 }))
      rows = rows.sort((a, b) => a.x - b.x || a.y - b.y || (a.id < b.id ? -1 : 1))
      let out = ''
      for (const r of rows) out += `${r.id}:${r.x},${r.y};`
      return out.length > 0 ? performance.now() - t : Infinity
    }
    work()
    const runs: number[] = []
    for (let i = 0; i < 9; i += 1) runs.push(work())
    return Math.min(...runs)
  })
  const throttle = FIXED_THROTTLE ?? Math.min(THROTTLE, Math.max(MIN_THROTTLE, THROTTLE * REFERENCE_CALIBRATION_MS / calibrationMs))
  await cdp.send('Emulation.setCPUThrottlingRate', { rate: throttle })

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
  // interval; its first frame is held to the replacement limit and the glide (there and back) to TIDY_LIMITS.
  await page.evaluate(() => window.__fixture.prepareTidy!())
  type Glide = { frames: number[]; longTasks: number[]; where: string[]; slow: string[]; callbackMs: number[]; commitFrame: number; commitTasks: number[] }
  const rounds: Glide[][] = []
  for (let round = 0; round < 3; round += 1) {
    const glides: Glide[] = []
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
        const all = w.__frames.filter((t) => t >= start && t <= end)
        // The first frame renders the committed layout (and the glide's first step); the glide is after it.
        const commitFrame = all.length > 1 ? all[1]! - all[0]! : 0
        const commitTasks = w.__longTasks.filter((e) => e.start < (all[1] ?? end) && e.start + e.duration > start).map((e) => e.duration)
        const stamps = all.slice(1)
        const from = stamps[0] ?? end
        const tasks = w.__longTasks.filter((e) => e.start >= from && e.start < end)
        const loaf = (w as unknown as { __loaf: { start: number; duration: number; script: number; style: number; raf: number }[] }).__loaf
        const callbacks = (w as unknown as { __callbacks: { at: number; ms: number }[] }).__callbacks.filter((c) => c.at >= start && c.at <= end)
        const perFrame = stamps.slice(1).map((t, i) => callbacks.filter((c) => c.at >= stamps[i]! && c.at < t).reduce((n, c) => n + c.ms, 0))
        return {
          commitFrame,
          commitTasks,
          callbackMs: perFrame.map((n) => Math.round(n)),
          slow: loaf.filter((e) => e.start < end && e.start + e.duration > start).map((e) => `${Math.round(e.start - start)}: ${e.duration} = tasks ${e.script} + frame callbacks ${e.raf} + style, layout, paint ${e.style}`),
          frames: stamps.slice(1).map((t, i) => t - stamps[i]!),
          longTasks: tasks.map((e) => e.duration),
          // Where each long task falls in the glide (ms after its start, of its length): for the profile.
          where: tasks.map((e) => `${Math.round(e.start - start)}+${Math.round(e.duration)}/${Math.round(end - start)}`),
        }
      }, { start, end }))
    }
    rounds.push(glides)
  }
  // The best round: the first that meets every Tidy limit; when none does, the fewest long tasks, then
  // the lowest median and p95 frame, then the fastest commit frame.
  const score = (glides: Glide[]) => {
    const frames = glides.flatMap((g) => g.frames)
    return [glides.flatMap((g) => g.longTasks).length, median(frames), p95(frames), Math.max(...glides.map((g) => g.commitFrame))]
  }
  const meetsLimits = (glides: Glide[]) => {
    const [longTasks, frameMedian, frameP95, commitFrame] = score(glides)
    return glides.flatMap((g) => g.frames).length > 2 && longTasks === TIDY_LIMITS.longTasks &&
      Math.round(frameMedian! * 10) / 10 <= TIDY_LIMITS.glideFrameMedianMs &&
      Math.round(frameP95! * 10) / 10 <= TIDY_LIMITS.glideFrameP95Ms &&
      Math.round(commitFrame! * 10) / 10 <= TIDY_LIMITS.commitFrameMs
  }
  const compare = (x: number[], y: number[]) => {
    for (let i = 0; i < x.length; i += 1) if (x[i] !== y[i]) return x[i]! - y[i]!
    return 0
  }
  const glides = rounds.find(meetsLimits) ?? [...rounds].sort((a, b) => compare(score(a), score(b)))[0]!
  const tidyFrames = glides.flatMap((g) => g.frames)
  const tidyLongTasks = glides.flatMap((g) => g.longTasks)
  await cdp.send('Emulation.setCPUThrottlingRate', { rate: 1 })

  const profile = {
    throttle: Math.round(throttle * 100) / 100,
    calibrationMs: Math.round(calibrationMs * 10) / 10,
    referenceCalibrationMs: REFERENCE_CALIBRATION_MS,
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
    tidyCommitFrameMs: glides.map((g) => Math.round(g.commitFrame * 10) / 10),
    tidyCommitLongTasks: glides.flatMap((g) => g.commitTasks),
    tidyFrameMedianMs: Math.round(median(tidyFrames) * 10) / 10,
    tidyFrameP95Ms: Math.round(p95(tidyFrames) * 10) / 10,
    tidyFrames: tidyFrames.length,
    tidyLongTasks,
    tidyLongTasksAt: glides.flatMap((g) => g.where),
    tidyFrameMs: glides.map((g) => g.frames.map((f) => Math.round(f))),
    tidySlowFrames: glides.map((g) => g.slow),
    tidyCallbackMs: glides.map((g) => g.callbackMs),
    // Every round, for the profile: [long tasks, median, p95, slowest commit frame].
    tidyRounds: rounds.map((r) => score(r).map((v) => Math.round(v * 10) / 10)),
    limits: LIMITS,
    tidyLimits: TIDY_LIMITS,
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
  for (const ms of profile.tidyCommitFrameMs) expect(ms, 'the frame that renders the tidied layout').toBeLessThanOrEqual(TIDY_LIMITS.commitFrameMs)
  expect(tidyLongTasks.length, 'long tasks during the Tidy glide').toBe(TIDY_LIMITS.longTasks)
  expect(profile.tidyFrameMedianMs, 'median frame during the Tidy glide').toBeLessThanOrEqual(TIDY_LIMITS.glideFrameMedianMs)
  expect(profile.tidyFrameP95Ms, 'p95 frame during the Tidy glide').toBeLessThanOrEqual(TIDY_LIMITS.glideFrameP95Ms)
})
