import { describe, expect, it } from 'vitest'
import { EPOCH, fakeEffects, fixture, overlayFixture, UnmatchedRequestError } from '../src/testing.ts'

import { nativeEntity } from '../src/kit.ts'
import { field, WireError } from '../src/index.ts'
import * as kit from '../typetests/kit.ts'
import * as c from '../typetests/acme/contracts.ts'

describe('fixture', () => {
  it('applies defaults, nulls optionals and derives a stable key', () => {
    const customer = fixture(c.Customer, { name: 'Ali' })
    const again = fixture(c.Customer, { name: 'Ali' })
    expect(customer.ref).toEqual({ entity: 'acme.insurance.customer', key: customer.record.key })
    expect(customer.ref).toEqual(again.ref)
    expect(customer.record).toMatchObject({ name: 'Ali', phone: null, owner: null, rowVersion: 1 })

    const policy = fixture(c.Policy, {
      number: 'P-1',
      holder: customer.ref,
      premium: '120.00' as never,
      expiresOn: '2027-01-01' as never,
    })
    expect(policy.record.status).toBe('draft')
    expect(policy.record.holder).toEqual(customer.ref)
  })

  it('validates values and requires keys for native entities', () => {
    expect(() => fixture(c.Customer, { name: 5 as never })).toThrow(WireError)
    expect(() => fixture(kit.Client, { name: 'x' })).toThrow(/explicit key/)
    expect(fixture(kit.Client, { name: 'x' }, { key: '3f1b6a54-0a5e-4a52-9d8a-7ee1a3a3d8b1' }).ref.key).toBe('3f1b6a54-0a5e-4a52-9d8a-7ee1a3a3d8b1')
  })

  it('defaults createdAt and updatedAt to EPOCH on every entity, declared as fields or not', () => {
    // core.User declares them as required fields, crm.Client as optional ones, other natives not at all.
    const required = nativeEntity('core.user2', { name: field.text(), createdAt: field.datetime(), updatedAt: field.datetime() }, { key: 'int64' })
    const optional = nativeEntity('crm.client2', { name: field.text(), createdAt: field.datetime().optional(), updatedAt: field.datetime().optional() }, { key: 'uuid' })
    const none = nativeEntity('crm.client3', { name: field.text() }, { key: 'uuid' })
    for (const fx of [fixture(required, { name: 'x' }, { key: '1' }), fixture(optional, { name: 'x' }, { key: 'k' }), fixture(none, { name: 'x' }, { key: 'k' })]) {
      expect(fx.record.createdAt).toBe(EPOCH)
      expect(fx.record.updatedAt).toBe(EPOCH)
    }
    expect(fixture(c.Customer, { name: 'x' }).record.createdAt).toBe(EPOCH)
    // Explicit values win, wherever the column is declared.
    const later = '2026-06-01T00:00:00Z' as typeof EPOCH
    expect(fixture(required, { name: 'x', createdAt: later }, { key: '1' }).record).toMatchObject({ createdAt: later, updatedAt: EPOCH })
    expect(fixture(optional, { name: 'x', updatedAt: later }, { key: 'k' }).record).toMatchObject({ createdAt: EPOCH, updatedAt: later })
  })

  it('builds overlay rows with defaults', () => {
    const client = fixture(kit.Client, { name: 'x' }, { key: 'k' })
    expect(overlayFixture(c.ClientExtra, client.ref)).toEqual({ baseKey: 'k', loyaltyTier: null, points: 0 })
    expect(overlayFixture(c.ClientExtra, client.ref, { loyaltyTier: 'gold' }).loyaltyTier).toBe('gold')
  })
})

describe('fakeEffects', () => {
  it('records sms, dedupes by idempotency key and injects failures', async () => {
    const fx = fakeEffects()
    const first = await fx.sms.send({ to: '+998901112233', text: 'a', idempotencyKey: 'i:1' })
    const repeat = await fx.sms.send({ to: '+998901112233', text: 'a', idempotencyKey: 'i:1' })
    expect(repeat).toEqual(first)
    expect(fx.sms.calls).toHaveLength(2)
    expect(fx.sms.delivered).toHaveLength(1)
    fx.email.failNext(new Error('smtp down'))
    await expect(fx.email.send({ to: 'a@b.uz', subject: 's' })).rejects.toThrow('smtp down')
    await expect(fx.email.send({ to: 'a@b.uz', subject: 's' })).resolves.toEqual({ id: 'email-1' })
    fx.reset()
    expect(fx.sms.calls).toEqual([])
  })

  it('serves canned http responses and rejects unknown routes', async () => {
    const fx = fakeEffects()
    fx.http.route('get', 'https://q.example.uz/1', { status: 200, body: { premium: '10.00' } })
    await expect(fx.http.request({ method: 'GET', url: 'https://q.example.uz/1', headers: {} })).resolves.toMatchObject({ status: 200 })
    await expect(fx.http.request({ method: 'GET', url: 'https://q.example.uz/2', headers: {} })).rejects.toBeInstanceOf(UnmatchedRequestError)
    expect(fx.http.calls).toHaveLength(2)
  })
})
