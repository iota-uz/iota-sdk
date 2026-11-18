import { describe, expect, it } from 'vitest'
import { decimal, deliveryPolicy, isoDate, ref } from '../src/index.ts'
import { implement, recipient } from '../src/server.ts'
import type { Channel } from '../src/model.ts'
import type { Ref } from '../src/schema.ts'
import { assert, AssertionError, createTestHost, fakeEffects, fixture, type Fixture, type TestHostOptions } from '../src/testing.ts'
import * as c from '../typetests/acme/contracts.ts'
import * as s from '../typetests/acme/server.ts'
import * as kit from '../typetests/kit.ts'

const expiresOn = isoDate('2027-01-01')

function setup(extra: Pick<TestHostOptions, 'contacts'> = {}) {
  const ali = fixture(c.Customer, { name: 'Ali', phone: '+998901112233' })
  const noPhone = fixture(c.Customer, { name: 'Vali' })
  const policy = fixture(c.Policy, { number: 'P-1', holder: ali.ref, premium: decimal('10.00'), expiresOn })
  const user = fixture(kit.User, { name: 'Operator', email: 'op@example.uz' }, { key: '42' })
  const host = createTestHost({
    fixtures: [ali, noPhone, policy, user],
    entities: [c.Customer],
    implementations: [s.reminderTemplates, s.customerContacts],
    ...extra,
  })
  return { host, ali, noPhone, policy }
}

describe('notice delivery', () => {
  it('renders the templates and delivers to the fake channels per recipient', async () => {
    const { host, policy, ali } = setup()
    await host.transaction((tx) =>
      tx.notify(c.RenewalReminder, {
        subject: policy.ref,
        payload: { number: 'P-1', expiresOn },
        to: [recipient.address({ sms: '+998900000001', email: 'a@example.uz' }), recipient.record(ali.ref), recipient.user(ref(kit.User, '42'))],
        channels: ['sms', 'email', 'inApp', 'push'],
      }),
    )
    expect(host.notices).toHaveLength(1)
    expect(host.fakes.sms.delivered.map((m) => [m.to, m.text])).toEqual([
      ['+998900000001', 'Policy P-1 expires on 2027-01-01'],
      ['+998901112233', 'Policy P-1 expires on 2027-01-01'],
    ])
    expect(host.fakes.email.delivered.map((m) => m.to)).toEqual(['a@example.uz', 'op@example.uz'])
    expect(host.fakes.email.delivered[0]).toMatchObject({ subject: 'Renewal P-1', text: 'P-1' })
    expect(host.fakes.inApp.delivered).toEqual([expect.objectContaining({ to: '42', title: 'P-1', body: '2027-01-01' })])
    expect(host.fakes.push.delivered).toEqual([expect.objectContaining({ to: '42', data: { policy: 'P-1' } })])

    const byKey = (status: string) => host.deliveries.filter((d) => d.status === status).map((d) => `${d.recipient.kind}/${d.channel}`)
    expect(byKey('delivered')).toEqual([
      'address/sms', 'address/email', 'record/sms', 'user/email', 'user/inApp', 'user/push',
    ])
    expect(host.deliveries.filter((d) => d.status === 'skipped').map((d) => [d.recipient.kind, d.channel, d.reason])).toEqual([
      ['address', 'inApp', 'the address has no inApp entry'],
      ['address', 'push', 'the address has no push entry'],
      ['record', 'email', expect.stringContaining('has no email target')],
      ['record', 'inApp', 'inApp notifications address ERP users, not records'],
      ['record', 'push', expect.stringContaining('has no push target')],
      ['user', 'sms', expect.stringContaining('has no phone')],
    ])
  })

  it('uses the contacts option before the bound resolver', async () => {
    const { host, noPhone, policy } = setup({ contacts: (record) => (record.key === noPhone.ref.key ? { sms: ['+998905550000'] } : undefined) })
    await host.transaction((tx) =>
      tx.notify(c.RenewalReminder, { subject: policy.ref, payload: { number: 'P-1', expiresOn }, to: [recipient.record(noPhone.ref)], channels: ['sms'] }),
    )
    expect(host.fakes.sms.delivered.map((m) => m.to)).toEqual(['+998905550000'])
  })

  it('records a rolled back transaction as never sent', async () => {
    const { host, policy } = setup()
    await expect(
      host.transaction(async (tx) => {
        await tx.notify(c.RenewalReminder, { subject: policy.ref, payload: { number: 'P-1', expiresOn }, to: [recipient.address({ sms: '+998900000001' })], channels: ['sms'] })
        throw new Error('boom')
      }),
    ).rejects.toThrow('boom')
    expect(host.notices).toEqual([])
    expect(host.deliveries).toEqual([])
    expect(host.fakes.sms.calls).toEqual([])
  })

  it('reports a failing provider as a failed delivery, not as an error of the transaction', async () => {
    const fakes = fakeEffects()
    fakes.sms.failNext(new Error('gateway down'))
    const ali = fixture(c.Customer, { name: 'Ali' })
    const policy = fixture(c.Policy, { number: 'P-1', holder: ali.ref, premium: decimal('1.00'), expiresOn })
    const host = createTestHost({ fixtures: [ali, policy], fakes, templates: [s.reminderTemplates] })
    await host.transaction((tx) =>
      tx.notify(c.RenewalReminder, { subject: policy.ref, payload: { number: 'P-1', expiresOn }, to: [recipient.address({ sms: '+998900000001' })], channels: ['sms'] }),
    )
    expect(host.deliveries).toEqual([expect.objectContaining({ status: 'failed', reason: 'gateway down', target: '+998900000001' })])
    expect(fakes.sms.delivered).toEqual([])
  })

  it('fails when templates are configured but none matches the notice', async () => {
    const ali = fixture(c.Customer, { name: 'Ali' })
    const policy = fixture(c.Policy, { number: 'P-1', holder: ali.ref, premium: decimal('1.00'), expiresOn })
    const host = createTestHost({ fixtures: [ali, policy], templates: [{ ...s.reminderTemplates, notice: { ...c.RenewalReminder, id: 'acme.insurance.other.v1' } }] })
    await expect(
      host.transaction((tx) =>
        tx.notify(c.RenewalReminder, { subject: policy.ref, payload: { number: 'P-1', expiresOn }, to: [recipient.address({ sms: '+998900000001' })], channels: ['sms'] }),
      ),
    ).rejects.toMatchObject({ code: 'notify.noTemplates' })
  })

  it('only records notices when no templates are bound', async () => {
    const ali = fixture(c.Customer, { name: 'Ali' })
    const policy = fixture(c.Policy, { number: 'P-1', holder: ali.ref, premium: decimal('1.00'), expiresOn })
    const host = createTestHost({ fixtures: [ali, policy] })
    await host.transaction((tx) =>
      tx.notify(c.RenewalReminder, { subject: policy.ref, payload: { number: 'P-1', expiresOn }, to: [recipient.address({ sms: '+998900000001' })], channels: ['sms'] }),
    )
    expect(host.notices).toHaveLength(1)
    expect(host.deliveries).toEqual([])
  })
})

