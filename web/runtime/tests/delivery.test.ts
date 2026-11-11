import { describe, expect, it } from 'vitest'
import { decimal, isoDate, ref } from '../src/index.ts'
import { recipient } from '../src/server.ts'
import { assert, AssertionError, createTestHost, fakeEffects, fixture, type TestHostOptions } from '../src/testing.ts'
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
