import { expect, test, type Page } from '@playwright/test'
import { expectNoA11yViolations } from '../../../../../../web/testing/src/a11y'

/**
 * The real screen mounted by the standard Solid host, with the fake RPC server of the fixture
 * (fixtures/integrations/fake.ts): the same envelope, results and refusals as the Go controller. The
 * Go tests own the contract; these scenarios check what only a browser can (checklist IN1–IN8 of
 * issue #1334 and the criteria of docs/automation-ui-standard.md they name).
 */

const SALES_ID = '00000000-0000-4000-8000-000000000001'

type Call = { method: string; params: Record<string, unknown> }
const calls = (page: Page) => page.evaluate(() => (window as unknown as { __calls: Call[] }).__calls)
const callsOf = async (page: Page, method: string) => (await calls(page)).filter((c) => c.method === `automations.${method}`)

async function open(page: Page, query = '') {
  await page.goto('/' + (query ? `?${query}` : ''))
  await expect(page.getByTestId('integrations-loading')).toHaveCount(0)
}

const row = (page: Page, name: string) => page.getByTestId(`connection-${name}`)
const menu = async (page: Page, name: string) => {
  await row(page, name).locator('summary[aria-haspopup]').click()
}

test('IN1: cards by category with status in words and an icon; states', async ({ page }) => {
  await open(page)
  await expect(page.getByRole('heading', { level: 2 })).toHaveText(['Messaging', 'Email', 'AI', 'Documents'])
  const card = page.getByTestId('connector-telegram_bot')
  await expect(card.getByRole('heading', { name: 'Telegram bot' })).toBeVisible()
  await expect(card.getByText('API key')).toBeVisible()
  await expect(row(page, 'sales_bot').locator('[data-status="connected"]')).toHaveText('Connected')
  await expect(row(page, 'sales_bot').locator('[data-status="connected"] svg')).toHaveCount(1)
  await expect(row(page, 'support_bot').locator('[data-status="error"]')).toHaveText('Error')
  await expect(row(page, 'office_mail').locator('[data-status="disabled"]')).toHaveText('Disabled')
  await expect(row(page, 'drive').locator('[data-status="needs_reauth"]')).toHaveText('Sign-in required')
  await expect(row(page, 'drive').getByTestId('reauth')).toHaveText('Sign in again')
  await expect(page.getByTestId('connector-openai').getByTestId('no-connections')).toContainText('Ask a model, Classify text')
  // C1: no raw id or code in the DOM.
  const text = await page.locator('[data-testid="integrations"]').innerText()
  expect(text).not.toMatch(/[0-9a-f]{8}-[0-9a-f]{4}-/)
  await expectNoA11yViolations(page)

  await open(page, 'scenario=empty')
  await expect(page.getByTestId('integrations-empty')).toContainText('Integrations are not set up in this application')

  await page.goto('/?scenario=error')
  const failure = page.getByTestId('integrations-load-error')
  await expect(failure).toContainText('Could not load integrations.')
  await expect(failure.getByRole('button', { name: 'Retry' })).toBeVisible()
})

test('a bot another service uses is connected only after the person confirms it', async ({ page }) => {
  await open(page)
  await page.getByTestId('connect-telegram_bot').click()
  const form = page.getByTestId('connection-form')
  await form.getByLabel('Name', { exact: true }).fill('Shared bot')
  await form.locator('[data-field="bot_token"]').fill('foreign')
  await form.getByTestId('form-save').click()
  // The warning says what connecting does to the other service; nothing is saved yet.
  const warning = form.getByTestId('form-warnings')
  await expect(warning).toContainText('already used by another service')
  await expect(warning).toContainText('will stop receiving messages')
  await expect(form).toBeVisible()
  expect(await callsOf(page, 'connections.create')).toHaveLength(1)

  // Without the confirmation the save does not go again.
  await form.getByTestId('form-save').click()
  await expect(form.getByTestId('form-error')).toContainText('Confirm that you want to connect this bot')
  expect(await callsOf(page, 'connections.create')).toHaveLength(1)

  await form.getByText('Connect anyway').click()
  await expect(form.getByTestId('form-confirm')).toBeChecked()
  await form.getByTestId('form-save').click()
  await expect(form).toHaveCount(0)
  const created = await callsOf(page, 'connections.create')
  expect(created).toHaveLength(2)
  expect(created[1]!.params.confirm).toEqual(['foreign_webhook'])
  await expect(row(page, 'shared_bot')).toBeVisible()
})

