// The Node host builds the context objects handlers receive. The host is compiled by its own tsconfig (Node types),
// so it is loaded dynamically here and described by the narrow shapes this test relies on.
import { describe, expect, it } from 'vitest'
import * as c from '../typetests/acme/contracts.ts'

type Call = { method: string; args: Record<string, unknown> }
interface Ctx {
  occurrence?: unknown
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
  createWorkflowContext(deps: Deps): WorkflowHost
  createEndpointRequest(input: unknown): { rawBody: Uint8Array; text(): string; body: unknown; method: string }
}

interface WorkflowHost {
  secrets: { get(name: string): Promise<string> }
  step(name: string, effect: (step: { idempotencyKey: string }) => Promise<unknown>): Promise<unknown>
  transaction(name: string, body: (tx: Record<string, Fn>) => Promise<unknown>): Promise<unknown>
  call(name: string, command: { id: string }, input: unknown): Promise<unknown>
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

describe('host context: ordinary intents are refused while a Go transaction is open', () => {
  it('rejects ctx.call and ctx.signal inside ctx.transaction and allows them outside', async () => {
    const { deps, calls } = harness(({ method }) => (method === 'tx.begin' ? { tx: 'h1' } : null))
    const ctx = host.createCommandContext(deps) as unknown as {
      call(c: { id: string }, input: unknown): Promise<unknown>
      signal(i: { workflow: string; instance: string }, name: string, payload: unknown): Promise<void>
      transaction<T>(cb: (tx: Record<string, Fn>) => Promise<T>): Promise<T>
    }
    const instance = { workflow: 'acme.w', instance: 'i-1' }
    await ctx.transaction(async () => {
      await expect(ctx.call({ id: 'acme.insurance.renewPolicy' }, {})).rejects.toMatchObject({ code: 'runtime.intentInTransaction' })
      await expect(ctx.signal(instance, 'decision', {})).rejects.toMatchObject({ code: 'runtime.intentInTransaction' })
    })
    expect(calls.map((c) => c.method)).toEqual(['tx.begin', 'tx.end'])
    await ctx.call({ id: 'acme.insurance.renewPolicy' }, { n: 1 })
    await ctx.signal(instance, 'decision', {})
    expect(calls.map((c) => c.method).slice(2)).toEqual(['invoke.command', 'invoke.signal'])
    // the rejection is not sticky: a failed transaction also re-opens the gate
    await expect(ctx.transaction(async () => { throw new Error('boom') })).rejects.toThrow('boom')
    await ctx.call({ id: 'acme.insurance.renewPolicy' }, {})
    expect(calls.at(-1)?.method).toBe('invoke.command')
  })

  it('rejects wf.call while a wf.transaction is in flight and allows it after', async () => {
    let release!: () => void
    const gate = new Promise<void>((resolve) => { release = resolve })
    const { deps, calls } = harness(({ method }) => (method === 'wf.transaction.begin' ? { replay: false, tx: 'h1' } : method === 'wf.call' ? 'called' : null))
    const wf = host.createWorkflowContext(deps)
    const tx = wf.transaction('load', async () => { await gate; return 1 })
    await expect(wf.call('renew', { id: 'acme.insurance.renewPolicy' }, {})).rejects.toMatchObject({ code: 'runtime.intentInTransaction' })
    expect(calls.some((c) => c.method === 'wf.call')).toBe(false)
    release()
    await tx
    expect(await wf.call('renew', { id: 'acme.insurance.renewPolicy' }, {})).toBe('called')
    // a rejected call consumed no occurrence: the first renew is still n=0
    expect(calls.filter((c) => c.method === 'wf.call').map((c) => c.args.n)).toEqual([0])
  })
})

describe('host context: durable results are normalized to JSON on the first execution', () => {
  class Money {
    constructor(readonly amount: number, readonly at: Date) {}
    get label() { return `${this.amount}!` }
  }
  const produce = () => ({ when: new Date('2026-01-02T03:04:05.000Z'), money: new Money(5, new Date('2026-02-03T00:00:00.000Z')), skipped: undefined, nan: Number.NaN, list: [undefined, () => 1] })

  function recorder() {
    const recorded = new Map<string, unknown>()
    return {
      recorded,
      reply: ({ method, args }: Call): unknown => {
        if (method === 'wf.step.begin') return { replay: false, idempotencyKey: 'k' }
        if (method === 'wf.transaction.begin') return { replay: false, tx: 'h1' }
        if (method === 'wf.step.end' || method === 'wf.transaction.end') recorded.set(String(args.name), args.value)
        return null
      },
    }
  }

  const replayDeps = (kind: string, name: string, result: unknown): Deps => ({
    ...harness().deps,
    ctx: { ...harness().deps.ctx, occurrence: { journal: [{ name, n: 0, kind, status: 'completed', result }] } } as Ctx,
  })

  it('returns from wf.step the value it recorded, equal to the replayed one', async () => {
    const rec = recorder()
    const { deps } = harness(rec.reply)
    const first = await host.createWorkflowContext(deps).step('quote', async () => produce())
    const stored = rec.recorded.get('quote')
    expect(first).toEqual(stored)
    expect(first).toEqual({
      when: '2026-01-02T03:04:05.000Z',
      money: { amount: 5, at: '2026-02-03T00:00:00.000Z' },
      nan: null,
      list: [null, null],
    })
    const replayed = await host.createWorkflowContext(replayDeps('step', 'quote', JSON.parse(JSON.stringify(stored)))).step('quote', async () => {
      throw new Error('must not run on replay')
    })
    expect(replayed).toEqual(first)
  })

  it('does the same for wf.transaction callbacks and turns undefined into null', async () => {
    const rec = recorder()
    const { deps } = harness(rec.reply)
    const wf = host.createWorkflowContext(deps)
    const first = await wf.transaction('load', async () => produce())
    expect(first).toEqual(rec.recorded.get('load'))
    expect(first).toMatchObject({ when: '2026-01-02T03:04:05.000Z' })
    expect(await wf.transaction('none', async () => undefined)).toBeNull()
    const replayed = await host.createWorkflowContext(replayDeps('transaction', 'load', JSON.parse(JSON.stringify(rec.recorded.get('load'))))).transaction('load', async () => {
      throw new Error('must not run on replay')
    })
    expect(replayed).toEqual(first)
  })

  it('fails a step whose result has no JSON form instead of recording something else', async () => {
    const rec = recorder()
    const { deps, calls } = harness(rec.reply)
    await expect(host.createWorkflowContext(deps).step('big', async () => ({ n: 1n }))).rejects.toMatchObject({ code: 'runtime.resultNotJson' })
    expect(calls.find((c) => c.method === 'wf.step.end')?.args).toMatchObject({ ok: false })
  })
})
