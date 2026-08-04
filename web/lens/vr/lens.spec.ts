import { expect, test, type Page } from '@playwright/test'

const storyIds = [
  'choropleth-map--dense-region-labels',
  'choropleth-map--state-matrix',
  'comparison-interactions--previous-period',
  'distributions--histogram',
  'distributions--box-plot',
  'distributions--heatmap',
  'chart-adapter--bar-and-horizontal-bar-dark',
  'chart-adapter--bar-and-horizontal-bar-light',
  'chart-adapter--controlled-selection',
  'chart-adapter--line-and-area-dark',
  'chart-adapter--line-and-area-light',
  'chart-adapter--pie-and-donut-dark',
  'chart-adapter--pie-and-donut-light',
  'chart-adapter--radial-dark',
  'chart-adapter--radial-light',
  'chart-adapter--radial-micro-slice',
  'chart-adapter--radial-narrow',
  'chart-adapter--radial-three-rings',
  'dashboard-header--header-dark',
  'dashboard-header--header-light',
  'dashboard-header--header-narrow',
  'dashboard-header--header-read-only',
  'dashboard-header--header-with-subtitle',
  'drawer-host--closed-dark',
  'drawer-host--closed-light',
  'drawer-host--error',
  'drawer-host--loading',
  'drawer-host--long-document-scrolls',
  'drawer-host--open-dark',
  'drawer-host--open-light',
  'drawer-host--open-over-expanded-panel',
  'drawer-host--open-wide',
  'drawer-host--open-wide-focus-canvas',
  'explore-focus--canvas-at-root',
  'explore-focus--drilled--dark',
  'explore-focus--drilled--light',
  'explore-focus--half-width-host-at-rest',
  'explore-focus--half-width-host-expands-while-exploring',
  'explore-focus--lens-switched-to-trend',
  'explore-focus--narrow-container-collapses-context',
  'explore--drill-overlay--dark',
  'explore--level-fork-awaits-a-view--dark',
  'explore--level-fork-awaits-a-view--light',
  'explore--drill-overlay--light',
  'explore--drill-overlay-inside-an-expanded-panel',
  'explore--full-drill-flow--three-levels',
  'explore--header-too-narrow-for-a-level-name',
  'explore--keyboard-walkthrough',
  'explore--narrow-card-deepest-path--dark',
  'explore--narrow-card-deepest-path--light',
  'explore--perspective-switching-on-a-segment',
  'explore--segment-overlay-statistics--dark',
  'explore--segment-overlay-statistics--light',
  'filter-controls--calendar-dark',
  'filter-controls--calendar-light',
  'filter-controls--calendar-locales',
  'filter-controls--calendar-month-panel',
  'filter-controls--calendar-range-pending',
  'filter-controls--comparison-custom-interval',
  'filter-controls--comparison-menu-open',
  'filter-controls--dashboard-filter-preset-applied',
  'filter-controls--dashboard-filter-dark',
  'filter-controls--dashboard-filter-light',
  'filter-controls--dashboard-facet-active',
  'filter-controls--facet-options-open',
  'filter-controls--filters-menu-open',
  'filter-controls--granularity-segmented',
  'filter-controls--granularity-segmented-dark',
  'filter-controls--popover-open-dark',
  'filter-controls--popover-open-light',
  'filter-controls--refetch-error',
  'metric-composition--full',
  'metric-composition--full-dark',
  'metric-composition--narrow',
  'metric-composition--quality-chips',
  'metric-composition--relationship-variants',
  'panel-matrix--all-kinds-and-states--dark',
  'panel-matrix--all-kinds-and-states--light',
  'panel-matrix--sparkline-and-coverage-target--dark',
  'panel-matrix--sparkline-and-coverage-target--light',
  'panels-v2--cascade-final-stage',
  'panels-v2--cascade-semantic-tone',
  'panels-v2--export-idle',
  'panels-v2--panel-info-tip-dark',
  'panels-v2--panel-info-tip-light',
  'panels-v2--export-pending',
  'panels-v2--export-snapshot-retry',
  'panels-v2--table-columns',
  'panels-v2--table-empty-page',
  'panels-v2--table-pagination-and-leaf-actions',
  'panels-v2--waterfall-checkpoint-total',
  'panels-v2--waterfall-closing-total',
  'panels-v2--waterfall-semantic-tone',
  'panels-v2--waterfall-split-callout',
  'print-report--composed-report',
  'progressive-panels--sibling-ready-loading-and-error',
  'temporal-overlays--category-axis-overlays',
  'temporal-overlays--comparison-ghost',
  'temporal-overlays--crowded-panel-header',
  'temporal-overlays--forecast-confidence',
  'temporal-overlays--incomplete-period',
  'temporal-overlays--moving-average',
  'temporal-overlays--overlay-switched-off',
  'temporal-overlays--overlay-vocabulary',
  'temporal-overlays--reference-lines',
  'temporal-overlays--regression',
  'temporal-overlays--time-annotations',
  'sharing--panel-image-formats',
  'sharing--slice-link',
  'table-readability--narrow',
  'table-readability--wide',
  'parity--clickable-panels',
  'parity--chart-readability-states',
  'parity--compact-table-cells',
  'parity--coverage-composite',
  'parity--dashboard-loading-skeleton-dark',
  'parity--dashboard-loading-skeleton-light',
  'parity--drill-pill-affordances',
  'parity--donut-collapsed-tail',
  'parity--expanded-panel-dark',
  'parity--expanded-panel-light',
  'parity--icon-set-dark',
  'parity--icon-set-light',
  'parity--gauge',
  'parity--hidden-stack-collapsed',
  'parity--legend-all-series-hidden',
  'parity--legend-controls-and-search',
  'parity--legend-hidden-series',
  'parity--legend-solo-state',
  'parity--line-with-series-legend',
  'parity--logarithmic-horizontal-bar',
  'parity--stacked-composition-with-a-line',
  'parity--stat-beside-a-tall-table',
  'parity--metric-group',
  'parity--metric-group-info',
  'parity--metric-group-responsive',
  'parity--metric-group-sparkline',
  'parity--panel-header-pressure',
  'parity--panel-header-four-up-pressure',
  'parity--panel-skeletons-dark',
  'parity--panel-skeletons-light',
  'parity--opt-in-data-labels',
  'parity--pie-with-legend-right--light',
  'parity--pie-with-legend-right--dark',
  'parity--pie-with-tall-legend',
  'parity--tab-group',
  'parity--tabbed-legend-state',
] as const

