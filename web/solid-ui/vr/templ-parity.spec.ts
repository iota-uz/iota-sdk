import { expect, test, type Browser, type Locator, type Page, type TestInfo } from '@playwright/test'

const templOrigin = `http://127.0.0.1:${process.env.SOLID_UI_TEMPL_VR_PORT ?? '61011'}`

type Fixture = {
  specimen: 'button' | 'input' | 'checkbox' | 'select' | 'radio' | 'switch' | 'tabs' | 'badge' | 'avatar' | 'pagination' | 'table' | 'dialog' | 'drawer' | 'combobox' | 'search-select' | 'date-picker' | 'copy-button' | 'language-select'
  state: string
  solidSpecimen: 'buttons' | 'form-controls' | 'advanced-forms' | 'display' | 'navigation' | 'data' | 'dialog' | 'drawer' | 'selection' | 'utilities'
  solidTarget: string
  templTarget: string
  solidStateTarget?: string
  templStateTarget?: string
  properties: readonly string[]
}

type ThemedFixture = Fixture & { theme: 'light' | 'dark' }

const baseFixtures: readonly Fixture[] = [
  ...['default', 'hover', 'focus', 'disabled', 'loading'].map((state): Fixture => ({
    specimen: 'button', state, solidSpecimen: 'buttons',
    solidTarget: '[data-vr-target="button-primary"]', templTarget: '[data-parity-target="button"]',
    properties: ['width', 'height', 'font-size', 'font-weight', 'padding', 'border-radius', 'background-color', 'color', 'opacity'],
  })),
  ...['default', 'hover', 'focus', 'disabled', 'error'].map((state): Fixture => ({
    specimen: 'input', state, solidSpecimen: 'form-controls',
    solidTarget: '[data-vr-target="form-input"]', templTarget: '[data-parity-target="input"]',
    properties: ['height', 'font-size', 'font-weight', 'padding', 'border-radius', 'border-color', 'background-color', 'color', 'box-shadow', 'opacity'],
  })),
  ...['default', 'checked', 'disabled'].map((state): Fixture => ({
    specimen: 'checkbox', state, solidSpecimen: 'form-controls',
    solidTarget: '#gallery-enabled + div', templTarget: '[data-parity-target="checkbox"] + div',
    properties: ['width', 'height', 'border-radius', 'border-color', 'background-color', 'color', 'opacity'],
  })),
  ...['default', 'hover', 'focus', 'disabled', 'error'].map((state): Fixture => ({
    specimen: 'select', state, solidSpecimen: 'form-controls',
    solidTarget: '[data-vr-target="form-select"]', templTarget: '[data-parity-target="select"]',
    properties: ['height', 'font-size', 'font-weight', 'padding', 'border-radius', 'border-color', 'background-color', 'color', 'box-shadow', 'opacity'],
  })),
  ...['default', 'focus', 'disabled', 'checked'].map((state): Fixture => ({
    specimen: 'radio', state, solidSpecimen: 'advanced-forms',
    solidTarget: '[data-vr-target="advanced-radio"] + div', templTarget: '[data-parity-target="radio"] + div',
    solidStateTarget: '[data-vr-target="advanced-radio"]', templStateTarget: '[data-parity-target="radio"]',
    properties: ['width', 'height', 'border-radius', 'border-color', 'background-color', 'box-shadow', 'opacity'],
  })),
  ...['default', 'focus', 'disabled', 'checked'].map((state): Fixture => ({
    specimen: 'switch', state, solidSpecimen: 'advanced-forms',
    solidTarget: '[data-vr-target="advanced-switch"] + div', templTarget: '[data-parity-target="switch"] + div',
    solidStateTarget: '[data-vr-target="advanced-switch"]', templStateTarget: '[data-parity-target="switch"]',
    properties: ['width', 'height', 'border-radius', 'border-color', 'background-color', 'color', 'box-shadow', 'opacity'],
  })),
  ...['default', 'hover', 'focus', 'selected'].map((state): Fixture => ({
    specimen: 'tabs', state, solidSpecimen: 'navigation',
    solidTarget: '[data-vr-target="navigation-tab"]', templTarget: '.fixture > a',
    properties: ['width', 'height', 'font-size', 'font-weight', 'padding', 'border-radius', 'background-color', 'color', 'box-shadow', 'outline-color', 'outline-offset', 'outline-style', 'outline-width', 'opacity'],
  })),
  {
    specimen: 'badge', state: 'default', solidSpecimen: 'display',
    solidTarget: '[data-vr-target="display-badge"]', templTarget: '.fixture > div',
    properties: ['width', 'height', 'font-size', 'font-weight', 'padding', 'border-radius', 'border-color', 'background-color', 'color', 'opacity'],
  },
  {
    specimen: 'avatar', state: 'default', solidSpecimen: 'data',
    solidTarget: '[data-vr-target="data-avatar"]', templTarget: '.fixture > div',
    properties: ['width', 'height', 'font-size', 'font-weight', 'border-radius', 'background-color', 'color', 'opacity'],
  },
  ...['default', 'focus'].map((state): Fixture => ({
    specimen: 'pagination', state, solidSpecimen: 'navigation',
    solidTarget: '[data-vr-target="navigation-pagination"]', templTarget: '.fixture > ul',
    solidStateTarget: '[data-vr-target="navigation-pagination"] [aria-current="page"]', templStateTarget: '.fixture > ul a.bg-brand-500',
    properties: ['width', 'height', 'font-size', 'padding', 'background-color', 'color', 'opacity'],
  })),
  {
    specimen: 'table', state: 'default', solidSpecimen: 'data',
    solidTarget: '[data-vr-target="data-table"] thead th:nth-child(2)', templTarget: '[data-parity-target="table"] thead th:nth-child(2)',
    properties: ['width', 'height', 'font-size', 'font-weight', 'padding', 'border-color', 'background-color', 'color', 'opacity'],
  },
  {
    specimen: 'dialog', state: 'open', solidSpecimen: 'dialog',
    solidTarget: 'dialog', templTarget: '[data-parity-target="dialog"]',
    properties: ['width', 'height', 'padding', 'border-radius', 'background-color', 'color', 'box-shadow', 'opacity'],
  },
  {
    specimen: 'drawer', state: 'open', solidSpecimen: 'drawer',
    solidTarget: 'dialog > div', templTarget: '[data-parity-target="drawer"] > div',
    properties: ['width', 'height', 'padding', 'background-color', 'color', 'box-shadow', 'opacity'],
  },
  ...['default', 'focus', 'disabled'].map((state): Fixture => ({
    specimen: 'combobox', state, solidSpecimen: 'selection',
    solidTarget: '[data-vr-target="selection-combobox"] input[type="text"]', templTarget: '.fixture input[type="text"]',
    properties: ['width', 'height', 'font-size', 'font-weight', 'padding', 'border-radius', 'border-color', 'background-color', 'color', 'box-shadow', 'opacity'],
  })),
  ...['default', 'hover', 'focus'].map((state): Fixture => ({
    specimen: 'copy-button', state, solidSpecimen: 'utilities',
    solidTarget: '[data-vr-target="utilities-copy"]', templTarget: '.fixture > button',
    properties: ['width', 'height', 'font-size', 'font-weight', 'padding', 'border-radius', 'background-color', 'color', 'box-shadow', 'opacity'],
  })),
  ...['default', 'focus'].map((state): Fixture => ({
    specimen: 'language-select', state, solidSpecimen: 'utilities',
    solidTarget: '[data-vr-target="utilities-language"]', templTarget: '[data-parity-target="language-select"]',
    properties: ['width', 'height', 'font-size', 'font-weight', 'padding', 'border-radius', 'border-color', 'background-color', 'color', 'box-shadow', 'opacity'],
  })),
  ...['default', 'focus'].map((state): Fixture => ({
    specimen: 'date-picker', state, solidSpecimen: 'selection',
    solidTarget: '#date-picker-solid input[type="text"]', templTarget: '.fixture input[type="text"]',
    properties: ['width', 'height', 'font-size', 'font-weight', 'padding', 'border-radius', 'border-color', 'background-color', 'color', 'box-shadow', 'opacity'],
  })),
  ...['default', 'focus', 'disabled'].map((state): Fixture => ({
    specimen: 'search-select', state, solidSpecimen: 'selection',
    solidTarget: '#search-select-solid-input', templTarget: '[data-parity-target="search-select"]',
    properties: ['width', 'height', 'font-size', 'font-weight', 'padding', 'border-radius', 'border-color', 'background-color', 'color', 'box-shadow', 'opacity'],
  })),
]

