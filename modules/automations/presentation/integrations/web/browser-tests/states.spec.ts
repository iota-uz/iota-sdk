import { expect, test, type Page } from '@playwright/test'
import { expectNoA11yViolations } from '../../../../../../web/testing/src/a11y'

/**
 * A1/Q1/S1: every state of the screen at 1440 and 390 px, light and dark: no serious or critical axe
 * violation, and a screenshot against the Linux baseline (browser-tests/__screenshots__).
 */

// Finite animations (a drawer sliding in) finish; infinite ones (a skeleton's pulse) are frozen by the
// screenshot's animations: 'disabled'.
const settle = (page: Page) => page.evaluate(() => Promise.all(document.getAnimations()
  .filter((a) => a.effect?.getComputedTiming().iterations !== Infinity)
  .map((a) => a.finished.catch(() => undefined))))

// scope: the open modal layer, when the page behind is inert (its own state is checked as «list»;
// axe's contrast pass over a whole page under a modal backdrop does not finish).
interface State { name: string; query: string; scope?: string; open?: (page: Page) => Promise<void>; ready: (page: Page) => Promise<void> }

const loaded = async (page: Page) => { await expect(page.getByTestId('connector-telegram_bot')).toBeVisible() }
const states: State[] = [
  { name: 'list', query: '', ready: loaded },
  { name: 'empty', query: 'scenario=empty', ready: async (p) => { await expect(p.getByTestId('integrations-empty')).toBeVisible() } },
  { name: 'error', query: 'scenario=error', ready: async (p) => { await expect(p.getByTestId('integrations-load-error')).toBeVisible() } },
  { name: 'loading', query: 'scenario=loading', ready: async (p) => { await expect(p.getByTestId('integrations-loading')).toBeVisible() } },
  { name: 'unconfigured', query: 'unconfigured', ready: loaded },
  { name: 'form', query: 'connect=telegram_bot', ready: async (p) => { await expect(p.getByTestId('form-save')).toBeVisible() } },
  {
    name: 'form-refused', query: 'connect=telegram_bot',
    open: async (p) => {
      const f = p.getByTestId('connection-form')
      await f.locator('[data-field="display_name"]').fill('Sales bot 2')
      await f.locator('[data-field="bot_token"]').fill('bad')
      await f.getByTestId('form-save').click()
    },
    ready: async (p) => { await expect(p.getByTestId('form-error')).toBeVisible() },
  },
  { name: 'form-edit', query: 'edit=00000000-0000-4000-8000-000000000001', ready: async (p) => { await expect(p.getByTestId('secret-set-bot_token')).toBeVisible() } },
  {
    name: 'usage', query: '',
    open: async (p) => { await loaded(p); await p.getByTestId('connection-sales_bot').getByTestId('used-by').click() },
    ready: async (p) => { await expect(p.getByTestId('usage-list')).toBeVisible() },
  },
  {
    name: 'delete', query: '', scope: '[data-testid="delete-dialog"]',
    open: async (p) => {
      await loaded(p)
      await p.getByTestId('connection-support_bot').locator('summary[aria-haspopup]').click()
      await p.getByTestId('connection-support_bot').getByTestId('menu-delete').click()
    },
    ready: async (p) => {
      await expect(p.getByTestId('delete-dialog')).toBeVisible()
      // The confirmation opens with a transition axe must not run into.
      await p.waitForFunction(() => {
        const d = document.querySelector<HTMLElement>('[data-testid="delete-dialog"]')
        return Boolean(d) && getComputedStyle(d!).opacity === '1'
      })
      await p.waitForTimeout(300)
    },
  },
]

for (const width of [1440, 390]) {
  for (const theme of ['light', 'dark'] as const) {
    for (const s of states) {
      test(`${s.name} ${width} ${theme}`, async ({ page }) => {
        await page.setViewportSize({ width, height: 900 })
        await page.emulateMedia({ reducedMotion: 'reduce' })
        await page.goto(`/?theme=${theme}${s.query ? `&${s.query}` : ''}`)
        await s.open?.(page)
        await s.ready(page)
        await page.evaluate(() => document.fonts.ready)
        await settle(page)
        await expectNoA11yViolations(page, s.scope ? { include: [s.scope] } : {})
        // A focus ring fades in; focus is the keyboard tests' business, the baseline is the layout.
        await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur())
        await page.waitForTimeout(250)
        await expect(page).toHaveScreenshot(`${s.name}-${width}-${theme}.png`, { fullPage: true })
      })
    }
  }
}
