import { assertCaseCoverage, componentCase, openComponentCase } from '@iota-uz/sdk/testing'
import { expect, test } from '@playwright/test'
import { backend, productEditorCases, productEditorHost } from './product-editor.host'

const open = async (page: import('@playwright/test').Page, id: string) =>
  openComponentCase(page, productEditorHost, await componentCase(productEditorHost, id))

test('cases cover every state, locale, theme and viewport', () => {
  assertCaseCoverage(productEditorCases, {
    states: ['editing', 'loading', 'empty', 'error', 'stale'],
    locales: ['en', 'ru'],
    themes: ['light', 'dark'],
    viewports: [{ width: 1280, height: 800 }, { width: 390, height: 844 }],
  })
})

test('editing: saves through the gated backend and shows the server value', async ({ page }) => {
  await open(page, 'editing-en-light-desktop')
  await expect(page.getByText('10,000')).toBeVisible()
  await page.getByLabel('Factor').fill('5')
  await page.getByRole('button', { name: 'Save changes' }).click()
  await expect(page.getByRole('button', { name: 'Saving…' })).toBeDisabled()
  await backend.respond(page)
  await expect(page.getByText('Saved', { exact: true })).toBeVisible()
  await expect(page.getByText('12,500')).toBeVisible()
})

test('loading: stays pending until the backend answers, then settles', async ({ page }) => {
  await open(page, 'loading-ru-dark-narrow')
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await expect(page.locator('html')).toHaveAttribute('lang', 'ru')
  await expect(page.getByRole('button', { name: 'Saving…' })).toBeDisabled()
  await expect(page.getByRole('button', { name: 'Cancel save' })).toBeEnabled()
  await expect(page.getByLabel('Calculated preview')).toContainText('10 000')
  await backend.respond(page)
  await expect(page.getByText('Saved', { exact: true })).toBeVisible()
  await expect(page.getByLabel('Name')).toHaveValue('Pending draft')
})

test('loading: cancellation keeps the input and blocks dirty navigation', async ({ page }) => {
  await open(page, 'loading-ru-dark-narrow')
  await page.getByRole('button', { name: 'Cancel save' }).click()
  expect(await backend.pending(page)).toBe(0)
  await expect(page.getByLabel('Name')).toHaveValue('Pending draft')
  await expect(page.getByText('Unsaved')).toBeVisible()
  await page.locator('[data-next]').click()
  await expect(page).not.toHaveURL(/#next$/)
})

test('empty: preview explains missing input instead of NaN, then recovers', async ({ page }) => {
  await open(page, 'empty-en-dark-desktop')
  const preview = page.getByLabel('Calculated preview')
  await expect(preview).toHaveAttribute('data-empty', 'true')
  await expect(preview).toContainText('Enter a factor and base amount')
  await expect(preview).not.toContainText('NaN')
  await page.getByLabel('Base amount').fill('1000')
  await expect(preview).toHaveAttribute('data-empty', 'false')
  await expect(preview).toContainText('4,000')
})

test('error: server refusal focuses the field error and a corrected save succeeds', async ({ page }) => {
  await open(page, 'error-ru-light-narrow')
  await expect(page.getByText('Factor must be at least 1')).toBeFocused()
  await expect(page.getByLabel('Factor')).toHaveAttribute('aria-invalid', 'true')
  await expect(page.getByLabel('Factor')).toHaveValue('0.5')
  await page.getByLabel('Factor').fill('2')
  await expect(page.getByText('Factor must be at least 1')).toHaveCount(0)
  await page.getByRole('button', { name: 'Save changes' }).click()
  await backend.respond(page)
  await expect(page.getByText('Saved', { exact: true })).toBeVisible()
})

test('stale: a response for an older snapshot does not overwrite newer input', async ({ page }) => {
  await open(page, 'stale-en-light-narrow')
  await backend.respond(page)
  await expect(page.getByRole('button', { name: 'Save changes' })).toBeEnabled()
  await expect(page.getByLabel('Name')).toHaveValue('Edited while saving')
  await expect(page.getByText('Unsaved')).toBeVisible()
  await page.getByRole('button', { name: 'Save changes' }).click()
  await backend.respond(page)
  await expect(page.getByText('Saved', { exact: true })).toBeVisible()
  await expect(page.getByLabel('Name')).toHaveValue('Edited while saving')
})

test('keyboard: the narrow layout is operable without a pointer', async ({ page }) => {
  await open(page, 'error-ru-light-narrow')
  await page.getByLabel('Name').focus()
  await page.keyboard.press('Tab')
  await expect(page.getByLabel('Factor')).toBeFocused()
  await page.keyboard.press('Control+A')
  await page.keyboard.type('3')
  await page.keyboard.press('Tab')
  await expect(page.getByLabel('Base amount')).toBeFocused()
  await page.keyboard.press('Tab')
  await expect(page.getByRole('button', { name: 'Save changes' })).toBeFocused()
  await page.keyboard.press('Enter')
  await backend.respond(page)
  await expect(page.getByText('Saved', { exact: true })).toBeVisible()
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
  expect(overflow).toBe(0)
})
