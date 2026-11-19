import type { Locator, Page, Request, Response } from '@playwright/test'
import { deadline, timeout, type WaitOptions } from './wait.js'

export type RequestMatch = { method: string; pathname: string }
export type ResponseMatch = RequestMatch | ((response: Response) => boolean)
export type ValidationResult = { controlName: string; message: string }
export type HtmxSettledOptions = WaitOptions & {
  /** Consumer readiness for the (possibly replaced) target, evaluated after every correlated swap settled; receives the remaining budget. */
  ready?: (target: Locator, options: { timeoutMs: number }) => Promise<unknown>
}

function describe(match: ResponseMatch): string {
  return typeof match === 'function' ? 'matching response' : `${match.method.toUpperCase()} ${match.pathname}`
}

/**
 * Registers the response wait before the action. A missing response rejects; there is no
 * success path after the timeout, so callers must not wrap this in a catch that continues.
 */
export async function actionAndResponse(page: Page, match: ResponseMatch, action: () => Promise<unknown>, options: WaitOptions = {}): Promise<Response> {
  const ms = timeout(options)
  const response = page.waitForResponse(r => typeof match === 'function' ? match(r) : r.request().method() === match.method.toUpperCase() && new URL(r.url()).pathname === match.pathname, { timeout: ms })
    .catch((cause: unknown) => { throw new Error(`No ${describe(match)} response within ${ms}ms`, { cause }) })
  const [result] = await Promise.all([response, Promise.resolve().then(action)])
  return result
}

type ObserverRecord = { url: string; method: string; status: number; settled: boolean; failed: boolean; aborted: boolean; navigated: boolean; scoped: boolean; decided: boolean }

/**
 * Observes production HTMX lifecycle events for every request started after installation.
 * Listeners are attached to the requesting element, the target and every element HTMX swapped
 * in, as well as the document, because a node detached before its event fires no longer bubbles.
 * The target (and scope membership) is refreshed at beforeSwap so HX-Retarget and beforeSwap
 * overrides are followed. A request is complete when its swap settled, or when it loaded without
 * producing swapped content (empty replacement, `hx-swap="none"`), which HTMX never follows with
 * afterSettle. HX-Redirect, HX-Refresh and HX-Location responses are recorded as navigations.
 */
