import { expect, test, type Page } from '@playwright/test'

const storyIds = [
  'chart-adapter--bar-and-horizontal-bar-dark',
  'chart-adapter--bar-and-horizontal-bar-light',
  'chart-adapter--controlled-selection',
  'chart-adapter--line-and-area-dark',
  'chart-adapter--line-and-area-light',
  'chart-adapter--pie-and-donut-dark',
  'chart-adapter--pie-and-donut-light',
  'explore--full-drill-flow--three-levels',
  'explore--keyboard-walkthrough',
  'explore--perspective-switching-on-a-segment',
  'panel-matrix--all-kinds-and-states--dark',
  'panel-matrix--all-kinds-and-states--light',
  'panels-v2--cascade-final-stage',
  'panels-v2--export-idle',
  'panels-v2--export-pending',
  'panels-v2--export-snapshot-retry',
  'panels-v2--table-columns',
  'panels-v2--table-empty-page',
  'panels-v2--table-pagination-and-leaf-actions',
  'parity--compact-table-cells',
  'parity--coverage-composite',
  'parity--dashboard-loading-skeleton-dark',
  'parity--dashboard-loading-skeleton-light',
  'parity--metric-group',
  'parity--panel-skeletons-dark',
  'parity--panel-skeletons-light',
  'parity--pie-with-legend-below',
  'parity--tab-group',
] as const

const staticStories = [
  ['chart-adapter--bar-and-horizontal-bar-dark', 2],
  ['chart-adapter--bar-and-horizontal-bar-light', 2],
  ['chart-adapter--controlled-selection', 1],
  ['chart-adapter--line-and-area-dark', 2],
  ['chart-adapter--line-and-area-light', 2],
  ['chart-adapter--pie-and-donut-dark', 2],
  ['chart-adapter--pie-and-donut-light', 2],
  ['explore--keyboard-walkthrough', 1],
  ['panel-matrix--all-kinds-and-states--dark', 0],
  ['panel-matrix--all-kinds-and-states--light', 0],
  ['panels-v2--cascade-final-stage', 0],
  ['panels-v2--export-idle', 0],
  ['panels-v2--export-pending', 0],
  ['panels-v2--export-snapshot-retry', 0],
  ['panels-v2--table-columns', 0],
  ['panels-v2--table-empty-page', 0],
  ['panels-v2--table-pagination-and-leaf-actions', 0],
  ['parity--compact-table-cells', 0],
  ['parity--coverage-composite', 0],
  ['parity--dashboard-loading-skeleton-dark', 0],
  ['parity--dashboard-loading-skeleton-light', 0],
  ['parity--metric-group', 0],
  ['parity--panel-skeletons-dark', 0],
  ['parity--panel-skeletons-light', 0],
  ['parity--pie-with-legend-below', 1],
  ['parity--tab-group', 0],
] as const

async function openStory(page: Page, storyId: string, canvasCount: number): Promise<void> {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  const query = new URLSearchParams({ story: storyId, mode: 'preview', 'lens-vr': '1' })
  await page.goto(`/?${query.toString()}`, { waitUntil: 'networkidle' })
  await expect(page.locator('.lens-root')).toBeVisible()
  await expect(page.locator('canvas')).toHaveCount(canvasCount)
  await page.evaluate(async () => {
    await document.fonts.ready
    await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())))
  })
  await expect(page.locator('html')).toHaveAttribute('data-lens-vr', 'true')
}

async function screenshot(page: Page, name: string): Promise<void> {
  // Baseline files ship inside the Go module zip, which rejects paths with
  // characters like the middle dot Ladle inherits from story names.
  expect(name).toMatch(/^[A-Za-z0-9._-]+$/)
  await page.evaluate(async () => {
    await document.fonts.ready
    await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())))
  })
  await expect(page).toHaveScreenshot(`${name}.png`, { fullPage: true })
}

test('VR manifest covers every Ladle story', async ({ request }) => {
  const response = await request.get('/meta.json')
  expect(response.ok()).toBe(true)
  const metadata: unknown = await response.json()
  if (
    typeof metadata !== 'object'
    || metadata === null
    || !('stories' in metadata)
    || typeof metadata.stories !== 'object'
    || metadata.stories === null
    || Array.isArray(metadata.stories)
  ) {
    throw new Error('Ladle meta.json does not contain a stories object')
  }
  const discovered = Object.keys(metadata.stories).sort()

  expect(discovered).toEqual([...storyIds].sort())
  const covered = new Set<string>([
    ...staticStories.map(([storyId]) => storyId),
    ...keyframeCovered,
  ])

  expect(storyIds.filter((storyId) => !covered.has(storyId))).toEqual([])
})

for (const [storyId, canvasCount] of staticStories) {
  test(storyId, async ({ page }) => {
    await openStory(page, storyId, canvasCount)
    await screenshot(page, storyId)
  })
}

const keyframeCovered = [
  'explore--full-drill-flow--three-levels',
  'explore--perspective-switching-on-a-segment',
] as const

test('explore full drill flow keyframes', async ({ page }) => {
  await openStory(page, 'explore--full-drill-flow--three-levels', 1)
  await screenshot(page, 'explore-full-drill-01-root')

  await page.getByRole('treeitem', { name: /Operating margin/ }).click()
  await expect(page.getByRole('listbox', { name: 'Perspectives for Operating margin' })).toBeVisible()
  await screenshot(page, 'explore-full-drill-02-perspectives')

  await page.getByRole('option', { name: /Composition/ }).click()
  await expect(page.getByRole('treeitem', { name: /Services/ })).toBeVisible()
  await screenshot(page, 'explore-full-drill-03-composition')

  await page.getByRole('treeitem', { name: /Services/ }).click()
  await expect(page.getByRole('treeitem', { name: /Sales/ })).toBeVisible()
  await screenshot(page, 'explore-full-drill-04-cost-centers')

  await page.getByRole('treeitem', { name: /Sales/ }).click()
  await expect(page.getByRole('treeitem', { name: /Invoice TX-1042/ })).toBeVisible()
  await screenshot(page, 'explore-full-drill-05-transactions')
})

test('explore perspective switching keyframes', async ({ page }) => {
  await openStory(page, 'explore--perspective-switching-on-a-segment', 1)
  await screenshot(page, 'explore-perspectives-01-choice')

  await page.getByRole('option', { name: /Trend/ }).click()
  await expect(page.locator('[data-explore-view="line"]')).toBeVisible()
  await screenshot(page, 'explore-perspectives-02-trend')

  // Switching enters the perspective's own level; return to the choice
  // point via the breadcrumb before selecting the next perspective.
  await page.getByRole('button', { name: /Operating margin/ }).click()
  await page.getByRole('option', { name: /Bridge/ }).click()
  await expect(page.locator('[data-explore-view="cascade"]')).toBeVisible()
  await screenshot(page, 'explore-perspectives-03-bridge')

  await page.getByRole('button', { name: /Operating margin/ }).click()
  await page.getByRole('option', { name: /Evidence/ }).click()
  await expect(page.locator('[data-explore-view="table"]')).toBeVisible()
  await screenshot(page, 'explore-perspectives-04-evidence')
})