const staticStories = [
  ['choropleth-map--dense-region-labels', 1, 10],
  // ECharts map labels can vary by a couple of antialiased edge pixels even
  // when the rendered regions, values, and geometry are identical.
  ['choropleth-map--state-matrix', 2, 10],
  // One chart-edge antialiasing pixel is bistable after the comparison shell
  // adds its prior-period series; the comparison geometry and values remain exact.
  ['comparison-interactions--previous-period', 1, 1],
  // ECharts renders these distribution canvases with stable geometry but
  // slightly different antialiasing along dense outlines and labels.
  ['distributions--histogram', 1, 900],
  ['distributions--box-plot', 1, 4_000],
  ['distributions--heatmap', 3, 2_000],
  ['chart-adapter--bar-and-horizontal-bar-dark', 2],
  ['chart-adapter--bar-and-horizontal-bar-light', 2],
  // Donut center labels have the same stable Chromium text-raster variance as
  // the exploration canvases; interaction state and geometry remain exact.
  ['chart-adapter--controlled-selection', 1, 300],
  ['chart-adapter--line-and-area-dark', 2],
  ['chart-adapter--line-and-area-light', 2],
  ['chart-adapter--pie-and-donut-dark', 2],
  ['chart-adapter--pie-and-donut-light', 2],
  ['chart-adapter--radial-dark', 2],
  ['chart-adapter--radial-light', 2],
  ['chart-adapter--radial-micro-slice', 2],
  ['chart-adapter--radial-narrow', 1],
  ['chart-adapter--radial-three-rings', 2],
  ['drawer-host--closed-dark', 0],
  ['drawer-host--closed-light', 0],
  ['drawer-host--error', 0],
  ['drawer-host--loading', 0],
  ['drawer-host--long-document-scrolls', 0, 10],
  ['drawer-host--open-dark', 0],
  ['drawer-host--open-light', 0, 10],
  ['drawer-host--open-over-expanded-panel', 0],
  ['drawer-host--open-wide', 0, 10],
  // A donut host renders one ECharts canvas; the card-corner raster is bistable
  // across a few antialiased pixels like the other focus stories (#932).
  ['drawer-host--open-wide-focus-canvas', 1, 50],
  // The focus card's rounded corner alternates between two stable rasters
  // differing by a few antialiased pixels (iota-uz/iota-sdk#932).
  ['explore-focus--canvas-at-root', 1, 50],
  ['explore-focus--drilled--dark', 1, 50],
  ['explore-focus--drilled--light', 1, 50],
  // Two side-by-side donut center labels can each use the alternate Chromium
  // glyph raster; the surrounding chart geometry remains pixel-identical.
  ['explore-focus--half-width-host-at-rest', 2, 750],
  ['explore-focus--half-width-host-expands-while-exploring', 2, 50],
  ['explore-focus--lens-switched-to-trend', 1, 50],
  ['explore-focus--narrow-container-collapses-context', 1, 50],
  // The dark overlay's dense white text has the same Chromium subpixel
  // raster variance as its light sibling; geometry and content stay exact.
  ['explore--drill-overlay--dark', 1, 1_500],
  ['explore--level-fork-awaits-a-view--dark', 0],
  ['explore--level-fork-awaits-a-view--light', 0],
  // Chromium's light-theme subpixel text raster in the floating overlay is
  // bistable; the geometry and content stay identical.
  ['explore--drill-overlay--light', 1, 800],
  ['explore--drill-overlay-inside-an-expanded-panel', 1],
  ['explore--header-too-narrow-for-a-level-name', 1],
  // The focused donut's center label is subject to Chromium text raster
  // variation while its keyboard state and geometry remain identical.
  ['explore--keyboard-walkthrough', 1, 300],
  ['explore--narrow-card-deepest-path--dark', 1],
  ['explore--narrow-card-deepest-path--light', 1],
  ['explore--segment-overlay-statistics--dark', 0],
  ['explore--segment-overlay-statistics--light', 0],
  ['dashboard-header--header-dark', 0],
  ['dashboard-header--header-light', 0],
  ['dashboard-header--header-narrow', 0],
  ['dashboard-header--header-read-only', 0],
  ['dashboard-header--header-with-subtitle', 0],
  ['filter-controls--calendar-dark', 0],
  ['filter-controls--calendar-light', 0],
  ['filter-controls--calendar-locales', 0],
  ['filter-controls--calendar-month-panel', 0],
  ['filter-controls--comparison-custom-interval', 0],
  ['filter-controls--comparison-menu-open', 0],
  ['filter-controls--dashboard-filter-preset-applied', 0],
  ['filter-controls--dashboard-filter-dark', 0],
  ['filter-controls--dashboard-filter-light', 0],
  ['filter-controls--dashboard-facet-active', 0],
  ['filter-controls--facet-options-open', 0],
  ['filter-controls--filters-menu-open', 0],
  ['filter-controls--granularity-segmented', 0],
  ['filter-controls--granularity-segmented-dark', 0],
  // The period trigger's focus ring alternates between two stable rasters
  // differing by ~25 antialiased pixels at its corners (iota-uz/iota-sdk#932).
  ['filter-controls--popover-open-dark', 0, 50],
  // The calendar's dense light-theme glyph raster differs across otherwise
  // identical Chromium captures; keep this tolerance local to that surface.
  ['filter-controls--popover-open-light', 0, 50],
  ['metric-composition--full-dark', 0],
  ['metric-composition--narrow', 0],
  ['metric-composition--quality-chips', 0],
  ['metric-composition--relationship-variants', 0],
  ['panel-matrix--all-kinds-and-states--dark', 0],
  ['panel-matrix--all-kinds-and-states--light', 0],
  // Card-edge antialiasing alternates between two stable rasters (#932).
  ['panel-matrix--sparkline-and-coverage-target--dark', 0, 50],
  ['panel-matrix--sparkline-and-coverage-target--light', 0, 50],
  ['panels-v2--cascade-final-stage', 0],
  ['panels-v2--cascade-semantic-tone', 0],
  ['panels-v2--export-idle', 0],
  ['panels-v2--panel-info-tip-dark', 0],
  ['panels-v2--panel-info-tip-light', 0],
  ['panels-v2--export-pending', 0],
  // The retry button's rounded focus-ring corners alternate across five edge
  // pixels on Darwin Chromium; keep that tolerance local to this state.
  ['panels-v2--export-snapshot-retry', 0, 10],
  ['panels-v2--table-columns', 0],
  ['panels-v2--table-empty-page', 0],
  ['panels-v2--table-pagination-and-leaf-actions', 0],
  ['panels-v2--waterfall-checkpoint-total', 0],
  ['panels-v2--waterfall-closing-total', 0],
  ['panels-v2--waterfall-semantic-tone', 0],
  ['panels-v2--waterfall-split-callout', 0],
  ['progressive-panels--sibling-ready-loading-and-error', 0],
  // A category axis: the shape /analytics/trends draws, and the one where a
  // single-category band needs a bar's sense of the band's edges.
  ['temporal-overlays--category-axis-overlays', 1],
  ['temporal-overlays--comparison-ghost', 1],
  ['temporal-overlays--crowded-panel-header', 1],
  ['temporal-overlays--forecast-confidence', 1],
  ['temporal-overlays--incomplete-period', 1],
  ['temporal-overlays--moving-average', 1],
  // Every overlay treatment on one plot, with the legend that names them.
  ['temporal-overlays--overlay-vocabulary', 1],
  // The same panel with one mark switched off from that legend.
  ['temporal-overlays--overlay-switched-off', 1],
  ['temporal-overlays--reference-lines', 1],
  ['temporal-overlays--regression', 1],
  ['temporal-overlays--time-annotations', 1],
  ['sharing--panel-image-formats', 0],
  ['sharing--slice-link', 0],
  ['table-readability--narrow', 0],
  ['table-readability--wide', 0],
  ['print-report--composed-report', 2],
  ['parity--clickable-panels', 0],
  ['parity--chart-readability-states', 2],
  ['parity--compact-table-cells', 0],
  ['parity--coverage-composite', 0],
  ['parity--dashboard-loading-skeleton-dark', 0],
  ['parity--dashboard-loading-skeleton-light', 0],
  ['parity--drill-pill-affordances', 0],
  ['parity--donut-collapsed-tail', 1],
  ['parity--expanded-panel-dark', 1],
  ['parity--expanded-panel-light', 1],
  ['parity--icon-set-dark', 0],
  ['parity--icon-set-light', 0],
  ['parity--gauge', 0],
  ['parity--hidden-stack-collapsed', 1],
  ['parity--legend-all-series-hidden', 1],
  ['parity--legend-controls-and-search', 1],
  ['parity--legend-hidden-series', 1],
  ['parity--legend-solo-state', 1, 10],
  ['parity--line-with-series-legend', 1, 10],
  // ECharts' logarithmic axis labels and bar edges can land on adjacent
  // subpixels in the Linux browser image even when the chart geometry and
  // values are unchanged. Keep the tolerance local to this canvas story.
  ['parity--logarithmic-horizontal-bar', 1, 4_000],
  ['parity--stacked-composition-with-a-line', 1],
  ['parity--stat-beside-a-tall-table', 0],
  ['parity--metric-group', 0],
  ['parity--metric-group-info', 0],
  ['parity--metric-group-sparkline', 0],
  ['parity--panel-header-pressure', 1],
  ['parity--panel-header-four-up-pressure', 0],
  ['parity--panel-skeletons-dark', 0],
  ['parity--panel-skeletons-light', 0],
  ['parity--opt-in-data-labels', 2, 10],
  ['parity--pie-with-legend-right--light', 1],
  ['parity--pie-with-legend-right--dark', 1],
  // A legend long enough to reach the top of its column — the arrangement the
  // total badge kept colliding with and the two-entry stories never produced.
  ['parity--pie-with-tall-legend', 1],
  ['parity--tab-group', 0],
  ['parity--tabbed-legend-state', 1, 25],
] as const

