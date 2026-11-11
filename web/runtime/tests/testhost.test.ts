import { describe, expect, it } from 'vitest'
import { $, decimal, isoDate, ref } from '../src/index.ts'
import { implement, subscribe } from '../src/server.ts'
import { createTestHost, fixture, overlayFixture, TestHostError, test as registerTest, describe as group, UnmatchedRequestError } from '../src/testing.ts'
import * as c from '../typetests/acme/contracts.ts'
import * as s from '../typetests/acme/server.ts'
import * as kit from '../typetests/kit.ts'

const READ = c.ReadPolicies.id
const premium = (v: string) => decimal(v)
const day = (v: string) => isoDate(v)

function world() {
  const owner = ref(kit.User, '42')
  const ali = fixture(c.Customer, { name: 'Ali', owner })
  const vali = fixture(c.Customer, { name: 'Vali' })
  const mk = (number: string, holder: typeof ali, expiresOn: string, status: 'draft' | 'active' | 'expired', notes?: string) =>
    fixture(c.Policy, { number, holder: holder.ref, premium: premium('10.00'), expiresOn: day(expiresOn), status, ...(notes === undefined ? {} : { notes }) })
  const p1 = mk('P-1', ali, '2027-03-01', 'active')
  const p2 = mk('P-2', ali, '2027-01-01', 'active', 'late')
  const p3 = mk('P-3', vali, '2027-02-01', 'draft')
  const p4 = mk('P-4', vali, '2027-04-01', 'expired')
  return { owner, ali, vali, p1, p2, p3, p4, all: [ali, vali, p1, p2, p3, p4] }
}

const numbers = (rows: readonly { number: string }[]) => rows.map((r) => r.number)

