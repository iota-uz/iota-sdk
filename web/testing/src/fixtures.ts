import type { Browser, PlaywrightTestArgs, PlaywrightTestOptions, PlaywrightWorkerArgs, PlaywrightWorkerOptions, TestType } from '@playwright/test'
import { lazyIdentities, withEnvironment, type EnvironmentDescriptor, type EnvironmentLifecycle, type ScenarioControl, type ScenarioLease, type IdentityLease } from './index.js'

export function createEnvironmentTest<E extends EnvironmentDescriptor, Input, State, Key, Identity>(base: TestType<PlaywrightTestArgs & PlaywrightTestOptions, PlaywrightWorkerArgs & PlaywrightWorkerOptions>, options: {
  lifecycle: EnvironmentLifecycle<E>
  scenarios(environment: E): ScenarioControl<Input, State>
  identity(environment: E, browser: Browser, key: Key): Promise<Identity>
}) {
  return base.extend<{ scenario: ScenarioLease<Input, State>; identity: IdentityLease<Key, Identity> }, { environment: E; workerIdentity: IdentityLease<Key, Identity> }>({
    environment: [async ({}, use, info) => {
      await withEnvironment(options.lifecycle, info.workerIndex, use)
    }, { scope: 'worker' }],
    workerIdentity: [async ({ environment, browser }, use) => {
      await use(lazyIdentities(key => options.identity(environment, browser, key)))
    }, { scope: 'worker' }],
    baseURL: async ({ environment }, use) => { await use(environment.baseURL) },
    identity: async ({ workerIdentity }, use) => { await use(workerIdentity) },
    scenario: async ({ environment }, use, info) => {
      const control = options.scenarios(environment)
      const scopes: string[] = []
      try {
        await use(async input => {
          const scope = `${environment.runId}-w${info.workerIndex}-r${info.retry}-${crypto.randomUUID()}`
          scopes.push(scope)
          return control.prepare(input, scope)
        })
      } finally {
        const results = await Promise.allSettled(scopes.map(scope => control.dispose(scope)))
        const errors = results.filter((result): result is PromiseRejectedResult => result.status === 'rejected').map(result => result.reason)
        if (errors.length) throw new AggregateError(errors, 'Scenario teardown failed')
      }
    },
  })
}