test('S2: loading is a skeleton of the cards', async ({ page }) => {
  await page.goto('/?scenario=loading')
  await expect(page.getByTestId('integrations-loading')).toBeVisible()
  await expect(page.getByTestId('integrations-loading')).toHaveAttribute('aria-busy', 'true')
})

test('IN2/IN3: a refused test keeps the form and its values; save is one request; the secret is write-only', async ({ page }) => {
  await open(page, 'delay=1500')
  await page.getByTestId('connect-telegram_bot').click()
  const form = page.getByTestId('connection-form')
  await form.getByLabel('Name', { exact: true }).fill('Новый бот')
  await expect(form.locator('[data-field="name"]')).toHaveValue('novyy_bot')
  const token = form.locator('[data-field="bot_token"]')
  await expect(token).toHaveAttribute('type', 'password')
  await form.getByRole('button', { name: 'Show' }).click()
  await expect(token).toHaveAttribute('type', 'text')
  await token.fill('bad')

  // Required field errors sit next to the field, before any request.
  await token.fill('')
  await form.getByTestId('form-save').click()
  await expect(form.getByText('Fill in this field.')).toBeVisible()
  expect(await callsOf(page, 'connections.create')).toHaveLength(0)

  await token.fill('bad')
  await form.getByTestId('form-save').click()
  await form.getByTestId('form-save').click({ force: true })
  await expect(form.getByTestId('form-save')).toContainText('Checking the connection…')
  // F4: a busy drawer does not close.
  await page.keyboard.press('Escape')
  await expect(form).toBeVisible()
  await expect(form.getByTestId('form-error')).toHaveText('The service refused the connection: the service did not accept the credentials. Check the key or the password, or issue a new one in the service, and replace it.')
  // C1/C2: a phrase and a fix, never the provider's text (none reaches the page).
  await expect(form.getByTestId('form-error')).not.toContainText('401')
  await expect(page.getByTestId('service-answer')).toHaveCount(0)
  expect(await callsOf(page, 'connections.create')).toHaveLength(1)
  await expect(token).toHaveValue('bad')

  await token.fill('good-token')
  await form.getByTestId('form-save').click()
  await expect(form).toHaveCount(0)
  await expect(page.getByText('Новый бот is connected')).toBeVisible()
  await expect(row(page, 'novyy_bot')).toContainText('@new_bot')
  // The create request carried the secret once; nothing on screen shows it.
  await expect(page.locator('body')).not.toContainText('good-token')
})

test('F2: a slow check says the screen is still waiting', async ({ page }) => {
  test.setTimeout(30_000)
  await open(page, 'delay=6000')
  await page.getByTestId('connect-telegram_bot').click()
  const form = page.getByTestId('connection-form')
  await form.locator('[data-field="bot_token"]').fill('good-token')
  await form.getByTestId('form-save').click()
  await expect(form.getByText('Still waiting for the service to answer. Nothing will be saved twice.')).toBeVisible({ timeout: 5_500 })
  await expect(form).toHaveCount(0, { timeout: 10_000 })
})

test('IN2: editing keeps a set secret unless it is replaced', async ({ page }) => {
  await open(page)
  await menu(page, 'sales_bot')
  await row(page, 'sales_bot').getByTestId('menu-edit').click()
  const form = page.getByTestId('connection-form')
  await expect(form.getByTestId('secret-set-bot_token')).toHaveText('••••• set')
  await expect(form.locator('[data-field="chat_id"]')).toHaveValue('-1001234567890')
  await expect(form.locator('[data-field="name"]')).toBeDisabled()
  await form.getByLabel('Name', { exact: true }).fill('Sales bot (main)')
  await form.getByTestId('form-save').click()
  await expect(form).toHaveCount(0)
  const [update] = await callsOf(page, 'connections.update')
  expect(update!.params).toMatchObject({ id: SALES_ID, version: 1, display_name: 'Sales bot (main)' })
  expect(update!.params.credentials).toBeUndefined()
  expect(update!.params.idempotency_key).toBeTruthy()
  await expect(page).not.toHaveURL(/edit=/)
})