describe('reads follow the Go data layer', () => {
  it('filters with SQL null semantics, operators and nested or', async () => {
    const w = world()
    const host = createTestHost({ fixtures: w.all, entities: [c.Customer], user: { permissions: [READ] } })
    const q = host.query()
    const find = (where: object, rest: object = {}) => q.find(c.Policy, { where, orderBy: [{ field: 'number' }], ...rest } as never).then(numbers)

    expect(await find({ status: 'active' })).toEqual(['P-1', 'P-2'])
    expect(await find({ status: { ne: 'active' } })).toEqual(['P-3', 'P-4'])
    expect(await find({ status: { in: ['draft', 'expired'] } })).toEqual(['P-3', 'P-4'])
    expect(await find({ status: { in: [] } })).toEqual([])
    expect(await find({ expiresOn: { gte: day('2027-02-01'), lt: day('2027-04-01') } })).toEqual(['P-1', 'P-3'])
    expect(await find({ number: { like: 'P-_' } })).toEqual(['P-1', 'P-2', 'P-3', 'P-4'])
    expect(await find({ number: { like: '%3' } })).toEqual(['P-3'])
    expect(await find({ notes: { isNull: true } })).toEqual(['P-1', 'P-3', 'P-4'])
    expect(await find({ notes: { isNull: false } })).toEqual(['P-2'])
    // x <> 'late' is IS DISTINCT FROM: rows with a null note match; x = null is IS NULL.
    expect(await find({ notes: { ne: 'late' } })).toEqual(['P-1', 'P-3', 'P-4'])
    expect(await find({ notes: { eq: null } })).toEqual(['P-1', 'P-3', 'P-4'])
    expect(await find({ notes: { lt: 'z' } })).toEqual(['P-2'])
    expect(await find({ or: [{ status: 'draft' }, { number: 'P-4' }] })).toEqual(['P-3', 'P-4'])
    expect(await find({ status: 'active', or: [{ number: 'P-1' }, { number: 'P-3' }] })).toEqual(['P-1'])
  })

  it('walks ref paths as left joins and applies the user placeholder', async () => {
    const w = world()
    const host = createTestHost({ fixtures: w.all, entities: [c.Customer], user: { id: '42', permissions: [READ] } })
    const q = host.query()
    expect(numbers(await q.find(c.Policy, { where: { 'holder.owner': w.owner }, orderBy: [{ field: 'number' }] }))).toEqual(['P-1', 'P-2'])
    expect(numbers(await q.find(c.Policy, { where: { 'holder.owner': $.user }, orderBy: [{ field: 'number' }] }))).toEqual(['P-1', 'P-2'])
    expect(numbers(await q.find(c.Policy, { where: { 'holder.owner': { isNull: true } }, orderBy: [{ field: 'number' }] }))).toEqual(['P-3', 'P-4'])
    expect(numbers(await q.find(c.Policy, { orderBy: [{ field: 'holder.name', desc: true }, { field: 'number' }] }))).toEqual(['P-3', 'P-4', 'P-1', 'P-2'])
  })

  it('orders with nulls last ascending and first descending, key as tiebreak, and pages', async () => {
    const w = world()
    const host = createTestHost({ fixtures: w.all, user: { permissions: [READ] } })
    const q = host.query()
    expect(numbers(await q.find(c.Policy, { orderBy: [{ field: 'notes' }, { field: 'number' }] }))).toEqual(['P-2', 'P-1', 'P-3', 'P-4'])
    expect(numbers(await q.find(c.Policy, { orderBy: [{ field: 'notes', desc: true }, { field: 'number' }] }))).toEqual(['P-1', 'P-3', 'P-4', 'P-2'])
    expect(numbers(await q.find(c.Policy, { orderBy: [{ field: 'number' }], limit: 2, offset: 1 }))).toEqual(['P-2', 'P-3'])
    expect(await q.count(c.Policy, { status: 'active' })).toBe(2)
    await expect(q.find(c.Policy, { limit: -1 })).rejects.toMatchObject({ code: 'data.invalidQuery' })
    await expect(q.find(c.Policy, { where: { nope: 1 } as never })).rejects.toMatchObject({ code: 'data.unknownField' })
    await expect(q.find(c.Policy, { where: { extra: { lt: 1 } } as never })).rejects.toMatchObject({ code: 'data.invalidQuery' })
    await expect(q.find(c.Policy, { where: { status: { like: 'a%' }, expiresOn: { like: 'x' } } as never })).rejects.toMatchObject({ code: 'data.invalidQuery' })
  })

  it('clamps the page size to 100 by default and 1000 at most', async () => {
    const many = Array.from({ length: 1100 }, (_, i) => fixture(c.Customer, { name: `c${String(i).padStart(4, '0')}` }))
    const host = createTestHost({ fixtures: many })
    const q = host.query()
    expect(await q.find(c.Customer, {})).toHaveLength(100)
    expect(await q.find(c.Customer, { limit: 5000 })).toHaveLength(1000)
    expect(await q.count(c.Customer)).toBe(1100)
  })

  it('applies read access before filter, paging and counts', async () => {
    const w = world()
    const as = (permissions: string[], id = '1') => createTestHost({ fixtures: w.all, entities: [c.Customer], user: { id, permissions } }).query()
    // rule 1: ReadPolicies sees all rows; rule 2: rows whose holder is owned by the subject.
    expect(await as([READ]).count(c.Policy)).toBe(4)
    expect(await as([], '42').count(c.Policy)).toBe(2)
    expect(await as([], '7').count(c.Policy)).toBe(0)
    expect(numbers(await as([], '42').find(c.Policy, { orderBy: [{ field: 'number' }], limit: 1, offset: 1 }))).toEqual(['P-2'])
    expect(await as([], '7').get(c.Policy, w.p1.ref)).toBeNull()
    expect(await as([], '42').get(c.Policy, w.p1.ref)).toMatchObject({ number: 'P-1' })
  })

  it('reads overlays as null when absent, rows when present', async () => {
    const client = fixture(kit.Client, { name: 'Zed' }, { key: 'client-1' })
    const other = fixture(kit.Client, { name: 'Yan' }, { key: 'client-2' })
    const host = createTestHost({ fixtures: [client, other, overlayFixture(c.ClientExtra, client.ref, { loyaltyTier: 'gold' })] })
    expect(await host.query().get(c.ClientExtra, client.ref)).toEqual({ baseKey: 'client-1', loyaltyTier: 'gold', points: 0 })
    expect(await host.query().get(c.ClientExtra, other.ref)).toBeNull()
  })
})

