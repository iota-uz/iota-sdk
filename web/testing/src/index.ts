import type { Locator, Page, Response } from '@playwright/test'

export type WaitOptions = { timeoutMs?: number }
export type RequestMatch = { method: string; pathname: string }
export type ResponseMatch = RequestMatch | ((response: Response) => boolean)
export type ValidationResult = { controlName: string; message: string }
export type ComponentCase = { id: string; state: string; locale: string; theme: 'light' | 'dark'; viewport: { width: number; height: number } }
function timeout(options: WaitOptions = {}): number {
  const value = options.timeoutMs ?? 10_000
  if (!Number.isInteger(value) || value <= 0) throw new Error('timeoutMs must be a positive integer')
  return value
}
export async function actionAndResponse(page: Page, match: ResponseMatch, action: () => Promise<unknown>, options: WaitOptions = {}): Promise<Response> {
  const response = page.waitForResponse(r => typeof match === 'function' ? match(r) : r.request().method() === match.method.toUpperCase() && new URL(r.url()).pathname === match.pathname, { timeout: timeout(options) })
  const [result] = await Promise.all([response, action()])
  return result
}

/** Installs the event observer before the action and follows replacement targets by selector. */
export async function actionAndHtmxSettled(page: Page, target: string, action: () => Promise<unknown>, options: WaitOptions = {}): Promise<void> {
  const key = `sdk-htmx-${crypto.randomUUID()}`
  await page.evaluate(({ key, target }) => {
    const state = { pending: 0, swapped: false, settled: false, failed: false }
    const requests = new WeakSet<object>()
    const matches = (event: Event) => {
      const detail = (event as CustomEvent).detail
      const element = detail?.target ?? detail?.elt
      return element instanceof Element && (element.matches(target) || Boolean(element.closest(target)))
    }
    const before = (event: Event) => {
      const detail = (event as CustomEvent).detail
      if (matches(event) && detail?.xhr && !requests.has(detail.xhr)) { requests.add(detail.xhr); state.pending++; state.settled = false }
    }
    const correlated = (event: Event) => { const xhr = (event as CustomEvent).detail?.xhr; return xhr && requests.has(xhr) }
    const swap = (event: Event) => { if (correlated(event)) state.swapped = true }
    const settle = (event: Event) => { if (correlated(event) && state.swapped) { requests.delete((event as CustomEvent).detail.xhr); state.pending--; state.settled = state.pending === 0 } }
    const error = (event: Event) => { if (correlated(event)) state.failed = true }
    document.addEventListener('htmx:beforeRequest', before)
    document.addEventListener('htmx:afterSwap', swap)
    document.addEventListener('htmx:afterSettle', settle)
    document.addEventListener('htmx:responseError', error)
    ;(window as any)[key] = { state, dispose: () => {
      document.removeEventListener('htmx:beforeRequest', before)
      document.removeEventListener('htmx:afterSwap', swap)
      document.removeEventListener('htmx:afterSettle', settle)
      document.removeEventListener('htmx:responseError', error)
    } }
  }, { key, target })
  try {
    await action()
    await page.waitForFunction(key => (window as any)[key].state.settled || (window as any)[key].state.failed, key, { timeout: timeout(options) })
    const failed = await page.evaluate(key => (window as any)[key].state.failed, key)
    if (failed) throw new Error(`HTMX response failed for ${target}`)
    await page.locator(target).waitFor({ state: 'visible', timeout: timeout(options) })
  } finally {
    await page.evaluate(key => { (window as any)[key]?.dispose(); delete (window as any)[key] }, key)
  }
}

export async function actionAndValidationRefusal(page: Page, form: Locator, action: () => Promise<unknown>): Promise<ValidationResult> {
  const key = `sdk-validation-${crypto.randomUUID()}`
  await form.evaluate((element, key) => {
    const state = { invalid: null as ValidationResult | null, submitted: false }
    const invalid = (event: Event) => {
      const control = event.target as HTMLInputElement
      state.invalid ??= { controlName: control.name, message: control.validationMessage }
    }
    const submit = () => { state.submitted = true }
    element.addEventListener('invalid', invalid, true)
    element.addEventListener('submit', submit)
    ;(window as any)[key] = { state, dispose: () => { element.removeEventListener('invalid', invalid, true); element.removeEventListener('submit', submit) } }
  }, key)
  const submission = await form.evaluate(element => {
    const f = element as HTMLFormElement
    return { url: new URL(f.getAttribute('hx-post') ?? f.action, location.href).href, method: f.hasAttribute('hx-post') ? 'POST' : f.method.toUpperCase() }
  })
  let submittedRequest = false
  const observe = (request: import('@playwright/test').Request) => {
    if (request.url().split('?')[0] === submission.url.split('?')[0] && request.method() === submission.method) submittedRequest = true
  }
  page.on('request', observe)
  try {
    await action()
    if (submittedRequest) throw new Error('Native validation unexpectedly issued a submission request')
    return await page.evaluate(key => {
      const state = (window as any)[key].state
      if (!state.invalid || state.submitted) throw new Error('Expected native validation refusal without submission')
      return state.invalid
    }, key)
  } finally {
    page.off('request', observe)
    await page.evaluate(key => { (window as any)[key]?.dispose(); delete (window as any)[key] }, key)
  }
}

