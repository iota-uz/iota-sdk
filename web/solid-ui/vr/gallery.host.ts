import { expect, type Page } from '@playwright/test'
import type { ComponentCase, ComponentHost } from '@iota-uz/sdk/testing'
import { responsiveStatesFor, SPECIMENS, THEMES, type SpecimenID, type SpecimenState } from '../src/catalog'

export const viewports = {
  desktop: { width: 1440, height: 900 },
  mobile: { width: 390, height: 844 },
} as const

export type GalleryCase = ComponentCase & { specimen: SpecimenID; state: SpecimenState; screenshot: string }

/**
 * Every declared specimen state in both themes on desktop, plus each specimen's default state (and
 * its declared responsive states) on the narrow viewport. Discovery comes from the gallery catalog,
 * not a separate screen DSL.
 */
export const galleryCases: readonly GalleryCase[] = SPECIMENS.flatMap(specimen => THEMES.flatMap(theme => [
  ...specimen.states.map((state): GalleryCase => ({
    id: `${specimen.id} ${theme} ${state}`, specimen: specimen.id, state, locale: 'en-US', theme,
    viewport: viewports.desktop, screenshot: `${specimen.id}-${theme}-${state}-desktop.png`,
  })),
  {
    id: `${specimen.id} ${theme} responsive`, specimen: specimen.id, state: 'default', locale: 'en-US', theme,
    viewport: viewports.mobile, screenshot: `${specimen.id}-${theme}-default-mobile.png`,
  },
  ...responsiveStatesFor(specimen.id).map((state): GalleryCase => ({
    id: `${specimen.id} ${theme} ${state} responsive`, specimen: specimen.id, state, locale: 'en-US', theme,
    viewport: viewports.mobile, screenshot: `${specimen.id}-${theme}-${state}-mobile.png`,
  })),
]))

async function prepareState(page: Page, state: SpecimenState, specimenID: SpecimenID): Promise<void> {
  if (state === 'hover') await page.locator('[data-vr-target]').first().hover()
  if (state === 'focus' && specimenID === 'selection') await page.locator('[data-vr-target="selection-combobox"] [role="combobox"]').focus()
  else if (state === 'focus' && specimenID === 'data') await page.locator('tbody tr[tabindex]').first().focus()
  else if (state === 'focus') await page.locator('[data-vr-target]').first().focus()
  if (state === 'open' && specimenID === 'selection') await page.locator('[data-vr-target="selection-combobox"] [role="combobox"]').focus()
  // A busy layer is captured after a refused close, so its status line is visible.
  if (state === 'busy') {
    await expect(page.getByRole('dialog')).toHaveAttribute('aria-busy', 'true')
    await page.keyboard.press('Escape')
    await expect(page.locator('[data-dialog-busy-status] [role="status"]')).not.toBeEmpty()
  }
}

/** The Solid UI gallery renders the packaged components with their production CSS and fonts. */
export const galleryHost: ComponentHost<GalleryCase> = {
  cases: () => galleryCases,
  readiness: { readinessAttribute: 'data-gallery-ready' },
  async navigate(page, c) {
    const query = new URLSearchParams({ specimen: c.specimen, theme: c.theme, state: c.state })
    await page.goto(`/?${query.toString()}`, { waitUntil: 'domcontentloaded' })
    await expect(page.locator('main')).toHaveAttribute('data-specimen-id', c.specimen)
    await expect(page.locator('main')).toHaveAttribute('data-theme', c.theme)
    await prepareState(page, c.state, c.specimen)
    await page.emulateMedia({ reducedMotion: 'reduce' })
    return page.locator('html')
  },
}

export function galleryCase(specimen: SpecimenID, state: SpecimenState, theme: 'light' | 'dark' = 'light', viewport: keyof typeof viewports = 'desktop'): GalleryCase {
  const found = galleryCases.find(c => c.specimen === specimen && c.state === state && c.theme === theme && c.viewport === viewports[viewport])
  if (!found) throw new Error(`Gallery does not declare ${specimen} ${state} ${theme}`)
  return found
}
