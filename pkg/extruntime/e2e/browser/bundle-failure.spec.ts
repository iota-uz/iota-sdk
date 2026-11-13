// Case 5: the host's states while a page's client code loads and when it cannot. The page module (the package's
// client entry) and the page's own lazy component chunk are both fetched from the immutable asset route; either
// answering 404 must show the host's error state with Retry, never an empty page, and Retry starts from a
// fresh document (a rejected dynamic import stays rejected in the browser's module map) without repeating anything
// else. The standard shell stays around the failure.
import { expect, signIn, test, type Page, type Request, type Session } from './harness'

// The scripts of the package, not its stylesheets (a version's stylesheets are linked in the page HTML and block its first paint).
const ASSETS = '**/_ext/assets/**/*.js'
const MOUNT = '#iota-client-route-mount'
const HOST_FAILURE = /Runtime page e2e\.browser\.fragile: The screen could not be loaded\./

/** The chunk of the package's client graph that holds the text (the lazy component of a page), found by reading the assets. */
async function chunkContaining(session: Session, needle: string): Promise<string> {
  const urls: string[] = []
  const page = await session.context.newPage()
  page.on('response', (r) => { if (r.url().includes('/_ext/assets/')) urls.push(r.url()) })
  await page.goto('/browser/fragile')
  await expect(page.getByTestId('fragile')).toBeVisible()
  await page.close()
  for (const url of urls) {
    if ((await (await session.context.request.get(url)).text()).includes(needle)) return url
  }
  throw new Error(`no chunk contains ${needle}`)
}

function recordRequests(page: Page): Request[] {
  const seen: Request[] = []
  page.on('request', (r) => seen.push(r))
  return seen
}

test.describe('bundle failure', () => {
  let s: Session
  test.beforeEach(async ({ browser }) => { s = await signIn(browser, 'member') })
  test.afterEach(async () => {
    expect(s.problems, 'no unexpected console errors or uncaught exceptions').toEqual([])
    await s.context.close()
  })

  test('while the page module loads the host shows its loading state, then the page', async () => {
    const { page } = s
    let release!: () => void
    const gate = new Promise<void>((resolve) => { release = resolve })
    await page.route(ASSETS, async (route) => { await gate; await route.continue() })
    await page.goto('/browser/fragile', { waitUntil: 'commit' })
    const loading = page.locator('[data-runtime-page-loading]')
    await expect(loading).toBeVisible()
    await expect(loading).toHaveAttribute('role', 'status')
    await expect(page.locator(MOUNT)).toHaveAttribute('aria-busy', 'true')
    await expect(page.getByTestId('fragile')).toHaveCount(0)
    release()
    await expect(page.getByTestId('fragile')).toBeVisible()
    await expect(loading).toHaveCount(0)
    await expect(page.locator(MOUNT)).not.toHaveAttribute('aria-busy', 'true')
  })

  test('a page module that answers 404 shows the error state with Retry; Retry reloads and recovers', async () => {
    const { page } = s
    let assetRequests = 0
    await page.route(ASSETS, (route) => { assetRequests++; return route.fulfill({ status: 404, body: 'gone' }) })
    await page.goto('/browser/fragile')
    const alert = page.locator('[data-runtime-page-error]')
    await expect(alert).toBeVisible()
    await expect(alert).toHaveAttribute('role', 'alert')
    await expect(alert).toContainText('The screen could not be loaded.')
    await expect(page.locator('[data-runtime-page-retry]')).toHaveText('Retry')
    await expect(page.locator(MOUNT)).not.toHaveAttribute('aria-busy', 'true')
    await expect(page.locator('[data-iota-shell-standard]')).toBeVisible() // the failure is inside the standard shell
    await expect(page.getByTestId('fragile')).toHaveCount(0)
    expect(assetRequests, 'nothing retries on its own').toBe(1)
    expect(s.problems.some((p) => HOST_FAILURE.test(p))).toBe(true) // reported to the console by the host
    s.problems.splice(0)

    // The bundle is reachable again; Retry reloads the document and nothing else is replayed.
    await page.unroute(ASSETS)
    const requests = recordRequests(page)
    await Promise.all([page.waitForEvent('load'), page.locator('[data-runtime-page-retry]').click()])
    await expect(page.getByTestId('fragile')).toBeVisible()
    await expect(alert).toHaveCount(0)
    expect(requests.filter((r) => r.method() !== 'GET'), 'Retry writes nothing').toEqual([])
    expect(requests.filter((r) => r.isNavigationRequest()).map((r) => new URL(r.url()).pathname)).toEqual(['/browser/fragile'])
    expect(requests.some((r) => r.url().includes('/_ext/assets/'))).toBe(true) // the bundle was fetched again
  })

  test('a lazy component chunk that answers 404 shows the same error state, not an empty page', async () => {
    const chunk = await chunkContaining(s, 'Fragile page loaded')
    const { page } = s
    let chunkRequests = 0
    await page.route(chunk, (route) => { chunkRequests++; return route.fulfill({ status: 404, body: 'gone' }) })
    await page.goto('/browser/fragile')
    const alert = page.locator('[data-runtime-page-error]')
    await expect(alert).toBeVisible()
    await expect(alert).toContainText('The screen could not be loaded.')
    await expect(page.locator('[data-runtime-page-retry]')).toBeVisible()
    await expect(page.getByTestId('fragile')).toHaveCount(0)
    await expect(page.locator('[data-iota-shell-standard]')).toBeVisible()
    expect(chunkRequests).toBe(1)
    expect(s.problems.some((p) => HOST_FAILURE.test(p))).toBe(true)
    s.problems.splice(0)

    await page.unroute(chunk)
    await Promise.all([page.waitForEvent('load'), page.locator('[data-runtime-page-retry]').click()])
    await expect(page.getByTestId('fragile')).toBeVisible()
    await expect(alert).toHaveCount(0)
  })

  test('a page of the package that loads fine is not affected by another page\'s failure', async () => {
    const { page } = s
    const chunk = await chunkContaining(s, 'Fragile page loaded')
    await page.route(chunk, (route) => route.fulfill({ status: 404, body: 'gone' }))
    await page.goto('/browser/lab') // the lab's own chunk is another file
    await expect(page.getByTestId('lab')).toBeVisible()
    await expect(page.locator('[data-runtime-page-error]')).toHaveCount(0)
  })
})