export async function expectComponentReady(page: Page, root: Locator, options: WaitOptions & { readinessAttribute?: string } = {}): Promise<void> {
  await waitForCondition(async () => await root.getAttribute(options.readinessAttribute ?? 'data-component-ready') === 'true', timeout(options), 'Component readiness marker missing')
  await page.waitForFunction(() => document.fonts.status === 'loaded', undefined, { timeout: timeout(options) })
  await root.waitFor({ state: 'visible', timeout: timeout(options) })
}
export interface ComponentHost {
  cases(): Promise<readonly ComponentCase[]>
  navigate(page: Page, componentCase: ComponentCase): Promise<Locator>
}
export async function openComponentCase(page: Page, host: ComponentHost, componentCase: ComponentCase, options: WaitOptions = {}): Promise<Locator> {
  await page.setViewportSize(componentCase.viewport)
  const root = await host.navigate(page, componentCase)
  await expectComponentReady(page, root, options)
  return root
}

export async function actionAndHtmxResponse(page: Page, match: ResponseMatch, action: () => Promise<unknown>, options: WaitOptions = {}): Promise<Response> {
  const key = `sdk-response-${crypto.randomUUID()}`
  await page.evaluate(key => {
    const records: { url: string; method: string; status: number; settled: boolean; failed: boolean; aborted: boolean }[] = []
    const requests = new WeakMap<object, typeof records[number]>()
    const observed = new Set<Element>()
    const before = (event: Event) => {
      const d = (event as CustomEvent).detail
      if (!d?.xhr) return
      const record = { url: new URL(d.requestConfig.path, location.href).href, method: String(d.requestConfig.verb).toUpperCase(), status: 0, settled: false, failed: false, aborted: false }
      records.push(record); requests.set(d.xhr, record)
      // A removed swap target no longer bubbles its lifecycle events to document.
      for (const element of [d.elt, d.target]) {
        if (!(element instanceof Element) || observed.has(element)) continue
        observed.add(element)
        element.addEventListener('htmx:afterSettle', update)
        element.addEventListener('htmx:responseError', update)
        element.addEventListener('htmx:sendAbort', update)
        element.addEventListener('htmx:sendError', update)
      }
    }
    const update = (event: Event) => {
      const d = (event as CustomEvent).detail
      const record = d?.xhr && requests.get(d.xhr)
      if (!record) return
      record.url = d.xhr.responseURL || record.url
      record.status = d.xhr.status
      record.failed = event.type === 'htmx:responseError' || event.type === 'htmx:sendError'
      record.aborted = event.type === 'htmx:sendAbort'
      record.settled = event.type === 'htmx:afterSettle'
    }
    document.addEventListener('htmx:beforeRequest', before)
    document.addEventListener('htmx:afterSettle', update)
    document.addEventListener('htmx:responseError', update)
    document.addEventListener('htmx:sendAbort', update)
    document.addEventListener('htmx:sendError', update)
    ;(window as any)[key] = { records, dispose: () => {
      document.removeEventListener('htmx:beforeRequest', before)
      document.removeEventListener('htmx:afterSettle', update)
      document.removeEventListener('htmx:responseError', update)
      document.removeEventListener('htmx:sendAbort', update)
      document.removeEventListener('htmx:sendError', update)
      for (const element of observed) {
        element.removeEventListener('htmx:afterSettle', update)
        element.removeEventListener('htmx:responseError', update)
        element.removeEventListener('htmx:sendAbort', update)
        element.removeEventListener('htmx:sendError', update)
      }
    } }
  }, key)
  try {
    const response = await actionAndResponse(page, match, action, options)
    const identity = { url: response.url(), method: response.request().method(), status: response.status() }
    await page.waitForFunction(({ key, identity }) => {
      const records = (window as any)[key].records.filter((r: any) => r.url === identity.url && r.method === identity.method)
      return records.some((r: any) => !r.aborted && (r.settled || r.failed)) && records.every((r: any) => r.settled || r.failed || r.aborted)
    }, { key, identity }, { timeout: timeout(options) })
    const failure = await page.evaluate(({ key, identity }) => {
      const records = (window as any)[key].records.filter((r: any) => r.url === identity.url && r.method === identity.method && !r.aborted)
      const latest = records.at(-1)
      return latest?.failed ? latest.status : null
    }, { key, identity })
    if (failure !== null) throw new Error(`HTMX request failed: ${failure} ${response.url()}`)
    if (!response.ok()) throw new Error(`HTMX request failed: ${response.status()} ${response.url()}`)
    return response
  } finally {
    await page.evaluate(key => { (window as any)[key]?.dispose(); delete (window as any)[key] }, key)
  }
}

