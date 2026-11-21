import { expect, type Locator, type Page } from '@playwright/test'

export const card = (page: Page, id: string) => page.locator(`.iota-flow__node[data-id="${id}"]`)
export const handle = (page: Page, node: string, outcome: string) => page.locator(`.iota-flow__handle.source[data-nodeid="${node}"][data-handleid="${outcome}"]`)
export const edge = (page: Page, node: string, outcome: string) => page.locator(`path.ae-edge[data-edge="${node}.${outcome}"]`)

export async function center(locator: Locator) {
  const box = await locator.boundingBox()
  if (!box) throw new Error('element has no box')
  return { x: box.x + box.width / 2, y: box.y + box.height / 2 }
}

/** Drags with the mouse in small steps, as a person would. */
export async function drag(page: Page, from: { x: number; y: number }, to: { x: number; y: number }, steps = 12) {
  await page.mouse.move(from.x, from.y)
  await page.mouse.down()
  await page.mouse.move(from.x + 4, from.y + 4, { steps: 2 })
  await page.mouse.move(to.x, to.y, { steps })
  await page.mouse.up()
}

export async function open(page: Page, query = '') {
  await page.goto(`/${query ? `?${query}` : ''}`)
  await expect(page.locator('.ae-canvas')).toBeVisible()
  await page.waitForFunction(() => document.documentElement.dataset.fixtureReady === 'true')
}


export const mod = process.platform === 'darwin' ? 'Meta' : 'Control'
