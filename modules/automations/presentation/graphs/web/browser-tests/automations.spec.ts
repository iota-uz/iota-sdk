import { expect, test, type Page } from '@playwright/test'
import { expectNoA11yViolations } from '../../../../../../web/testing/src/a11y'

/**
 * The automations screens against the fixture host (fixtures/automations): the issue's checklist
 * (AL1–AL10) and the standard's states, with axe and screenshots against the Linux baseline.
 */

// Relative times («4 hours ago») are part of the screenshots: the clock is the fixture's.
test.beforeEach(async ({ page }) => { await page.clock.setFixedTime(new Date('2026-11-20T10:00:00Z')) })

const DIGEST = '00000000-0000-4000-8000-0000000000a1'
const WELCOME = '00000000-0000-4000-8000-0000000000a2'
const STALE = '00000000-0000-4000-8000-0000000000a3'
const FAILED_RUN = '00000000-0000-4000-8000-0000000000f1'
const BLOCKED_RUN = '00000000-0000-4000-8000-0000000000f3'
const OLD_RUN = '00000000-0000-4000-8000-0000000000f5'
const runOpen = async (p: Page) => { await expect(p.getByTestId('run-strip')).toBeVisible(); await expect(p.locator('.ae-runview .iota-flow__node, .ae-runview .ae-outline__item').first()).toBeVisible() }

const calls = (page: Page) => page.evaluate(() => (window as unknown as { __calls: { method: string; params: unknown }[] }).__calls)
const settle = (page: Page) => page.evaluate(() => Promise.all(document.getAnimations()
  .filter((a) => a.effect?.getComputedTiming().iterations !== Infinity)
  .map((a) => a.finished.catch(() => undefined))))
const listed = async (page: Page) => { await expect(page.locator(`[data-testid="row-${DIGEST}"], [data-testid="card-${DIGEST}"]`).filter({ visible: true })).toBeVisible() }

// S1/A1/Q1: every state at 1440 and 390 px, light and dark.
const states: { name: string; query: string; ready: (p: Page) => Promise<void>; open?: (p: Page) => Promise<void>; scope?: string }[] = [
  { name: 'list', query: '', ready: listed },
  { name: 'empty', query: 'scenario=empty', ready: async (p) => { await expect(p.getByTestId('list-empty')).toBeVisible() } },
  { name: 'empty-filtered', query: 'q=nothing', ready: async (p) => { await expect(p.getByTestId('list-empty-filtered')).toBeVisible() } },
  { name: 'loading', query: 'scenario=loading', ready: async (p) => { await expect(p.getByTestId('list-loading')).toBeVisible() } },
  { name: 'error', query: 'scenario=error', ready: async (p) => { await expect(p.getByTestId('list-error')).toBeVisible() } },
  {
    name: 'review-blocked', query: '', scope: '[data-testid="review-dialog"]',
    open: async (p) => { await listed(p); await toggleOf(p, STALE).click() },
    ready: async (p) => { await expect(p.getByTestId('review-blocked')).toBeVisible() },
  },
  // #1347: the Runs tab, a run with its node's drawer, a blocked run, no runs, a run not found.
  { name: 'runs', query: `view=editor&id=${DIGEST}&run=${FAILED_RUN}&node=email`, ready: async (p) => { await runOpen(p); await expect(p.getByTestId('run-node')).toBeVisible() } },
  { name: 'run-blocked', query: `view=editor&id=${DIGEST}&run=${BLOCKED_RUN}`, ready: async (p) => { await runOpen(p); await expect(p.getByTestId('run-resolve')).toBeVisible() } },
  { name: 'runs-empty', query: `view=editor&id=${STALE}&tab=runs`, ready: async (p) => { await expect(p.getByTestId('runs-empty')).toBeVisible() } },
  { name: 'run-not-found', query: 'view=run&run=00000000-0000-4000-8000-00000000dead', ready: async (p) => { await expect(p.getByTestId('run-not-found')).toBeVisible() } },
  {
    name: 'review-code', query: 'view=editor&id=' + DIGEST, scope: '[data-testid="review-dialog"]',
    open: async (p) => { await p.getByTestId('review-and-enable').click() },
    ready: async (p) => { await expect(p.getByTestId('code-source')).toBeVisible() },
  },
]

