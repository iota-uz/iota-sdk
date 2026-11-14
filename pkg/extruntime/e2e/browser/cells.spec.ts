// Case 4: a package cell renderer on the native CRM clients list. The list stays server-rendered: the standard cell
// is the content of the mount element, the canonical client host replaces it with the package component in the
// browser, and a page that never runs script (or whose renderer cannot load) still shows the standard value.
import { expect, signIn, test, type Session } from './harness'

declare global {
  interface Window { htmx: { ajax(verb: string, url: string, options: Record<string, unknown>): Promise<void> } }
}

const TIER_FILTER = 'ext.e2e.clientextra.tier'

/** The row of a client of the list (rows are in the server's order; the specs name them). */
const row = (s: Session, first: string) => s.page.locator('#clients-table-body tr', { hasText: `${first} Doe` })

test.describe('cell renderer islands', () => {
  let s: Session
  test.afterEach(async () => {
    expect(s.problems, 'no console errors or uncaught exceptions').toEqual([])
    await s.context.close()
  })

  test('the renderer of the package replaces the standard cell in the browser', async ({ browser }) => {
    s = await signIn(browser, 'auditor')
    const { page } = s
    await page.goto('/crm/clients')
    const cells = page.locator('#clients-table-body [data-iota-cell]')
    await expect(cells).toHaveCount(2)
    await expect(row(s, 'Ann').getByTestId('tier-cell')).toHaveText('GOLD') // the package component
    await expect(row(s, 'Bob').getByTestId('tier-cell')).toHaveText('STANDARD')
    // Each island keeps one output child, and the standard cell it replaced is gone from it.
    await expect(page.locator('[data-iota-cell] > [data-iota-island-output]')).toHaveCount(2)
    await expect(row(s, 'Ann').locator('[data-iota-cell]')).not.toHaveText(/gold/) // case-sensitive: the standard value is gone
    await expect(page.locator('script[src="/assets/runtime-host.js"]')).toHaveCount(1)
  })

  test('without JavaScript the standard value is still in the page', async ({ browser }) => {
    s = await signIn(browser, 'auditor', { javaScriptEnabled: false })
    const { page } = s
    await page.goto('/crm/clients')
    const cells = page.locator('#clients-table-body [data-iota-cell]')
    await expect(cells).toHaveCount(2)
    await expect(row(s, 'Ann').locator('[data-iota-cell]')).toHaveText('gold') // the server-rendered cells, readable as they are
    await expect(row(s, 'Bob').locator('[data-iota-cell]')).toHaveText('standard')
    await expect(page.getByTestId('tier-cell')).toHaveCount(0)
    await expect(page.locator('[data-iota-island-output]')).toHaveCount(0)
  })

  test('rows that arrive by an HTMX swap get their islands; the rows swapped out lose theirs', async ({ browser }) => {
    s = await signIn(browser, 'auditor')
    const { page } = s
    await page.goto('/crm/clients')
    await expect(page.getByTestId('tier-cell')).toHaveCount(2)
    const swap = (query: string) => page.evaluate(
      (url) => window.htmx.ajax('GET', url, { target: '#clients-table-body', swap: 'innerHTML' }),
      `/crm/clients${query}`,
    )
    await swap(`?${TIER_FILTER}=gold`)
    await expect(page.locator('#clients-table-body [data-iota-cell]')).toHaveCount(1)
    await expect(page.getByTestId('tier-cell')).toHaveText(['GOLD'])
    await expect(page.locator('[data-iota-island-output]')).toHaveCount(1) // the island of the removed row is gone
    await swap('')
    await expect(row(s, 'Ann').getByTestId('tier-cell')).toHaveText('GOLD')
    await expect(row(s, 'Bob').getByTestId('tier-cell')).toHaveText('STANDARD')
    await expect(page.locator('[data-iota-island-output]')).toHaveCount(2)
  })

  test('a renderer that cannot be loaded leaves the standard cell', async ({ browser }) => {
    s = await signIn(browser, 'auditor')
    const { page } = s
    await page.route('**/_ext/assets/**', (route) => route.fulfill({ status: 404, body: 'gone' }))
    await page.goto('/crm/clients')
    const cells = page.locator('#clients-table-body [data-iota-cell]')
    await expect(row(s, 'Ann').locator('[data-iota-cell]')).toHaveText('gold')
    await expect(row(s, 'Bob').locator('[data-iota-cell]')).toHaveText('standard')
    await expect(cells).toHaveCount(2)
    await expect(page.getByTestId('tier-cell')).toHaveCount(0)
    // The renderer was requested and refused: the browser reports the 404, nothing else broke.
    await expect.poll(() => s.problems.filter((p) => /status of 404/.test(p)).length).toBeGreaterThan(0)
    s.problems.splice(0)
  })
})
