import { expect, test } from '@playwright/test'
import { expectNoA11yViolations } from '@iota-uz/sdk/testing'
import { card, center, drag, edge, handle, mod, open } from './helpers'

test.describe('canvas', () => {
  test('connecting an occupied outcome replaces its edge; undo brings the old one back (ED3)', async ({ page }) => {
    await open(page, 'graph=connected')
    await expect(edge(page, 'check', 'true')).toHaveAttribute('data-to', 'yes')
    const from = await center(handle(page, 'check', 'true'))
    const to = await center(card(page, 'no'))
    await page.mouse.move(from.x, from.y)
    await page.mouse.down()
    await page.mouse.move(to.x, to.y, { steps: 10 })
    // While rewiring the old edge fades to 0.3 (#1352) and the hint names the old target.
    await expect(edge(page, 'check', 'true')).toHaveClass(/ae-edge--rewiring/)
    await expect.poll(() => edge(page, 'check', 'true').evaluate((el) => getComputedStyle(el).opacity)).toBe('0.3')
    await expect(page.locator('.ae-hint')).toContainText('was →')
    await page.mouse.up()
    await expect(edge(page, 'check', 'true')).toHaveCount(1)
    await expect(edge(page, 'check', 'true')).toHaveAttribute('data-to', 'no')
    await page.locator('.ae-canvas').click({ position: { x: 600, y: 20 } })
    await page.keyboard.press(`${mod}+z`)
    await expect(edge(page, 'check', 'true')).toHaveAttribute('data-to', 'yes')
  })

  test('releasing an outcome on empty canvas opens the palette at the release point; Esc changes nothing', async ({ page }) => {
    await open(page, 'graph=connected')
    const from = await center(handle(page, 'yes', 'done'))
    const canvas = (await page.locator('.ae-canvas').boundingBox())!
    const drop = { x: canvas.x + 640, y: canvas.y + 120 }
    await drag(page, from, drop)
    const palette = page.locator('.ae-palette')
    await expect(palette).toBeVisible()
    const box = (await palette.boundingBox())!
    expect(Math.abs(box.x - drop.x)).toBeLessThan(40)
    expect(Math.abs(box.y - drop.y)).toBeLessThan(40)
    await expect(palette).toContainText('done')
    await page.keyboard.press('Escape')
    await expect(palette).toBeHidden()
    await expect(edge(page, 'yes', 'done')).toHaveAttribute('data-to', 'end')
  })

  test('releasing an outcome elsewhere changes nothing', async ({ page }) => {
    await open(page, 'graph=connected')
    const from = await center(handle(page, 'yes', 'done'))
    await drag(page, from, { x: 700, y: 20 })
    // The line pulls back into its handle (#1352), then goes.
    await expect(page.locator('.ae-edge--retract')).toHaveCount(1)
    await expect(page.locator('.ae-edge--retract')).toHaveCount(0)
    await expect(page.locator('.ae-palette')).toHaveCount(0)
    await expect(edge(page, 'yes', 'done')).toHaveAttribute('data-to', 'end')
  })

  test('picking a step from the palette after a release connects it to that outcome', async ({ page }) => {
    await open(page)
    const from = await center(handle(page, 'delay', 'done'))
    await drag(page, from, { x: from.x + 320, y: from.y - 40 })
    await page.locator('.ae-palette input').fill('end')
    await page.keyboard.press('Enter')
    await expect(edge(page, 'delay', 'done')).toHaveAttribute('data-to', 'end_2')
    await expect(card(page, 'end_2')).toBeVisible()
  })

  test('dragging a node calls onMove once with snapped positions', async ({ page }) => {
    await open(page, 'mode=canvas&graph=connected')
    const start = await center(card(page, 'yes').locator('.ae-card__title'))
    await drag(page, start, { x: start.x + 133, y: start.y + 71 }, 20)
    const moves = await page.evaluate(() => window.__fixture.moves)
    expect(moves).toHaveLength(1)
    const moved = moves[0]!.yes!
    expect(moved.x % 16).toBe(0)
    expect(moved.y % 16).toBe(0)
  })

  test('keyboard: Shift+F10 → "Connect <outcome> to…" → target palette (ED4)', async ({ page }) => {
    await open(page, 'graph=connected')
    await card(page, 'check').focus()
    await page.keyboard.press('Shift+F10')
    const menu = page.locator('.ae-menu')
    await expect(menu).toBeVisible()
    await menu.getByRole('menuitem', { name: 'Connect true to…' }).click()
    const palette = page.locator('.ae-palette')
    await expect(palette.getByRole('option', { name: /Not connected/ })).toBeVisible()
    await palette.locator('input').fill('End')
    await page.keyboard.press('Enter')
    await expect(edge(page, 'check', 'true')).toHaveAttribute('data-to', 'end')
  })

  test('arrows move between steps, Enter opens the panel, Delete focuses the parent (ED5)', async ({ page }) => {
    await open(page, 'graph=connected')
    await card(page, 'check').focus()
    await page.keyboard.press('ArrowRight')
    expect(['yes', 'no']).toContain(await page.evaluate(() => document.activeElement?.getAttribute('data-id')))
    await page.keyboard.press('Enter')
    await expect(page.locator('.ae-panel')).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(page.locator('.ae-panel')).toHaveCount(0)
    const focused = await page.evaluate(() => document.activeElement?.getAttribute('data-id'))
    await page.keyboard.press('Delete')
    await expect(card(page, focused!)).toHaveCount(0)
    await expect(card(page, 'check')).toBeFocused()
  })

  test('Alt+arrows move a step by one grid cell as one history step', async ({ page }) => {
    await open(page, 'mode=canvas&graph=connected')
    await card(page, 'end').focus()
    await page.keyboard.down('Alt')
    await page.keyboard.press('ArrowDown')
    await page.keyboard.press('ArrowDown')
    await page.keyboard.up('Alt')
    const moves = await page.evaluate(() => window.__fixture.moves)
    expect(moves).toHaveLength(1)
  })
})