test('D1: the open drawer is in the URL and Back closes it', async ({ page }) => {
  await open(page)
  await page.getByTestId('connect-smtp').click()
  await expect(page).toHaveURL(/connect=smtp/)
  await page.goBack()
  await expect(page.getByTestId('connection-form')).toHaveCount(0)
  await page.goto(`/?edit=${SALES_ID}`)
  await expect(page.getByTestId('connection-form')).toContainText('Edit Sales bot')
})

test('IN3: check says who it is connected as, or why not', async ({ page }) => {
  await open(page)
  await menu(page, 'sales_bot')
  await row(page, 'sales_bot').getByTestId('menu-test').click()
  await expect(row(page, 'sales_bot').getByTestId('test-result')).toHaveText('Connected as @acme_sales_bot')
  await menu(page, 'support_bot')
  await row(page, 'support_bot').getByTestId('menu-test').click()
  await expect(row(page, 'support_bot').getByTestId('test-result')).toHaveText('Check failed: the service did not accept the credentials. Check the key or the password, or issue a new one in the service, and replace it.')
  // The stored status reason is a phrase with a fix, never the provider's text.
  await expect(row(page, 'support_bot').getByTestId('status-reason')).toHaveText('The service did not accept the credentials. Check the key or the password, or issue a new one in the service, and replace it.')
})

test('IN3/F1–F3: a slow check says so; a lost answer offers Retry', async ({ page }) => {
  test.setTimeout(30_000)
  await open(page, 'testDelay=5000&testFailOnce')
  await menu(page, 'sales_bot')
  await row(page, 'sales_bot').getByTestId('menu-test').click()
  const status = row(page, 'sales_bot').getByTestId('check-status')
  await expect(status).toContainText('Checking…')
  await expect(status.locator('svg.animate-spin')).toBeVisible()
  await expect(row(page, 'sales_bot').getByTestId('test-slow')).toHaveText('Still waiting for the service. The check is safe to repeat.', { timeout: 5_000 })
  await expect(row(page, 'sales_bot').getByTestId('test-result')).toHaveText('Could not check the connection. Try again in a minute.', { timeout: 10_000 })
  await row(page, 'sales_bot').getByTestId('test-retry').click()
  await expect(row(page, 'sales_bot').getByTestId('test-result')).toHaveText('Connected as @acme_sales_bot', { timeout: 10_000 })
  expect(await callsOf(page, 'connections.test')).toHaveLength(2)
})

test('F3: Retry after a lost answer resends the same key and creates one connection', async ({ page }) => {
  await open(page, 'lossy')
  await page.getByTestId('connect-telegram_bot').click()
  const form = page.getByTestId('connection-form')
  await form.locator('[data-field="display_name"]').fill('Lost bot')
  await form.locator('[data-field="bot_token"]').fill('good-token')
  await form.getByTestId('form-save').click()
  await expect(form.getByTestId('form-error')).toHaveText('Could not save. Try again in a minute.')
  await expect(form.getByTestId('form-save')).toHaveText('Retry')
  await form.getByTestId('form-save').click()
  await expect(form).toHaveCount(0)
  const creates = await callsOf(page, 'connections.create')
  expect(creates).toHaveLength(2)
  expect(creates[0]!.params.idempotency_key).toBeTruthy()
  expect(creates[1]!.params.idempotency_key).toBe(creates[0]!.params.idempotency_key)
  await expect(page.locator('[data-testid="connection-lost_bot"]')).toHaveCount(1)
})

test('F3: a draft changed before Retry of a lost create says how to recover', async ({ page }) => {
  await open(page, 'lossy')
  await page.getByTestId('connect-telegram_bot').click()
  const form = page.getByTestId('connection-form')
  await form.locator('[data-field="display_name"]').fill('Lost bot')
  await form.locator('[data-field="bot_token"]').fill('good-token')
  await form.getByTestId('form-save').click()
  await expect(form.getByTestId('form-save')).toHaveText('Retry')
  await form.locator('[data-field="display_name"]').fill('Lost bot 2')
  await form.getByTestId('form-save').click()
  await expect(form.getByTestId('form-error')).toHaveText('The first save may have gone through with the earlier values. Close the form and edit the connection from the list.')
  await expect(form.locator('[data-field="idempotency_key"]')).toHaveCount(0)
  // The list shows what the first request saved.
  await expect(page.locator('[data-testid="connection-lost_bot"]')).toHaveCount(1)
})

