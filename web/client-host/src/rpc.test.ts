import { describe, expect, it } from 'vitest'
import { MemoryQueryCache, queryKey, stableSerialize } from './cache'
import { HostError } from './errors'
import { RPCClient, type RPCMethodContract, type RPCTransport } from './rpc'

describe('RPCClient', () => {
  it('constructs stable keys and suppresses duplicate cache calls', async () => {
    expect(stableSerialize({ b: 2, a: { d: 4, c: 3 } })).toBe('{"a":{"c":3,"d":4},"b":2}')
    const calls: unknown[] = []
    const transport: RPCTransport = { call: async <TRequest, TResponse>(_method: string, request: TRequest) => { calls.push(request); return { ok: true } as TResponse } }
    const client = new RPCClient(transport)
    const contract: RPCMethodContract = { namespace: 'users', method: 'users.list', kind: 'query', cacheable: true }
    const signal = new AbortController().signal
    await client.query(contract, { page: 1 }, signal)
    await client.query(contract, { page: 1 }, signal)
    expect(calls).toHaveLength(1)
    expect(queryKey('users', 'users.list', { page: 1 })).toEqual(['users', 'users.list', '{"page":1}'])
  })

  it('retries only transient queries and never mutations', async () => {
    let queryCalls = 0
    const transport: RPCTransport = { call: async <TRequest, TResponse>() => {
      queryCalls += 1
      if (queryCalls < 3) throw new HostError('transient', 'later')
      return 'ok' as TResponse
    } }
    const client = new RPCClient(transport)
    await expect(client.query({ namespace: 'x', method: 'x.get', kind: 'query' }, {}, new AbortController().signal)).resolves.toBe('ok')
    expect(queryCalls).toBe(3)

    let mutationCalls = 0
    const failing = new RPCClient({ call: async <TRequest, TResponse>() => { mutationCalls += 1; throw new HostError('transient', 'no retry') } })
    await expect(failing.mutate({ namespace: 'x', method: 'x.save', kind: 'mutation' }, {}, new AbortController().signal)).rejects.toThrow('no retry')
    expect(mutationCalls).toBe(1)
  })

  it('invalidates only declared query families', async () => {
    const cache = new MemoryQueryCache()
    cache.set(queryKey('users', 'users.list', {}), 1)
    cache.set(queryKey('users', 'users.stats', {}), 2)
    cache.set(queryKey('roles', 'roles.list', {}), 3)
    const client = new RPCClient({ call: async <TRequest, TResponse>() => ({ id: 1 }) as TResponse }, { cache })
    await client.mutate({ namespace: 'users', method: 'users.save', kind: 'mutation', invalidates: ['users.list'] }, {}, new AbortController().signal)
    expect(cache.get(queryKey('users', 'users.list', {}))).toBeUndefined()
    expect(cache.get(queryKey('users', 'users.stats', {}))).toBe(2)
    expect(cache.get(queryKey('roles', 'roles.list', {}))).toBe(3)
  })
})
