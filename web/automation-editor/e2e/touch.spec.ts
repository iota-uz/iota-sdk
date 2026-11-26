// Touch (ED7, Q4): Chromium sends real touch events through CDP (Input.dispatchTouchEvent); WebKit
// iPad covers what Playwright can drive there (taps, layout of the touch UI).
import { expect, test, type CDPSession, type Page } from '@playwright/test'
import { card, center, edge, handle, open } from './helpers'

type Point = { x: number; y: number }

async function touch(cdp: CDPSession, type: 'touchStart' | 'touchMove' | 'touchEnd', points: Point[]) {
  await cdp.send('Input.dispatchTouchEvent', { type, touchPoints: points.map((p, id) => ({ x: Math.round(p.x), y: Math.round(p.y), id })) })
}

async function swipe(page: Page, cdp: CDPSession, from: Point, to: Point, steps = 12) {
  await touch(cdp, 'touchStart', [from])
  for (let i = 1; i <= steps; i += 1) {
    await touch(cdp, 'touchMove', [{ x: from.x + ((to.x - from.x) * i) / steps, y: from.y + ((to.y - from.y) * i) / steps }])
    await page.waitForTimeout(16)
  }
  await touch(cdp, 'touchEnd', [])
}

test.describe('touch (CDP)', () => {
  test.skip(({ browserName }) => browserName !== 'chromium', 'CDP touch events are Chromium only')

  test('a long press of 550 ms opens the step menu with a response ring; moving cancels it', async ({ page }) => {
    await open(page, 'graph=connected')
    const cdp = await page.context().newCDPSession(page)
    const point = await center(card(page, 'check').locator('.ae-card__title'))
    await touch(cdp, 'touchStart', [point])
    await expect(page.locator('.ae-press')).toBeVisible()
    await page.waitForTimeout(700)
    await touch(cdp, 'touchEnd', [])
    const menu = page.locator('.ae-menu')
    await expect(menu).toBeVisible()
    await expect(menu.getByRole('menuitem', { name: 'Duplicate' })).toBeVisible()
    // Shortcuts are not shown on touch (T3).
    await expect(menu.locator('kbd')).toHaveCount(0)
    await page.keyboard.press('Escape')
    await expect(menu).toHaveCount(0)

    // The press selected the step and opened its panel; a card on the left is still on screen.
    const other = await center(card(page, 'user_created').locator('.ae-card__title'))
    await touch(cdp, 'touchStart', [other])
    await touch(cdp, 'touchMove', [{ x: other.x + 30, y: other.y }])
    await page.waitForTimeout(700)
    await touch(cdp, 'touchEnd', [])
    await expect(page.locator('.ae-menu')).toHaveCount(0)
  })

  test('one finger drags a step; layout is saved once', async ({ page }) => {
    await open(page, 'graph=connected')
    const cdp = await page.context().newCDPSession(page)
    const before = await card(page, 'end').boundingBox()
    const point = await center(card(page, 'end').locator('.ae-card__title'))
    await swipe(page, cdp, point, { x: point.x + 96, y: point.y + 80 })
    const after = await card(page, 'end').boundingBox()
    expect(Math.abs(after!.x - before!.x)).toBeGreaterThan(40)
    await expect.poll(() => page.evaluate(() => window.__fixture.saveLayouts.length)).toBe(1)
  })

  test('dragging an outcome with a finger connects it (all outcomes already connected)', async ({ page }) => {
    await open(page, 'graph=connected')
    const cdp = await page.context().newCDPSession(page)
    const from = await center(handle(page, 'check', 'false'))
    const to = await center(card(page, 'yes').locator('.ae-card__title'))
    await swipe(page, cdp, from, to, 16)
    await expect(edge(page, 'check', 'false')).toHaveAttribute('data-to', 'yes')
  })

  test('pinch zooms the canvas', async ({ page }) => {
    await open(page, 'graph=connected')
    const cdp = await page.context().newCDPSession(page)
    const zoom = () => page.locator('.ae-controls .ae-iconbtn--text').innerText()
    const before = await zoom()
    const c = await center(page.locator('.ae-canvas'))
    await touch(cdp, 'touchStart', [{ x: c.x - 40, y: c.y }, { x: c.x + 40, y: c.y }])
    for (let i = 1; i <= 10; i += 1) {
      await touch(cdp, 'touchMove', [{ x: c.x - 40 - i * 12, y: c.y }, { x: c.x + 40 + i * 12, y: c.y }])
      await page.waitForTimeout(16)
    }
    await touch(cdp, 'touchEnd', [])
    await expect.poll(zoom).not.toBe(before)
  })

  test('"Select" then a finger drag selects steps; the menu acts on the whole selection', async ({ page }) => {
    await open(page, 'graph=connected')
    const cdp = await page.context().newCDPSession(page)
    await page.getByRole('button', { name: 'Select' }).tap()
    await expect(page.getByRole('button', { name: 'Select' })).toHaveAttribute('aria-pressed', 'true')
    const a = (await card(page, 'yes').boundingBox())!
    const b = (await card(page, 'no').boundingBox())!
    const from = { x: Math.min(a.x, b.x) - 20, y: Math.min(a.y, b.y) - 20 }
    const to = { x: Math.max(a.x + a.width, b.x + b.width) + 20, y: Math.max(a.y + a.height, b.y + b.height) + 20 }
    await swipe(page, cdp, from, to)
    const point = await center(card(page, 'yes').locator('.ae-card__title'))
    await touch(cdp, 'touchStart', [point])
    await page.waitForTimeout(700)
    await touch(cdp, 'touchEnd', [])
    await expect(page.locator('.ae-menu').getByRole('menuitem', { name: 'Delete 2 steps' })).toBeVisible()
  })

  test('the palette opens at the top without focusing search and has Close; targets are ≥ 44 px', async ({ page }) => {
    await open(page)
    await page.getByRole('button', { name: 'Add', exact: true }).first().tap()
    const palette = page.locator('.ae-palette')
    await expect(palette).toBeVisible()
    await expect(palette.locator('input')).not.toBeFocused()
    await expect(palette.getByRole('button', { name: 'Close' })).toBeVisible()
    const box = (await palette.boundingBox())!
    expect(box.y).toBeLessThan(140)
    await palette.getByRole('button', { name: 'Close' }).tap()
    const small = await page.evaluate(() => [...document.querySelectorAll<HTMLElement>('.ae-editor button, .ae-editor [role="tab"]')]
      .filter((el) => el.offsetParent !== null && !el.closest('[inert]'))
      .map((el) => ({ name: el.getAttribute('aria-label') ?? el.textContent?.trim(), h: el.getBoundingClientRect().height }))
      .filter((el) => el.h > 0 && el.h < 44))
    expect(small, JSON.stringify(small)).toEqual([])
  })
})

test.describe('touch (WebKit iPad)', () => {
  test.skip(({ browserName }) => browserName !== 'webkit', 'WebKit only')

  test('touch UI: Add and Select buttons, gestures first in the key map, palette as a sheet', async ({ page }) => {
    await open(page)
    await expect(page.getByRole('button', { name: 'Select' })).toBeVisible()
    await page.getByRole('button', { name: 'Keys', exact: true }).tap()
    await expect(page.getByRole('heading', { name: 'Gestures', level: 2 })).toBeVisible()
    await page.getByRole('button', { name: 'Close', exact: true }).last().tap()
    await page.getByRole('button', { name: 'Add', exact: true }).first().tap()
    await expect(page.locator('.ae-palette--sheet')).toBeVisible()
    await expect(page.locator('.ae-palette input')).not.toBeFocused()
  })

  test('tapping a step opens its panel', async ({ page }) => {
    await open(page)
    await card(page, 'send_email').locator('.ae-card__title').tap()
    await expect(page.locator('.ae-panel')).toContainText('Send email')
  })
})