async function waitForCharts(page: Page): Promise<void> {
  await expect.poll(async () => page.locator('[_echarts_instance_]').evaluateAll((elements) =>
    elements.every((element) => element.getAttribute('data-chart-ready') === 'true'),
  )).toBe(true)
}

async function openStory(page: Page, storyId: string, canvasCount: number): Promise<void> {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  const query = new URLSearchParams({ story: storyId, mode: 'preview', 'lens-vr': '1' })
  await page.goto(`/?${query.toString()}`, { waitUntil: 'networkidle' })
  // An expanded panel portals a second .lens-root (its dialog host) to body.
  await expect(page.locator('.lens-root').first()).toBeVisible()
  await expect(page.locator('canvas')).toHaveCount(canvasCount)
  await waitForCharts(page)
  await page.evaluate(async () => {
    await document.fonts.ready
    await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())))
  })
  await expect(page.locator('html')).toHaveAttribute('data-lens-vr', 'true')
}

/**
 * `pointer: 'keep'` leaves the mouse where the test put it — for the stories
 * whose subject *is* a hover state (an affordance revealed on hover, a chart
 * tooltip). Everywhere else the pointer is incidental and must be parked.
 */
async function screenshot(page: Page, name: string, { pointer = 'park', maxDiffPixels }: { pointer?: 'park' | 'keep', maxDiffPixels?: number } = {}): Promise<void> {
  // Baseline files ship inside the Go module zip, which rejects paths with
  // characters like the middle dot Ladle inherits from story names.
  expect(name).toMatch(/^[A-Za-z0-9._-]+$/)
  // Playwright leaves the pointer wherever it last clicked, so whatever sits
  // under that spot is captured in its hover state and becomes part of the
  // reference. Then any layout change that moves a control by a few pixels
  // lands the pointer on a different element and fails a story that did not
  // actually change. Park the pointer off the content before every capture.
  if (pointer === 'park') await page.mouse.move(0, 0)
  await page.evaluate(async () => {
    await document.fonts.ready
    await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())))
  })
  // Fonts and container layout can cause a final ResizeObserver pass after the
  // initial story wait. Re-check the finished-backed marker after those frames
  // so the capture cannot race that resize.
  await waitForCharts(page)
  await expect(page).toHaveScreenshot(`${name}.png`, { fullPage: true, maxDiffPixels })
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
    'parity--metric-group-responsive',
  ])

  expect(storyIds.filter((storyId) => !covered.has(storyId))).toEqual([])
})