export interface EnvironmentDescriptor { runId: string; baseURL: string; artifactDir: string }
export interface EnvironmentLifecycle<E extends EnvironmentDescriptor> {
  start(workerIndex: number): Promise<E>
  ready(environment: E): Promise<void>
  stop(environment: E): Promise<void>
}
export interface ScenarioControl<Input, Result> {
  prepare(input: Input, scopeId: string): Promise<Result>
  dispose(scopeId: string): Promise<void>
}
/** Teardown runs even when readiness, the scenario or the browser assertion fails. */
export async function withEnvironment<E extends EnvironmentDescriptor, Result>(lifecycle: EnvironmentLifecycle<E>, workerIndex: number, use: (environment: E) => Promise<Result>): Promise<Result> {
  const environment = await lifecycle.start(workerIndex)
  let failed = false
  let failure: unknown
  try { await lifecycle.ready(environment); return await use(environment) }
  catch (error) { failed = true; failure = error; throw error }
  finally {
    try { await lifecycle.stop(environment) }
    catch (error) {
      if (failed) throw new AggregateError([failure, error], 'Environment failed and teardown failed')
      throw error
    }
  }
}
export async function withScenario<Input, State, Result>(control: ScenarioControl<Input, State>, input: Input, scopeId: string, use: (state: State) => Promise<Result>): Promise<Result> {
  try { return await use(await control.prepare(input, scopeId)) }
  finally { await control.dispose(scopeId) }
}
/** Instantiate per worker/environment; identities are materialised only when requested. */
export function lazyIdentities<Key, State>(create: (identity: Key) => Promise<State>): (identity: Key) => Promise<State> {
  const identities = new Map<Key, Promise<State>>()
  return identity => {
    let state = identities.get(identity)
    if (!state) {
      state = create(identity).catch(error => { identities.delete(identity); throw error })
      identities.set(identity, state)
    }
    return state
  }
}

export type ScenarioLease<Input, State> = (input: Input) => Promise<State>
export type IdentityLease<Key, State> = (identity: Key) => Promise<State>
export { createEnvironmentTest } from './fixtures.js'
export { createProcessEnvironmentDriver, type DriverDescriptor } from './driver.js'

export async function expectFontsReady(page: Page, options: WaitOptions = {}): Promise<void> {
  await page.waitForFunction(() => document.fonts.status === 'loaded', undefined, { timeout: timeout(options) })
}
export async function expectChartsReady(charts: Locator, options: WaitOptions & { allowEmpty?: boolean } = {}): Promise<void> {
  await waitForCondition(async () => charts.evaluateAll((elements, allowEmpty) => (allowEmpty || elements.length > 0) && elements.every(element => element.getAttribute('data-chart-ready') === 'true'), options.allowEmpty ?? false), timeout(options), 'Chart readiness missing')
  if (!options.allowEmpty) await charts.first().waitFor({ state: 'visible', timeout: timeout(options) })
}
async function waitForCondition(condition: () => Promise<boolean>, timeoutMs: number, diagnostic: string): Promise<void> {
  const deadline = Date.now() + timeoutMs
  while (!(await condition())) {
    if (Date.now() >= deadline) throw new Error(diagnostic)
    await new Promise(resolve => setTimeout(resolve, Math.min(16, Math.max(1, deadline - Date.now()))))
  }
}
export { createHttpScenarioControl, ScenarioControlError, type ScenarioInput, type ScenarioResult } from './scenarios.js'