async function installHtmxObserver(page: Page, scope: string | null): Promise<string> {
  const key = `sdk-htmx-${crypto.randomUUID()}`
  await page.evaluate(({ key, scope }) => {
    if (scope !== null) document.querySelector(scope) // Throws for selectors that are not CSS.
    type Tracked = Omit<ObserverRecord, 'decided'> & { swapped: boolean; loaded: boolean; swapDecided: boolean; target: Element | null }
    const terminal = ['htmx:beforeSwap', 'htmx:afterSwap', 'htmx:afterOnLoad', 'htmx:afterSettle', 'htmx:responseError', 'htmx:sendError', 'htmx:sendAbort', 'htmx:timeout', 'htmx:swapError', 'htmx:onLoadError']
    const records: Tracked[] = []
    const requests = new WeakMap<object, Tracked>()
    const observed = new Set<Element>()
    const observe = (element: unknown) => {
      if (!(element instanceof Element) || observed.has(element)) return
      observed.add(element)
      for (const type of terminal) element.addEventListener(type, update)
    }
    const inScope = (element: unknown) => scope === null || (element instanceof Element && (element.matches(scope) || element.closest(scope) !== null || element.querySelector(scope) !== null))
    const navigation = /^(HX-Redirect|HX-Refresh|HX-Location):/im
    function update(event: Event) {
      const d = (event as CustomEvent).detail
      const record = d?.xhr && requests.get(d.xhr)
      if (!record || record.failed || record.aborted || record.settled) return
      record.url = d.xhr.responseURL || record.url
      record.status = d.xhr.status
      if (event.type === 'htmx:afterSettle') record.settled = true
      else if (event.type === 'htmx:beforeSwap') {
        record.swapDecided = true
        if (d.target instanceof Element) { record.target = d.target; record.scoped = inScope(d.target); observe(d.target) }
      } else if (event.type === 'htmx:afterSwap') { record.swapped = true; observe(event.target) }
      else if (event.type === 'htmx:afterOnLoad') {
        record.loaded = true
        if (navigation.test(d.xhr.getAllResponseHeaders())) record.navigated = true
      } else if (event.type === 'htmx:sendAbort') record.aborted = true
      else record.failed = true
    }
    const before = (event: Event) => {
      const d = (event as CustomEvent).detail
      if (!d?.xhr || requests.has(d.xhr)) return
      const target = d.target ?? d.elt
      const record: Tracked = { url: new URL(d.requestConfig.path, location.href).href, method: String(d.requestConfig.verb).toUpperCase(), status: 0, settled: false, failed: false, aborted: false, navigated: false, scoped: inScope(target), swapped: false, loaded: false, swapDecided: false, target: target instanceof Element ? target : null }
      records.push(record)
      requests.set(d.xhr, record)
      observe(d.elt)
      observe(d.target)
    }
    // A delayed swap keeps htmx-swapping on a connected target until it runs.
    const complete = (r: Tracked) => r.settled || r.navigated || (r.loaded && !r.swapped && !(r.target?.isConnected && r.target.classList.contains('htmx-swapping')))
    document.addEventListener('htmx:beforeRequest', before)
    for (const type of terminal) document.addEventListener(type, update)
    ;(window as any)[key] = {
      records: () => records.map(r => ({ url: r.url, method: r.method, status: r.status, settled: complete(r), failed: r.failed, aborted: r.aborted, navigated: r.navigated, scoped: r.scoped,
        // Scope is final once beforeSwap ran or the request ended; HX-Retarget may still move it before that.
        decided: r.swapDecided || r.loaded || r.settled || r.failed || r.aborted || r.navigated })),
      dispose: () => {
        document.removeEventListener('htmx:beforeRequest', before)
        for (const type of terminal) document.removeEventListener(type, update)
        for (const element of observed) for (const type of terminal) element.removeEventListener(type, update)
      },
    }
  }, { key, scope })
  return key
}
async function disposeObserver(page: Page, key: string): Promise<void> {
  await page.evaluate(key => { (window as any)[key]?.dispose(); delete (window as any)[key] }, key).catch(() => {})
}
/** Records of the observer, or null when the document that held it was replaced by a navigation. */
async function observedRecords(page: Page, key: string): Promise<ObserverRecord[] | null> {
  return page.evaluate(key => (window as any)[key]?.records() ?? null, key).catch(() => null)
}
const settling = '.htmx-swapping, .htmx-settling, .htmx-added'

/**
 * Waits for every HTMX request whose target is, contains, or is inside the CSS selector `target`
 * (after HX-Retarget): each must swap and settle. Refusals, send errors, load errors and HTMX
 * timeouts reject; so does a response that navigates (HX-Redirect/HX-Refresh/HX-Location),
 * which callers wait for with `page.waitForURL` instead. Requests started while earlier ones are
 * still pending are included; ones triggered later (delays, intersect) need their own call.
 * Every observed request must first answer (beforeSwap or a terminal event) because HX-Retarget
 * can move it into the scope; an unrelated slow request started during the wait (a long poll)
 * therefore delays completion until it answers or the budget ends.
 * Afterwards at least one element must match `target`, none of the matches may carry transient
 * swap classes, and `ready` runs with the remaining budget. The budget starts after `action`.
 */