for (const mode of ['result', 'error'] as const) {
test(`F2: a version conflict (${mode}) offers the latest version and keeps what was typed`, async ({ page }) => {
  await open(page, `conflictOnce=${mode}`)
  await menu(page, 'sales_bot')
  await row(page, 'sales_bot').getByTestId('menu-edit').click()
  const form = page.getByTestId('connection-form')
  await form.locator('[data-field="display_name"]').fill('My bot')
  await form.getByTestId('form-save').click()
  await expect(form.getByTestId('form-error')).toHaveText('Someone changed this connection meanwhile. Load their version: what you typed stays in the form.')
  await form.getByTestId('form-reload').click()
  await expect(form.getByTestId('form-notice')).toHaveText('The latest version is loaded. Check the form and save again.')
  await expect(form.locator('[data-field="display_name"]')).toHaveValue('My bot')
  await expect(form.locator('[data-field="chat_id"]')).toHaveValue('-1009999', { timeout: 5_000 })
  await form.getByTestId('form-save').click()
  await expect(form).toHaveCount(0)
  const updates = await callsOf(page, 'connections.update')
  expect(updates).toHaveLength(2)
  expect(updates[1]!.params).toMatchObject({ version: 2, display_name: 'My bot' })
  expect(updates[1]!.params.credentials).toBeUndefined()
  expect(updates[1]!.params.idempotency_key).not.toBe(updates[0]!.params.idempotency_key)
})
}

test('D1: history from one edit drawer to another opens a new draft', async ({ page }) => {
  await open(page)
  await menu(page, 'sales_bot')
  await row(page, 'sales_bot').getByTestId('menu-edit').click()
  const form = page.getByTestId('connection-form')
  await form.locator('[data-field="display_name"]').fill('Draft for sales')
  await page.evaluate(() => {
    window.history.pushState({}, '', '?edit=00000000-0000-4000-8000-000000000002')
    window.dispatchEvent(new PopStateEvent('popstate'))
  })
  await expect(form).toContainText('Edit Support bot')
  await expect(form.locator('[data-field="display_name"]')).toHaveValue('Support bot')
})

test('F8: the platform switch waits for its request and returns to the saved value', async ({ page }) => {
  await open(page, 'platformFails&delay=400')
  const panel = page.getByTestId('connector-openai').getByTestId('platform-key')
  const sw = panel.getByRole('switch', { name: 'Use the platform key' })
  await panel.getByText('Use the platform key', { exact: true }).click()
  await expect(sw).toBeChecked()
  await expect(sw).toBeDisabled()
  await expect(sw).not.toBeChecked({ timeout: 5_000 })
  await expect(sw).toBeEnabled()
  expect(await callsOf(page, 'platform.set_enabled')).toHaveLength(1)
})

test('IN4: a used connection lists its uses with editor links and cannot be deleted', async ({ page }) => {
  await open(page)
  await row(page, 'sales_bot').getByTestId('used-by').click()
  const dialog = page.getByTestId('usage-dialog')
  await expect(dialog.getByRole('link', { name: 'New order → notify sales' })).toHaveAttribute('href', '/automations/a1b2c3d4-0000-4000-8000-000000000001?node=notify')
  await expect(dialog).toContainText('version 3')
  await expect(dialog).toContainText('enabled')
  await dialog.getByRole('button', { name: 'Close' }).click()
  await menu(page, 'sales_bot')
  const del = row(page, 'sales_bot').getByTestId('menu-delete')
  await expect(del).toBeDisabled()
  await expect(row(page, 'sales_bot')).toContainText('Used in automations: disable it instead of deleting.')
  await expectNoA11yViolations(page)
})

test('IN5/U1: deleting asks with the name and the nodes, focus on Cancel', async ({ page }) => {
  await open(page)
  await menu(page, 'support_bot')
  await row(page, 'support_bot').getByTestId('menu-delete').click()
  const dialog = page.getByTestId('delete-dialog')
  await expect(dialog).toContainText('Delete Support bot?')
  await expect(dialog).toContainText('Send a Telegram message, Ask in Telegram')
  await expect(dialog.getByRole('button', { name: 'Cancel' })).toBeFocused()
  await dialog.getByRole('button', { name: 'Delete connection' }).click()
  await expect(dialog).toHaveCount(0)
  await expect(row(page, 'support_bot')).toHaveCount(0)
  await expect(page.getByText('Support bot is deleted')).toBeVisible()
})