for (const [storyId, canvasCount, maxDiffPixels] of staticStories) {
  test(storyId, async ({ page }) => {
    await openStory(page, storyId, canvasCount)
    await screenshot(page, storyId, { maxDiffPixels })
  })
}

for (const [width, expectedColumns] of [
  [1101, 4],
  [1100, 2],
  [641, 2],
  [640, 1],
] as const) {
  test(`parity--metric-group-responsive-${width}`, async ({ page }) => {
    await page.setViewportSize({ width, height: 1000 })
    await openStory(page, 'parity--metric-group-responsive', 0)
    const metrics = page.locator('.lens-metric-row-columns > *')
    await expect(metrics).toHaveCount(4)
    const templateColumns = await page.locator('.lens-metric-row-columns').evaluate((element) => (
      getComputedStyle(element).gridTemplateColumns.split(/\s+/).filter(Boolean)
    ))
    expect(templateColumns).toHaveLength(expectedColumns)
    const boxes = await metrics.evaluateAll((elements) => elements.map((element) => {
      const { left, top, width: elementWidth } = element.getBoundingClientRect()
      return { left, top, width: elementWidth }
    }))
    expect(new Set(boxes.map(({ left }) => Math.round(left))).size).toBe(expectedColumns)
    expect(new Set(boxes.map(({ top }) => Math.round(top))).size).toBe(4 / expectedColumns)
    expect(Math.max(...boxes.map(({ width: elementWidth }) => elementWidth))
      - Math.min(...boxes.map(({ width: elementWidth }) => elementWidth))).toBeLessThanOrEqual(1)
    await screenshot(page, `parity--metric-group-responsive-${width}`)
  })
}

