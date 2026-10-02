import { test as base, expect } from '@playwright/test'
import { createEnvironmentTest } from '../src/index.js'
const prepared: string[] = []
const disposed: string[] = []
let identities = 0
const test = createEnvironmentTest(base, {
  lifecycle: {
    start: async worker => ({ runId: `run-${worker}`, baseURL: 'http://fixture.test', artifactDir: '/tmp/fixture' }),
    ready: async () => {}, stop: async () => {},
  },
  scenarios: () => ({
    prepare: async (input: { fail?: boolean }, scope: string) => { prepared.push(scope); if (input.fail) throw new Error('partial fixture'); return scope },
    dispose: async scope => { disposed.push(scope) },
  }),
  identity: async (_environment, _browser, key: string) => { identities++; return key },
})
test.describe.configure({ mode: 'serial' })
// Characterisation: falsely green if preparation never allocates a partially failing scope.
test('the consumer-injected fixture tracks successful and partially failed scenarios', async ({ scenario, page }) => {
  await page.route("http://fixture.test/fixture", route => route.fulfill({ contentType: "text/html", body: "<h1>Environment page</h1>" }));
  await page.goto("/fixture");
  await expect(page.getByRole("heading")).toHaveText("Environment page");
  expect(identities).toBe(0)
  const first = await scenario({})
  const second = await scenario({})
  expect(first).not.toBe(second)
  await expect(scenario({ fail: true })).rejects.toThrow('partial fixture')
})
// Characterisation: falsely green if teardown is asserted within the first test before its fixture unwinds.
test('the next test sees all prior leases disposed and creates identity lazily', async ({ identity }) => {
  expect(disposed).toEqual(prepared)
  expect(identities).toBe(0)
  expect(await Promise.all([identity('admin'), identity('admin')])).toEqual(['admin', 'admin'])
  expect(identities).toBe(1)
})
