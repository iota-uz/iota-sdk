import { test, expect } from '@playwright/test'
import { actionAndResponse, actionAndHtmxResponse, actionAndValidationRefusal, expectComponentReady } from '../src/index.js'
import path from 'node:path'

// Characterisation: falsely green if synthetic events replace the production HTMX asset.
async function fixture(page: import('@playwright/test').Page, status = 200) {
  await page.route('http://fixture.test/**', route => {
    if (new URL(route.request().url()).pathname === '/save') return route.fulfill({ status, contentType: 'text/html', body: '<section id="result">Saved</section>' })
    return route.fulfill({ contentType: 'text/html', body: '<form hx-post="/save" hx-target="#result" hx-swap="outerHTML settle:150ms"><input name="name" required><button>Save</button></form><section id="result">Before</section>' })
  })
  await page.goto('http://fixture.test/')
  await page.addScriptTag({ path: path.resolve('../../modules/core/presentation/assets/js/lib/htmx.min.js') })
  await page.evaluate(() => (window as any).htmx.process(document.body))
}
test('captures fast response and waits for delayed replacement settle', async ({ page }) => {
  await fixture(page)
  await page.locator('input').fill('Ada')
  const start = Date.now()
  await actionAndHtmxResponse(page, { method: 'POST', pathname: '/save' }, () => page.getByRole('button').click())
  expect(Date.now() - start).toBeGreaterThanOrEqual(140)
  await expect(page.locator('#result')).toHaveText('Saved')
})
// Characterisation: falsely green if invalid forms never attach the submission handler.
test('native refusal completes without waiting for a network timeout', async ({ page }) => {
  await fixture(page)
  let requests = 0
  page.on('request', r => { if (r.method() === 'POST') requests++ })
  const start = Date.now()
  const result = await actionAndValidationRefusal(page, page.locator('form'), () => page.getByRole('button').click())
  expect(result.controlName).toBe('name')
  expect(result.message).not.toBe('')
  expect(requests).toBe(0)
  expect(Date.now() - start).toBeLessThan(2000)
  await page.locator('input').fill('Ada')
  await actionAndResponse(page, { method: 'POST', pathname: '/save' }, () => page.getByRole('button').click())
  expect(requests).toBe(1)
})
// Characterisation: falsely green if the failure assertions are removed or requests are mocked as successes.
test('missing request, server refusal and absent readiness reject', async ({ page }) => {
  await fixture(page, 422)
  await page.locator('input').fill('Ada')
  await expect(actionAndHtmxResponse(page, { method: 'POST', pathname: '/save' }, () => page.getByRole('button').click())).rejects.toThrow('422')
  await expect(actionAndResponse(page, { method: 'POST', pathname: '/missing' }, async () => {}, { timeoutMs: 100 })).rejects.toThrow()
  await expect(expectComponentReady(page, page.locator('#result'), { timeoutMs: 100 })).rejects.toThrow()
})
// Characterisation: falsely green if the second request is never started before the first settles.
test('waits for both matching requests, including the slower second swap', async ({ page }) => {
  await fixture(page)
  await page.evaluate(() => {
    document.body.innerHTML = '<button id="a" hx-post="/save" hx-target="#first" hx-swap="innerHTML settle:20ms">First</button><button id="b" hx-post="/save" hx-target="#second" hx-swap="innerHTML settle:200ms">Second</button><section id="first"></section><section id="second"></section>'
    ;(window as any).htmx.process(document.body)
  })
  await actionAndHtmxResponse(page, { method: 'POST', pathname: '/save' }, () => page.evaluate(() => {
    document.getElementById('a')!.click(); document.getElementById('b')!.click()
  }))
  await expect(page.locator('#first')).toHaveText('Saved')
  await expect(page.locator('#second')).toHaveText('Saved')
  await expect(page.locator('.htmx-settling')).toHaveCount(0)
})
// Characterisation: falsely green if the pending second XHR is excluded until it has an HTTP status.
test('does not resolve while a matching second response remains gated', async ({ page }) => {
  await fixture(page)
  let release!: () => void
  const gate = new Promise<void>(resolve => { release = resolve })
  let count = 0
  await page.route('http://fixture.test/save', async route => {
    count++
    if (count === 2) await gate
    await route.fulfill({ contentType: 'text/html', body: 'Saved' })
  })
  await page.evaluate(() => {
    document.body.innerHTML = '<button id="a" hx-post="/save" hx-target="#first" hx-swap="innerHTML settle:0ms">First</button><button id="b" hx-post="/save" hx-target="#second" hx-swap="innerHTML settle:0ms">Second</button><section id="first"></section><section id="second"></section>'
    ;(window as any).htmx.process(document.body)
  })
  let complete = false
  const interaction = actionAndHtmxResponse(page, { method: 'POST', pathname: '/save' }, () => page.evaluate(() => {
    document.getElementById('a')!.click(); document.getElementById('b')!.click()
  })).then(() => { complete = true })
  try {
    await expect(page.locator('#first')).toHaveText('Saved')
    await expect(page.locator('#first')).not.toHaveClass(/htmx-settling/)
    expect(complete).toBe(false)
  } finally { release() }
  await interaction
  await expect(page.locator('#second')).toHaveText('Saved')
})
// Falsely green if the real HTMX target remains connected and events still bubble.
test('completes when the swap deletes its request target', async ({ page }) => {
  await fixture(page)
  await page.evaluate(() => {
    document.body.innerHTML = '<button hx-post="/save" hx-swap="delete settle:150ms">Remove</button>'
    ;(window as any).htmx.process(document.body)
  })
  await actionAndHtmxResponse(page, { method: 'POST', pathname: '/save' }, () => page.getByRole('button').click())
  await expect(page.getByRole('button')).toHaveCount(0)
})