describe('writes and contexts', () => {
  it('inserts with defaults, validates, enforces unique and row versions', async () => {
    const host = createTestHost({ user: { permissions: [READ] } })
    const ali = await host.transaction((tx) => tx.insert(c.Customer, { name: 'Ali' }))
    expect(ali).toMatchObject({ name: 'Ali', phone: null, owner: null, rowVersion: 1 })
    const holder = ref(c.Customer, ali.key)
    const policy = await host.transaction((tx) => tx.insert(c.Policy, { number: 'N-1', holder, premium: premium('1.50'), expiresOn: day('2027-01-01') }))
    expect(policy.status).toBe('draft')
    await expect(host.transaction((tx) => tx.insert(c.Policy, { number: 'N-1', holder, premium: premium('2.00'), expiresOn: day('2027-01-02') }))).rejects.toMatchObject({ code: 'data.uniqueViolation' })
    await expect(host.transaction((tx) => tx.insert(c.Policy, { number: 'N-2', holder, premium: 5 as never, expiresOn: day('2027-01-02') }))).rejects.toMatchObject({ code: 'data.invalidValue' })
    const updated = await host.transaction((tx) => tx.update(c.Policy, ref(c.Policy, policy.key), { status: 'active' }, { rowVersion: 1 }))
    expect(updated).toMatchObject({ status: 'active', rowVersion: 2 })
    await expect(host.transaction((tx) => tx.update(c.Policy, ref(c.Policy, policy.key), { status: 'expired' }, { rowVersion: 1 }))).rejects.toMatchObject({ code: 'data.versionConflict' })
    await expect(host.transaction((tx) => tx.update(c.Policy, ref(c.Policy, 'nope'), { status: 'expired' }))).rejects.toMatchObject({ code: 'data.notFound' })
    await host.transaction((tx) => tx.delete(c.Policy, ref(c.Policy, policy.key)))
    expect(host.table(c.Policy)).toHaveLength(0)
  })

  it('rejects generic writes to native entities', async () => {
    const client = fixture(kit.Client, { name: 'Zed' }, { key: 'k' })
    const host = createTestHost({ fixtures: [client] })
    const tx = host.transaction.bind(host)
    // @ts-expect-error native entities are not OwnEntity
    await expect(tx((x) => x.insert(kit.Client, { name: 'n' }))).rejects.toMatchObject({ code: 'data.nativeWrite' })
    // @ts-expect-error
    await expect(tx((x) => x.update(kit.Client, client.ref, { name: 'n' }))).rejects.toMatchObject({ code: 'data.nativeWrite' })
    // @ts-expect-error
    await expect(tx((x) => x.delete(kit.Client, client.ref))).rejects.toMatchObject({ code: 'data.nativeWrite' })
  })

  it('upserts overlay rows keeping omitted fields', async () => {
    const client = fixture(kit.Client, { name: 'Zed' }, { key: 'k' })
    const host = createTestHost({ fixtures: [client] })
    await host.transaction((tx) => tx.put(c.ClientExtra, client.ref, { loyaltyTier: 'silver' }))
    expect(host.overlayRow(c.ClientExtra, client.ref)).toEqual({ baseKey: 'k', loyaltyTier: 'silver', points: 0 })
    await host.transaction((tx) => tx.put(c.ClientExtra, client.ref, { points: 5 }))
    expect(host.overlayRow(c.ClientExtra, client.ref)).toEqual({ baseKey: 'k', loyaltyTier: 'silver', points: 5 })
  })

  it('commits or rolls back data and intents together', async () => {
    const w = world()
    const host = createTestHost({ fixtures: w.all, user: { permissions: [READ] } })
    await expect(
      host.transaction(async (tx) => {
        await tx.update(c.Policy, w.p3.ref, { status: 'active' })
        await tx.emit(c.PolicyRenewed, { subject: w.p3.ref, payload: { expiresOn: day('2028-01-01'), premium: premium('1.00'), version: '1' as never } })
        await tx.start(c.RenewalFollowUp, { policy: w.p3.ref }, { key: 'k1' })
        throw new Error('boom')
      }),
    ).rejects.toThrow('boom')
    expect(host.emitted).toHaveLength(0)
    expect(host.started).toHaveLength(0)
    expect(await host.query().get(c.Policy, w.p3.ref)).toMatchObject({ status: 'draft' })

    await host.transaction(async (tx) => {
      await tx.update(c.Policy, w.p3.ref, { status: 'active' })
      await tx.emit(c.PolicyRenewed, { subject: w.p3.ref, payload: { expiresOn: day('2028-01-01'), premium: premium('1.00'), version: '1' as never } })
      await tx.notify(c.RenewalReminder, { subject: w.p3.ref, payload: { number: 'P-3', expiresOn: day('2028-01-01') }, to: [{ kind: 'address', address: { sms: '+998901234567' } }], channels: ['sms'] })
      await tx.start(c.RenewalFollowUp, { policy: w.p3.ref }, { key: 'k1' })
      await tx.start(c.RenewalFollowUp, { policy: w.p3.ref }, { key: 'k1' })
    })
    expect(await host.query().get(c.Policy, w.p3.ref)).toMatchObject({ status: 'active' })
    expect(host.emitted).toHaveLength(1)
    expect(host.emitted[0]).toMatchObject({ event: c.PolicyRenewed.id, subject: w.p3.ref })
    expect(host.notices).toHaveLength(1)
    expect(host.notices[0]).toMatchObject({ notice: c.RenewalReminder.id, channels: ['sms'] })
    expect(host.started).toHaveLength(1)
    expect(host.started[0]).toMatchObject({ workflow: c.RenewalFollowUp.id, key: 'k1' })
  })

  it('requires a start key outside subscriptions and derives it inside', async () => {
    const w = world()
    const host = createTestHost({ fixtures: w.all, user: { permissions: [READ] } })
    // @ts-expect-error the key is required in a plain transaction
    await expect(host.transaction((tx) => tx.start(c.RenewalFollowUp, { policy: w.p1.ref }))).rejects.toMatchObject({ code: 'runtime.startKeyRequired' })
    await host.deliver(s.onRenewed, { subject: w.p1.ref, payload: { expiresOn: '2028-01-01', premium: '1.00', version: '1' } })
    expect(host.started).toHaveLength(1)
  })

  it('tx.call accepts operations and atomic commands only, and joins the transaction', async () => {
    const w = world()
    const guards = [s.extendGuard]
    const host = createTestHost({ fixtures: w.all, entities: [c.Customer], implementations: [s.extendExpiry, s.activate], guards, user: { permissions: [READ] } })
    // @ts-expect-error an ordinary command is not callable inside a transaction
    await expect(host.transaction((tx) => tx.call(c.RenewPolicy, { policy: w.p1.ref }))).rejects.toMatchObject({ code: 'runtime.callNotAtomic' })
    // @ts-expect-error queries are not callable either
    await expect(host.transaction((tx) => tx.call(c.Expiring, { before: day('2030-01-01') }))).rejects.toMatchObject({ code: 'runtime.callNotAtomic' })
    const out = await host.transaction((tx) => tx.call(c.ExtendExpiry, { policy: w.p1.ref, months: 12 }))
    expect(out.expiresOn).toBe('2028-03-01')
    expect(host.emitted).toHaveLength(1)
    // the guard of the operation runs independently of the implementation
    await expect(host.transaction((tx) => tx.call(c.ExtendExpiry, { policy: w.p1.ref, months: 99 }))).rejects.toMatchObject({ kind: 'validation' })
    // a failing nested call rolls the whole transaction back
    const before = host.emitted.length
    await expect(
      host.transaction(async (tx) => {
        await tx.call(c.ExtendExpiry, { policy: w.p1.ref, months: 1 })
        await tx.call(c.ExtendExpiry, { policy: w.p1.ref, months: 99 })
      }),
    ).rejects.toBeDefined()
    expect(host.emitted).toHaveLength(before)
    expect(host.table(c.Policy).find((r) => r.number === 'P-1')?.expiresOn).toBe('2028-03-01')
  })

  it('ordinary commands: one transaction, never nested; external calls stay outside it', async () => {
    const w = world()
    const host = createTestHost({ fixtures: w.all, entities: [c.Customer], implementations: [s.renew, s.extendExpiry], user: { permissions: [READ] } })
    host.fakes.http.route('GET', 'https://quotes.example.uz/P-1', { status: 200, body: { premium: '99.00' } })
    const ctx = host.command()
    const orig = globalThis.fetch
    globalThis.fetch = (async (url: string) => {
      const res = await host.fakes.http.request({ method: 'GET', url, headers: {} })
      return { json: async () => res.body } as Response
    }) as typeof fetch
    try {
      const out = await s.renew.handler(ctx, { policy: w.p1.ref })
      expect(out).toEqual({ expiresOn: '2028-03-01', quote: '99.00' })
    } finally {
      globalThis.fetch = orig
    }
    await expect(ctx.transaction((a) => ctx.transaction(async () => a.count(c.Policy)))).rejects.toMatchObject({ code: 'runtime.nestedTransaction' })
    // the transaction handle is dead after the callback returns
    let leaked: Parameters<Parameters<typeof ctx.transaction>[0]>[0] | undefined
    await ctx.transaction(async (tx) => void (leaked = tx))
    await expect(leaked?.count(c.Policy)).rejects.toMatchObject({ code: 'runtime.txClosed' })
  })

  it('ctx.call dispatches ordinary and atomic commands through their bound implementations', async () => {
    const w = world()
    const precheckFail = createTestHost({
      fixtures: [...w.all],
      entities: [c.Customer],
      implementations: [s.renew, s.extendExpiry],
      guards: [s.renewPrecheck],
      user: { permissions: [READ] },
    })
    // the precheck runs on a read snapshot before the implementation
    await expect(precheckFail.command().call(c.RenewPolicy, { policy: w.p4.ref })).rejects.toMatchObject({ code: 'acme.insurance.notActive' })
    const host = createTestHost({ fixtures: w.all, entities: [c.Customer], implementations: [s.activate], user: { permissions: [READ] } })
    expect(await host.command().call(c.ActivatePolicy, { policy: w.p3.ref })).toEqual({ status: 'active' })
    expect(host.started).toHaveLength(1)
    await expect(createTestHost().command().call(c.ActivatePolicy, { policy: w.p3.ref })).rejects.toMatchObject({ code: 'test.noImplementation' })
  })

  it('runs entity write rules in the writing transaction', async () => {
    const w = world()
    const host = createTestHost({ fixtures: w.all, guards: [s.notesRule], user: { permissions: [READ] } })
    await expect(host.transaction((tx) => tx.update(c.Policy, w.p1.ref, { notes: 'much too long' }))).rejects.toMatchObject({ kind: 'validation' })
    expect((await host.transaction((tx) => tx.update(c.Policy, w.p1.ref, { notes: 'ok' }))).notes).toBe('ok')
  })

  it('provides secrets and logs only as declared', async () => {
    const host = createTestHost({ secrets: { SMS_TOKEN: 'x' } })
    const ctx = host.command()
    expect(await ctx.secrets.get('SMS_TOKEN')).toBe('x')
    await expect(ctx.secrets.get('OTHER')).rejects.toMatchObject({ code: 'runtime.secretUndeclared' })
    ctx.log.info('hello', { a: 1 })
    expect(host.logs).toEqual([{ level: 'info', message: 'hello {"a":1}' }])
  })
})