export async function actionAndHtmxSettled(page: Page, target: string, action: () => Promise<unknown>, options: HtmxSettledOptions = {}): Promise<void> {
  timeout(options)
  const key = await installHtmxObserver(page, target)
  try {
    await action()
    const remaining = deadline(options)
    const scopedRecords = async () => (await observedRecords(page, key))?.filter(r => r.scoped) ?? null
    try {
      await page.waitForFunction(key => {
        const records = (window as any)[key].records() as ObserverRecord[]
        const scoped = records.filter(r => r.scoped)
        if (scoped.some(r => r.failed || r.navigated)) return true
        return records.every(r => r.decided) && scoped.length > 0 && scoped.every(r => r.settled || r.aborted || r.failed)
      }, key, { timeout: remaining() })
    } catch (cause) {
      const seen = await scopedRecords()
      if (seen === null) throw new Error(`Page navigated while waiting for HTMX settlement of ${target}`, { cause })
      const undecided = (await observedRecords(page, key))?.filter(r => !r.decided) ?? []
      if (seen.length === 0 && undecided.length > 0) throw new Error(`HTMX requests that may still retarget into ${target} did not respond: ${JSON.stringify(undecided)}`, { cause })
      if (seen.length === 0) throw new Error(`Action issued no HTMX request targeted ${target}`, { cause })
      throw new Error(`HTMX requests for ${target} did not settle: ${JSON.stringify(seen)}`, { cause })
    }
    const scoped = await scopedRecords()
    if (scoped === null) throw new Error(`Page navigated while waiting for HTMX settlement of ${target}`)
    const failure = scoped.find(r => r.failed)
    if (failure) throw new Error(`HTMX request failed: ${failure.status} ${failure.method} ${failure.url}`)
    const navigated = scoped.find(r => r.navigated)
    if (navigated) throw new Error(`HTMX response navigates instead of swapping ${target}: ${navigated.method} ${navigated.url}; wait for the URL instead`)
    if (!scoped.some(r => r.settled)) throw new Error(`HTMX requests for ${target} were all aborted`)
    await page.waitForFunction(({ target, settling }) => {
      const elements = Array.from(document.querySelectorAll(target))
      return elements.length > 0 && elements.every(element => !element.matches(settling) && element.querySelector(settling) === null)
    }, { target, settling }, { timeout: remaining() })
    await options.ready?.(page.locator(target), { timeoutMs: remaining() })
  } finally {
    await disposeObserver(page, key)
  }
}

/** Follows the matching production HTMX XHR through afterSettle; server refusals and send errors reject. */
export async function actionAndHtmxResponse(page: Page, match: ResponseMatch, action: () => Promise<unknown>, options: WaitOptions = {}): Promise<Response> {
  const remaining = deadline(options)
  const key = await installHtmxObserver(page, null)
  try {
    const response = await actionAndResponse(page, match, action, { timeoutMs: remaining() })
    const identity = { url: response.url(), method: response.request().method(), status: response.status() }
    await page.waitForFunction(({ key, identity }) => {
      const records = ((window as any)[key].records() as ObserverRecord[]).filter(r => r.url === identity.url && r.method === identity.method)
      return records.some(r => !r.aborted && (r.settled || r.failed)) && records.every(r => r.settled || r.failed || r.aborted)
    }, { key, identity }, { timeout: remaining() })
    const failure = await page.evaluate(({ key, identity }) => {
      const records = ((window as any)[key].records() as ObserverRecord[]).filter(r => r.url === identity.url && r.method === identity.method && !r.aborted)
      const latest = records.at(-1)
      return latest?.failed ? latest.status : null
    }, { key, identity })
    if (failure !== null) throw new Error(`HTMX request failed: ${failure} ${response.url()}`)
    if (!response.ok()) throw new Error(`HTMX request failed: ${response.status()} ${response.url()}`)
    return response
  } finally {
    await disposeObserver(page, key)
  }
}

/**
 * Expects the browser to refuse the form natively: waits for the invalid event, checks that the
 * reported control is still :invalid, and that neither a submit event, an HTMX request from the
 * form, nor a network request to the form's submission URL happened during the interaction.
 * Completion does not depend on a network timeout.
 */