test('U1: disabling is undone from the toast', async ({ page }) => {
  await open(page)
  await menu(page, 'sales_bot')
  await row(page, 'sales_bot').getByTestId('menu-toggle').click()
  await expect(row(page, 'sales_bot').locator('[data-status="disabled"]')).toBeVisible()
  await page.getByRole('button', { name: 'Undo' }).click()
  await expect(row(page, 'sales_bot').locator('[data-status="connected"]')).toBeVisible()
})

test('F8: an accepted platform switch stays when the refresh after it fails', async ({ page }) => {
  await open(page, 'refreshFails')
  const panel = page.getByTestId('connector-openai').getByTestId('platform-key')
  const sw = panel.getByRole('switch', { name: 'Use the platform key' })
  await panel.getByText('Use the platform key', { exact: true }).click()
  await expect(sw).toBeEnabled()
  // The refresh after the success failed: the screen says so, and the switch keeps the accepted value.
  await expect(page.getByText('Could not load integrations.').first()).toBeVisible({ timeout: 5_000 })
  await expect(sw).toBeChecked()
  expect(await callsOf(page, 'platform.set_enabled')).toHaveLength(1)
})

test('IN6/F8: the platform key switch rolls back when the server refuses', async ({ page }) => {
  await open(page, 'platformFails')
  const panel = page.getByTestId('connector-openai').getByTestId('platform-key')
  await expect(panel).toContainText('Platform key')
  const toggle = panel.getByRole('switch', { name: 'Use the platform key' })
  await expect(toggle).not.toBeChecked()
  await panel.getByText('Use the platform key', { exact: true }).click()
  await expect(panel.getByRole('alert')).toContainText('Could not change the platform key.')
  await expect(toggle).not.toBeChecked()

  await open(page)
  const ok = page.getByTestId('connector-openai').getByRole('switch', { name: 'Use the platform key' })
  await page.getByTestId('connector-openai').getByText('Use the platform key', { exact: true }).click()
  await expect(ok).toBeChecked()
  expect(await callsOf(page, 'platform.set_enabled')).toEqual([{ method: 'automations.platform.set_enabled', params: { kind: 'openai', enabled: true } }])
})

test('IN2: editing an OAuth connection updates it; Sign in again re-authorizes that connection', async ({ page }) => {
  await open(page)
  await menu(page, 'drive')
  await row(page, 'drive').getByTestId('menu-edit').click()
  const form = page.getByTestId('connection-form')
  await expect(form.getByTestId('form-save')).toHaveText('Save')
  await form.locator('[data-field="display_name"]').fill('Docs drive')
  await form.getByTestId('form-save').click()
  await expect(form).toHaveCount(0)
  expect(await callsOf(page, 'connections.oauth_start')).toHaveLength(0)
  const [update] = await callsOf(page, 'connections.update')
  expect(update!.params).toMatchObject({ id: '00000000-0000-4000-8000-000000000004', display_name: 'Docs drive' })
  expect(update!.params.credentials).toBeUndefined()

  await menu(page, 'drive')
  await row(page, 'drive').getByTestId('menu-edit').click()
  await page.getByTestId('form-reauth').click()
  await expect(page).toHaveURL(/edit=00000000-0000-4000-8000-000000000004/)
  // The fake returns to ?edit=<id> only for a start that named the connection (connection_id).
  await expect(page.getByTestId('connection-form')).toContainText('The account is connected.')
  // The fake keeps its catalog across the provider round trip: the edit and the new sign-in are there.
  await expect(page.getByTestId('connection-form')).toContainText('Edit Docs drive')
  await page.getByTestId('connection-form').getByRole('button', { name: 'Cancel' }).click()
  await expect(row(page, 'drive').locator('[data-status="connected"]')).toHaveText('Connected')
})

test('IN2: a failed OAuth return reopens the form it came from', async ({ page }) => {
  await open(page, 'oauthFail=refused')
  await page.getByTestId('connect-google_drive').click()
  const form = page.getByTestId('connection-form')
  await form.locator('[data-field="display_name"]').fill('Team drive')
  await form.getByTestId('form-save').click()
  await expect(page).toHaveURL(/oauth_error=refused/)
  const back = page.getByTestId('connection-form')
  await expect(back.getByTestId('oauth-error')).toHaveText('The service signed you in, but the account cannot be used for this connector. Sign in with another account.')
  await expect(back.locator('[data-field="display_name"]')).toHaveValue('Team drive')
  await expect(back.locator('[data-field="name"]')).toHaveValue('team_drive')

  // A failed re-authorization returns to the edit form of that connection.
  await open(page, 'oauthFail=exchange')
  await row(page, 'drive').getByTestId('reauth').click()
  await expect(page.getByTestId('connection-form')).toContainText('Edit Company drive')
  await expect(page.getByTestId('oauth-error')).toHaveText('The service did not confirm the sign-in. Try again in a minute.')
})

