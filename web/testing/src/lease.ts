import type { EnvironmentDescriptor, EnvironmentLifecycle } from './index.js'

export interface EnvironmentLease<E extends EnvironmentDescriptor> {
  environment: E
  close(): Promise<void>
}
export interface LeaseOptions { closeDriver?(): Promise<void> }

export async function preservingCleanup<Result>(use: () => Promise<Result>, cleanup: () => Promise<void>): Promise<Result> {
  let failed = false
  let failure: unknown
  try { return await use() }
  catch (error) { failed = true; failure = error; throw error }
  finally {
    try { await cleanup() }
    catch (error) {
      if (failed) throw new AggregateError([failure, error], 'Operation and cleanup failed')
      throw error
    }
  }
}

export async function acquireEnvironmentLease<E extends EnvironmentDescriptor>(lifecycle: EnvironmentLifecycle<E>, workerIndex = 0, options: LeaseOptions = {}): Promise<EnvironmentLease<E>> {
  let environment: E
  try { environment = await lifecycle.start(workerIndex) }
  catch (error) {
    return preservingCleanup(async () => { throw error }, async () => { await options.closeDriver?.() })
  }
  let closing: Promise<void> | undefined
  const close = () => closing ??= preservingCleanup(() => lifecycle.stop(environment), async () => { await options.closeDriver?.() })
  try { await lifecycle.ready(environment) }
  catch (error) { return preservingCleanup(async () => { throw error }, close) }
  return { environment, close }
}

export async function withEnvironmentLease<E extends EnvironmentDescriptor, Result>(lifecycle: EnvironmentLifecycle<E>, workerIndex: number, use: (environment: E) => Promise<Result>, options: LeaseOptions = {}): Promise<Result> {
  const lease = await acquireEnvironmentLease(lifecycle, workerIndex, options)
  return preservingCleanup(() => use(lease.environment), lease.close)
}
