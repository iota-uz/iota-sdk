import { expect, test, type Page } from '@playwright/test'
import { expectNoA11yViolations } from '../../../../../../web/testing/src/a11y'

/**
 * The section «Backtests» of a graph (#1389) against the fixture host: its states at 1440 and 390 px in
 * both themes (S1, A1, Q1) and its scenarios (BT1–BT5).
 */

test.beforeEach(async ({ page }) => { await page.clock.setFixedTime(new Date('2026-11-20T10:00:00Z')) })

const GRAPH = '00000000-0000-4000-8000-0000000000b1'
const DS = '00000000-0000-4000-8000-0000000000d1'
const V2 = '00000000-0000-4000-8000-0000000000e1'
const V3 = '00000000-0000-4000-8000-0000000000e2'
const RUNNING = '00000000-0000-4000-8000-0000000000e3'
const base = `view=backtests&id=${GRAPH}`

const calls = (page: Page) => page.evaluate(() => (window as unknown as { __calls: { method: string; params: Record<string, unknown> }[] }).__calls)
const settle = (page: Page) => page.evaluate(() => Promise.all(document.getAnimations()
  .filter((a) => a.effect?.getComputedTiming().iterations !== Infinity)
  .map((a) => a.finished.catch(() => undefined))))

const states: { name: string; query: string; ready: (p: Page) => Promise<void>; open?: (p: Page) => Promise<void>; scope?: string }[] = [
  { name: 'backtests', query: base, ready: async (p) => { await expect(p.getByTestId(`backtest-${V3}`)).toBeVisible() } },
  { name: 'backtests-empty', query: `${base}&bt=empty`, ready: async (p) => { await expect(p.getByTestId('backtests-empty')).toBeVisible() } },
  { name: 'backtests-loading', query: `${base}&scenario=loading`, ready: async (p) => { await expect(p.getByTestId('backtests-loading')).toBeVisible() } },
  { name: 'backtests-error', query: `${base}&bt=error`, ready: async (p) => { await expect(p.getByTestId('backtests-error')).toBeVisible() } },
  { name: 'backtests-dataset', query: `${base}&dataset=${DS}`, ready: async (p) => { await expect(p.locator('[data-testid^="example-"], [data-testid="examples-cards"] li').filter({ visible: true }).first()).toBeVisible() } },
  { name: 'backtests-report', query: `${base}&backtest=${V3}&compare=${V2}`, ready: async (p) => { await expect(p.getByTestId('compare-table')).toBeVisible(); await expect(p.getByTestId('cases-table')).toBeVisible() } },
  { name: 'backtests-running', query: `${base}&backtest=${RUNNING}`, ready: async (p) => { await expect(p.getByTestId('report-status')).toContainText('2 of 6 events') } },
  {
    name: 'backtests-run-dialog', query: base, scope: '[data-testid="backtest-run-dialog"]',
    open: async (p) => { await p.getByTestId(`dataset-${DS}`).getByTestId('dataset-run').click(); await p.getByTestId('decisions-live').check() },
    ready: async (p) => { await expect(p.getByTestId('backtest-estimate')).toContainText('about') },
  },
]

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
        // T6: no horizontal scroll of the page.
        expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
      })
    }
  }
}

test('BT1: the editor links to «Backtests: N»; without Automations.Author the link is not in the DOM', async ({ page }) => {
  await page.goto(`/?view=editor&id=${GRAPH}`)
  const link = page.getByTestId('backtests-link')
  await expect(link).toHaveText('Backtests: 3')
  await expect(link).toHaveAttribute('href', `/automations/${GRAPH}/backtests`)
  // The canvas is the editor's own: the link sits in the host's actions, not on the canvas.
  await expect(page.locator('.iota-flow [data-testid="backtests-link"]')).toHaveCount(0)
  // Unsaved edits are not dropped by the link: the editor asks before the page goes (beforeunload).
  await page.locator('.ae-toolbar__name').click()
  await page.getByLabel('Name').fill('Edited routing')
  await page.keyboard.press('Escape')
  await expect(page.locator('.ae-toolbar__name')).toHaveText('Edited routing')
  await expect(page.locator('.ae-save')).toBeEnabled()
  let asked = false
  page.once('dialog', async (d) => { asked = d.type() === 'beforeunload'; await d.dismiss() })
  await link.click({ noWaitAfter: true })
  await expect.poll(() => asked).toBe(true)
  await expect(page.locator('.ae-toolbar__name')).toHaveText('Edited routing')
  await page.goto(`/?view=editor&id=${GRAPH}&cannot=author`)
  await expect(page.getByTestId('crumb')).toBeVisible()
  await expect(page.getByTestId('backtests-link')).toHaveCount(0)
  await page.goto(`/?view=editor&id=${GRAPH}&nobacktests`)
  await expect(page.getByTestId('crumb')).toBeVisible()
  await expect(page.getByTestId('backtests-link')).toHaveCount(0)
})

