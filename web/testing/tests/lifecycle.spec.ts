import { test, expect } from '@playwright/test'
import { lazyIdentities, withEnvironment, withScenario } from '../src/index.js'

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
