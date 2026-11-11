import { describe, expect, it } from 'vitest'
import { WireError, decimal, decode, encode, int64, isoDate, isoDateTime, t, toBigInt, validate } from '../src/index.ts'

const fails = (schema: Parameters<typeof validate>[0], value: unknown, message: RegExp) =>
  expect(() => validate(schema, value)).toThrowError(message)

describe('wire codec', () => {
  it('round-trips int64 beyond 2^53 as strings', () => {
    const big = int64(9223372036854775807n)
    expect(big).toBe('9223372036854775807')
    const schema = t.object({ id: t.int64() })
    const wire = JSON.parse(JSON.stringify(encode(schema, { id: big })))
    expect(wire).toEqual({ id: '9223372036854775807' })
    expect(toBigInt(decode(schema, wire).id)).toBe(9223372036854775807n)
    expect(int64(-(2n ** 63n))).toBe('-9223372036854775808')
  })

  it('rejects float, number and out-of-range int64', () => {
    fails(t.int64(), 12, /expected string for int64/)
    fails(t.int64(), '1.5', /int64 decimal string/)
    fails(t.int64(), '9223372036854775808', /out of range/)
    fails(t.int64(), '01', /int64 decimal string/)
    expect(() => int64('9223372036854775808')).toThrow(WireError)
  })

  it('accepts only safe integers for int', () => {
    validate(t.int(), 9007199254740991)
    validate(t.int(), -5)
    fails(t.int(), 9007199254740992, /safe integer/)
    fails(t.int(), 1.5, /safe integer/)
    fails(t.int(), '1', /safe integer/)
    fails(t.int(), Number.NaN, /safe integer/)
  })

  it('validates decimal precision and scale', () => {
    const price = t.decimal({ precision: 6, scale: 2 })
    validate(price, '1234.56')
    validate(price, '-0.10')
    fails(price, '1.234', /fractional digits/)
    fails(price, '12345.67', /more than 6 digits/)
    fails(price, '1e3', /decimal string/)
    fails(price, 12.5, /expected string for decimal/)
    expect(decimal('12.50')).toBe('12.50')
  })

  it('validates dates and datetimes strictly', () => {
    validate(t.date(), '2026-02-28')
    fails(t.date(), '2026-02-30', /YYYY-MM-DD/)
    fails(t.date(), '2026-2-3', /YYYY-MM-DD/)
    validate(t.datetime(), '2026-11-11T10:00:00Z')
    validate(t.datetime(), '2026-11-11T10:00:00.123456+05:00')
    fails(t.datetime(), '2026-11-11T10:00:00', /RFC3339/)
    fails(t.datetime(), '2026-11-11 10:00:00Z', /RFC3339/)
    expect(isoDate(new Date('2026-03-04T23:00:00Z'))).toBe('2026-03-04')
    expect(isoDateTime(new Date('2026-03-04T23:00:00Z'))).toBe('2026-03-04T23:00:00.000Z')
  })

  it('checks refs, enums, uuids and max length', () => {
    const policy = { id: 'acme.insurance.policy' }
    validate(t.ref(policy), { entity: 'acme.insurance.policy', key: 'k' })
    fails(t.ref(policy), { entity: 'acme.insurance.customer', key: 'k' }, /expected ref to acme.insurance.policy/)
    fails(t.ref(policy), { entity: 'acme.insurance.policy', key: '' }, /expected ref/)
    validate(t.enum(['a', 'b']), 'a')
    fails(t.enum(['a', 'b']), 'c', /one of a, b/)
    validate(t.uuid(), '123e4567-e89b-42d3-a456-426614174000')
    fails(t.uuid(), 'nope', /uuid/)
    fails(t.string({ maxLength: 3 }), 'абвг', /longer than 3/)
    validate(t.string({ maxLength: 3 }), 'абв')
  })

  it('handles optional, nullable, unknown keys and nested paths', () => {
    const schema = t.object({
      a: t.string(),
      b: t.string().optional(),
      c: t.int().nullable(),
      items: t.array(t.object({ n: t.int() })),
    })
    validate(schema, { a: 'x', c: null, items: [] })
    expect(encode(schema, { a: 'x', b: undefined, c: 1, items: [{ n: 1 }] })).toEqual({ a: 'x', c: 1, items: [{ n: 1 }] })
    fails(schema, { c: 1, items: [] }, /^a: value is required/)
    fails(schema, { a: 'x', b: null, c: 1, items: [] }, /^b: value is required/)
    fails(schema, { a: 'x', c: 1, items: [], z: 1 }, /^z: unknown property/)
    fails(schema, { a: 'x', c: 1, items: [{ n: 1 }, { n: 'x' }] }, /^items\.1\.n: expected safe integer/)
    try {
      validate(schema, { a: 'x', c: 1, items: [{ n: 1.5 }] })
    } catch (e) {
      expect((e as WireError).path).toEqual(['items', '0', 'n'])
    }
  })

  it('accepts only JSON values for json', () => {
    validate(t.json(), { a: [1, 'x', null] })
    fails(t.json(), { d: new Date() }, /invalid json/)
    fails(t.json(), { n: Number.POSITIVE_INFINITY }, /non-finite/)
  })
})
