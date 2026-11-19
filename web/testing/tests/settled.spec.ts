import { test, expect, type Page } from '@playwright/test'
import { actionAndHtmxSettled, actionAndResponse, actionAndValidationRefusal } from '../src/index.js'
import path from 'node:path'

// All cases load the production HTMX asset; synthetic htmx:* events would make them falsely green.
async function host(page: Page, body: string, respond: (pathname: string) => Promise<{ status?: number; body: string } | 'abort' | 'hang'>) {
  await page.route('http://fixture.test/**', async route => {
    const pathname = new URL(route.request().url()).pathname
    if (pathname === '/') return route.fulfill({ contentType: 'text/html', body })
    const result = await respond(pathname)
    if (result === 'hang') return
    if (result === 'abort') return route.abort('failed')
    return route.fulfill({ status: result.status ?? 200, contentType: 'text/html', body: result.body })
  })
  await page.goto('http://fixture.test/')
  await page.addScriptTag({ path: path.resolve('../../modules/core/presentation/assets/js/lib/htmx.min.js') })
  await page.evaluate(() => (window as any).htmx.process(document.body))
}
function gate() {
  let release!: () => void
  const opened = new Promise<void>(resolve => { release = resolve })
  return { opened, release }
}

// Characterisation: falsely green if an outerHTML replacement were matched against the detached original target.
test('settles a delayed outerHTML replacement of the target', async ({ page }) => {
  await host(page, '<button hx-post="/save" hx-target="#panel" hx-swap="outerHTML settle:150ms">Save</button><section id="panel">Before</section>',
    async () => ({ body: '<section id="panel" data-version="2">After</section>' }))
  const start = Date.now()
  await actionAndHtmxSettled(page, '#panel', () => page.getByRole('button').click())
  expect(Date.now() - start).toBeGreaterThanOrEqual(140)
  await expect(page.locator('#panel')).toHaveAttribute('data-version', '2')
  await expect(page.locator('.htmx-settling')).toHaveCount(0)
})

// Regression: a row replaced by an empty response detaches the request target; its afterSettle no longer bubbles.
test('settles when the swap detaches the request target inside the scope', async ({ page }) => {
  await host(page, '<ul id="list"><li id="row">Row <button hx-delete="/rows/1" hx-target="closest li" hx-swap="outerHTML settle:50ms">Delete</button></li></ul>',
    async () => ({ body: '' }))
  await actionAndHtmxSettled(page, '#list', () => page.getByRole('button', { name: 'Delete' }).click(), { timeoutMs: 3000 })
  await expect(page.locator('#row')).toHaveCount(0)
})

// Characterisation: falsely green if the second gated request were excluded from correlation.
test('waits for every request that targets the scope', async ({ page }) => {
  const second = gate()
  await host(page, '<button id="a" hx-get="/a" hx-target="#panel .a">A</button><button id="b" hx-get="/b" hx-target="#panel .b">B</button><section id="panel"><p class="a"></p><p class="b"></p></section>',
    async pathname => {
      if (pathname === '/b') await second.opened
      return { body: pathname }
    })
  let complete = false
  const interaction = actionAndHtmxSettled(page, '#panel', () => page.evaluate(() => {
    document.getElementById('a')!.click(); document.getElementById('b')!.click()
  })).then(() => { complete = true })
  try {
    await expect(page.locator('#panel .a')).toHaveText('/a')
    expect(complete).toBe(false)
  } finally { second.release() }
  await interaction
  await expect(page.locator('#panel .b')).toHaveText('/b')
})

// Regression: a send error previously left the helper waiting for the full timeout.
test('rejects a network failure promptly', async ({ page }) => {
  await host(page, '<button hx-post="/save" hx-target="#panel">Save</button><section id="panel"></section>', async () => 'abort')
  const start = Date.now()
  await expect(actionAndHtmxSettled(page, '#panel', () => page.getByRole('button').click(), { timeoutMs: 5000 })).rejects.toThrow(/HTMX request failed: 0 POST/)
  expect(Date.now() - start).toBeLessThan(2000)
})

// Regression: a refusal rejected without its status; falsely green if refusals were treated as settled swaps.
test('rejects a server refusal with its status', async ({ page }) => {
  await host(page, '<button hx-post="/save" hx-target="#panel">Save</button><section id="panel"></section>', async () => ({ status: 422, body: 'invalid' }))
  await expect(actionAndHtmxSettled(page, '#panel', () => page.getByRole('button').click(), { timeoutMs: 5000 })).rejects.toThrow(/422/)
})

