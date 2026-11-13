// Case 7: package styles. A package styles its client without inline styles: the SDK's Solid UI kit (the host's own
// instance, loaded lazily), Tailwind utilities compiled on the SDK design tokens, and a scoped CSS module. All of it is
// checked in a real Chromium by what the browser computes, not by what the markup says.
import { expect, signIn, state, test, type Page, type Session } from './harness'

declare global {
  interface Window { htmx: { ajax(verb: string, url: string, options: Record<string, unknown>): Promise<void> } }
}

const ann = state.clients.Ann
const clientPage = `/crm/clients/${ann}`
const KIT_MARKER = 'btn-loading-indicator' // a string of the kit's Button source: only the kit's chunk carries it

/** Computed style properties of an element by test id. */
async function computed<K extends string>(page: Page, testId: string, props: readonly K[]): Promise<Record<K, string>> {
  return page.getByTestId(testId).evaluate((el, names) => {
    const style = getComputedStyle(el)
    return Object.fromEntries(names.map((n) => [n, style.getPropertyValue(n)]))
  }, props) as Promise<Record<K, string>>
}

/** The text of every script the host bundle loads lazily (its chunks), by URL. */
function watchHostChunks(page: Page): Map<string, string> {
  const chunks = new Map<string, string>()
  page.on('response', (response) => {
    const url = new URL(response.url())
    if (/\/assets\/runtime-host-chunk-[\w-]+\.js$/.test(url.pathname)) void response.text().then((text) => chunks.set(url.pathname, text))
  })
  return chunks
}

const kitChunks = (chunks: Map<string, string>) => [...chunks].filter(([, text]) => text.includes(KIT_MARKER)).map(([url]) => url)