export async function actionAndValidationRefusal(page: Page, form: Locator, action: () => Promise<unknown>, options: WaitOptions = {}): Promise<ValidationResult> {
  timeout(options)
  const key = `sdk-validation-${crypto.randomUUID()}`
  const submission = await form.evaluate((element, key) => {
    const f = element as HTMLFormElement
    const state = { invalid: null as { controlName: string; message: string } | null, control: null as Element | null, submitted: false, htmx: false }
    const invalid = (event: Event) => {
      if (state.invalid) return
      const control = event.target as HTMLInputElement
      state.control = control
      state.invalid = { controlName: control.name, message: control.validationMessage }
    }
    const submit = () => { state.submitted = true }
    const htmx = (event: Event) => {
      const elt = (event as CustomEvent).detail?.elt
      if (elt instanceof Element && (elt === f || f.contains(elt) || (elt as HTMLButtonElement).form === f)) state.htmx = true
    }
    f.addEventListener('invalid', invalid, true)
    f.addEventListener('submit', submit)
    document.addEventListener('htmx:beforeRequest', htmx)
    ;(window as any)[key] = { state, dispose: () => {
      f.removeEventListener('invalid', invalid, true)
      f.removeEventListener('submit', submit)
      document.removeEventListener('htmx:beforeRequest', htmx)
    } }
    // The form and each submitter may declare where submission goes; getAttribute avoids
    // properties shadowed by controls named "action" or "method".
    const verbs = ['post', 'put', 'patch', 'delete', 'get']
    const candidates: { url: string; method: string }[] = []
    const resolve = (path: string | null) => new URL(path ?? location.href, location.href).href
    for (const element of [f, ...Array.from(f.querySelectorAll('button:not([type]), button[type="submit"], input[type="submit"], input[type="image"]'))]) {
      for (const verb of verbs) if (element.hasAttribute(`hx-${verb}`)) candidates.push({ url: resolve(element.getAttribute(`hx-${verb}`)), method: verb.toUpperCase() })
      if (element !== f && (element.hasAttribute('formaction') || element.hasAttribute('formmethod'))) candidates.push({ url: resolve(element.getAttribute('formaction') ?? f.getAttribute('action')), method: (element.getAttribute('formmethod') ?? f.getAttribute('method') ?? 'get').toUpperCase() })
    }
    if (!verbs.some(verb => f.hasAttribute(`hx-${verb}`))) candidates.push({ url: resolve(f.getAttribute('action')), method: (f.getAttribute('method') ?? 'get').toUpperCase() })
    return candidates
  }, key)
  const path = (url: string) => url.split(/[?#]/)[0]
  let request: string | null = null
  const observe = (r: Request) => {
    if (submission.some(candidate => path(r.url()) === path(candidate.url) && r.method() === candidate.method)) request ??= `${r.method()} ${r.url()}`
  }
  page.on('request', observe)
  try {
    await action()
    const remaining = deadline(options)
    try {
      await page.waitForFunction(key => (window as any)[key].state.invalid !== null || (window as any)[key].state.submitted || (window as any)[key].state.htmx, key, { timeout: remaining() })
    } catch (cause) {
      throw new Error('Expected a native invalid event; none was dispatched', { cause })
    }
    // A page round trip delivers request events the interaction already caused.
    const state = await page.evaluate(key => {
      const s = (window as any)[key].state
      return { invalid: s.invalid, submitted: s.submitted, htmx: s.htmx, stillInvalid: s.control instanceof Element && s.control.matches(':invalid') }
    }, key)
    if (request) throw new Error(`Native validation unexpectedly issued a submission request: ${request}`)
    if (state.submitted || state.htmx) throw new Error('Expected native validation refusal without submission')
    if (!state.stillInvalid) throw new Error(`Control ${state.invalid.controlName} reported invalid but is not :invalid`)
    return state.invalid
  } finally {
    page.off('request', observe)
    await page.evaluate(key => { (window as any)[key]?.dispose(); delete (window as any)[key] }, key).catch(() => {})
  }
}