test('BT2: an empty section offers one action; a dataset from 30 days of the journal is created and imports in the background', async ({ page }) => {
  await page.goto(`/?${base}&bt=empty&delay=300`)
  await expect(page.getByTestId('backtests-empty')).toContainText('No datasets yet')
  await expect(page.getByTestId('dataset-new')).toHaveCount(0)
  await page.getByTestId('dataset-first').click()
  const dlg = page.getByTestId('import-dialog')
  await dlg.getByTestId('import-submit').click()
  await expect(dlg.getByText('Name the dataset.')).toBeVisible()
  await dlg.getByLabel('Name *').fill('Last month')
  await expect(dlg.getByTestId('import-days')).toHaveValue('30')
  await dlg.getByTestId('import-submit').click()
  // F1/F4: one request in flight, the dialog cannot be closed meanwhile.
  await expect(dlg.getByTestId('import-submit')).toContainText('Starting…')
  await dlg.getByTestId('import-submit').click({ force: true })
  await page.keyboard.press('Escape')
  await expect(dlg).toBeVisible()
  await expect(dlg).toBeHidden()
  await expect(page.getByText('The import is running in the background')).toBeVisible()
  const card = page.locator('[data-testid^="dataset-0"]').first()
  await expect(card.getByTestId('import-line')).toContainText('Importing…')
  const sent = (await calls(page)).filter((c) => c.method === 'automations.dataset_create' || c.method === 'automations.dataset_import')
  expect(sent.map((c) => c.method)).toEqual(['automations.dataset_create', 'automations.dataset_import'])
  expect(sent[1]!.params).toMatchObject({ kind: 'journal', days: 30 })
})

test('BT3: a label is a choice per classifier, saved at once; the open dataset is in the URL', async ({ page }) => {
  await page.goto(`/?${base}`)
  await page.getByTestId(`dataset-${DS}`).getByTestId('dataset-open').click()
  await expect(page).toHaveURL(new RegExp(`dataset=${DS}`))
  const row = page.locator('[data-testid^="example-"]').nth(3)
  await expect(row).toContainText('Do you have a discount for schools?')
  await expect(row.getByTestId('approximate')).toContainText('as it was at import')
  await row.getByTestId('label-intent').selectOption('refund')
  await expect.poll(async () => (await calls(page)).filter((c) => c.method === 'automations.example_label').map((c) => c.params))
    .toEqual([expect.objectContaining({ node: 'intent', outcome: 'refund' })])
  await page.getByTestId('backtests-back').click()
  await expect(page).not.toHaveURL(/dataset=/)
  // Back in the browser reopens the dataset (D1); the label is the one saved.
  await page.goBack()
  await expect(page).toHaveURL(new RegExp(`dataset=${DS}`))
  await expect(page.locator('[data-testid^="example-"]').nth(3).getByTestId('label-intent')).toHaveValue('refund')
})

test('BT3b: saves of one label run in order; a failed later save restores the label the server saved', async ({ page }) => {
  await page.goto(`/?${base}&dataset=${DS}&delay=400&labelFails=support`)
  const row = page.locator('[data-testid^="example-"]').nth(3)
  await expect(row).toContainText('Do you have a discount for schools?')
  const select = row.getByTestId('label-intent')
  await select.selectOption('refund')
  await select.selectOption('support')
  await expect(select).toHaveValue('support')
  // While the first save is on the way (400 ms) the second waits for it.
  const labels = async () => (await calls(page)).filter((c) => c.method === 'automations.example_label').map((c) => c.params.outcome)
  expect(await labels()).toEqual(['refund'])
  // The second save starts after the first one is saved, and its failure restores «refund».
  await expect(select).toHaveValue('refund')
  expect(await labels()).toEqual(['refund', 'support'])
})

