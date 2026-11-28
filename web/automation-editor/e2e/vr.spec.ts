// Visual regression (Q1): every state of the screen at 1440 and 390 (1200 for the toolbar breakpoint),
// light and dark. Linux baselines are canonical; scripts/vr.mjs bootstraps missing ones as candidates.
import { expect, test, type Page } from '@playwright/test'
import { card, open, settled } from './helpers'

type Shot = { name: string; query: string; width: number; height: number; ready?: string; prepare?: (page: Page) => Promise<void> }

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
    // Save keeps its pressed and busy styles a moment after the answer: the shot waits until they settle.
    await page.mouse.move(0, 0)
    await page.locator('.ae-save').blur()
    await expect(page.locator('.ae-save')).not.toHaveAttribute('aria-busy', 'true')
    await page.waitForTimeout(400)
  } },
  { name: 'phone', query: 'overlay=1', width: 390, height: 844 },
  // Tidy v2 (#1391): a column per trigger, a failure lane, orthogonal links; a parallel with its join and a for each.
  { name: 'tidy-multi', query: 'graph=multi', width: 1440, height: 900 },
  { name: 'tidy-foreach', query: 'graph=foreach', width: 1440, height: 900 },
  // Router: a card touching the source row off the source column does not close the source's lanes; the edge goes around it.
  { name: 'router-offcolumn', query: 'graph=offcolumn', width: 1440, height: 900 },
  // Runs (#1347): a failed run with its node's drawer, a run of an older version, a waiting parallel,
  // the empty and filtered states, a run not found, the phone list and run, Dry run and Test node.
  { name: 'runs', query: 'tab=runs&run=00000000-0000-4000-8000-0000000000r2&node=send_email', width: 1440, height: 900, ready: '.ae-runview .ae-canvas', prepare: async (page) => { await expect(page.getByTestId('run-node')).toBeVisible() } },
  { name: 'runs-ru', query: 'locale=ru&tab=runs&run=00000000-0000-4000-8000-0000000000r2&node=send_email', width: 1440, height: 900, ready: '.ae-runview .ae-canvas', prepare: async (page) => { await expect(page.getByTestId('run-node')).toBeVisible() } },
  { name: 'run-old-version', query: 'tab=runs&run=00000000-0000-4000-8000-0000000000r3', width: 1440, height: 900, ready: '.ae-runview .ae-canvas', prepare: async (page) => { await expect(page.getByTestId('run-version-banner')).toBeVisible() } },
  { name: 'run-blocked', query: 'tab=runs&run=00000000-0000-4000-8000-0000000000r4', width: 1440, height: 900, ready: '.ae-runview .ae-canvas', prepare: async (page) => { await expect(page.getByTestId('run-resolve')).toBeVisible() } },
  { name: 'run-parallel', query: 'graph=approval&tab=runs&run=00000000-0000-4000-8000-00000000a001&node=answer', width: 1440, height: 900, ready: '.ae-runview .ae-canvas', prepare: async (page) => { await expect(page.getByTestId('run-node')).toBeVisible() } },
  { name: 'runs-empty', query: 'tab=runs&runs=empty&manual=1', width: 1440, height: 900, ready: '[data-testid="runs-empty"]' },
  { name: 'runs-filtered', query: 'tab=runs&state=blocked&subject=zz', width: 1440, height: 900, ready: '[data-testid="runs-empty-filter"]' },
  { name: 'runs-error', query: 'tab=runs&runs=error', width: 1440, height: 900, ready: '[data-testid="runs-error"]' },
  { name: 'run-not-found', query: 'tab=runs&run=00000000-0000-4000-8000-0000000000r2&runs=notfound', width: 1440, height: 900, ready: '[data-testid="run-not-found"]' },
  { name: 'runs-phone', query: 'tab=runs', width: 390, height: 844, ready: '.ae-runlist__rows[role="listbox"]' },
  { name: 'run-phone', query: 'tab=runs&run=00000000-0000-4000-8000-0000000000r2&node=send_email', width: 390, height: 844, ready: '.ae-runview .ae-outline', prepare: async (page) => { await expect(page.getByTestId('run-node')).toBeVisible() } },
  { name: 'dry-run', query: '', width: 1440, height: 900, prepare: async (page) => {
    await page.getByTestId('open-dry-run').click()
    // A dispatched click leaves no pressed or hovered state on the button the shot would catch.
    await page.getByTestId('dry-run-start').dispatchEvent('click')
    await expect(page.getByTestId('dry-effects')).toBeVisible()
  } },
  { name: 'test-node', query: 'test=secret', width: 1440, height: 900, prepare: async (page) => {
    await card(page, 'send_email').click()
    await page.getByTestId('open-test-node').click()
    await page.getByTestId('test-start').click()
    await page.getByTestId('test-confirm').getByRole('button', { name: 'Run for real' }).click()
    await expect(page.getByTestId('test-error')).toBeVisible()
    // The dialog gives the focus back to «Test» once it closes: the shot blurs it after that.
    await expect(page.getByTestId('test-start')).toBeFocused()
    await expect(page.locator('dialog[open]')).toHaveCount(0)
    await page.waitForTimeout(500)
  } },
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
        await open(page, query, shot.ready ?? (shot.width < 768 ? '.ae-outline' : '.ae-canvas'))
      }
      await shot.prepare?.(page)
      // The canvas layers are composited (will-change) for drag performance; tiled software raster of a
      // composited layer anti-aliases edges differently run to run. Screenshots paint them in place.
      await page.addStyleTag({ content: '.ae-viewport, .ae-grid { will-change: auto !important; }' })
      // The path a run took lights up with a finite opacity animation on its edges and arrowheads. A layer that
      // has just stopped animating is re-rasterised on a later frame, and an arrowhead or a line pixel lands on
      // either side of an anti-aliasing step from one CI run to the next. The settled look is the shot, so
      // the entrance is dropped: the elements are already at their resting opacity.
      await page.addStyleTag({ content: '.ae-edge--taken, .ae-arrowhead--taken, .ae-edge--flash, .ae-arrowhead--flash { animation: none !important; }' })
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
      // Entrances (#1352: the run path lights up step by step, badges and layers fade in) finish on their own
      // before the shot: a frame taken while one still runs composites, and so rasterises text, differently.
      await settled(page)
      await expect(page).toHaveScreenshot(`${shot.name}-${theme}.png`)
    })
  }
}
