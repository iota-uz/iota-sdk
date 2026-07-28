// Renders a printed lens report to a real PDF, the way a browser's "Save as
// PDF" would, without opening a print dialog — which would block both the page
// and any automation driving it, and is why printed output is otherwise
// impossible to review.
//
//   pnpm ladle:build && pnpm ladle:preview   # in one shell
//   pnpm print-pdf [url] [output]            # in another
//
// Any page that renders `.lens-print-report` works, including a running
// dashboard opened with `?lens-print-preview=1`.
import { chromium } from '@playwright/test'

const url = process.argv[2] ?? 'http://127.0.0.1:61000/?story=print-report--composed-report&mode=preview'
const out = process.argv[3] ?? '/tmp/print-report.pdf'

const browser = await chromium.launch()
const page = await browser.newPage({ viewport: { width: 1280, height: 1000 } })
await page.goto(url, { waitUntil: 'networkidle' })
// The printed document lives behind html.lens-print-active in @media print.
await page.evaluate(async () => {
  document.documentElement.classList.add('lens-print-active')
  const report = document.querySelector('.lens-print-report')
  if (report) report.removeAttribute('data-preview')
  await document.fonts.ready
  await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)))
})
await page.pdf({ path: out, format: 'A4', printBackground: true, preferCSSPageSize: true })
await browser.close()
console.log('wrote', out)