// Regression: a missing request must fail red with a diagnostic instead of an unexplained timeout.
test('rejects when the action issues no request for the target', async ({ page }) => {
  await host(page, '<button hx-post="/save" hx-target="#other">Save</button><section id="panel"></section><section id="other"></section>', async () => ({ body: 'x' }))
  await expect(actionAndHtmxSettled(page, '#panel', () => page.getByRole('button').click(), { timeoutMs: 500 })).rejects.toThrow(/no HTMX request targeted #panel/)
})

// Characterisation (new ready option): falsely green if consumer readiness were evaluated before the swap settled.
test('waits for consumer target readiness after settle', async ({ page }) => {
  await host(page, '<button hx-get="/chart" hx-target="#panel">Load</button><section id="panel"></section>',
    async () => ({ body: '<div class="chart" data-ready="false"></div><script>setTimeout(() => document.querySelector(".chart").dataset.ready = "true", 120)</script>' }))
  await actionAndHtmxSettled(page, '#panel', () => page.getByRole('button').click(), {
    ready: target => expect(target.locator('.chart')).toHaveAttribute('data-ready', 'true'),
  })
  await expect(page.locator('.chart')).toHaveAttribute('data-ready', 'true')
})

// Characterisation: falsely green if the listener were attached after the action.
test('captures a response that arrives before the action finishes', async ({ page }) => {
  await host(page, '<button hx-post="/save" hx-target="#panel">Save</button><section id="panel"></section>', async () => ({ body: 'done' }))
  const response = await actionAndResponse(page, { method: 'POST', pathname: '/save' }, async () => {
    await page.getByRole('button').click()
    await expect(page.locator('#panel')).toHaveText('done')
  })
  expect(response.status()).toBe(200)
})

// Characterisation: falsely green if the response wait were created without a handler and the action threw first.
test('reports the action failure and leaves no dangling response wait', async ({ page }) => {
  await host(page, '<p>idle</p>', async () => ({ body: '' }))
  const unhandled: unknown[] = []
  const record = (reason: unknown) => { unhandled.push(reason) }
  process.on('unhandledRejection', record)
  try {
    await expect(actionAndResponse(page, { method: 'POST', pathname: '/save' }, async () => { throw new Error('action exploded') }, { timeoutMs: 100 })).rejects.toThrow('action exploded')
    await expect.poll(async () => { await new Promise(resolve => setImmediate(resolve)); return true }).toBe(true)
    await page.waitForFunction(() => new Promise(resolve => setTimeout(() => resolve(true), 200)))
    expect(unhandled).toEqual([])
  } finally { process.off('unhandledRejection', record) }
})

// Regression: hx-put forms were classified as GET, so a submission issued during the refusal stayed unnoticed.
test('detects a submission request issued during a native refusal for non-POST verbs', async ({ page }) => {
  await host(page, '<form hx-put="/profile"><input name="email" type="email" required><button>Save</button></form>', async () => ({ body: '' }))
  await page.locator('button').evaluate(button => button.addEventListener('click', () => { void fetch('/profile', { method: 'PUT' }) }))
  await expect(actionAndValidationRefusal(page, page.locator('form'), () => page.getByRole('button').click())).rejects.toThrow(/submission request/)
})

// Characterisation: falsely green if the reported control were not checked as :invalid.
test('reports the first invalid control without spending the network timeout', async ({ page }) => {
  let requests = 0
  await host(page, '<form hx-patch="/profile"><input name="email" type="email" value="bad"><input name="name" required><button>Save</button></form>', async () => { requests++; return { body: '' } })
  const start = Date.now()
  const result = await actionAndValidationRefusal(page, page.locator('form'), () => page.getByRole('button').click(), { timeoutMs: 10_000 })
  expect(result.controlName).toBe('email')
  expect(result.message).not.toBe('')
  expect(Date.now() - start).toBeLessThan(2000)
  expect(requests).toBe(0)
})

// Characterisation: falsely green if a delayed swap counted as complete when the response loaded.
test('waits for a delayed swap before completing', async ({ page }) => {
  await host(page, '<button hx-get="/late" hx-target="#panel" hx-swap="innerHTML swap:200ms">Load</button><section id="panel">Before</section>', async () => ({ body: 'After' }))
  const start = Date.now()
  await actionAndHtmxSettled(page, '#panel', () => page.getByRole('button').click())
  expect(Date.now() - start).toBeGreaterThanOrEqual(180)
  await expect(page.locator('#panel')).toHaveText('After')
})

// Characterisation: falsely green if requests without swapped content waited for an afterSettle that never comes.
test('completes a request that swaps nothing', async ({ page }) => {
  await host(page, '<button hx-post="/ping" hx-target="#panel" hx-swap="none">Ping</button><section id="panel">Same</section>', async () => ({ body: 'ignored' }))
  await actionAndHtmxSettled(page, '#panel', () => page.getByRole('button').click(), { timeoutMs: 2000 })
  await expect(page.locator('#panel')).toHaveText('Same')
})
