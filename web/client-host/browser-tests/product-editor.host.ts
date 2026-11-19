import { expect, type Page } from '@playwright/test'
import type { ComponentCase, ComponentHost } from '@iota-uz/sdk/testing'

export type ProductEditorState = 'editing' | 'loading' | 'empty' | 'error' | 'stale'
export type ProductEditorCase = ComponentCase & { state: ProductEditorState }

const desktop = { width: 1280, height: 800 }
const narrow = { width: 390, height: 844 }

/** Each locale, theme and viewport appears at least once; the full product is not enumerated. */
export const productEditorCases: readonly ProductEditorCase[] = [
  { id: 'editing-en-light-desktop', state: 'editing', locale: 'en', theme: 'light', viewport: desktop },
  { id: 'loading-ru-dark-narrow', state: 'loading', locale: 'ru', theme: 'dark', viewport: narrow },
  { id: 'empty-en-dark-desktop', state: 'empty', locale: 'en', theme: 'dark', viewport: desktop },
  { id: 'error-ru-light-narrow', state: 'error', locale: 'ru', theme: 'light', viewport: narrow },
  { id: 'stale-en-light-narrow', state: 'stale', locale: 'en', theme: 'light', viewport: narrow },
]

export const backend = {
  pending: (page: Page) => page.evaluate(() => window.productEditorBackend.pending()),
  respond: (page: Page) => page.evaluate(() => window.productEditorBackend.respond()),
}

/**
 * Renders the fixture product editor (real client-host draft/mount exports, production styles)
 * and enters the declared state through user interactions; only the save backend is faked.
 */
export const productEditorHost: ComponentHost<ProductEditorCase> = {
  cases: () => productEditorCases,
  async navigate(page, c) {
    await page.goto(`/?${new URLSearchParams({ theme: c.theme, locale: c.locale })}`)
    const root = page.locator('.editor-shell')
    switch (c.state) {
      case 'editing': break
      case 'loading':
        await page.getByLabel('Name').fill('Pending draft')
        await page.getByRole('button', { name: 'Save changes' }).click()
        await expect.poll(() => backend.pending(page)).toBe(1)
        break
      case 'empty':
        await page.getByLabel('Base amount').fill('')
        break
      case 'error':
        await page.getByLabel('Factor').fill('0.5')
        await page.getByRole('button', { name: 'Save changes' }).click()
        await expect.poll(() => backend.respond(page)).toBe(true)
        break
      case 'stale':
        await page.getByLabel('Name').fill('Submitted name')
        await page.getByRole('button', { name: 'Save changes' }).click()
        await expect.poll(() => backend.pending(page)).toBe(1)
        // The draft moves past the snapshot the server still holds.
        await page.getByLabel('Name').fill('Edited while saving')
        break
    }
    return root
  },
}

declare global { interface Window { productEditorBackend: { pending(): number; respond(): boolean } } }
