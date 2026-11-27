import { expectNoA11yViolations, openComponentCase } from '@iota-uz/sdk/testing'
import { expect, test, type Page } from '@playwright/test'
import { galleryCase, galleryCases, galleryHost, type GalleryCase } from './gallery.host'

async function open(page: Page, c: GalleryCase): Promise<void> {
  await openComponentCase(page, galleryHost, c)
  // Screenshot determinism only: readiness itself is the host's data-gallery-ready marker.
  await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))))
  // Park the pointer unless the case is the hover state the host just applied.
  if (c.state !== 'hover') await page.mouse.move(0, 0)
}

/** Finishes enter transitions so geometry and contrast are read on settled pixels (spinners keep running). */
async function settleAnimations(page: Page): Promise<void> {
  await page.evaluate(() => document.getAnimations().forEach((animation) => {
    if (Number.isFinite(animation.effect?.getComputedTiming().endTime ?? Number.POSITIVE_INFINITY)) animation.finish()
  }))
}

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
  for (const [specimen, selector] of [
    ['form-controls', '#gallery-enabled'],
    ['advanced-forms', '[data-vr-target="advanced-switch"]'],
  ] as const) {
    test(`${specimen} keeps the native toggle inside its scrolled label`, async ({ page }) => {
      await open(page, galleryCase(specimen, 'default', 'light'))
      const input = page.locator(selector)
      const label = input.locator('..')
      await label.evaluate(element => {
        const outlet = document.createElement('div')
        outlet.style.cssText = 'height: 180px; overflow: auto'
        const content = document.createElement('div')
        content.style.padding = '500px 0'
        document.body.replaceChildren(outlet)
        outlet.append(content)
        content.append(element)
        element.scrollIntoView({ block: 'center' })
      })
      const box = await label.boundingBox()
      const control = await input.boundingBox()
      expect(control!.y).toBeGreaterThanOrEqual(box!.y)
      expect(control!.y).toBeLessThanOrEqual(box!.y + box!.height)
      await label.click()
      await expect(input).toBeChecked()
      expect((await label.boundingBox())!.y).toBe(box!.y)
    })
  }

  // createFlash (#1352): flashing again restarts the highlight animation itself, also for a burst of calls
  // made before the browser renders the first removal.
  for (const calls of [1, 2, 3]) {
    test(`a repeated flash restarts the highlight animation (${calls} call${calls > 1 ? 's' : ''} at once)`, async ({ page }) => {
      await open(page, galleryCase('motion', 'default', 'light'))
      const elapsed = () => page.getByTestId('motion-flash').evaluate((card) => {
        const animation = document.getAnimations().find((a) => {
          const effect = a.effect as KeyframeEffect | null
          return effect?.target === card && effect.pseudoElement === '::after'
        })
        return animation ? Number(animation.currentTime ?? -1) : -1
      })
      await expect.poll(elapsed).toBeGreaterThan(1000)
      await page.getByTestId('motion-flash-again').evaluate((button, n) => {
        for (let i = 0; i < n; i++) (button as HTMLButtonElement).click()
      }, calls)
      await expect.poll(elapsed).toBeGreaterThanOrEqual(0)
      expect(await elapsed()).toBeLessThan(600)
    })
  }

  test('open controls expose canonical roles and state', async ({ page }) => {
    await open(page, galleryCase('navigation', 'open', 'light'))
    await expect(page.locator('summary[aria-label="Product actions"]')).toHaveAttribute('aria-expanded', 'true')
    await expect(page.getByRole('menu')).toBeVisible()

    await open(page, galleryCase('dialog', 'open', 'light'))
    await expect(page.getByRole('dialog')).toHaveAttribute('aria-modal', 'true')
    await expect(page.getByRole('heading', { name: 'Delete insurance product?' })).toBeVisible()

    await open(page, galleryCase('toasts', 'open', 'light'))
    await expect(page.getByRole('status')).toHaveCount(2)
    await expect(page.getByRole('alert')).toHaveCount(2)
    await expect(page.getByRole('status').first()).toHaveAttribute('aria-live', 'polite')
    await expect(page.locator('[data-toast-region]')).not.toHaveAttribute('aria-live', /.*/)
    await expect(page.getByRole('button', { name: 'Undo' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Retry' })).toBeVisible()
  })

  test('phone toasts span the width minus gutters and stay above the fixed bottom bar', async ({ page }) => {
    for (const theme of ['light', 'dark'] as const) {
      await open(page, galleryCase('toasts', 'open', theme, 'mobile'))
      const bar = await page.locator('[data-vr-target="toast-bottom-bar"]').boundingBox()
      const toasts = page.locator('[data-toast]')
      await expect(toasts).toHaveCount(4)
      for (const box of await toasts.evaluateAll(nodes => nodes.map(node => node.getBoundingClientRect().toJSON() as DOMRect))) {
        expect(box.x).toBe(12)
        expect(box.width).toBe(390 - 24)
        expect(box.y + box.height).toBeLessThanOrEqual(bar!.y)
      }
    }
  })

  test('toasts keep a light surface with dark text in both themes', async ({ page }) => {
    for (const theme of ['light', 'dark'] as const) {
      await open(page, galleryCase('toasts', 'open', theme))
      const surfaces = await page.locator('[data-toast]').evaluateAll(nodes => nodes.map(node => {
        const style = getComputedStyle(node)
        const title = getComputedStyle(node.querySelector('p')!)
        return { background: style.backgroundColor, title: title.color }
      }))
      expect(surfaces).toHaveLength(4)
      for (const surface of surfaces) {
        expect(surface.background, `${theme} toast surface`).toBe('oklch(1 0 0)')
        expect(surface.title, `${theme} toast title`).toBe('oklch(0.1867 0 0)')
      }
    }
  })

  // The status line is measured before any close attempt, at the phone width
  // where the built-in message wraps, and again after the first refused close.
  for (const specimen of ['dialog', 'drawer'] as const) {
    test(`busy ${specimen} refuses to close without moving its controls`, async ({ page }) => {
      await page.setViewportSize({ width: 390, height: 844 })
      await page.goto(`/?${new URLSearchParams({ specimen, theme: 'light', state: 'busy' })}`)
      await expect(page.locator('html')).toHaveAttribute('data-gallery-ready', 'true')
      const dialog = page.getByRole('dialog')
      await expect(dialog).toHaveAttribute('aria-busy', 'true')
      await settleAnimations(page)
      const status = page.locator('[data-dialog-busy-status] [role="status"]')
      await expect(status).toBeHidden()
      const geometry = () => page.locator('[data-dialog-busy-status], dialog button').evaluateAll(nodes => nodes.map(node => node.getBoundingClientRect().toJSON()))
      const before = await geometry()
      await page.keyboard.press('Escape')
      await expect(status).toBeVisible()
      await expect(status).toContainText('will not be sent twice')
      expect(await geometry()).toEqual(before)
      await page.getByRole('button', { name: 'Close' }).click()
      await expect(dialog).toBeVisible()
      expect(await geometry()).toEqual(before)
    })
  }

  test('a busy layer dims its content but not the loading action, close controls or status', async ({ page }) => {
    await open(page, galleryCase('dialog', 'busy', 'light'))
    await settleAnimations(page)
    const opacity = (locator: ReturnType<Page['locator']>) => locator.evaluate((node) => {
      let value = 1
      for (let element: Element | null = node; element; element = element.parentElement) value *= Number(getComputedStyle(element).opacity)
      return value
    })
    expect(await opacity(page.getByText('This action cannot be undone.'))).toBeCloseTo(0.8)
    expect(await opacity(page.getByRole('heading', { name: 'Delete insurance product?' }))).toBeCloseTo(0.8)
    expect(await opacity(page.locator('[data-test-id="dialog-confirm-btn"]'))).toBe(1)
    expect(await opacity(page.getByRole('button', { name: 'Close' }))).toBe(1)
    expect(await opacity(page.getByRole('button', { name: 'Cancel' }))).toBe(1)
    expect(await opacity(page.getByRole('status'))).toBe(1)

    // A bare layer (Dialog/Drawer without the confirmation layout) is dimmed by the same rule.
    const bare = await page.evaluate(() => {
      document.querySelectorAll('dialog').forEach(dialog => dialog.close())
      const dialog = document.createElement('dialog')
      dialog.className = 'dialog'
      dialog.dataset.busy = 'true'
      dialog.innerHTML = '<form><label>Name <input value="Travel"></label><div><button type="button" aria-busy="true">Save</button></div></form><p id="free">Notes</p>'
      document.body.append(dialog)
      dialog.showModal()
      dialog.getAnimations({ subtree: true }).forEach(animation => animation.finish())
      const visible = (selector: string) => {
        let value = 1
        for (let element: Element | null = dialog.querySelector(selector); element; element = element.parentElement) value *= Number(getComputedStyle(element).opacity)
        return value
      }
      return { label: visible('label'), save: visible('button'), free: visible('#free'), pointer: getComputedStyle(dialog.querySelector('label')!).pointerEvents }
    })
    expect(bare).toEqual({ label: 0.8, save: 1, free: 0.8, pointer: 'none' })
  })
})

// Every specimen, state, theme and viewport of the gallery: 0 serious/critical axe violations.
test.describe('accessibility', () => {
  for (const c of galleryCases) {
    test(`axe ${c.id}`, async ({ page }) => {
      await open(page, c)
      await settleAnimations(page)
      await expectNoA11yViolations(page)
    })
  }

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
