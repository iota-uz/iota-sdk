import { expect, test, type Page } from '@playwright/test'
import { SPECIMENS, THEMES, type SpecimenState } from '../src/catalog'

const viewports = {
  desktop: { width: 1440, height: 900 },
  mobile: { width: 390, height: 844 },
} as const

async function settle(page: Page): Promise<void> {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await expect(page.locator('html')).toHaveAttribute('data-gallery-ready', 'true')
  await page.evaluate(async () => {
    await document.fonts.ready
    await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())))
  })
  await page.mouse.move(0, 0)
}

async function prepareState(page: Page, state: SpecimenState, specimenID: string): Promise<void> {
  if (state === 'hover') await page.locator('[data-vr-target]').first().hover()
  if (state === 'focus' && specimenID === 'selection') await page.locator('[data-vr-target="selection-combobox"] [role="combobox"]').focus()
  else if (state === 'focus' && specimenID === 'data') await page.locator('tbody tr[tabindex]').first().focus()
  else if (state === 'focus') await page.locator('[data-vr-target]').first().focus()
  if (state === 'open' && specimenID === 'selection') await page.locator('[data-vr-target="selection-combobox"] [role="combobox"]').focus()
}

for (const specimen of SPECIMENS) {
  for (const theme of THEMES) {
    for (const state of specimen.states) {
      test(`${specimen.id} ${theme} ${state}`, async ({ page }) => {
        const query = new URLSearchParams({ specimen: specimen.id, theme, state })
        await page.goto(`/?${query.toString()}`, { waitUntil: 'networkidle' })
        await expect(page.locator('main')).toHaveAttribute('data-specimen-id', specimen.id)
        await expect(page.locator('main')).toHaveAttribute('data-theme', theme)
        await prepareState(page, state, specimen.id)
        await settle(page)
        await expect(page).toHaveScreenshot(`${specimen.id}-${theme}-${state}-desktop.png`, { fullPage: true })
      })
    }

    test(`${specimen.id} ${theme} responsive`, async ({ page }) => {
      await page.setViewportSize(viewports.mobile)
      const query = new URLSearchParams({ specimen: specimen.id, theme, state: 'default' })
      await page.goto(`/?${query.toString()}`, { waitUntil: 'networkidle' })
      await settle(page)
      await expect(page).toHaveScreenshot(`${specimen.id}-${theme}-default-mobile.png`, { fullPage: true })
    })
  }
}

test.describe('templ geometry contracts', () => {
  test('forms and display preserve canonical dimensions and styles', async ({ page }) => {
    await page.goto('/?specimen=form-controls&theme=light&state=default', { waitUntil: 'networkidle' })
    await settle(page)
    const input = page.locator('[data-vr-target="form-input"]')
    await expect(input).toHaveCSS('border-radius', '8px')
    await expect(input).toHaveCSS('font-size', '14px')
    await expect(input).toHaveCSS('font-weight', '500')
    const inputBox = await input.boundingBox()
    expect(inputBox?.height).toBe(41)

    await page.goto('/?specimen=buttons&theme=light&state=default', { waitUntil: 'networkidle' })
    await settle(page)
    const button = page.locator('[data-vr-target="button-primary"]')
    await expect(button).toHaveCSS('border-radius', '8px')
    await expect(button).toHaveCSS('font-size', '14px')
    await expect(button).toHaveCSS('font-weight', '500')
    const buttonBox = await button.boundingBox()
    expect(buttonBox?.height).toBe(42)

    await page.goto('/?specimen=display&theme=light&state=default', { waitUntil: 'networkidle' })
    await settle(page)
    await expect(page.locator('[data-vr-target="display-card"]')).toHaveCSS('border-radius', '8px')

    await page.goto('/?specimen=advanced-forms&theme=light&state=checked', { waitUntil: 'networkidle' })
    await settle(page)
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
    await page.goto('/?specimen=navigation&theme=light&state=open', { waitUntil: 'networkidle' })
    await settle(page)
    await expect(page.locator('summary[aria-label="Product actions"]')).toHaveAttribute('aria-expanded', 'true')
    await expect(page.getByRole('menu')).toBeVisible()

    await page.goto('/?specimen=dialog&theme=light&state=open', { waitUntil: 'networkidle' })
    await settle(page)
    await expect(page.getByRole('dialog')).toHaveAttribute('aria-modal', 'true')
    await expect(page.getByRole('heading', { name: 'Delete insurance product?' })).toBeVisible()

    await page.goto('/?specimen=toasts&theme=light&state=open', { waitUntil: 'networkidle' })
    await settle(page)
    await expect(page.getByRole('status')).toHaveCount(1)
    await expect(page.getByRole('alert')).toHaveCount(2)
  })

  test('selection controls expose combobox, listbox, and calendar semantics', async ({ page }) => {
    await page.goto('/?specimen=selection&theme=light&state=default', { waitUntil: 'networkidle' })
    await settle(page)
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
    await page.goto('/?specimen=data&theme=light&state=selected', { waitUntil: 'networkidle' })
    await settle(page)
    await expect(page.locator('tbody tr').first()).toHaveAttribute('aria-selected', 'true')
    await expect(page.getByRole('columnheader', { name: /Product/ })).toHaveAttribute('aria-sort', 'ascending')

    await page.goto('/?specimen=scaffold&theme=light&state=open', { waitUntil: 'networkidle' })
    await settle(page)
    await expect(page.locator('summary[aria-label="More actions"]')).toHaveAttribute('aria-expanded', 'true')
    await expect(page.getByRole('menu')).toBeVisible()
  })
})
