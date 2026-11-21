import { chromium } from '@playwright/test'
const [,, url] = process.argv
const browser = await chromium.launch()
const page = await browser.newPage({ viewport: { width: 1440, height: 900 } })
await page.goto(url)
await page.waitForTimeout(500)
const box = await page.locator('.iota-flow__node[data-id="n0"] .ae-card__title').boundingBox()
const start = { x: box.x + 10, y: box.y + box.height / 2 }
await page.mouse.move(start.x, start.y); await page.mouse.down(); await page.mouse.move(start.x + 5, start.y + 5, { steps: 2 })
const cdp = await page.context().newCDPSession(page)
await cdp.send('Profiler.enable'); await cdp.send('Profiler.setSamplingInterval', { interval: 50 }); await cdp.send('Profiler.start')
const t0 = Date.now()
for (let i = 1; i <= 60; i++) { await page.mouse.move(start.x + i * 6, start.y + Math.sin(i / 6) * 40) }
const elapsed = Date.now() - t0
const { profile } = await cdp.send('Profiler.stop')
await page.mouse.up()
const byId = new Map(profile.nodes.map((n) => [n.id, n]))
const parent = new Map(); for (const n of profile.nodes) for (const c of n.children ?? []) parent.set(c, n.id)
const self = new Map(), total = new Map()
profile.samples.forEach((id, i) => {
  const dt = profile.timeDeltas[i] ?? 0
  const f = byId.get(id).callFrame
  const k = `${f.functionName || '(anon)'}:${f.lineNumber}`
  self.set(k, (self.get(k) ?? 0) + dt)
  const seen = new Set(); let cur = id
  while (cur !== undefined) { const cf = byId.get(cur).callFrame; const key = `${cf.functionName || '(anon)'}:${cf.lineNumber}`; if (!seen.has(key)) { seen.add(key); total.set(key, (total.get(key) ?? 0) + dt) } cur = parent.get(cur) }
})
console.log('elapsed', elapsed)
console.log('SELF'); for (const [k, v] of [...self].sort((a, b) => b[1] - a[1]).slice(0, 20)) console.log((v / 1000).toFixed(1).padStart(7), k)
console.log('TOTAL'); for (const [k, v] of [...total].sort((a, b) => b[1] - a[1]).slice(0, 40)) console.log((v / 1000).toFixed(1).padStart(7), k)
await browser.close()