test.describe('tidy', () => {
  const place = (page: import('@playwright/test').Page, id: string) => card(page, id).evaluate((el) => (el as HTMLElement).style.transform)
  // A freshly opened graph is already tidy, so a Tidy would move nothing: move a card by hand first.
  const moveByHand = async (page: import('@playwright/test').Page, id: string) => {
    const start = await center(card(page, id).locator('.ae-card__title'))
    await drag(page, start, { x: start.x + 96, y: start.y + 64 })
    await page.waitForTimeout(100)
  }

  test('the button sits next to Fit with a tooltip naming ⇧⌘L, and takes focus', async ({ page }) => {
    await open(page, 'graph=multi')
    const buttons = page.locator('.ae-controls button')
    await expect(buttons.nth(3)).toHaveAttribute('aria-label', 'Tidy up')
    await expect(buttons.nth(2)).toHaveAttribute('aria-label', 'Fit to screen')
    await expect(buttons.nth(3)).toHaveAttribute('title', /Tidy up \((Ctrl\+Shift\+L|⇧⌘L)\)/)
    await buttons.nth(3).focus()
    await expect(buttons.nth(3)).toBeFocused()
  })

  // Samples, every frame, where a card is and the path of an edge leaving it: a glide shows in-between places.
  const sample = (page: import('@playwright/test').Page, id: string) => page.evaluate((id) => {
    const seen: { place: string; edge: string }[] = []
    ;(window as unknown as { __samples: typeof seen }).__samples = seen
    const tick = () => {
      const el = document.querySelector<HTMLElement>(`.iota-flow__node[data-id="${id}"]`)
      const path = document.querySelector(`path.ae-edge[data-edge^="${id}."]`)
      seen.push({ place: el?.style.transform ?? '', edge: path?.getAttribute('d') ?? '' })
      requestAnimationFrame(tick)
    }
    requestAnimationFrame(tick)
  }, id)
  const samples = (page: import('@playwright/test').Page) => page.evaluate(() => (window as unknown as { __samples: { place: string; edge: string }[] }).__samples)

  test('one Tidy is one undo step; the cards glide over --motion-slow and their edges follow, undo glides back', async ({ page }) => {
    await open(page, 'graph=multi')
    const tidy = await place(page, 'a_check')
    await moveByHand(page, 'a_check')
    const moved = await place(page, 'a_check')
    expect(moved).not.toBe(tidy)
    await sample(page, 'a_check')
    await page.locator('.ae-controls .ae-tidy').click()
    await expect.poll(() => place(page, 'a_check')).toBe(tidy)
    await expect(page.locator('.ae-canvas--tidying')).toHaveCount(0)
    const glide = await samples(page)
    const places = new Set(glide.map((s) => s.place))
    // In-between places, not a jump from where it was to where it lands.
    expect([...places].filter((p) => p !== moved && p !== tidy).length).toBeGreaterThan(2)
    // The edge is redrawn on the way, frame by frame, not faded out and back.
    expect(new Set(glide.map((s) => s.edge)).size).toBeGreaterThan(3)
    await sample(page, 'a_check')
    await page.getByLabel('Undo').click()
    await expect.poll(() => place(page, 'a_check')).toBe(moved)
    const back = await samples(page)
    expect([...new Set(back.map((s) => s.place))].filter((p) => p !== moved && p !== tidy).length).toBeGreaterThan(2)
  })

  test('⇧⌘L tidies, and prefers-reduced-motion skips the glide', async ({ page }) => {
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await open(page, 'graph=multi')
    const tidy = await place(page, 'a_check')
    await moveByHand(page, 'a_check')
    const moved = await place(page, 'a_check')
    expect(moved).not.toBe(tidy)
    await sample(page, 'a_check')
    await page.keyboard.press(`${mod}+Shift+L`)
    await expect.poll(() => place(page, 'a_check')).toBe(tidy)
    await expect(page.locator('.ae-canvas--tidying')).toHaveCount(0)
    // No in-between frame: only where it was and where it landed.
    for (const s of await samples(page)) expect([moved, tidy]).toContain(s.place)
    expect((await card(page, 'a_check').evaluate((el) => getComputedStyle(el).transitionDuration)).split(', ').every((d) => d === '0s')).toBe(true)
  })

  test('the canvas menu has Tidy too', async ({ page }) => {
    await open(page, 'graph=multi')
    await page.locator('.ae-pane').click({ button: 'right', position: { x: 900, y: 700 } })
    await expect(page.getByRole('menuitem', { name: /Tidy up/ })).toBeVisible()
  })
})

