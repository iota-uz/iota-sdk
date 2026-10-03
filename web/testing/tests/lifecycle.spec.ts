import { test, expect } from '@playwright/test'
import { acquireEnvironmentLease, lazyIdentities, withEnvironment, withScenario } from '../src/index.js'

// Falsely green if prepare/use and disposal cannot both fail in the same scope.
test('preserves primary scenario failure alongside disposal failure', async () => {
  for (const phase of ['prepare', 'use'] as const) {
    const primary = new Error(phase)
    const cleanup = new Error('dispose')
    const result = await withScenario({
      prepare: async () => { if (phase === 'prepare') throw primary; return 'state' },
      dispose: async () => { throw cleanup },
    }, {}, 'owned', async () => { throw primary }).catch(error => error)
    expect(result).toBeInstanceOf(AggregateError)
    expect(result.errors).toEqual([primary, cleanup])
  }
})

// New API characterisation: falsely green if closing never crosses the lifecycle and driver boundaries.
test('acquired leases compensate startup/readiness and close each resource once', async () => {
  const effects: string[] = []
  const lifecycle = {
    start: async () => ({ runId: 'owned', baseURL: 'http://fixture.test', artifactDir: '/tmp/owned' }),
    ready: async () => {},
    stop: async () => { effects.push('stop') },
  }
  const options = { closeDriver: async () => { effects.push('driver') } }
  const lease = await acquireEnvironmentLease(lifecycle, 0, options)
  await Promise.all([lease.close(), lease.close()])
  expect(effects).toEqual(['stop', 'driver'])
  effects.length = 0
  await expect(acquireEnvironmentLease({ ...lifecycle, ready: async () => { throw new Error('ready') } }, 0, options)).rejects.toThrow('ready')
  expect(effects).toEqual(['stop', 'driver'])
  effects.length = 0
  await expect(acquireEnvironmentLease({ ...lifecycle, start: async () => { throw new Error('start') } }, 0, options)).rejects.toThrow('start')
  expect(effects).toEqual(['driver'])
})

// Falsely green if only teardown failures or only successful teardown are exercised.
test('preserves readiness and assertion failures when teardown also fails', async () => {
  for (const phase of ['ready', 'use'] as const) {
    const failure = new Error(phase)
    const stopFailure = new Error('stop')
    const result = withEnvironment({
      start: async () => ({ runId: 'run', baseURL: 'http://fixture.test', artifactDir: '/tmp/run' }),
      ready: async () => { if (phase === 'ready') throw failure },
      stop: async () => { throw stopFailure },
    }, 0, async () => { throw failure })
    const error = await result.catch(error => error)
    expect(error).toBeInstanceOf(AggregateError)
    expect(error.errors).toEqual([failure, stopFailure])
  }
})

// Characterisation: falsely green if only successful setup is exercised.
test('stops a started environment when readiness fails and cleans partially prepared scope', async () => {
  const effects: string[] = []
  await expect(withEnvironment({
    start: async () => ({ runId: 'run', baseURL: 'http://fixture.test', artifactDir: '/tmp/run' }),
    ready: async () => { throw new Error('not ready') },
    stop: async () => { effects.push('stop') },
  }, 0, async () => { effects.push('use') })).rejects.toThrow('not ready')
  await expect(withScenario({
    prepare: async () => { effects.push('prepare'); throw new Error('partial setup') },
    dispose: async scope => { effects.push(`dispose:${scope}`) },
  }, {}, 'scope', async () => {})).rejects.toThrow('partial setup')
  expect(effects).toEqual(['stop', 'prepare', 'dispose:scope'])
})
// Characterisation: falsely green if callers never concurrently request an identity or retry failure.
test('identities are lazy, deduplicated and retry failed creation', async () => {
  let calls = 0
  const identity = lazyIdentities(async (key: string) => { calls++; if (calls === 1) throw new Error('login failed'); return key })
  expect(calls).toBe(0)
  await expect(identity('admin')).rejects.toThrow('login failed')
  const states = await Promise.all([identity('admin'), identity('admin')])
  expect(states).toEqual(['admin', 'admin'])
  expect(calls).toBe(2)
})