test('BT3c: a failed save restores the label the examples read last, after another user changed it', async ({ page }) => {
  await page.goto(`/?${base}&dataset=${DS}&labelFails=support`)
  const at = page.locator('[data-testid^="example-"]').nth(3)
  await expect(at).toContainText('Do you have a discount for schools?')
  const testid = (await at.getAttribute('data-testid'))!
  const id = testid.replace('example-', '')
  // The row itself: the order of rows may change when the examples are read again.
  const select = page.getByTestId(testid).getByTestId('label-intent')
  const other = page.locator('[data-testid^="example-"]').nth(5).getByTestId('label-intent')
  await select.selectOption('refund')
  await expect.poll(async () => (await calls(page)).filter((c) => c.method === 'automations.example_label').length).toBe(1)
  // Another user labels it «sales»; a label of another example moves the count, and the examples are read again.
  await page.evaluate((exampleId) => (window as unknown as { __elsewhere: (m: string, p: unknown) => unknown }).__elsewhere('automations.example_label', { example_id: exampleId, node: 'intent', outcome: 'sales' }), id)
  await other.selectOption('sales')
  await expect(select).toHaveValue('sales')
  await select.selectOption('support')
  // The rollback itself shows «sales» (not «refund», this panel's own earlier save), before any later read.
  await expect.poll(() => select.inputValue()).not.toBe('support')
  expect(await select.inputValue()).toBe('sales')
  await expect.poll(async () => (await calls(page)).filter((c) => c.method === 'automations.example_label').map((c) => c.params.outcome)).toEqual(['refund', 'sales', 'support'])
})

test('BT3d: examples read while a save is pending keep the edit on screen; its failure restores the label they read', async ({ page }) => {
  await page.goto(`/?${base}&dataset=${DS}&labelFails=support&delay=1500&delayOnly=automations.example_label`)
  const rows = page.locator('[data-testid^="example-"]')
  await expect(rows.nth(3)).toContainText('Do you have a discount for schools?')
  const testid = (await rows.nth(3).getAttribute('data-testid'))!
  const other = (await rows.nth(5).getAttribute('data-testid'))!.replace('example-', '')
  const select = page.getByTestId(testid).getByTestId('label-intent')
  await select.selectOption('support')
  // While the save is on the way another user labels the example «refund» (and another one, which moves
  // the count), and the examples are read again.
  const reads = async () => (await calls(page)).filter((c) => c.method === 'automations.dataset_examples').length
  const before = await reads()
  await page.evaluate(([exampleId, otherId]) => {
    const elsewhere = (window as unknown as { __elsewhere: (m: string, p: unknown) => unknown }).__elsewhere
    elsewhere('automations.example_label', { example_id: exampleId, node: 'intent', outcome: 'refund' })
    elsewhere('automations.example_label', { example_id: otherId, node: 'intent', outcome: 'sales' })
    window.dispatchEvent(new Event('online'))
  }, [testid.replace('example-', ''), other] as const)
  await expect.poll(reads).toBeGreaterThan(before)
  await expect(select).toHaveValue('support')
  // The save fails: the rollback shows the label the examples read, not the one before the edit.
  await expect.poll(() => select.inputValue(), { timeout: 5000 }).not.toBe('support')
  expect(await select.inputValue()).toBe('refund')
})

test('BT3e: examples read before a save committed are not shown over it; they are read again', async ({ page }) => {
  await page.goto(`/?${base}&dataset=${DS}&delay=1500&delayOnly=automations.dataset_examples,automations.backtests&staleReads`)
  const rows = page.locator('[data-testid^="example-"]')
  // An example labeled already: its save does not move the count, so nothing else reads the examples.
  await expect(rows.nth(0)).toContainText('I want to buy 20 licenses')
  const testid = (await rows.nth(0).getAttribute('data-testid'))!
  const other = (await rows.nth(5).getAttribute('data-testid'))!.replace('example-', '')
  const select = page.getByTestId(testid).getByTestId('label-intent')
  const reads = async () => (await calls(page)).filter((c) => c.method === 'automations.dataset_examples').length
  const before = await reads()
  // Another user's label moves the count: the examples are read again, and that read answers 1.5 s
  // later with the state at its start. The overview answers 1.5 s late too.
  await page.evaluate((otherId) => {
    ;(window as unknown as { __elsewhere: (m: string, p: unknown) => unknown }).__elsewhere('automations.example_label', { example_id: otherId, node: 'intent', outcome: 'sales' })
    window.dispatchEvent(new Event('online'))
  }, other)
  await expect.poll(reads).toBeGreaterThan(before)
  // Meanwhile the panel saves «support», at once; every value the field shows from then on is recorded.
  await select.selectOption('support')
  await expect.poll(async () => (await calls(page)).filter((c) => c.method === 'automations.example_label').length).toBe(1)
  await page.evaluate((id) => {
    const w = window as unknown as { __seen: string[] }
    w.__seen = []
    const el = document.querySelector(`[data-testid="${id}"] [data-testid="label-intent"]`) as HTMLSelectElement | null
    setInterval(() => { const v = (document.querySelector(`[data-testid="${id}"] [data-testid="label-intent"]`) as HTMLSelectElement | null ?? el)?.value; if (v !== undefined) w.__seen.push(v) }, 20)
  }, testid)
  // The older read answers (1.5 s after it began); the overview read after the save answers later.
  await page.waitForTimeout(4500)
  const seen = await page.evaluate(() => (window as unknown as { __seen: string[] }).__seen)
  expect(seen.length).toBeGreaterThan(10)
  expect([...new Set(seen)]).toEqual(['support'])
})

