// Case 1: a runtime Solid page with a navigation entry, on the canonical client host and the host bundle served by
// the application: it mounts, reads through useQuery, writes through useCommand, decodes typed route parameters,
// is disposed when the document goes away (or the route is unmounted) and mounts exactly once per visit.
import { expect, persistedLog, probe, signIn, test, type Session } from './harness'

const lifecycle = (log: string[]) => log.filter((e) => /:(lab|item)$/.test(e))

test.describe('runtime page', () => {
  let s: Session
  test.beforeEach(async ({ browser }) => { s = await signIn(browser, 'member') })
  test.afterEach(async () => {
    expect(s.problems, 'no console errors or uncaught exceptions').toEqual([])
    await s.context.close()
  })

  test('mounts, useQuery renders data and useCommand writes through /_ext/rpc', async () => {
    const { page } = s
    await page.goto('/browser/lab')
    const lab = page.getByTestId('lab')
    await expect(lab.getByRole('heading', { name: 'Browser lab' })).toBeVisible()
    await expect(page.locator('#iota-client-route-mount')).not.toHaveAttribute('aria-busy', 'true')
    await expect(page.getByTestId('lab-hits')).toBeAttached() // the query answered (an empty list is still a list)
    const label = `hit-${Date.now()}`
    const command = page.waitForResponse((r) => r.url().endsWith('/_ext/rpc') && (r.request().postData() ?? '').includes('e2e.browser.recordHit'))
    await page.getByTestId('lab-label').fill(label)
    await page.getByTestId('lab-record').click()
    expect((await command).status()).toBe(200)
    await expect(page.getByTestId('lab-hits').getByText(label, { exact: true })).toBeVisible() // the UI updated from the refetch
    // The write is on the server: a second document sees it.
    await page.reload()
    await expect(page.getByTestId('lab-hits').getByText(label, { exact: true })).toBeVisible()
    expect((await probe(page)).live.lab).toBe(1)
  })

  test('the navigation entry of the package is in the standard shell and opens the page', async () => {
    const { page } = s
    await page.goto('/crm/clients')
    // The server projects the package's page into the sidebar (the drawer and the desktop one, collapsed group);
    // the link a user opens is a plain anchor to the page.
    const entry = page.locator('a[href="/browser/lab"]').last()
    await expect(entry).toHaveText(/Browser lab/)
    await entry.evaluate((a) => (a as HTMLAnchorElement).click())
    await expect(page).toHaveURL(/\/browser\/lab$/)
    await expect(page.getByTestId('lab')).toBeVisible()
  })

  test('typed route parameters reach the page decoded', async () => {
    const { page } = s
    await page.goto('/browser/lab')
    await page.getByTestId('lab-item-link').click()
    await expect(page).toHaveURL(/\/browser\/items\/7$/)
    await expect(page.getByTestId('item-id')).toHaveText('Item 7')
    await expect(page.getByTestId('item-type')).toHaveText('number') // int, not the string "7" of the path
    // A value the route's type refuses never reaches a page: the server answers 404 before any script is sent.
    const refused = await s.context.request.get('/browser/items/abc')
    expect(refused.status()).toBe(404)
    expect(await refused.text()).not.toContain('iota-client-context')
  })

  test('navigating away disposes the page; back and forward mount it once per visit', async () => {
    const { page } = s
    await page.goto('/browser/lab')
    await expect(page.getByTestId('lab')).toBeVisible()
    await expect.poll(async () => (await probe(page)).live).toMatchObject({ lab: 1, 'lab-note': 1 })
    expect((await probe(page)).listeners).toBe(2) // the page and its slot contribution each hold one listener
    await page.getByTestId('lab-item-link').click()
    await expect(page.getByTestId('item')).toBeVisible()
    // The lab was disposed when its document went away (its cleanup ran before the unload).
    expect(lifecycle(await persistedLog(page))).toEqual(['mount:lab', 'dispose:lab', 'mount:item'])
    expect((await probe(page)).live).toEqual({ item: 1 })

    await page.goBack()
    await expect(page.getByTestId('lab')).toBeVisible()
    await expect(page.getByTestId('lab')).toHaveCount(1) // one root, not two
    await expect.poll(async () => (await probe(page)).live.lab).toBe(1)
    expect((await probe(page)).log.filter((e) => e === 'mount:lab')).toHaveLength(1)
    expect(lifecycle(await persistedLog(page))).toEqual(['mount:lab', 'dispose:lab', 'mount:item', 'dispose:item', 'mount:lab'])

    await page.goForward()
    await expect(page.getByTestId('item')).toBeVisible()
    await expect(page.getByTestId('item')).toHaveCount(1)
    expect(lifecycle(await persistedLog(page))).toEqual([
      'mount:lab', 'dispose:lab', 'mount:item', 'dispose:item', 'mount:lab', 'dispose:lab', 'mount:item',
    ])
  })

  test('unmounting the client route releases the root, its slot contributions and every listener', async () => {
    const { page } = s
    await page.goto('/browser/lab')
    await expect(page.getByTestId('panel-note')).toBeVisible() // the slot contribution of the package
    await page.evaluate(() => window.dispatchEvent(new Event('e2e-ping')))
    const before = await probe(page)
    expect(before.pings).toEqual({ lab: 1, 'lab-note': 1 })

    // A router that leaves the route without a document change announces it on the document.
    await page.evaluate(() => document.dispatchEvent(new Event('iota:client-route-unmount')))
    await expect(page.getByTestId('lab')).toHaveCount(0)
    await expect(page.locator('#iota-client-route-mount')).toBeEmpty()
    await expect(page.locator('#iota-client-route-mount')).not.toHaveAttribute('data-iota-client-owner', /.+/)
    const after = await probe(page)
    expect(after.live).toEqual({ lab: 0, 'lab-note': 0 })
    expect(after.listeners).toBe(0)
    // A listener that leaked would still answer.
    await page.evaluate(() => window.dispatchEvent(new Event('e2e-ping')))
    expect((await probe(page)).pings).toEqual(before.pings)
  })

  test('a user without the page permission does not get the page', async ({ browser }) => {
    const viewer = await signIn(browser, 'viewer')
    try {
      const response = await viewer.page.goto('/browser/lab')
      expect(response?.status()).toBeGreaterThanOrEqual(400)
      await expect(viewer.page.getByTestId('lab')).toHaveCount(0)
      await viewer.page.goto('/crm/clients')
      await expect(viewer.page.locator('a[href="/browser/lab"]')).toHaveCount(0) // nor the menu entry
    } finally {
      await viewer.context.close()
    }
  })
})