const fixtures: readonly ThemedFixture[] = baseFixtures.flatMap((fixture) =>
  (['light', 'dark'] as const).map((theme) => ({ ...fixture, theme })),
)

async function ready(page: Page, marker: 'gallery' | 'fixture'): Promise<void> {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await expect(page.locator('html')).toHaveAttribute(`data-${marker}-ready`, 'true')
  await page.evaluate(async () => {
    await document.fonts.ready
    await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())))
  })
}

async function applyState(locator: Locator, state: string): Promise<void> {
  if (state === 'hover') await locator.hover()
  if (state === 'focus') await locator.focus()
  await locator.evaluate((element) => element.getAnimations({ subtree: true }).forEach((animation) => {
    if (Number.isFinite(animation.effect?.getComputedTiming().endTime ?? Number.POSITIVE_INFINITY)) animation.finish()
  }))
}

async function computed(locator: Locator, properties: readonly string[]) {
  return locator.evaluate((element, names) => {
    const style = getComputedStyle(element)
    const box = element.getBoundingClientRect()
    const colorCanvas = document.createElement('canvas')
    colorCanvas.width = 1
    colorCanvas.height = 1
    const colorContext = colorCanvas.getContext('2d', { willReadFrequently: true })!
    const normalizedValue = (name: string, value: string) => {
      if (!name.endsWith('color')) return value
      colorContext.clearRect(0, 0, 1, 1)
      colorContext.fillStyle = value
      colorContext.fillRect(0, 0, 1, 1)
      return Array.from(colorContext.getImageData(0, 0, 1, 1).data).join(',')
    }
    return {
      box: { x: box.x, y: box.y, width: box.width, height: box.height },
      styles: Object.fromEntries(names.map((name) => {
        const value = style.getPropertyValue(name)
        return [name, normalizedValue(name, value)]
      })),
    }
  }, properties)
}