test('BT4: the cases table filters by classifier and mismatches, in the URL; «Reset filters» clears only them', async ({ page }) => {
  await page.goto(`/?${base}&backtest=${V2}`)
  await expect(page.getByTestId('metrics-intent')).toContainText('Correct 60% (3 of 5)')
  await expect(page.getByTestId('metrics-intent-refund').getByTestId('recall')).toHaveText('0%')
  await expect(page.getByTestId('mismatch-total')).toContainText('2 differ from the labels')
  await page.getByTestId('cases-classifier').selectOption('intent')
  await page.getByText('Only mismatches').click()
  await expect(page).toHaveURL(/node=intent/)
  await expect(page).toHaveURL(/mismatch=1/)
  await expect(page.locator('[data-testid^="case-0"]')).toHaveCount(2)
  await page.reload()
  await expect(page.locator('[data-testid^="case-0"]')).toHaveCount(2)
  await page.locator('[data-testid^="case-0"]').first().getByTestId('case-toggle').click()
  await expect(page.getByTestId('case-detail')).toContainText('support@acme.test')
  await page.goto(`/?${base}&backtest=${V3}&node=intent&mismatch=1`)
  await expect(page.getByTestId('cases-empty-filtered')).toContainText('No mismatches with the labels')
  await page.getByRole('button', { name: 'Reset filters' }).click()
  await expect(page).not.toHaveURL(/mismatch=1/)
  await expect(page).toHaveURL(new RegExp(`backtest=${V3}`))
})

test('BT5: live decisions show their cost before the run; an agent is offered cached decisions only; a run opens its progress', async ({ page }) => {
  await page.goto(`/?${base}`)
  await page.getByTestId('backtest-new').click()
  const dlg = page.getByTestId('backtest-run-dialog')
  await expect(dlg.getByTestId('backtest-estimate')).toContainText('At most 6 calls to models')
  await dlg.getByTestId('decisions-live').check()
  await expect(dlg.getByTestId('backtest-estimate')).toContainText('Up to 6 calls to models, about $0.0126')
  // A paid run waits for its estimate: while it is counted, «Run» is off.
  await page.goto(`/?${base}&delay=1500`)
  await page.getByTestId('backtest-new').click()
  await dlg.getByTestId('decisions-live').check()
  await expect(dlg.getByTestId('backtest-run-submit')).toBeDisabled()
  await expect(dlg.getByTestId('backtest-estimate')).toContainText('about $0.0126')
  await expect(dlg.getByTestId('backtest-run-submit')).toBeEnabled()
  await expect(dlg).toContainText('Nothing is sent')
  await dlg.getByTestId('backtest-run-submit').click()
  await expect(page.getByTestId('report-status')).toContainText('Waiting for the worker')
  await expect(page).toHaveURL(/backtest=/)
  await expect(page.getByTestId('report-cancel')).toBeVisible()
  await page.getByTestId('report-cancel').click()
  await expect(page.getByTestId('report-status')).toContainText('Cancelled')

  await page.goto(`/?${base}&agent`)
  await page.getByTestId('backtest-new').click()
  await expect(page.getByTestId('backtest-run-dialog').getByTestId('decisions-live')).toHaveCount(0)
  await expect(page.getByTestId('live-note')).toContainText('run by a person')
})