function toggleOf(page: Page, id: string) {
  return page.locator(`[data-testid="row-${id}"] label:has([data-testid="toggle"]), [data-testid="card-${id}"] label:has([data-testid="toggle"])`).filter({ visible: true }).first()
}

for (const width of [1440, 390]) {
  for (const theme of ['light', 'dark']) {
    for (const s of states) {
      test(`${s.name} ${width} ${theme}`, async ({ page }) => {
        await page.setViewportSize({ width, height: 900 })
        await page.goto(`/?theme=${theme}&${s.query}`)
        await s.open?.(page)
        await s.ready(page)
        await settle(page)
        await expectNoA11yViolations(page, s.scope ? { include: [s.scope] } : {})
        await expect(page).toHaveScreenshot(`${s.name}-${width}-${theme}.png`)
      })
    }
  }
}

test('AL1: triggers by meaning, status in words, search and filter in the URL', async ({ page }) => {
  await page.goto('/')
  await listed(page)
  const row = page.getByTestId(`row-${DIGEST}`)
  await expect(row).toContainText('On a schedule · 0 20 * * * (Asia/Tashkent)')
  await expect(row.getByTestId('status')).toContainText('On v2 · new version v3')
  await expect(page.getByTestId(`row-${WELCOME}`).getByTestId('status')).toContainText('On v1')
  await page.getByRole('button', { name: 'Need attention' }).click()
  await expect(page).toHaveURL(/status=attention/)
  await expect(page.getByTestId(`row-${WELCOME}`)).toHaveCount(0)
  await page.getByTestId('search').fill('daily')
  await expect(page).toHaveURL(/q=daily/)
  await page.reload()
  await expect(page.getByTestId('search')).toHaveValue('daily')
  await expect(page.getByRole('button', { name: 'Need attention' })).toHaveAttribute('aria-pressed', 'true')
  await expect(page.getByTestId(`row-${DIGEST}`)).toBeVisible()
})

test('AL2: without Automations.Author there is no create button in the DOM', async ({ page }) => {
  await page.goto('/?scenario=empty&cannot=author')
  await expect(page.getByTestId('list-empty')).toBeVisible()
  await expect(page.getByTestId('create')).toHaveCount(0)
  await expect(page.getByRole('link', { name: 'Create an automation' })).toHaveCount(0)
  await page.goto('/?scenario=empty')
  await expect(page.getByRole('link', { name: 'Create an automation' })).toBeVisible()
})

test('AL3: a graph saved by an agent is marked; an agent session renders no enable control', async ({ page }) => {
  await page.goto('/?agent')
  await listed(page)
  await expect(page.getByTestId(`row-${DIGEST}`).getByTestId('saved-by-agent')).toContainText('Saved by an agent')
  await expect(page.getByTestId('toggle')).toHaveCount(0)
  await page.goto(`/?agent&view=editor&id=${DIGEST}`)
  await expect(page.getByTestId('pending-banner')).toContainText('saved by an agent')
  await expect(page.getByTestId('review-and-enable')).toHaveCount(0)
})

test('AL4: the editor chunk loads only on the graph page; a failed load offers a retry', async ({ page }) => {
  const editorRequests: string[] = []
  page.on('request', (r) => { if (/EditorPage|\/src\/chunk\.ts|@xyflow/.test(r.url())) editorRequests.push(r.url()) })
  await page.goto('/')
  await listed(page)
  expect(editorRequests).toEqual([])

  let failed = false
  await page.route(/EditorPage\.tsx/, async (route) => {
    if (!failed) { failed = true; await route.abort(); return }
    await route.continue()
  })
  await page.goto(`/?view=editor&id=${WELCOME}`)
  await expect(page.getByTestId('editor-load-error')).toContainText('Could not load the editor')
  await page.getByRole('button', { name: 'Retry' }).click()
  await expect(page.getByTestId('crumb')).toBeVisible()
  expect(editorRequests.length).toBeGreaterThan(0)
})

