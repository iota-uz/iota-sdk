import { chromium } from '@playwright/test'
const [,, url, action = 'mount'] = process.argv
const browser = await chromium.launch()
const page = await browser.newPage({ viewport: { width: 1440, height: 900 } })
await page.goto(url)
await page.waitForTimeout(500)
const cdp = await page.context().newCDPSession(page)
await cdp.send('Profiler.enable')
await cdp.send('Profiler.setSamplingInterval', { interval: 100 })
await cdp.send('Profiler.start')
if (action === 'mount') { await page.reload(); await page.waitForTimeout(300) }
else { for (let i = 0; i < 6; i++) await page.evaluate(() => window.__fixture.replace()) }
const { profile } = await cdp.send('Profiler.stop')
const self = new Map()
const byId = new Map(profile.nodes.map((n) => [n.id, n]))
const dt = profile.timeDeltas
const counts = new Map()
profile.samples.forEach((id, i) => counts.set(id, (counts.get(id) ?? 0) + (dt[i] ?? 0)))
for (const [id, t] of counts) {
  const n = byId.get(id)
  const f = n.callFrame
  const key = `${f.functionName || '(anon)'} ${f.url.split('/').pop()}:${f.lineNumber}`
  self.set(key, (self.get(key) ?? 0) + t)
}
const total = [...self.values()].reduce((a, b) => a + b, 0)
console.log('total ms', (total / 1000).toFixed(1))
for (const [k, v] of [...self].sort((a, b) => b[1] - a[1]).slice(0, 35)) console.log((v / 1000).toFixed(1).padStart(7), k)
await browser.close()