test.describe('package styles', () => {
  let s: Session
  test.afterEach(async () => {
    expect(s.problems, 'no console errors or uncaught exceptions').toEqual([])
    await s.context.close()
  })

  test('a package page renders kit components styled, with utilities on the SDK tokens and a scoped module class', async ({ browser }) => {
    s = await signIn(browser, 'auditor')
    const { page } = s
    await page.goto('/styled/dashboard')
    const dashboard = page.getByTestId('styled-dashboard')
    await expect(dashboard).toBeVisible()

    // A kit Button: its markup is the kit's; `shrink-0` is a utility only the kit's own sheet carries (the host stylesheet of
    // this suite is built without any source to scan), `.btn-primary` is the host's component class.
    const button = await computed(page, 'kit-button', ['cursor', 'flex-shrink', 'display', 'background-color', 'padding-left'])
    expect(button['cursor']).toBe('pointer')
    expect(button['flex-shrink']).toBe('0')
    expect(button['display']).toMatch(/flex/)
    expect(button['background-color']).not.toBe('rgba(0, 0, 0, 0)')
    expect(Number.parseFloat(button['padding-left'])).toBeGreaterThan(0)

    // Tabs from the kit work: the first panel shows, selecting the second swaps them.
    await expect(page.getByTestId('tab-body-terms')).toBeVisible()
    await expect(page.getByTestId('tab-body-claims')).toBeHidden()
    await page.getByTestId('tab-claims').click()
    await expect(page.getByTestId('tab-body-claims')).toBeVisible()
    await expect(page.getByTestId('tab-body-terms')).toBeHidden()
    await expect(page.getByTestId('tab-claims')).toHaveAttribute('aria-selected', 'true')

    // Tailwind utilities from the package's own sheet: `p-4` on the theme's spacing, `bg-primary-600` on the host's token.
    const utility = await computed(page, 'utility-box', ['padding-top', 'padding-left', 'background-color'])
    expect(utility['padding-top']).toBe('16px')
    expect(utility['padding-left']).toBe('16px')
    const token = await page.evaluate(() => {
      const probe = document.createElement('div')
      probe.style.backgroundColor = 'oklch(var(--primary-600) / 1)'
      document.body.append(probe)
      const color = getComputedStyle(probe).backgroundColor
      probe.remove()
      return color
    })
    expect(token).not.toBe('rgba(0, 0, 0, 0)') // the host defines the SDK token the utility refers to
    expect(utility['background-color']).toBe(token)

    // A scoped CSS module: the rules apply, under a class name unique to the package's sheet.
    const panel = await computed(page, 'styled-dashboard', ['display', 'letter-spacing', 'row-gap'])
    expect(panel['display']).toBe('grid')
    expect(panel['letter-spacing']).toBe('3px')
    expect(panel['row-gap']).toBe('11px')
    const accent = await computed(page, 'module-box', ['outline-style', 'outline-width'])
    expect(accent['outline-style']).toBe('solid')
    expect(accent['outline-width']).toBe('5px')
    expect(await dashboard.getAttribute('class')).toMatch(/^Dashboard_[0-9a-f]{8}_panel$/)

    // The version's two stylesheets are linked once: the HTML's links were reused, the host added none.
    const links = await page.locator('link[rel~="stylesheet"][data-iota-ext-css]').evaluateAll((els) => els.map((el) => el.getAttribute('href')))
    expect(links).toHaveLength(2)
    expect(new Set(links).size).toBe(2)
    expect(links[0]).toMatch(/\/client\/tailwind\.css$/)
  })

  test('the UI kit is a lazy chunk of the host: a page that does not use it never downloads it', async ({ browser }) => {
    s = await signIn(browser, 'auditor')
    const { page } = s
    const chunks = watchHostChunks(page)

    await page.goto('/browser/lab')
    await expect(page.getByTestId('lab')).toBeVisible()
    await page.waitForLoadState('networkidle')
    expect(kitChunks(chunks), 'a page of a package that does not use the kit').toEqual([])
    expect(await page.locator('style[data-iota-kit-css]').count()).toBe(0)
    expect(await page.evaluate(() => document.adoptedStyleSheets.length)).toBe(0)

    await page.goto('/styled/dashboard')
    await expect(page.getByTestId('kit-button')).toBeVisible()
    expect(kitChunks(chunks)).toHaveLength(1)
    // Its utilities came with it: exactly one sheet, installed once for the document.
    expect(await page.evaluate(() => document.adoptedStyleSheets.length + document.querySelectorAll('style[data-iota-kit-css]').length)).toBe(1)
  })

  test('a slot island loads its version stylesheet once, across HTMX swaps that remove and restore it', async ({ browser }) => {
    s = await signIn(browser, 'auditor')
    const { page } = s
    const requests: string[] = []
    page.on('request', (request) => { if (new URL(request.url()).pathname.endsWith('.css')) requests.push(new URL(request.url()).pathname) })
    await page.goto(clientPage)
    const badge = page.getByTestId('styled-badge')
    await expect(badge).toHaveText(`Styled for client ${ann}`)

    // The sidebar slot has other packages' contributions too (each with its own version); the styled package's list is
    // what the server put in the island's configuration.
    const styles = await page.locator('[data-iota-slot]').evaluate((el) => {
      const config = JSON.parse(el.getAttribute('data-iota-slot-config') ?? '{}') as { contributions: { id: string; styles?: string[] }[] }
      return config.contributions.find((c) => c.id === 'e2e.styled.clientBadge')?.styles ?? []
    })
    expect(styles, 'the version\'s two stylesheets').toHaveLength(2)
    const linked = () => page.locator('link[rel~="stylesheet"][data-iota-ext-css]').evaluateAll((els, wanted) => els.map((el) => el.getAttribute('href')).filter((h) => wanted.includes(h ?? '')), styles)
    const mine = () => requests.filter((r) => styles.includes(r))
    expect((await linked()).sort(), 'each linked once').toEqual([...styles].sort())
    expect(mine().sort(), 'and requested once').toEqual([...styles].sort())

    // The island is styled: the module class (7px left border) and the `px-3` utility (12px) both apply.
    const style = await computed(page, 'styled-badge', ['border-left-width', 'letter-spacing', 'padding-left'])
    expect(style['border-left-width']).toBe('7px')
    expect(style['letter-spacing']).toBe('2px')
    expect(style['padding-left']).toBe('12px')

    const swap = (select: string) => page.evaluate(
      ([url, pick]) => window.htmx.ajax('GET', url, { target: '[data-testid="crm-client-sidebar"]', swap: 'innerHTML', select: pick }),
      [clientPage, select] as const,
    )
    await swap('h1') // the region is replaced by the page's heading: the island is gone
    await expect(badge).toHaveCount(0)
    expect((await linked()).sort(), 'the stylesheets belong to the document, not to the island').toEqual([...styles].sort())

    await swap('[data-iota-slot]') // and the swap that brings the slot element back
    await expect(badge).toHaveCount(1)
    await expect(badge).toHaveText(`Styled for client ${ann}`)
    expect((await computed(page, 'styled-badge', ['border-left-width']))['border-left-width']).toBe('7px')

    expect((await linked()).sort(), 'no second link after the island came back').toEqual([...styles].sort())
    expect(mine().sort(), 'and no second request for any of the version\'s stylesheets').toEqual([...styles].sort())
  })
})