test('AL6: the review shows code, secrets by name, triggers and run-as; a changed version is explained inside the dialog', async ({ page }) => {
  await page.goto(`/?view=editor&id=${DIGEST}&changeOnce&delay=400`)
  await page.getByTestId('review-and-enable').click()
  const dialog = page.getByTestId('review-dialog')
  await expect(dialog.getByTestId('code-source')).toContainText('export default async function run(ctx)')
  await expect(dialog.getByTestId('code-source').locator('span.text-purple').first()).toHaveText('export')
  await expect(dialog).toContainText('crm.api_token')
  await expect(dialog).toContainText('May call the internet')
  await expect(dialog.getByTestId('review-runas')).toContainText('you become the owner')
  await expect(dialog.getByTestId('review-enable')).toHaveText('Enable v3')
  await dialog.getByTestId('review-enable').click()
  // F4: while the request runs the dialog does not close.
  await page.keyboard.press('Escape')
  await expect(dialog).toBeVisible()
  await expect(dialog.getByTestId('review-notice')).toContainText('The code changed: review it again.')
  await expect(dialog.getByTestId('code-source')).toContainText('changed by the agent')
  await expect(dialog.getByTestId('review-enable')).toHaveText('Enable v4')
  await dialog.getByTestId('review-enable').click()
  await expect(dialog).toHaveCount(0)
  await expect(page.getByText('Enabled v4')).toBeVisible()
})

test('AL8: turning off names the pending runs, with the focus on Cancel', async ({ page }) => {
  await page.goto('/')
  await listed(page)
  await toggleOf(page, DIGEST).click()
  const dialog = page.getByTestId('disable-dialog')
  await expect(dialog.getByTestId('disable-text')).toHaveText('Its pending runs will be cancelled (3).')
  await expect(dialog.getByTestId('confirm-cancel')).toBeFocused()
  await dialog.getByTestId('confirm-action').click()
  await expect(page.getByText('The automation is turned off')).toBeVisible()
  await expect(page.getByTestId(`row-${DIGEST}`).getByTestId('status')).toContainText('Off')
})

test('AL8: deleting names the runs and waits it cancels', async ({ page }) => {
  await page.goto(`/?view=editor&id=${DIGEST}`)
  await page.getByTestId('delete').click()
  await expect(page.getByTestId('delete-text')).toHaveText('Deleting cancels 3 runs and 2 waits.')
  await expect(page.getByTestId('delete-dialog').getByTestId('confirm-cancel')).toBeFocused()
})

test('AL9: a manual start sends one request for a double click and opens the run from the toast', async ({ page }) => {
  await page.goto(`/?view=editor&id=${WELCOME}&delay=500`)
  await page.getByTestId('run-manual').click()
  const dialog = page.getByTestId('run-dialog')
  await dialog.locator('[data-field="email"]').fill('ada@acme.test')
  await dialog.getByTestId('run-start').dblclick()
  await expect(page.getByText('The run started')).toBeVisible()
  const starts = (await calls(page)).filter((c) => c.method === 'automations.run_manual')
  expect(starts).toHaveLength(1)
  expect(starts[0]!.params).toMatchObject({ trigger_id: 'by_hand', input: { email: 'ada@acme.test', vip: false } })
  await expect(page.getByRole('button', { name: 'Open the run' })).toBeVisible()
})

test('AL9: a required field is explained next to it', async ({ page }) => {
  await page.goto(`/?view=editor&id=${WELCOME}`)
  await page.getByTestId('run-manual').click()
  await page.getByTestId('run-start').click()
  await expect(page.getByTestId('run-dialog')).toContainText('Fill this field')
  expect((await calls(page)).some((c) => c.method === 'automations.run_manual')).toBe(false)
})

test('AL10: 390 px has no horizontal scroll and keeps the header on one line', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/')
  await listed(page)
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
  expect(overflow).toBeLessThanOrEqual(0)
  const header = page.locator('[data-testid="automations-list"] > header')
  const box = await header.boundingBox()
  expect(box!.height).toBeLessThan(60)
  await page.goto(`/?view=editor&id=${WELCOME}`)
  await expect(page.getByTestId('phone-banner')).toContainText('Editing is available on a computer')
})

