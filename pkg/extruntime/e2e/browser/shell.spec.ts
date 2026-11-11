// Case 2: the whole-shell renderer. A package binds to the shell slot and wraps every authenticated page: a legacy
// Templ/HTMX page (its HTMX swaps, scripts and modals keep working inside the host-owned outlet) and a Solid
// runtime page. `?iota-shell=standard` and closing the binding give the standard shell back; theme and locale
// are the user's, reflected by the renderer.
import { bindShell, expect, setLanguage, signIn, test, unbindShell, type Session } from './harness'

const shell = (s: Session) => s.page.getByTestId('lab-shell')
const standard = (s: Session) => s.page.locator('[data-iota-shell-standard]')

test.describe.configure({ mode: 'serial' })

test.describe('shell renderer', () => {
  let s: Session
  test.beforeAll(async () => { await unbindShell() })
  test.afterAll(async () => {
    await unbindShell()
    await setLanguage('auditor', 'en')
  })
  test.beforeEach(async ({ browser }) => { s = await signIn(browser, 'auditor') })
  test.afterEach(async () => {
    expect(s.problems, 'no console errors or uncaught exceptions').toEqual([])
    await s.context.close()
  })

  test('without a binding the standard shell serves and a legacy page without islands does not load the runtime host', async () => {
    const { page } = s
    await page.goto('/account')
    await expect(standard(s)).toBeVisible()
    await expect(shell(s)).toHaveCount(0)
    await expect(page.locator('#iota-runtime-context')).toHaveCount(0)
    await expect(page.locator('script[src="/assets/runtime-host.js"]')).toHaveCount(0)
    await page.goto('/crm/clients') // the list carries cell islands, so the host loads for them alone
    await expect(standard(s)).toBeVisible()
    await expect(page.locator('#iota-runtime-context')).toHaveCount(0)
    await expect(page.locator('script[src="/assets/runtime-host.js"]')).toHaveCount(1)
  })

  test('a bound renderer wraps a legacy page: HTMX swap, page script and modal keep working in the outlet', async () => {
    await bindShell()
    const { page } = s
    await page.goto('/crm/clients')
    await expect(shell(s)).toBeVisible()
    await expect(standard(s)).toBeHidden()
    await expect(page.getByTestId('lab-shell-user')).toHaveText('auditor Tester')
    // The outlet is the very element of the legacy page, moved into the renderer's layout.
    const outlet = shell(s).locator('main [data-iota-shell-outlet]')
    await expect(outlet).toHaveCount(1)
    await expect(outlet.getByRole('heading', { name: /clients/i })).toBeVisible()
    // Navigation and palette are the server's model: the same entries a standard sidebar shows.
    await expect(page.getByTestId('lab-shell-nav').locator('section a[href="/crm/clients"]')).toHaveCount(1)
    await expect(page.getByTestId('lab-shell-nav').locator('section a[href="/browser/lab"]')).toHaveCount(1)
    await expect(page.getByTestId('lab-shell-palette')).toHaveAttribute('href', '/browser/lab')

    // An HTMX swap inside the outlet: the filter form refreshes the table body.
    const body = outlet.locator('#clients-table-body')
    await expect(body.getByText('Ann Doe')).toBeVisible()
    await expect(body.getByText('Bob Doe')).toBeVisible()
    const swapped = page.waitForResponse((r) => r.url().includes('/crm/clients?') && r.request().headers()['hx-request'] === 'true')
    await outlet.locator('input[name="Search"]').pressSequentially('Ann') // the form listens to keyup
    await swapped
    await expect(body.getByText('Bob Doe')).toHaveCount(0)
    await expect(body.getByText('Ann Doe')).toBeVisible()

    // A page script (Alpine) still runs on what moved: the relative date of a row is computed in the browser.
    await expect(body.locator('[x-data="relativeformat"] span').first()).not.toBeEmpty()

    // A modal still opens: the client drawer is fetched by HTMX and appended to the document body.
    await body.getByRole('button', { name: /Ann Doe/ }).click()
    const drawer = page.locator('#view-drawer dialog[open]')
    await expect(drawer).toBeVisible()
    await expect(drawer).toContainText('Details about the client')
    await expect(page).toHaveURL(/view=/) // hx-push-url: the history entry the drawer closes back from
  })

  test('a bound renderer wraps a Solid runtime page in the same outlet', async () => {
    await bindShell()
    const { page } = s
    await page.goto('/browser/lab')
    await expect(shell(s)).toBeVisible()
    await expect(standard(s)).toBeHidden()
    const lab = shell(s).locator('main').getByTestId('lab')
    await expect(lab).toBeVisible() // the page mounted by the client host inside the outlet
    await expect(lab.getByTestId('lab-hits')).toBeAttached() // useQuery works under the renderer
    const label = `shell-hit-${Date.now()}`
    await lab.getByTestId('lab-label').fill(label)
    await lab.getByTestId('lab-record').click()
    await expect(lab.getByTestId('lab-hits').getByText(label, { exact: true })).toBeVisible() // useCommand too
    await expect(page.locator('script#iota-runtime-context')).toHaveCount(1)
    await expect(page.getByTestId('lab-shell-nav').locator('section a[href="/browser/lab"]')).toHaveAttribute('data-active', 'true')
  })

  test('?iota-shell=standard brings the standard shell back for a legacy and a runtime page', async () => {
    await bindShell()
    const { page } = s
    for (const path of ['/crm/clients', '/browser/lab']) {
      await page.goto(`${path}?iota-shell=standard`)
      await expect(standard(s)).toBeVisible()
      await expect(shell(s)).toHaveCount(0)
      await expect(page.locator('[data-iota-shell-outlet]')).toBeVisible()
    }
    await expect(page.getByTestId('lab')).toBeVisible() // the Solid page needs no package shell
    await page.goto('/crm/clients') // and without the override the renderer is back
    await expect(shell(s)).toBeVisible()
  })

  test('closing the binding restores the standard shell on the next load; the legacy page needs no host again', async () => {
    await bindShell()
    const { page } = s
    await page.goto('/crm/clients')
    await expect(shell(s)).toBeVisible()
    await unbindShell()
    await page.reload()
    await expect(standard(s)).toBeVisible()
    await expect(shell(s)).toHaveCount(0)
    await expect(page.locator('#iota-runtime-context')).toHaveCount(0)
    await page.goto('/account')
    await expect(page.locator('script[src="/assets/runtime-host.js"]')).toHaveCount(0)
    await page.goto('/browser/lab')
    await expect(standard(s)).toBeVisible()
    await expect(page.getByTestId('lab')).toBeVisible()
  })

  test('the theme is the document\'s: the renderer reads it, changes it, and the legacy content follows', async () => {
    const { page } = s
    await page.emulateMedia({ colorScheme: 'dark' })
    await bindShell()
    await page.goto('/crm/clients')
    // The theme script of the standard layout resolved "system" to dark; the renderer starts from it.
    await expect(page.locator('html')).toHaveClass(/dark/)
    await expect(page.getByTestId('lab-shell-theme')).toHaveText('dark')
    await expect(shell(s)).toHaveAttribute('data-theme-resolved', 'dark')

    await page.getByTestId('lab-shell-light').click()
    await expect(page.getByTestId('lab-shell-theme')).toHaveText('light')
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
    // Tailwind's dark variants follow the class of <html>: the legacy page content changes with the renderer's choice.
    await expect(page.locator('html')).toHaveClass(/light/)
    await expect(page.locator('html')).not.toHaveClass(/dark/)
    // The choice is the user's: it survives a navigation, in the renderer and in the standard shell.
    await page.goto('/browser/lab')
    await expect(page.getByTestId('lab-shell-theme')).toHaveText('light')
    await page.goto('/crm/clients?iota-shell=standard')
    await expect(page.locator('html')).toHaveClass(/light/)

    await page.goto('/crm/clients')
    await page.getByTestId('lab-shell-dark').click()
    await expect(page.locator('html')).toHaveClass(/dark/)
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  })

  test('the locale is the user\'s: labels of the renderer and the document language follow a language switch', async () => {
    await bindShell()
    const { page } = s
    await page.goto('/browser/lab')
    await expect(page.locator('html')).toHaveAttribute('lang', 'en')
    await expect(page.getByTestId('lab-shell-locale')).toHaveText('en')
    await expect(page.getByTestId('lab-shell-nav').locator('section a[href="/browser/lab"]')).toHaveText('Browser lab')
    await expect(page.getByTestId('lab-shell-palette')).toHaveText('Open the lab')

    await setLanguage('auditor', 'ru')
    await page.reload()
    await expect(page.locator('html')).toHaveAttribute('lang', 'ru')
    await expect(page.getByTestId('lab-shell-locale')).toHaveText('ru')
    await expect(page.getByTestId('lab-shell-nav').locator('section a[href="/browser/lab"]')).toHaveText('Лаборатория')
    await expect(page.getByTestId('lab-shell-palette')).toHaveText('Открыть лабораторию')
    // A legacy page under the same renderer carries the same language.
    await page.goto('/crm/clients')
    await expect(page.getByTestId('lab-shell-locale')).toHaveText('ru')

    await setLanguage('auditor', 'en')
    await page.reload()
    await expect(page.getByTestId('lab-shell-locale')).toHaveText('en')
    await expect(page.getByTestId('lab-shell-palette')).toHaveText('Open the lab')
  })
})
