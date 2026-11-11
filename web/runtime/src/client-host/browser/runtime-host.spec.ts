// @playwright/test is installed with web/client-host; load that very copy so the runner and the spec share it.
import playwright from '../../../../client-host/node_modules/@playwright/test/index.js'

const { expect, test } = playwright

test.describe('package page on the canonical client host', () => {
  test('mounts a runtime page with typed params and a server query through /_ext/rpc', async ({ page }) => {
    await page.goto('/page.html')
    const screen = page.getByTestId('policy-screen')
    await expect(screen.getByRole('heading')).toHaveText('Policy 7') // route param decoded to a number
    await expect(page.getByTestId('expiring')).toHaveText('3 expiring') // useQuery over POST /_ext/rpc
    await expect(page.locator('#iota-client-route-mount')).not.toHaveAttribute('aria-busy', 'true')
    await expect(page.locator('#standard-sidebar')).toBeVisible() // no shell binding: the standard shell stays
  })

  test('shows a bundle failure with Retry when the page chunk is missing', async ({ page }) => {
    await page.goto('/page-broken.html')
    await expect(page.locator('[data-runtime-page-error]')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Retry' })).toBeVisible()
  })
})

test.describe('whole-shell renderer', () => {
  test('wraps a legacy Templ page and keeps its DOM identity and handlers', async ({ page }) => {
    await page.goto('/legacy-shell.html')
    const shell = page.getByTestId('package-shell')
    await expect(shell).toBeVisible()
    await expect(page.locator('#standard')).toBeHidden()
    await expect(shell.locator('#outlet')).toBeVisible() // the very same outlet element, moved
    await expect(page.getByTestId('package-header')).toContainText('Ann Doe')
    await expect(shell.getByRole('link', { name: 'Clients' })).toHaveAttribute('href', '/crm/clients') // server-projected navigation
    await expect(page.getByTestId('palette')).toContainText('Go to policies')
    // the legacy page still works inside the new shell
    await page.locator('#legacy-button').click()
    await expect(page.locator('#legacy-button')).toHaveText('clicked 1')
  })

  test('wraps a runtime page in the same outlet', async ({ page }) => {
    await page.goto('/page-shell.html')
    const shell = page.getByTestId('package-shell')
    await expect(shell.getByTestId('policy-screen')).toBeVisible()
    await expect(shell.getByTestId('expiring')).toHaveText('3 expiring')
    await expect(page.locator('#standard')).toBeHidden()
  })

  test('?iota-shell=standard brings the standard shell back', async ({ page }) => {
    await page.goto('/legacy-shell.html?iota-shell=standard')
    await expect(page.locator('#standard-sidebar')).toBeVisible()
    await expect(page.getByTestId('package-shell')).toHaveCount(0)
  })

  test('without a shell binding the legacy page keeps the standard shell', async ({ page }) => {
    await page.goto('/legacy.html')
    await expect(page.locator('#standard-sidebar')).toBeVisible()
    await expect(page.getByTestId('package-shell')).toHaveCount(0)
  })
})
