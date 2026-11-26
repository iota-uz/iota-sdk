// The Runs tab, the run view, Dry run and Test node (#1347) against the fixture host's fake EditorApi.
import { expect, test, type Page } from '@playwright/test'
import { expectNoA11yViolations } from '@iota-uz/sdk/testing'
import { card, open } from './helpers'

const LIVE = '00000000-0000-4000-8000-0000000000r1'
const FAILED = '00000000-0000-4000-8000-0000000000r2'
const OLD = '00000000-0000-4000-8000-0000000000r3'
const runView = (page: Page) => page.getByTestId('run-view')
const runCard = (page: Page, id: string) => runView(page).locator(`.iota-flow__node[data-id="${id}"]`)
const runCalls = (page: Page) => page.evaluate(() => window.__fixture.runCalls)

test.describe('runs', () => {
  test('RV4 / acceptance: a live run is polled every 2 s, updates its cards in place and stops for good once it ends', async ({ page }) => {
    await open(page, `tab=runs&run=${LIVE}`, '.ae-runview')
    await expect(page.getByTestId('run-conn')).toHaveText('Live')
    await expect(runCard(page, 'send_email')).toHaveClass(/ae-card--running/)
    const before = await runCard(page, 'has_email').elementHandle()
    // The strip and the run's row are updated in place by a poll, never rebuilt (P4, #1352).
    const strip = await page.getByTestId('run-strip').elementHandle()
    const row = await page.locator(`[data-run="${LIVE}"]`).elementHandle()
    // After two polls the fake moves the run on: the email is sent and the run waits on the delay.
    await expect(runCard(page, 'delay')).toHaveClass(/ae-card--waiting/, { timeout: 8000 })
    await expect(runCard(page, 'send_email')).toHaveClass(/ae-card--passed/)
    // The card that did not change is the same element (nothing blinks, P4).
    expect(await before!.evaluate((el) => el.isConnected)).toBe(true)
    await expect(page.getByTestId('run-state')).toHaveText('Waiting')
    // The row of the list follows the run.
    await expect(page.getByTestId('run-row').first()).toContainText('Waiting')
    expect(await strip!.evaluate((el) => el.isConnected)).toBe(true)
    expect(await row!.evaluate((el) => el.isConnected)).toBe(true)
  })

  test('acceptance: polling stops when the run is final and does not resume on a tab switch', async ({ page }) => {
    await open(page, `tab=runs&run=${FAILED}`, '.ae-runview')
    await expect(page.getByTestId('run-conn')).toHaveText('Finished')
    const calls = await runCalls(page)
    await page.waitForTimeout(4500)
    expect(await runCalls(page)).toBe(calls)
    await page.getByTestId('tab-edit').click()
    await page.getByTestId('tab-runs').click()
    await expect(page.getByTestId('run-conn')).toHaveText('Finished')
    await page.waitForTimeout(2500)
    expect(await runCalls(page)).toBe(calls)
  })

  test('acceptance: a run of version 2 with version 3 open draws the graph of version 2 and its banner; the removed step is there', async ({ page }) => {
    await open(page, `tab=runs&run=${OLD}`, '.ae-runview')
    await expect(page.getByTestId('run-version-banner')).toContainText('Run on version 2')
    await expect(runCard(page, 'legacy_note')).toBeVisible()
    await expect(runCard(page, 'legacy_note')).toHaveClass(/ae-card--passed/)
    // The failed branch carries its message on the card.
    await expect(runCard(page, 'send_email').locator('.ae-card__note')).toHaveText('Address rejected by the provider')
    await page.getByTestId('open-run-version').click()
    expect(await page.evaluate(() => window.__fixture.openedVersions)).toEqual([2])
  })

  test('RV5: logs are masked text, filtered with their own empty state; Copy is announced', async ({ page }) => {
    await page.context().grantPermissions(['clipboard-read', 'clipboard-write'])
    await open(page, `tab=runs&run=${FAILED}&node=send_email`, '.ae-runview')
    const drawer = page.getByTestId('run-node')
    await expect(drawer.getByRole('tab', { name: 'Attempt 2' })).toHaveAttribute('aria-selected', 'true')
    const logs = drawer.getByTestId('run-logs')
    await expect(logs.locator('pre')).toContainText('calling api with token ***')
    // HTML is text, never markup.
    await expect(logs.locator('pre')).toContainText('<b>not html</b>')
    await expect(logs.locator('b')).toHaveCount(0)
    await logs.getByRole('searchbox').fill('smtp')
    await expect(logs.locator('pre')).toHaveText('connecting to smtp.acme.test\nSMTP 550 mailbox unavailable')
    await expect(logs).toContainText('2 of 4')
    await logs.getByRole('searchbox').fill('nothing like it')
    await expect(logs.getByTestId('logs-nomatch')).toHaveText('No log line matches “nothing like it”')
    await drawer.getByRole('button', { name: 'Copy: Input' }).click()
    await expect(drawer.getByRole('status').filter({ hasText: 'Copied' })).toHaveCount(1)
    // A step the run did not reach says so; Escape closes the drawer and the node leaves the URL.
    await runCard(page, 'end').click()
    await expect(drawer.getByTestId('run-node-empty')).toHaveText('The run did not reach this step.')
    await expect(page).toHaveURL(/node=end/)
    await page.keyboard.press('Escape')
    // It leaves through its exit animation, inert (A7), and the focus is back on the step's card.
    await expect(page.getByTestId('run-node')).toHaveCount(0)
    await expect(page).not.toHaveURL(/node=/)
    await expect(runCard(page, 'end')).toBeFocused()
  })

  test('RV9: no runs → «No runs yet» and «Run» for a manual graph; a filter without match → its reset', async ({ page }) => {
    await open(page, 'tab=runs&runs=empty&manual=1', '.ae-runlist')
    await expect(page.getByTestId('runs-empty')).toContainText('No runs yet')
    await page.getByRole('button', { name: 'Run', exact: true }).click()
    expect(await page.evaluate(() => window.__fixture.ops)).toEqual(['start'])
    await open(page, 'tab=runs&state=blocked&subject=zz', '.ae-runlist')
    await expect(page.getByTestId('runs-empty-filter')).toContainText('No runs match the filter')
    await page.getByRole('button', { name: 'Reset filters' }).click()
    await expect(page.getByTestId('run-row')).toHaveCount(4) // the open version's (3) runs
  })

  test('RV6 / D3: a run that is not found has its own screen', async ({ page }) => {
    await open(page, `tab=runs&run=${FAILED}&runs=notfound`, '.ae-runlist')
    await expect(page.getByTestId('run-not-found')).toContainText('Run not found')
  })

  test('S5: without Operate the buttons are not rendered; without Author there is no Dry run or Test', async ({ page }) => {
    await open(page, 'tab=runs&run=00000000-0000-4000-8000-0000000000r4&can=read', '.ae-runview')
    await expect(page.getByTestId('run-state')).toHaveText('Blocked')
    await expect(page.getByTestId('run-retry')).toHaveCount(0)
    await expect(page.getByTestId('run-cancel')).toHaveCount(0)
    await page.getByTestId('tab-edit').click()
    await expect(page.getByTestId('open-dry-run')).toHaveCount(0)
    await card(page, 'send_email').click()
    await expect(page.getByTestId('open-test-node')).toHaveCount(0)
  })

  test('K3: on a phone the node sheet takes the focus and gives it back to the outline item', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 })
    await open(page, `tab=runs&run=${FAILED}`, '.ae-runview .ae-outline')
    const item = page.locator('[data-pick="send_email"]')
    await item.focus()
    await page.keyboard.press('Enter')
    const sheet = page.getByRole('dialog', { name: 'Send email' })
    await expect(sheet).toBeVisible()
    await expect(sheet.locator('.ae-panel__title')).toBeFocused()
    // Tab stays inside the sheet.
    for (let i = 0; i < 12; i++) await page.keyboard.press('Tab')
    expect(await sheet.evaluate((el) => el.contains(document.activeElement))).toBe(true)
    await page.keyboard.press('Escape')
    await expect(sheet).toHaveCount(0)
    await expect(item).toBeFocused()
  })

  test('K3: a link that names a node opens the sheet with the focus in it; Shift+Tab from the title stays inside', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 })
    await open(page, `tab=runs&run=${FAILED}&node=send_email`, '.ae-runview .ae-outline')
    const sheet = page.getByRole('dialog', { name: 'Send email' })
    await expect(sheet.locator('.ae-panel__title')).toBeFocused()
    await page.keyboard.press('Shift+Tab')
    expect(await sheet.evaluate((el) => el.contains(document.activeElement))).toBe(true)
    await page.keyboard.press('Shift+Tab')
    expect(await sheet.evaluate((el) => el.contains(document.activeElement))).toBe(true)
  })

  test('RV10: after Retry the actions wait for the run\'s new state: no second Retry meanwhile', async ({ page }) => {
    await open(page, 'tab=runs&run=00000000-0000-4000-8000-0000000000r4&slowread=1', '.ae-runview')
    await page.getByTestId('run-retry').click()
    await expect(page.getByText('The call is repeated')).toBeVisible()
    await expect(page.getByTestId('run-retry')).toHaveCount(0)
    await expect(page.getByTestId('run-resolve')).toHaveCount(0)
    await expect(page.getByTestId('run-cancel')).toHaveCount(0)
    // The read comes back: the call is still blocked here, so the actions return.
    await expect(page.getByTestId('run-retry')).toBeVisible({ timeout: 6000 })
    expect(await page.evaluate(() => window.__fixture.ops.filter((o) => o.startsWith('retry')).length)).toBe(1)
  })

  test('A1: the Runs tab with a run and its drawer has no serious violations', async ({ page }) => {
    await open(page, `tab=runs&run=${FAILED}&node=send_email`, '.ae-runview')
    await expect(page.getByTestId('run-node')).toBeVisible()
    await expectNoA11yViolations(page, { include: ['.ae-runs'] })
  })
})

