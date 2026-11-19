import { createServer } from 'node:http'
import { test, expect } from '@playwright/test'
import { createHttpTestControls, JobWaitError, ScenarioControlError } from '../src/index.js'
// Characterisation of the Go control wire shapes (pkg/testenv/http.go). Falsely green if a
// timeout wait resolved like success, or if diagnostics were dropped from the typed error.
test('scoped jobs, provider gates, clock and replay use the owned routes and typed errors', async () => {
  const token = 'test-only-control-token-'.repeat(2)
  const calls: string[] = []
  const server = createServer(async (request, response) => {
    const chunks: Buffer[] = []
    for await (const chunk of request) chunks.push(Buffer.from(chunk))
    const body = chunks.length ? JSON.parse(Buffer.concat(chunks).toString()) : undefined
    calls.push(`${request.method} ${request.url}${body === undefined ? '' : ' ' + JSON.stringify(body)}`)
    expect(request.headers.authorization).toBe(`Bearer ${token}`)
    response.setHeader('Content-Type', 'application/json')
    response.setHeader('X-Test-Environment-ID', 'env')
    const execution = { executionId: 'x1', scopeId: 'env-1', job: 'dun', state: 'succeeded', startedAt: '2026-10-02T00:00:00Z' }
    const reply = (status: number, value: unknown) => { response.statusCode = status; response.end(JSON.stringify(value)) }
    switch (`${request.method} ${request.url}`) {
      case 'POST /__test__/scopes/env-1/jobs': return reply(200, { executionId: 'x1' })
      case 'POST /__test__/scopes/env-1/executions/x1/wait':
        if (body.condition === 'completed') return reply(200, { outcome: 'reached', condition: 'completed', execution, checks: 0, elapsedMs: 1 })
        return reply(504, { code: 'timeout', message: 'condition dunned not reached', details: { outcome: 'timeout', condition: 'dunned', execution, checks: 3, elapsedMs: 30 } })
      case 'POST /__test__/scopes/env-1/providers/notifier/scripts': return reply(200, { scripted: true })
      case 'POST /__test__/scopes/env-1/providers/notifier/gates/send/wait': return reply(200, { held: true })
      case 'POST /__test__/scopes/env-1/providers/notifier/gates/send/release': return reply(200, { released: true })
      case 'GET /__test__/scopes/env-1/providers/notifier/calls': return reply(200, { calls: [{ sequence: 1, operation: 'send', key: 'INV-1', outcome: 'held', correlation: 'env-1/x1', hold: 'send' }] })
      case 'POST /__test__/scopes/env-1/clock/advance': return reply(200, { now: '2026-10-03T00:00:00Z' })
      case 'POST /__test__/scopes/shared/clock/advance': return reply(412, { code: 'missing_capability', message: 'clock control requires a dedicated environment with a manual clock' })
      case 'GET /__test__/scopes/env-1/manifest': return reply(200, { format: 'iota.testenv.manifest/v1', environment: {}, scenario: { name: 'n', version: '1', scopeId: 'env-1', seed: 's', now: '2026-10-02T00:00:00Z', params: {} }, result: {}, controls: { clock: '2026-10-03T00:00:00Z' } })
      case 'POST /__test__/scopes': return reply(200, { scopeId: body.scopeId })
      case 'POST /__test__/scopes/env-2/replay': return reply(200, { scopeId: 'env-2', data: { replayed: true } })
      case 'GET /__test__/scopes/broken/manifest': response.statusCode = 200; response.end(''); return
      case 'GET /__test__/scopes/array/manifest': return reply(200, [])
    }
    reply(404, { code: 'unknown_scenario', message: 'unexpected route' })
  })
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve))
  const address = server.address()
  if (!address || typeof address === 'string') throw new Error('No HTTP address')
  const controls = createHttpTestControls({ environmentId: 'env', baseURL: `http://127.0.0.1:${address.port}` }, token)
  try {
    const executionId = await controls.jobs.run('env-1', 'dun', { invoice: { kind: 'invoice', id: 'INV-1' } })
    expect((await controls.jobs.wait('env-1', executionId, 'completed', 5000)).outcome).toBe('reached')
    const failure = await controls.jobs.wait('env-1', executionId, 'dunned', 30).catch(error => error)
    expect(failure).toBeInstanceOf(JobWaitError)
    expect(failure).toMatchObject({ code: 'timeout', status: 504, result: { outcome: 'timeout', checks: 3, execution: { state: 'succeeded' } } })
    expect(() => controls.jobs.wait('env-1', executionId, 'dunned', 120_001)).toThrow('timeoutMs')
    await controls.providers.script('env-1', 'notifier', [{ operation: 'send', outcome: 'success', hold: 'send', response: { messageId: 'm-1' } }])
    await controls.providers.waitHeld('env-1', 'notifier', 'send', 5000)
    expect(await controls.providers.calls('env-1', 'notifier')).toEqual([expect.objectContaining({ outcome: 'held', correlation: 'env-1/x1' })])
    await controls.providers.release('env-1', 'notifier', 'send')
    expect((await controls.clock.advance('env-1', 86_400_000)).toISOString()).toBe('2026-10-03T00:00:00.000Z')
    await expect(controls.clock.advance('shared', 1)).rejects.toMatchObject({ code: 'missing_capability', status: 412 })
    await expect(controls.clock.advance('shared', 1)).rejects.toBeInstanceOf(ScenarioControlError)
    const manifest = await controls.manifest('env-1')
    expect(await controls.replay('env-2', manifest)).toMatchObject({ scopeId: 'env-2' })
    await expect(controls.manifest('broken')).rejects.toMatchObject({ code: 'invalid_response' })
    await expect(controls.manifest('array')).rejects.toMatchObject({ code: 'invalid_response' })
    expect(calls.slice(-4, -2).map(call => call.split(' ').slice(0, 2).join(' '))).toEqual(['POST /__test__/scopes', 'POST /__test__/scopes/env-2/replay'])
  } finally { await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve())) }
})
