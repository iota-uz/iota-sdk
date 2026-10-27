import { createServer } from 'node:http'
import { test, expect } from '@playwright/test'
import { createHttpScenarioControl, ScenarioControlError } from '../src/index.js'
// Characterisation: falsely green if only request constructors are inspected instead of real HTTP order, ownership and typed refusal.
test('reserves before preparation and propagates ownership and typed HTTP errors', async () => {
  const calls: string[] = []
  const server = createServer(async (request, response) => {
    const chunks: Buffer[] = []
    for await (const chunk of request) chunks.push(Buffer.from(chunk))
    calls.push(`${request.method} ${request.url}`)
    expect(request.headers.authorization).toBe(`Bearer ${'test-only-control-token-'.repeat(2)}`)
    response.setHeader('Content-Type', 'application/json')
    response.setHeader('X-Test-Environment-ID', request.url?.includes('foreign') ? 'foreign' : 'environment')
    if (request.method === 'DELETE' && request.headers['content-type']) { response.statusCode = 400; response.end(JSON.stringify({ code: 'invalid_input', message: 'empty JSON body' })); return }
    if (request.url?.includes('refused')) { response.statusCode = 409; response.end(JSON.stringify({ code: 'scope_conflict', message: 'owned scope unavailable' })); return }
    const body = chunks.length ? JSON.parse(Buffer.concat(chunks).toString()) : {}
    response.end(JSON.stringify({ scopeId: body.scopeId, data: { ready: true } }))
  })
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve))
  const address = server.address()
  if (!address || typeof address === 'string') throw new Error('No HTTP address')
  const control = createHttpScenarioControl({ environmentId: 'environment', baseURL: `http://127.0.0.1:${address.port}` }, 'test-only-control-token-'.repeat(2))
  try {
    const result = await control.prepare({ name: 'pilot', version: '1', seed: 'known', now: '2026-10-02T00:00:00Z', params: {} }, 'environment-owned')
    expect(result.data).toEqual({ ready: true })
    await control.dispose('environment-owned')
    expect(calls).toEqual(['POST /__test__/scopes', 'POST /__test__/scenarios/prepare', 'DELETE /__test__/scopes/environment-owned'])
    await expect(control.dispose('foreign')).rejects.toMatchObject({ code: 'environment_mismatch' })
    try { await control.dispose('refused'); throw new Error('Expected refusal') }
    catch (error) { expect(error).toBeInstanceOf(ScenarioControlError); expect(error).toMatchObject({ code: 'scope_conflict', status: 409 }) }
  } finally { await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve())) }
})