describe('workflows are driven by the test and replay their journal', () => {
  it('suspends on tasks, signals and sleeps and resumes when satisfied', async () => {
    const w = world()
    const host = createTestHost({ fixtures: w.all, entities: [c.Customer], implementations: [s.renew, s.extendExpiry], user: { permissions: [READ] } })
    host.fakes.http.route('GET', 'https://quotes.example.uz/P-1', { status: 200, body: { premium: '5.00' } })
    const globalFetch = globalThis.fetch
    globalThis.fetch = (async (url: string) => ({ json: async () => (await host.fakes.http.request({ method: 'GET', url, headers: {} })).body })) as unknown as typeof fetch
    try {
      const run = host.workflow(s.renewalFollowUp, { policy: w.p1.ref })
      let out = await run.run()
      expect(out).toMatchObject({ status: 'suspended', on: { kind: 'sleep', name: 'cooldown' } })
      expect(run.journal).toEqual(['load#0'])

      host.clock.advance('1d')
      out = await run.run()
      expect(out).toMatchObject({ status: 'suspended', on: { kind: 'task', name: 'approve' } })
      expect(run.tasks).toMatchObject([{ name: 'approve', n: 0, status: 'open' }])
      expect(() => run.decide('approve', { approved: 'yes' })).toThrow(TestHostError)

      run.decide('approve', { approved: true })
      out = await run.run()
      expect(out).toMatchObject({ status: 'suspended', on: { kind: 'signal', name: 'decision' } })
      expect(run.journal).toEqual(['load#0', 'cooldown#0', 'approve#0', 'renew#0', 'sms#0', 'log#0'])

      run.signal('decision', { approved: true })
      out = await run.run()
      expect(out).toEqual({ status: 'completed', value: { renewed: true } })
      expect(host.notices).toHaveLength(1)
      expect(host.table(c.Policy).find((r) => r.number === 'P-1')?.expiresOn).toBe('2028-03-01')
    } finally {
      globalThis.fetch = globalFetch
    }
  })

  it('times out tasks and signals by advancing the clock', async () => {
    const w = world()
    const host = createTestHost({ fixtures: w.all, user: { permissions: [READ] } })
    const run = host.workflow(s.renewalFollowUp, { policy: w.p1.ref })
    await run.run()
    host.clock.advance('1d')
    expect(await run.run()).toMatchObject({ status: 'suspended', on: { kind: 'task' } })
    host.clock.advance('3d')
    expect(await run.run()).toEqual({ status: 'completed', value: { renewed: false } })
  })

  it('journals occurrences by name and repeat count; steps are at-least-once with a stable key', async () => {
    const host = createTestHost()
    const wf = c.RenewalFollowUp
    const calls: string[] = []
    let attempts = 0
    const impl = implement(wf, async (ctx) => {
      for (let i = 0; i < 2; i++) {
        await ctx.step('send', async ({ idempotencyKey }) => {
          calls.push(idempotencyKey)
          return idempotencyKey
        })
      }
      const flaky = await ctx.step('flaky', async () => {
        if (++attempts === 1) throw new Error('provider down')
        return { ok: true }
      })
      const id = await ctx.uuid('id')
      const at = await ctx.now('at')
      return { renewed: flaky.ok && id.length > 0 && at.length > 0 }
    })
    const run = host.workflow(impl, { policy: ref(c.Policy, 'p') }, { instance: 'wf-x' })
    const first = await run.run()
    expect(first).toMatchObject({ status: 'failed' })
    expect(calls).toEqual(['wf-x:send:0', 'wf-x:send:1'])
    expect(await run.run()).toEqual({ status: 'completed', value: { renewed: true } })
    expect(calls).toEqual(['wf-x:send:0', 'wf-x:send:1']) // completed steps replay without executing again
    expect(attempts).toBe(2)
    expect(run.journal).toEqual(['send#0', 'send#1', 'flaky#0', 'id#0', 'at#0'])
  })

  it('detects non-deterministic replays', async () => {
    const host = createTestHost()
    let flip = false
    const impl = implement(c.RenewalFollowUp, async (ctx) => {
      if (flip) await ctx.now('a')
      else await ctx.uuid('a')
      await ctx.sleep('wait', '1h')
      return { renewed: true }
    })
    const run = host.workflow(impl, { policy: ref(c.Policy, 'p') })
    expect(await run.run()).toMatchObject({ status: 'suspended' })
    flip = true
    expect(await run.run()).toMatchObject({ status: 'failed', error: { code: 'workflow.nondeterminism' } })
  })

  it('starts child workflows and calls commands with receipts', async () => {
    const w = world()
    const host = createTestHost({ fixtures: w.all, implementations: [s.activate], user: { permissions: [READ] } })
    const impl = implement(c.RenewalFollowUp, async (ctx, input) => {
      const child = await ctx.start('child', c.RenewalFollowUp, input)
      const result = await ctx.call('activate', c.ActivatePolicy, { policy: input.policy })
      return { renewed: child.instance.length > 0 && result.status === 'active' }
    })
    const run = host.workflow(impl, { policy: w.p3.ref })
    expect(await run.run()).toEqual({ status: 'completed', value: { renewed: true } })
    expect(host.started.map((x) => x.workflow)).toEqual([c.RenewalFollowUp.id, c.RenewalFollowUp.id])
    expect(await run.run()).toEqual({ status: 'completed', value: { renewed: true } })
    expect(host.started).toHaveLength(2) // replay returns the receipts
  })
})

