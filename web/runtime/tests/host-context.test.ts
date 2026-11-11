// The Node host builds the context objects handlers receive. The host is compiled by its own tsconfig (Node types),
// so it is loaded dynamically here and described by the narrow shapes this test relies on.
import { describe, expect, it } from 'vitest'
import * as c from '../typetests/acme/contracts.ts'

type Call = { method: string; args: Record<string, unknown> }
interface Ctx {
  tenant: string
  user: { id: string; kind: 'initiator'; permissions: string[] }
  invocation: string
  trigger: 'ui'
}
interface Deps {
  invocation: string
  ctx: Ctx
  call: (method: string, args: Record<string, unknown>) => Promise<unknown>
  log: (level: string, message: string) => void
}
type Fn = (...args: never[]) => Promise<unknown>
interface HostContext {
  createTx(deps: Deps, handle: string, scope: { open: boolean }): Record<string, Fn> & { log: object }
  createQuery(deps: Deps): Record<string, Fn>
  createGuardContext(deps: Deps, handle: string, scope: { open: boolean }): Record<string, unknown>
  createCommandContext(deps: Deps): Record<string, unknown> & { query: Fn & Record<string, Fn> }
  createWorkflowContext(deps: Deps): Record<string, unknown> & { secrets: { get(name: string): Promise<string> } }
  createEndpointRequest(input: unknown): { rawBody: Uint8Array; text(): string; body: unknown; method: string }
}

const specifier = '../src/host/context.ts'
const host = (await import(/* @vite-ignore */ specifier)) as HostContext

function harness(reply: (c: Call) => unknown = () => null) {
  const calls: Call[] = []
  const deps: Deps = {
    invocation: 'inv-1',
    ctx: { tenant: 't1', user: { id: '7', kind: 'initiator', permissions: [] }, invocation: 'inv-1', trigger: 'ui' },
    call: async (method, args) => {
      calls.push({ method, args })
      return reply({ method, args })
    },
    log: () => undefined,
  }
  return { calls, deps }
}

const row = { key: '123e4567-e89b-42d3-a456-426614174000', createdAt: 'x', updatedAt: 'x', rowVersion: 1, name: 'Ali' }
const policyEntity = { id: 'acme.insurance.policy' }

describe('host context: rows carry a typed ref', () => {
  it('adds ref to get, find, insert and update results, for tx and snapshot readers', async () => {
    const { deps } = harness(({ method }) => (method.endsWith('find') ? [row, { ...row, key: 'k2' }] : row))
    const tx = host.createTx(deps, 'h1', { open: true })
    const want = { entity: 'acme.insurance.policy', key: row.key }
    expect(await tx.get!(policyEntity as never, { key: row.key } as never)).toMatchObject({ ref: want, name: 'Ali' })
    expect(await tx.insert!(policyEntity as never, {} as never)).toMatchObject({ ref: want })
    expect(await tx.update!(policyEntity as never, { key: row.key } as never, {} as never)).toMatchObject({ ref: want })
    expect(await tx.find!(policyEntity as never)).toMatchObject([{ ref: want }, { ref: { key: 'k2' } }])
    const q = host.createQuery(deps)
    expect(await q.get!(policyEntity as never, row.key as never)).toMatchObject({ ref: want })
  })

  it('leaves null, overlay rows and counts alone', async () => {
    const { deps } = harness(({ method }) => (method === 'tx.get' ? null : method === 'tx.putOverlay' ? { baseKey: 'b', points: 1 } : 3))
    const tx = host.createTx(deps, 'h1', { open: true })
    expect(await tx.get!(policyEntity as never, 'k' as never)).toBeNull()
    expect(await tx.count!(policyEntity as never)).toBe(3)
    expect(await tx.put!({ id: 'acme.insurance.clientExtra' } as never, 'b' as never, {} as never)).toEqual({ baseKey: 'b', points: 1 })
  })
})