test('layout: a save over positions of another tab is refused; Refresh shows them', async ({ page }) => {
  await page.goto(`/?view=editor&id=${DIGEST}&layoutConflict`)
  await expect(page.getByTestId('crumb')).toBeVisible()
  const card = page.locator('.iota-flow__node[data-id="email"]')
  await expect(card).toBeVisible()
  const box = (await card.boundingBox())!
  await page.mouse.move(box.x + 40, box.y + 20)
  await page.mouse.down()
  await page.mouse.move(box.x + 120, box.y + 60, { steps: 6 })
  await page.mouse.up()
  await expect(page.getByText('The layout was changed in another tab')).toBeVisible({ timeout: 5000 })
  await page.getByRole('button', { name: 'Refresh' }).click()
  const saves = (await calls(page)).filter((c) => c.method === 'automations.save_layout')
  expect(saves).toHaveLength(1)
})

test('AL8: without the count of pending runs the action waits, and a failed count offers a retry', async ({ page }) => {
  await page.goto('/?liveFailsOnce')
  await listed(page)
  await toggleOf(page, DIGEST).click()
  const dialog = page.getByTestId('disable-dialog')
  await expect(dialog.getByTestId('disable-dialog-load-error')).toBeVisible()
  await expect(dialog.getByTestId('confirm-action')).toBeDisabled()
  await dialog.getByRole('button', { name: 'Retry' }).click()
  await expect(dialog.getByTestId('disable-text')).toHaveText('Its pending runs will be cancelled (3).')
  await expect(dialog.getByTestId('confirm-action')).toBeEnabled()
  expect((await calls(page)).some((c) => c.method === 'automations.disable')).toBe(false)
})

test('AL7: why enabling is off is said next to the button; without the right the status explains itself', async ({ page }) => {
  await page.goto('/')
  await listed(page)
  await toggleOf(page, STALE).click()
  const dialog = page.getByTestId('review-dialog')
  await expect(dialog.getByTestId('review-blocked')).toContainText('a connection it uses is unavailable')
  await expect(dialog.getByTestId('review-enable')).toBeDisabled()
  await expect(dialog.getByTestId('review-enable')).toHaveAttribute('aria-describedby', 'review-blocked')

  await page.goto('/?cannot=enable')
  await listed(page)
  await expect(page.getByTestId('toggle')).toHaveCount(0)
  await expect(page.getByTestId('enable-note')).toContainText('needs the right')
  await page.goto(`/?cannot=enable&view=editor&id=${WELCOME}`)
  await expect(page.getByTestId('enable-note')).toBeVisible()
})

test('AL6: a failed reload after a refusal keeps nothing to confirm; Retry reviews the latest version', async ({ page }) => {
  await page.goto(`/?view=editor&id=${DIGEST}&changeOnce&reviewFailsAfterChange`)
  await page.getByTestId('review-and-enable').click()
  const dialog = page.getByTestId('review-dialog')
  await expect(dialog.getByTestId('review-enable')).toHaveText('Enable v3')
  await dialog.getByTestId('review-enable').click()
  await expect(dialog.getByTestId('review-load-error')).toBeVisible()
  await expect(dialog.getByTestId('code-source')).toHaveCount(0)
  await expect(dialog.getByTestId('review-enable')).toBeDisabled()
  await dialog.getByRole('button', { name: 'Retry' }).click()
  await expect(dialog.getByTestId('review-enable')).toHaveText('Enable v4')
  const reviews = (await calls(page)).filter((c) => c.method === 'automations.enable_review').map((c) => (c.params as { version: number }).version)
  expect(reviews).toEqual([3, 0, 0])
})