async function differingPixels(page: Page, first: Buffer, second: Buffer) {
  return page.evaluate(async ({ firstURL, secondURL }) => {
    const load = async (url: string) => {
      const image = new Image()
      image.src = url
      await image.decode()
      const canvas = document.createElement('canvas')
      canvas.width = image.naturalWidth
      canvas.height = image.naturalHeight
      const context = canvas.getContext('2d', { willReadFrequently: true })!
      context.drawImage(image, 0, 0)
      return { width: canvas.width, height: canvas.height, pixels: context.getImageData(0, 0, canvas.width, canvas.height).data }
    }
    const [a, b] = await Promise.all([load(firstURL), load(secondURL)])
    if (a.width !== b.width || a.height !== b.height) return { count: -1, first: [a.width, a.height], second: [b.width, b.height] }
    let count = 0
    for (let offset = 0; offset < a.pixels.length; offset += 4) {
      if (a.pixels[offset] !== b.pixels[offset]
        || a.pixels[offset + 1] !== b.pixels[offset + 1]
        || a.pixels[offset + 2] !== b.pixels[offset + 2]
        || a.pixels[offset + 3] !== b.pixels[offset + 3]) count++
    }
    return { count, first: [a.width, a.height], second: [b.width, b.height] }
  }, {
    firstURL: `data:image/png;base64,${first.toString('base64')}`,
    secondURL: `data:image/png;base64,${second.toString('base64')}`,
  })
}

async function deterministicTargetScreenshots(
  page: Page,
  solidTarget: Locator,
  templTarget: Locator,
  state: string,
): Promise<[Buffer, Buffer]> {
  const [solidHTML, templHTML, box] = await Promise.all([
    solidTarget.evaluate((element) => {
      const clone = element.cloneNode(true) as HTMLElement
      if (element instanceof HTMLSelectElement && clone instanceof HTMLSelectElement) {
        Array.from(clone.options).forEach((option, index) => option.toggleAttribute('selected', element.options[index]?.selected ?? false))
      }
      return clone.outerHTML
    }),
    templTarget.evaluate((element) => {
      const clone = element.cloneNode(true) as HTMLElement
      if (element instanceof HTMLSelectElement && clone instanceof HTMLSelectElement) {
        Array.from(clone.options).forEach((option, index) => option.toggleAttribute('selected', element.options[index]?.selected ?? false))
      }
      return clone.outerHTML
    }),
    solidTarget.boundingBox(),
  ])
  if (!box) throw new Error('Parity target has no layout box')
  await page.locator('#root').evaluate((element) => { (element as HTMLElement).style.display = 'none' })
  const host = await page.locator('body').evaluateHandle((body, width) => {
    const element = document.createElement('div')
    element.id = 'native-parity-host'
    element.style.cssText = `position:fixed;left:48px;top:48px;width:${width}px`
    body.append(element)
    return element
  }, box.width)
  const capture = async (html: string) => {
    await host.evaluate((element, markup) => { element.innerHTML = markup }, html)
    const target = page.locator('#native-parity-host').locator(':scope > *')
    await applyState(target, state)
    return target.screenshot({ animations: 'disabled', caret: 'hide', scale: 'css' })
  }
  const solidPNG = await capture(solidHTML)
  const templPNG = await capture(templHTML)
  await host.dispose()
  return [solidPNG, templPNG]
}

