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
  await expect(form.getByTestId('form-error')).toContainText('The service refused the connection: Unauthorized: the service does not accept this token.')
  await expect(form.getByTestId('form-error')).toContainText('Check the values and save again.')
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
  expect(update!.params).toEqual({ id: SALES_ID, version: 1, display_name: 'Sales bot (main)' })
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
  await expect(row(page, 'support_bot').getByTestId('test-result')).toContainText('Check failed: Unauthorized: the token was revoked.')
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

test('IN2: OAuth returns to the same form with its result', async ({ page }) => {
  await open(page)
  await page.getByTestId('connect-google_drive').click()
  const form = page.getByTestId('connection-form')
  await expect(form.getByTestId('form-save')).toHaveText('Sign in with Google Drive')
  await form.getByTestId('form-save').click()
  await expect(page).toHaveURL(/connected=/)
  await expect(page.getByTestId('connection-form')).toContainText('The account is connected.')

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