test('layout: a Save after a layout conflict sends the revision the positions were made over', async ({ page }) => {
  await page.goto(`/?view=editor&id=${DIGEST}&layoutConflict`)
  const card = page.locator('.iota-flow__node[data-id="email"]')
  await expect(card).toBeVisible()
  const box = (await card.boundingBox())!
  await page.mouse.move(box.x + 40, box.y + 20)
  await page.mouse.down()
  await page.mouse.move(box.x + 120, box.y + 60, { steps: 6 })
  await page.mouse.up()
  await expect(page.getByText('The layout was changed in another tab')).toBeVisible({ timeout: 5000 })
  // An edit of the graph, saved without «Refresh».
  await page.locator('.ae-toolbar__name').click()
  await page.getByLabel('Name').fill('Daily sales digest (edited)')
  await page.keyboard.press('Escape')
  await page.locator('.ae-save').click()
  await expect.poll(async () => (await calls(page)).filter((c) => c.method === 'automations.save').length).toBe(1)
  const save = (await calls(page)).find((c) => c.method === 'automations.save')!.params as { layout_revision?: number }
  expect(save.layout_revision).toBe(4)
})

test('D1: Back restores the tab the URL names', async ({ page }) => {
  await page.goto(`/?view=editor&id=${WELCOME}`)
  await expect(page.getByTestId('crumb')).toBeVisible()
  await page.getByRole('tab', { name: 'Runs' }).click()
  // The newest run opens in place of the bare tab (the same history entry).
  await expect(page).toHaveURL(/\/runs\/00000000-0000-4000-8000-0000000000f2/)
  await expect(page.getByRole('tab', { name: 'Runs' })).toHaveAttribute('aria-selected', 'true')
  await page.goBack()
  await expect(page).not.toHaveURL(/tab=runs|\/runs\//)
  await expect(page.getByRole('tab', { name: 'Edit' })).toHaveAttribute('aria-selected', 'true')
  await page.goForward()
  await expect(page.getByRole('tab', { name: 'Runs' })).toHaveAttribute('aria-selected', 'true')
})

test('layout: a fresher snapshot withdraws the notice of an older conflict', async ({ page }) => {
  await page.goto(`/?view=editor&id=${DIGEST}&layoutConflict`)
  const card = page.locator('.iota-flow__node[data-id="email"]')
  await expect(card).toBeVisible()
  const box = (await card.boundingBox())!
  await page.mouse.move(box.x + 40, box.y + 20)
  await page.mouse.down()
  await page.mouse.move(box.x + 120, box.y + 60, { steps: 6 })
  await page.mouse.up()
  await expect(page.getByText('The layout was changed in another tab')).toBeVisible({ timeout: 5000 })
  await page.getByRole('button', { name: 'Versions' }).click()
  await page.getByRole('menuitemcheckbox', { name: /^Version 2/ }).click()
  await expect(page.getByTestId('open-latest')).toBeVisible()
  await expect(page.getByText('The layout was changed in another tab')).toHaveCount(0)
})

const openVersion = async (page: Page, v: number) => {
  await page.getByRole('button', { name: 'Versions' }).click()
  await page.getByRole('menuitemcheckbox', { name: new RegExp(`^Version ${v}`) }).click()
}

test('D1: Back while an old version loads keeps the version the URL names', async ({ page }) => {
  await page.goto(`/?view=editor&id=${DIGEST}&oldVersionDelay=1500`)
  await expect(page.getByTestId('crumb')).toBeVisible()
  await openVersion(page, 2)
  await expect(page).toHaveURL(/version=2/)
  await page.goBack()
  await expect(page).not.toHaveURL(/version=/)
  await page.waitForTimeout(2500)
  await expect(page.getByTestId('open-latest')).toHaveCount(0)
  await expect(page.locator('.ae-save')).toBeVisible()
})

test('D1: unsaved edits are not dropped by opening a version or by Back', async ({ page }) => {
  await page.goto(`/?view=editor&id=${DIGEST}`)
  await expect(page.getByTestId('crumb')).toBeVisible()
  await openVersion(page, 2)
  await expect(page.getByTestId('open-latest')).toBeVisible()
  await page.goBack()
  await expect(page.locator('.ae-save')).toBeVisible()
  await page.locator('.ae-toolbar__name').click()
  await page.getByLabel('Name').fill('Edited digest')
  await page.keyboard.press('Escape')

  await page.goForward()
  const dialog = page.getByTestId('discard-dialog')
  await expect(dialog).toBeVisible()
  await dialog.getByTestId('discard-cancel').click()
  await expect(page).not.toHaveURL(/version=/)
  await expect(page.locator('.ae-toolbar__name')).toHaveText('Edited digest')

  await openVersion(page, 1)
  await expect(dialog).toBeVisible()
  await dialog.getByTestId('discard-confirm').click()
  await expect(page).toHaveURL(/version=1/)
  await expect(page.getByTestId('open-latest')).toBeVisible()
})

test('D1: edits made while another version loads are not dropped without consent', async ({ page }) => {
  await page.goto(`/?view=editor&id=${DIGEST}&oldVersionDelay=2500`)
  await expect(page.getByTestId('crumb')).toBeVisible()
  const rename = async (name: string) => {
    await page.locator('.ae-toolbar__name').click()
    await page.getByLabel('Name').fill(name)
    await page.keyboard.press('Escape')
  }
  const dialog = page.getByTestId('discard-dialog')

  // Keep editing: the draft stays and the URL names the version still open.
  await openVersion(page, 2)
  await rename('Edited while v2 loads')
  await expect(dialog).toBeVisible({ timeout: 5000 })
  await dialog.getByTestId('discard-cancel').click()
  await expect(page.locator('.ae-toolbar__name')).toHaveText('Edited while v2 loads')
  await expect(page).not.toHaveURL(/version=/)
  await expect(page.getByTestId('open-latest')).toHaveCount(0)

  // Discard: the version that arrived opens.
  await openVersion(page, 2)
  await expect(dialog).toBeVisible()
  await dialog.getByTestId('discard-confirm').click()
  await expect(page.getByTestId('open-latest')).toBeVisible({ timeout: 5000 })
  await expect(page).toHaveURL(/version=2/)
})

test('RV1–RV3: Runs is a tab of the same shell; the draft survives; the list is the open version\'s, the newest opens', async ({ page }) => {
  await page.goto(`/?view=editor&id=${DIGEST}`)
  await expect(page.locator('.ae-canvas')).toBeVisible()
  await page.locator('.ae-toolbar__name').click()
  await page.getByLabel('Name').fill('Digest (draft)')
  await page.keyboard.press('Escape')
  await page.getByRole('tab', { name: 'Runs' }).click()
  // The canvas tools are hidden; the draft, its version and Save stay.
  await expect(page.getByRole('button', { name: 'Validate' })).toHaveCount(0)
  await expect(page.locator('.ae-save')).toBeVisible()
  await expect(page.locator('.ae-toolbar__name')).toHaveText('Digest (draft)')
  await expect(page.getByTestId('run-row')).toHaveCount(2)
  await expect(page).toHaveURL(new RegExp(`/runs/${FAILED_RUN}`))
  await expect(page.getByTestId('run-state')).toHaveText('Failed')
  await expect(page.getByTestId('run-error')).toHaveText('The mail server refused the message')
  await expect(page.getByTestId('run-version-banner')).toHaveCount(0)
  await expect(page.getByTestId('run-conn')).toHaveText('Finished')
  await expect(page.locator('.ae-runview .iota-flow__node[data-id="email"]')).toHaveClass(/ae-card--failed/)
  // The list asks for the open version's runs only.
  const listed = (await calls(page)).filter((c) => c.method === 'automations.runs').map((c) => (c.params as { version?: number }).version)
  expect(listed.every((v) => v === 3)).toBe(true)
  await page.getByRole('tab', { name: 'Edit' }).click()
  await expect(page.locator('.ae-toolbar__name')).toHaveText('Digest (draft)')
  await expect(page.getByRole('button', { name: 'Validate' })).toBeVisible()
})

test('RV2/RV5: ↑ ↓ move between runs; state and subject filters and the node live in the URL', async ({ page }) => {
  await page.context().grantPermissions(['clipboard-read', 'clipboard-write'])
  await page.goto(`/?view=editor&id=${DIGEST}&tab=runs`)
  await expect(page.getByTestId('run-row')).toHaveCount(2)
  await page.getByTestId('run-row').first().focus()
  await page.keyboard.press('ArrowDown')
  await expect(page).toHaveURL(new RegExp(`/runs/${BLOCKED_RUN}`))
  await expect(page.getByTestId('run-state')).toHaveText('Blocked')
  await expect(page.getByTestId('run-row').nth(1)).toBeFocused()
  await page.keyboard.press('ArrowUp')
  await expect(page).toHaveURL(new RegExp(`/runs/${FAILED_RUN}`))
  // The node's drawer: input and output as a tree, a node the run did not reach says so.
  await page.locator('.ae-runview .iota-flow__node[data-id="email"]').click()
  await expect(page).toHaveURL(/node=email/)
  const drawer = page.getByTestId('run-node')
  await expect(drawer).toContainText('sales@acme.test')
  await expect(drawer).toContainText('The mail server refused the message')
  await drawer.getByRole('button', { name: 'Copy: Input' }).click()
  await expect(drawer.getByRole('status').filter({ hasText: 'Copied' })).toHaveCount(1)
  await page.keyboard.press('Escape')
  await expect(page).not.toHaveURL(/node=/)
  await page.getByRole('button', { name: 'Failed' }).click()
  await expect(page).toHaveURL(/state=failed/)
  await expect(page.getByTestId('run-row')).toHaveCount(1)
  await page.getByTestId('runs-search').fill('zzz')
  await expect(page.getByTestId('runs-empty-filter')).toBeVisible()
  await expect(page).toHaveURL(/subject=zzz/)
  await page.getByRole('button', { name: 'Reset filters' }).click()
  await expect(page.getByTestId('run-row')).toHaveCount(2)
  await expect(page).not.toHaveURL(/state=|subject=/)
  const filtered = (await calls(page)).filter((c) => c.method === 'automations.runs').map((c) => c.params as { state?: string[]; subject?: string })
  expect(filtered.some((p) => p.state?.join() === 'failed')).toBe(true)
  expect(filtered.some((p) => p.subject === 'zzz')).toBe(true)
})

test('RV6: a link with only a run id opens its graph; a run that is not there says so', async ({ page }) => {
  await page.goto(`/?view=run&run=${BLOCKED_RUN}`)
  await expect(page).toHaveURL(new RegExp(`/automations/${DIGEST}/runs/${BLOCKED_RUN}$`))
  await page.goto('/?view=run&run=00000000-0000-4000-8000-00000000dead')
  await expect(page.getByTestId('run-not-found')).toContainText('Run not found')
  // Inside the editor, a run that is not found has its own screen too.
  await page.goto(`/?view=editor&id=${DIGEST}&run=00000000-0000-4000-8000-00000000dead`)
  await expect(page.getByTestId('run-not-found')).toContainText('Run not found')
})

test('RV10: Retry and Resolve a blocked call; Cancel is confirmed with the focus on «Keep running»', async ({ page }) => {
  await page.goto(`/?view=editor&id=${DIGEST}&run=${BLOCKED_RUN}&delay=300`)
  await expect(page.getByTestId('run-state')).toHaveText('Blocked')
  const retry = page.getByTestId('run-retry')
  await retry.click()
  await retry.click() // F1: a second click while the request runs is ignored
  await expect(page.getByText('The call is repeated')).toBeVisible()
  const retries = (await calls(page)).filter((c) => c.method === 'automations.run_retry')
  expect(retries).toHaveLength(1)
  expect(retries[0]!.params).toEqual({ run_id: BLOCKED_RUN, wait_id: 'call-1' })
  await expect(page.getByTestId('run-state')).toHaveText('Running')
  await page.getByTestId('run-cancel').click()
  const dialog = page.getByTestId('cancel-dialog')
  await expect(dialog.getByRole('button', { name: 'Keep running' })).toBeFocused()
  await dialog.getByRole('button', { name: 'Cancel run' }).click()
  await expect(page.getByText('Run cancelled')).toBeVisible()
  await expect(page.getByTestId('run-state')).toHaveText('Cancelled')
  await expect(page.getByTestId('run-cancel')).toHaveCount(0)
})

test('RV10: Resolve picks an outcome of the blocked step', async ({ page }) => {
  await page.goto(`/?view=editor&id=${DIGEST}&run=${BLOCKED_RUN}`)
  await page.getByTestId('run-resolve').click()
  const dialog = page.getByTestId('resolve-dialog')
  await dialog.getByRole('combobox').click()
  await page.getByRole('option', { name: /^sent/ }).click()
  await dialog.getByTestId('resolve-confirm').click()
  await expect(page.getByText('The run goes on')).toBeVisible()
  const resolved = (await calls(page)).find((c) => c.method === 'automations.run_resolve')!
  expect(resolved.params).toEqual({ run_id: BLOCKED_RUN, wait_id: 'call-1', outcome: 'sent' })
  await expect(page.getByTestId('run-state')).toHaveText('Done')
})

test('RV10 / S5: without Operate there is no Cancel, Retry or Resolve in the DOM', async ({ page }) => {
  await page.goto(`/?view=editor&id=${DIGEST}&run=${BLOCKED_RUN}&cannot=operate`)
  await expect(page.getByTestId('run-state')).toHaveText('Blocked')
  await expect(page.getByTestId('run-retry')).toHaveCount(0)
  await expect(page.getByTestId('run-resolve')).toHaveCount(0)
  await expect(page.getByTestId('run-cancel')).toHaveCount(0)
})

test('RV3: a run of another version opens by its link on that version\'s graph, with its banner', async ({ page }) => {
  await page.goto(`/?view=editor&id=${DIGEST}&run=${OLD_RUN}`)
  await expect(page.getByTestId('run-version-banner')).toContainText('Run on version 2')
  await expect(page.getByTestId('run-row')).toHaveCount(2) // the open version's list, without this run
  const versions = (await calls(page)).filter((c) => c.method === 'automations.get' && (c.params as { version?: number }).version === 2)
  expect(versions).toHaveLength(1)
})

test('RV6: a run-only link keeps the node it names', async ({ page }) => {
  await page.goto(`/?view=run&run=${BLOCKED_RUN}&node=email`)
  await expect(page).toHaveURL(new RegExp(`/automations/${DIGEST}/runs/${BLOCKED_RUN}\\?node=email$`))
})

test('RV7: the decision mode reaches the server', async ({ page }) => {
  await page.goto(`/?view=editor&id=${DIGEST}`)
  await page.getByTestId('open-dry-run').click()
  await page.getByTestId('dry-run').getByRole('radio', { name: 'Live call' }).check()
  await page.getByTestId('dry-run-start').click()
  await expect(page.getByTestId('dry-trace')).toBeVisible()
  const sent = (await calls(page)).find((c) => c.method === 'automations.dry_run')!.params as { decisions?: string }
  expect(sent.decisions).toBe('live')
})

test('RV8 / F3: a retry after a lost answer sends the same key and the step runs once', async ({ page }) => {
  await page.goto(`/?view=editor&id=${DIGEST}&testLostOnce`)
  await page.locator('.iota-flow__node[data-id="email"]').click()
  await page.getByTestId('open-test-node').click()
  await page.getByTestId('test-start').click()
  await page.getByTestId('test-confirm').getByRole('button', { name: 'Run for real' }).click()
  await expect(page.getByTestId('test-node').getByRole('alert')).toBeVisible()
  await page.getByTestId('test-start').click()
  await page.getByTestId('test-confirm').getByRole('button', { name: 'Run for real' }).click()
  await expect(page.getByTestId('test-result')).toContainText('m-1')
  const keys = (await calls(page)).filter((c) => c.method === 'automations.test_node').map((c) => (c.params as { idempotency_key?: string }).idempotency_key)
  expect(keys).toHaveLength(2)
  expect(keys[0]).toBeTruthy()
  expect(keys[1]).toBe(keys[0])
})