test.describe('dry run and test node', () => {
  test('RV7: Dry run draws the path, names the failed trigger condition and lists what would be sent', async ({ page }) => {
    await open(page)
    await page.getByTestId('open-dry-run').click()
    const panel = page.getByTestId('dry-run')
    await expect(panel).toContainText('Nothing is sent outside')
    await panel.getByRole('radio', { name: 'Live call' }).check()
    await panel.getByTestId('dry-run-start').click()
    await expect(panel.getByTestId('dry-trace')).toContainText('The trigger did not fire')
    await expect(panel.getByTestId('dry-condition')).toHaveText('Condition: event.hour == 20')
    await expect(panel.getByTestId('dry-effects')).toContainText('dilnoza@acme.test')
    await expect(panel).toContainText('1 live decision')
    const sent = await page.evaluate(() => window.__fixture.dryRuns[0] as { decisions: string })
    expect(sent.decisions).toBe('live')
    await expect(card(page, 'has_email')).toHaveClass(/ae-card--passed/)
    await expect(card(page, 'schedule')).toHaveClass(/ae-card--skipped/)
    // An edit takes the result off the canvas: it belongs to the draft it ran on.
    await card(page, 'has_email').click()
    await page.locator('.ae-panel [data-param="expression"] textarea').first().fill('true')
    await expect(card(page, 'has_email')).not.toHaveClass(/ae-card--passed/)
    await expectNoA11yViolations(page, { include: ['[data-testid="dry-run"]'] })
  })

  test('RV7: invalid events are refused in place', async ({ page }) => {
    await open(page)
    await page.getByTestId('open-dry-run').click()
    await page.getByTestId('dry-events').fill('{ not json')
    await page.getByTestId('dry-run-start').click()
    await expect(page.getByTestId('dry-run')).toContainText('This is not a JSON array of objects.')
    expect(await page.evaluate(() => window.__fixture.dryRuns.length)).toBe(0)
  })

  test('RV8: Test node confirms an irreversible effect with the focus on Cancel; a secret not set is marked', async ({ page }) => {
    await open(page, 'test=secret')
    await card(page, 'send_email').click()
    await page.getByTestId('open-test-node').click()
    await page.getByTestId('test-start').click()
    const dialog = page.getByTestId('test-confirm')
    await expect(dialog).toContainText('The step calls Send email.')
    await expect(dialog.getByTestId('test-irreversible')).toHaveText(/The effect is irreversible/)
    await expect(dialog.getByRole('button', { name: 'Cancel' })).toBeFocused()
    await page.keyboard.press('Escape')
    expect(await page.evaluate(() => window.__fixture.tests.length)).toBe(0)
    await page.getByTestId('test-start').click()
    await page.getByTestId('test-confirm').getByRole('button', { name: 'Run for real' }).click()
    await expect(page.getByTestId('test-error')).toHaveText('Secret crm.token is not set')
    await expect(card(page, 'send_email')).toHaveClass(/ae-card--failed/)
  })

  test('RV8: the test\'s event carries its kind; «From the last run» fills the event of the newest run that reached the step', async ({ page }) => {
    await open(page)
    await card(page, 'send_email').click()
    await page.getByTestId('open-test-node').click()
    await page.getByTestId('test-example').click()
    await expect(page.getByTestId('test-node').locator('input[type="number"]').first()).toHaveValue('42')
    await page.getByTestId('test-start').click()
    await page.getByTestId('test-confirm').getByRole('button', { name: 'Run for real' }).click()
    await expect(page.getByTestId('test-result')).toContainText('sent')
    const sent = await page.evaluate(() => window.__fixture.tests[0] as { context?: { event?: { kind?: string; user?: { id?: number } } }; idempotency_key?: string })
    expect(sent.context?.event?.kind).toBe('core.user.created.v1')
    expect(sent.context?.event?.user?.id).toBe(42)
    expect(sent.idempotency_key).toBeTruthy()
  })

  test('RV8 / F2: a long test says it is still running and does not run twice', async ({ page }) => {
    await open(page, 'test=slow')
    await card(page, 'send_email').click()
    await page.getByTestId('open-test-node').click()
    await page.getByTestId('test-start').click()
    const confirm = page.getByTestId('test-confirm').getByRole('button', { name: 'Run for real' })
    await confirm.click()
    await expect(page.getByTestId('test-confirm')).toContainText('Still waiting for the step. It will not run twice.', { timeout: 6000 })
    await expect(page.getByTestId('test-result')).toContainText('sent', { timeout: 8000 })
    expect(await page.evaluate(() => window.__fixture.tests.length)).toBe(1)
    await expect(page.getByTestId('test-result')).toContainText('token ***')
  })
})