test('IN2: OAuth returns to the same form with its result', async ({ page }) => {
  await open(page)
  await page.getByTestId('connect-google_drive').click()
  const form = page.getByTestId('connection-form')
  await expect(form.getByTestId('form-save')).toHaveText('Sign in with Google Drive')
  await form.getByTestId('form-save').click()
  await expect(page).toHaveURL(/connected=/)
  await expect(page.getByTestId('connection-form')).toContainText('The account is connected.')
  // The new connection is in the list the page loaded after the return.
  const id = new URL(page.url()).searchParams.get('connected')!
  await page.getByTestId('connection-form').getByRole('button', { name: 'Cancel' }).click()
  await expect(page.getByTestId('connector-google_drive').locator('[data-testid^="connection-"]')).toHaveCount(2)
  expect(id).toMatch(/^00000000-0000-4000-8000-0000000001/)

  await page.goto('/?oauth_error=denied&kind=google_drive')
  await expect(page.getByTestId('connection-form').getByRole('alert')).toContainText('Access was not granted.')
})

test('IN7: actions are absent from the DOM without the right', async ({ page }) => {
  await open(page, 'manage=0')
  await expect(page.getByTestId('connector-telegram_bot')).toBeVisible()
  await expect(page.locator('button[data-testid^="connect-"]')).toHaveCount(0)
  await expect(page.locator('summary[aria-haspopup]')).toHaveCount(0)
  await expect(page.getByTestId('platform-switch')).toHaveCount(0)
})

test('IN8/T6: 390 px has one column and no horizontal scroll; the form fills the screen', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await open(page, 'lang=ru')
  // The populated layout, not a page before mount or after a load error, is what is measured.
  await expect(page.locator('article')).toHaveCount(4)
  await expect(page.getByTestId('connector-telegram_bot')).toBeVisible()
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
  expect(overflow).toBeLessThanOrEqual(0)
  const boxes = await page.locator('article').evaluateAll((els) => els.map((e) => e.getBoundingClientRect().width))
  for (const w of boxes) expect(w).toBeGreaterThan(330)
  await page.getByTestId('connect-telegram_bot').click()
  const panel = page.getByTestId('connection-form').locator('[data-dialog-body]')
  await expect(panel).toBeVisible()
  const width = await panel.evaluate((e) => e.getBoundingClientRect().width)
  expect(width).toBeGreaterThanOrEqual(380)
  // The drawer slides in: axe measures contrast of the settled layer.
  await page.evaluate(() => Promise.all(document.getAnimations().map((a) => a.finished)))
  await expectNoA11yViolations(page)
})

test('MCP: a connection with OAuth is saved, then signed in; one that needs it offers «Sign in again»', async ({ page }) => {
  await open(page, 'scenario=mcp')
  await expect(row(page, 'crm_tools').getByTestId('reauth')).toHaveText('Sign in again')
  await expect(row(page, 'docs_search').getByTestId('reauth')).toHaveCount(0)
  await row(page, 'crm_tools').getByTestId('reauth').click()
  // The sign-in names the saved connection (the fake's return then opens its edit form).
  await expect(page).toHaveURL(/connected=.*edit=/)

  await open(page, 'scenario=mcp')
  await page.getByTestId('connect-mcp').click()
  const form = page.getByTestId('connection-form')
  await form.getByLabel('Name', { exact: true }).fill('Billing tools')
  await form.locator('[data-field="server_url"]').fill('https://mcp.billing.example.com/mcp')
  await form.locator('[data-field="auth"]').selectOption('oauth2')
  await form.getByTestId('form-save').click()
  // Saved first, then signed in as that connection: the return names it.
  await expect(page).toHaveURL(/connected=.*kind=mcp&edit=/)
  await expect(page.getByTestId('connection-form')).toContainText('Billing tools')
})
