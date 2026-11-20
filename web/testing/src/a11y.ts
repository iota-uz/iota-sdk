import type { Page, TestInfo } from '@playwright/test'
import { createRequire } from 'node:module'
import path from 'node:path'
import { pathToFileURL } from 'node:url'

/** Scope and rule overrides for {@link expectNoA11yViolations}. */
export type A11yCheckOptions = {
  /** CSS selectors of the checked area; the whole document by default. */
  include?: string[]
  exclude?: string[]
  /** axe rule ids to skip. Only with a comment in the test that says why. */
  disableRules?: string[]
  /** Test that receives the JSON report; defaults to the running Playwright test. */
  testInfo?: TestInfo
}

/** Impacts that fail the check. `minor` and `moderate` stay in the attached report only. */
export const blockingA11yImpacts = ['serious', 'critical'] as const

export const axePackage = '@axe-core/playwright'

type AxeNode = { target: unknown[]; html: string; failureSummary?: string }
type AxeViolation = { id: string; impact?: string | null; help: string; helpUrl: string; nodes: AxeNode[] }
type AxeResults = { violations: AxeViolation[]; incomplete: unknown[]; passes: unknown[]; inapplicable: unknown[]; url: string; testEngine?: unknown; [key: string]: unknown }
type AxeBuilderInstance = {
  include(selector: string): AxeBuilderInstance
  exclude(selector: string): AxeBuilderInstance
  disableRules(rules: string | string[]): AxeBuilderInstance
  analyze(): Promise<AxeResults>
}
type AxeBuilderConstructor = new (options: { page: Page }) => AxeBuilderInstance

/** Raised when the optional axe peer is not installed in the test project. */
export class A11yDependencyError extends Error {
  constructor(cause: unknown) {
    super(`expectNoA11yViolations needs the optional peer ${axePackage}. Install it in the test project: pnpm add -D ${axePackage}`, { cause })
    this.name = 'A11yDependencyError'
  }
}

/** Raised for serious/critical violations; `violations` keeps the blocking ones. */
export class A11yViolationError extends Error {
  readonly violations: readonly AxeViolation[]
  constructor(violations: readonly AxeViolation[]) {
    super(formatViolations(violations))
    this.name = 'A11yViolationError'
    this.violations = violations
  }
}

function formatViolations(violations: readonly AxeViolation[]): string {
  const lines = violations.map(violation => {
    const targets = violation.nodes.slice(0, 5).map(node => `    - ${node.target.map(String).join(' ')}`).join('\n')
    const more = violation.nodes.length > 5 ? `\n    … ${violation.nodes.length - 5} more` : ''
    return `  [${violation.impact}] ${violation.id}: ${violation.help} (${violation.helpUrl})\n${targets}${more}`
  })
  return `${violations.length} serious/critical accessibility violation(s):\n${lines.join('\n')}`
}

type AxeModule = { default?: AxeBuilderConstructor | { default?: AxeBuilderConstructor }; AxeBuilder?: AxeBuilderConstructor }

async function importAxe(): Promise<AxeModule> {
  const specifier = axePackage
  try {
    return await import(specifier) as AxeModule
  } catch (error) {
    // A linked or vendored SDK resolves from its own location; the peer is
    // installed in the test project, so look there as well.
    try {
      const resolved = createRequire(path.join(process.cwd(), 'package.json')).resolve(specifier)
      return await import(pathToFileURL(resolved).href) as AxeModule
    } catch {
      throw error
    }
  }
}

async function loadAxeBuilder(): Promise<AxeBuilderConstructor> {
  // The peer is optional: importing @iota-uz/sdk/testing must not require it,
  // so it is resolved only when the check runs.
  try {
    const module = await importAxe()
    const candidate = module.AxeBuilder ?? module.default
    const builder = typeof candidate === 'function' ? candidate : candidate?.default
    if (typeof builder !== 'function') throw new Error(`${axePackage} exports no AxeBuilder`)
    return builder
  } catch (error) {
    throw new A11yDependencyError(error)
  }
}

async function currentTestInfo(): Promise<TestInfo | undefined> {
  try {
    const playwright = await import('@playwright/test')
    return playwright.test.info()
  } catch {
    return undefined
  }
}

/**
 * Runs axe on the page and fails on violations with impact `serious` or
 * `critical`. The complete axe result is attached to the test as JSON
 * (`a11y-report.json`, `{ options, results }`) whether the check passes or fails.
 */
export async function expectNoA11yViolations(page: Page, options: A11yCheckOptions = {}): Promise<void> {
  const AxeBuilder = await loadAxeBuilder()
  let builder = new AxeBuilder({ page })
  for (const selector of options.include ?? []) builder = builder.include(selector)
  for (const selector of options.exclude ?? []) builder = builder.exclude(selector)
  if (options.disableRules?.length) builder = builder.disableRules(options.disableRules)
  const results = await builder.analyze()
  const testInfo = options.testInfo ?? await currentTestInfo()
  if (testInfo) {
    // The complete axe result (passes, inapplicable, incomplete, violations,
    // engine and environment metadata) next to the check's scope options.
    await testInfo.attach('a11y-report.json', {
      body: JSON.stringify({ options: { include: options.include, exclude: options.exclude, disableRules: options.disableRules }, results }, null, 2),
      contentType: 'application/json',
    })
  }
  const blocking = results.violations.filter(violation => (blockingA11yImpacts as readonly string[]).includes(violation.impact ?? ''))
  if (blocking.length > 0) throw new A11yViolationError(blocking)
}
