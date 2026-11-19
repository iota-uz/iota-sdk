import { assertCaseCoverage, openComponentCase } from '@iota-uz/sdk/testing'
import { expect, test, type Page } from '@playwright/test'
import { SPECIMENS, THEMES } from '../src/catalog'
import { galleryCase, galleryCases, galleryHost, viewports, type GalleryCase } from './gallery.host'

async function open(page: Page, c: GalleryCase): Promise<void> {
  await openComponentCase(page, galleryHost, c)
  // Screenshot determinism only: readiness itself is the host's data-gallery-ready marker.
  await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))))
  await page.mouse.move(0, 0)
}

test('gallery cases cover every specimen state, theme and viewport', () => {
  assertCaseCoverage(galleryCases, {
    states: [...new Set(SPECIMENS.flatMap(specimen => specimen.states))],
    themes: THEMES,
    viewports: Object.values(viewports),
  })
})

for (const c of galleryCases) {
  test(c.id, async ({ page }) => {
    await open(page, c)
    await expect(page).toHaveScreenshot(c.screenshot, { fullPage: true })
  })
}

test.describe('templ geometry contracts', () => {
  test('forms and display preserve canonical dimensions and styles', async ({ page }) => {
    await open(page, galleryCase('form-controls', 'default', 'light'))
    const input = page.locator('[data-vr-target="form-input"]')
    await expect(input).toHaveCSS('border-radius', '8px')
    await expect(input).toHaveCSS('font-size', '14px')
    await expect(input).toHaveCSS('font-weight', '500')
    const inputBox = await input.boundingBox()
    expect(inputBox?.height).toBe(41)

    await open(page, galleryCase('buttons', 'default', 'light'))
    const button = page.locator('[data-vr-target="button-primary"]')
    await expect(button).toHaveCSS('border-radius', '8px')
    await expect(button).toHaveCSS('font-size', '14px')
    await expect(button).toHaveCSS('font-weight', '500')
    const buttonBox = await button.boundingBox()
    expect(buttonBox?.height).toBe(42)

    await open(page, galleryCase('display', 'default', 'light'))
    await expect(page.locator('[data-vr-target="display-card"]')).toHaveCSS('border-radius', '8px')

    await open(page, galleryCase('advanced-forms', 'checked', 'light'))
    const switchControl = page.getByRole('switch', { name: 'Automatic renewal' })
    await expect(switchControl).toBeChecked()
    const switchTrack = switchControl.locator('xpath=following-sibling::div[1]')
    const switchBox = await switchTrack.boundingBox()
    expect(switchBox?.width).toBe(44)
    expect(switchBox?.height).toBe(24)
  })
})

test.describe('interaction and accessibility contracts', () => {
  test('open controls expose canonical roles and state', async ({ page }) => {
    await open(page, galleryCase('navigation', 'open', 'light'))
    await expect(page.locator('summary[aria-label="Product actions"]')).toHaveAttribute('aria-expanded', 'true')
    await expect(page.getByRole('menu')).toBeVisible()

    await open(page, galleryCase('dialog', 'open', 'light'))
    await expect(page.getByRole('dialog')).toHaveAttribute('aria-modal', 'true')
    await expect(page.getByRole('heading', { name: 'Delete insurance product?' })).toBeVisible()

    await open(page, galleryCase('toasts', 'open', 'light'))
    await expect(page.getByRole('status')).toHaveCount(1)
    await expect(page.getByRole('alert')).toHaveCount(2)
  })

  test('selection controls expose combobox, listbox, and calendar semantics', async ({ page }) => {
    await open(page, galleryCase('selection', 'default', 'light'))
    const comboboxes = page.getByRole('combobox')
    await expect(comboboxes).toHaveCount(5)

    await comboboxes.nth(0).focus()
    await expect(comboboxes.nth(0)).toHaveAttribute('aria-expanded', 'true')
    await expect(page.getByRole('listbox')).toBeVisible()
    await page.keyboard.press('Escape')

    await comboboxes.nth(1).fill('a')
    await expect(page.getByRole('option', { name: 'Acme Insurance LLC' })).toBeVisible()
    await page.keyboard.press('Escape')

    await comboboxes.nth(3).click()
    await expect(comboboxes.nth(3)).toHaveAttribute('aria-haspopup', 'grid')
    await expect(page.getByRole('dialog', { name: 'Choose date' })).toBeVisible()
    await expect(page.getByRole('gridcell')).toHaveCount(42)
  })

  test('data and scaffold states expose selection, sorting, and menus', async ({ page }) => {
    await open(page, galleryCase('data', 'selected', 'light'))
    await expect(page.locator('tbody tr').first()).toHaveAttribute('aria-selected', 'true')
    await expect(page.getByRole('columnheader', { name: /Product/ })).toHaveAttribute('aria-sort', 'ascending')

    await open(page, galleryCase('scaffold', 'open', 'light'))
    await expect(page.locator('summary[aria-label="More actions"]')).toHaveAttribute('aria-expanded', 'true')
    await expect(page.getByRole('menu')).toBeVisible()
  })
})
