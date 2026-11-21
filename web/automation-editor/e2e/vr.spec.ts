// Visual regression (Q1): every state of the screen at 1440 and 390 (1200 for the toolbar breakpoint),
// light and dark. Linux baselines are canonical; scripts/vr.mjs bootstraps missing ones as candidates.
import { expect, test, type Page } from '@playwright/test'
import { card, open } from './helpers'

type Shot = { name: string; query: string; width: number; height: number; prepare?: (page: Page) => Promise<void> }

const shots: Shot[] = [
  { name: 'editor', query: '', width: 1440, height: 900 },
  { name: 'editor-1200', query: '', width: 1200, height: 800 },
  { name: 'editor-ru', query: 'locale=ru', width: 1440, height: 900 },
  { name: 'loading', query: 'loading=1', width: 1440, height: 900 },
  { name: 'empty', query: 'graph=empty', width: 1440, height: 900 },
  { name: 'read-only', query: 'readonly=1', width: 1440, height: 900 },
  { name: 'run-overlay', query: 'overlay=1', width: 1440, height: 900 },
  { name: 'pending-review', query: 'pending=1', width: 1440, height: 900, prepare: async (page) => {
    await card(page, 'has_email').click()
    await page.locator('.ae-panel [data-param="expression"] textarea').first().fill('true')
    await page.locator('.ae-save').click()
    await expect(page.locator('.ae-banner').filter({ hasText: 'not enabled' })).toBeVisible()
  } },
  { name: 'panel', query: '', width: 1440, height: 900, prepare: async (page) => {
    await card(page, 'send_email').click()
    await expect(page.locator('.ae-panel')).toBeVisible()
  } },
  { name: 'schedule-panel', query: '', width: 1440, height: 900, prepare: async (page) => {
    await card(page, 'schedule').click()
    await expect(page.locator('.ae-panel')).toContainText('Every day')
  } },
  { name: 'validation', query: 'errors=1', width: 1440, height: 900, prepare: async (page) => {
    await page.getByRole('button', { name: 'Validate' }).click()
    await expect(page.locator('.ae-validation')).toContainText('Unknown field')
  } },
  { name: 'connection-lost', query: 'connection=lost', width: 1440, height: 900, prepare: async (page) => {
    await page.getByRole('button', { name: 'Validate' }).click()
    await expect(page.locator('.ae-validation')).toContainText('Sales bot')
  } },
  { name: 'versions', query: '', width: 1440, height: 900, prepare: async (page) => {
    await page.getByRole('button', { name: 'Versions' }).click()
    await expect(page.locator('.ae-menu')).toBeVisible()
  } },
  { name: 'palette', query: '', width: 1440, height: 900, prepare: async (page) => {
    await page.locator('.ae-canvas').click({ position: { x: 700, y: 30 } })
    await page.keyboard.press('Control+k')
    await expect(page.locator('.ae-palette')).toBeVisible()
  } },
  { name: 'conflict', query: 'conflict=1', width: 1440, height: 900, prepare: async (page) => {
    await card(page, 'has_email').click()
    await page.locator('.ae-panel [data-param="expression"] textarea').first().fill('true')
    await page.locator('.ae-save').click()
    await expect(page.locator('.ae-banner').filter({ hasText: 'Saved by another user' })).toBeVisible()
  } },
  { name: 'phone', query: 'overlay=1', width: 390, height: 844 },
]

for (const theme of ['light', 'dark'] as const) {
  for (const shot of shots) {
    test(`${shot.name} · ${theme}`, async ({ page }) => {
      await page.setViewportSize({ width: shot.width, height: shot.height })
      // Schedule labels show the next run: the clock is fixed so the shots do not change with the date.
      await page.clock.setFixedTime(new Date('2026-11-20T09:00:00Z'))
      const query = [shot.query, theme === 'dark' ? 'theme=dark' : ''].filter(Boolean).join('&')
      if (shot.name === 'loading') {
        await page.goto(`/?${query}`)
        await expect(page.locator('.ae-skeleton')).toBeVisible()
      } else {
        await open(page, query, shot.width < 768 ? '.ae-outline' : '.ae-canvas')
      }
      await shot.prepare?.(page)
      // The canvas layers are composited (will-change) for drag performance; tiled software raster of a
      // composited layer anti-aliases edges differently run to run. Screenshots paint them in place.
      await page.addStyleTag({ content: '.ae-viewport, .ae-grid { will-change: auto !important; }' })
      await page.mouse.move(0, 0)
      // A clicked button keeps focus and its hover transition; the shot is of the resting state.
      await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur?.())
      // Web fonts settle the toolbar's sub-pixel positions; shoot only after they load.
      await page.evaluate(async () => {
        await document.fonts.ready
        await Promise.all([...document.fonts].map((font) => font.load().catch(() => undefined)))
      })
      // Let the debounced check and the layout save finish so no status is caught mid-way.
      await page.waitForTimeout(1200)
      await expect(page.locator('.ae-spinner')).toHaveCount(0)
      await expect(page).toHaveScreenshot(`${shot.name}-${theme}.png`)
    })
  }
}
