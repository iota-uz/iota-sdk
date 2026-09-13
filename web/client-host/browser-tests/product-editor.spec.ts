import { expect, test } from '@playwright/test'

test('preserves nested input and field errors, then accepts the server save', async ({ page }) => {
  await page.goto('/')
  await page.getByLabel('Factor').fill('0.5')
  await page.getByRole('button', { name: 'Save changes' }).click()
  await expect(page.getByText('Factor must be at least 1')).toBeFocused()
  await expect(page.getByLabel('Factor')).toHaveValue('0.5')
  await expect(page.getByText('Unsaved')).toBeVisible()
  await page.getByLabel('Factor').fill('5')
  await page.getByRole('button', { name: 'Save changes' }).click()
  await expect(page.getByText('Saved', { exact: true })).toBeVisible()
  await expect(page.getByText('12,500')).toBeVisible()
})

test('cancels save and blocks dirty navigation without losing input', async ({ page }) => {
  await page.goto('/?theme=dark')
  await page.getByLabel('Name').fill('Draft after cancellation')
  await page.getByRole('button', { name: 'Save changes' }).click()
  await page.getByRole('button', { name: 'Cancel save' }).click()
  await expect(page.getByLabel('Name')).toHaveValue('Draft after cancellation')
  await page.locator('[data-next]').click()
  await expect(page).not.toHaveURL(/#next$/)
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
})

test('remains usable in a narrow keyboard-driven viewport', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/')
  await page.keyboard.press('Tab')
  await expect(page.getByLabel('Name')).toBeFocused()
  await expect(page.getByLabel('Calculated preview')).toBeVisible()
})