test.describe('editor', () => {
  test('⌘K adds a step; ⌘Z removes it; copy and paste rename taken ids', async ({ page }) => {
    await open(page)
    await page.locator('.ae-canvas').click({ position: { x: 700, y: 30 } })
    await page.keyboard.press(`${mod}+k`)
    const palette = page.locator('.ae-palette')
    await expect(palette).toBeVisible()
    await palette.locator('input').fill('delay')
    await expect(palette.locator('.ae-palette__count')).toHaveText(/1 of \d+/)
    await page.keyboard.press('Enter')
    await expect(card(page, 'delay_2')).toBeVisible()
    await page.keyboard.press(`${mod}+z`)
    await expect(card(page, 'delay_2')).toHaveCount(0)
    await card(page, 'send_email').click()
    await page.keyboard.press('Escape')
    await card(page, 'send_email').click()
    await page.locator('.ae-canvas').focus()
    await page.keyboard.press(`${mod}+d`)
    await expect(card(page, 'send_email_2')).toBeVisible()
  })

  test('moving a step saves the layout once, debounced, without making the graph dirty', async ({ page }) => {
    await open(page)
    const start = await center(card(page, 'end').locator('.ae-card__title'))
    await drag(page, start, { x: start.x + 64, y: start.y + 96 })
    await expect.poll(() => page.evaluate(() => window.__fixture.saveLayouts.length)).toBe(1)
    await page.waitForTimeout(700)
    expect(await page.evaluate(() => window.__fixture.saveLayouts.length)).toBe(1)
    await expect(page.locator('.ae-tag--dirty')).toHaveCount(0)
    await expect(page.locator('.ae-save')).toHaveAttribute('aria-disabled', 'true')
  })

  test('validation errors block saving; a click on a row selects the step and its field', async ({ page }) => {
    await open(page, 'errors=1')
    await page.getByRole('button', { name: 'Validate' }).click()
    const dock = page.locator('.ae-validation')
    await expect(dock).toContainText('Unknown field user.mail.')
    await expect(card(page, 'has_email').locator('.ae-badge--error')).toBeVisible()
    await dock.getByRole('button', { name: /Unknown field/ }).click()
    await expect(page.locator('.ae-panel')).toContainText('Branch')
    await expect(page.locator('.ae-panel [data-param="expression"]')).toBeVisible()
    await card(page, 'send_email').click()
    const save = page.locator('.ae-save')
    await expect(save).toHaveAttribute('aria-disabled', 'true')
  })

  test('a step whose connection is unavailable is marked and named in Validation (ED10)', async ({ page }) => {
    await open(page, 'connection=lost')
    await page.getByRole('button', { name: 'Validate' }).click()
    const dock = page.locator('.ae-validation')
    await expect(dock).toContainText('Connection “Sales bot” is unavailable')
    await expect(dock).toContainText('“Classify text”: its connection is unavailable')
    for (const id of ['notify_sales', 'classify']) {
      await card(page, id).scrollIntoViewIfNeeded().catch(() => undefined)
    }
    await page.locator('.ae-canvas').focus()
    await page.keyboard.press('Shift+1')
    await expect(card(page, 'notify_sales').locator('.ae-badge--error')).toContainText('No connection')
    await expect(card(page, 'classify').locator('.ae-badge--error')).toContainText('No connection')
    await expect(page.locator('.ae-save')).toHaveAttribute('aria-disabled', 'true')
  })

  test('an edit during a check: the old answer is not shown for the new draft, and Save checks what it sends', async ({ page }) => {
    await open(page, 'latency=800')
    await card(page, 'has_email').click()
    const field = page.locator('.ae-panel [data-param="expression"] textarea').first()
    await field.fill('')
    await page.getByRole('button', { name: 'Validate' }).click()
    // While that check runs the expression is fixed again: the answer for the empty one must not stay.
    await field.fill('user.email != "" && true')
    await expect.poll(() => page.evaluate(() => window.__fixture.validates)).toBeGreaterThanOrEqual(2)
    await expect(page.locator('.ae-validation')).not.toContainText('An expression is required.')
    await page.locator('.ae-save').click()
    await expect.poll(() => page.evaluate(() => window.__fixture.saves.length)).toBe(1)
    const sent = await page.evaluate(() => window.__fixture.saves[0]!.graph.nodes.find((n) => n.id === 'has_email')!.params.expression)
    expect(sent).toBe('user.email != "" && true')
  })

  test('Save checks a changed draft first and does not send it when the check finds errors', async ({ page }) => {
    await open(page, 'latency=300')
    await card(page, 'has_email').click()
    await page.locator('.ae-panel [data-param="expression"] textarea').first().fill('')
    await page.locator('.ae-save').click()
    await expect(page.locator('.ae-validation')).toContainText('An expression is required.')
    expect(await page.evaluate(() => window.__fixture.saves.length)).toBe(0)
  })

  test('an undo while the layout is being saved saves the undone layout too (AS1)', async ({ page }) => {
    await open(page, 'latency=1400')
    const box = (await card(page, 'end').boundingBox())!
    const start = await center(card(page, 'end').locator('.ae-card__title'))
    await drag(page, start, { x: start.x + 80, y: start.y + 48 })
    await expect.poll(() => page.evaluate(() => window.__fixture.saveLayoutCalls), { timeout: 3000 }).toBe(1)
    await page.locator('.ae-canvas').focus()
    await page.keyboard.press(`${mod}+z`)
    await expect.poll(async () => (await card(page, 'end').boundingBox())!.x).toBeCloseTo(box.x, 0)
    await expect.poll(() => page.evaluate(() => window.__fixture.saveLayouts.length), { timeout: 6000 }).toBe(2)
    const [first, second] = await page.evaluate(() => window.__fixture.saveLayouts)
    expect(second!.end).not.toEqual(first!.end)
    expect(second!.end).toEqual(await page.evaluate(() => window.__fixture.initialLayout.end))
    await expect(page.locator('.ae-quiet')).toContainText('Layout saved')
  })

  test('Load latest switches draft, layout, baseline and version together, even if the width changes meanwhile', async ({ page }) => {
    await open(page, 'conflict=1&latency=1000')
    await card(page, 'has_email').click()
    await page.locator('.ae-panel [data-param="expression"] textarea').first().fill('true')
    await page.locator('.ae-save').click()
    await page.locator('.ae-banner').filter({ hasText: 'Saved by another user' }).getByRole('button', { name: 'Load latest' }).click()
    await page.getByRole('button', { name: 'Replace my draft' }).click()
    await page.setViewportSize({ width: 390, height: 844 })
    await expect(page.locator('.ae-toolbar__name')).toHaveText('Saved by Aziz', { timeout: 5000 })
    await expect(page.locator('.ae-tag').first()).toHaveText('v4')
    await page.setViewportSize({ width: 1440, height: 900 })
    // The replaced draft is the saved version: nothing to save, so no write with the old graph and base 4.
    await expect(page.locator('.ae-save')).toHaveAttribute('aria-disabled', 'true')
    await expect(page.locator('.ae-tag--dirty')).toHaveCount(0)
    expect(await page.evaluate(() => window.__fixture.saves.length)).toBe(1)
  })

  test('a move while Save waits is not overwritten by the older layout of that Save', async ({ page }) => {
    await open(page, 'latency=1800')
    await card(page, 'has_email').click()
    await page.locator('.ae-panel [data-param="expression"] textarea').first().fill('user.email != "" && true')
    await page.locator('.ae-save').click()
    // The step panel's drawer slides out for a moment after it opens and covers the right of the canvas;
    // hovering waits until nothing is over the title, so the drag starts on the card.
    const title = card(page, 'user_created').locator('.ae-card__title')
    await title.hover()
    const start = await center(title)
    await drag(page, start, { x: start.x + 64, y: start.y + 48 })
    const onScreen = await page.evaluate(() => window.__fixture.initialLayout.user_created!)
    await expect.poll(() => page.evaluate(() => window.__fixture.saves.length), { timeout: 10000 }).toBe(1)
    // When every write has settled, storage holds what the canvas shows, not the Save's older snapshot.
    await expect.poll(async () => {
      const stored = await page.evaluate(() => window.__fixture.layout.user_created)
      return stored && (stored.x !== onScreen.x || stored.y !== onScreen.y)
    }, { timeout: 15000 }).toBe(true)
    await page.waitForTimeout(2500)
    const stored = await page.evaluate(() => window.__fixture.layout.user_created!)
    expect(stored).not.toEqual(onScreen)
  })

  test('errors of a Save check are not published after the field was fixed', async ({ page }) => {
    await open(page, 'latency=1200')
    await card(page, 'has_email').click()
    const field = page.locator('.ae-panel [data-param="expression"] textarea').first()
    await field.fill('')
    // Records whether the stale error is ever shown, not only at the end (a later check would clear it).
    await page.evaluate(() => {
      const w = window as unknown as { __staleShown: boolean }
      w.__staleShown = false
      new MutationObserver(() => {
        if (document.body.textContent?.includes('An expression is required.')) w.__staleShown = true
      }).observe(document.body, { subtree: true, childList: true, characterData: true })
    })
    await page.locator('.ae-save').click()
    await field.fill('true')
    await page.waitForTimeout(4000)
    expect(await page.evaluate(() => (window as unknown as { __staleShown: boolean }).__staleShown)).toBe(false)
    await expect(page.getByText('An expression is required.')).toHaveCount(0)
    await expect(card(page, 'has_email').locator('.ae-badge--error')).toHaveCount(0)
    expect(await page.evaluate(() => window.__fixture.saves.length)).toBe(0)
  })

  test('a refused layout save stays visible with Retry and is not resent on its own', async ({ page }) => {
    await open(page, 'fail=savelayout&latency=300')
    const start = await center(card(page, 'end').locator('.ae-card__title'))
    await drag(page, start, { x: start.x + 64, y: start.y + 48 })
    const status = page.locator('.ae-quiet')
    await expect(status).toContainText('Layout not saved')
    await page.waitForTimeout(3000)
    expect(await page.evaluate(() => window.__fixture.saveLayoutCalls)).toBe(1)
    await expect(status).toContainText('Layout not saved')
    // Retry sends it again; once the host accepts, it is saved.
    await page.evaluate(() => window.__fixture.recover('savelayout'))
    await status.getByRole('button', { name: 'Retry' }).click()
    await expect(status).toContainText('Layout saved')
    expect(await page.evaluate(() => window.__fixture.saveLayoutCalls)).toBe(2)
    expect(await page.evaluate(() => window.__fixture.saveLayouts.length)).toBe(1)
  })

  test('a new move after a refused layout save is saved again', async ({ page }) => {
    await open(page, 'fail=savelayout&latency=100')
    const start = await center(card(page, 'end').locator('.ae-card__title'))
    await drag(page, start, { x: start.x + 64, y: start.y + 48 })
    await expect(page.locator('.ae-quiet')).toContainText('Layout not saved')
    await page.evaluate(() => window.__fixture.recover('savelayout'))
    const again = await center(card(page, 'end').locator('.ae-card__title'))
    await drag(page, again, { x: again.x + 32, y: again.y })
    await expect(page.locator('.ae-quiet')).toContainText('Layout saved')
    expect(await page.evaluate(() => window.__fixture.saveLayoutCalls)).toBe(2)
  })

  test('Versions lists the versions and opens one', async ({ page }) => {
    await open(page)
    await page.getByRole('button', { name: 'Versions' }).click()
    const menu = page.locator('.ae-menu')
    await expect(menu.getByRole('menuitemcheckbox')).toHaveCount(3)
    await menu.getByRole('menuitemcheckbox', { name: /Version 2/ }).click()
    await expect.poll(() => page.evaluate(() => window.__fixture.openedVersions)).toEqual([2])
  })

  test('narrowing to a phone closes Settings and stops editing', async ({ page }) => {
    await open(page)
    await page.getByRole('button', { name: 'Settings' }).click()
    await expect(page.getByRole('heading', { name: 'Automation settings' })).toBeVisible()
    await page.setViewportSize({ width: 390, height: 844 })
    await expect(page.locator('.ae-outline')).toBeVisible()
    await expect(page.getByRole('heading', { name: 'Automation settings' })).toHaveCount(0)
    await expect(page.locator('input:not([type="checkbox"]), textarea')).toHaveCount(0)
  })

  test('save sends the draft with its base; a network error keeps the draft and the retry sends the same base (ED12)', async ({ page }) => {
    await open(page, 'fail=save')
    await card(page, 'has_email').click()
    const field = page.locator('.ae-panel [data-param="expression"] textarea').first()
    await field.fill('user.email != "" && true')
    await expect(page.locator('.ae-tag--dirty')).toBeVisible()
    await page.locator('.ae-save').click()
    const toast = page.getByRole('alert').filter({ hasText: 'Could not save' }).or(page.locator('[role="status"]').filter({ hasText: 'Could not save' }))
    await expect(toast.first()).toBeVisible()
    await page.evaluate(() => window.__fixture.recover('save'))
    await page.getByRole('button', { name: 'Retry' }).first().click()
    await expect.poll(() => page.evaluate(() => window.__fixture.saves.length)).toBe(2)
    const saves = await page.evaluate(() => window.__fixture.saves.map((s) => s.base))
    expect(saves[0]).toBe(saves[1])
    await expect(page.locator('.ae-tag--dirty')).toHaveCount(0)
  })

  test('a conflict keeps the draft; "Load latest" replaces it after confirmation', async ({ page }) => {
    await open(page, 'conflict=1')
    await card(page, 'has_email').click()
    await page.locator('.ae-panel [data-param="expression"] textarea').first().fill('true')
    await page.locator('.ae-save').click()
    const banner = page.locator('.ae-banner').filter({ hasText: 'Saved by another user' })
    await expect(banner).toBeVisible()
    await banner.getByRole('button', { name: 'Load latest' }).click()
    await page.getByRole('button', { name: 'Replace my draft' }).click()
    await expect(page.locator('.ae-toolbar__name')).toHaveText('Saved by Aziz')
    await expect(banner).toHaveCount(0)
  })

  test('"Stay" keeps the draft and the next save goes over the other version', async ({ page }) => {
    await open(page, 'conflict=1')
    await card(page, 'has_email').click()
    await page.locator('.ae-panel [data-param="expression"] textarea').first().fill('true')
    await page.locator('.ae-save').click()
    await page.getByRole('button', { name: 'Stay' }).click()
    await expect(page.locator('.ae-save')).toHaveText(/Save over v4/)
    await page.locator('.ae-save').click()
    await expect.poll(() => page.evaluate(() => window.__fixture.saves.map((s) => s.base))).toEqual([3, 4])
  })

  test('offline shows the connection banner (S7)', async ({ page, context }) => {
    await open(page)
    await context.setOffline(true)
    await expect(page.locator('.ae-banner').filter({ hasText: 'No connection' })).toBeVisible()
    await context.setOffline(false)
    await expect(page.locator('.ae-banner').filter({ hasText: 'No connection' })).toHaveCount(0)
  })

  test('empty graph offers to add a trigger; the event palette is grouped and searchable', async ({ page }) => {
    await open(page, 'graph=empty')
    await page.getByRole('button', { name: 'Add a trigger' }).click()
    const palette = page.locator('.ae-palette')
    await expect(palette).toContainText('Telegram message received')
    await expect(palette).toContainText('no connection')
    await palette.locator('input').fill('schedule')
    await page.keyboard.press('Enter')
    await expect(card(page, 'schedule')).toBeVisible()
    await expect(card(page, 'schedule')).toContainText('Every day at 09:00')
  })

  test('no events in the catalog → a phrase about integrations and permissions', async ({ page }) => {
    await open(page, 'graph=empty&catalog=empty')
    await page.getByRole('button', { name: 'Add a trigger' }).click()
    await expect(page.locator('.ae-palette')).toContainText('No events available: check integrations and permissions')
  })

  test('read-only: a banner with the reason, no connectable handles, disabled fields', async ({ page }) => {
    await open(page, 'readonly=1')
    await expect(page.locator('.ae-banner').filter({ hasText: 'This is version 2' })).toBeVisible()
    await expect(page.locator('.iota-flow__handle.connectable')).toHaveCount(0)
    await expect(page.locator('.ae-save')).toHaveCount(0)
  })

  test('the toolbar fits one row at 1200 px (ED11)', async ({ page }) => {
    await page.setViewportSize({ width: 1200, height: 800 })
    await open(page)
    const toolbar = (await page.locator('.ae-toolbar').boundingBox())!
    expect(toolbar.height).toBeLessThanOrEqual(64)
    for (const name of ['Undo', 'Redo', 'Keys']) await expect(page.getByRole('button', { name, exact: true })).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
  })

  test('phones get the read-only outline (< 768 px)', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 })
    await open(page, 'overlay=1', '.ae-outline')
    await expect(page.locator('.ae-outline')).toBeVisible()
    await expect(page.getByText('Editing is available on a larger screen')).toBeVisible()
    await expect(page.locator('.ae-canvas')).toHaveCount(0)
    await expect(page.locator('.ae-save')).toHaveCount(0)
    await expect(page.locator('.ae-outline')).toContainText('Failed')
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
  })

  test('the key map opens with ?', async ({ page }) => {
    await open(page)
    await page.locator('.ae-canvas').click({ position: { x: 700, y: 30 } })
    await page.keyboard.press('?')
    await expect(page.getByRole('heading', { name: 'Keyboard shortcuts' })).toBeVisible()
  })

  test('localized: ru', async ({ page }) => {
    await open(page, 'locale=ru')
    await expect(page.getByRole('button', { name: 'Проверить' })).toBeVisible()
    await expect(card(page, 'schedule')).toContainText('Каждый день в 20:00')
  })

  for (const theme of ['light', 'dark'] as const) {
    test(`axe: no serious violations (${theme})`, async ({ page }) => {
      await open(page, theme === 'dark' ? 'theme=dark' : '')
      await card(page, 'has_email').click()
      await expectNoA11yViolations(page, { include: ['.ae-editor'] })
    })
  }
})

