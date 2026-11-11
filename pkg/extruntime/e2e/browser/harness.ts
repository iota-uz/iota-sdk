// Shared helpers of the browser specs: the state the Go server published, signed-in browser contexts, the
// management API as the operator uses it, and a console watcher.
import { readFileSync } from 'node:fs'
import playwrightModule from '../../../../web/client-host/node_modules/@playwright/test/index.js'
import type * as PlaywrightTypes from '../../../../web/client-host/node_modules/@playwright/test'
import type { Browser, BrowserContext, ConsoleMessage, Page, Request } from '../../../../web/client-host/node_modules/@playwright/test'

// The runtime import is the CommonJS entry (the one the runner loaded); the types come from its declarations.
const playwright = playwrightModule as unknown as typeof PlaywrightTypes

export const { expect, test } = playwright
export type { Page, Request }

export interface BrowserUser { id: number; name: string; email: string; cookieName: string; token: string }
export interface State { baseURL: string; users: Record<string, BrowserUser>; clients: Record<string, string> }

export const state: State = JSON.parse(readFileSync(process.env.IOTA_EXT_BROWSER_STATE as string, 'utf8'))

export type Who = 'admin' | 'auditor' | 'member' | 'viewer'

export interface Session { context: BrowserContext; page: Page; problems: string[] }

/** A fresh browser context holding the session cookie of the user; `problems` collects console errors and page errors. */
export async function signIn(browser: Browser, who: Who, options: { locale?: string; colorScheme?: 'light' | 'dark'; javaScriptEnabled?: boolean } = {}): Promise<Session> {
  const user = state.users[who]
  const context = await browser.newContext({ baseURL: state.baseURL, ...options })
  await context.addCookies([{ name: user.cookieName, value: user.token, url: state.baseURL }])
  const page = await context.newPage()
  return { context, page, problems: watch(page) }
}

/** What the composed application cannot provide and the host has nothing to do with: the realtime socket (/ws) of the standard layout. */
const ENVIRONMENT_NOISE = [/WebSocket connection to '[^']*\/ws'/, /^Event$/]

/** Console errors and uncaught page errors of the page, as readable lines. */
export function watch(page: Page): string[] {
  const problems: string[] = []
  page.on('pageerror', (error) => problems.push(`pageerror: ${error.message}`))
  page.on('console', (message: ConsoleMessage) => {
    if (message.type() !== 'error') return
    const text = message.text()
    if (ENVIRONMENT_NOISE.some((pattern) => pattern.test(text))) return
    problems.push(`console.error: ${text}`)
  })
  return problems
}

/** The management API as the administrator: bindings are the operator's decisions. */
export async function manage(method: 'GET' | 'POST' | 'DELETE', path: string, body?: unknown): Promise<Response> {
  const admin = state.users.admin
  const response = await fetch(`${state.baseURL}/_ext/manage${path}`, {
    method,
    headers: { cookie: `${admin.cookieName}=${admin.token}`, ...(body === undefined ? {} : { 'content-type': 'application/json' }) },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  })
  return response
}

/** Whether a page the user opens carries the activation of a bound shell (the runtime context of the page). */
async function shellActive(who: Who): Promise<boolean> {
  const user = state.users[who]
  const response = await fetch(`${state.baseURL}/crm/clients`, { headers: { cookie: `${user.cookieName}=${user.token}` } })
  return (await response.text()).includes('id="iota-runtime-context"')
}

/** Polls until the application serves what the binding decided (the UI source refreshes its snapshot within milliseconds). */
async function until(what: string, condition: () => Promise<boolean>): Promise<void> {
  const deadline = Date.now() + 10_000
  while (!(await condition())) {
    if (Date.now() > deadline) throw new Error(`timed out waiting for ${what}`)
    await new Promise((resolve) => setTimeout(resolve, 25))
  }
}

export async function bindShell(): Promise<void> {
  const response = await manage('POST', '/bindings', { slotKind: 'shell', targetId: 'app', package: 'e2e.browser', version: '1.0.0', export: 'shell' })
  // 409: this very implementation already holds the slot (a previous test of the file bound it).
  if (response.status !== 201 && response.status !== 409) throw new Error(`bind the shell: ${response.status} ${await response.text()}`)
  await until('the shell binding to be served', () => shellActive('auditor'))
}

/** Closes the shell binding; a binding that is not open is not an error. */
export async function unbindShell(): Promise<void> {
  const response = await manage('DELETE', '/bindings/shell/app')
  if (response.status !== 204 && response.status !== 404) throw new Error(`unbind the shell: ${response.status} ${await response.text()}`)
  await until('the standard shell to be served again', async () => !(await shellActive('auditor')))
}

/** The user's saved interface language, as the account page would set it. */
export async function setLanguage(who: Who, language: string): Promise<void> {
  const response = await fetch(`${state.baseURL}/__e2e/language`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ user: state.users[who].id, language }),
  })
  if (response.status !== 204) throw new Error(`set the language: ${response.status} ${await response.text()}`)
}

/** What the lifecycle probe of the fixture package (testdata/browser/src/probe.ts) reports. */
export interface Probe { live: Record<string, number>; log: string[]; listeners: number; pings: Record<string, number> }

export function probe(page: Page): Promise<Probe> {
  return page.evaluate(() => (window as unknown as { __extE2E?: Probe }).__extE2E ?? { live: {}, log: [], listeners: 0, pings: {} })
}

/** The events the probe persisted in localStorage across documents of this origin. */
export function persistedLog(page: Page): Promise<string[]> {
  return page.evaluate(() => JSON.parse(window.localStorage.getItem('e2e.lifecycle') ?? '[]') as string[])
}
