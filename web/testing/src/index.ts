export { type WaitOptions } from './wait.js'
export { actionAndResponse, actionAndHtmxSettled, actionAndHtmxResponse, actionAndValidationRefusal, type RequestMatch, type ResponseMatch, type ValidationResult, type HtmxSettledOptions } from './htmx.js'
export { expectComponentReady, expectFontsReady, expectChartsReady, openComponentCase, componentCase, assertCaseCoverage, attachDiagnosticScreenshot, ReadinessError, type ComponentCase, type ComponentHost, type ReadinessOptions, type ReadinessState, type CaseDimensions } from './components.js'

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
  return withEnvironmentLease(lifecycle, workerIndex, use)
}
export async function withScenario<Input, State, Result>(control: ScenarioControl<Input, State>, input: Input, scopeId: string, use: (state: State) => Promise<Result>): Promise<Result> {
  return preservingCleanup(async () => use(await control.prepare(input, scopeId)), () => control.dispose(scopeId))
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
export { createEnvironmentTest, createEnvironmentLeaseTest, type EnvironmentLeaseInfo } from './fixtures.js'
export { createProcessEnvironmentDriver, DriverError, type DriverDescriptor } from './driver.js'
import { preservingCleanup, withEnvironmentLease } from './lease.js'
export { acquireEnvironmentLease, withEnvironmentLease, preservingCleanup, type EnvironmentLease, type LeaseOptions } from './lease.js'

export { createHttpScenarioControl, createHttpTestControls, ScenarioControlError, JobWaitError, type ScenarioInput, type ScenarioResult, type EntityRef, type ExecutionStatus, type WaitResult, type ProviderStep, type ProviderCall, type ScenarioManifest } from './scenarios.js'
export { expectNoA11yViolations, A11yViolationError, A11yDependencyError, blockingA11yImpacts, type A11yCheckOptions } from './a11y.js'
