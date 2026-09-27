// @vitest-environment jsdom
import { createRoot, createSignal } from 'solid-js'
import { describe, expect, it, vi } from 'vitest'
import { HostError } from './errors'
import { RPCClient, type RPCTransport } from './rpc-core'
import { createRPCAction, createRPCQuery } from './solid-rpc'

describe('Solid RPC bindings', () => {
  it('cancels the prior reactive query and suppresses its late response', async () => {
    const pending = new Map<number, (value: string) => void>()
    const transport: RPCTransport = { call: async <TRequest, TResponse>(_method: string, request: TRequest) => new Promise<TResponse>((resolve) => pending.set((request as { id: number }).id, resolve as (value: string) => void)) }
    const { query, setRequest, dispose } = createRoot((dispose) => {
      const [request, setRequest] = createSignal({ id: 1 })
      const query = createRPCQuery<{ id: number }, string>(new RPCClient(transport), { namespace: 'product', method: 'product.get', kind: 'query' }, request)
      return { query, setRequest, dispose }
    })
    await vi.waitFor(() => expect(pending.has(1)).toBe(true))
    setRequest({ id: 2 })
    await vi.waitFor(() => expect(pending.has(2)).toBe(true))
    pending.get(1)?.('old')
    pending.get(2)?.('new')
    await vi.waitFor(() => expect(query.state()).toEqual({ status: 'success', data: 'new' }))
    dispose()
  })

  it('surfaces mutation field errors and does not retry', async () => {
    const { action, dispose } = createRoot((dispose) => {
      let calls = 0
      const client = new RPCClient({ call: async () => { calls += 1; throw new HostError('field_validation', 'Invalid', { 'terms.0.rate': 'Too low' }) } })
      const action = createRPCAction(client, { namespace: 'product', method: 'product.save', kind: 'mutation' })
      return { action, dispose, calls: () => calls }
    })
    await expect(action.execute({})).rejects.toThrow('Invalid')
    expect(action.fieldErrors()).toEqual({ 'terms.0.rate': 'Too low' })
    dispose()
  })
})
