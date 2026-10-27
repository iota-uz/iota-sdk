import type { Browser, PlaywrightTestArgs, PlaywrightTestOptions, PlaywrightWorkerArgs, PlaywrightWorkerOptions, TestType } from '@playwright/test'
import { lazyIdentities, withEnvironment, type EnvironmentDescriptor, type EnvironmentLifecycle, type ScenarioControl, type ScenarioLease, type IdentityLease } from './index.js'
import { preservingCleanup, withEnvironmentLease } from './lease.js'

export interface EnvironmentLeaseInfo {
  workerIndex: number
  parallelIndex: number
  testId?: string
  retry?: number
  runName: string
  environmentName: string
}
type LeaseFixtureOptions<E extends EnvironmentDescriptor> = {
  scope: 'test' | 'worker'
  runName?: string
  name?(info: Omit<EnvironmentLeaseInfo, 'environmentName'>): string
  lifecycle(info: EnvironmentLeaseInfo): EnvironmentLifecycle<E> & { close?(): Promise<void> }
}
export function createEnvironmentLeaseTest<T extends {}, W extends {}, E extends EnvironmentDescriptor>(base: TestType<T, W>, options: LeaseFixtureOptions<E> & { scope: 'test' }): TestType<T & { environmentLease: E }, W>
export function createEnvironmentLeaseTest<T extends {}, W extends {}, E extends EnvironmentDescriptor>(base: TestType<T, W>, options: LeaseFixtureOptions<E> & { scope: 'worker' }): TestType<T, W & { environmentLease: E }>
export function createEnvironmentLeaseTest<T extends {}, W extends {}, E extends EnvironmentDescriptor>(base: TestType<T, W>, options: LeaseFixtureOptions<E>): TestType<T & { environmentLease: E }, W> | TestType<T, W & { environmentLease: E }> {
  const runName = options.runName ?? crypto.randomUUID()
  const fixtures = {
    environmentLease: [async ({}, use: (environment: E) => Promise<void>, info: { workerIndex: number; parallelIndex: number; testId?: string; retry?: number }) => {
      const identity = { workerIndex: info.workerIndex, parallelIndex: info.parallelIndex, testId: info.testId, retry: info.retry, runName }
      const environmentName = options.name?.(identity) ?? `${runName}-w${info.workerIndex}-${info.testId ? `t${info.testId}-r${info.retry}` : 'worker'}`
      const lifecycle = options.lifecycle({ ...identity, environmentName })
      await withEnvironmentLease(lifecycle, info.workerIndex, use, { closeDriver: lifecycle.close?.bind(lifecycle) })
    }, { scope: options.scope }],
    baseURL: async ({ environmentLease }: { environmentLease: E }, use: (baseURL: string) => Promise<void>) => { await use(environmentLease.baseURL) },
  }
  // The runtime scope selects which fixture namespace owns the same lease;
  // overloads preserve that namespace for the consumer's injected base test.
  return base.extend(fixtures as unknown as Parameters<typeof base.extend>[0]) as ReturnType<typeof createEnvironmentLeaseTest<T, W, E>>
}

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
      await preservingCleanup(async () => {
        await use(async input => {
          const scope = `${environment.runId}-w${info.workerIndex}-r${info.retry}-${crypto.randomUUID()}`
          scopes.push(scope)
          return control.prepare(input, scope)
        })
      }, async () => {
        const results = await Promise.allSettled(scopes.map(scope => control.dispose(scope)))
        const errors = results.filter((result): result is PromiseRejectedResult => result.status === 'rejected').map(result => result.reason)
        if (errors.length) throw new AggregateError(errors, 'Scenario teardown failed')
      })
    },
  })
}