// Motion (#1352): microinteractions answer each action, layers leave by their reverse animation, and under
// prefers-reduced-motion nothing has an in-between frame.
test.describe('motion', () => {
  test('rewiring dims the old edge to 0.3, lights the target under the pointer, and the new edge flashes', async ({ page }) => {
    await open(page)
    const from = await center(handle(page, 'has_email', 'true'))
    const to = await center(card(page, 'end').locator('.ae-card__title'))
    await page.mouse.move(from.x, from.y)
    await page.mouse.down()
    await page.mouse.move(from.x + 4, from.y + 4, { steps: 2 })
    await page.mouse.move(to.x, to.y, { steps: 10 })
    await expect(edge(page, 'has_email', 'true')).toHaveClass(/ae-edge--rewiring/)
    await expect.poll(() => edge(page, 'has_email', 'true').evaluate((el) => getComputedStyle(el).opacity)).toBe('0.3')
    await expect.poll(() => card(page, 'end').evaluate((el) => getComputedStyle(el).borderColor)).not.toBe(
      await card(page, 'delay').evaluate((el) => getComputedStyle(el).borderColor))
    await page.mouse.up()
    await expect(edge(page, 'has_email', 'true')).toHaveAttribute('data-to', 'end')
    await expect(edge(page, 'has_email', 'true')).toHaveClass(/ae-edge--flash/)
    await expect(edge(page, 'has_email', 'true')).not.toHaveClass(/ae-edge--flash/)
  })

  test('Esc while rewiring brings the old edge back', async ({ page }) => {
    await open(page)
    const from = await center(handle(page, 'has_email', 'true'))
    await page.mouse.move(from.x, from.y)
    await page.mouse.down()
    await page.mouse.move(from.x + 40, from.y + 80, { steps: 6 })
    await expect(edge(page, 'has_email', 'true')).toHaveClass(/ae-edge--rewiring/)
    await page.keyboard.press('Escape')
    await page.mouse.up()
    await expect(edge(page, 'has_email', 'true')).not.toHaveClass(/ae-edge--rewiring/)
    await expect(edge(page, 'has_email', 'true')).toHaveAttribute('data-to', 'send_email')
  })

  test('a dragged card lifts, then lands on the grid', async ({ page }) => {
    await open(page)
    const start = await center(card(page, 'delay').locator('.ae-card__title'))
    await page.mouse.move(start.x, start.y)
    await page.mouse.down()
    await page.mouse.move(start.x + 30, start.y + 21, { steps: 6 })
    await expect(card(page, 'delay')).toHaveClass(/ae-card--lifted/)
    await expect.poll(() => card(page, 'delay').evaluate((el) => getComputedStyle(el).scale)).toBe('1.02')
    await page.mouse.up()
    await expect(card(page, 'delay')).not.toHaveClass(/ae-card--lifted/)
    await expect(card(page, 'delay')).toHaveClass(/ae-card--landing/)
    await expect(card(page, 'delay')).not.toHaveClass(/ae-card--landing/)
  })

  test('filtering the palette keeps the rows that stay; it leaves by its reverse animation', async ({ page }) => {
    await open(page)
    await page.locator('.ae-canvas').click({ position: { x: 700, y: 30 } })
    await page.keyboard.press(`${mod}+k`)
    const palette = page.locator('.ae-palette')
    await expect(palette).toBeVisible()
    await palette.locator('[data-choice="n:delay"]').evaluate((el) => { (el as HTMLElement & { __kept?: boolean }).__kept = true })
    await palette.locator('input').fill('del')
    await expect(palette.locator('.ae-palette__count')).toHaveText(/\d+ of \d+/)
    expect(await palette.locator('[data-choice="n:delay"]').evaluate((el) => (el as HTMLElement & { __kept?: boolean }).__kept === true)).toBe(true)
    await page.keyboard.press('Escape')
    await expect(palette).toHaveClass(/ae-layer--leaving/)
    await expect(palette).toHaveAttribute('inert', '')
    await expect(palette).toHaveCount(0)
  })

  test('the node panel slides out inert and focus goes back to the card', async ({ page }) => {
    await open(page)
    await card(page, 'send_email').click()
    const side = page.locator('.ae-side')
    await expect(side).toBeVisible()
    await side.getByRole('button', { name: /close/i }).first().click()
    await expect(side).toHaveCount(0)
    await expect(card(page, 'send_email')).toBeFocused()
  })

  test('prefers-reduced-motion: no animation has an in-between frame on any layer', async ({ page }) => {
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await open(page, 'overlay=1')
    // Every animation the page runs must be instant; an opacity change may keep at most 80 ms.
    const moving = () => page.evaluate(() => document.getAnimations()
      .map((a) => ({ name: (a as CSSAnimation).animationName ?? (a as CSSTransition).transitionProperty ?? '', timing: a.effect?.getComputedTiming() }))
      .filter((a) => a.timing && (a.timing.iterations === Infinity || Number(a.timing.duration) > 80))
      .map((a) => `${a.name} ${a.timing!.duration}ms ×${a.timing!.iterations}`))
    expect(await moving()).toEqual([])
    await open(page)
    await card(page, 'send_email').click()
    await expect(page.locator('.ae-side')).toBeVisible()
    expect(await moving()).toEqual([])
    await page.locator('.ae-canvas').click({ position: { x: 700, y: 30 } })
    await page.keyboard.press(`${mod}+k`)
    await expect(page.locator('.ae-palette')).toBeVisible()
    expect(await moving()).toEqual([])
    await page.keyboard.press('Escape')
    await expect(page.locator('.ae-palette')).toHaveCount(0)
    await page.getByRole('button', { name: 'Validate' }).click()
    expect(await moving()).toEqual([])
    const start = await center(card(page, 'delay').locator('.ae-card__title'))
    await drag(page, start, { x: start.x + 60, y: start.y + 40 })
    expect(await moving()).toEqual([])
  })
})