describe('host context: queries, signals, logs, secrets', () => {
  it('forwards tx.query and q.query with the query id and input', async () => {
    const { deps, calls } = harness(() => ['r'])
    const tx = host.createTx(deps, 'h1', { open: true })
    expect(await tx.query!(c.Expiring as never, { before: '2026-01-01' } as never)).toEqual(['r'])
    expect(calls.at(-1)).toEqual({ method: 'tx.query', args: { tx: 'h1', query: 'acme.insurance.expiring', input: { before: '2026-01-01' } } })
    await host.createQuery(deps).query!(c.Expiring as never, { before: 'x' } as never)
    expect(calls.at(-1)).toEqual({ method: 'q.query', args: { query: 'acme.insurance.expiring', input: { before: 'x' } } })
  })

  it('makes ctx.query callable while it stays the read-only snapshot context', async () => {
    const { deps, calls } = harness(() => 'ok')
    const ctx = host.createCommandContext(deps)
    expect(await (ctx.query as unknown as (q: never, i: never) => Promise<unknown>)(c.Expiring as never, { before: 'x' } as never)).toBe('ok')
    expect(calls.at(-1)?.method).toBe('q.query')
    await ctx.query.get!(policyEntity as never, 'k' as never)
    expect(calls.at(-1)?.method).toBe('q.get')
    expect(Object.keys(ctx.query).sort()).toEqual(['count', 'find', 'get', 'invocation', 'log', 'query', 'user'])
  })

  it('forwards signals: tx.signal in the transaction, ctx.signal as a new invocation', async () => {
    const { deps, calls } = harness()
    const instance = { workflow: 'acme.insurance.renewalFollowUp', instance: 'i-9' }
    await host.createTx(deps, 'h1', { open: true }).signal!(instance as never, 'decision' as never, { approved: true } as never)
    expect(calls.at(-1)).toEqual({
      method: 'tx.signal',
      args: { tx: 'h1', workflow: instance.workflow, instance: 'i-9', name: 'decision', payload: { approved: true } },
    })
    await (host.createCommandContext(deps).signal as unknown as Fn)(instance as never, 'decision' as never, { approved: false } as never)
    expect(calls.at(-1)).toEqual({
      method: 'invoke.signal',
      args: { workflow: instance.workflow, instance: 'i-9', name: 'decision', payload: { approved: false } },
    })
  })

  it('gives tx a log and workflows secrets', async () => {
    const logs: string[] = []
    const { deps } = harness(() => 's3cret')
    deps.log = (level, message) => logs.push(`${level}:${message}`)
    const tx = host.createTx(deps, 'h1', { open: true })
    ;(tx.log as { info(m: string, f?: object): void }).info('hi', { a: 1 })
    expect(logs).toEqual(['info:hi {"a":1}'])
    expect(await host.createWorkflowContext(deps).secrets.get('SMS_TOKEN')).toBe('s3cret')
  })

  it('refuses tx calls after the transaction closed', async () => {
    const { deps } = harness()
    const scope = { open: true }
    const tx = host.createTx(deps, 'h1', scope)
    scope.open = false
    await expect(tx.query!(c.Expiring as never, {} as never)).rejects.toMatchObject({ code: 'runtime.txClosed' })
  })
})

describe('host context: guards are read-only', () => {
  it('exposes reads, queries, identity and log, and no write API', async () => {
    const { deps, calls } = harness(() => null)
    const guard = host.createGuardContext(deps, 'h1', { open: true })
    expect(Object.keys(guard).sort()).toEqual(['count', 'find', 'get', 'invocation', 'log', 'query', 'user'])
    for (const write of ['insert', 'update', 'delete', 'put', 'call', 'emit', 'notify', 'start', 'signal']) expect(guard[write]).toBeUndefined()
    await (guard.get as Fn)(policyEntity as never, 'k' as never)
    expect(calls.at(-1)).toMatchObject({ method: 'tx.get', args: { tx: 'h1' } })
  })
})

describe('host context: endpoint request', () => {
  it('exposes the exact bytes and their text next to the parsed body', () => {
    const bytes = Buffer.from('{"policy":"x"}\n', 'utf8')
    const req = host.createEndpointRequest({ method: 'POST', path: '/hook', headers: {}, query: {}, body: { policy: 'x' }, rawBody: bytes.toString('base64') })
    expect(req.method).toBe('POST')
    expect(req.body).toEqual({ policy: 'x' })
    expect(Buffer.from(req.rawBody).equals(bytes)).toBe(true)
    expect(req.text()).toBe('{"policy":"x"}\n')
  })

  it('handles non-JSON payloads and derives bytes from the body when none are sent', () => {
    const form = host.createEndpointRequest({ method: 'POST', path: '/hook', rawBody: Buffer.from('a=1&b=%D0%B6', 'utf8').toString('base64') })
    expect(form.body).toBeUndefined()
    expect(form.text()).toBe('a=1&b=%D0%B6')
    expect(host.createEndpointRequest({ method: 'POST', path: '/p', body: { n: 1 } }).text()).toBe('{"n":1}')
    const empty = host.createEndpointRequest({})
    expect(empty.rawBody.byteLength).toBe(0)
    expect(empty.text()).toBe('')
  })
})