async function compareFixture(browser: Browser, fixture: ThemedFixture, testInfo: TestInfo): Promise<void> {
  const context = await browser.newContext({
    colorScheme: fixture.theme,
    deviceScaleFactor: 1,
    locale: 'en-US',
    timezoneId: 'UTC',
    viewport: { width: 1440, height: 900 },
  })
  const solid = await context.newPage()
  const templ = await context.newPage()
  const solidQuery = new URLSearchParams({ specimen: fixture.solidSpecimen, theme: fixture.theme, state: fixture.state })
  const templQuery = new URLSearchParams({ specimen: fixture.specimen, theme: fixture.theme, state: fixture.state })
  await Promise.all([
    solid.goto(`http://127.0.0.1:${process.env.SOLID_UI_VR_PORT ?? '61010'}/?${solidQuery}`, { waitUntil: 'networkidle' }),
    templ.goto(`${templOrigin}/?${templQuery}`, { waitUntil: 'networkidle' }),
  ])
  await Promise.all([ready(solid, 'gallery'), ready(templ, 'fixture')])
  const solidTarget = solid.locator(fixture.solidTarget)
  const templTarget = templ.locator(fixture.templTarget)
  if (['badge', 'avatar', 'tabs', 'pagination', 'copy-button'].includes(fixture.specimen)) {
    await templ.locator('.fixture').evaluate((element) => { element.style.width = 'max-content' })
  }
  if (fixture.specimen === 'table') {
    const [solidTableBox, templTableBox] = await Promise.all([
      solid.locator('[data-vr-target="data-table"]').boundingBox(),
      templ.locator('[data-parity-target="table"]').boundingBox(),
    ])
    if (solidTableBox && templTableBox) {
      await templ.locator('.fixture').evaluate((element, width) => { element.style.width = `${width}px` }, 480 + solidTableBox.width - templTableBox.width)
    }
  }
  const solidBox = await solidTarget.boundingBox()
  if (['input', 'select', 'combobox', 'search-select', 'date-picker', 'language-select'].includes(fixture.specimen) && solidBox) {
    const templBox = await templTarget.boundingBox()
    if (templBox) {
      await templ.locator('.fixture').evaluate((element, width) => { element.style.width = `${width}px` }, 480 + solidBox.width - templBox.width)
    }
  }
  if (solidBox) {
    const templBox = await templTarget.boundingBox()
    if (templBox) {
      await templ.locator('.fixture').evaluate((element, offset) => {
        element.style.position = 'relative'
        element.style.left = `${offset.x}px`
        element.style.top = `${offset.y}px`
      }, { x: solidBox.x - templBox.x, y: solidBox.y - templBox.y })
    }
  }
  await Promise.all([
    applyState(fixture.solidStateTarget ? solid.locator(fixture.solidStateTarget) : solidTarget, fixture.state),
    applyState(fixture.templStateTarget ? templ.locator(fixture.templStateTarget) : templTarget, fixture.state),
  ])
  const [solidContract, templContract] = await Promise.all([
    computed(solidTarget, fixture.properties),
    computed(templTarget, fixture.properties),
  ])
  expect(solidContract, 'Solid DOMRect/computed styles must equal the rendered templ primitive').toEqual(templContract)

  const requiresSameDocumentPaint = ['select', 'language-select'].includes(fixture.specimen)
    || (fixture.specimen === 'tabs' && fixture.state === 'focus')
  const [solidPNG, templPNG] = requiresSameDocumentPaint
    ? await deterministicTargetScreenshots(solid, solidTarget, templTarget, fixture.state)
    : await Promise.all([
      solidTarget.screenshot({ animations: 'disabled', caret: 'hide', scale: 'css' }),
      templTarget.screenshot({ animations: 'disabled', caret: 'hide', scale: 'css' }),
    ])
  const pixelDiff = await differingPixels(solid, solidPNG, templPNG)
  if (pixelDiff.count !== 0) {
    await testInfo.attach('solid.png', { body: solidPNG, contentType: 'image/png' })
    await testInfo.attach('templ.png', { body: templPNG, contentType: 'image/png' })
  }
  expect(pixelDiff, 'Solid and templ decoded RGBA pixels must match exactly').toEqual({
    count: 0,
    first: pixelDiff.second,
    second: pixelDiff.second,
  })
  await context.close()
}

for (const fixture of fixtures) {
  test(`templ parity ${fixture.specimen} ${fixture.theme} ${fixture.state}`, async ({ browser }, testInfo) => {
    await compareFixture(browser, fixture, testInfo)
  })
}
