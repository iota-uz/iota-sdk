import { test, expect } from '@playwright/test'
import { assertCaseCoverage, componentCase, expectComponentReady, openComponentCase, ReadinessError, type ComponentCase, type ComponentHost } from '../src/index.js'

// Regression: the timeout previously rejected with a bare message, losing the root state needed to diagnose it.
test('readiness timeout rejects with the diagnostic root state', async ({ page }) => {
  await page.setContent('<section id="root" data-component-ready="loading"><canvas data-chart-ready="false"></canvas></section>')
  const error = await expectComponentReady(page, page.locator('#root'), { timeoutMs: 150, charts: page.locator('canvas') }).catch(e => e)
  expect(error).toBeInstanceOf(ReadinessError)
  expect(error.state).toMatchObject({ rootCount: 1, readiness: 'loading', charts: ['false'] })
  expect(error.message).toContain('data-component-ready="loading"')
})

// Regression: a root detaching between locator reads blocked on auto-waiting past the readiness budget.
test('a root that detaches during the poll still rejects within the budget', async ({ page }) => {
  await page.setContent('<section id="root" data-component-ready="false">x</section><script>setTimeout(() => document.getElementById("root").remove(), 60)</script>')
  const start = Date.now()
  await expect(expectComponentReady(page, page.locator('#root'), { timeoutMs: 400 })).rejects.toThrow(ReadinessError)
  expect(Date.now() - start).toBeLessThan(1500)
})

// Characterisation: falsely green if chart readiness were not part of the component readiness wait.
test('waits for declared charts after the root marker', async ({ page }) => {
  await page.setContent('<section id="root" data-component-ready="true"><canvas data-chart-ready="false"></canvas></section><script>setTimeout(() => document.querySelector("canvas").dataset.chartReady = "true", 150)</script>')
  const start = Date.now()
  await expectComponentReady(page, page.locator('#root'), { charts: page.locator('canvas') })
  expect(Date.now() - start).toBeGreaterThanOrEqual(100)
})

// Characterisation: falsely green if a missing root counted as ready.
test('an absent root rejects instead of passing vacuously', async ({ page }) => {
  await page.setContent('<p>no host</p>')
  await expect(expectComponentReady(page, page.locator('#root'), { timeoutMs: 100 })).rejects.toThrow(/roots=0/)
})

const cases: ComponentCase[] = [
  { id: 'ready-en-light-desktop', state: 'ready', locale: 'en', theme: 'light', viewport: { width: 1280, height: 800 } },
  { id: 'loading-ru-dark-narrow', state: 'loading', locale: 'ru', theme: 'dark', viewport: { width: 390, height: 844 } },
]
const host: ComponentHost = {
  cases: () => cases,
  readiness: { readinessAttribute: 'data-host-ready' },
  async navigate(page, c) {
    await page.setContent(`<html lang="${c.locale}" data-theme="${c.theme}"><body><main data-state="${c.state}">${c.state}</main><script>requestAnimationFrame(() => document.querySelector('main').setAttribute('data-host-ready', 'true'))</script></body></html>`)
    return page.locator('main')
  },
}

// Characterisation: falsely green if openComponentCase ignored the case viewport or host readiness attribute.
test('opens a discovered case at its viewport with host readiness', async ({ page }) => {
  const c = await componentCase(host, 'loading-ru-dark-narrow')
  const root = await openComponentCase(page, host, c)
  await expect(root).toHaveAttribute('data-state', 'loading')
  expect(page.viewportSize()).toEqual({ width: 390, height: 844 })
  await expect(componentCase(host, 'missing')).rejects.toThrow('Unknown component case missing')
})

test('coverage requires every dimension value without the Cartesian product', () => {
  expect(() => assertCaseCoverage(cases, { states: ['ready', 'loading'], locales: ['en', 'ru'], themes: ['light', 'dark'], viewports: [{ width: 390, height: 844 }] })).not.toThrow()
  expect(() => assertCaseCoverage(cases, { states: ['error'], locales: ['uz'] })).toThrow('state error, locale uz')
  expect(() => assertCaseCoverage([...cases, cases[0]], {})).toThrow('Duplicate component case')
})