describe('subscriptions, registration and network blocking', () => {
  it('delivers events to subscriptions in their own derived transaction', async () => {
    const host = createTestHost()
    const seen: unknown[] = []
    const sub = subscribe('acme.insurance.probe', c.PolicyRenewed, async (tx, e) => {
      seen.push(e.payload.premium)
      await tx.start(c.RenewalFollowUp, { policy: e.subject })
    })
    await host.deliver(sub, { subject: ref(c.Policy, 'p'), payload: { expiresOn: '2028-01-01', premium: '1.00', version: '2' } })
    expect(seen).toEqual(['1.00'])
    expect(host.started).toHaveLength(1)
    await expect(host.deliver(sub, { subject: ref(c.Policy, 'p'), payload: { nope: true } })).rejects.toThrow()
  })

  it('registers cases through test() and describe()', () => {
    const state = (globalThis as Record<symbol, { cases: { name: string }[] }>)[Symbol.for('iota.testing.state')]!
    const before = state.cases.length
    group('outer', () => {
      group('inner', () => registerTest('works', () => undefined))
    })
    expect(state.cases.slice(before).map((x) => x.name)).toEqual(['outer > inner > works'])
    state.cases.length = before
  })

  it('exposes the fake http through the shared testing state', async () => {
    const host = createTestHost()
    const state = (globalThis as Record<symbol, { http: unknown }>)[Symbol.for('iota.testing.state')]!
    expect(state.http).toBe(host.fakes.http)
    await expect(host.fakes.http.request({ method: 'GET', url: 'https://x.test', headers: {} })).rejects.toBeInstanceOf(UnmatchedRequestError)
  })
})
