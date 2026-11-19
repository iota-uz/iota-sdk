import type { Locator, Page, TestInfo } from '@playwright/test'
import { deadline, waitForCondition, type WaitOptions } from './wait.js'

export type ComponentCase = { id: string; state: string; locale: string; theme: 'light' | 'dark'; viewport: { width: number; height: number } }
export type ReadinessOptions = WaitOptions & {
  /** Attribute that must equal "true" on the root. Default `data-component-ready`. */
  readinessAttribute?: string
  /** Charts that must each expose `data-chart-ready="true"` before the component counts as ready. */
  charts?: Locator
}
export type ReadinessState = {
  rootCount: number
  readiness: string | null
  visible: boolean
  fonts: string
  charts: (string | null)[] | null
  html: string
}

/** Rejects a component readiness wait and retains what the page looked like when the budget ran out. */
export class ReadinessError extends Error {
  constructor(readonly attribute: string, readonly state: ReadinessState) {
    super(`Component not ready: ${attribute}=${JSON.stringify(state.readiness)}, roots=${state.rootCount}, visible=${state.visible}, fonts=${state.fonts}${state.charts ? `, charts=${JSON.stringify(state.charts)}` : ''}\n${state.html}`)
    this.name = 'ReadinessError'
  }
}

async function readinessState(page: Page, root: Locator, attribute: string, charts?: Locator): Promise<ReadinessState> {
  const rootCount = await root.count()
  const single = rootCount === 1
  return {
    rootCount,
    readiness: single ? await root.getAttribute(attribute) : null,
    visible: single ? await root.isVisible() : false,
    fonts: await page.evaluate(() => document.fonts.status),
    charts: charts ? await charts.evaluateAll(elements => elements.map(element => element.getAttribute('data-chart-ready'))) : null,
    html: single ? (await root.evaluate(element => element.outerHTML)).slice(0, 600) : '',
  }
}

/**
 * Explicit host readiness: one visible root with the readiness marker, loaded fonts and, when
 * given, every chart ready. A timeout rejects with {@link ReadinessError}; there is no suppression
 * and no fallback to network idleness or a fixed delay.
 */
export async function expectComponentReady(page: Page, root: Locator, options: ReadinessOptions = {}): Promise<void> {
  const attribute = options.readinessAttribute ?? 'data-component-ready'
  const remaining = deadline(options)
  const ready = (state: ReadinessState) => state.rootCount === 1 && state.readiness === 'true' && state.visible && state.fonts === 'loaded'
    && (state.charts === null || (state.charts.length > 0 && state.charts.every(chart => chart === 'true')))
  let last: ReadinessState | undefined
  await waitForCondition(async () => ready(last = await readinessState(page, root, attribute, options.charts)), remaining(),
    async () => new ReadinessError(attribute, last ?? await readinessState(page, root, attribute, options.charts)))
}

export async function expectFontsReady(page: Page, options: WaitOptions = {}): Promise<void> {
  await page.waitForFunction(() => document.fonts.status === 'loaded', undefined, { timeout: deadline(options)() })
}
export async function expectChartsReady(charts: Locator, options: WaitOptions & { allowEmpty?: boolean } = {}): Promise<void> {
  const remaining = deadline(options)
  await waitForCondition(async () => charts.evaluateAll((elements, allowEmpty) => (allowEmpty || elements.length > 0) && elements.every(element => element.getAttribute('data-chart-ready') === 'true'), options.allowEmpty ?? false), remaining(), () => new Error('Chart readiness missing'))
  if (!options.allowEmpty) await charts.first().waitFor({ state: 'visible', timeout: remaining() })
}

/**
 * Adapter over an existing fixture host. `cases` discovers the declared cases; `navigate` renders
 * the real exported component with production CSS/fonts/JS for one case (applying locale, theme
 * and state) and returns its root. Mocks belong only at the host's declared backend boundary.
 */
export interface ComponentHost<C extends ComponentCase = ComponentCase> {
  cases(): Promise<readonly C[]> | readonly C[]
  navigate(page: Page, componentCase: C): Promise<Locator>
  readiness?: Omit<ReadinessOptions, 'timeoutMs'>
}
export async function openComponentCase<C extends ComponentCase>(page: Page, host: ComponentHost<C>, componentCase: C, options: WaitOptions = {}): Promise<Locator> {
  await page.setViewportSize(componentCase.viewport)
  const root = await host.navigate(page, componentCase)
  await expectComponentReady(page, root, { ...host.readiness, ...options })
  return root
}
export async function componentCase<C extends ComponentCase>(host: ComponentHost<C>, id: string): Promise<C> {
  const found = (await host.cases()).find(candidate => candidate.id === id)
  if (!found) throw new Error(`Unknown component case ${id}`)
  return found
}

export type CaseDimensions = { states?: readonly string[]; locales?: readonly string[]; themes?: readonly ('light' | 'dark')[]; viewports?: readonly { width: number; height: number }[] }
/**
 * Checks that every declared state/locale/theme/viewport appears in at least one case and that
 * case IDs are unique, so hosts cover each dimension without the full Cartesian product.
 */
export function assertCaseCoverage(cases: readonly ComponentCase[], dimensions: CaseDimensions): void {
  const ids = new Set<string>()
  for (const c of cases) {
    if (ids.has(c.id)) throw new Error(`Duplicate component case ${c.id}`)
    ids.add(c.id)
  }
  const missing: string[] = []
  const check = <V>(name: string, values: readonly V[] | undefined, key: (c: ComponentCase) => string, format: (v: V) => string) => {
    for (const value of values ?? []) if (!cases.some(c => key(c) === format(value))) missing.push(`${name} ${format(value)}`)
  }
  check('state', dimensions.states, c => c.state, v => v)
  check('locale', dimensions.locales, c => c.locale, v => v)
  check('theme', dimensions.themes, c => c.theme, v => v)
  check('viewport', dimensions.viewports, c => `${c.viewport.width}x${c.viewport.height}`, v => `${v.width}x${v.height}`)
  if (missing.length) throw new Error(`Component cases do not cover: ${missing.join(', ')}`)
}

/**
 * Attaches a screenshot to the report for diagnosis only. It is never compared; approved
 * baselines use `expect(...).toHaveScreenshot` in the host's VR runner.
 */
export async function attachDiagnosticScreenshot(target: Page | Locator, testInfo: TestInfo, name: string): Promise<void> {
  await testInfo.attach(`diagnostic-${name}`, { body: await target.screenshot(), contentType: 'image/png' })
}
