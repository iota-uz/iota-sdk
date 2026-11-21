import { chromium } from '@playwright/test'
const [,, url] = process.argv
const browser = await chromium.launch()
const page = await browser.newPage({ viewport: { width: 1440, height: 900 } })
await page.goto(url)
await page.waitForTimeout(500)
await page.evaluate(() => { for (let i = 0; i < 4; i++) window.__fixture.replace() })
await browser.startTracing(page, { categories: ['devtools.timeline', 'blink', 'v8'] })
const times = await page.evaluate(() => { const r = []; for (let i = 0; i < 10; i++) r.push(+window.__fixture.replace().toFixed(1)); return r })
const buf = await browser.stopTracing()
const events = JSON.parse(buf.toString()).traceEvents
const sum = new Map()
for (const e of events) if (e.ph === 'X' && e.dur) sum.set(e.name, (sum.get(e.name) ?? 0) + e.dur)
console.log(times)
for (const [k, v] of [...sum].sort((a, b) => b[1] - a[1]).slice(0, 25)) console.log((v / 1000).toFixed(1).padStart(8), k)
const lt = events.filter((e) => e.name === 'UpdateLayoutTree' || e.name === 'Layout').slice(0, 6).map((e) => ({ n: e.name, d: (e.dur/1000).toFixed(2), a: JSON.stringify(e.args).slice(0, 200) }))
console.log(lt)
await browser.close()