const keyframeCovered = [
  'explore--full-drill-flow--three-levels',
  'filter-controls--calendar-range-pending',
  'filter-controls--refetch-error',
  'explore--header-too-narrow-for-a-level-name',
  'explore--perspective-switching-on-a-segment',
  'metric-composition--full',
  'parity--clickable-panels',
  'parity--pie-with-legend-right--light',
] as const

test('filter refetch failure keeps stale panels and surfaces the error', async ({ page }) => {
  await openStory(page, 'filter-controls--refetch-error', 0)
  await page.getByRole('button', { name: '2025' }).click()
  await expect(page.getByRole('alert')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Profitability' })).toBeVisible()
  // The calendar icon can flip one antialiased edge pixel on Darwin Chromium;
  // all stale values, the alert, and the retry state remain exact.
  await screenshot(page, 'filter-refetch-error', { maxDiffPixels: 1 })
})

test('calendar range preview follows the hovered day', async ({ page }) => {
  await openStory(page, 'filter-controls--calendar-range-pending', 0)
  // The anchor is Jul 3; hovering Jul 15 washes the days between them.
  await page.getByRole('gridcell', { name: 'Jul 15, 2026' }).hover()
  await expect(page.locator('[data-state="previewEdge"]')).toBeVisible()
  await screenshot(page, 'filter-calendar-hover-preview', { pointer: 'keep' })
})

test('calendar keyboard focus ring walks the grid', async ({ page }) => {
  await openStory(page, 'filter-controls--calendar-range-pending', 0)
  await page.getByRole('gridcell', { name: 'Jul 3, 2026' }).focus()
  await page.keyboard.press('ArrowRight')
  await page.keyboard.press('ArrowDown')
  await expect(page.getByRole('gridcell', { name: 'Jul 11, 2026' })).toBeFocused()
  await screenshot(page, 'filter-calendar-keyboard-focus')
})

test('panel-level actions expose their affordance on hover', async ({ page }) => {
  await openStory(page, 'parity--clickable-panels', 0)
  await page.getByRole('link', { name: /Открыть|Open/ }).first().hover()
  await expect(page.locator('.lens-stat-drill-mark').first()).toBeVisible()
  // A card's rounded top-left edge can alternate by one antialiased pixel on
  // Darwin Chromium; the hover affordance itself remains pixel-identical.
  await screenshot(page, 'parity-clickable-panels-hover', { pointer: 'keep', maxDiffPixels: 1 })
})

test('a split band names itself when its column is hovered', async ({ page }) => {
  await openStory(page, 'panels-v2--waterfall-split-callout', 0)
  const tip = page.locator('.lens-waterfall-tip')
  // Nothing floats over the plot until asked — the static baseline of this same
  // story is the proof that the split takes none of the reader's attention.
  await expect(tip).toHaveCount(0)
  // The whole column is the target, not the few pixels of the band itself.
  await page.locator('.lens-waterfall-column').filter({ has: page.locator('.lens-waterfall-bar-split') }).hover()
  // A body-level portal, so the card the plot sits in cannot clip it.
  await expect(page.locator('body > .lens-waterfall-tip-overlay-root .lens-waterfall-tip')).toBeVisible()
  await screenshot(page, 'panels-v2-waterfall-split-callout-hover', { pointer: 'keep' })
})

test('chart tooltips escape the card', async ({ page }) => {
  await openStory(page, 'parity--pie-with-legend-right--light', 1)
  // Left edge of the pie: anchored inside the card this tooltip was clipped.
  await page.locator('canvas').hover({ position: { x: 300, y: 240 } })
  // The tooltip is a direct child of body now, so no ancestor can clip it.
  const tooltip = page.locator('body > div').filter({ hasText: 'Заработанная премия' }).last()
  await expect(tooltip).toBeVisible()
  await screenshot(page, 'parity-pie-tooltip', { pointer: 'keep' })
})

test('the full path stays reachable from a narrow header', async ({ page }) => {
  await openStory(page, 'explore--narrow-card-deepest-path--light', 1)
  // The header shows the current level only; clicking it opens the overlay,
  // where the whole path is listed and jumpable.
  await page.getByRole('button', { name: /Transactions/ }).click()
  await expect(page.getByRole('dialog')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Operating margin' })).toBeVisible()
  await screenshot(page, 'explore-narrow-path-overlay')
})

test('focus canvas source data expands to the audit table', async ({ page }) => {
  await openStory(page, 'explore-focus--drilled--light', 1)
  await page.getByRole('button', { name: /Source policies/ }).click()
  await expect(page.getByRole('cell', { name: 'MTR-20441' })).toBeVisible()
  // Same bistable card-corner raster as the static focus stories (#932).
  await screenshot(page, 'explore-focus-source-expanded', { maxDiffPixels: 50 })
})

test('explore full drill flow keyframes', async ({ page }) => {
  await openStory(page, 'explore--full-drill-flow--three-levels', 1)
  // Same center-label raster variance as the keyboard walkthrough.
  await screenshot(page, 'explore-full-drill-01-root', { maxDiffPixels: 300 })

  // Every level is entered through the same contextual overlay: the header
  // affordance opens it for the level, a mark opens it for that segment.
  await page.getByRole('button', { name: 'Show breakdown' }).click()
  await expect(page.getByRole('dialog')).toBeVisible()
  await screenshot(page, 'explore-full-drill-02-overlay', { maxDiffPixels: 300 })

  await page.getByRole('dialog').getByRole('button', { name: /Operating margin/ }).click()
  await page.getByRole('button', { name: 'Show breakdown' }).click()
  await expect(page.getByRole('option', { name: /Composition/ })).toBeVisible()
  await screenshot(page, 'explore-full-drill-03-perspectives', { maxDiffPixels: 300 })

  await page.getByRole('option', { name: /Composition/ }).click()
  await page.getByRole('button', { name: 'Show breakdown' }).click()
  await expect(page.getByRole('dialog').getByRole('button', { name: /Services/ })).toBeVisible()
  // Same bistable card-corner raster as the next drill keyframe (#932).
  await screenshot(page, 'explore-full-drill-04-composition', { maxDiffPixels: 300 })

  await page.getByRole('dialog').getByRole('button', { name: /Services/ }).click()
  await page.getByRole('button', { name: 'Show breakdown' }).click()
  await expect(page.getByRole('dialog').getByRole('button', { name: /Sales/ })).toBeVisible()
  // This keyframe alternates between two stable rasters differing by a few
  // antialiased pixels at the drill panel's corners (iota-uz/iota-sdk#932).
  await screenshot(page, 'explore-full-drill-05-cost-centers', { maxDiffPixels: 300 })

  await page.getByRole('dialog').getByRole('button', { name: /Sales/ }).click()
  await expect(page.getByRole('navigation', { name: /exploration path/ })).toBeVisible()
  await screenshot(page, 'explore-full-drill-06-transactions', { maxDiffPixels: 300 })
})

test('explore perspective switching keyframes', async ({ page }) => {
  // The story opens on the fork, which draws no chart until a view is chosen.
  await openStory(page, 'explore--perspective-switching-on-a-segment', 0)
  await page.getByRole('button', { name: 'Show breakdown' }).click()
  await screenshot(page, 'explore-perspectives-01-choice')

  await page.getByRole('option', { name: /Trend/ }).click()
  await expect(page.locator('[data-explore-view="line"]')).toBeVisible()
  await screenshot(page, 'explore-perspectives-02-trend')

  // Switching enters the perspective's own level; the header's back button is
  // the one-click way to the choice point (the full path lives in the overlay).
  await page.getByRole('button', { name: 'Back' }).click()
  await page.getByRole('button', { name: 'Show breakdown' }).click()
  await page.getByRole('option', { name: /Bridge/ }).click()
  await expect(page.locator('[data-explore-view="cascade"]')).toBeVisible()
  await screenshot(page, 'explore-perspectives-03-bridge')

  await page.getByRole('button', { name: 'Back' }).click()
  await page.getByRole('button', { name: 'Show breakdown' }).click()
  await page.getByRole('option', { name: /Evidence/ }).click()
  await expect(page.locator('[data-explore-view="table"]')).toBeVisible()
  await screenshot(page, 'explore-perspectives-04-evidence')
})

test('nested tabs: keyboard navigation and inner-tab persistence', async ({ page }) => {
  await openStory(page, 'metric-composition--full', 0)

  // The outer tablist starts on "Association"; ArrowRight moves focus to
  // "Composition" without activating it. Enter then mounts its nested
  // Stock/Movement tablist.
  await page.getByRole('tab', { name: 'Association' }).focus()
  await page.keyboard.press('ArrowRight')
  await expect(page.getByRole('tab', { name: 'Composition', selected: false })).toBeFocused()
  await screenshot(page, 'metric-composition-outer-tab-focus-visible')
  await page.keyboard.press('Enter')
  await expect(page.getByRole('tab', { name: 'Composition', selected: true })).toBeFocused()

  // Move into the inner tablist and switch it to "Movement". Its own
  // ArrowRight must not activate the inner tab or move the outer tablist's
  // selection; Space activates the newly focused inner tab.
  const innerTabs = page.getByRole('tablist').nth(1)
  await innerTabs.getByRole('tab', { name: 'Stock' }).focus()
  await page.keyboard.press('ArrowRight')
  await expect(innerTabs.getByRole('tab', { name: 'Movement', selected: false })).toBeFocused()
  await expect(page.getByRole('tab', { name: 'Composition', selected: true })).toBeVisible()
  await screenshot(page, 'metric-composition-inner-tab-focus-visible')
  await page.keyboard.press('Space')
  await expect(innerTabs.getByRole('tab', { name: 'Movement', selected: true })).toBeFocused()

  // Switching the outer tab away and back preserves the inner selection, and
  // the parent flow's values (rendered above the tabs) never change.
  await page.getByRole('tab', { name: 'Breakdown' }).click()
  await page.getByRole('tab', { name: 'Composition' }).click()
  await expect(page.getByRole('tablist').nth(1).getByRole('tab', { name: 'Movement', selected: true })).toBeVisible()
  await expect(page.getByText(/800,000/).first()).toBeVisible()
  await screenshot(page, 'metric-composition-inner-tab-persisted')
})