describe('delivery policies', () => {
  const send = (host: ReturnType<typeof createTestHost>, to: Ref | undefined, channels: readonly [Channel, ...Channel[]], policy: Fixture<typeof c.Policy>) =>
    host.transaction((tx) =>
      tx.notify(c.RenewalReminder, {
        subject: policy.ref,
        payload: { number: 'P-1', expiresOn },
        to: to === undefined ? [recipient.address({ sms: '+1555000', email: 'x@example.uz' })] : [recipient.record(to)],
        channels,
      }),
    )

  function policyHost(extra: TestHostOptions = {}) {
    const ali = fixture(c.Customer, { name: 'Ali', phone: '+998901112233' })
    const abroad = fixture(c.Customer, { name: 'Bob', phone: '+1555123456' })
    const policy = fixture(c.Policy, { number: 'P-1', holder: ali.ref, premium: decimal('10.00'), expiresOn })
    const host = createTestHost({
      fixtures: [ali, abroad, policy],
      implementations: [s.reminderTemplates, s.customerContacts, s.customerConsent],
      contacts: (record) => (record.key === ali.ref.key ? { sms: ['+998901112233'], email: ['ali@example.uz'] } : record.key === abroad.ref.key ? { sms: ['+1555123456'], email: ['bob@example.uz'] } : undefined),
      ...extra,
    })
    return { host, ali, abroad, policy }
  }

  it('skips a denied target as policy_denied and still delivers the others', async () => {
    const { host, abroad, policy } = policyHost()
    await send(host, abroad.ref, ['sms', 'email'], policy)
    expect(host.fakes.sms.delivered).toEqual([])
    expect(host.fakes.email.delivered.map((m) => m.to)).toEqual(['bob@example.uz'])
    expect(host.deliveries.map((d) => [d.channel, d.target, d.status, d.reason])).toEqual([
      ['sms', '+1555123456', 'skipped', 'policy_denied'],
      ['email', 'bob@example.uz', 'delivered', undefined],
    ])
  })

  it('allows what the policy allows', async () => {
    const { host, ali, policy } = policyHost()
    await send(host, ali.ref, ['sms'], policy)
    expect(host.fakes.sms.delivered.map((m) => m.to)).toEqual(['+998901112233'])
  })

  it('passes the platform request shape, with a snake_case notice key and the configured snapshot', async () => {
    const seen: unknown[] = []
    const spy = implement(c.CustomerConsent, async (_q, request) => {
      seen.push(request)
      return { allow: true }
    })
    const { host, ali, policy } = policyHost({ implementations: [s.reminderTemplates, s.customerContacts, spy], snapshot: 'acme.insurance@1.4.0' })
    await send(host, ali.ref, ['email'], policy)
    expect(seen).toEqual([
      {
        notice: { key: 'acme.insurance.renewal_reminder.v1', version: 1, snapshot: 'acme.insurance@1.4.0' },
        subject: policy.ref,
        recipient: { kind: 'record', ref: ali.ref, address: null },
        channel: 'email',
        target: { address: 'ali@example.uz', deviceId: null },
      },
    ])
    const dflt = policyHost({ implementations: [s.reminderTemplates, s.customerContacts, spy] })
    seen.length = 0
    await send(dflt.host, dflt.ali.ref, ['sms'], dflt.policy)
    expect(seen).toEqual([expect.objectContaining({ notice: expect.objectContaining({ snapshot: 'acme.insurance@0.0.0' }) })])
  })

  it('stores a valid reason and drops one that is not a machine code', async () => {
    const reasoned = (reason: string) => implement(c.CustomerConsent, async () => ({ allow: false, reason }))
    for (const [reason, stored] of [['sms_opt_out', 'policy_denied:sms_opt_out'], ['Has Spaces', 'policy_denied'], ['x'.repeat(65), 'policy_denied']] as const) {
      const { host, ali, policy } = policyHost({ implementations: [s.reminderTemplates, s.customerContacts, reasoned(reason)] })
      await send(host, ali.ref, ['sms'], policy)
      expect(host.deliveries.map((d) => [d.status, d.reason])).toEqual([['skipped', stored]])
    }
  })

  it('asks every bound policy in contract-ID order and the first denial wins', async () => {
    const First = deliveryPolicy('acme.insurance.aFirst', c.Customer)
    const asked: string[] = []
    const allowing = implement(First, async () => (asked.push('aFirst'), { allow: true }))
    const denying = implement(c.CustomerConsent, async () => (asked.push('customerConsent'), { allow: false, reason: 'second' }))
    const Last = deliveryPolicy('acme.insurance.zLast', c.Customer)
    const never = implement(Last, async () => (asked.push('zLast'), { allow: false, reason: 'third' }))
    const { host, ali, policy } = policyHost({ implementations: [s.reminderTemplates, s.customerContacts, never, denying, allowing] })
    await send(host, ali.ref, ['sms'], policy)
    expect(asked).toEqual(['aFirst', 'customerConsent'])
    expect(host.deliveries.map((d) => d.reason)).toEqual(['policy_denied:second'])
  })

  it('never lets a failing or malformed policy through', async () => {
    const throwing = implement(c.CustomerConsent, async () => {
      throw new Error('boom')
    })
    const malformed = implement(c.CustomerConsent, (async () => ({ allow: 'yes' })) as never)
    for (const [impl, text] of [[throwing, 'boom'], [malformed, 'must return {allow: boolean']] as const) {
      const { host, ali, policy } = policyHost({ implementations: [s.reminderTemplates, s.customerContacts, impl] })
      await send(host, ali.ref, ['sms'], policy)
      expect(host.fakes.sms.delivered).toEqual([])
      expect(host.deliveries).toEqual([expect.objectContaining({ status: 'failed', reason: expect.stringContaining(text) })])
    }
  })

  it('does not apply to address recipients or to a policy of another entity', async () => {
    const asked: string[] = []
    const spy = implement(c.CustomerConsent, async () => (asked.push('asked'), { allow: false }))
    const { host, policy } = policyHost({ implementations: [s.reminderTemplates, s.customerContacts, spy] })
    await send(host, undefined, ['sms', 'email'], policy)
    expect(asked).toEqual([])
    expect(host.fakes.sms.delivered.map((m) => m.to)).toEqual(['+1555000'])
  })
})

