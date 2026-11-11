// Case 3: slot contributions. The CRM client view (Templ/HTMX, a full page and a drawer) and a Solid page of the
// package each declare a slot; the canonical client host mounts the contributions the viewer gets, in their own
// roots, and releases them when HTMX swaps their region out. A contribution the viewer may not have is absent from
// the server's HTML, not hidden by the browser.
import { expect, probe, signIn, state, test, type Session } from './harness'

declare global {
  interface Window { htmx: { ajax(verb: string, url: string, options: Record<string, unknown>): Promise<void> } }
}

const ann = state.clients.Ann
const clientPage = `/crm/clients/${ann}`

test.describe('slots', () => {
  let s: Session
  test.afterEach(async () => {
    expect(s.problems, 'no console errors or uncaught exceptions').toEqual([])
    await s.context.close()
  })

  test('the CRM client page (Templ) mounts the contributions of the viewer in its sidebar', async ({ browser }) => {
    s = await signIn(browser, 'auditor')
    const { page } = s
    await page.goto(clientPage)
    const sidebar = page.getByTestId('crm-client-sidebar')
    await expect(sidebar.getByTestId('client-probe')).toHaveText(`Probe for client ${ann}`) // typed props: the client of the page
    await expect(sidebar.getByTestId('client-audit')).toHaveText(`Audit for client ${ann}`)
    await expect(page.locator('script[src="/assets/runtime-host.js"]')).toHaveCount(1)
    expect((await probe(page)).live['client-probe']).toBe(1)
  })

  test('a contribution the viewer may not have is absent from the HTML and from the page', async ({ browser }) => {
    s = await signIn(browser, 'member')
    const { page } = s
    const html = await (await s.context.request.get(clientPage)).text()
    expect(html).toContain('e2e.browser.clientProbe')
    expect(html).not.toContain('e2e.browser.clientAudit') // not shipped, so not hidden by script

    await page.goto(clientPage)
    const sidebar = page.getByTestId('crm-client-sidebar')
    await expect(sidebar.getByTestId('client-probe')).toBeVisible()
    await expect(sidebar.getByTestId('client-audit')).toHaveCount(0)
    await expect(page.locator('[data-iota-slot]')).toHaveCount(1)
  })

  test('a Solid page mounts the contributions to its own slot; the audit one only for the permission holder', async ({ browser }) => {
    s = await signIn(browser, 'auditor')
    await s.page.goto('/browser/lab')
    const panel = s.page.getByTestId('lab-panel')
    await expect(panel.getByTestId('panel-note')).toHaveText('Note for Lab')
    await expect(panel.getByTestId('panel-audit')).toHaveText('Audit for Lab')
    // In the order of the contributions (10, 20).
    await expect(panel.locator('[data-testid^="panel-"]')).toHaveText(['Note for Lab', 'Audit for Lab'])
    await s.context.close()

    s = await signIn(browser, 'member')
    await s.page.goto('/browser/lab')
    await expect(s.page.getByTestId('lab-panel').getByTestId('panel-note')).toBeVisible()
    await expect(s.page.getByTestId('lab-panel').getByTestId('panel-audit')).toHaveCount(0)
    expect(await s.page.content()).not.toContain('e2e.browser.labAudit') // the page data never carried it
  })

  test('an HTMX swap of the slot\'s region disposes the contribution; a swap that brings the slot back mounts it once', async ({ browser }) => {
    s = await signIn(browser, 'auditor')
    const { page } = s
    await page.goto(clientPage)
    await expect(page.getByTestId('client-probe')).toBeVisible()
    await page.evaluate(() => window.dispatchEvent(new Event('e2e-ping')))
    expect((await probe(page)).pings['client-probe']).toBe(1)

    // A real HTMX swap: the region is replaced by the page's own heading.
    const swap = (select: string) => page.evaluate(
      ([url, pick]) => window.htmx.ajax('GET', url, { target: '[data-testid="crm-client-sidebar"]', swap: 'innerHTML', select: pick }),
      [clientPage, select] as const,
    )
    await swap('h1')
    await expect(page.getByTestId('crm-client-sidebar').getByTestId('client-probe')).toHaveCount(0)
    await expect.poll(async () => (await probe(page)).live['client-probe']).toBe(0)
    await page.evaluate(() => window.dispatchEvent(new Event('e2e-ping')))
    const gone = await probe(page)
    expect(gone.pings['client-probe']).toBe(1) // the ping after the swap reached no listener
    expect(gone.listeners).toBe(0)

    // The swap that brings a slot element back: one new root, not two.
    await swap('[data-iota-slot]')
    await expect(page.getByTestId('crm-client-sidebar').getByTestId('client-probe')).toHaveCount(1)
    await expect.poll(async () => (await probe(page)).live['client-probe']).toBe(1)
    const back = await probe(page)
    expect(back.log.filter((e) => e.endsWith(':client-probe'))).toEqual(['mount:client-probe', 'dispose:client-probe', 'mount:client-probe'])
    expect(back.listeners).toBe(1)
  })

  test('the client drawer (an HTMX fragment) mounts the slot, and closing it disposes the contribution', async ({ browser }) => {
    s = await signIn(browser, 'auditor')
    const { page } = s
    await page.goto('/crm/clients')
    expect((await probe(page)).live['client-probe'] ?? 0).toBe(0)
    await page.locator('#clients-table-body').getByRole('button', { name: /Ann Doe/ }).click()
    const drawer = page.locator('#view-drawer')
    await expect(drawer.getByTestId('client-probe')).toHaveText(`Probe for client ${ann}`)
    await expect.poll(async () => (await probe(page)).live['client-probe']).toBe(1)

    await drawer.locator('dialog[open] form[method="dialog"] button').first().click() // the drawer's close button
    await expect(drawer).toHaveCount(0) // the page removes the element when the drawer has closed
    await expect.poll(async () => (await probe(page)).live['client-probe']).toBe(0)
    expect((await probe(page)).listeners).toBe(0)

    await page.locator('#clients-table-body').getByRole('button', { name: /Ann Doe/ }).click() // and opening again mounts once more
    await expect(page.locator('#view-drawer').getByTestId('client-probe')).toBeVisible()
    await expect.poll(async () => (await probe(page)).live['client-probe']).toBe(1)
    expect((await probe(page)).log.filter((e) => e.endsWith(':client-probe'))).toEqual(['mount:client-probe', 'dispose:client-probe', 'mount:client-probe'])
  })
})