describe('assert', () => {
  it('passes and fails with readable messages', async () => {
    assert.ok(true)
    assert.equal(1, 1)
    assert.deepEqual({ a: [1, { b: 2 }] }, { a: [1, { b: 2 }] })
    assert.partial({ a: 1, b: { c: 2, d: 3 } }, { b: { c: 2 } })
    assert.match('policy P-1', /P-\d/)
    assert.throws(() => { throw new Error('nope') }, 'nope')
    await assert.rejects(Promise.reject(new TypeError('bad')), TypeError)
    await assert.rejects(() => Promise.reject(Object.assign(new Error('x'), { code: 'data.notFound' })), { code: 'data.notFound' })

    expect(() => assert.equal(1, 2)).toThrow(AssertionError)
    expect(() => assert.deepEqual({ a: [1, { b: 2 }] }, { a: [1, { b: 3 }] })).toThrow('values differ at a[1].b')
    expect(() => assert.partial({ a: 1 }, { a: 2 })).toThrow('first difference at a')
    expect(() => assert.throws(() => undefined)).toThrow('expected the function to throw')
    await expect(assert.rejects(Promise.resolve(1))).rejects.toThrow('expected the promise to reject')
    await expect(assert.rejects(Promise.reject(new Error('a')), 'b')).rejects.toThrow('rejected with Error: a, expected message b')
  })

  it('matches domain errors and validation issues of a contract', async () => {
    const { host } = setup()
    const err = c.NotActive.of({ status: 'expired' })
    await assert.rejects(Promise.reject(err), c.NotActive)
    await assert.rejects(() => host.transaction(async () => { throw err }), { kind: 'domain', code: c.NotActive.id })
  })
})
